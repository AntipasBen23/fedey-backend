package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/sashabaranov/go-openai"
)

func fetchUserTweets(accessToken string) (string, error) {
	// 1. Get User ID
	req, _ := http.NewRequest("GET", "https://api.twitter.com/2/users/me", nil)
	req.Header.Add("Authorization", "Bearer "+accessToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var userResp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return "", err
	}

	// 2. Get Tweets
	tweetURL := fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets?max_results=5", userResp.Data.ID)
	req, _ = http.NewRequest("GET", tweetURL, nil)
	req.Header.Add("Authorization", "Bearer "+accessToken)
	resp, err = client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tweetResp struct {
		Data []struct {
			Text string `json:"text"`
		} `json:"data"`
	}
	json.Unmarshal(body, &tweetResp)

	tweetSummary := ""
	for i, t := range tweetResp.Data {
		tweetSummary += fmt.Sprintf("%d. %s\n", i+1, t.Text)
	}

	if tweetSummary == "" {
		return "No recent tweets found.", nil
	}
	return tweetSummary, nil
}

type StrategyRequest struct {
	ProductSummary string `json:"productSummary"`
	Platform       string `json:"platform"`
	AccountType    string `json:"accountType"` // "old" or "new"
}

type ProfessionalStrategy struct {
	IdentityAudit     string   `json:"identityAudit"`     // New: AI summary of existing profile vs new goal
	TrendMonitoring   []string `json:"trendMonitoring"`
	GrowthExperiments []string `json:"growthExperiments"`
	AnalyticsReporting []string `json:"analyticsReporting"`
}

const strategyPromptTemplate = `You are Furci AI, a social media growth agent. 
Analyze the following context and develop a professional strategy.

USER GOAL (The new product/career):
%s

%s

Provide:
1. **Identity Audit**: Based on the context above, summarize the "Identity Gap." If they are transitioning from one field (e.g. Engineering) to another (e.g. Healthcare), acknowledge it and suggest a "Clean Pivot" strategy as requested.
2. **Trend Monitoring Tactics**: How to monitor industry trends for the NEW goal.
3. **Growth Experiments**: 3 specific hypotheses to test for rapid growth in the NEW field.
4. **Analytics Reporting**: Key metrics and reporting logic.

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "identityAudit": "Summary text here...",
  "trendMonitoring": ["tactic1", "tactic2"],
  "growthExperiments": ["experiment1", "experiment2"],
  "analyticsReporting": ["metric1", "metric2"]
}
`

func StrategyHandler(c *gin.Context) {
	var req StrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Furci couldn't access her brain (API Key missing)."})
		return
	}

	client := openai.NewClient(apiKey)

	// Fetch Audit Context if "old" account
	auditContext := "No profile audit performed (New Account)."
	if req.AccountType == "old" && database.DB != nil {
		var account models.SocialAccount
		result := database.DB.Where("platform = ?", req.Platform).First(&account)
		if result.Error == nil {
			tweets, err := fetchUserTweets(account.AccessToken)
			if err == nil {
				auditContext = fmt.Sprintf("PROFILE AUDIT (Recent Posts):\n%s", tweets)
			}
		}
	}

	prompt := fmt.Sprintf(strategyPromptTemplate, req.ProductSummary, auditContext)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate strategy: " + err.Error()})
		return
	}

	var strategy ProfessionalStrategy
	err = json.Unmarshal([]byte(resp.Choices[0].Message.Content), &strategy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse strategy response: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, strategy)
}
