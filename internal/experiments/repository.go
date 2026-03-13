package experiments

import "context"

type Repository interface {
	Create(ctx context.Context, input CreateInput) (Experiment, error)
	List(ctx context.Context) ([]Experiment, error)
	UpdateStatus(ctx context.Context, experimentID string, status Status) (Experiment, error)
}
