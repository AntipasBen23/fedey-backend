package onboarding

import "context"

type Repository interface {
	List(ctx context.Context) ([]Session, error)
	CreateSession(ctx context.Context, session Session) error
	UpdateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, sessionID string) (Session, error)
	ListQuestions(ctx context.Context, sessionID string) ([]Question, error)
	UpsertQuestion(ctx context.Context, question Question) error
	GetQuestion(ctx context.Context, sessionID, questionID string) (Question, error)
}
