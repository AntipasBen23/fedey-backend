package brandmemory

import (
	"context"
	"errors"
	"strings"
)

var ErrInvalidProfileInput = errors.New("invalid brand profile input")

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Get(ctx context.Context) (Profile, error) {
	return s.repository.Get(ctx)
}

func (s *Service) Upsert(ctx context.Context, input UpsertInput) (Profile, error) {
	if strings.TrimSpace(input.BrandName) == "" ||
		strings.TrimSpace(input.Tone) == "" ||
		strings.TrimSpace(input.Audience) == "" {
		return Profile{}, ErrInvalidProfileInput
	}

	return s.repository.Upsert(ctx, UpsertInput{
		BrandName:  strings.TrimSpace(input.BrandName),
		Tone:       strings.TrimSpace(input.Tone),
		Audience:   strings.TrimSpace(input.Audience),
		Pillars:    normalizeList(input.Pillars),
		Guardrails: normalizeList(input.Guardrails),
	})
}

func normalizeList(items []string) []string {
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}

		normalized = append(normalized, value)
	}

	return normalized
}
