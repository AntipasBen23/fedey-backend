package handlers

import (
		"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type DashboardData struct {
	Calendar       []models.ScheduledPost `json:"calendar"`
	History        []models.ScheduledPost `json:"history"`
	SocialAccounts []models.SocialAccount `json:"socialAccounts"`
	Strategy       *models.UserStrategy   `json:"strategy"`
	Stats          map[string]interface{} `json:"stats"`
	Plan           string                 `json:"plan"`      // "free" | "pro"
	Analytics      AnalyticsOverview      `json:"analytics"` // live metrics
}

func GetDashboardHandler(c *gin.Context) {
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

	// 1. Pending posts (upcoming queue)
	var scheduledPosts []models.ScheduledPost
	if err := database.DB.Where("user_id = ? AND status = ?", uid, "pending").Order("scheduled_at asc").Find(&scheduledPosts).Error; err != nil {
		scheduledPosts = []models.ScheduledPost{}
	}

	// 2. History: posted + failed — most recent 20
	var history []models.ScheduledPost
	if err := database.DB.Preload("Analytics").Where("user_id = ? AND status IN ?", uid, []string{"posted", "failed"}).
		Order("scheduled_at desc").Limit(20).Find(&history).Error; err != nil {
		history = []models.ScheduledPost{}
	}

	// 3. Fetch Connected Social Accounts
	var accounts []models.SocialAccount
	database.DB.Where("user_id = ?", uid).Find(&accounts)

	// 4. Fetch Current Strategy
	var strategy models.UserStrategy
	database.DB.Where("user_id = ?", uid).Order("created_at desc").First(&strategy)

	// 5. Stats
	var postedCount int64
	database.DB.Model(&models.ScheduledPost{}).Where("user_id = ? AND status = ?", uid, "posted").Count(&postedCount)

	stats := map[string]interface{}{
		"totalPosts":        len(scheduledPosts),
		"postedCount":       postedCount,
		"activeExperiments": 3,
		"impactScore":       "92%",
	}

	// 6. Plan from user record
	var user models.User
	database.DB.First(&user, uid)
	plan := user.Plan
	if plan == "" {
		plan = "free"
	}

	c.JSON(http.StatusOK, DashboardData{
		Calendar:       scheduledPosts,
		History:        history,
		SocialAccounts: accounts,
		Strategy:       &strategy,
		Stats:          stats,
		Plan:           plan,
		Analytics:      buildAnalyticsOverview(uid),
	})
}
