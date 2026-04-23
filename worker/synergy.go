package worker

import (
	"log"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
)

// RunSynergyMatrix autonomously repurposes high-performing content across platforms
func RunSynergyMatrix() {
	if database.DB == nil {
		return
	}

	log.Println("[SynergyMatrix] Scanning for successful content to spread across platforms...")

	// 1. Find "Success" posts from the last 24h
	yesterday := time.Now().Add(-24 * time.Hour)
	
	var winners []models.ScheduledPost
	database.DB.Table("scheduled_posts").
		Joins("JOIN post_analytics ON post_analytics.scheduled_post_id = scheduled_posts.id").
		Where("scheduled_posts.status = ? AND scheduled_posts.updated_at >= ? AND post_analytics.engagement_rate > ?", "posted", yesterday, 2.0). // Threshold 2%
		Find(&winners)

	for _, win := range winners {
		spreadTheWin(win)
	}
}

func spreadTheWin(post models.ScheduledPost) {
	// Find other accounts for this user
	var otherAccs []models.SocialAccount
	database.DB.Where("user_id = ? AND id != ?", post.UserID, post.AccountID).Find(&otherAccs)

	var strategy models.UserStrategy
	database.DB.Where("user_id = ?", post.UserID).Order("created_at desc").First(&strategy)

	for _, acc := range otherAccs {
		// Dedup: skip if we already created a synergy post from this exact source post for this account
		var existing int64
		database.DB.Model(&models.ScheduledPost{}).
			Where("account_id = ? AND synergy_source_id = ?", acc.ID, post.ID).
			Count(&existing)
		if existing > 0 {
			continue
		}

		refactored, err := utils.RepurposeContent(post.Content, post.Platform, acc.Platform, strategy.IdentityAudit)
		if err != nil {
			continue
		}

		sourceID := post.ID
		newPost := models.ScheduledPost{
			UserID:          post.UserID,
			AccountID:       acc.ID,
			Platform:        acc.Platform,
			Content:         refactored,
			Status:          "pending",
			ScheduledAt:     time.Now().Add(6 * time.Hour),
			Source:          "synergy",
			SynergySourceID: &sourceID,
		}
		database.DB.Create(&newPost)
		log.Printf("[SynergyMatrix] SYNERGIZED winner %d to platform %s for user %d", post.ID, acc.Platform, post.UserID)
	}
}
