package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("token has expired")
	ErrMissingToken       = errors.New("missing authorization token")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// Config holds the authentication configuration.
type Config struct {
	Username  string
	Password  string
	JWTSecret string
	TokenTTL  time.Duration
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

// Auth handles authentication and JWT operations.
type Auth struct {
	config Config
}

// New creates a new Auth instance with the given configuration.
func New(cfg Config) *Auth {
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 24 * time.Hour
	}
	return &Auth{config: cfg}
}

// Authenticate checks the provided username and password against the configured
// credentials. Returns a signed JWT token string on success.
func (a *Auth) Authenticate(username, password string) (string, error) {
	if username != a.config.Username || password != a.config.Password {
		return "", ErrInvalidCredentials
	}
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
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Verify signature.
	signingInput := parts[0] + "." + parts[1]
	expectedSig := a.computeHMAC(signingInput)
	actualSig := parts[2]

	if !hmacEqual(expectedSig, actualSig) {
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
	if time.Now().Unix() > claims.Exp {
		return nil, ErrExpiredToken
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
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			status := http.StatusUnauthorized
			writeAuthError(w, status, err.Error())
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
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		claims, err := a.ValidateToken(token)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, err.Error())
			return
		}

		_ = claims

		next.ServeHTTP(w, r)
	}
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

// hmacEqual performs a constant-time comparison of two base64url-encoded HMAC
// signatures to prevent timing attacks.
func hmacEqual(a, b string) bool {
	aDec, errA := base64URLDecode(a)
	bDec, errB := base64URLDecode(b)
	if errA != nil || errB != nil {
		return false
	}
	return hmac.Equal(aDec, bDec)
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

// writeAuthError writes a JSON error response for authentication failures.
func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "error",
		"error":  message,
	})
}
