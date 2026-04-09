package handlers

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type DisconnectRequest struct {
	Platform string `json:"platform"`
}

func DisconnectHandler(c *gin.Context) {
	var req DisconnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not connected"})
		return
	}

	// 1. Find the account first to get the ID and verify existence
	var account models.SocialAccount
	if err := database.DB.Where("platform = ?", req.Platform).First(&account).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Account not found for " + req.Platform})
		return
	}

	// 2. Wipe all scheduled posts for this account to ensure a clean slate
	if err := database.DB.Where("account_id = ?", account.ID).Delete(&models.ScheduledPost{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to wipe scheduled posts"})
		return
	}

	// 3. Finally, delete the social account (soft delete)
	if err := database.DB.Delete(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to wipe account tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully disconnected from " + req.Platform + ". Your tokens have been wiped.",
	})
}
