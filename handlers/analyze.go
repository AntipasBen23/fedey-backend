package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

type AnalyzeRequest struct {
	JobDescription string `json:"jobDescription"`
}

type StrategyReport struct {
	Summary     string   `json:"summary"`
	Audience    []string `json:"audience"`
	Competitors []string `json:"competitors"`
}

const promptTemplate = `Analyze the following job description and provide a strategic breakdown for a social media manager. Extract the core product/service summary, the primary target audiences, and the inferred or stated competitors.

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "summary": "1-2 sentence summary",
  "audience": ["audience1", "audience2"],
  "competitors": ["comp1", "comp2"]
}

Job Description:
%s`

func AnalyzeHandler(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.APIError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request payload.")
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "API key missing.")
		return
	}

	client := openai.NewClient(apiKey)
	prompt := fmt.Sprintf(promptTemplate, req.JobDescription)

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
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to call OpenAI API.")
		return
	}

	var report StrategyReport
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &report)
	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to parse OpenAI response.")
		return
	}

	c.JSON(http.StatusOK, report)
}
