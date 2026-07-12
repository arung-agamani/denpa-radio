package handler

import (
	"fmt"
	"log/slog"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/auth"
	"github.com/gin-gonic/gin"
)

// AuthHandlers holds the gin route handlers for authentication endpoints.
type AuthHandlers struct {
	a *auth.Auth
}

func NewAuthHandlers(a *auth.Auth) *AuthHandlers {
	return &AuthHandlers{a: a}
}

// Login handles POST /api/auth/login
func (h *AuthHandlers) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	if len(body.Username) == 0 || len(body.Username) > 256 ||
		len(body.Password) == 0 || len(body.Password) > 256 {
		apiresponse.Error(c, apierror.ErrValidation("invalid credentials format"))
		return
	}
	token, err := h.a.Authenticate(body.Username, body.Password, c.Request.RemoteAddr)
	if err != nil {
		slog.Warn("Failed login attempt",
			"remote", c.Request.RemoteAddr,
			"error_type", err.Error(),
		)
		if err == auth.ErrRateLimited {
			remaining := h.a.RemainingLockout(c.Request.RemoteAddr)
			c.Header("Retry-After", fmt.Sprintf("%d", int(remaining.Seconds())))
			apiresponse.Error(c, apierror.ErrRateLimited("too many login attempts, please try again later"))
			return
		}
		apiresponse.Error(c, apierror.ErrUnauthorized("invalid credentials"))
		return
	}
	slog.Info("DJ logged in", "username", body.Username, "remote", c.Request.RemoteAddr)
	apiresponse.OK(c, gin.H{
		"token":    token,
		"username": body.Username,
	})
}

// VerifyToken handles GET /api/auth/verify  (middleware already validated the token)
func (h *AuthHandlers) VerifyToken(c *gin.Context) {
	apiresponse.OK(c, gin.H{"message": "token is valid"})
}
