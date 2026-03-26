package automation

import "context"

type Repository interface {
	List(ctx context.Context) ([]Run, error)
	Create(ctx context.Context, run Run) error
}
