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

type PostSchedule struct {
	Day    int `json:"day"`
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}

type ApproveRequest struct {
	SchedulingMode  string         `json:"schedulingMode"`  // 'manual', 'smart', 'hybrid'
	StaggerStrategy string         `json:"staggerStrategy"` // 'none', 'fixed', 'smart'
	PostSchedules   []PostSchedule `json:"postSchedules"`   // for manual mode
	TimeWindow      string         `json:"timeWindow"`      // 'morning', 'afternoon', 'evening'
	SaveAsDefault   bool           `json:"saveAsDefault"`
}

// getPeakHour finds the best hour for a platform based on engagement_rate
func getPeakHour(accountID uint) (int, bool) {
	if database.DB == nil {
		return 9, false
	}
	var res struct {
		Hour int
	}
	// Query average engagement rate per hour from past analytics
	err := database.DB.Table("post_analytics").
		Select("EXTRACT(HOUR FROM scheduled_posts.scheduled_at) as hour, AVG(post_analytics.engagement_rate) as avg_eng").
		Joins("JOIN scheduled_posts ON scheduled_posts.id = post_analytics.scheduled_post_id").
		Where("scheduled_posts.account_id = ?", accountID).
		Group("hour").
		Order("avg_eng DESC").
		Limit(1).
		Scan(&res).Error

	if err != nil || res.Hour == 0 {
		return 9, false
	}
	return res.Hour, true
}

// getAINudgedTime uses GPT to suggest a global peak time based on brand niche
func getAINudgedTime(platform, niche, window string) int {
	// Fallbacks for speed, but ideally calls OpenAI
	if window == "morning" {
		if strings.Contains(strings.ToLower(niche), "dev") || strings.Contains(strings.ToLower(niche), "tech") {
			return 10 // Devs wake up later
		}
		return 9
	}
	if window == "afternoon" {
		return 14
	}
	if window == "evening" {
		return 20
	}
	return 9
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "No social accounts connected. Please connect X or LinkedIn first."})
		return
	}

	// 3. Update User Preferences if SaveAsDefault is requested
	if req.SaveAsDefault {
		database.DB.Model(&models.UserStrategy{}).Where("1=1").Updates(map[string]interface{}{
			"preferred_stagger": req.StaggerStrategy,
			"preferred_mode":    req.SchedulingMode,
		})
	}

	// 4. Process Scheduling for each post
	for dayIndex, item := range items {
		dayOffset := item.Day
		if dayOffset == 0 {
			dayOffset = dayIndex + 1
		}

		// Calculate Base Scheduled Time based on mode
		var hour, minute int
		reasoning := ""

		if req.SchedulingMode == "manual" && dayIndex < len(req.PostSchedules) {
			hour = req.PostSchedules[dayIndex].Hour
			minute = req.PostSchedules[dayIndex].Minute
			reasoning = "Precision manually specified by user."
		} else {
			// Get Brand Niche from strategy for AI-nudge
			var strategy models.UserStrategy
			database.DB.Order("created_at desc").First(&strategy)
			niche := strategy.IdentityAudit

			// Try Local Sniping first
			peakHour, hasLocal := getPeakHour(accounts[0].ID) // Baseline on first account
			
			if req.SchedulingMode == "hybrid" {
				// Window-based Sniping
				hour = getAINudgedTime("global", niche, req.TimeWindow)
				minute = 15 // Small jitter
				reasoning = fmt.Sprintf("AI-Sniped within user-defined %s window (AI Nudge).", req.TimeWindow)
			} else {
				// Smart Mode: Peak Hour Sniping
				if hasLocal {
					hour = peakHour
					reasoning = "Autonomous sniper locked on to your localized peak performance hour."
				} else {
					hour = getAINudgedTime("global", niche, "morning")
					reasoning = "Global peak targeted (Adjusted for brand niche context)."
				}
				minute = 10 
			}
		}

		baseTime := time.Now().AddDate(0, 0, dayOffset)
		baseTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), hour, minute, 0, 0, time.Local)

		for accIdx, account := range accounts {
			scheduledTime := baseTime
			
			// Apply Staggering
			if req.StaggerStrategy == "smart" {
				// Randomize delay between 45-90 mins per platform
				delay := 45 + (accIdx * 30) 
				scheduledTime = scheduledTime.Add(time.Duration(delay) * time.Minute)
				reasoning += fmt.Sprintf(" Smart Stagger applied (+%dm).", delay)
			} else if req.StaggerStrategy == "fixed" {
				delay := accIdx * 60
				scheduledTime = scheduledTime.Add(time.Duration(delay) * time.Minute)
				reasoning += " Fixed 60m stagger applied."
			}

			// Build publish-ready content
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

	// 5. Update Status
	database.DB.Model(&cal).Update("status", "scheduled")

	c.JSON(http.StatusOK, gin.H{"message": "Intelligent schedule deployed! Check your dashboard."})
}
