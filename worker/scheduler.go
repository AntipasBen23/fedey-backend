package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
)

// StartScheduler initializes the background worker that polls for due posts
func StartScheduler() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			CheckForDuePosts()
		}
	}()
}

func CheckForDuePosts() {
	if database.DB == nil {
		return
	}

	var duePosts []models.ScheduledPost
	now := time.Now()

	// Find pending posts that are due
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

func executePost(post models.ScheduledPost) {
	// 1. Fetch Account and Token
	var account models.SocialAccount
	if err := database.DB.First(&account, post.AccountID).Error; err != nil {
		log.Printf("[Worker] Error finding account for post %d: %v", post.ID, err)
		database.DB.Model(&post).Update("status", "failed")
		return
	}

	log.Printf("[Worker] Publishing to %s: %s", post.Platform, post.Content)

	// 2. Perform API Call (Stubbed for real Twitter/LinkedIn logic)
	success := true
	var err error

	if post.Platform == "twitter" {
		success, err = postToTwitter(account.AccessToken, post.Content)
	} else if post.Platform == "linkedin" {
		success, err = postToLinkedIn(account.AccessToken, post.Content)
	}

	// 3. Update Status
	if success {
		log.Printf("[Worker] Successfully posted %d to %s", post.ID, post.Platform)
		database.DB.Model(&post).Update("status", "posted")
	} else {
		log.Printf("[Worker] Failed to post %d to %s: %v", post.ID, post.Platform, err)
		database.DB.Model(&post).Update("status", "failed")
	}
}

// REAL LOGIC STUBS (To be fully implemented with official SDKs)

func postToTwitter(token, text string) (bool, error) {
	// In a real production environment, this would hit https://api.twitter.com/2/tweets
	// For now, we simulate success
	log.Printf("[Worker] [STUB] Hitting Twitter API with token %s...", token[:10]+"***")
	return true, nil
}

func postToLinkedIn(token, text string) (bool, error) {
	// In a real production environment, this would hit the LinkedIn Share API
	log.Printf("[Worker] [STUB] Hitting LinkedIn API with token %s...", token[:10]+"***")
	return true, nil
}
