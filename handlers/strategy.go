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
	// 1. Get User Profile (including Bio)
	req, _ := http.NewRequest("GET", "https://api.twitter.com/2/users/me?user.fields=description", nil)
	req.Header.Add("Authorization", "Bearer "+accessToken)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var userResp struct {
		Data struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Name        string `json:"name"`
		} `json:"data"`
	}
	bodyUser, _ := io.ReadAll(resp.Body)
	json.Unmarshal(bodyUser, &userResp)
	fmt.Printf("TWITTER USER DATA: Name=%s, Bio=%s\n", userResp.Data.Name, userResp.Data.Description)

	// 2. Get Tweets
	tweetURL := fmt.Sprintf("https://api.twitter.com/2/users/%s/tweets?max_results=5", userResp.Data.ID)
	req, _ = http.NewRequest("GET", tweetURL, nil)
	req.Header.Add("Authorization", "Bearer "+accessToken)
	resp, err = client.Do(req)
	
	tweetSummary := fmt.Sprintf("USER PROFILE NAME: %s\nBIO/DESCRIPTION: %s\n\nRECENT POSTS:\n", userResp.Data.Name, userResp.Data.Description)
	
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var tweetResp struct {
			Data []struct {
				Text string `json:"text"`
			} `json:"data"`
		}
		json.Unmarshal(body, &tweetResp)
		
		if len(tweetResp.Data) > 0 {
			for i, t := range tweetResp.Data {
				tweetSummary += fmt.Sprintf("%d. %s\n", i+1, t.Text)
			}
		} else {
			tweetSummary += "(No recent posts found in the last 7 days)\n"
		}
	} else {
		tweetSummary += "(Could not fetch posts: Access Denied or API Limit)\n"
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

STRICT INSTRUCTIONS FOR IDENTITY AUDIT:
1. You MUST start the "identityAudit" by stating the user's Profile Name and Bio/Description if provided.
2. Check the "RECENT POSTS" section. If it says "No recent posts found", acknowledge that you are basing your audit primarily on their Profile Bio.
3. Identify the "Identity Gap" clearly. If they are a Software Engineer (per Bio) and want to do Healthcare (per User Goal), call it out. 
4. Suggest a "Clean Pivot" strategy as requested. Do NOT make up facts about their background that aren't in the provided Bio or Posts.

Format requirements: Return ONLY a valid JSON object matching this schema exactly:
{
  "identityAudit": "Based on your name [Name] and bio [Bio], I see you are a [Role]. Your last posts are about [Topics]. Since you want to move to [Goal], here is the pivot plan...",
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
