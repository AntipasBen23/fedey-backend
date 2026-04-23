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

// StartScheduler initializes the background worker that polls for due posts
func StartScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	listeningTicker := time.NewTicker(15 * time.Minute) // Social listening every 15 mins
	intelTicker := time.NewTicker(4 * time.Hour)       // Intelligence Hub every 4 hours
	synergyTicker := time.NewTicker(12 * time.Hour)     // Synergy Matrix every 12 hours

	go func() {
		lastReportDate := ""
		for {
			select {
			case <-ticker.C:
				CheckForDuePosts()
				
				// Daily Progress Report check
				now := time.Now()
				today := now.Format("2006-01-02")
				
				// 1:00 AM -> Strategic Optimization
				if now.Hour() == 1 && now.Minute() == 0 && lastReportDate != today {
					RunStrategicOptimization()
				}

				// 23:30 -> Daily Progress Report
				if now.Hour() == 23 && now.Minute() == 30 && lastReportDate != today {
					SendDailyReports()
					lastReportDate = today
				}
			case <-intelTicker.C:
				RunIntelligenceHub()
			case <-synergyTicker.C:
				RunSynergyMatrix()
			case <-listeningTicker.C:
				RunSocialListening()
			}
		}
	}()
}

func RunSocialListening() {
	if database.DB == nil {
		return
	}

	log.Println("[GhostOperator] Starting autonomous social listening cycle...")

	var accounts []models.SocialAccount
	database.DB.Where("platform IN ?", []string{"twitter", "x"}).Find(&accounts)

	for _, acc := range accounts {
		// 1. Get Niche Context
		var strategy models.UserStrategy
		database.DB.Order("created_at desc").First(&strategy)
		niche := strategy.IdentityAudit
		if niche == "" {
			niche = "Tech and SaaS Growth" // Fallback
		}

		// 2. Scan for Mentions
		userCtx, err := utils.GetXUserContext(acc.AccessToken)
		if err != nil {
			log.Printf("[GhostOperator] Failed to get user context for account %d: %v", acc.ID, err)
			continue
		}
		userID := fmt.Sprintf("%v", userCtx["id"])
		handle := fmt.Sprintf("%v", userCtx["username"])

		mentions, _ := utils.GetXMentions(acc.AccessToken, userID)
		for _, m := range mentions {
			processIncomingEnagement(acc, m, "mention", niche, handle)
		}

		// 3. Niche Discovery (Proactive networking)
		keywords := []string{"SaaS", "Growth", "Solopreneur"} // Ideally parsed from strategy
		if len(keywords) > 0 {
			query := keywords[0]
			nichePosts, _ := utils.SearchXNiche(acc.AccessToken, query)
			for _, p := range nichePosts {
				processIncomingEnagement(acc, p, "niche_discovery", niche, "")
			}
		}
	}
}

func processIncomingEnagement(acc models.SocialAccount, raw map[string]interface{}, engType, niche, myHandle string) {
	postID := fmt.Sprintf("%v", raw["id"])
	authorID := fmt.Sprintf("%v", raw["author_id"])
	content := fmt.Sprintf("%v", raw["text"])

	// Don't reply to self
	if myHandle != "" && strings.Contains(content, "@"+myHandle) && authorID == myHandle {
		return 
	}

	// Check if already processed
	var existing models.EngagementEvent
	if err := database.DB.Where("external_post_id = ?", postID).First(&existing).Error; err == nil {
		return // Already handled
	}

	log.Printf("[GhostOperator] New %s found: %s", engType, postID)

	// Draft AI Reply
	reply, err := utils.DraftEngagementReply(content, authorID, niche)
	if err != nil {
		log.Printf("[GhostOperator] AI Drafting failed: %v", err)
		return
	}

	event := models.EngagementEvent{
		AccountID:       acc.ID,
		Platform:        acc.Platform,
		Type:            engType,
		ExternalPostID:  postID,
		AuthorHandle:    authorID,
		OriginalContent: content,
		ProposedReply:   reply,
		Status:          "pending",
	}

	// If Ghost Mode is ON, execute immediately
	if acc.GhostModeEnabled {
		_, success, _ := utils.PostSingleTweetToX(acc.AccessToken, reply) // Ideally use reply-to logic
		if success {
			event.Status = "sent"
			log.Printf("[GhostOperator] AUTO-REPLIED to %s", postID)
		}
	}

	database.DB.Create(&event)
}

func CheckForDuePosts() {
	if database.DB == nil {
		return
	}

	var duePosts []models.ScheduledPost
	now := time.Now()

	err := database.DB.Where("status = ? AND scheduled_at <= ?", "pending", now).Find(&duePosts).Error
	if err != nil {
		log.Printf("[Worker] Error fetching due posts: %v", err)
		return
	}

	if len(duePosts) == 0 {
		return
	}

	log.Printf("[Worker] Found %d posts due for publication", len(duePosts))

	for _, post := range duePosts {
		executePost(post)
	}
}

func failPost(post models.ScheduledPost, reason string) {
	log.Printf("[Worker] FAILED — post %d: %s", post.ID, reason)
	database.DB.Model(&post).Updates(map[string]interface{}{
		"status":         "failed",
		"failure_reason": reason,
	})
}

func executePost(post models.ScheduledPost) {
	var account models.SocialAccount
	if err := database.DB.First(&account, post.AccountID).Error; err != nil {
		failPost(post, fmt.Sprintf("account not found (accountId=%d): %v", post.AccountID, err))
		return
	}

	if account.AccessToken == "" {
		failPost(post, fmt.Sprintf("access token is empty for account %d — re-connect %s", account.ID, post.Platform))
		return
	}

	var externalID string
	var success bool
	var err error

	switch post.Platform {
	case "twitter", "x":
		externalID, success, err = utils.PostToX(account.AccessToken, post)
	case "linkedin":
		personURN, errURN := utils.GetLinkedInPersonURN(account.AccessToken)
		if errURN != nil {
			failPost(post, errURN.Error())
			return
		}
		externalID, success, err = utils.PostLinkedInText(account.AccessToken, personURN, post.Content)
	default:
		failPost(post, fmt.Sprintf("unknown platform: %s", post.Platform))
		return
	}

	if success {
		log.Printf("[Worker] SUCCESS — post %d published to %s", post.ID, post.Platform)
		database.DB.Model(&post).Updates(map[string]interface{}{
			"status":         "posted",
			"external_id":    externalID,
			"failure_reason": "",
		})
	} else {
		failPost(post, fmt.Sprintf("%v", err))
	}
}

func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
