package automation

import (
	"context"
	"slices"
	"sync"
)

type MemoryRepository struct {
	mu   sync.RWMutex
	runs []Run
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{runs: []Run{}}
}

func (r *MemoryRepository) List(_ context.Context) ([]Run, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := append([]Run(nil), r.runs...)
	slices.SortFunc(items, func(left, right Run) int {
		return right.CreatedAt.Compare(left.CreatedAt)
	})
	return items, nil
}

func (r *MemoryRepository) Create(_ context.Context, run Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.runs = append(r.runs, run)
	return nil
}
