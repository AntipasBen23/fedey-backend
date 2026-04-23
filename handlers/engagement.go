package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
)

// GetEngagementsHandler returns pending engagement events for the dashboard feed.
func GetEngagementsHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var events []models.EngagementEvent
	database.DB.Where("user_id = ?", uid).Order("created_at desc").Limit(20).Find(&events)

	c.JSON(http.StatusOK, events)
}

// ApproveEngagementHandler manually sends a proposed reply.
func ApproveEngagementHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	var event models.EngagementEvent
	if err := database.DB.Where("user_id = ?", uid).First(&event, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Engagement event not found"})
		return
	}

	var account models.SocialAccount
	database.DB.Where("user_id = ?", uid).First(&account, event.AccountID)

	_, success, err := utils.PostSingleTweetToX(account.AccessToken, event.ProposedReply)
	if !success {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send reply: %v", err)})
		return
	}

	database.DB.Model(&event).Update("status", "sent")
	c.JSON(http.StatusOK, gin.H{"message": "Reply sent successfully!"})
}

// ToggleGhostModeHandler switches the autonomous engagement flag.
func ToggleGhostModeHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	err := database.DB.Model(&models.SocialAccount{}).Where("user_id = ?", uid).Update("ghost_mode_enabled", req.Enabled).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update Ghost Mode settings"})
		return
	}

	msg := "Ghost Mode: DEACTIVATED (Review Drafts mode)"
	if req.Enabled {
		msg = "Ghost Mode: ACTIVATED (Full Autonomous mode)"
	}

	c.JSON(http.StatusOK, gin.H{"message": msg, "ghostModeEnabled": req.Enabled})
}
