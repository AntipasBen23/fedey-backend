package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

type RefineRequest struct {
	CurrentStrategy ProfessionalStrategy `json:"currentStrategy"`
	Feedback        string               `json:"feedback"`
	ProductSummary  string               `json:"productSummary"`
}

const refinePromptTemplate = `You are Furci AI. The user is dissatisfied with the social media strategy you generated.
Your task is to REHAPE and IMPROVE the strategy based strictly on their feedback.

USER GOAL:
%s

CURRENT STRATEGY:
- Identity Audit: %s
- Trend Monitoring: %v
- Growth Experiments: %v
- Analytics Reporting: %v

USER COMPLAINT/FEEDBACK:
"%s"

INSTRUCTIONS:
1. Maintain the Identity Audit from the current strategy (unless the feedback specifically asks to change it).
2. Completely rewrite the Trend Monitoring, Growth Experiments, and Analytics logic to specifically address the user's dissatisfaction.
3. Make the tone even more professional and actionable.

Format requirements: Return ONLY a valid JSON object matching the ProfessionalStrategy schema.
`

func StrategyRefineHandler(c *gin.Context) {
	var req RefineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key missing"})
		return
	}

	client := openai.NewClient(apiKey)
	prompt := fmt.Sprintf(refinePromptTemplate, 
		req.ProductSummary, 
		req.CurrentStrategy.IdentityAudit, 
		req.CurrentStrategy.TrendMonitoring, 
		req.CurrentStrategy.GrowthExperiments, 
		req.CurrentStrategy.AnalyticsReporting, 
		req.Feedback,
	)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refine strategy: " + err.Error()})
		return
	}

	var strategy ProfessionalStrategy
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &strategy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse refined strategy"})
		return
	}

	c.JSON(http.StatusOK, strategy)
}
