package community

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
	linkedinplatform "github.com/AntipasBen23/fedey-backend/internal/platform/linkedin"
	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type Service struct {
	repository         Repository
	brandMemoryService *brandmemory.Service
	publishingService  *publishing.Service
	xClient            *xplatform.Client
	xAccountService    *xaccounts.Service
	linkedinClient     *linkedinplatform.Client
	linkedinService    *linkedinaccounts.Service
}

func NewService(
	repository Repository,
	brandMemoryService *brandmemory.Service,
	publishingService *publishing.Service,
	xClient *xplatform.Client,
	xAccountService *xaccounts.Service,
	linkedinClient *linkedinplatform.Client,
	linkedinService *linkedinaccounts.Service,
) *Service {
	return &Service{
		repository:         repository,
		brandMemoryService: brandMemoryService,
		publishingService:  publishingService,
		xClient:            xClient,
		xAccountService:    xAccountService,
		linkedinClient:     linkedinClient,
		linkedinService:    linkedinService,
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
		if s.xClient == nil || strings.TrimSpace(item.ExternalCommentID) == "" || strings.TrimSpace(item.ReplyDraft) == "" {
			return Item{}, ErrInvalidInboxInput
		}

		account, err := s.resolveXCredentials(ctx)
		if err != nil {
			return Item{}, err
		}

		if _, err := s.xClient.PublishPostWithToken(ctx, account.AccessToken, item.ReplyDraft, item.ExternalCommentID); err != nil {
			return Item{}, err
		}
	}
	if strings.EqualFold(item.Platform, "linkedin") {
		if s.linkedinClient == nil || s.linkedinService == nil || strings.TrimSpace(item.LinkedPostRef) == "" || strings.TrimSpace(item.ExternalCommentID) == "" || strings.TrimSpace(item.ReplyDraft) == "" {
			return Item{}, ErrInvalidInboxInput
		}

		account, err := s.linkedinService.GetActive(ctx)
		if err != nil {
			return Item{}, err
		}

		parentCommentURN := buildLinkedInCommentURN(item.LinkedPostRef, item.ExternalCommentID)
		if _, err := s.linkedinClient.CreateComment(ctx, account.AccessToken, account.AuthorURN, item.LinkedPostRef, item.LinkedPostRef, item.ReplyDraft, parentCommentURN); err != nil {
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
		account, err := s.resolveXCredentials(ctx)
		if err != nil {
			return 0, ErrInvalidInboxInput
		}
		return s.syncMentionsWithAccount(ctx, account)
	}

	account, err := s.resolveXCredentials(ctx)
	if err != nil {
		return 0, ErrInvalidInboxInput
	}

	return s.syncMentionsWithAccount(ctx, account)
}

func (s *Service) SyncLinkedInComments(ctx context.Context) (int, error) {
	if s.linkedinClient == nil || s.linkedinService == nil || s.publishingService == nil {
		return 0, ErrInvalidInboxInput
	}

	account, err := s.linkedinService.GetActive(ctx)
	if err != nil {
		return 0, ErrInvalidInboxInput
	}

	schedules, err := s.publishingService.List(ctx)
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
			known["linkedin:"+item.ExternalCommentID] = struct{}{}
		}
	}

	created := 0
	for _, schedule := range schedules {
		if !strings.EqualFold(schedule.Channel, "linkedin") || schedule.Status != publishing.StatusPublished || strings.TrimSpace(schedule.PlatformPostID) == "" {
			continue
		}

		threadURN := normalizeLinkedInThreadURN(schedule.PlatformPostID)
		comments, err := s.linkedinClient.ListComments(ctx, account.AccessToken, threadURN, 10)
		if err != nil {
			return created, err
		}

		for _, comment := range comments {
			key := "linkedin:" + comment.ID
			if _, exists := known[key]; exists {
				continue
			}

			_, err := s.repository.Create(ctx, CreateInput{
				Platform:          "linkedin",
				Author:            firstNonEmpty(comment.ActorURN, "linkedin_member"),
				Message:           firstNonEmpty(comment.Message, "New LinkedIn comment"),
				Sentiment:         "neutral",
				LinkedPostRef:     threadURN,
				ExternalCommentID: comment.ID,
			})
			if err != nil {
				return created, err
			}
			known[key] = struct{}{}
			created++
		}
	}

	return created, nil
}

func (s *Service) syncMentionsWithAccount(ctx context.Context, account xaccounts.Account) (int, error) {
	mentions, err := s.xClient.FetchMentionsWithCredentials(ctx, account.AccessToken, account.UserID, 10)
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

func normalizeLinkedInThreadURN(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.HasPrefix(trimmed, "urn:") {
		return trimmed
	}

	return "urn:li:ugcPost:" + trimmed
}

func buildLinkedInCommentURN(objectURN, commentID string) string {
	return fmt.Sprintf("urn:li:comment:(%s,%s)", strings.TrimSpace(objectURN), strings.TrimSpace(commentID))
}

func (s *Service) resolveXCredentials(ctx context.Context) (xaccounts.Account, error) {
	if s.xAccountService != nil {
		account, err := s.xAccountService.GetActive(ctx)
		if err == nil {
			return account, nil
		}
	}

	if s.xClient != nil && s.xClient.Configured() {
		return xaccounts.Account{
			AccessToken: s.xClient.AccessToken(),
			UserID:      s.xClient.UserID(),
		}, nil
	}

	return xaccounts.Account{}, ErrInvalidInboxInput
}
