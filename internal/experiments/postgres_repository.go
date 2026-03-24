package experiments

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, input CreateInput) (Experiment, error) {
	now := time.Now().UTC()
	id := "exp-" + uuid.NewString()
	experiment := Experiment{
		ID:           id,
		HypothesisID: input.HypothesisID,
		Metric:       input.Metric,
		Status:       StatusDraft,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	const query = `
		INSERT INTO experiments (id, hypothesis_id, metric, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		experiment.ID,
		experiment.HypothesisID,
		experiment.Metric,
		experiment.Status,
		experiment.CreatedAt,
		experiment.UpdatedAt,
	)
	if err != nil {
		return Experiment{}, fmt.Errorf("insert experiment: %w", err)
	}

	return experiment, nil
}

func (r *PostgresRepository) List(ctx context.Context) ([]Experiment, error) {
	const query = `
		SELECT id, hypothesis_id, metric, status, created_at, updated_at
		FROM experiments
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list experiments: %w", err)
	}
	defer rows.Close()

	experiments := make([]Experiment, 0)
	for rows.Next() {
		var experiment Experiment
		if err := rows.Scan(
			&experiment.ID,
			&experiment.HypothesisID,
			&experiment.Metric,
			&experiment.Status,
			&experiment.CreatedAt,
			&experiment.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan experiment: %w", err)
		}

		experiments = append(experiments, experiment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experiments: %w", err)
	}

	return experiments, nil
}

func (r *PostgresRepository) UpdateStatus(
	ctx context.Context,
	experimentID string,
	status Status,
) (Experiment, error) {
	const query = `
		UPDATE experiments
		SET status = $1, updated_at = $2
		WHERE id = $3
		RETURNING id, hypothesis_id, metric, status, created_at, updated_at
	`

	var experiment Experiment
	err := r.pool.QueryRow(
		ctx,
		query,
		status,
		time.Now().UTC(),
		experimentID,
	).Scan(
		&experiment.ID,
		&experiment.HypothesisID,
		&experiment.Metric,
		&experiment.Status,
		&experiment.CreatedAt,
		&experiment.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Experiment{}, ErrExperimentNotFound
	}
	if err != nil {
		return Experiment{}, fmt.Errorf("update experiment status: %w", err)
	}

	return experiment, nil
}

func (r *PostgresRepository) RecordMetric(ctx context.Context, input RecordMetricInput) error {
	const existsQuery = `SELECT EXISTS (SELECT 1 FROM experiments WHERE id = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, existsQuery, input.ExperimentID).Scan(&exists); err != nil {
		return fmt.Errorf("check experiment existence: %w", err)
	}
	if !exists {
		return ErrExperimentNotFound
	}

	const query = `
		INSERT INTO analytics_events (id, experiment_id, variant, value, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		"evt-"+uuid.NewString(),
		input.ExperimentID,
		strings.ToUpper(input.Variant),
		input.Value,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert analytics event: %w", err)
	}

	return nil
}

func (r *PostgresRepository) GetSummaries(ctx context.Context, experimentIDs []string) (map[string]Summary, error) {
	if len(experimentIDs) == 0 {
		return map[string]Summary{}, nil
	}

	const query = `
		SELECT experiment_id, variant, COUNT(*) AS events_count, COALESCE(SUM(value), 0) AS total_value
		FROM analytics_events
		WHERE experiment_id = ANY($1)
		GROUP BY experiment_id, variant
		ORDER BY experiment_id, total_value DESC
	`

	rows, err := r.pool.Query(ctx, query, experimentIDs)
	if err != nil {
		return nil, fmt.Errorf("query analytics summaries: %w", err)
	}
	defer rows.Close()

	grouped := make(map[string][]RecordMetricInput)
	for rows.Next() {
		var experimentID string
		var variant string
		var eventsCount int
		var totalValue float64
		if err := rows.Scan(&experimentID, &variant, &eventsCount, &totalValue); err != nil {
			return nil, fmt.Errorf("scan analytics summary: %w", err)
		}

		average := 0.0
		if eventsCount > 0 {
			average = totalValue / float64(eventsCount)
		}

		for range eventsCount {
			grouped[experimentID] = append(grouped[experimentID], RecordMetricInput{
				ExperimentID: experimentID,
				Variant:      variant,
				Value:        average,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analytics summaries: %w", err)
	}

	summaries := make(map[string]Summary, len(grouped))
	for experimentID, events := range grouped {
		summaries[experimentID] = buildSummary(events)
	}

	return summaries, nil
}
