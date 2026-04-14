package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	Day          int      `json:"day"`
	Hook         string   `json:"hook"`
	Content      string   `json:"content"`
	Media        string   `json:"media"`
	ContentType  string   `json:"contentType"`  // tweet | thread | carousel | video_script | linkedin_post
	Script       string   `json:"script"`        // Full script body (threads and video scripts)
	Slides       []string `json:"slides"`        // Carousel slide copy (slide 1 = hook, last = CTA)
	Hashtags     []string `json:"hashtags"`
	VisualPrompt string   `json:"visualPrompt"` // DALL-E prompt for thumbnail/cover image
	CTAText      string   `json:"ctaText"`       // Call to action line
}

type CalendarResponse struct {
	Calendar []CalendarItem `json:"calendar"`
}

const calendarPromptTemplate = `You are Furci AI, a professional social media manager who writes like a human — not a robot.

Create a %d-day content calendar for the product below. Each day must use a DIFFERENT content type to create variety.

CONTENT TYPES you must rotate through:
- "tweet": A single punchy tweet (max 280 chars). No fluff. Direct value.
- "thread": A numbered Twitter/X thread (6-8 tweets). Tweet 1 = hook, tweets 2-7 = value, tweet 8 = CTA.
- "carousel": A LinkedIn/Instagram carousel (6-8 slides). Slide 1 = bold hook, slides 2-6 = one insight per slide, last slide = CTA + follow prompt.
- "video_script": A short-form video script (30-60 seconds). Write it scene-by-scene with timestamps.
- "linkedin_post": A long-form LinkedIn post (150-300 words). Hook line, story/insight, actionable takeaway.

For EVERY item, return these fields:
- day: Day number
- hook: The attention-grabbing first line (used as preview text)
- content: Full post body (for tweet/linkedin_post types — the complete text ready to publish)
- media: Visual direction (e.g., "Talking head, natural light", "Screen recording with face cam", "Minimal graphic, white bg", "Text on dark bg")
- contentType: One of the 5 types above
- script:
  * For "thread": All tweets written out, numbered 1/ through 8/, each tweet max 280 chars
  * For "video_script": Scene-by-scene with format "[0-3s] HOOK: ...", "[3-20s] PROBLEM: ...", "[20-50s] SOLUTION (step by step): ...", "[50-60s] CTA: ..."
  * For other types: leave empty string ""
- slides: For "carousel" only — array of 6-8 strings, each string is the full copy for that slide. For other types: empty array [].
- hashtags: 3-5 highly relevant hashtags (no generic ones like #motivation)
- visualPrompt: A DALL-E image generation prompt for the thumbnail or cover visual (be specific about style, colors, composition)
- ctaText: The explicit call-to-action (e.g., "Follow for daily tips", "Comment your biggest challenge", "DM me 'START' to learn more")

Format requirements: Return ONLY a valid JSON object. No markdown, no explanation. Schema:
{
  "calendar": [
    {
      "day": 1,
      "hook": "...",
      "content": "...",
      "media": "...",
      "contentType": "tweet|thread|carousel|video_script|linkedin_post",
      "script": "...",
      "slides": ["...", "..."],
      "hashtags": ["#tag1", "#tag2"],
      "visualPrompt": "...",
      "ctaText": "..."
    }
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

	model := openai.GPT4oMini
	if envModel := os.Getenv("OPENAI_MODEL"); envModel != "" {
		model = envModel
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: model,
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
	StartHour       int      `json:"startHour"`
	SaveAsDefault   bool     `json:"saveAsDefault"`
}

type SmartStaggerResult struct {
	Platform     string `json:"platform"`
	DelayMinutes int    `json:"delayMinutes"`
	Reason       string `json:"reason"`
}

func ApproveCalendarHandler(c *gin.Context) {
	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Approve] JSON Bind Error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload: " + err.Error()})
		return
	}

	if database.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	// 1. Fetch Draft Calendar
	var cal models.ContentCalendar
	if err := database.DB.Where("status = ?", "draft").Order("created_at desc").First(&cal).Error; err != nil {
		log.Printf("[Approve] Draft Fetch Error: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "No draft calendar found to approve"})
		return
	}

	var items []CalendarItem
	if err := json.Unmarshal([]byte(cal.ContentJSON), &items); err != nil {
		log.Printf("[Approve] JSON Unmarshal Error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse calendar content"})
		return
	}

	// 2. Fetch Connected Accounts
	var accounts []models.SocialAccount
	database.DB.Find(&accounts)
	if len(accounts) == 0 {
		log.Printf("[Approve] No accounts found in DB")
		c.JSON(http.StatusBadRequest, gin.H{"error": "No social accounts connected. Please connect X or LinkedIn first."})
		return
	}

	// 3. Update User Preferences if SaveAsDefault is requested
	if req.SaveAsDefault {
		database.DB.Model(&models.UserStrategy{}).Where("1=1").Updates(map[string]interface{}{
			"preferred_start_hour": req.StartHour,
			"preferred_stagger":    req.StaggerStrategy,
			"preferred_mode":       req.SchedulingMode,
		})
	}

	// 4. Process Scheduling for each day
	startHour := req.StartHour
	if startHour == 0 {
		// Fallback to strategy default if not passed
		var strategy models.UserStrategy
		database.DB.Order("created_at desc").First(&strategy)
		startHour = strategy.PreferredStartHour
	}
	if startHour == 0 {
		startHour = 9
	}

	for dayIndex, item := range items {
		// Calculate the correct day based on item.Day or dayIndex
		offset := item.Day
		if offset == 0 {
			offset = dayIndex + 1
		}

		// Base time for this day (starting from 'offset' days from now)
		baseTime := time.Now().AddDate(0, 0, offset)
		baseTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), startHour, 0, 0, 0, time.Local)

		for _, account := range accounts {
			scheduledTime := baseTime
			reasoning := ""

			// Apply Scheduling Logic
			if req.StaggerStrategy == "smart" {
				if account.Platform == "linkedin" {
					scheduledTime = scheduledTime.Add(2 * time.Hour) 
					reasoning = "LinkedIn optimized for mid-morning professional peak."
				} else {
					reasoning = "X optimized for maximal morning reach."
				}
			} else if req.StaggerStrategy == "fixed" && account.Platform == "linkedin" {
				scheduledTime = scheduledTime.Add(1 * time.Hour)
				reasoning = "Staggered for cross-platform distribution."
			} else {
				reasoning = "Scheduled via user precision mode."
			}

			// Build publish-ready content with hashtags appended
				publishContent := item.Content
				if publishContent == "" {
					publishContent = item.Hook
				} else {
					publishContent = fmt.Sprintf("%s\n\n%s", item.Hook, publishContent)
				}
				if len(item.Hashtags) > 0 {
					publishContent += "\n\n" + strings.Join(item.Hashtags, " ")
				}

				slidesJSON, _ := json.Marshal(item.Slides)

				post := models.ScheduledPost{
					AccountID:   account.ID,
					Platform:    account.Platform,
					Content:     publishContent,
					ContentType: item.ContentType,
					Script:      item.Script,
					SlidesJSON:  string(slidesJSON),
					Hashtags:    strings.Join(item.Hashtags, " "),
					CTAText:     item.CTAText,
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
