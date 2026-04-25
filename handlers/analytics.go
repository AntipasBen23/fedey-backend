package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/AntipasBen23/fedey-backend/utils"
	"github.com/gin-gonic/gin"
)

type PeakHourData struct {
	Day        int     `json:"day"`        // 0-6 (Sun-Sat)
	Hour       int     `json:"hour"`       // 0-23
	Engagement float64 `json:"engagement"` // Avg Engagement Rate
	Reach      int     `json:"reach"`      // Total Impressions
}

func GetPeakHoursHandler(c *gin.Context) {
	if database.DB == nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Database not initialized.")
		return
	}

	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		utils.APIError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required.")
		return
	}

	var results []PeakHourData
	// Query to aggregate engagement and reach by day and hour
	err := database.DB.Table("post_analytics").
		Select("EXTRACT(DOW FROM scheduled_posts.scheduled_at) as day, EXTRACT(HOUR FROM scheduled_posts.scheduled_at) as hour, AVG(post_analytics.engagement_rate) as engagement, SUM(post_analytics.impressions) as reach").
		Joins("JOIN scheduled_posts ON scheduled_posts.id = post_analytics.scheduled_post_id").
		Where("scheduled_posts.user_id = ?", uid).
		Group("day, hour").
		Scan(&results).Error

	if err != nil {
		utils.APIError(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to query peak hours.")
		return
	}

	c.JSON(http.StatusOK, results)
}

// ensure models is used (FollowerSnapshot is referenced in sync handler)
var _ = models.FollowerSnapshot{}

// ─── Overview types (shared with dashboard) ───────────────────────────────────

type PostPerformance struct {
	PostID      uint    `json:"postId"`
	Snippet     string  `json:"snippet"`
	Platform    string  `json:"platform"`
	ContentType string  `json:"contentType"`
	Likes       int     `json:"likes"`
	Reposts     int     `json:"reposts"`
	Comments    int     `json:"comments"`
	Impressions int     `json:"impressions"`
	EngRate     float64 `json:"engRate"`
	PostedAt    string  `json:"postedAt"`
}

type FollowerPoint struct {
	Date  string `json:"date"`  // "Apr 12"
	Count int    `json:"count"`
}

type AnalyticsOverview struct {
	FollowerCount    int               `json:"followerCount"`
	FollowerGrowth   int               `json:"followerGrowth"`   // change vs 30 days ago
	TotalImpressions int               `json:"totalImpressions"`
	TotalReactions   int               `json:"totalReactions"`   // likes + reposts + comments
	AvgEngRate       float64           `json:"avgEngRate"`
	PostsAnalyzed    int               `json:"postsAnalyzed"`
	TopPosts         []PostPerformance `json:"topPosts"`
	FollowerHistory  []FollowerPoint   `json:"followerHistory"`  // last 30 days
	LastSynced       string            `json:"lastSynced"`
	Insight          string            `json:"insight"`
}

// ─── GET /v1/analytics ────────────────────────────────────────────────────────

func GetAnalyticsHandler(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		utils.APIError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required.")
		return
	}
	c.JSON(http.StatusOK, buildAnalyticsOverview(uid))
}

// buildAnalyticsOverview assembles AnalyticsOverview from stored data.
// Called by both GetAnalyticsHandler and GetDashboardHandler.
func buildAnalyticsOverview(userID uint) AnalyticsOverview {
	overview := AnalyticsOverview{Insight: "Sync your analytics to see performance insights."}
	if database.DB == nil {
		return overview
	}

	// Follower count from the connected account
	var account models.SocialAccount
	if database.DB.Where("user_id = ? AND platform IN ?", userID, []string{"twitter", "x"}).
		Order("updated_at desc").First(&account).Error == nil {
		overview.FollowerCount = account.FollowerCount
	}

	// Pull analytics for posted posts
	type row struct {
		models.PostAnalytics
		Content     string
		Platform    string
		ContentType string
		ScheduledAt time.Time
	}

	var rows []row
	database.DB.Table("post_analytics").
		Select("post_analytics.*, scheduled_posts.content, scheduled_posts.platform, scheduled_posts.content_type, scheduled_posts.scheduled_at").
		Joins("JOIN scheduled_posts ON scheduled_posts.id = post_analytics.scheduled_post_id").
		Where("post_analytics.deleted_at IS NULL AND scheduled_posts.user_id = ?", userID).
		Order("post_analytics.engagement_rate DESC").
		Limit(50).
		Scan(&rows)

	if len(rows) == 0 {
		return overview
	}

	overview.PostsAnalyzed = len(rows)
	var totalEngRate float64
	var lastSync time.Time

	for _, r := range rows {
		overview.TotalImpressions += r.Impressions
		overview.TotalReactions += r.Likes + r.Reposts + r.Comments
		totalEngRate += r.EngagementRate
		if r.SyncTime.After(lastSync) {
			lastSync = r.SyncTime
		}
	}
	overview.AvgEngRate = totalEngRate / float64(len(rows))
	if !lastSync.IsZero() {
		overview.LastSynced = lastSync.Format(time.RFC3339)
	}

	// Top 5 posts by engagement rate
	limit := 5
	if len(rows) < limit {
		limit = len(rows)
	}
	for _, r := range rows[:limit] {
		snippet := r.Content
		if len(snippet) > 90 {
			snippet = snippet[:90] + "…"
		}
		overview.TopPosts = append(overview.TopPosts, PostPerformance{
			PostID:      r.ScheduledPostID,
			Snippet:     snippet,
			Platform:    r.Platform,
			ContentType: r.ContentType,
			Likes:       r.Likes,
			Reposts:     r.Reposts,
			Comments:    r.Comments,
			Impressions: r.Impressions,
			EngRate:     r.EngagementRate,
			PostedAt:    r.ScheduledAt.Format("Jan 2"),
		})
	}

	// Follower growth history — last 30 daily snapshots
	var snapshots []models.FollowerSnapshot
	database.DB.Where("user_id = ? AND date >= ?", userID, time.Now().AddDate(0, 0, -30).Format("2006-01-02")).
		Order("date asc").
		Find(&snapshots)

	for _, s := range snapshots {
		t, err := time.Parse("2006-01-02", s.Date)
		label := s.Date
		if err == nil {
			label = t.Format("Jan 2")
		}
		overview.FollowerHistory = append(overview.FollowerHistory, FollowerPoint{
			Date:  label,
			Count: s.Count,
		})
	}

	// Growth vs oldest snapshot in window
	if len(snapshots) >= 2 {
		overview.FollowerGrowth = snapshots[len(snapshots)-1].Count - snapshots[0].Count
	}

	overview.Insight = generateInsight(overview)
	return overview
}

// generateInsight returns a short, actionable growth tip based on the metrics.
func generateInsight(o AnalyticsOverview) string {
	if o.PostsAnalyzed == 0 {
		return "Post your first piece of content to start tracking performance."
	}
	switch {
	case o.AvgEngRate >= 5:
		return "Your engagement rate is above average — keep the posting frequency consistent."
	case o.AvgEngRate >= 2:
		return "Solid engagement. Try more threads or carousels — they typically outperform single tweets."
	case o.TotalImpressions < 500:
		return "Impressions are low. Posting daily for the next 7 days is the fastest way to grow reach."
	default:
		return "Your best posts get the most replies. Aim to end every post with a direct question."
	}
}

// ─── POST /v1/analytics/sync ─────────────────────────────────────────────────

func SyncAnalyticsHandler(c *gin.Context) {
	userIDVal, _ := c.Get("userID")
	uid, _ := userIDVal.(uint)
	if uid == 0 {
		utils.APIError(c, http.StatusUnauthorized, "AUTH_REQUIRED", "Authentication required.")
		return
	}

	// Find connected Twitter/X account
	var account models.SocialAccount
	err := database.DB.Where("user_id = ? AND platform IN ?", uid, []string{"twitter", "x"}).
		Order("updated_at desc").First(&account).Error
	if err != nil || account.AccessToken == "" {
		utils.APIError(c, http.StatusNotFound, "NOT_FOUND", "No Twitter/X account connected. Connect one first.")
		return
	}

	log.Printf("[Analytics] Syncing for account %d (platform=%s)", account.ID, account.Platform)

	// 1. Fetch Twitter user profile (ID + follower count)
	userID, followerCount, profileErr := fetchTwitterProfile(account.AccessToken)
	if profileErr != nil {
		log.Printf("[Analytics] Profile fetch failed: %v", profileErr)
		utils.APIError(c, http.StatusBadGateway, "SERVER_ERROR", "Twitter API error: "+profileErr.Error())
		return
	}

	// Persist updated follower count on the account row
	database.DB.Model(&account).Update("follower_count", followerCount)
	log.Printf("[Analytics] Follower count: %d", followerCount)

	// Save daily snapshot (one per day — upsert by user + date + platform)
	today := time.Now().Format("2006-01-02")
	snapshot := models.FollowerSnapshot{
		UserID:   uid,
		Date:     today,
		Platform: account.Platform,
		Count:    followerCount,
	}
	database.DB.Where("user_id = ? AND date = ? AND platform = ?", uid, today, account.Platform).
		Assign(snapshot).
		FirstOrCreate(&snapshot)

	// 2. Fetch recent tweet metrics
	tweets, tweetsErr := fetchTwitterTweetMetrics(account.AccessToken, userID)
	if tweetsErr != nil {
		log.Printf("[Analytics] Tweet metrics fetch failed: %v", tweetsErr)
		// Non-fatal — still return follower update
		c.JSON(http.StatusOK, gin.H{
			"message":       fmt.Sprintf("Followers updated (%d). Tweet metrics unavailable: %s", followerCount, tweetsErr.Error()),
			"followerCount": followerCount,
			"synced":        0,
		})
		return
	}

	// 3. Match tweets to our ScheduledPosts by ExternalID and upsert analytics
	synced := 0
	for _, t := range tweets {
		var post models.ScheduledPost
		// Use Find+limit instead of First to avoid GORM logging "record not found" for every
		// tweet that was posted directly on X (not through Furci).
		var results []models.ScheduledPost
		database.DB.Where("user_id = ? AND external_id = ?", uid, t.ID).Limit(1).Find(&results)
		if len(results) == 0 {
			continue // not our post
		}
		post = results[0]

		total := t.PublicMetrics.LikeCount + t.PublicMetrics.RetweetCount + t.PublicMetrics.ReplyCount
		impressions := t.PublicMetrics.ImpressionCount
		
		// DEBUG: Log the raw numbers we got from Twitter
		log.Printf("[Analytics] Post %d (ext: %s) -> Likes: %d, Reposts: %d, Impressions: %d", 
			post.ID, t.ID, t.PublicMetrics.LikeCount, t.PublicMetrics.RetweetCount, impressions)

		engRate := 0.0
		if impressions > 0 {
			engRate = float64(total) / float64(impressions) * 100
		} else if total > 0 {
			// Fallback estimate if impressions is somehow missing but likes exist
			impressions = total * 30 
			engRate = float64(total) / float64(impressions) * 100
		}
		
		// Ensure we always save at least a small number of impressions if the post exists
		if impressions == 0 && post.Status == "posted" {
			impressions = 1 // At least 1 (the user themselves)
		}

		analytics := models.PostAnalytics{
			ScheduledPostID: post.ID,
			Likes:           t.PublicMetrics.LikeCount,
			Reposts:         t.PublicMetrics.RetweetCount,
			Comments:        t.PublicMetrics.ReplyCount,
			Impressions:     impressions,
			EngagementRate:  engRate,
			ImpactScore:     impactScore(engRate, impressions),
			SyncTime:        time.Now(),
		}

		database.DB.Where("scheduled_post_id = ?", post.ID).
			Assign(analytics).
			FirstOrCreate(&analytics)
		synced++
	}

	log.Printf("[Analytics] Synced %d posts", synced)
	c.JSON(http.StatusOK, gin.H{
		"message":       fmt.Sprintf("Synced %d posts. Followers: %d", synced, followerCount),
		"followerCount": followerCount,
		"synced":        synced,
	})
}

// ─── Twitter API helpers ──────────────────────────────────────────────────────

type twitterTweet struct {
	ID            string `json:"id"`
	Text          string `json:"text"`
	PublicMetrics struct {
		LikeCount       int `json:"like_count"`
		RetweetCount    int `json:"retweet_count"`
		ReplyCount      int `json:"reply_count"`
		QuoteCount      int `json:"quote_count"`
		ImpressionCount int `json:"impression_count"`
	} `json:"public_metrics"`
}

func fetchTwitterProfile(token string) (userID string, followers int, err error) {
	req, err := http.NewRequest("GET",
		"https://api.twitter.com/2/users/me?user.fields=public_metrics", nil)
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Twitter /users/me returned %d: %s", resp.StatusCode, string(body))
		return
	}

	var result struct {
		Data struct {
			ID            string `json:"id"`
			PublicMetrics struct {
				FollowersCount int `json:"followers_count"`
			} `json:"public_metrics"`
		} `json:"data"`
	}
	if e := json.Unmarshal(body, &result); e != nil {
		err = e
		return
	}
	userID = result.Data.ID
	followers = result.Data.PublicMetrics.FollowersCount
	return
}

func fetchTwitterTweetMetrics(token, userID string) ([]twitterTweet, error) {
	url := fmt.Sprintf(
		"https://api.twitter.com/2/users/%s/tweets?max_results=20&tweet.fields=public_metrics",
		userID,
	)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Twitter /tweets returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []twitterTweet `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result.Data, nil
}

// impactScore converts engagement rate + impressions into a 1–100 score.
func impactScore(engRate float64, impressions int) int {
	base := int(engRate * 12)
	if impressions > 5000 {
		base += 15
	} else if impressions > 1000 {
		base += 8
	}
	if base > 100 {
		return 100
	}
	if base < 10 {
		return 10
	}
	return base
}
