package community

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
)

type Service struct {
	repository         Repository
	brandMemoryService *brandmemory.Service
	xClient            *xplatform.Client
}

func NewService(repository Repository, brandMemoryService *brandmemory.Service, xClient *xplatform.Client) *Service {
	return &Service{
		repository:         repository,
		brandMemoryService: brandMemoryService,
		xClient:            xClient,
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
		Platform:          strings.TrimSpace(input.Platform),
		Author:            strings.TrimSpace(input.Author),
		Message:           strings.TrimSpace(input.Message),
		Sentiment:         normalizeSentiment(input.Sentiment),
		LinkedPostRef:     strings.TrimSpace(input.LinkedPostRef),
		ExternalCommentID: strings.TrimSpace(input.ExternalCommentID),
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

	if strings.EqualFold(item.Platform, "x") {
		if s.xClient == nil || !s.xClient.Configured() || strings.TrimSpace(item.ExternalCommentID) == "" || strings.TrimSpace(item.ReplyDraft) == "" {
			return Item{}, ErrInvalidInboxInput
		}

		if _, err := s.xClient.PublishPost(ctx, item.ReplyDraft, item.ExternalCommentID); err != nil {
			return Item{}, err
		}
	}

	item.Status = StatusReplied
	item.RepliedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, item); err != nil {
		return Item{}, err
	}

	return item, nil
}

func (s *Service) SyncXMentions(ctx context.Context) (int, error) {
	if s.xClient == nil || !s.xClient.Configured() {
		return 0, ErrInvalidInboxInput
	}

	mentions, err := s.xClient.FetchMentions(ctx, 10)
	if err != nil {
		return 0, err
	}

	existing, err := s.repository.List(ctx)
	if err != nil {
		return 0, err
	}

	known := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		if item.ExternalCommentID != "" {
			known[item.ExternalCommentID] = struct{}{}
		}
	}

	created := 0
	for _, mention := range mentions {
		if _, exists := known[mention.ID]; exists {
			continue
		}

		_, err := s.repository.Create(ctx, CreateInput{
			Platform:          "x",
			Author:            firstNonEmpty(mention.Author, "x_user"),
			Message:           mention.Text,
			Sentiment:         "neutral",
			LinkedPostRef:     mention.ID,
			ExternalCommentID: mention.ID,
		})
		if err != nil {
			return created, err
		}
		created++
	}

	return created, nil
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

func firstNonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return value
}
