package xaccounts

import (
	"context"
	"errors"
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

func (r *PostgresRepository) GetActive(ctx context.Context) (Account, error) {
	const query = `
		SELECT id, provider, user_id, username, access_token, refresh_token, scopes, token_type, expires_at, connected_at
		FROM x_accounts
		WHERE provider = 'x'
		LIMIT 1
	`

	var account Account
	err := r.pool.QueryRow(ctx, query).Scan(
		&account.ID,
		&account.Provider,
		&account.UserID,
		&account.Username,
		&account.AccessToken,
		&account.RefreshToken,
		&account.Scopes,
		&account.TokenType,
		&account.ExpiresAt,
		&account.ConnectedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrAccountNotConnected
	}
	if err != nil {
		return Account{}, fmt.Errorf("get x account: %w", err)
	}

	return account, nil
}

func (r *PostgresRepository) UpsertActive(ctx context.Context, account Account) error {
	const query = `
		INSERT INTO x_accounts (id, provider, user_id, username, access_token, refresh_token, scopes, token_type, expires_at, connected_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (provider) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    username = EXCLUDED.username,
		    access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    scopes = EXCLUDED.scopes,
		    token_type = EXCLUDED.token_type,
		    expires_at = EXCLUDED.expires_at,
		    connected_at = EXCLUDED.connected_at
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		account.ID,
		account.Provider,
		account.UserID,
		account.Username,
		account.AccessToken,
		account.RefreshToken,
		account.Scopes,
		account.TokenType,
		account.ExpiresAt,
		account.ConnectedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert x account: %w", err)
	}

	return nil
}

func (r *PostgresRepository) SaveState(ctx context.Context, state OAuthState) error {
	const query = `
		INSERT INTO x_oauth_states (state, code_verifier, created_at)
		VALUES ($1, $2, $3)
	`

	_, err := r.pool.Exec(ctx, query, state.State, state.CodeVerifier, state.CreatedAt)
	if err != nil {
		return fmt.Errorf("save x oauth state: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetState(ctx context.Context, state string) (OAuthState, error) {
	const query = `
		SELECT state, code_verifier, created_at
		FROM x_oauth_states
		WHERE state = $1
	`

	var value OAuthState
	err := r.pool.QueryRow(ctx, query, state).Scan(&value.State, &value.CodeVerifier, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthState{}, ErrOAuthStateNotFound
	}
	if err != nil {
		return OAuthState{}, fmt.Errorf("get x oauth state: %w", err)
	}

	return value, nil
}

func (r *PostgresRepository) DeleteState(ctx context.Context, state string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM x_oauth_states WHERE state = $1`, state)
	if err != nil {
		return fmt.Errorf("delete x oauth state: %w", err)
	}
	return nil
}
