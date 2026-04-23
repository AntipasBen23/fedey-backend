package worker

import (
	"fmt"
	"log"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
)

// RunDigitalFortress scans competitors and engineers counter-content
func RunDigitalFortress() {
	if database.DB == nil {
		return
	}

	log.Println("[DigitalFortress] Scanning competitors for strategic counter-play opportunities...")

	var users []models.User
	database.DB.Find(&users)

	for _, user := range users {
		var rivals []models.CompetitorProfile
		database.DB.Where("user_id = ?", user.ID).Find(&rivals)

		// If no rivals defined, define some based on niche if possible,
		// but for now we wait for user to define them.
		for _, rival := range rivals {
			analyzeRival(user, rival)
		}
	}
}

func analyzeRival(user models.User, rival models.CompetitorProfile) {
	// 1. Get User's account to search
	var account models.SocialAccount
	if database.DB.Where("user_id = ? AND platform = ?", user.ID, rival.Platform).First(&account).Error != nil {
		return
	}

	// 2. Search for Rival's latest hits
	query := fmt.Sprintf("from:%s", rival.ExternalHandle)
	results, err := utils.SearchXNiche(account.AccessToken, query)
	if err != nil || len(results) == 0 {
		return
	}

	topPost := fmt.Sprintf("%v", results[0]["text"])
	postID := fmt.Sprintf("%v", results[0]["id"])

	if postID == rival.LastViralPostID {
		return // Already analyzed
	}

	// 3. Engineer Counter-Angle
	var strategy models.UserStrategy
	database.DB.Where("user_id = ?", user.ID).Order("created_at desc").First(&strategy)

	counterContent, err := engineerCounterTake(topPost, strategy.IdentityAudit)
	if err != nil {
		return
	}

	// 4. Draft the "Counter-Play"
	newPost := models.ScheduledPost{
		UserID:      user.ID,
		AccountID:   account.ID,
		Platform:    account.Platform,
		Content:     counterContent,
		Status:      "pending",
		ScheduledAt: time.Now().Add(2 * time.Hour),
		FailureReason: fmt.Sprintf("COMPETITOR COUNTER: @%s", rival.ExternalHandle),
	}
	database.DB.Create(&newPost)

	// 5. Update Rival Memory
	database.DB.Model(&rival).Updates(map[string]interface{}{
		"last_viral_post_id": postID,
		"insights":         fmt.Sprintf("Analyzed viral post about: %s", topPost[:30]),
	})

	log.Printf("[DigitalFortress] COUNTER-PLAY drafted for user %d against @%s", user.ID, rival.ExternalHandle)
}

func engineerCounterTake(rivalContent, myNiche string) (string, error) {
	prompt := fmt.Sprintf(`
		You are a Strategic Ghost Operator in the "%s" niche.
		Your competitor just posted this and it's performing well:
		"%s"

		TASK:
		Engineering a "Masterclass" level counter-take or an "Advanced Nuance" post (under 280 chars).
		Do not be toxic. Instead, be intellectually superior. 
		Provide the "Missing Piece" or the "Alternative Future" that the competitor missed.
		
		Return ONLY the post content.
	`, myNiche, rivalContent)

	return utils.GenerateAIResponse(prompt)
}
