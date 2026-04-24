package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func APIError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func APISuccess(c *gin.Context, data gin.H) {
	c.JSON(http.StatusOK, data)
}
