package trends

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	signals []Signal
}

func NewMemoryRepository() *MemoryRepository {
	now := time.Now().UTC()

	return &MemoryRepository{
		signals: []Signal{
			{
				ID:         "trd-1",
				Topic:      "AI employees replacing specialist roles",
				Source:     "x",
				Angle:      "Operators are asking what a reliable AI social manager stack looks like.",
				Velocity:   84,
				Relevance:  0.92,
				ObservedAt: now.Add(-2 * time.Hour),
			},
			{
				ID:         "trd-2",
				Topic:      "Founders documenting their systems publicly",
				Source:     "linkedin",
				Angle:      "Process breakdowns and build-in-public posts are earning high saves.",
				Velocity:   67,
				Relevance:  0.81,
				ObservedAt: now.Add(-4 * time.Hour),
			},
		},
	}
}

func (r *MemoryRepository) List(_ context.Context) ([]Signal, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := append([]Signal(nil), r.signals...)
	slices.SortFunc(items, func(left, right Signal) int {
		if left.Relevance == right.Relevance {
			return right.ObservedAt.Compare(left.ObservedAt)
		}
		if left.Relevance > right.Relevance {
			return -1
		}
		return 1
	})

	return items, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput) (Signal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	signal := Signal{
		ID:         fmt.Sprintf("trd-%d", time.Now().UTC().UnixNano()),
		Topic:      input.Topic,
		Source:     input.Source,
		Angle:      input.Angle,
		Velocity:   input.Velocity,
		Relevance:  input.Relevance,
		ObservedAt: time.Now().UTC(),
	}

	r.signals = append(r.signals, signal)
	return signal, nil
}
