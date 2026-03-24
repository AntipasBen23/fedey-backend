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
	metrics map[string][]RecordMetricInput
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byID:    make(map[string]Experiment),
		ordered: make([]string, 0),
		metrics: make(map[string][]RecordMetricInput),
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

func (r *MemoryRepository) RecordMetric(_ context.Context, input RecordMetricInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[input.ExperimentID]; !ok {
		return ErrExperimentNotFound
	}

	r.metrics[input.ExperimentID] = append(r.metrics[input.ExperimentID], input)
	return nil
}

func (r *MemoryRepository) GetSummaries(_ context.Context, experimentIDs []string) (map[string]Summary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summaries := make(map[string]Summary, len(experimentIDs))
	for _, experimentID := range experimentIDs {
		events := r.metrics[experimentID]
		if len(events) == 0 {
			continue
		}

		summaries[experimentID] = buildSummary(events)
	}

	return summaries, nil
}
