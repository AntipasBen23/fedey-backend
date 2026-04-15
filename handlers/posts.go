package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type UpdatePostRequest struct {
	Content       string `json:"content"`
	ScheduledAt   string `json:"scheduledAt"`   // ISO8601 or datetime-local
	VideoURL      string `json:"videoUrl"`       // Runway / Cloudinary video URL
	ImageURLsJSON string `json:"imageUrlsJson"`  // JSON array of image URLs (carousel slides)
}

func UpdatePostHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	var req UpdatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	var post models.ScheduledPost
	if err := database.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	if req.Content != "" {
		post.Content = req.Content
	}

	if req.ScheduledAt != "" {
		// Try RFC3339 (from toISOString()), then datetime-local fallbacks
		formats := []string{
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02T15:04:05",
			"2006-01-02T15:04",
		}
		var parsed time.Time
		var parseErr error
		for _, f := range formats {
			parsed, parseErr = time.Parse(f, req.ScheduledAt)
			if parseErr == nil {
				break
			}
		}
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduledAt format. Expected ISO8601."})
			return
		}
		post.ScheduledAt = parsed
	}

	if req.VideoURL != "" {
		post.VideoURL = req.VideoURL
	}
	if req.ImageURLsJSON != "" {
		post.ImageURLsJSON = req.ImageURLsJSON
	}

	if err := database.DB.Save(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post updated successfully", "post": post})
}

func DeletePostHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	// Verify it exists first so we return 404 rather than silently succeeding
	var post models.ScheduledPost
	if err := database.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	if err := database.DB.Delete(&models.ScheduledPost{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post removed from queue"})
}
