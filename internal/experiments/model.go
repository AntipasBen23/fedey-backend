package experiments

import "time"

type Status string

const (
	StatusDraft     Status = "draft"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
)

type Experiment struct {
	ID           string    `json:"id"`
	HypothesisID string    `json:"hypothesisId"`
	Metric       string    `json:"metric"`
	Status       Status    `json:"status"`
	Summary      *Summary  `json:"summary,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateInput struct {
	HypothesisID string `json:"hypothesisId"`
	Metric       string `json:"metric"`
}

type RecordMetricInput struct {
	ExperimentID string  `json:"experimentId"`
	Variant      string  `json:"variant"`
	Value        float64 `json:"value"`
}

type VariantScore struct {
	Variant      string  `json:"variant"`
	Events       int     `json:"events"`
	TotalValue   float64 `json:"totalValue"`
	AverageValue float64 `json:"averageValue"`
}

type Summary struct {
	WinnerVariant string         `json:"winnerVariant,omitempty"`
	WinnerScore   float64        `json:"winnerScore"`
	Confidence    float64        `json:"confidence"`
	Variants      []VariantScore `json:"variants"`
}
