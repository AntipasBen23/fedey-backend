package handlers

import (
	"fmt"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

type AuthCallbackRequest struct {
	AccessToken string `json:"accessToken"`
	Platform    string `json:"platform"`
	AccountType string `json:"accountType"` // "old" or "new"
}

func AuthCallbackHandler(c *gin.Context) {
	var req AuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid auth payload"})
		return
	}

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

	// Use Atomic Upsert (On Conflict) to handle potential race conditions from double-clicks
	account := models.SocialAccount{
		UserID:      uid,
		Platform:    req.Platform,
		AccessToken: req.AccessToken,
		AccountType: req.AccountType,
	}

	err := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "platform"}, {Name: "account_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"access_token", "updated_at", "deleted_at"}),
	}).Create(&account).Error

	if err != nil {
		fmt.Printf("[AUTH] Upsert Error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync account credentials"})
		return
	}

	fmt.Printf("[AUTH] Account synced successfully: Platform=%s, Type=%s\n", req.Platform, req.AccountType)

	c.JSON(http.StatusOK, gin.H{
		"message":  "Authentication successful. Furci now has access to your account (Persistent).",
		"platform": req.Platform,
	})
}
