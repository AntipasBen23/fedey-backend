package handlers

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
)

type RepurposeRequest struct {
	PostID     uint   `json:"postId" binding:"required"`
	TargetPlatform string `json:"targetPlatform" binding:"required"`
}

func RepurposePostHandler(c *gin.Context) {
	var req RepurposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "MISSING_FIELDS", "postId and targetPlatform are required.")
		return
	}

	var post models.ScheduledPost
	if err := database.DB.First(&post, req.PostID).Error; err != nil {
		utils.APIError(c, http.StatusNotFound, "NOT_FOUND", "Post not found.")
		return
	}

	// Get Niche for context
	var strategy models.UserStrategy
	database.DB.Where("user_id = ?", post.UserID).Order("created_at desc").First(&strategy)

	refactored, err := utils.RepurposeContent(post.Content, post.Platform, req.TargetPlatform, strategy.IdentityAudit)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Refactoring failed.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"original":   post.Content,
		"refactored": refactored,
		"platform":   req.TargetPlatform,
	})
}
