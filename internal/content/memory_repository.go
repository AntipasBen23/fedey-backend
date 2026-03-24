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

func (r *MemoryRepository) SaveBatch(_ context.Context, drafts []Draft) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.drafts = append(r.drafts, drafts...)
	return nil
}
