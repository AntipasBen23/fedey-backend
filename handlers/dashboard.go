package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type DashboardData struct {
	Calendar       []CalendarItem         `json:"calendar"`
	SocialAccounts []models.SocialAccount `json:"socialAccounts"`
	Strategy       *models.UserStrategy   `json:"strategy"`
	Stats          map[string]interface{} `json:"stats"`
}

func GetDashboardHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	// 1. Fetch Scheduled Content
	var cal models.ContentCalendar
	result := database.DB.Where("status = ?", "scheduled").Order("created_at desc").First(&cal)
	
	var calendarItems []CalendarItem
	if result.Error == nil {
		json.Unmarshal([]byte(cal.ContentJSON), &calendarItems)
	}

	// 2. Fetch Connected Social Accounts
	var accounts []models.SocialAccount
	database.DB.Find(&accounts)

	// 3. Fetch Current Strategy
	var strategy models.UserStrategy
	database.DB.Order("created_at desc").First(&strategy)

	// 4. Prepare Mock Stats
	stats := map[string]interface{}{
		"totalPosts":    len(calendarItems),
		"activeExperiments": 3,
		"impactScore":   "92%",
	}

	c.JSON(http.StatusOK, DashboardData{
		Calendar:       calendarItems,
		SocialAccounts: accounts,
		Strategy:       &strategy,
		Stats:          stats,
	})
}
