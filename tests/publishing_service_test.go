package tests

import (
	"context"
	"testing"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/publishing"
)

func TestRecommendNextTimeHonorsNewAccountSpacing(t *testing.T) {
	t.Parallel()

	repo := publishing.NewMemoryRepository()
	service := publishing.NewService(repo, nil, nil, nil, nil, nil, nil, nil, nil)

	base := time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	_, err := repo.Create(context.Background(), publishing.CreateInput{
		DraftID:      "draft-1",
		Channel:      "x",
		QueueProfile: "new",
		ScheduledFor: base,
	})
	if err != nil {
		t.Fatalf("repo.Create returned error: %v", err)
	}

	recommended := service.RecommendNextTime(context.Background(), "x", base.Add(30*time.Minute), "new")
	if !recommended.After(base.Add(11 * time.Hour)) {
		t.Fatalf("expected new-account X schedule to respect a 12h gap, got %s", recommended)
	}
}

func TestRecommendNextTimeAllowsTighterExistingSpacing(t *testing.T) {
	t.Parallel()

	repo := publishing.NewMemoryRepository()
	service := publishing.NewService(repo, nil, nil, nil, nil, nil, nil, nil, nil)

	base := time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	_, err := repo.Create(context.Background(), publishing.CreateInput{
		DraftID:      "draft-1",
		Channel:      "x",
		QueueProfile: "existing",
		ScheduledFor: base,
	})
	if err != nil {
		t.Fatalf("repo.Create returned error: %v", err)
	}

	recommended := service.RecommendNextTime(context.Background(), "x", base.Add(30*time.Minute), "existing")
	if !recommended.After(base.Add(5 * time.Hour)) {
		t.Fatalf("expected existing-account X schedule to respect a 6h gap, got %s", recommended)
	}
}
