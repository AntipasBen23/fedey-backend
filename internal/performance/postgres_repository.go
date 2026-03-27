package performance

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

func (r *PostgresRepository) SaveBatch(ctx context.Context, snapshots []Snapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, snapshot := range snapshots {
		batch.Queue(
			`INSERT INTO platform_performance_snapshots (id, platform, external_post_id, author_ref, content_preview, like_count, reply_count, quote_count, comment_count, captured_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			snapshot.ID,
			snapshot.Platform,
			snapshot.ExternalPostID,
			snapshot.AuthorRef,
			snapshot.ContentPreview,
			snapshot.LikeCount,
			snapshot.ReplyCount,
			snapshot.QuoteCount,
			snapshot.CommentCount,
			snapshot.CapturedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range snapshots {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert platform performance snapshot: %w", err)
		}
	}
	return nil
}

func (r *PostgresRepository) ListRecent(ctx context.Context, platform string, limit int) ([]Snapshot, error) {
	if limit <= 0 {
		limit = 10
	}

	const query = `
		SELECT id, platform, external_post_id, author_ref, content_preview, like_count, reply_count, quote_count, comment_count, captured_at
		FROM platform_performance_snapshots
		WHERE ($1 = '' OR platform = $1)
		ORDER BY captured_at DESC
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, platform, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent platform performance snapshots: %w", err)
	}
	defer rows.Close()

	var items []Snapshot
	for rows.Next() {
		var item Snapshot
		if err := rows.Scan(
			&item.ID,
			&item.Platform,
			&item.ExternalPostID,
			&item.AuthorRef,
			&item.ContentPreview,
			&item.LikeCount,
			&item.ReplyCount,
			&item.QuoteCount,
			&item.CommentCount,
			&item.CapturedAt,
		); err != nil {
			return nil, fmt.Errorf("scan platform performance snapshot: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate platform performance snapshots: %w", err)
	}
	return items, nil
}
