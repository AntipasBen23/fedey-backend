package performance

import "context"

type Repository interface {
	SaveBatch(ctx context.Context, snapshots []Snapshot) error
	ListRecent(ctx context.Context, platform string, limit int) ([]Snapshot, error)
	ListForPost(ctx context.Context, platform, externalPostID string, limit int) ([]Snapshot, error)
}
