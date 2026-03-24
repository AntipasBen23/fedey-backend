package experiments

import "context"

type Repository interface {
	Create(ctx context.Context, input CreateInput) (Experiment, error)
	List(ctx context.Context) ([]Experiment, error)
	UpdateStatus(ctx context.Context, experimentID string, status Status) (Experiment, error)
	RecordMetric(ctx context.Context, input RecordMetricInput) error
	GetSummaries(ctx context.Context, experimentIDs []string) (map[string]Summary, error)
}
