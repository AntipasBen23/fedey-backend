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
	Plan           string                 `json:"plan"` // "free" | "pro"
}

func GetDashboardHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	// 1. Pending posts (upcoming queue)
	var scheduledPosts []models.ScheduledPost
	if err := database.DB.Where("status = ?", "pending").Order("scheduled_at asc").Find(&scheduledPosts).Error; err != nil {
		scheduledPosts = []models.ScheduledPost{}
	}

	// 2. History: posted + failed — most recent 20
	var history []models.ScheduledPost
	if err := database.DB.Where("status IN ?", []string{"posted", "failed"}).
		Order("scheduled_at desc").Limit(20).Find(&history).Error; err != nil {
		history = []models.ScheduledPost{}
	}

	// 3. Fetch Connected Social Accounts
	var accounts []models.SocialAccount
	database.DB.Find(&accounts)

	// 4. Fetch Current Strategy
	var strategy models.UserStrategy
	database.DB.Order("created_at desc").First(&strategy)

	// 5. Stats
	var postedCount int64
	database.DB.Model(&models.ScheduledPost{}).Where("status = ?", "posted").Count(&postedCount)

	stats := map[string]interface{}{
		"totalPosts":        len(scheduledPosts),
		"postedCount":       postedCount,
		"activeExperiments": 3,
		"impactScore":       "92%",
	}

	plan := "free"
	if IsPro() {
		plan = "pro"
	}

	c.JSON(http.StatusOK, DashboardData{
		Calendar:       scheduledPosts,
		History:        history,
		SocialAccounts: accounts,
		Strategy:       &strategy,
		Stats:          stats,
		Plan:           plan,
	})
}
