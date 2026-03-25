package community

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	items []Item
}

func NewMemoryRepository() *MemoryRepository {
	now := time.Now().UTC()
	return &MemoryRepository{
		items: []Item{
			{
				ID:            "cmt-1",
				Platform:      "x",
				Author:        "founder_ops",
				Message:       "This is exactly the kind of AI social operator stack I have been looking for. How would you keep replies on-brand?",
				Sentiment:     "positive",
				LinkedPostRef: "sch-1",
				Status:        StatusPending,
				CreatedAt:     now.Add(-35 * time.Minute),
			},
		},
	}
}

func (r *MemoryRepository) List(_ context.Context) ([]Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := append([]Item(nil), r.items...)
	slices.SortFunc(items, func(left, right Item) int {
		return right.CreatedAt.Compare(left.CreatedAt)
	})
	return items, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput) (Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item := Item{
		ID:            fmt.Sprintf("cmt-%d", time.Now().UTC().UnixNano()),
		Platform:      input.Platform,
		Author:        input.Author,
		Message:       input.Message,
		Sentiment:     input.Sentiment,
		LinkedPostRef: input.LinkedPostRef,
		Status:        StatusPending,
		CreatedAt:     time.Now().UTC(),
	}

	r.items = append(r.items, item)
	return item, nil
}

func (r *MemoryRepository) Update(_ context.Context, item Item) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for index := range r.items {
		if r.items[index].ID == item.ID {
			r.items[index] = item
			return nil
		}
	}

	return ErrItemNotFound
}

func (r *MemoryRepository) GetByID(_ context.Context, id string) (Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, item := range r.items {
		if item.ID == id {
			return item, nil
		}
	}

	return Item{}, ErrItemNotFound
}
