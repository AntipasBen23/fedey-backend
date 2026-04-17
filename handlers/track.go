package handlers

import (
	"net/http"
	"strings"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type TrackRequest struct {
	Path      string `json:"path"`
	Referrer  string `json:"referrer"`
	SessionID string `json:"sessionId"`
	UserID    *uint  `json:"userId"`
}

// POST /v1/track — public, fire-and-forget page view recording
func TrackPageViewHandler(c *gin.Context) {
	var req TrackRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.Status(http.StatusNoContent)
		return
	}

	if database.DB == nil {
		c.Status(http.StatusNoContent)
		return
	}

	ua := c.GetHeader("User-Agent")
	pv := models.PageView{
		Path:       req.Path,
		Referrer:   req.Referrer,
		DeviceType: detectDevice(ua),
		SessionID:  req.SessionID,
		UserID:     req.UserID,
	}

	database.DB.Create(&pv)
	c.Status(http.StatusNoContent)
}

func detectDevice(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") ||
		strings.Contains(ua, "iphone") || strings.Contains(ua, "ipod") {
		return "mobile"
	}
	if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "tablet"
	}
	return "desktop"
}
