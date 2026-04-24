package handlers

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
)

type ToggleAutoPilotRequest struct {
	Platform string `json:"platform"`
	Enabled  bool   `json:"enabled"`
}

func ToggleAutoPilotHandler(c *gin.Context) {
	var req ToggleAutoPilotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request payload.")
		return
	}

	if database.DB == nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Database not initialized.")
		return
	}

	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		utils.APIError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required.")
		return
	}

	result := database.DB.Model(&models.SocialAccount{}).
		Where("user_id = ? AND platform = ?", uid, req.Platform).
		Update("auto_pilot_enabled", req.Enabled)

	if result.Error != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to update Auto-Pilot status.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Auto-Pilot status updated successfully", "enabled": req.Enabled})
}
