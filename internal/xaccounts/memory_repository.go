package xaccounts

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	account Account
	states  map[string]OAuthState
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		states: make(map[string]OAuthState),
	}
}

func (r *MemoryRepository) GetActive(_ context.Context) (Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.account.UserID == "" {
		return Account{}, ErrAccountNotConnected
	}

	return r.account, nil
}

func (r *MemoryRepository) UpsertActive(_ context.Context, account Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.account = account
	return nil
}

func (r *MemoryRepository) SaveState(_ context.Context, state OAuthState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[state.State] = state
	return nil
}

func (r *MemoryRepository) GetState(_ context.Context, state string) (OAuthState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.states[state]
	if !ok {
		return OAuthState{}, ErrOAuthStateNotFound
	}
	return value, nil
}

func (r *MemoryRepository) DeleteState(_ context.Context, state string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.states, state)
	return nil
}
