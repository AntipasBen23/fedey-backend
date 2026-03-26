package linkedinaccounts

import (
	"github.com/AntipasBen23/fedey-backend/internal/security/tokens"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRepository(pool *pgxpool.Pool, cipher tokens.Cipher) Repository {
	if pool == nil {
		return NewMemoryRepository()
	}

	return NewPostgresRepository(pool, cipher)
}
