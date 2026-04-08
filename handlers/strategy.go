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

type StrategyRequest struct {
	ProductSummary string `json:"productSummary"`
}

type ProfessionalStrategy struct {
	TrendMonitoring   []string `json:"trendMonitoring"`
	GrowthExperiments []string `json:"growthExperiments"`
	AnalyticsReporting []string `json:"analyticsReporting"`
}

const strategyPromptTemplate = `Based on the following product summary, develop a professional social media growth strategy. 

Provide:
1. Trend Monitoring Tactics: How to monitor industry trends.
2. Growth Experiments: 3 specific hypotheses to test for rapid growth.
3. Analytics Reporting: Key metrics and reporting logic to stay on top of performance.

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "trendMonitoring": ["tactic1", "tactic2"],
  "growthExperiments": ["experiment1", "experiment2"],
  "analyticsReporting": ["metric1", "metric2"]
}

Product Summary:
%s`

func StrategyHandler(c *gin.Context) {
	var req StrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Furci couldn't access her brain (API Key missing)."})
		return
	}

	client := openai.NewClient(apiKey)
	prompt := fmt.Sprintf(strategyPromptTemplate, req.ProductSummary)

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate strategy: " + err.Error()})
		return
	}

	var strategy ProfessionalStrategy
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &strategy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse strategy response: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, strategy)
}
