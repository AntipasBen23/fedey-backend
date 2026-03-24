package brandmemory

import "context"

type Repository interface {
	Get(ctx context.Context) (Profile, error)
	Upsert(ctx context.Context, input UpsertInput) (Profile, error)
}
