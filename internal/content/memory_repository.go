package content

import (
	"context"
	"slices"
	"sync"
)

type MemoryRepository struct {
	mu     sync.RWMutex
	drafts []Draft
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		drafts: []Draft{},
	}
}

func (r *MemoryRepository) List(_ context.Context) ([]Draft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := append([]Draft(nil), r.drafts...)
	slices.Reverse(items)
	return items, nil
}

func (r *MemoryRepository) GetByID(_ context.Context, draftID string) (Draft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, draft := range r.drafts {
		if draft.ID == draftID {
			return draft, nil
		}
	}

	return Draft{}, ErrDraftNotFound
}

func (r *MemoryRepository) SaveBatch(_ context.Context, drafts []Draft) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drafts = append(r.drafts, drafts...)
	return nil
}

func (r *MemoryRepository) Update(_ context.Context, updated Draft) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for index := range r.drafts {
		if r.drafts[index].ID == updated.ID {
			r.drafts[index] = updated
			return nil
		}
	}

	return ErrDraftNotFound
}

func (r *MemoryRepository) SaveVariants(_ context.Context, experimentID string, variants []Variant) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for index := range r.drafts {
		if r.drafts[index].ExperimentID == experimentID {
			r.drafts[index].Variants = append([]Variant(nil), variants...)
			return nil
		}
	}

	return nil
}
