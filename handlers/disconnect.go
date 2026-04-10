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

	// 2. Wipe ALL related data for this account to ensures a 100% clean slate
	// Delete Scheduled Posts
	database.DB.Where("account_id = ?", account.ID).Delete(&models.ScheduledPost{})
	
	// Delete platform-specific Analytics (to prevent legacy data from polluting the new connection)
	database.DB.Where("scheduled_post_id IN (SELECT id FROM scheduled_posts WHERE account_id = ?)", account.ID).Delete(&models.PostAnalytics{})

	// 3. Finally, delete the social account (soft delete)
	if err := database.DB.Delete(&account).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to wipe account tokens"})
		return
	}

	// 4. Wipe ALL Draft Calendars (since calendars are generated based on the specific connection state)
	database.DB.Where("status = ?", "draft").Delete(&models.ContentCalendar{})

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully disconnected from " + req.Platform + ". Your tokens have been wiped.",
	})
}
