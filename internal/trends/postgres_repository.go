package trends

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Signal, error) {
	const query = `
		SELECT id, topic, source, angle, velocity, relevance, observed_at
		FROM trend_signals
		ORDER BY relevance DESC, observed_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list trend signals: %w", err)
	}
	defer rows.Close()

	items := make([]Signal, 0)
	for rows.Next() {
		var signal Signal
		if err := rows.Scan(
			&signal.ID,
			&signal.Topic,
			&signal.Source,
			&signal.Angle,
			&signal.Velocity,
			&signal.Relevance,
			&signal.ObservedAt,
		); err != nil {
			return nil, fmt.Errorf("scan trend signal: %w", err)
		}

		items = append(items, signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trend signals: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input CreateInput) (Signal, error) {
	const query = `
		INSERT INTO trend_signals (id, topic, source, angle, velocity, relevance, observed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, topic, source, angle, velocity, relevance, observed_at
	`

	var signal Signal
	err := r.pool.QueryRow(
		ctx,
		query,
		"trd-"+uuid.NewString(),
		input.Topic,
		input.Source,
		input.Angle,
		input.Velocity,
		input.Relevance,
		time.Now().UTC(),
	).Scan(
		&signal.ID,
		&signal.Topic,
		&signal.Source,
		&signal.Angle,
		&signal.Velocity,
		&signal.Relevance,
		&signal.ObservedAt,
	)
	if err != nil {
		return Signal{}, fmt.Errorf("create trend signal: %w", err)
	}

	return signal, nil
}
