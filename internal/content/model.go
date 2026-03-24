package content

import "time"

type Draft struct {
	ID           string    `json:"id"`
	Channel      string    `json:"channel"`
	Hook         string    `json:"hook"`
	Body         string    `json:"body"`
	Rationale    string    `json:"rationale"`
	SourceTrend  string    `json:"sourceTrend"`
	ExperimentID string    `json:"experimentId,omitempty"`
	Variants     []Variant `json:"variants,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Variant struct {
	Label string `json:"label"`
	Hook  string `json:"hook"`
	Body  string `json:"body"`
	Angle string `json:"angle"`
}
