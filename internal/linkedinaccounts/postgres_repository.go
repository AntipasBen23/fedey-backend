package linkedinaccounts

import (
	"context"
	"errors"
	"fmt"

	"github.com/AntipasBen23/fedey-backend/internal/security/tokens"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool   *pgxpool.Pool
	cipher tokens.Cipher
}

func NewPostgresRepository(pool *pgxpool.Pool, cipher tokens.Cipher) *PostgresRepository {
	if cipher == nil {
		cipher = tokens.NewNoopCipher()
	}

	return &PostgresRepository{pool: pool, cipher: cipher}
}

func (r *PostgresRepository) GetActive(ctx context.Context) (Account, error) {
	const query = `
		SELECT id, provider, member_id, display_name, author_urn, access_token, COALESCE(refresh_token, ''), scopes, expires_at, connected_at
		FROM linkedin_accounts
		WHERE provider = 'linkedin'
		LIMIT 1
	`

	var account Account
	err := r.pool.QueryRow(ctx, query).Scan(
		&account.ID,
		&account.Provider,
		&account.MemberID,
		&account.DisplayName,
		&account.AuthorURN,
		&account.AccessToken,
		&account.RefreshToken,
		&account.Scopes,
		&account.ExpiresAt,
		&account.ConnectedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrAccountNotConnected
	}
	if err != nil {
		return Account{}, fmt.Errorf("get linkedin account: %w", err)
	}

	if account.AccessToken, err = r.cipher.Decrypt(account.AccessToken); err != nil {
		return Account{}, fmt.Errorf("decrypt linkedin access token: %w", err)
	}
	if account.RefreshToken, err = r.cipher.Decrypt(account.RefreshToken); err != nil {
		return Account{}, fmt.Errorf("decrypt linkedin refresh token: %w", err)
	}

	return account, nil
}

func (r *PostgresRepository) UpsertActive(ctx context.Context, account Account) error {
	const query = `
		INSERT INTO linkedin_accounts (id, provider, member_id, display_name, author_urn, access_token, refresh_token, scopes, expires_at, connected_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (provider) DO UPDATE
		SET member_id = EXCLUDED.member_id,
		    display_name = EXCLUDED.display_name,
		    author_urn = EXCLUDED.author_urn,
		    access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    scopes = EXCLUDED.scopes,
		    expires_at = EXCLUDED.expires_at,
		    connected_at = EXCLUDED.connected_at
	`

	encryptedAccessToken, err := r.cipher.Encrypt(account.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt linkedin access token: %w", err)
	}
	encryptedRefreshToken, err := r.cipher.Encrypt(account.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt linkedin refresh token: %w", err)
	}

	_, err = r.pool.Exec(
		ctx,
		query,
		account.ID,
		account.Provider,
		account.MemberID,
		account.DisplayName,
		account.AuthorURN,
		encryptedAccessToken,
		nullString(encryptedRefreshToken),
		account.Scopes,
		account.ExpiresAt,
		account.ConnectedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert linkedin account: %w", err)
	}

	return nil
}

func (r *PostgresRepository) SaveState(ctx context.Context, state OAuthState) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO linkedin_oauth_states (state, created_at) VALUES ($1, $2)`, state.State, state.CreatedAt)
	if err != nil {
		return fmt.Errorf("save linkedin oauth state: %w", err)
	}
	return nil
}

func (r *PostgresRepository) GetState(ctx context.Context, state string) (OAuthState, error) {
	var value OAuthState
	err := r.pool.QueryRow(ctx, `SELECT state, created_at FROM linkedin_oauth_states WHERE state = $1`, state).Scan(&value.State, &value.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return OAuthState{}, ErrOAuthStateNotFound
	}
	if err != nil {
		return OAuthState{}, fmt.Errorf("get linkedin oauth state: %w", err)
	}

	return value, nil
}

func (r *PostgresRepository) DeleteState(ctx context.Context, state string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM linkedin_oauth_states WHERE state = $1`, state)
	if err != nil {
		return fmt.Errorf("delete linkedin oauth state: %w", err)
	}
	return nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}

	return value
}
