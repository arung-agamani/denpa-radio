package radio

import (
	"strings"

	"github.com/arung-agamani/denpa-radio/internal/auth"
	"github.com/gin-gonic/gin"
)

// SecurityHeadersMiddleware adds standard HTTP security headers to every
// response. These mitigate clickjacking, MIME-sniffing, XSS reflection, and
// information leakage.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; media-src 'self'; connect-src 'self'; font-src 'self'")
		c.Next()
	}
}

// AuthRequired returns a gin middleware that enforces JWT authentication via
// the Authorization: Bearer <token> header. Aborts with 401 on failure.
func AuthRequired(a *auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"status": "error",
				"error":  "authentication required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(401, gin.H{
				"status": "error",
				"error":  "authentication required",
			})
			return
		}

		token := strings.TrimSpace(parts[1])
		if _, err := a.ValidateToken(token); err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"status": "error",
				"error":  "invalid or expired token",
			})
			return
		}

		c.Next()
	}
}
