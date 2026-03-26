package automation

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AntipasBen23/fedey-backend/internal/community"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
)

type Service struct {
	repository        Repository
	contentService    *content.Service
	publishingService *publishing.Service
	communityService  *community.Service
}

func NewService(
	repository Repository,
	contentService *content.Service,
	publishingService *publishing.Service,
	communityService *community.Service,
) *Service {
	return &Service{
		repository:        repository,
		contentService:    contentService,
		publishingService: publishingService,
		communityService:  communityService,
	}
}

func (s *Service) List(ctx context.Context) ([]Run, error) {
	return s.repository.List(ctx)
}

func (s *Service) Run(ctx context.Context, triggeredBy string) (Run, error) {
	run := Run{
		ID:          "run-" + uuid.NewString(),
		Status:      "completed",
		TriggeredBy: triggeredBy,
		CreatedAt:   time.Now().UTC(),
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

		_, err := s.publishingService.Create(ctx, publishing.CreateInput{
			DraftID:      unscheduledDraft.ID,
			VariantLabel: variantLabel,
			Channel:      unscheduledDraft.Channel,
			ScheduledFor: time.Now().UTC().Add(30 * time.Minute),
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
		"Generated %d drafts, created %d schedule, synced %d mentions, drafted %d replies.",
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
