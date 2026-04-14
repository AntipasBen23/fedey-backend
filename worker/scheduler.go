package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
	var account models.SocialAccount
	if err := database.DB.First(&account, post.AccountID).Error; err != nil {
		log.Printf("[Worker] Error finding account for post %d: %v", post.ID, err)
		database.DB.Model(&post).Update("status", "failed")
		return
	}

	log.Printf("[Worker] Publishing [%s] to %s (contentType=%s)", post.Status, post.Platform, post.ContentType)

	var externalID string
	var success bool
	var err error

	switch post.Platform {
	case "twitter", "x":
		externalID, success, err = postToX(account.AccessToken, post)
	case "linkedin":
		externalID, success, err = postToLinkedIn(account.AccessToken, post)
	default:
		log.Printf("[Worker] Unknown platform %s for post %d", post.Platform, post.ID)
		database.DB.Model(&post).Update("status", "failed")
		return
	}

	if success {
		log.Printf("[Worker] Successfully posted %d to %s (external: %s)", post.ID, post.Platform, externalID)
		database.DB.Model(&post).Updates(map[string]interface{}{
			"status":      "posted",
			"external_id": externalID,
		})
	} else {
		log.Printf("[Worker] Failed to post %d to %s: %v", post.ID, post.Platform, err)
		database.DB.Model(&post).Update("status", "failed")
	}
}

// ─── X (Twitter) Publishing ──────────────────────────────────────────────────

func postToX(token string, post models.ScheduledPost) (string, bool, error) {
	switch post.ContentType {
	case "thread":
		return postThreadToX(token, post)
	default:
		// tweet, linkedin_post, carousel, video_script — all publish as single tweets
		return postSingleTweetToX(token, post.Content)
	}
}

// postSingleTweetToX publishes a single tweet and returns the tweet ID.
func postSingleTweetToX(token, text string) (string, bool, error) {
	// X API v2 tweet endpoint
	const xTweetsURL = "https://api.twitter.com/2/tweets"

	payload := map[string]interface{}{
		"text": text,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", xTweetsURL, bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", false, fmt.Errorf("X API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, err
	}

	return result.Data.ID, true, nil
}

// postThreadToX publishes a numbered thread by chaining tweet replies.
// The script field contains tweets separated by newlines starting with "1/ ", "2/ ", etc.
func postThreadToX(token string, post models.ScheduledPost) (string, bool, error) {
	script := post.Script
	if script == "" {
		// Fall back to single tweet with post content
		return postSingleTweetToX(token, post.Content)
	}

	// Split tweets — each starts with a number followed by "/"
	tweets := splitThread(script)
	if len(tweets) == 0 {
		return postSingleTweetToX(token, post.Content)
	}

	var firstID string
	var inReplyTo string

	const xTweetsURL = "https://api.twitter.com/2/tweets"
	client := &http.Client{Timeout: 15 * time.Second}

	for i, tweet := range tweets {
		payload := map[string]interface{}{
			"text": tweet,
		}
		if inReplyTo != "" {
			payload["reply"] = map[string]string{
				"in_reply_to_tweet_id": inReplyTo,
			}
		}

		body, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", xTweetsURL, bytes.NewReader(body))
		if err != nil {
			return firstID, i > 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return firstID, i > 0, err
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			return firstID, i > 0, fmt.Errorf("X API returned %d on tweet %d: %s", resp.StatusCode, i+1, string(respBody))
		}

		var result struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		json.Unmarshal(respBody, &result)

		if i == 0 {
			firstID = result.Data.ID
		}
		inReplyTo = result.Data.ID

		// Small delay between tweets to respect rate limits
		if i < len(tweets)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	return firstID, true, nil
}

// splitThread splits a thread script into individual tweet texts.
// Handles both "1/ tweet\n2/ tweet" and newline-separated formats.
func splitThread(script string) []string {
	lines := strings.Split(script, "\n")
	var tweets []string
	var current strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Detect new tweet marker: starts with a digit followed by "/"
		isNewTweet := false
		for i, ch := range line {
			if ch == '/' && i > 0 {
				// Check everything before "/" is digits
				prefix := strings.TrimSpace(line[:i])
				allDigits := true
				for _, c := range prefix {
					if c < '0' || c > '9' {
						allDigits = false
						break
					}
				}
				if allDigits && prefix != "" {
					isNewTweet = true
					break
				}
			}
		}

		if isNewTweet && current.Len() > 0 {
			tweets = append(tweets, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		tweets = append(tweets, strings.TrimSpace(current.String()))
	}

	return tweets
}

// ─── LinkedIn Publishing ─────────────────────────────────────────────────────

func postToLinkedIn(token string, post models.ScheduledPost) (string, bool, error) {
	// 1. Get the LinkedIn person URN
	personURN, err := getLinkedInPersonURN(token)
	if err != nil {
		return "", false, fmt.Errorf("failed to get LinkedIn person URN: %w", err)
	}

	// 2. Build post text — carousel and video_script get the full caption
	text := post.Content
	if post.ContentType == "carousel" || post.ContentType == "video_script" {
		if post.Script != "" {
			text = post.Content // Caption is stored in Content (already includes hook + hashtags)
		}
	}

	// 3. Post via UGC Posts API
	return publishLinkedInTextPost(token, personURN, text)
}

// getLinkedInPersonURN fetches the authenticated user's LinkedIn person URN.
func getLinkedInPersonURN(token string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.linkedin.com/v2/me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LinkedIn /me returned %d: %s", resp.StatusCode, string(body))
	}

	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &me); err != nil || me.ID == "" {
		return "", fmt.Errorf("failed to parse LinkedIn me response: %s", string(body))
	}

	return "urn:li:person:" + me.ID, nil
}

// publishLinkedInTextPost publishes a text-only post via the UGC Posts API.
func publishLinkedInTextPost(token, personURN, text string) (string, bool, error) {
	payload := map[string]interface{}{
		"author":         personURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]interface{}{
			"com.linkedin.ugc.ShareContent": map[string]interface{}{
				"shareCommentary": map[string]string{
					"text": text,
				},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]string{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", "https://api.linkedin.com/v2/ugcPosts", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Restli-Protocol-Version", "2.0.0")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", false, fmt.Errorf("LinkedIn API returned %d: %s", resp.StatusCode, string(respBody))
	}

	// LinkedIn returns the post URN in the X-RestLi-Id header
	postID := resp.Header.Get("X-RestLi-Id")
	if postID == "" {
		// Fall back to parsing body
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)
		if id, ok := result["id"].(string); ok {
			postID = id
		}
	}

	return postID, true, nil
}
