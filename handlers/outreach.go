package handlers

import (
	"fmt"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/AntipasBen23/fedey-backend/worker"
	"github.com/gin-gonic/gin"
)

// AdminOutreachLogsHandler returns all onboarding re-engagement emails sent,
// newest first, with optional limit via ?limit=N query param.
func AdminOutreachLogsHandler(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if n, err := parseInt(l); err == nil && n > 0 {
			limit = n
		}
	}

	var logs []models.OnboardingOutreach
	if err := database.DB.Order("sent_at desc").Limit(limit).Find(&logs).Error; err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to fetch outreach logs.")
		return
	}

	var total int64
	database.DB.Model(&models.OnboardingOutreach{}).Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"total": total,
		"logs":  logs,
	})
}

// AdminTriggerOutreachHandler lets the admin manually fire the outreach scan.
func AdminTriggerOutreachHandler(c *gin.Context) {
	go worker.RunOnboardingOutreach()
	c.JSON(http.StatusOK, gin.H{"message": "Onboarding outreach scan triggered."})
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
