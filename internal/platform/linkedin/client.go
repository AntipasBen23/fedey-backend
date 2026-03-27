package linkedin

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

const currentLinkedInVersion = "202602"

type Client struct {
	apiBaseURL string
	httpClient *http.Client
}

type TokenResponse struct {
	AccessToken           string `json:"access_token"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshToken          string `json:"refresh_token"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	Scope                 string `json:"scope"`
}

type Member struct {
	ID        string
	FirstName string
	LastName  string
}

type Comment struct {
	ID        string
	ActorURN  string
	Message   string
	ObjectURN string
	ParentURN string
}

type Post struct {
	ID         string
	Commentary string
	CreatedAt  time.Time
}

func NewClient(apiBaseURL string) *Client {
	return &Client{
		apiBaseURL: strings.TrimRight(strings.TrimSpace(apiBaseURL), "/"),
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) ExchangeCode(ctx context.Context, clientID, clientSecret, redirectURI, code string) (TokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", strings.TrimSpace(code))
	values.Set("client_id", strings.TrimSpace(clientID))
	values.Set("client_secret", strings.TrimSpace(clientSecret))
	values.Set("redirect_uri", strings.TrimSpace(redirectURI))

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.linkedin.com/oauth/v2/accessToken", strings.NewReader(values.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("build linkedin token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("perform linkedin token request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return TokenResponse{}, fmt.Errorf("linkedin token exchange failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
		return TokenResponse{}, fmt.Errorf("decode linkedin token response: %w", err)
	}

	return tokenResponse, nil
}

func (c *Client) GetMember(ctx context.Context, accessToken string) (Member, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/v2/me", nil)
	if err != nil {
		return Member{}, fmt.Errorf("build linkedin me request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))

	response, err := c.httpClient.Do(request)
	if err != nil {
		return Member{}, fmt.Errorf("perform linkedin me request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return Member{}, fmt.Errorf("linkedin me lookup failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		ID                 string `json:"id"`
		LocalizedFirstName string `json:"localizedFirstName"`
		LocalizedLastName  string `json:"localizedLastName"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Member{}, fmt.Errorf("decode linkedin me response: %w", err)
	}

	return Member{
		ID:        result.ID,
		FirstName: result.LocalizedFirstName,
		LastName:  result.LocalizedLastName,
	}, nil
}

func (c *Client) CreatePost(ctx context.Context, accessToken, authorURN, commentary string) (string, error) {
	payload := map[string]any{
		"author":         strings.TrimSpace(authorURN),
		"commentary":     strings.TrimSpace(commentary),
		"visibility":     "PUBLIC",
		"lifecycleState": "PUBLISHED",
		"distribution": map[string]any{
			"feedDistribution":               "MAIN_FEED",
			"targetEntities":                 []string{},
			"thirdPartyDistributionChannels": []string{},
		},
		"isReshareDisabledByAuthor": false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal linkedin post payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseURL+"/rest/posts", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build linkedin create post request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	request.Header.Set("Linkedin-Version", currentLinkedInVersion)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("perform linkedin create post request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("linkedin post create failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	postID := strings.TrimSpace(response.Header.Get("x-restli-id"))
	if postID != "" {
		return postID, nil
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err == nil && strings.TrimSpace(result.ID) != "" {
		return result.ID, nil
	}

	return "", nil
}

func (c *Client) ListComments(ctx context.Context, accessToken, targetURN string, maxCount int) ([]Comment, error) {
	endpoint := fmt.Sprintf("%s/rest/socialActions/%s/comments", c.apiBaseURL, url.PathEscape(strings.TrimSpace(targetURN)))
	if maxCount > 0 {
		endpoint += fmt.Sprintf("?count=%d", maxCount)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build linkedin list comments request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	request.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	request.Header.Set("Linkedin-Version", currentLinkedInVersion)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform linkedin list comments request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("linkedin comment listing failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		Elements []struct {
			ID      string `json:"id"`
			Actor   string `json:"actor"`
			Object  string `json:"object"`
			Parent  string `json:"parentComment"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
		} `json:"elements"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode linkedin comments response: %w", err)
	}

	comments := make([]Comment, 0, len(result.Elements))
	for _, item := range result.Elements {
		comments = append(comments, Comment{
			ID:        item.ID,
			ActorURN:  item.Actor,
			Message:   item.Message.Text,
			ObjectURN: item.Object,
			ParentURN: item.Parent,
		})
	}

	return comments, nil
}

func (c *Client) CreateComment(ctx context.Context, accessToken, actorURN, objectURN, targetURN, message, parentCommentURN string) (string, error) {
	payload := map[string]any{
		"actor":  strings.TrimSpace(actorURN),
		"object": strings.TrimSpace(objectURN),
		"message": map[string]string{
			"text": strings.TrimSpace(message),
		},
	}
	if strings.TrimSpace(parentCommentURN) != "" {
		payload["parentComment"] = strings.TrimSpace(parentCommentURN)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal linkedin comment payload: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/rest/socialActions/%s/comments", c.apiBaseURL, url.PathEscape(strings.TrimSpace(targetURN))), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build linkedin comment request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	request.Header.Set("Linkedin-Version", currentLinkedInVersion)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("perform linkedin comment request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return "", fmt.Errorf("linkedin comment create failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	commentID := strings.TrimSpace(response.Header.Get("x-restli-id"))
	if commentID != "" {
		return commentID, nil
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err == nil {
		return strings.TrimSpace(result.ID), nil
	}

	return "", nil
}

func (c *Client) ListAuthorPosts(ctx context.Context, accessToken, authorURN string, count int) ([]Post, error) {
	if count <= 0 {
		count = 10
	}

	query := url.Values{}
	query.Set("author", strings.TrimSpace(authorURN))
	query.Set("q", "author")
	query.Set("count", fmt.Sprintf("%d", count))
	query.Set("sortBy", "LAST_MODIFIED")
	endpoint := c.apiBaseURL + "/rest/posts?" + query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build linkedin author posts request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	request.Header.Set("X-Restli-Protocol-Version", "2.0.0")
	request.Header.Set("Linkedin-Version", currentLinkedInVersion)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("perform linkedin author posts request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		payload, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("linkedin author posts failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(payload)))
	}

	var result struct {
		Elements []struct {
			ID         string `json:"id"`
			Commentary string `json:"commentary"`
			CreatedAt  int64  `json:"createdAt"`
		} `json:"elements"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode linkedin author posts response: %w", err)
	}

	posts := make([]Post, 0, len(result.Elements))
	for _, item := range result.Elements {
		posts = append(posts, Post{
			ID:         item.ID,
			Commentary: item.Commentary,
			CreatedAt:  time.UnixMilli(item.CreatedAt).UTC(),
		})
	}

	return posts, nil
}
