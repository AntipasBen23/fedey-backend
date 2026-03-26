package publishing

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

func (r *PostgresRepository) List(ctx context.Context) ([]Schedule, error) {
	const query = `
		SELECT id, draft_id, COALESCE(variant_label, ''), channel, COALESCE(platform_post_id, ''), scheduled_for, status, COALESCE(published_at, TIMESTAMPTZ '0001-01-01'), created_at
		FROM publishing_schedules
		ORDER BY scheduled_for ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list publishing schedules: %w", err)
	}
	defer rows.Close()

	var items []Schedule
	for rows.Next() {
		var item Schedule
		if err := rows.Scan(
			&item.ID,
			&item.DraftID,
			&item.VariantLabel,
			&item.Channel,
			&item.PlatformPostID,
			&item.ScheduledFor,
			&item.Status,
			&item.PublishedAt,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan publishing schedule: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate publishing schedules: %w", err)
	}

	return items, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, scheduleID string) (Schedule, error) {
	const query = `
		SELECT id, draft_id, COALESCE(variant_label, ''), channel, COALESCE(platform_post_id, ''), scheduled_for, status, COALESCE(published_at, TIMESTAMPTZ '0001-01-01'), created_at
		FROM publishing_schedules
		WHERE id = $1
	`

	var item Schedule
	err := r.pool.QueryRow(ctx, query, scheduleID).Scan(
		&item.ID,
		&item.DraftID,
		&item.VariantLabel,
		&item.Channel,
		&item.PlatformPostID,
		&item.ScheduledFor,
		&item.Status,
		&item.PublishedAt,
		&item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Schedule{}, ErrScheduleNotFound
	}
	if err != nil {
		return Schedule{}, fmt.Errorf("get publishing schedule: %w", err)
	}

	return item, nil
}

func (r *PostgresRepository) Create(ctx context.Context, input CreateInput) (Schedule, error) {
	const query = `
		INSERT INTO publishing_schedules (id, draft_id, variant_label, channel, scheduled_for, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, draft_id, COALESCE(variant_label, ''), channel, COALESCE(platform_post_id, ''), scheduled_for, status, COALESCE(published_at, TIMESTAMPTZ '0001-01-01'), created_at
	`

	now := time.Now().UTC()
	var item Schedule
	err := r.pool.QueryRow(
		ctx,
		query,
		"sch-"+uuid.NewString(),
		input.DraftID,
		nullString(input.VariantLabel),
		input.Channel,
		input.ScheduledFor.UTC(),
		StatusScheduled,
		now,
	).Scan(
		&item.ID,
		&item.DraftID,
		&item.VariantLabel,
		&item.Channel,
		&item.PlatformPostID,
		&item.ScheduledFor,
		&item.Status,
		&item.PublishedAt,
		&item.CreatedAt,
	)
	if err != nil {
		return Schedule{}, fmt.Errorf("create publishing schedule: %w", err)
	}

	return item, nil
}

func (r *PostgresRepository) MarkPublished(ctx context.Context, scheduleID string, platformPostID string) (Schedule, error) {
	const query = `
		UPDATE publishing_schedules
		SET status = $1, platform_post_id = $2, published_at = $3
		WHERE id = $4
		RETURNING id, draft_id, COALESCE(variant_label, ''), channel, COALESCE(platform_post_id, ''), scheduled_for, status, COALESCE(published_at, TIMESTAMPTZ '0001-01-01'), created_at
	`

	var item Schedule
	err := r.pool.QueryRow(
		ctx,
		query,
		StatusPublished,
		nullString(platformPostID),
		time.Now().UTC(),
		scheduleID,
	).Scan(
		&item.ID,
		&item.DraftID,
		&item.VariantLabel,
		&item.Channel,
		&item.PlatformPostID,
		&item.ScheduledFor,
		&item.Status,
		&item.PublishedAt,
		&item.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Schedule{}, ErrScheduleNotFound
	}
	if err != nil {
		return Schedule{}, fmt.Errorf("mark schedule published: %w", err)
	}

	return item, nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}

	return value
}
