package onboarding

import "github.com/jackc/pgx/v5/pgxpool"

func NewRepository(pool *pgxpool.Pool) Repository {
	if pool == nil {
		return NewMemoryRepository()
	}
	return NewPostgresRepository(pool)
}
