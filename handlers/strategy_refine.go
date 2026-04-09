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
	RefineMode      string               `json:"refineMode"` // "merge" or "replace"
}

const refinePromptTemplate = `You are Furci AI. The user is dissatisfied with the social media strategy you generated.
Your task is to REHAPE and IMPROVE the strategy based strictly on their feedback and their chosen Refine Mode ("%s").

USER GOAL:
%s

CURRENT STRATEGY (To be potentially merged or replaced):
- Identity Audit: %s
- Trend Monitoring: %v
- Growth Experiments: %v
- Analytics Reporting: %v

USER COMPLAINT/FEEDBACK:
"%s"

INSTRUCTIONS FOR MODE "%s":
- If mode is "replace": Discard all current Trends, Experiments, and Analytics logic. Create a 100%% fresh strategy based ONLY on the User Goal and the Feedback.
- If mode is "merge": Keep the winning ideas from the Current Strategy but mix in the new Feedback to create a blended, improved version.
- In both modes, maintain the original Identity Audit.

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
		req.RefineMode,
		req.ProductSummary, 
		req.CurrentStrategy.IdentityAudit, 
		req.CurrentStrategy.TrendMonitoring, 
		req.CurrentStrategy.GrowthExperiments, 
		req.CurrentStrategy.AnalyticsReporting, 
		req.Feedback,
		req.RefineMode,
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
