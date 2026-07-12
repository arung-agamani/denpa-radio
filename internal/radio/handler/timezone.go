package handler

import (
	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// TimezoneHandlers holds the gin route handlers for timezone endpoints.
type TimezoneHandlers struct {
	svc *service.RadioService
}

func NewTimezoneHandlers(svc *service.RadioService) *TimezoneHandlers {
	return &TimezoneHandlers{svc: svc}
}

// GetTimezone handles GET /api/timezone
func (h *TimezoneHandlers) GetTimezone(c *gin.Context) {
	tz, serverTime := h.svc.GetTimezone()
	apiresponse.OK(c, gin.H{"timezone": tz, "server_time": serverTime})
}

// SetTimezone handles PUT /api/timezone  (protected)
func (h *TimezoneHandlers) SetTimezone(c *gin.Context) {
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	tz, serverTime, activeTag, err := h.svc.SetTimezone(body.Timezone)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{
		"timezone":    tz,
		"server_time": serverTime,
		"active_tag":  activeTag,
	})
}
