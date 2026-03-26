package publishing

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	schedules []Schedule
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{schedules: []Schedule{}}
}

func (r *MemoryRepository) List(_ context.Context) ([]Schedule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := append([]Schedule(nil), r.schedules...)
	slices.SortFunc(items, func(left, right Schedule) int {
		return left.ScheduledFor.Compare(right.ScheduledFor)
	})

	return items, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput) (Schedule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	item := Schedule{
		ID:           fmt.Sprintf("sch-%d", now.UnixNano()),
		DraftID:      input.DraftID,
		VariantLabel: input.VariantLabel,
		Channel:      input.Channel,
		ScheduledFor: input.ScheduledFor.UTC(),
		Status:       StatusScheduled,
		CreatedAt:    now,
	}

	r.schedules = append(r.schedules, item)
	return item, nil
}

func (r *MemoryRepository) GetByID(_ context.Context, scheduleID string) (Schedule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, item := range r.schedules {
		if item.ID == scheduleID {
			return item, nil
		}
	}

	return Schedule{}, ErrScheduleNotFound
}

func (r *MemoryRepository) MarkPublished(_ context.Context, scheduleID string, platformPostID string) (Schedule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for index := range r.schedules {
		if r.schedules[index].ID != scheduleID {
			continue
		}

		r.schedules[index].Status = StatusPublished
		r.schedules[index].PlatformPostID = platformPostID
		r.schedules[index].PublishedAt = time.Now().UTC()
		return r.schedules[index], nil
	}

	return Schedule{}, ErrScheduleNotFound
}
