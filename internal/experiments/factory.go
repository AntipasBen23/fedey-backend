package experiments

import (
	"context"
	"fmt"
	"strings"
)

func NewRepository(ctx context.Context, databaseURL string) (Repository, func(), error) {
	if strings.TrimSpace(databaseURL) == "" {
		return NewMemoryRepository(), func() {}, nil
	}

	postgresRepository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("initialize postgres repository: %w", err)
	}

	return postgresRepository, postgresRepository.Close, nil
}
