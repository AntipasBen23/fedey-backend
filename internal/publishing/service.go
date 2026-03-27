package publishing

import (
	"context"
	"strings"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
	"github.com/AntipasBen23/fedey-backend/internal/performance"
	linkedinplatform "github.com/AntipasBen23/fedey-backend/internal/platform/linkedin"
	xplatform "github.com/AntipasBen23/fedey-backend/internal/platform/x"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type Service struct {
	repository         Repository
	contentService     *content.Service
	experimentService  *experiments.Service
	performanceService *performance.Service
	defaultWindows     []Window
	xClient            *xplatform.Client
	xAccountService    *xaccounts.Service
	linkedinClient     *linkedinplatform.Client
	linkedinService    *linkedinaccounts.Service
}

func NewService(repository Repository, contentService *content.Service, experimentService *experiments.Service, performanceService *performance.Service, defaultWindows []Window, xClient *xplatform.Client, xAccountService *xaccounts.Service, linkedinClient *linkedinplatform.Client, linkedinService *linkedinaccounts.Service) *Service {
	return &Service{
		repository:         repository,
		contentService:     contentService,
		experimentService:  experimentService,
		performanceService: performanceService,
		defaultWindows:     append([]Window(nil), defaultWindows...),
		xClient:            xClient,
		xAccountService:    xAccountService,
		linkedinClient:     linkedinClient,
		linkedinService:    linkedinService,
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

func (s *Service) RecommendNextTime(ctx context.Context, channel string, after time.Time) time.Time {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if after.IsZero() {
		after = time.Now().UTC()
	}

	fallbackHours := fallbackHoursFromWindows(s.defaultWindows)
	bestHours := fallbackHours
	if s.performanceService != nil {
		if hours, err := s.performanceService.BestHours(ctx, channel, fallbackHours); err == nil && len(hours) > 0 {
			bestHours = hours
		}
	}

	return nextOptimalTime(after, bestHours)
}

func (s *Service) MarkPublished(ctx context.Context, scheduleID string) (Schedule, error) {
	if strings.TrimSpace(scheduleID) == "" {
		return Schedule{}, ErrInvalidScheduleInput
	}

	schedule, err := s.repository.GetByID(ctx, strings.TrimSpace(scheduleID))
	if err != nil {
		return Schedule{}, err
	}
	draft, err := s.contentService.GetByID(ctx, schedule.DraftID)
	if err != nil {
		return Schedule{}, err
	}

	platformPostID := ""
	switch {
	case strings.EqualFold(schedule.Channel, "x"):
		if s.xClient == nil {
			return Schedule{}, ErrInvalidScheduleInput
		}

		credentials, err := s.resolveXCredentials(ctx)
		if err != nil {
			return Schedule{}, err
		}

		postID, err := s.xClient.PublishPostWithToken(ctx, credentials.AccessToken, buildPublishText(draft, schedule.VariantLabel), "")
		if err != nil {
			return Schedule{}, err
		}
		platformPostID = postID
	case strings.EqualFold(schedule.Channel, "linkedin"):
		if s.linkedinClient == nil || s.linkedinService == nil {
			return Schedule{}, ErrInvalidScheduleInput
		}

		account, err := s.linkedinService.GetActive(ctx)
		if err != nil {
			return Schedule{}, err
		}

		postID, err := s.linkedinClient.CreatePost(ctx, account.AccessToken, account.AuthorURN, buildPublishText(draft, schedule.VariantLabel))
		if err != nil {
			return Schedule{}, err
		}
		platformPostID = postID
	}

	if s.experimentService != nil && draft.ExperimentID != "" {
		_, _ = s.experimentService.UpdateStatus(ctx, draft.ExperimentID, experiments.StatusRunning)
		variant := strings.TrimSpace(schedule.VariantLabel)
		if variant == "" {
			variant = "A"
		}
		_ = s.experimentService.RecordMetric(ctx, experiments.RecordMetricInput{
			ExperimentID: draft.ExperimentID,
			Variant:      variant,
			Value:        0,
		})
	}

	return s.repository.MarkPublished(ctx, strings.TrimSpace(scheduleID), platformPostID)
}

func (s *Service) PublishDue(ctx context.Context, now time.Time) ([]Schedule, error) {
	items, err := s.repository.ListDue(ctx, now.UTC())
	if err != nil {
		return nil, err
	}

	published := make([]Schedule, 0, len(items))
	for _, item := range items {
		result, err := s.MarkPublished(ctx, item.ID)
		if err != nil {
			return published, err
		}
		published = append(published, result)
	}

	return published, nil
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

	return xaccounts.Account{}, ErrInvalidScheduleInput
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

func fallbackHoursFromWindows(windows []Window) []int {
	if len(windows) == 0 {
		return []int{9, 12, 15}
	}
	result := make([]int, 0, len(windows))
	for _, window := range windows {
		result = append(result, window.Hour)
	}
	return result
}

func nextOptimalTime(after time.Time, hours []int) time.Time {
	if len(hours) == 0 {
		hours = []int{9, 12, 15}
	}
	current := after.UTC()
	for dayOffset := 0; dayOffset < 7; dayOffset++ {
		day := current.AddDate(0, 0, dayOffset)
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		for _, hour := range hours {
			candidate := time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, time.UTC)
			if candidate.After(current) {
				return candidate
			}
		}
	}
	return current.Add(24 * time.Hour)
}
