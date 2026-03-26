package publishing

import "context"

type Repository interface {
	List(ctx context.Context) ([]Schedule, error)
	Create(ctx context.Context, input CreateInput) (Schedule, error)
	GetByID(ctx context.Context, scheduleID string) (Schedule, error)
	MarkPublished(ctx context.Context, scheduleID string, platformPostID string) (Schedule, error)
}
