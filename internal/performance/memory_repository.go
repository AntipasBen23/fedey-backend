package performance

import (
	"context"
	"slices"
	"strings"
	"sync"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	snapshots []Snapshot
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{snapshots: []Snapshot{}}
}

func (r *MemoryRepository) SaveBatch(_ context.Context, snapshots []Snapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots = append(r.snapshots, snapshots...)
	return nil
}

func (r *MemoryRepository) ListRecent(_ context.Context, platform string, limit int) ([]Snapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Snapshot, 0, len(r.snapshots))
	for _, item := range r.snapshots {
		if platform != "" && !strings.EqualFold(item.Platform, platform) {
			continue
		}
		items = append(items, item)
	}

	slices.SortFunc(items, func(left, right Snapshot) int {
		return right.CapturedAt.Compare(left.CapturedAt)
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
