package brandmemory

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Get(ctx context.Context) (Profile, error) {
	const query = `
		SELECT id, brand_name, tone, audience, pillars, guardrails, updated_at
		FROM brand_memory
		WHERE id = $1
	`

	var profile Profile
	err := r.pool.QueryRow(ctx, query, defaultProfileID).Scan(
		&profile.ID,
		&profile.BrandName,
		&profile.Tone,
		&profile.Audience,
		&profile.Pillars,
		&profile.Guardrails,
		&profile.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("get brand profile: %w", err)
	}

	return profile, nil
}

func (r *PostgresRepository) Upsert(ctx context.Context, input UpsertInput) (Profile, error) {
	const query = `
		INSERT INTO brand_memory (id, brand_name, tone, audience, pillars, guardrails, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE
		SET brand_name = EXCLUDED.brand_name,
		    tone = EXCLUDED.tone,
		    audience = EXCLUDED.audience,
		    pillars = EXCLUDED.pillars,
		    guardrails = EXCLUDED.guardrails,
		    updated_at = EXCLUDED.updated_at
		RETURNING id, brand_name, tone, audience, pillars, guardrails, updated_at
	`

	var profile Profile
	err := r.pool.QueryRow(
		ctx,
		query,
		defaultProfileID,
		input.BrandName,
		input.Tone,
		input.Audience,
		input.Pillars,
		input.Guardrails,
		time.Now().UTC(),
	).Scan(
		&profile.ID,
		&profile.BrandName,
		&profile.Tone,
		&profile.Audience,
		&profile.Pillars,
		&profile.Guardrails,
		&profile.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("upsert brand profile: %w", err)
	}

	return profile, nil
}
