package handlers

import (
	"net/http"

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

	// TODO: Save this token and account type to your database associated with the user.
	// For now, we'll just acknowledge receipt.
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Authentication successful. Furci now has access to your account.",
		"platform": req.Platform,
	})
}
