package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

type ChatRequest struct {
	Message string `json:"message"`
}

func ChatWithFurciHandler(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 1. Gather Dashboard Context for the AI
	
	// A. Pending Posts (Upcoming Queue)
	var pendingPosts []models.ScheduledPost
	database.DB.Where("status = ?", "pending").Order("scheduled_at asc").Limit(10).Find(&pendingPosts)
	postContext := ""
	for _, p := range pendingPosts {
		postContext += fmt.Sprintf("- [%s at %s]: %s\n", p.Platform, p.ScheduledAt.Format("Jan 02, 15:04"), p.Content[:60]+"...")
	}

	// B. Current Strategy
	var strategy models.UserStrategy
	database.DB.Order("created_at desc").First(&strategy)

	// C. Analytics Summary
	analytics := buildAnalyticsOverview()
	analyticsContext := fmt.Sprintf(`
		- Total Followers: %d (%+d in last 30d)
		- Total Impressions: %d
		- Avg Engagement Rate: %.2f%%
		- Posts Analyzed: %d
		- Top Post: %s (%.2f%% eng)
	`, analytics.FollowerCount, analytics.FollowerGrowth, analytics.TotalImpressions, analytics.AvgEngRate, analytics.PostsAnalyzed, 
	   func() string { if len(analytics.TopPosts) > 0 { return analytics.TopPosts[0].Snippet }; return "None yet" }(),
	   func() float64 { if len(analytics.TopPosts) > 0 { return analytics.TopPosts[0].EngRate }; return 0.0 }())

	systemPrompt := fmt.Sprintf(`
		You are Furci, the intelligent Social Growth COO and "Brain" of this Operating System.
		Your goal is to help the user manage their brand, understand their dashboard metrics, and scale their presence autonomously.

		DASHBOARD INTELLIGENCE (REAL-TIME DATA):
		- Brand Identity: %s
		- Growth Strategy: %s
		- Analytics Overview: %s
		- Upcoming Queue (%d pending total):
		%s

		INSTRUCTIONS:
		- Speak with authority as the OS component responsible for growth.
		- Always reference specific data from the "DASHBOARD INTELLIGENCE" (e.g., follower counts, engagement stats, or specific upcoming posts).
		- If the user asks "how is work going", provide a professional summary of their recent growth and the next key posts in the queue.
		- Keep replies professional, slightly provocative/ambitious, and concise (under 4 sentences).
		- You are not just a chatbot; you are the executive controller of this brand.
	`, strategy.IdentityAudit, strategy.GrowthExperiments, analyticsContext, len(pendingPosts), postContext)

	// 2. Call OpenAI
	apiKey := os.Getenv("OPENAI_API_KEY")
	client := openai.NewClient(apiKey)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: req.Message},
			},
		},
	)

	if err != nil {
		fmt.Printf("[CHAT] API ERROR: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Furci's neural link is unstable. Try again."})
		return
	}

	reply := resp.Choices[0].Message.Content
	c.JSON(http.StatusOK, gin.H{"reply": strings.TrimSpace(reply)})
}
