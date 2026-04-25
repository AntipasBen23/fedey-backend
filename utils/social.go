package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/models"
)

// PostToX publishes a single tweet or a thread to X (Twitter).
func PostToX(token string, post models.ScheduledPost) (string, bool, error) {
	if post.ContentType == "thread" {
		return PostThreadToX(token, post.Script)
	}
	// Default: single tweet
	return PostSingleTweetToX(token, post.Content)
}

// PostSingleTweetToX publishes a single tweet and returns the tweet ID.
func PostSingleTweetToX(token, text string) (string, bool, error) {
	const xTweetsURL = "https://api.twitter.com/2/tweets"
	text = TruncateTweet(text)

	payload := map[string]interface{}{
		"text": text,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", xTweetsURL, bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer " + token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		log.Printf("[X-Post] FAILED status=%d body=%s | ratelimit-limit=%s ratelimit-remaining=%s ratelimit-reset=%s | token-prefix=%.8s...",
			resp.StatusCode,
			string(respBody),
			resp.Header.Get("x-rate-limit-limit"),
			resp.Header.Get("x-rate-limit-remaining"),
			resp.Header.Get("x-rate-limit-reset"),
			token,
		)
		return "", false, fmt.Errorf("X API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", false, err
	}

	return result.Data.ID, true, nil
}

// PostThreadToX publishes a numbered thread by chaining tweet replies.
func PostThreadToX(token, script string) (string, bool, error) {
	tweets := SplitThread(script)
	if len(tweets) == 0 {
		return "", false, fmt.Errorf("empty thread script")
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
		req.Header.Set("Authorization", "Bearer " + token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return firstID, i > 0, err
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusCreated {
			return firstID, i > 0, fmt.Errorf("X API returned %d on tweet %d: %s", resp.StatusCode, i + 1, string(respBody))
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
		if i < len(tweets)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return firstID, true, nil
}

// TruncateTweet trims text to Twitter's 280-character limit.
func TruncateTweet(text string) string {
	const limit = 280
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := limit - 1
	for cut > 0 && runes[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = limit - 1
	}
	return string(runes[:cut]) + "…"
}

// SplitThread splits a thread script into individual tweet texts.
func SplitThread(script string) []string {
	lines := strings.Split(script, "\n")
	var tweets []string
	var current strings.Builder

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		isNewTweet := false
		for i, ch := range line {
			if ch == '/' && i > 0 {
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

// PostLinkedInText publishes a text post to LinkedIn.
func PostLinkedInText(token, personURN, text string) (string, bool, error) {
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
	req.Header.Set("Authorization", "Bearer " + token)
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

	postID := resp.Header.Get("X-RestLi-Id")
	return postID, true, nil
}

// ─── X (Twitter) Social Listening ───────────────────────────────────────────

// SearchXNiche searches for recent tweets matching a query.
func SearchXNiche(token, query string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.twitter.com/2/tweets/search/recent?query=%s&tweet.fields=author_id,created_at&max_results=10", query)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer " + token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(readAll(resp.Body), &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetXMentions fetches the latest mentions for the authenticated user.
func GetXMentions(token, userID string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("https://api.twitter.com/2/users/%s/mentions?tweet.fields=author_id,created_at", userID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer " + token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(readAll(resp.Body), &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetXMentions fetches the latest mentions for the authenticated user.
func GetXRecentReplies(token, userID string) ([]map[string]interface{}, error) {
	// We search for conversations involving us
	url := fmt.Sprintf("https://api.twitter.com/2/tweets/search/recent?query=to:%s&tweet.fields=author_id,in_reply_to_user_id,conversation_id", userID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer " + token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(readAll(resp.Body), &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func readAll(r io.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}

// GetXUserContext fetches the basic user profile for the authenticated X account.
func GetXUserContext(token string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", "https://api.twitter.com/2/users/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer " + token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(readAll(resp.Body), &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetLinkedInPersonURN fetches the authenticated user's LinkedIn person URN.
func GetLinkedInPersonURN(token string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.linkedin.com/v2/me", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer " + token)

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
		return "", fmt.Errorf("failed to parse LinkedIn me response")
	}

	return "urn:li:person:" + me.ID, nil
}
