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

	// Full reset — same cascade as admin delete, but user account is kept so they can re-onboard.

	// Tokens first so the session is dead immediately
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.RefreshToken{})

	// All user-owned data
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.SocialAccount{})
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.ContentCalendar{})
	// PostAnalytics has no user_id — cascade via post IDs
	database.DB.Unscoped().Where("scheduled_post_id IN (SELECT id FROM scheduled_posts WHERE user_id = ?)", uid).Delete(&models.PostAnalytics{})
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.ScheduledPost{})
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.UserStrategy{})
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.EngagementEvent{})
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.FollowerSnapshot{})
	database.DB.Unscoped().Where("user_id = ?", uid).Delete(&models.OnboardingOutreach{})

	// Reset onboarding state so the user starts fresh on next login
	database.DB.Model(&models.User{}).Where("id = ?", uid).Updates(map[string]interface{}{
		"last_onboarding_step": "",
		"job_description":      "",
		"platform_context":     "",
	})

	// Expire auth cookies — the client is logged out immediately
	clearAuthCookies(c)

	c.JSON(http.StatusOK, gin.H{
		"loggedOut": true,
		"message":   "Furci disconnected. All data has been wiped.",
	})
}
