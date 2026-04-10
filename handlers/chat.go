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
	
	// A. Pending Posts
	var pendingPosts []models.ScheduledPost
	database.DB.Where("status = ?", "pending").Order("scheduled_at asc").Limit(5).Find(&pendingPosts)
	postContext := ""
	for _, p := range pendingPosts {
		postContext += fmt.Sprintf("- [%s at %s]: %s\n", p.Platform, p.ScheduledAt.Format("Jan 02, 15:04"), p.Content[:50]+"...")
	}

	// B. Current Strategy
	var strategy models.UserStrategy
	database.DB.Order("created_at desc").First(&strategy)

	// C. Trends (Simplified context)
	// We'll skip deep trend context to keep prompt small, but mention we have them.
	
	systemPrompt := fmt.Sprintf(`
		You are Furci, the intelligent Social Growth COO of this operating system.
		Your goal is to help the user manage their brand, understand their dashboard, and grow their presence.

		CURRENT SITUATION:
		- Brand Identity: %s
		- Strategy Focus: %s
		- Upcoming Posts in Queue (%d pending total):
		%s

		INSTRUCTIONS:
		- Be professional, concise, and insightful.
		- Always reference specific data from the "CURRENT SITUATION" if relevant to the question.
		- If asked how "work is going", summarize the upcoming posts and the overall strategy health.
		- Keep replies under 3 sentences unless a deep strategy explanation is requested.
		- You are an OS component, not just a chatbot. Speak like the brain of the platform.
	`, strategy.IdentityAudit, strategy.GrowthExperiments, len(pendingPosts), postContext)

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
