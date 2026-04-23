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

// RunStrategicOptimization starts the recursive self-optimization loop
func RunStrategicOptimization() {
	if database.DB == nil {
		return
	}

	log.Println("[Strategist] Starting autonomous recursive optimization cycle...")

	var users []models.User
	database.DB.Find(&users)

	for _, user := range users {
		optimizeUserStrategy(user)
	}
}

func optimizeUserStrategy(user models.User) {
	// 1. Gather historical data (Last 14 days)
	twoWeeksAgo := time.Now().AddDate(0, 0, -14)
	
	type PostWithStats struct {
		ID             uint    `json:"id"`
		Content        string  `json:"content"`
		EngagementRate float64 `json:"engagementRate"`
		Likes          int     `json:"likes"`
		Impressions    int     `json:"impressions"`
	}

	var results []PostWithStats
	database.DB.Table("scheduled_posts").
		Select("scheduled_posts.id, scheduled_posts.content, post_analytics.engagement_rate, post_analytics.likes, post_analytics.impressions").
		Joins("JOIN post_analytics ON post_analytics.scheduled_post_id = scheduled_posts.id").
		Where("scheduled_posts.user_id = ? AND scheduled_posts.status = ? AND scheduled_posts.updated_at >= ?", user.ID, "posted", twoWeeksAgo).
		Order("post_analytics.engagement_rate DESC").
		Limit(10).
		Scan(&results)

	if len(results) < 3 {
		log.Printf("[Strategist] Not enough post data for user %d yet (need at least 3 analyzed posts)", user.ID)
		return
	}

	// 2. Prepare the "Learning Context"
	topPerformers := results
	if len(results) > 5 {
		topPerformers = results[:5]
	}

	dataJSON, _ := json.Marshal(topPerformers)

	// 3. Ask AI for the "Recursive Pivot"
	prompt := fmt.Sprintf(`
		You are the Furci AI Super-Strategist. Your goal is to analyze your own performance for user %s and pivot the strategy.
		
		Here are your top performing posts and their engagement rates:
		%s

		TASK:
		1. Identify the "Success DNA" (What tone, keywords, or topics are making people click?).
		2. Identify what to stop doing.
		3. Write a "Recursive Strategic Injection" (A 200-word instruction that I will inject into your future post generation engine).
		
		FORMAT: Return ONLY a JSON object with:
		{
			"successDna": "...",
			"strategicPivot": "...",
			"updatedIdentityFocus": "..."
		}
	`, user.Name, string(dataJSON))

	aiResp, err := utils.GenerateAIResponse(prompt)
	if err != nil {
		log.Printf("[Strategist] AI analysis failed for user %d: %v", user.ID, err)
		return
	}

	var insight struct {
		SuccessDna           string `json:"successDna"`
		StrategicPivot       string `json:"strategicPivot"`
		UpdatedIdentityFocus string `json:"updatedIdentityFocus"`
	}
	if err := json.Unmarshal([]byte(aiResp), &insight); err != nil {
		log.Printf("[Strategist] Failed to parse AI insight for user %d: %v", user.ID, err)
		return
	}

	// 4. Recursive Self-Update: Update the User's Strategy in the DB
	var strategy models.UserStrategy
	if err := database.DB.Where("user_id = ?", user.ID).Order("created_at desc").First(&strategy).Error; err == nil {
		// Update the existing strategy with the new "Learned Intelligence"
		newAudit := fmt.Sprintf("%s\n\n[AUTONOMOUS PIVOT %s]:\n%s", 
			strategy.IdentityAudit, 
			time.Now().Format("Jan 02"), 
			insight.UpdatedIdentityFocus)
		
		// Update DB
		database.DB.Model(&strategy).Updates(map[string]interface{}{
			"identity_audit": newAudit,
		})
		
		log.Printf("[Strategist] RECURSIVE UPDATE SUCCESS for %s: %s", user.Email, insight.StrategicPivot)
	}
}
