package handlers

import (
	"fmt"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
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

	// 1. Check if account already exists
	var existing models.SocialAccount
	err := database.DB.Where("platform = ? AND account_type = ?", req.Platform, req.AccountType).First(&existing).Error

	if err == nil {
		// Update existing
		existing.AccessToken = req.AccessToken
		if saveErr := database.DB.Save(&existing).Error; saveErr != nil {
			fmt.Printf("[AUTH] Failed to update existing account: %v\n", saveErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update account tokens"})
			return
		}
		fmt.Printf("[AUTH] Token UPDATED for Platform=%s\n", req.Platform)
	} else {
		// Create new
		newAccount := models.SocialAccount{
			Platform:    req.Platform,
			AccessToken: req.AccessToken,
			AccountType: req.AccountType,
		}
		if createErr := database.DB.Create(&newAccount).Error; createErr != nil {
			fmt.Printf("[AUTH] Failed to create new account: %v\n", createErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save new account info"})
			return
		}
		fmt.Printf("[AUTH] Token CREATED for Platform=%s\n", req.Platform)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Authentication successful. Furci now has access to your account (Persistent).",
		"platform": req.Platform,
	})
}
