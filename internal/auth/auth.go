package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("token has expired")
	ErrMissingToken       = errors.New("missing authorization token")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrRateLimited        = errors.New("too many login attempts, please try again later")
)

// Config holds the authentication configuration.
type Config struct {
	Username  string
	Password  string
	JWTSecret string
	TokenTTL  time.Duration

	// Rate limiting configuration.
	// MaxLoginAttempts is the number of allowed failures per window.
	// LoginWindowSeconds is the duration of the sliding window in seconds.
	MaxLoginAttempts   int
	LoginWindowSeconds int
}

// jwtHeader is the fixed header for HS256 tokens.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// Claims represents the JWT payload.
type Claims struct {
	Sub string `json:"sub"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

// loginAttempt records a single failed login timestamp.
type loginAttempt struct {
	timestamps []time.Time
}

// rateLimiter tracks failed login attempts per IP address using a sliding
// window approach.
type rateLimiter struct {
	mu         sync.Mutex
	attempts   map[string]*loginAttempt
	maxFails   int
	windowSize time.Duration
}

func newRateLimiter(maxFails int, windowSize time.Duration) *rateLimiter {
	if maxFails <= 0 {
		maxFails = 5
	}
	if windowSize <= 0 {
		windowSize = 15 * time.Minute
	}
	rl := &rateLimiter{
		attempts:   make(map[string]*loginAttempt),
		maxFails:   maxFails,
		windowSize: windowSize,
	}
	// Background cleanup of stale entries every 5 minutes.
	go rl.cleanup()
	return rl
}

// isAllowed checks whether the given key (IP) is allowed to attempt login.
func (rl *rateLimiter) isAllowed(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.attempts[key]
	if !exists {
		return true
	}

	rl.pruneOld(entry)
	return len(entry.timestamps) < rl.maxFails
}

// recordFailure records a failed login attempt for the given key (IP).
func (rl *rateLimiter) recordFailure(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.attempts[key]
	if !exists {
		entry = &loginAttempt{}
		rl.attempts[key] = entry
	}

	rl.pruneOld(entry)
	entry.timestamps = append(entry.timestamps, time.Now())
}

// recordSuccess clears the failure record for the given key (IP) on
// successful authentication.
func (rl *rateLimiter) recordSuccess(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, key)
}

// pruneOld removes timestamps outside the sliding window. Caller must hold
// the mutex.
func (rl *rateLimiter) pruneOld(entry *loginAttempt) {
	cutoff := time.Now().Add(-rl.windowSize)
	n := 0
	for _, t := range entry.timestamps {
		if t.After(cutoff) {
			entry.timestamps[n] = t
			n++
		}
	}
	entry.timestamps = entry.timestamps[:n]
}

// cleanup periodically removes stale entries to prevent memory growth.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		for key, entry := range rl.attempts {
			rl.pruneOld(entry)
			if len(entry.timestamps) == 0 {
				delete(rl.attempts, key)
			}
		}
		rl.mu.Unlock()
	}
}

// remainingLockout returns how long until the oldest failure in the window
// expires, giving the client a hint of when to retry.
func (rl *rateLimiter) remainingLockout(key string) time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.attempts[key]
	if !exists || len(entry.timestamps) == 0 {
		return 0
	}

	rl.pruneOld(entry)
	if len(entry.timestamps) < rl.maxFails {
		return 0
	}

	// The oldest failure in the current window determines when the window
	// slides enough to allow a new attempt.
	oldest := entry.timestamps[0]
	return time.Until(oldest.Add(rl.windowSize))
}

// Auth handles authentication and JWT operations.
type Auth struct {
	config       Config
	passwordHash []byte
	limiter      *rateLimiter
}

// New creates a new Auth instance with the given configuration.
// The plaintext password from config is immediately hashed with bcrypt and the
// original is not retained in the Auth struct.
func New(cfg Config) *Auth {
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 24 * time.Hour
	}
	if cfg.MaxLoginAttempts == 0 {
		cfg.MaxLoginAttempts = 5
	}
	if cfg.LoginWindowSeconds == 0 {
		cfg.LoginWindowSeconds = 900 // 15 minutes
	}

	// Validate JWT secret strength.
	if len(cfg.JWTSecret) < 32 {
		slog.Warn("JWT secret is shorter than 32 characters — this is insecure in production")
	}
	if cfg.JWTSecret == "change-me-in-production-please" {
		slog.Warn("Using default JWT secret — CHANGE THIS in production!")
	}

	// Pre-hash the configured password with bcrypt so we never compare
	// plaintext passwords at runtime.
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		// This should essentially never fail with valid input. Fall back to a
		// hash that will never match so the server can still start but login
		// will always fail.
		slog.Error("Failed to hash DJ password with bcrypt", "error", err)
		hash = []byte("$2a$10$INVALIDHASHXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX")
	}

	// Clear the plaintext password from the config copy held in memory.
	cfg.Password = ""

	windowDuration := time.Duration(cfg.LoginWindowSeconds) * time.Second

	return &Auth{
		config:       cfg,
		passwordHash: hash,
		limiter:      newRateLimiter(cfg.MaxLoginAttempts, windowDuration),
	}
}

// Authenticate checks the provided username and password against the
// configured credentials using bcrypt. Returns a signed JWT token string on
// success. The remoteAddr parameter is used for rate limiting.
func (a *Auth) Authenticate(username, password, remoteAddr string) (string, error) {
	// Extract IP from remoteAddr (strip port).
	ip := extractIP(remoteAddr)

	// Check rate limiter first.
	if !a.limiter.isAllowed(ip) {
		remaining := a.limiter.remainingLockout(ip)
		slog.Warn("Login rate-limited",
			"ip", ip,
			"retry_after_seconds", int(remaining.Seconds()),
		)
		return "", ErrRateLimited
	}

	// Constant-time username comparison to avoid timing side-channels on the
	// username alone. We still check both username and password before
	// returning to avoid leaking which one was wrong.
	usernameMatch := hmacEqualStrings(username, a.config.Username)

	// Always run bcrypt comparison even if username is wrong, to prevent
	// timing attacks that could reveal whether the username exists.
	passwordErr := bcrypt.CompareHashAndPassword(a.passwordHash, []byte(password))
	passwordMatch := passwordErr == nil

	if !usernameMatch || !passwordMatch {
		a.limiter.recordFailure(ip)
		return "", ErrInvalidCredentials
	}

	// Successful auth — clear rate limit history for this IP.
	a.limiter.recordSuccess(ip)

	return a.CreateToken(username)
}

// CreateToken generates a signed JWT token for the given subject.
func (a *Auth) CreateToken(subject string) (string, error) {
	now := time.Now()
	claims := Claims{
		Sub: subject,
		Iat: now.Unix(),
		Exp: now.Add(a.config.TokenTTL).Unix(),
	}
	return a.sign(claims)
}

// ValidateToken parses and validates a JWT token string. Returns the claims if
// the token is valid and not expired.
func (a *Auth) ValidateToken(tokenStr string) (*Claims, error) {
	// Reject obviously malformed or excessively long tokens.
	if len(tokenStr) > 4096 {
		return nil, ErrInvalidToken
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Validate header to ensure algorithm is what we expect (HS256 only).
	headerJSON, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode header", ErrInvalidToken)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("%w: failed to parse header", ErrInvalidToken)
	}

	// Reject algorithm confusion attacks — only accept HS256.
	if header.Alg != "HS256" {
		return nil, fmt.Errorf("%w: unsupported algorithm %q", ErrInvalidToken, header.Alg)
	}
	if header.Typ != "JWT" {
		return nil, fmt.Errorf("%w: unsupported type %q", ErrInvalidToken, header.Typ)
	}

	// Verify signature.
	signingInput := parts[0] + "." + parts[1]
	expectedSig := a.computeHMAC(signingInput)
	actualSig := parts[2]

	if !hmacEqualB64(expectedSig, actualSig) {
		return nil, ErrInvalidToken
	}

	// Decode claims.
	claimsJSON, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode claims", ErrInvalidToken)
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("%w: failed to parse claims", ErrInvalidToken)
	}

	// Check expiry.
	now := time.Now().Unix()
	if now > claims.Exp {
		return nil, ErrExpiredToken
	}

	// Reject tokens issued in the future (clock skew tolerance: 60 seconds).
	if claims.Iat > now+60 {
		return nil, fmt.Errorf("%w: token issued in the future", ErrInvalidToken)
	}

	// Validate subject is not empty.
	if claims.Sub == "" {
		return nil, fmt.Errorf("%w: empty subject", ErrInvalidToken)
	}

	return &claims, nil
}

// Middleware returns an HTTP middleware that requires a valid JWT token in the
// Authorization header (Bearer scheme). If the token is missing or invalid, a
// 401 response is returned.
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractBearerToken(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		// Attach claims to request context if needed in the future.
		_ = claims

		next.ServeHTTP(w, r)
	})
}

// MiddlewareFunc is a convenience wrapper that works with http.HandlerFunc.
func (a *Auth) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := extractBearerToken(r)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		_ = claims

		next.ServeHTTP(w, r)
	}
}

// IsRateLimited checks if the given remote address is currently rate-limited
// for login attempts.
func (a *Auth) IsRateLimited(remoteAddr string) bool {
	ip := extractIP(remoteAddr)
	return !a.limiter.isAllowed(ip)
}

// RemainingLockout returns the time remaining until the given IP can retry.
func (a *Auth) RemainingLockout(remoteAddr string) time.Duration {
	ip := extractIP(remoteAddr)
	return a.limiter.remainingLockout(ip)
}

// --- Internal helpers ---

// sign creates a complete JWT string from the given claims.
func (a *Auth) sign(claims Claims) (string, error) {
	header := jwtHeader{Alg: "HS256", Typ: "JWT"}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	headerB64 := base64URLEncode(headerJSON)
	claimsB64 := base64URLEncode(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	signature := a.computeHMAC(signingInput)

	return signingInput + "." + signature, nil
}

// computeHMAC computes HMAC-SHA256 of the input using the configured secret,
// returning the base64url-encoded result.
func (a *Auth) computeHMAC(input string) string {
	mac := hmac.New(sha256.New, []byte(a.config.JWTSecret))
	mac.Write([]byte(input))
	return base64URLEncode(mac.Sum(nil))
}

// hmacEqualB64 performs a constant-time comparison of two base64url-encoded
// HMAC signatures to prevent timing attacks.
func hmacEqualB64(a, b string) bool {
	aDec, errA := base64URLDecode(a)
	bDec, errB := base64URLDecode(b)
	if errA != nil || errB != nil {
		return false
	}
	return hmac.Equal(aDec, bDec)
}

// hmacEqualStrings performs a constant-time comparison of two strings to
// prevent timing-based user enumeration.
func hmacEqualStrings(a, b string) bool {
	// Use HMAC comparison on the raw bytes. We hash both sides so the
	// comparison is constant-time regardless of input length differences.
	h1 := sha256.Sum256([]byte(a))
	h2 := sha256.Sum256([]byte(b))
	return hmac.Equal(h1[:], h2[:])
}

// base64URLEncode encodes bytes to base64url without padding.
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// base64URLDecode decodes a base64url string (with or without padding).
func base64URLDecode(s string) ([]byte, error) {
	// Try without padding first (standard JWT), then with padding.
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(s)
	}
	return data, err
}

// extractBearerToken extracts the JWT token from the Authorization header.
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingToken
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("%w: expected Bearer scheme", ErrInvalidToken)
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrMissingToken
	}

	return token, nil
}

// extractIP extracts the IP address from a remote address string, stripping
// the port component. Handles both IPv4 ("1.2.3.4:1234") and IPv6
// ("[::1]:1234") formats.
func extractIP(remoteAddr string) string {
	// Handle IPv6 with brackets.
	if strings.HasPrefix(remoteAddr, "[") {
		if idx := strings.LastIndex(remoteAddr, "]:"); idx != -1 {
			return remoteAddr[1:idx]
		}
		return strings.Trim(remoteAddr, "[]")
	}

	// Handle IPv4.
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}

	return remoteAddr
}

// writeAuthError writes a JSON error response for authentication failures.
// Error messages are intentionally generic to avoid leaking information.
func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "error",
		"error":  message,
	})
}
