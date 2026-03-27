package onboarding

import (
	"context"
	"slices"
	"sync"
)

type MemoryRepository struct {
	mu        sync.RWMutex
	sessions  map[string]Session
	questions map[string][]Question
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		sessions:  make(map[string]Session),
		questions: make(map[string][]Question),
	}
}

func (r *MemoryRepository) List(_ context.Context) ([]Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]Session, 0, len(r.sessions))
	for _, session := range r.sessions {
		session.Questions = append([]Question(nil), r.questions[session.ID]...)
		items = append(items, session)
	}
	slices.SortFunc(items, func(left, right Session) int {
		return right.CreatedAt.Compare(left.CreatedAt)
	})

	return items, nil
}

func (r *MemoryRepository) CreateSession(_ context.Context, session Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}

func (r *MemoryRepository) UpdateSession(_ context.Context, session Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[session.ID]; !ok {
		return ErrSessionNotFound
	}
	r.sessions[session.ID] = session
	return nil
}

func (r *MemoryRepository) GetSession(_ context.Context, sessionID string) (Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	session.Questions = append([]Question(nil), r.questions[sessionID]...)
	return session, nil
}

func (r *MemoryRepository) ListQuestions(_ context.Context, sessionID string) ([]Question, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Question(nil), r.questions[sessionID]...), nil
}

func (r *MemoryRepository) UpsertQuestion(_ context.Context, question Question) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.questions[question.SessionID]
	for index := range items {
		if items[index].ID == question.ID {
			items[index] = question
			r.questions[question.SessionID] = items
			return nil
		}
	}
	r.questions[question.SessionID] = append(items, question)
	return nil
}

func (r *MemoryRepository) GetQuestion(_ context.Context, sessionID, questionID string) (Question, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, question := range r.questions[sessionID] {
		if question.ID == questionID {
			return question, nil
		}
	}
	return Question{}, ErrQuestionNotFound
}
