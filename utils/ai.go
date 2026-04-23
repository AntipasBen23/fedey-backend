package utils

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// DraftEngagementReply uses AI to generate a contextual, high-value social media reply.
func DraftEngagementReply(originalText, authorHandle, niche string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY missing")
	}

	client := openai.NewClient(apiKey)
	
	prompt := fmt.Sprintf(`You are a community manager for a brand in the "%s" niche. 
An account (@%s) posted this: "%s"

Write an intelligent, engaging, and non-spammy reply. 
- If it's a mention/question to us: Answer helpfully and punchily.
- If it's a niche post: Add value, share a quick insight, or ask a smart follow-up question.
- Stay under 250 characters.
- Use a friendly, human tone (not corporate).
- Avoid generic "Great post!" comments.

Return ONLY the reply text.`, niche, authorHandle, originalText)

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
