package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
)

// SendDailyReports scans users and sends their daily progress report
func SendDailyReports() {
	if database.DB == nil {
		return
	}

	log.Println("[Reporter] Starting daily progress report cycle...")

	var users []models.User
	database.DB.Find(&users)

	for _, user := range users {
		if user.Email == "" {
			continue
		}

		// 1. Gather Day's Data (Last 24h)
		yesterday := time.Now().Add(-24 * time.Hour)
		
		var postsToday []models.ScheduledPost
		database.DB.Where("user_id = ? AND status = ? AND updated_at >= ?", user.ID, "posted", yesterday).Find(&postsToday)

		var engagementsToday []models.EngagementEvent
		database.DB.Where("account_id IN (SELECT id FROM social_accounts WHERE user_id = ?) AND status = ? AND created_at >= ?", user.ID, "sent", yesterday).Find(&engagementsToday)

		var impressionsToday int64
		database.DB.Table("post_analytics").
			Joins("JOIN scheduled_posts ON scheduled_posts.id = post_analytics.scheduled_post_id").
			Where("scheduled_posts.user_id = ? AND post_analytics.sync_time >= ?", user.ID, yesterday).
			Select("SUM(post_analytics.impressions)").
			Row().Scan(&impressionsToday)

		// Skip if nothing happened today
		if len(postsToday) == 0 && len(engagementsToday) == 0 {
			log.Printf("[Reporter] Skipping user %s (no activity)", user.Email)
			continue
		}

		// 2. Generate AI Report
		reportContent, err := generateAIProgressReport(user, postsToday, engagementsToday, impressionsToday)
		if err != nil {
			log.Printf("[Reporter] AI Report generation failed for %s: %v", user.Email, err)
			continue
		}

		// 3. Send Email
		subject := fmt.Sprintf("📊 Furci AI: Your Daily Progress Report (%s)", time.Now().Format("Jan 02"))
		err = utils.SendEmail(user.Email, subject, reportContent)
		if err != nil {
			log.Printf("[Reporter] Failed to send email to %s: %v", user.Email, err)
			continue
		}

		log.Printf("[Reporter] Daily report sent to %s", user.Email)
	}
}

func generateAIProgressReport(user models.User, posts []models.ScheduledPost, engs []models.EngagementEvent, impressions int64) (string, error) {
	// Prepare context for AI
	type Stats struct {
		PostsSent     int    `json:"postsSent"`
		RepliesSent   int    `json:"repliesSent"`
		TotalReach    int64  `json:"totalReach"`
		TopPostSnippet string `json:"topPostSnippet"`
	}

	s := Stats{
		PostsSent:   len(posts),
		RepliesSent: len(engs),
		TotalReach:  impressions,
	}
	if len(posts) > 0 {
		s.TopPostSnippet = posts[0].Content
		if len(s.TopPostSnippet) > 100 {
			s.TopPostSnippet = s.TopPostSnippet[:100] + "..."
		}
	}

	statsJSON, _ := json.Marshal(s)

	prompt := fmt.Sprintf(`
		You are Furci AI, a highly professional autonomous social media growth operator.
		Generate a daily progress report for the user %s based on these stats from the last 24 hours:
		%s

		Include:
		1. A summary of work completed (Posts published and Ghost Operator autonomous replies).
		2. Reach and impact overview.
		3. A "Strategic Observation" (one insightful suggestion for tomorrow).
		4. A professional, motivating closing.

		Format the output as a beautiful, high-fidelity HTML email. Use a modern, dark-themed style with #093f67 and white text highlights. 
		Avoid generic templates. Make it feel like a personal briefing from a powerful AI assistant.
	`, user.Name, string(statsJSON))

	return utils.GenerateAIResponse(prompt)
}
