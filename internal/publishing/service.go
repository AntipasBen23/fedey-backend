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
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	if s.performanceService == nil {
		return items, nil
	}
	for index := range items {
		if items[index].Status != StatusPublished || strings.TrimSpace(items[index].PlatformPostID) == "" {
			continue
		}
		timeline, err := s.performanceTimeline(ctx, items[index].Channel, items[index].PlatformPostID)
		if err != nil {
			continue
		}
		items[index].Timeline = timeline
	}
	return items, nil
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
		QueueProfile: normalizeQueueProfile(input.QueueProfile),
		ScheduledFor: input.ScheduledFor,
	})
}

func (s *Service) RecommendNextTime(ctx context.Context, channel string, after time.Time, queueProfile string) time.Time {
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
	schedules, _ := s.repository.List(ctx)
	return applyQueuePolicies(nextOptimalTime(after, bestHours), channel, normalizeQueueProfile(queueProfile), schedules)
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

func (s *Service) SyncPublishedPerformance(ctx context.Context) (int, error) {
	if s.performanceService == nil || s.experimentService == nil {
		return 0, nil
	}

	schedules, err := s.repository.List(ctx)
	if err != nil {
		return 0, err
	}

	recorded := 0
	for _, schedule := range schedules {
		if schedule.Status != StatusPublished || strings.TrimSpace(schedule.PlatformPostID) == "" {
			continue
		}

		draft, err := s.contentService.GetByID(ctx, schedule.DraftID)
		if err != nil || strings.TrimSpace(draft.ExperimentID) == "" {
			continue
		}

		_, delta, err := s.performanceService.CapturePublishedPost(ctx, schedule.Channel, schedule.PlatformPostID)
		if err != nil {
			continue
		}

		deltaValue := float64(delta.Likes + delta.Replies + delta.Quotes + delta.Comments)
		if deltaValue <= 0 {
			_, _ = s.repository.MarkPerformanceSynced(ctx, schedule.ID, time.Now().UTC())
			continue
		}

		variant := strings.TrimSpace(schedule.VariantLabel)
		if variant == "" {
			variant = "A"
		}
		if err := s.experimentService.RecordMetric(ctx, experiments.RecordMetricInput{
			ExperimentID: draft.ExperimentID,
			Variant:      variant,
			Value:        deltaValue,
		}); err == nil {
			recorded++
			_, _ = s.repository.MarkPerformanceSynced(ctx, schedule.ID, time.Now().UTC())
		}
	}

	_, _ = s.AutoRescheduleUnderperforming(ctx)
	return recorded, nil
}

func (s *Service) AutoRescheduleUnderperforming(ctx context.Context) (int, error) {
	if s.performanceService == nil {
		return 0, nil
	}
	schedules, err := s.repository.List(ctx)
	if err != nil {
		return 0, err
	}

	rescheduled := 0
	underperforming := map[string]map[int]struct{}{}
	for _, channel := range []string{"x", "linkedin"} {
		hours, err := s.underperformingHours(ctx, channel)
		if err != nil || len(hours) == 0 {
			continue
		}
		underperforming[channel] = hours
	}

	for _, schedule := range schedules {
		if schedule.Status != StatusScheduled {
			continue
		}
		hours := underperforming[strings.ToLower(schedule.Channel)]
		if len(hours) == 0 {
			continue
		}
		if _, bad := hours[schedule.ScheduledFor.UTC().Hour()]; !bad {
			continue
		}
		next := s.RecommendNextTime(ctx, schedule.Channel, schedule.ScheduledFor.Add(3*time.Hour), schedule.QueueProfile)
		if next.Equal(schedule.ScheduledFor) {
			continue
		}
		if _, err := s.repository.UpdateScheduledFor(ctx, schedule.ID, next); err == nil {
			rescheduled++
		}
	}
	return rescheduled, nil
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

func (s *Service) performanceTimeline(ctx context.Context, channel, platformPostID string) ([]PerformancePoint, error) {
	snapshots, err := s.performanceService.Timeline(ctx, strings.ToLower(strings.TrimSpace(channel)), strings.TrimSpace(platformPostID), 20)
	if err != nil {
		return nil, err
	}
	points := make([]PerformancePoint, 0, len(snapshots))
	for _, item := range snapshots {
		points = append(points, PerformancePoint{
			CapturedAt:      item.CapturedAt,
			Likes:           item.LikeCount,
			Replies:         item.ReplyCount,
			Quotes:          item.QuoteCount,
			Comments:        item.CommentCount,
			TotalEngagement: item.LikeCount + item.ReplyCount + item.QuoteCount + item.CommentCount,
		})
	}
	return points, nil
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

func applyQueuePolicies(candidate time.Time, channel string, queueProfile string, schedules []Schedule) time.Time {
	current := candidate.UTC()
	for {
		conflict := false
		for _, item := range schedules {
			if item.Status != StatusScheduled && item.Status != StatusPublished {
				continue
			}
			if item.ScheduledFor.IsZero() {
				continue
			}
			if withinGap(current, item.ScheduledFor.UTC(), sameChannelGap(channel, item.Channel, queueProfile, item.QueueProfile)) {
				current = item.ScheduledFor.UTC().Add(sameChannelGap(channel, item.Channel, queueProfile, item.QueueProfile))
				conflict = true
				break
			}
			if withinGap(current, item.ScheduledFor.UTC(), crossChannelGap(channel, item.Channel, queueProfile, item.QueueProfile)) {
				current = item.ScheduledFor.UTC().Add(crossChannelGap(channel, item.Channel, queueProfile, item.QueueProfile))
				conflict = true
				break
			}
		}
		if !conflict {
			return current
		}
	}
}

func sameChannelGap(left, right, leftProfile, rightProfile string) time.Duration {
	if !strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right)) {
		return 0
	}
	profile := dominantQueueProfile(leftProfile, rightProfile)
	switch strings.ToLower(strings.TrimSpace(left)) {
	case "linkedin":
		if profile == "new" {
			return 24 * time.Hour
		}
		return 18 * time.Hour
	default:
		if profile == "new" {
			return 12 * time.Hour
		}
		return 6 * time.Hour
	}
}

func crossChannelGap(left, right, leftProfile, rightProfile string) time.Duration {
	if strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right)) {
		return 0
	}
	if dominantQueueProfile(leftProfile, rightProfile) == "new" {
		return 4 * time.Hour
	}
	return 2 * time.Hour
}

func withinGap(left, right time.Time, gap time.Duration) bool {
	if gap <= 0 {
		return false
	}
	diff := left.Sub(right)
	if diff < 0 {
		diff = -diff
	}
	return diff < gap
}

func normalizeQueueProfile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "new":
		return "new"
	case "existing":
		return "existing"
	default:
		return "standard"
	}
}

func dominantQueueProfile(left, right string) string {
	if normalizeQueueProfile(left) == "new" || normalizeQueueProfile(right) == "new" {
		return "new"
	}
	if normalizeQueueProfile(left) == "existing" || normalizeQueueProfile(right) == "existing" {
		return "existing"
	}
	return "standard"
}

func (s *Service) underperformingHours(ctx context.Context, channel string) (map[int]struct{}, error) {
	snapshots, err := s.performanceService.Recent(ctx, strings.ToLower(strings.TrimSpace(channel)), 96)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	type bucket struct {
		total int
		count int
	}
	buckets := make(map[int]bucket)
	bestAverage := 0.0
	for _, item := range snapshots {
		hour := item.PublishedAt.UTC().Hour()
		current := buckets[hour]
		current.total += item.LikeCount + item.ReplyCount + item.QuoteCount + item.CommentCount
		current.count++
		buckets[hour] = current
		avg := float64(current.total) / float64(current.count)
		if avg > bestAverage {
			bestAverage = avg
		}
	}
	if bestAverage == 0 {
		return nil, nil
	}
	result := make(map[int]struct{})
	for hour, item := range buckets {
		avg := float64(item.total) / float64(item.count)
		if avg <= bestAverage*0.55 {
			result[hour] = struct{}{}
		}
	}
	return result, nil
}
