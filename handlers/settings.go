package handlers

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type ToggleAutoPilotRequest struct {
	Platform string `json:"platform"`
	Enabled  bool   `json:"enabled"`
}

func ToggleAutoPilotHandler(c *gin.Context) {
	var req ToggleAutoPilotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	result := database.DB.Model(&models.SocialAccount{}).
		Where("platform = ?", req.Platform).
		Update("auto_pilot_enabled", req.Enabled)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Auto-Pilot status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Auto-Pilot status updated successfully", "enabled": req.Enabled})
}
