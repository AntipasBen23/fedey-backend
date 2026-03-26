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
