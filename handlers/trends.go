package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

type TrendingTopic struct {
	Topic       string `json:"topic"`
	Engagement  string `json:"engagement"` // e.g. "High", "Explosive"
	Description string `json:"description"`
}

type TrendWrapper struct {
	Trends []TrendingTopic `json:"trends"`
	Topics []TrendingTopic `json:"topics"`
}

func GetTrendsHandler(c *gin.Context) {
	// 1. Fetch user strategy for context
	var strategy models.UserStrategy
	if err := database.DB.Order("created_at desc").First(&strategy).Error; err != nil {
		// Default to generic tech trends if no strategy found
		strategy.IdentityAudit = "A tech and AI enthusiast looking to grow their personal brand."
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)

	prompt := fmt.Sprintf(`
		You are a world-class social media trend analyst. 
		Based on this user's brand identity: "%s", 
		list 10 highly relevant, "viral", or "insightful" trending topics happening RIGHT NOW on X (Twitter).
		Return the response as a valid JSON object with a "trends" key containing an array of objects.
		Each object MUST have:
		- "topic": Short, catchy topic name.
		- "engagement": One of ["Explosive", "High", "Growing", "Insightful"].
		- "description": 1-2 sentences explaining why this is trending and the unique angle.
		
		Ensure the topics are specific and professional.
	`, strategy.IdentityAudit)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject},
		},
	)

	if err != nil {
		fmt.Printf("[TRENDS] API ERROR: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to listen for trends"})
		return
	}

	content := resp.Choices[0].Message.Content
	fmt.Printf("[TRENDS] RAW RESP: %s\n", content)

	var wrapper TrendWrapper
	result := []TrendingTopic{}

	if jsonErr := json.Unmarshal([]byte(content), &wrapper); jsonErr == nil {
		if len(wrapper.Trends) > 0 {
			result = wrapper.Trends
		} else if len(wrapper.Topics) > 0 {
			result = wrapper.Topics
		}
	} else {
		// Try unmarshaling directly as array
		if jsonErr2 := json.Unmarshal([]byte(content), &result); jsonErr2 != nil {
			fmt.Printf("[TRENDS] PARSE ERROR: %v\n", jsonErr2)
		}
	}

	if result == nil {
		result = []TrendingTopic{}
	}

	c.JSON(http.StatusOK, gin.H{"trends": result})
}

type ReactRequest struct {
	Topic string `json:"topic"`
}

func ReactToTrendHandler(c *gin.Context) {
	var req ReactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var strategy models.UserStrategy
	database.DB.Order("created_at desc").First(&strategy)

	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)

	prompt := fmt.Sprintf(`
		Trend: %s
		User Brand Context: %s
		
		Draft a viral, high-hook post reacting to this trend. 
		The tone should be professional yet provocative. 
		Focus on a unique angle that others are missing.
		Return the response as a simple text string.
	`, req.Topic, strategy.IdentityAudit)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"topic":    req.Topic,
		"reaction": resp.Choices[0].Message.Content,
	})
}

type ScheduleReactionRequest struct {
	Content  string `json:"content"`
	Platform string `json:"platform"` // x, linkedin, or both
}

func ScheduleReactionHandler(c *gin.Context) {
	var req ScheduleReactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 1. Fetch connected accounts for the requested platform
	var accounts []models.SocialAccount
	query := database.DB
	if req.Platform != "both" {
		query = query.Where("platform = ?", req.Platform)
	}
	query.Find(&accounts)

	if len(accounts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No connected accounts found for " + req.Platform})
		return
	}

	// 2. Create immediate scheduled posts (e.g. 5 minutes from now)
	scheduledTime := time.Now().Add(5 * time.Minute)

	for _, account := range accounts {
		post := models.ScheduledPost{
			AccountID:   account.ID,
			Platform:    account.Platform,
			Content:     req.Content,
			Day:         0, // 0 indicates a real-time reactive post
			ScheduledAt: scheduledTime,
			Status:      "pending",
			AIReasoning: "Reactive trend response generated by Furci AI.",
		}
		database.DB.Create(&post)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Reaction scheduled! It will fire in 5 minutes."})
}
