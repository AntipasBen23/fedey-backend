package experiments

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidExperimentInput = errors.New("invalid experiment input")
	ErrInvalidStatus          = errors.New("invalid experiment status")
	ErrInvalidMetricInput     = errors.New("invalid metric event input")
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Experiment, error) {
	if strings.TrimSpace(input.HypothesisID) == "" || strings.TrimSpace(input.Metric) == "" {
		return Experiment{}, ErrInvalidExperimentInput
	}

	return s.repository.Create(ctx, CreateInput{
		HypothesisID: strings.TrimSpace(input.HypothesisID),
		Metric:       strings.TrimSpace(input.Metric),
	})
}

func (s *Service) List(ctx context.Context) ([]Experiment, error) {
	items, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return items, nil
	}

	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	summaries, err := s.repository.GetSummaries(ctx, ids)
	if err != nil {
		return nil, err
	}

	for index := range items {
		if summary, ok := summaries[items[index].ID]; ok {
			items[index].Summary = &summary
		}
	}

	return items, nil
}

func (s *Service) UpdateStatus(ctx context.Context, experimentID string, status Status) (Experiment, error) {
	if strings.TrimSpace(experimentID) == "" {
		return Experiment{}, ErrInvalidExperimentInput
	}

	if !isValidStatus(status) {
		return Experiment{}, ErrInvalidStatus
	}

	return s.repository.UpdateStatus(ctx, strings.TrimSpace(experimentID), status)
}

func isValidStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusRunning, StatusCompleted:
		return true
	default:
		return false
	}
}

func (s *Service) RecordMetric(ctx context.Context, input RecordMetricInput) error {
	if strings.TrimSpace(input.ExperimentID) == "" || strings.TrimSpace(input.Variant) == "" {
		return ErrInvalidMetricInput
	}
	if input.Value < 0 {
		return ErrInvalidMetricInput
	}

	return s.repository.RecordMetric(ctx, RecordMetricInput{
		ExperimentID: strings.TrimSpace(input.ExperimentID),
		Variant:      strings.ToUpper(strings.TrimSpace(input.Variant)),
		Value:        input.Value,
	})
}
