package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// DraftEngagementReply uses AI to generate a contextual, high-value social media reply.
func DraftEngagementReply(originalText, authorHandle, niche, history string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY missing")
	}

	client := openai.NewClient(apiKey)
	
	prompt := fmt.Sprintf(`You are a community manager for a brand in the "%s" niche. 
An account (@%s) posted this: "%s"

OUR SHARED HISTORY WITH THIS PERSON:
%s

Write an intelligent, engaging, and non-spammy reply. 
- If we have history: Refer to it or use a more familiar tone.
- If it's a mention/question to us: Answer helpfully and punchily.
- Stay under 250 characters.
`, niche, authorHandle, originalText, history)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateAIResponse is a general helper for getting a completion from OpenAI
func GenerateAIResponse(prompt string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY missing")
	}

	client := openai.NewClient(apiKey)
	
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4oMini,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
		},
	)

	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
