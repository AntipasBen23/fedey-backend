package experiments

import (
	"math"
	"sort"
)

func buildSummary(events []RecordMetricInput) Summary {
	type aggregate struct {
		events int
		total  float64
	}

	aggregates := make(map[string]aggregate)
	for _, event := range events {
		current := aggregates[event.Variant]
		current.events++
		current.total += event.Value
		aggregates[event.Variant] = current
	}

	var variants []VariantScore
	for variant, aggregate := range aggregates {
		variants = append(variants, VariantScore{
			Variant:      variant,
			Events:       aggregate.events,
			TotalValue:   round2(aggregate.total),
			AverageValue: round2(aggregate.total / float64(aggregate.events)),
		})
	}

	sort.Slice(variants, func(i, j int) bool {
		if variants[i].AverageValue == variants[j].AverageValue {
			return variants[i].Variant < variants[j].Variant
		}
		return variants[i].AverageValue > variants[j].AverageValue
	})

	summary := Summary{
		Variants: variants,
	}
	if len(variants) == 0 {
		return summary
	}

	summary.WinnerVariant = variants[0].Variant
	summary.WinnerScore = variants[0].AverageValue
	if len(variants) == 1 {
		summary.Confidence = 1
		return summary
	}

	top := variants[0].AverageValue
	second := variants[1].AverageValue
	if top <= 0 {
		return summary
	}

	summary.Confidence = round2((top - second) / top)
	if summary.Confidence < 0 {
		summary.Confidence = 0
	}

	return summary
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
