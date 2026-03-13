package experiments

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	byID    map[string]Experiment
	ordered []string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byID:    make(map[string]Experiment),
		ordered: make([]string, 0),
	}
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput) (Experiment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	id := fmt.Sprintf("exp-%d", now.UnixNano())
	experiment := Experiment{
		ID:           id,
		HypothesisID: input.HypothesisID,
		Metric:       input.Metric,
		Status:       StatusDraft,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	r.byID[id] = experiment
	r.ordered = append(r.ordered, id)

	return experiment, nil
}

func (r *MemoryRepository) List(_ context.Context) ([]Experiment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Experiment, 0, len(r.ordered))
	for _, id := range r.ordered {
		result = append(result, r.byID[id])
	}
	slices.Reverse(result)

	return result, nil
}

func (r *MemoryRepository) UpdateStatus(_ context.Context, experimentID string, status Status) (Experiment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	experiment, ok := r.byID[experimentID]
	if !ok {
		return Experiment{}, ErrExperimentNotFound
	}

	experiment.Status = status
	experiment.UpdatedAt = time.Now().UTC()
	r.byID[experimentID] = experiment

	return experiment, nil
}
