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

type ReviseRequest struct {
	CurrentCalendar []CalendarItem `json:"currentCalendar"`
	Feedback        string         `json:"feedback"`
}

const revisePromptTemplate = `The user has the following content calendar but has feedback on it.

Feedback from user: "%s"

Revise the calendar to address their feedback while keeping the same content types and structure.
Keep scripts, slides, hashtags, and all rich content fields intact — just improve the quality based on the feedback.

Current Calendar:
%s

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "calendar": [
    {
      "day": 1,
      "hook": "...",
      "content": "...",
      "media": "...",
      "contentType": "...",
      "script": "...",
      "slides": ["..."],
      "hashtags": ["#tag"],
      "visualPrompt": "...",
      "ctaText": "..."
    }
  ]
}`

func ReviseHandler(c *gin.Context) {
	var req ReviseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key missing."})
		return
	}

	currentCalJSON, _ := json.Marshal(req.CurrentCalendar)
	client := openai.NewClient(apiKey)
	prompt := fmt.Sprintf(revisePromptTemplate, req.Feedback, string(currentCalJSON))

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revise calendar: " + err.Error()})
		return
	}

	var calRes CalendarResponse
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &calRes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse revised calendar: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, calRes)
}
