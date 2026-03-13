package experiments

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &PostgresRepository{pool: pool}, nil
}

func (r *PostgresRepository) Close() {
	r.pool.Close()
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
