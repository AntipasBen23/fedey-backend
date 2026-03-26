package x

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL     string
	accessToken string
	userID      string
	httpClient  *http.Client
}

type Mention struct {
	ID        string
	Author    string
	Text      string
	CreatedAt time.Time
}

type TokenResponse struct {
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	Scope        string `json:"scope"`
	RefreshToken string `json:"refresh_token"`
}

type User struct {
	ID       string
	Username string
}

func NewClient(baseURL, accessToken, userID string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		accessToken: strings.TrimSpace(accessToken),
		userID:      strings.TrimSpace(userID),
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.accessToken != "" && c.userID != ""
}

func (c *Client) AccessToken() string {
	return c.accessToken
}

func (c *Client) UserID() string {
	return c.userID
}

func (c *Client) ExchangeCode(ctx context.Context, clientID, redirectURI, code, codeVerifier string) (TokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("client_id", clientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("code", strings.TrimSpace(code))
	values.Set("code_verifier", strings.TrimSpace(codeVerifier))

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/2/oauth2/token", strings.NewReader(values.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("build x token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("perform x token request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return TokenResponse{}, fmt.Errorf("x token exchange failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
		return TokenResponse{}, fmt.Errorf("decode x token response: %w", err)
	}

	return tokenResponse, nil
}

func (c *Client) GetAuthenticatedUser(ctx context.Context, accessToken string) (User, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/2/users/me?user.fields=username", nil)
	if err != nil {
		return User{}, fmt.Errorf("build x me request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return User{}, fmt.Errorf("perform x me request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return User{}, fmt.Errorf("x me lookup failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return User{}, fmt.Errorf("decode x me response: %w", err)
	}

	return User{
		ID:       result.Data.ID,
		Username: result.Data.Username,
	}, nil
}

func (c *Client) PublishPost(ctx context.Context, text string, replyToPostID string) (string, error) {
	return c.PublishPostWithToken(ctx, c.accessToken, text, replyToPostID)
}

func (c *Client) PublishPostWithToken(ctx context.Context, accessToken, text string, replyToPostID string) (string, error) {
	if !c.Configured() {
		if strings.TrimSpace(accessToken) == "" {
			return "", fmt.Errorf("x client is not configured")
		}
	}

	payload := map[string]any{
		"text": strings.TrimSpace(text),
	}
	if strings.TrimSpace(replyToPostID) != "" {
		payload["reply"] = map[string]string{
			"in_reply_to_tweet_id": strings.TrimSpace(replyToPostID),
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal x publish payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/2/tweets", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build x publish request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("perform x publish request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("x publish failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode x publish response: %w", err)
	}

	return result.Data.ID, nil
}

func (c *Client) FetchMentions(ctx context.Context, maxResults int) ([]Mention, error) {
	return c.FetchMentionsWithCredentials(ctx, c.accessToken, c.userID, maxResults)
}

func (c *Client) FetchMentionsWithCredentials(ctx context.Context, accessToken, userID string, maxResults int) ([]Mention, error) {
	if !c.Configured() {
		if strings.TrimSpace(accessToken) == "" || strings.TrimSpace(userID) == "" {
			return nil, fmt.Errorf("x client is not configured")
		}
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	query := url.Values{}
	query.Set("max_results", fmt.Sprintf("%d", maxResults))
	query.Set("expansions", "author_id")
	query.Set("user.fields", "username")
	query.Set("tweet.fields", "created_at,author_id")

	endpoint := fmt.Sprintf("%s/2/users/%s/mentions?%s", c.baseURL, strings.TrimSpace(userID), query.Encode())
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build x mentions request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform x mentions request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("x mentions failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		Data []struct {
			ID        string `json:"id"`
			Text      string `json:"text"`
			AuthorID  string `json:"author_id"`
			CreatedAt string `json:"created_at"`
		} `json:"data"`
		Includes struct {
			Users []struct {
				ID       string `json:"id"`
				Username string `json:"username"`
			} `json:"users"`
		} `json:"includes"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode x mentions response: %w", err)
	}

	usernames := make(map[string]string, len(result.Includes.Users))
	for _, user := range result.Includes.Users {
		usernames[user.ID] = user.Username
	}

	mentions := make([]Mention, 0, len(result.Data))
	for _, item := range result.Data {
		createdAt, _ := time.Parse(time.RFC3339, item.CreatedAt)
		mentions = append(mentions, Mention{
			ID:        item.ID,
			Author:    usernames[item.AuthorID],
			Text:      item.Text,
			CreatedAt: createdAt,
		})
	}

	return mentions, nil
}
