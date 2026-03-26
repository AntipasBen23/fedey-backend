package automation

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Run, error) {
	const query = `
		SELECT id, status, drafts_generated, schedules_created, posts_published, signals_ingested, mentions_synced, replies_drafted, triggered_by, notes, created_at
		FROM automation_runs
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list automation runs: %w", err)
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var run Run
		if err := rows.Scan(
			&run.ID,
			&run.Status,
			&run.DraftsGenerated,
			&run.SchedulesCreated,
			&run.PostsPublished,
			&run.SignalsIngested,
			&run.MentionsSynced,
			&run.RepliesDrafted,
			&run.TriggeredBy,
			&run.Notes,
			&run.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan automation run: %w", err)
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate automation runs: %w", err)
	}

	return runs, nil
}

func (r *PostgresRepository) Create(ctx context.Context, run Run) error {
	const query = `
		INSERT INTO automation_runs (id, status, drafts_generated, schedules_created, posts_published, signals_ingested, mentions_synced, replies_drafted, triggered_by, notes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		run.ID,
		run.Status,
		run.DraftsGenerated,
		run.SchedulesCreated,
		run.PostsPublished,
		run.SignalsIngested,
		run.MentionsSynced,
		run.RepliesDrafted,
		run.TriggeredBy,
		run.Notes,
		run.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create automation run: %w", err)
	}

	return nil
}
