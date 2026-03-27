package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/community"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/performance"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
)

type Service struct {
	repository         Repository
	brandMemoryService *brandmemory.Service
	trendService       *trends.Service
	contentService     *content.Service
	publishingService  *publishing.Service
	communityService   *community.Service
	performanceService *performance.Service
	settings           Settings
}

func NewService(
	repository Repository,
	brandMemoryService *brandmemory.Service,
	trendService *trends.Service,
	contentService *content.Service,
	publishingService *publishing.Service,
	communityService *community.Service,
	performanceService *performance.Service,
	settings Settings,
) *Service {
	return &Service{
		repository:         repository,
		brandMemoryService: brandMemoryService,
		trendService:       trendService,
		contentService:     contentService,
		publishingService:  publishingService,
		communityService:   communityService,
		performanceService: performanceService,
		settings:           settings,
	}
}

func (s *Service) List(ctx context.Context) ([]Run, error) {
	return s.repository.List(ctx)
}

func (s *Service) Settings() Settings {
	return s.settings
}

func (s *Service) Run(ctx context.Context, triggeredBy string) (Run, error) {
	now := time.Now().UTC()
	run := Run{
		ID:          "run-" + uuid.NewString(),
		Status:      "completed",
		TriggeredBy: triggeredBy,
		CreatedAt:   now,
	}

	publishedSchedules, err := s.publishingService.PublishDue(ctx, now)
	if err != nil {
		return Run{}, fmt.Errorf("publish due schedules: %w", err)
	}
	run.PostsPublished = len(publishedSchedules)

	performanceMetricsRecorded := 0
	if s.publishingService != nil {
		count, err := s.publishingService.SyncPublishedPerformance(ctx)
		if err == nil {
			performanceMetricsRecorded = count
		}
	}

	if s.trendService != nil && s.brandMemoryService != nil {
		profile, err := s.brandMemoryService.Get(ctx)
		if err == nil {
			signals, err := s.trendService.IngestDefaults(ctx, profile)
			if err == nil {
				run.SignalsIngested = len(signals)
			}
		}
	}

	performanceSnapshots := 0
	if s.performanceService != nil {
		result, err := s.performanceService.SyncConnectedAccounts(ctx)
		if err == nil {
			performanceSnapshots = result.XSnapshots + result.LinkedInSnapshots
		}
	}

	generatedDrafts, err := s.contentService.Generate(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("generate drafts: %w", err)
	}
	run.DraftsGenerated = len(generatedDrafts)

	mentionsSynced, err := s.communityService.SyncXMentions(ctx)
	if err == nil {
		run.MentionsSynced = mentionsSynced
	}
	linkedinCommentsSynced, err := s.communityService.SyncLinkedInComments(ctx)
	if err == nil {
		run.MentionsSynced += linkedinCommentsSynced
	}

	allDrafts, err := s.contentService.List(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("list drafts: %w", err)
	}
	allSchedules, err := s.publishingService.List(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("list schedules: %w", err)
	}

	unscheduledDraft := findFirstUnscheduledDraft(allDrafts, allSchedules)
	if unscheduledDraft != nil {
		variantLabel := ""
		if len(unscheduledDraft.Variants) > 1 {
			variantLabel = unscheduledDraft.Variants[1].Label
		}

		scheduledFor := s.publishingService.RecommendNextTime(ctx, unscheduledDraft.Channel, now, "standard")
		if nextWindow, ok := nextWindowAfter(now, s.settings.Windows); ok && nextWindow.After(now) && scheduledFor.Before(now.Add(5*time.Minute)) {
			scheduledFor = nextWindow
		}

		_, err := s.publishingService.Create(ctx, publishing.CreateInput{
			DraftID:      unscheduledDraft.ID,
			VariantLabel: variantLabel,
			Channel:      unscheduledDraft.Channel,
			ScheduledFor: scheduledFor,
		})
		if err != nil {
			return Run{}, fmt.Errorf("create schedule: %w", err)
		}
		run.SchedulesCreated = 1
	}

	inboxItems, err := s.communityService.List(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("list community inbox: %w", err)
	}
	for _, item := range inboxItems {
		if item.Status != community.StatusPending {
			continue
		}

		if _, err := s.communityService.DraftReply(ctx, item.ID); err != nil {
			return Run{}, fmt.Errorf("draft community reply: %w", err)
		}
		run.RepliesDrafted++
	}

	run.Notes = fmt.Sprintf(
		"Published %d posts, recorded %d live performance updates, ingested %d live signals, synced %d performance snapshots, generated %d drafts, created %d schedule, synced %d mentions, drafted %d replies.",
		run.PostsPublished,
		performanceMetricsRecorded,
		run.SignalsIngested,
		performanceSnapshots,
		run.DraftsGenerated,
		run.SchedulesCreated,
		run.MentionsSynced,
		run.RepliesDrafted,
	)

	if err := s.repository.Create(ctx, run); err != nil {
		return Run{}, err
	}

	return run, nil
}

func nextWindowAfter(now time.Time, windows []publishing.Window) (time.Time, bool) {
	if len(windows) == 0 {
		return time.Time{}, false
	}

	current := now.UTC()
	for dayOffset := 0; dayOffset < 2; dayOffset++ {
		base := current.AddDate(0, 0, dayOffset)
		for _, window := range windows {
			candidate := time.Date(base.Year(), base.Month(), base.Day(), window.Hour, window.Minute, 0, 0, time.UTC)
			if candidate.After(current) {
				return candidate, true
			}
		}
	}

	return time.Time{}, false
}

func findFirstUnscheduledDraft(drafts []content.Draft, schedules []publishing.Schedule) *content.Draft {
	scheduledDrafts := make(map[string]struct{}, len(schedules))
	for _, schedule := range schedules {
		scheduledDrafts[schedule.DraftID] = struct{}{}
	}

	for index := range drafts {
		if _, exists := scheduledDrafts[drafts[index].ID]; exists {
			continue
		}

		return &drafts[index]
	}

	return nil
}
