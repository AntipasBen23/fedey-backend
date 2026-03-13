package experiments

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidExperimentInput = errors.New("invalid experiment input")
	ErrInvalidStatus          = errors.New("invalid experiment status")
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
	return s.repository.List(ctx)
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
