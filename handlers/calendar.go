package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

type CalendarRequest struct {
	ProductSummary string `json:"productSummary"`
}

type CalendarItem struct {
	Day     int    `json:"day"`
	Hook    string `json:"hook"`
	Content string `json:"content"`
	Media   string `json:"media"`
}

type CalendarResponse struct {
	Calendar []CalendarItem `json:"calendar"`
}

const calendarPromptTemplate = `Create a professional %d-day social media content calendar for the following product. 

For each day, provide:
1. Hook: A catchy opening line.
2. Content: The main body or value proposition.
3. Media: Suggested media format (e.g., Image, Short Video, Thread, Infographic).

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "calendar": [
    { "day": 1, "hook": "...", "content": "...", "media": "..." },
    ...
  ]
}

Product Summary:
%s`

func CalendarHandler(c *gin.Context) {
	var req CalendarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key missing."})
		return
	}

	client := openai.NewClient(apiKey)
	
	// Trial Logic: Default to 3 days for now
	isPremium := false // Placeholder for future subscription check
	days := 3
	if isPremium {
		days = 14
	}

	prompt := fmt.Sprintf(calendarPromptTemplate, days, req.ProductSummary)

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate calendar: " + err.Error()})
		return
	}

	var calRes CalendarResponse
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &calRes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse calendar: " + err.Error()})
		return
	}

	// Persist to Database as Draft
	if database.DB != nil {
		contentJSON, _ := json.Marshal(calRes.Calendar)
		dbCal := models.ContentCalendar{
			ProductSummary: req.ProductSummary,
			ContentJSON:    string(contentJSON),
			Status:         "draft",
			DayCount:       days,
		}
		database.DB.Create(&dbCal)
	}
	c.JSON(http.StatusOK, calRes)
}

func ApproveCalendarHandler(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	// For now, approve the most recent draft
	var cal models.ContentCalendar
	result := database.DB.Where("status = ?", "draft").Order("created_at desc").First(&cal)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No draft calendar found to approve"})
		return
	}

	database.DB.Model(&cal).Update("status", "scheduled")

	c.JSON(http.StatusOK, gin.H{"message": "Calendar approved and scheduled!"})
}
