package content

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

func (r *PostgresRepository) List(ctx context.Context) ([]Draft, error) {
	const query = `
		SELECT id, channel, hook, body, rationale, source_trend, COALESCE(experiment_id, ''), status, created_at
		FROM content_drafts
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list content drafts: %w", err)
	}
	defer rows.Close()

	var drafts []Draft
	experimentIDs := make([]string, 0)
	for rows.Next() {
		var draft Draft
		if err := rows.Scan(
			&draft.ID,
			&draft.Channel,
			&draft.Hook,
			&draft.Body,
			&draft.Rationale,
			&draft.SourceTrend,
			&draft.ExperimentID,
			&draft.Status,
			&draft.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan content draft: %w", err)
		}

		if draft.ExperimentID != "" {
			experimentIDs = append(experimentIDs, draft.ExperimentID)
		}

		drafts = append(drafts, draft)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate content drafts: %w", err)
	}

	variantsByExperiment, err := r.loadVariants(ctx, experimentIDs)
	if err != nil {
		return nil, err
	}

	for index := range drafts {
		if drafts[index].ExperimentID == "" {
			continue
		}

		drafts[index].Variants = variantsByExperiment[drafts[index].ExperimentID]
	}

	return drafts, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, draftID string) (Draft, error) {
	const query = `
		SELECT id, channel, hook, body, rationale, source_trend, COALESCE(experiment_id, ''), status, created_at
		FROM content_drafts
		WHERE id = $1
	`

	var draft Draft
	err := r.pool.QueryRow(ctx, query, draftID).Scan(
		&draft.ID,
		&draft.Channel,
		&draft.Hook,
		&draft.Body,
		&draft.Rationale,
		&draft.SourceTrend,
		&draft.ExperimentID,
		&draft.Status,
		&draft.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Draft{}, ErrDraftNotFound
	}
	if err != nil {
		return Draft{}, fmt.Errorf("get content draft: %w", err)
	}

	if draft.ExperimentID != "" {
		variantsByExperiment, err := r.loadVariants(ctx, []string{draft.ExperimentID})
		if err != nil {
			return Draft{}, err
		}
		draft.Variants = variantsByExperiment[draft.ExperimentID]
	}

	return draft, nil
}

func (r *PostgresRepository) SaveBatch(ctx context.Context, drafts []Draft) error {
	batch := &pgx.Batch{}
	for _, draft := range drafts {
		batch.Queue(
			`INSERT INTO content_drafts (id, channel, hook, body, rationale, source_trend, experiment_id, status, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			draft.ID,
			draft.Channel,
			draft.Hook,
			draft.Body,
			draft.Rationale,
			draft.SourceTrend,
			nullExperimentID(draft.ExperimentID),
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

func (r *PostgresRepository) Update(ctx context.Context, draft Draft) error {
	const query = `
		UPDATE content_drafts
		SET channel = $1,
		    hook = $2,
		    body = $3,
		    rationale = $4,
		    source_trend = $5,
		    experiment_id = $6,
		    status = $7
		WHERE id = $8
	`

	commandTag, err := r.pool.Exec(
		ctx,
		query,
		draft.Channel,
		draft.Hook,
		draft.Body,
		draft.Rationale,
		draft.SourceTrend,
		nullExperimentID(draft.ExperimentID),
		draft.Status,
		draft.ID,
	)
	if err != nil {
		return fmt.Errorf("update content draft: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrDraftNotFound
	}

	return nil
}

func (r *PostgresRepository) SaveVariants(ctx context.Context, experimentID string, variants []Variant) error {
	batch := &pgx.Batch{}
	for _, variant := range variants {
		payload, err := json.Marshal(variant)
		if err != nil {
			return fmt.Errorf("marshal content variant: %w", err)
		}

		batch.Queue(
			`INSERT INTO experiment_variants (id, experiment_id, name, payload, created_at)
			 VALUES ($1, $2, $3, $4, NOW())`,
			"var-"+uuid.NewString(),
			experimentID,
			variant.Label,
			payload,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range variants {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("insert experiment variant: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) loadVariants(ctx context.Context, experimentIDs []string) (map[string][]Variant, error) {
	if len(experimentIDs) == 0 {
		return map[string][]Variant{}, nil
	}

	const query = `
		SELECT experiment_id, payload
		FROM experiment_variants
		WHERE experiment_id = ANY($1)
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, experimentIDs)
	if err != nil {
		return nil, fmt.Errorf("query experiment variants: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]Variant)
	for rows.Next() {
		var experimentID string
		var payload []byte
		if err := rows.Scan(&experimentID, &payload); err != nil {
			return nil, fmt.Errorf("scan experiment variant: %w", err)
		}

		var variant Variant
		if err := json.Unmarshal(payload, &variant); err != nil {
			return nil, fmt.Errorf("unmarshal experiment variant: %w", err)
		}

		result[experimentID] = append(result[experimentID], variant)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate experiment variants: %w", err)
	}

	return result, nil
}

func nullExperimentID(experimentID string) any {
	if experimentID == "" {
		return nil
	}

	return experimentID
}
