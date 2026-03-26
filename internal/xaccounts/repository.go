package xaccounts

import "context"

type Repository interface {
	GetActive(ctx context.Context) (Account, error)
	UpsertActive(ctx context.Context, account Account) error
	SaveState(ctx context.Context, state OAuthState) error
	GetState(ctx context.Context, state string) (OAuthState, error)
	DeleteState(ctx context.Context, state string) error
}
