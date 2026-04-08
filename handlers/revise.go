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

const revisePromptTemplate = `The user has the following content calendar but is unhappy with it. 

Feedback from user: "%s"

Please revise the entire 14-day calendar to strictly address their concerns while maintaining a professional and strategic tone.

Current Calendar:
%s

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "calendar": [
    { "day": 1, "hook": "...", "content": "...", "media": "..." },
    ...
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
