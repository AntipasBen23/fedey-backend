package content

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Draft, error) {
	const query = `
		SELECT id, channel, hook, body, rationale, source_trend, status, created_at
		FROM content_drafts
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list content drafts: %w", err)
	}
	defer rows.Close()

	var drafts []Draft
	for rows.Next() {
		var draft Draft
		if err := rows.Scan(
			&draft.ID,
			&draft.Channel,
			&draft.Hook,
			&draft.Body,
			&draft.Rationale,
			&draft.SourceTrend,
			&draft.Status,
			&draft.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan content draft: %w", err)
		}

		drafts = append(drafts, draft)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content drafts: %w", err)
	}

	return drafts, nil
}

func (r *PostgresRepository) SaveBatch(ctx context.Context, drafts []Draft) error {
	batch := &pgx.Batch{}
	for _, draft := range drafts {
		batch.Queue(
			`INSERT INTO content_drafts (id, channel, hook, body, rationale, source_trend, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			draft.ID,
			draft.Channel,
			draft.Hook,
			draft.Body,
			draft.Rationale,
			draft.SourceTrend,
			draft.Status,
			draft.CreatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range drafts {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert content draft: %w", err)
		}
	}

	return nil
}
