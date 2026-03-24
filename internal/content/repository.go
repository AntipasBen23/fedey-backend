package content

import "context"

type Repository interface {
	List(ctx context.Context) ([]Draft, error)
	GetByID(ctx context.Context, draftID string) (Draft, error)
	SaveBatch(ctx context.Context, drafts []Draft) error
	Update(ctx context.Context, draft Draft) error
	SaveVariants(ctx context.Context, experimentID string, variants []Variant) error
}
