package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.openai.com/v1"

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type OnboardingResolution struct {
	AssistantMessage string             `json:"assistant_message"`
	ResolvedAnswers  []ResolvedQuestion `json:"resolved_answers"`
}

type ResolvedQuestion struct {
	QuestionID string `json:"question_id"`
	Answer     string `json:"answer"`
}

func NewClient(baseURL, apiKey, model string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = "gpt-4.1-mini"
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(apiKey),
		model:   model,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.apiKey != ""
}

func (c *Client) ResolveOnboardingChat(ctx context.Context, systemPrompt string, messages []Message) (OnboardingResolution, error) {
	if !c.Configured() {
		return OnboardingResolution{}, fmt.Errorf("openai client is not configured")
	}

	requestBody := chatCompletionRequest{
		Model: c.model,
		Messages: append([]chatMessage{
			{Role: "system", Content: systemPrompt},
		}, toChatMessages(messages)...),
		ResponseFormat: &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaConfig{
				Name:   "onboarding_resolution",
				Strict: true,
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"assistant_message": map[string]any{
							"type": "string",
						},
						"resolved_answers": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"question_id": map[string]any{"type": "string"},
									"answer":      map[string]any{"type": "string"},
								},
								"required":             []string{"question_id", "answer"},
								"additionalProperties": false,
							},
						},
					},
					"required":             []string{"assistant_message", "resolved_answers"},
					"additionalProperties": false,
				},
			},
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return OnboardingResolution{}, fmt.Errorf("marshal openai request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return OnboardingResolution{}, fmt.Errorf("build openai request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return OnboardingResolution{}, fmt.Errorf("perform openai request: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return OnboardingResolution{}, fmt.Errorf("read openai response: %w", err)
	}
	if response.StatusCode >= http.StatusBadRequest {
		return OnboardingResolution{}, fmt.Errorf("openai request failed with status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return OnboardingResolution{}, fmt.Errorf("decode openai response: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return OnboardingResolution{}, fmt.Errorf("openai response did not include assistant content")
	}

	var resolution OnboardingResolution
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &resolution); err != nil {
		return OnboardingResolution{}, fmt.Errorf("decode onboarding resolution: %w", err)
	}
	return resolution, nil
}

type Message struct {
	Role    string
	Content string
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type       string            `json:"type"`
	JSONSchema *jsonSchemaConfig `json:"json_schema,omitempty"`
}

type jsonSchemaConfig struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func toChatMessages(messages []Message) []chatMessage {
	items := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		items = append(items, chatMessage{
			Role:    role,
			Content: content,
		})
	}
	return items
}
