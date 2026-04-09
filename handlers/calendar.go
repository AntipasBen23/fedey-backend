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

type ApproveRequest struct {
	SchedulingMode  string   `json:"schedulingMode"`  // 'manual', 'smart', 'hybrid'
	StaggerStrategy string   `json:"staggerStrategy"` // 'none', 'fixed', 'smart'
	PreferredSlots  []string `json:"preferredSlots"`  // for manual mode
}

type SmartStaggerResult struct {
	Platform     string `json:"platform"`
	DelayMinutes int    `json:"delayMinutes"`
	Reason       string `json:"reason"`
}

func ApproveCalendarHandler(c *gin.Context) {
	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	// 1. Fetch Draft Calendar
	var cal models.ContentCalendar
	if err := database.DB.Where("status = ?", "draft").Order("created_at desc").First(&cal).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No draft calendar found"})
		return
	}

	var items []CalendarItem
	json.Unmarshal([]byte(cal.ContentJSON), &items)

	// 2. Fetch Connected Accounts
	var accounts []models.SocialAccount
	database.DB.Find(&accounts)
	if len(accounts) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No social accounts connected"})
		return
	}

	// 3. Process Scheduling for each day
	for dayIndex, item := range items {
		// Base time for this day (starting tomorrow at 9 AM)
		baseTime := time.Now().AddDate(0, 0, dayIndex+1)
		baseTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), 9, 0, 0, 0, time.Local)

		for _, account := range accounts {
			scheduledTime := baseTime
			reasoning := ""

			// Apply Smart Stagger Logic if selected
			if req.StaggerStrategy == "smart" {
				// AI sequencing logic simulated for stability
				if account.Platform == "linkedin" {
					scheduledTime = scheduledTime.Add(2 * time.Hour) // LinkedIn delayed for professional peak
					reasoning = "LinkedIn scheduled for mid-morning professional peak (AI Optimized)."
				} else {
					reasoning = "X scheduled for immediate morning hook impact (AI Optimized)."
				}
			} else if req.StaggerStrategy == "fixed" && account.Platform == "linkedin" {
				scheduledTime = scheduledTime.Add(1 * time.Hour)
				reasoning = "Staggered 60 mins after X."
			}

			post := models.ScheduledPost{
				AccountID:   account.ID,
				Platform:    account.Platform,
				Content:     fmt.Sprintf("%s\n\n%s", item.Hook, item.Content),
				Day:         item.Day,
				ScheduledAt: scheduledTime,
				AIReasoning: reasoning,
				Status:      "pending",
			}
			database.DB.Create(&post)
		}
	}

	// 4. Update Status
	database.DB.Model(&cal).Update("status", "scheduled")

	c.JSON(http.StatusOK, gin.H{"message": "Intelligent schedule deployed! Check your dashboard."})
}
