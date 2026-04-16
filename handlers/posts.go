package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type CreatePostRequest struct {
	Content     string   `json:"content"`
	Platforms   []string `json:"platforms"`   // ["twitter", "linkedin"]
	TimingMode  string   `json:"timingMode"`  // "now", "smart", "specific"
	ScheduledAt string   `json:"scheduledAt"` // ISO string if "specific"
}

func CreatePostHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Fetch Connected Accounts
	var accounts []models.SocialAccount
	database.DB.Where("platform IN ?", req.Platforms).Find(&accounts)
	if len(accounts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No connected accounts found for selected platforms"})
		return
	}

	// Determine Scheduled Time
	var scheduledTime time.Time
	switch req.TimingMode {
	case "now":
		scheduledTime = time.Now().Add(1 * time.Minute)
	case "specific":
		parsed, err := time.Parse(time.RFC3339, req.ScheduledAt)
		if err != nil {
			// fallback to simpler ISO
			parsed, err = time.Parse("2006-01-02T15:04", req.ScheduledAt)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid scheduledAt format"})
			return
		}
		scheduledTime = parsed
	case "smart":
		// Reuse logic: Find peak hour
		hour, _ := getPeakHour(accounts[0].ID)
		tomorrow := time.Now().AddDate(0, 0, 1)
		scheduledTime = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, 15, 0, 0, time.Local)
	}

	for _, acc := range accounts {
		post := models.ScheduledPost{
			AccountID:   acc.ID,
			Platform:    acc.Platform,
			Content:     req.Content,
			ContentType: "manual",
			ScheduledAt: scheduledTime,
			Status:      "pending",
			AIReasoning: "Manually created via Custom Post Creator.",
		}
		database.DB.Create(&post)
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Post(s) queued successfully"})
}

type AIPolishRequest struct {
	Content string `json:"content"`
	Style   string `json:"style"` // "professional", "provocative", "educational"
}

func AIPolishHandler(c *gin.Context) {
	var req AIPolishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key missing"})
		return
	}

	client := openai.NewClient(apiKey)
	prompt := fmt.Sprintf("Rewrite the following social media post to be more %s. Keep it under 280 characters if possible, and ensure it is engaging and punchy. Return ONLY the rewritten text.\n\nPost: %s", req.Style, req.Content)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI Polish failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"polishedContent": resp.Choices[0].Message.Content})
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
