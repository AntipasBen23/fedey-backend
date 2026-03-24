package brandmemory

import (
	"context"
	"sync"
	"time"
)

const defaultProfileID = "default"

type MemoryRepository struct {
	mu      sync.RWMutex
	profile Profile
}

func NewMemoryRepository() *MemoryRepository {
	now := time.Now().UTC()

	return &MemoryRepository{
		profile: Profile{
			ID:         defaultProfileID,
			BrandName:  "Fedey",
			Tone:       "Clear, strategic, confident, and human.",
			Audience:   "Founders, creators, and businesses that want AI employees for growth.",
			Pillars:    []string{"AI agents", "social growth systems", "automation strategy"},
			Guardrails: []string{"No spammy hooks", "No misleading claims", "No off-brand slang"},
			UpdatedAt:  now,
		},
	}
}

func (r *MemoryRepository) Get(_ context.Context) (Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.profile, nil
}

func (r *MemoryRepository) Upsert(_ context.Context, input UpsertInput) (Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.profile = Profile{
		ID:         defaultProfileID,
		BrandName:  input.BrandName,
		Tone:       input.Tone,
		Audience:   input.Audience,
		Pillars:    append([]string(nil), input.Pillars...),
		Guardrails: append([]string(nil), input.Guardrails...),
		UpdatedAt:  time.Now().UTC(),
	}

	return r.profile, nil
}
