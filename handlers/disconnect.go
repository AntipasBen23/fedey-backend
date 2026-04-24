package handlers

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
)

type DisconnectRequest struct {
	Platform string `json:"platform"`
}

func DisconnectHandler(c *gin.Context) {
	var req DisconnectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request.")
		return
	}

	if database.DB == nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Database not connected.")
		return
	}

	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		utils.APIError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required.")
		return
	}

	// 1. Find the account first to get the ID and verify existence
	var account models.SocialAccount
	if err := database.DB.Where("user_id = ? AND platform = ?", uid, req.Platform).First(&account).Error; err != nil {
		utils.APIError(c, http.StatusNotFound, "NOT_FOUND", "Account not found for "+req.Platform+".")
		return
	}

	// 2. Wipe ALL related data for this account to ensure a 100% clean slate
	// Delete Scheduled Posts
	database.DB.Where("user_id = ? AND account_id = ?", uid, account.ID).Delete(&models.ScheduledPost{})

	// Delete platform-specific Analytics (to prevent legacy data from polluting the new connection)
	database.DB.Where("scheduled_post_id IN (SELECT id FROM scheduled_posts WHERE user_id = ? AND account_id = ?)", uid, account.ID).Delete(&models.PostAnalytics{})

	// 3. Finally, delete the social account (soft delete)
	if err := database.DB.Delete(&account).Error; err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to wipe account tokens.")
		return
	}

	// 4. Wipe user's Draft Calendars (since calendars are generated based on the specific connection state)
	database.DB.Where("user_id = ? AND status = ?", uid, "draft").Delete(&models.ContentCalendar{})

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully disconnected from " + req.Platform + ". Your tokens have been wiped.",
	})
}
