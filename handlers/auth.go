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

	// Persist to Database
	if database.DB != nil {
		account := models.SocialAccount{
			Platform:    req.Platform,
			AccessToken: req.AccessToken,
			AccountType: req.AccountType,
		}

		// Upsert logic: if platform and accountType combo exists, update token. Else create.
		result := database.DB.Where(models.SocialAccount{Platform: req.Platform, AccountType: req.AccountType}).
			Assign(models.SocialAccount{AccessToken: req.AccessToken}).
			FirstOrCreate(&account)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save account info: " + result.Error.Error()})
			return
		}
		fmt.Printf("TOKEN PERSISTED: Platform=%s, Type=%s\n", req.Platform, req.AccountType)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Authentication successful. Furci now has access to your account (Persistent).",
		"platform": req.Platform,
	})
}
