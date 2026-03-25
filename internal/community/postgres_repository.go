package community

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

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]Item, error) {
	const query = `
		SELECT id, platform, author, message, sentiment, COALESCE(reply_draft, ''), linked_post_ref, status, created_at, COALESCE(replied_at, TIMESTAMPTZ '0001-01-01')
		FROM community_inbox
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list community items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.ID,
			&item.Platform,
			&item.Author,
			&item.Message,
			&item.Sentiment,
			&item.ReplyDraft,
			&item.LinkedPostRef,
			&item.Status,
			&item.CreatedAt,
			&item.RepliedAt,
		); err != nil {
			return nil, fmt.Errorf("scan community item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate community items: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input CreateInput) (Item, error) {
	const query = `
		INSERT INTO community_inbox (id, platform, author, message, sentiment, linked_post_ref, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, platform, author, message, sentiment, COALESCE(reply_draft, ''), linked_post_ref, status, created_at, COALESCE(replied_at, TIMESTAMPTZ '0001-01-01')
	`

	var item Item
	err := r.pool.QueryRow(
		ctx,
		query,
		"cmt-"+uuid.NewString(),
		input.Platform,
		input.Author,
		input.Message,
		input.Sentiment,
		input.LinkedPostRef,
		StatusPending,
		time.Now().UTC(),
	).Scan(
		&item.ID,
		&item.Platform,
		&item.Author,
		&item.Message,
		&item.Sentiment,
		&item.ReplyDraft,
		&item.LinkedPostRef,
		&item.Status,
		&item.CreatedAt,
		&item.RepliedAt,
	)
	if err != nil {
		return Item{}, fmt.Errorf("create community item: %w", err)
	}

	return item, nil
}

func (r *PostgresRepository) Update(ctx context.Context, item Item) error {
	const query = `
		UPDATE community_inbox
		SET platform = $1,
		    author = $2,
		    message = $3,
		    sentiment = $4,
		    reply_draft = $5,
		    linked_post_ref = $6,
		    status = $7,
		    replied_at = $8
		WHERE id = $9
	`

	commandTag, err := r.pool.Exec(
		ctx,
		query,
		item.Platform,
		item.Author,
		item.Message,
		item.Sentiment,
		nullReply(item.ReplyDraft),
		item.LinkedPostRef,
		item.Status,
		nullTime(item.RepliedAt),
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("update community item: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrItemNotFound
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id string) (Item, error) {
	const query = `
		SELECT id, platform, author, message, sentiment, COALESCE(reply_draft, ''), linked_post_ref, status, created_at, COALESCE(replied_at, TIMESTAMPTZ '0001-01-01')
		FROM community_inbox
		WHERE id = $1
	`

	var item Item
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&item.ID,
		&item.Platform,
		&item.Author,
		&item.Message,
		&item.Sentiment,
		&item.ReplyDraft,
		&item.LinkedPostRef,
		&item.Status,
		&item.CreatedAt,
		&item.RepliedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrItemNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("get community item: %w", err)
	}

	return item, nil
}

func nullReply(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}
