package worker

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
)

// RunIntelligenceHub scans for external news/trends and drafts high-value content
func RunIntelligenceHub() {
	if database.DB == nil {
		return
	}

	log.Println("[IntelligenceHub] Scanning for emerging news and viral opportunities...")

	var accounts []models.SocialAccount
	database.DB.Where("platform IN ?", []string{"twitter", "x"}).Find(&accounts)

	for _, acc := range accounts {
		// 1. Get User's Strategic Keywords
		var strategy models.UserStrategy
		database.DB.Where("user_id = ?", acc.UserID).Order("created_at desc").First(&strategy)
		
		niche := strategy.IdentityAudit
		if niche == "" {
			niche = "SaaS and AI Growth"
		}

		// 2. Scan Niche for High-Signal Events
		// We use a few generic high-signal keywords combined with user niche
		query := "breaking AI"
		if strings.Contains(strings.ToLower(niche), "crypto") {
			query = "crypto news"
		} else if strings.Contains(strings.ToLower(niche), "saas") {
			query = "saas growth"
		}

		news, err := utils.SearchXNiche(acc.AccessToken, query)
		if err != nil || len(news) == 0 {
			continue
		}

		// 3. Select the most relevant piece of news (top 1)
		topNews := fmt.Sprintf("%v", news[0]["text"])

		// 4. Draft a "Super Intelligent" take
		draftTake, err := engineerStrategicTake(topNews, niche)
		if err != nil {
			continue
		}

		// 5. Save as a "PROACTIVE DRAFT"
		post := models.ScheduledPost{
			UserID:      acc.UserID,
			AccountID:   acc.ID,
			Platform:    acc.Platform,
			Content:     draftTake,
			Status:      "pending",
			ScheduledAt: time.Now().Add(4 * time.Hour),
			Source:      "intelligence",
		}
		
		// Check if we already drafted something for this news today to avoid spam
		var existing models.ScheduledPost
		if database.DB.Where("user_id = ? AND content LIKE ?", acc.UserID, "%"+draftTake[:20]+"%").First(&existing).Error != nil {
			database.DB.Create(&post)
			log.Printf("[IntelligenceHub] DRAFTED news-driven post for user %d: %s", acc.UserID, draftTake[:50])
		}
	}
}

func engineerStrategicTake(news, niche string) (string, error) {
	prompt := fmt.Sprintf(`
		You are a world-class strategic content engineer for a high-authority brand in the "%s" niche.
		
		NEWS EVENT: "%s"

		TASK:
		Engineer a provocative, high-authority social media post (under 280 chars) that provides a unique "Angle" on this news.
		DO NOT just summarize it. 
		Choose one of these angles:
		1. CONTRA-TREND: Why everyone is wrong about this news.
		2. FUTURIST: What this news means for the industry in 2 years.
		3. ACTIONABLE: The one thing people should do right now because of this.

		Return ONLY the final post content.
	`, niche, news)

	return utils.GenerateAIResponse(prompt)
}
