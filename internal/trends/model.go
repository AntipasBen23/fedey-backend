package trends

import "time"

type Signal struct {
	ID         string    `json:"id"`
	Topic      string    `json:"topic"`
	Source     string    `json:"source"`
	Angle      string    `json:"angle"`
	Velocity   int       `json:"velocity"`
	Relevance  float64   `json:"relevance"`
	ObservedAt time.Time `json:"observedAt"`
}

type CreateInput struct {
	Topic     string  `json:"topic"`
	Source    string  `json:"source"`
	Angle     string  `json:"angle"`
	Velocity  int     `json:"velocity"`
	Relevance float64 `json:"relevance"`
}
