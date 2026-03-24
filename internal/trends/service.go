package trends

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidSignalInput = errors.New("invalid trend signal input")

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context) ([]Signal, error) {
	return s.repository.List(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Signal, error) {
	if strings.TrimSpace(input.Topic) == "" ||
		strings.TrimSpace(input.Source) == "" ||
		strings.TrimSpace(input.Angle) == "" ||
		input.Velocity < 0 ||
		input.Relevance < 0 ||
		input.Relevance > 1 {
		return Signal{}, ErrInvalidSignalInput
	}

	return s.repository.Create(ctx, CreateInput{
		Topic:     strings.TrimSpace(input.Topic),
		Source:    strings.TrimSpace(input.Source),
		Angle:     strings.TrimSpace(input.Angle),
		Velocity:  input.Velocity,
		Relevance: input.Relevance,
	})
}
