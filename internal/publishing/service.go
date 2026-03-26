package publishing

import (
	"context"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/content"
	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
)

type Service struct {
	repository     Repository
	contentService *content.Service
	xClient        *xplatform.Client
}

func NewService(repository Repository, contentService *content.Service, xClient *xplatform.Client) *Service {
	return &Service{
		repository:     repository,
		contentService: contentService,
		xClient:        xClient,
	}
}

func (s *Service) List(ctx context.Context) ([]Schedule, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Schedule, error) {
	if strings.TrimSpace(input.DraftID) == "" || strings.TrimSpace(input.Channel) == "" || input.ScheduledFor.IsZero() {
		return Schedule{}, ErrInvalidScheduleInput
	}

	draft, err := s.contentService.GetByID(ctx, strings.TrimSpace(input.DraftID))
	if err != nil {
		return Schedule{}, err
	}

	if input.VariantLabel != "" && !hasVariant(draft, input.VariantLabel) {
		return Schedule{}, ErrInvalidScheduleInput
	}

	if input.ScheduledFor.Before(time.Now().UTC().Add(-1 * time.Minute)) {
		return Schedule{}, ErrInvalidScheduleInput
	}

	return s.repository.Create(ctx, CreateInput{
		DraftID:      draft.ID,
		VariantLabel: strings.ToUpper(strings.TrimSpace(input.VariantLabel)),
		Channel:      strings.TrimSpace(input.Channel),
		ScheduledFor: input.ScheduledFor,
	})
}

func (s *Service) MarkPublished(ctx context.Context, scheduleID string) (Schedule, error) {
	if strings.TrimSpace(scheduleID) == "" {
		return Schedule{}, ErrInvalidScheduleInput
	}

	schedule, err := s.repository.GetByID(ctx, strings.TrimSpace(scheduleID))
	if err != nil {
		return Schedule{}, err
	}

	platformPostID := ""
	if strings.EqualFold(schedule.Channel, "x") {
		if s.xClient == nil || !s.xClient.Configured() {
			return Schedule{}, ErrInvalidScheduleInput
		}

		draft, err := s.contentService.GetByID(ctx, schedule.DraftID)
		if err != nil {
			return Schedule{}, err
		}

		postID, err := s.xClient.PublishPost(ctx, buildPublishText(draft, schedule.VariantLabel), "")
		if err != nil {
			return Schedule{}, err
		}
		platformPostID = postID
	}

	return s.repository.MarkPublished(ctx, strings.TrimSpace(scheduleID), platformPostID)
}

func hasVariant(draft content.Draft, label string) bool {
	needle := strings.ToUpper(strings.TrimSpace(label))
	for _, variant := range draft.Variants {
		if strings.ToUpper(variant.Label) == needle {
			return true
		}
	}

	return false
}

func buildPublishText(draft content.Draft, variantLabel string) string {
	if strings.TrimSpace(variantLabel) != "" {
		for _, variant := range draft.Variants {
			if strings.EqualFold(variant.Label, variantLabel) {
				return variant.Hook + "\n\n" + variant.Body
			}
		}
	}

	return draft.Hook + "\n\n" + draft.Body
}
