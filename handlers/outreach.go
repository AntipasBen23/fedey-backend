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

// AdminIncompleteUsersHandler returns all users who started onboarding but never finished.
func AdminIncompleteUsersHandler(c *gin.Context) {
	var users []models.User
	if err := database.DB.
		Where("is_verified = true").
		Where("last_onboarding_step != '' AND last_onboarding_step != 'completed'").
		Order("created_at desc").
		Find(&users).Error; err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to fetch incomplete users.")
		return
	}

	type row struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		StoppedAt string `json:"stoppedAt"`
		CreatedAt string `json:"createdAt"`
	}
	result := make([]row, len(users))
	for i, u := range users {
		result[i] = row{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			StoppedAt: u.LastOnboardingStep,
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04"),
		}
	}

	c.JSON(http.StatusOK, gin.H{"total": len(result), "users": result})
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
