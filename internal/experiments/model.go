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
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateInput struct {
	HypothesisID string `json:"hypothesisId"`
	Metric       string `json:"metric"`
}
