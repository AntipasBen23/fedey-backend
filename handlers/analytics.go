package handlers

import (
	"math/rand"
	"net/http"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
)

type AnalyticsSummary struct {
	TotalReach     int     `json:"totalReach"`
	AvgImpact      int     `json:"avgImpact"`
	EngagementRate float64 `json:"engagementRate"`
	BestPerformers []any   `json:"bestPerformers"`
}

func GetAnalyticsHandler(c *gin.Context) {
	var stats []models.PostAnalytics
	database.DB.Order("sync_time desc").Limit(30).Find(&stats)

	summary := AnalyticsSummary{
		TotalReach:     0,
		AvgImpact:      0,
		EngagementRate: 0.0,
		BestPerformers: []any{},
	}

	if len(stats) > 0 {
		totalImpact := 0
		totalRate := 0.0
		for _, s := range stats {
			summary.TotalReach += s.Impressions
			totalImpact += s.ImpactScore
			totalRate += s.EngagementRate
		}
		summary.AvgImpact = totalImpact / len(stats)
		summary.EngagementRate = totalRate / float64(len(stats))
	}

	c.JSON(http.StatusOK, summary)
}

func SyncAnalyticsHandler(c *gin.Context) {
	// 1. Fetch all 'posted' items that haven't been synced in the last hour
	var posts []models.ScheduledPost
	database.DB.Where("status = ?", "posted").Find(&posts)

	if len(posts) == 0 {
		c.JSON(http.StatusOK, gin.H{"message": "No active posts to sync. Try scheduling some content first!"})
		return
	}

	// 2. Simulate API Call to X/LinkedIn and AI Impact Analysis
	// In production, this would use official SDKs
	for _, post := range posts {
		// Simulation logic: Generate random but realistic growth numbers
		rand.Seed(time.Now().UnixNano())
		likes := rand.Intn(50) + 10
		reposts := rand.Intn(20) + 5
		impressions := (likes + reposts) * (rand.Intn(50) + 10)
		rate := float64(likes+reposts) / float64(impressions) * 100

		// AI calculates impact score based on content quality vs engagement
		impact := rand.Intn(30) + 70 // Furci usually performs well ;)

		analytics := models.PostAnalytics{
			ScheduledPostID: post.ID,
			Likes:           likes,
			Reposts:         reposts,
			Impressions:     impressions,
			EngagementRate:  rate,
			ImpactScore:     impact,
			SyncTime:        time.Now(),
		}

		// Upsert analytics
		database.DB.Where("scheduled_post_id = ?", post.ID).
			Assign(analytics).
			FirstOrCreate(&analytics)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Analytics synced! Your impact score has been updated."})
}
