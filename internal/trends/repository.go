package trends

import "context"

type Repository interface {
	List(ctx context.Context) ([]Signal, error)
	Create(ctx context.Context, input CreateInput) (Signal, error)
}
