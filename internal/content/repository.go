package content

import "context"

type Repository interface {
	List(ctx context.Context) ([]Draft, error)
	SaveBatch(ctx context.Context, drafts []Draft) error
}
