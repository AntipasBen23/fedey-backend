package community

import "context"

type Repository interface {
	List(ctx context.Context) ([]Item, error)
	Create(ctx context.Context, input CreateInput) (Item, error)
	Update(ctx context.Context, item Item) error
	GetByID(ctx context.Context, id string) (Item, error)
}
