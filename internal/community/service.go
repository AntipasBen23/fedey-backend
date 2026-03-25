package community

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
)

type Service struct {
	repository         Repository
	brandMemoryService *brandmemory.Service
}

func NewService(repository Repository, brandMemoryService *brandmemory.Service) *Service {
	return &Service{
		repository:         repository,
		brandMemoryService: brandMemoryService,
	}
}

func (s *Service) List(ctx context.Context) ([]Item, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Item, error) {
	if strings.TrimSpace(input.Platform) == "" ||
		strings.TrimSpace(input.Author) == "" ||
		strings.TrimSpace(input.Message) == "" ||
		strings.TrimSpace(input.LinkedPostRef) == "" {
		return Item{}, ErrInvalidInboxInput
	}

	return s.repository.Create(ctx, CreateInput{
		Platform:      strings.TrimSpace(input.Platform),
		Author:        strings.TrimSpace(input.Author),
		Message:       strings.TrimSpace(input.Message),
		Sentiment:     normalizeSentiment(input.Sentiment),
		LinkedPostRef: strings.TrimSpace(input.LinkedPostRef),
	})
}

func (s *Service) DraftReply(ctx context.Context, itemID string) (Item, error) {
	if strings.TrimSpace(itemID) == "" {
		return Item{}, ErrInvalidInboxInput
	}

	item, err := s.repository.GetByID(ctx, strings.TrimSpace(itemID))
	if err != nil {
		return Item{}, err
	}

	profile, err := s.brandMemoryService.Get(ctx)
	if err != nil {
		return Item{}, fmt.Errorf("get brand memory: %w", err)
	}

	item.ReplyDraft = buildReply(profile, item)
	item.Status = StatusDrafted
	if err := s.repository.Update(ctx, item); err != nil {
		return Item{}, err
	}

	return item, nil
}

func (s *Service) MarkReplied(ctx context.Context, itemID string) (Item, error) {
	if strings.TrimSpace(itemID) == "" {
		return Item{}, ErrInvalidInboxInput
	}

	item, err := s.repository.GetByID(ctx, strings.TrimSpace(itemID))
	if err != nil {
		return Item{}, err
	}

	item.Status = StatusReplied
	item.RepliedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, item); err != nil {
		return Item{}, err
	}

	return item, nil
}

func normalizeSentiment(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "positive", "negative", "neutral":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "neutral"
	}
}

func buildReply(profile brandmemory.Profile, item Item) string {
	base := fmt.Sprintf("Thanks %s. %s", item.Author, profile.BrandName)
	switch item.Sentiment {
	case "negative":
		return base + " We want the system to stay useful and grounded, so we would handle this by keeping replies aligned with the brand guardrails and escalating anything sensitive."
	case "positive":
		return base + " We would keep replies on-brand by grounding them in the voice, audience, and guardrails stored in the agent memory, not by improvising randomly."
	default:
		return base + " The reply logic should use the same brand memory and strategy context that drives the posts, so the tone stays consistent."
	}
}
