package tests

import (
	"context"
	"testing"

	"github.com/AntipasBen23/fedey-backend/internal/experiments"
)

func TestExperimentServiceCreatesSummaryFromRecordedMetrics(t *testing.T) {
	t.Parallel()

	service := experiments.NewService(experiments.NewMemoryRepository())

	experiment, err := service.Create(context.Background(), experiments.CreateInput{
		HypothesisID: "hyp-1",
		Metric:       "engagement_rate",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	events := []experiments.RecordMetricInput{
		{ExperimentID: experiment.ID, Variant: "A", Value: 10},
		{ExperimentID: experiment.ID, Variant: "A", Value: 8},
		{ExperimentID: experiment.ID, Variant: "B", Value: 14},
		{ExperimentID: experiment.ID, Variant: "B", Value: 12},
	}
	for _, event := range events {
		if err := service.RecordMetric(context.Background(), event); err != nil {
			t.Fatalf("RecordMetric returned error: %v", err)
		}
	}

	list, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 experiment, got %d", len(list))
	}
	if list[0].Summary == nil {
		t.Fatalf("expected summary to be populated")
	}
	if list[0].Summary.WinnerVariant != "B" {
		t.Fatalf("expected variant B to be winner, got %q", list[0].Summary.WinnerVariant)
	}
}

func TestExperimentServiceRejectsInvalidMetricInput(t *testing.T) {
	t.Parallel()

	service := experiments.NewService(experiments.NewMemoryRepository())

	if err := service.RecordMetric(context.Background(), experiments.RecordMetricInput{
		ExperimentID: "",
		Variant:      "A",
		Value:        2,
	}); err == nil {
		t.Fatalf("expected invalid metric input error")
	}
}
