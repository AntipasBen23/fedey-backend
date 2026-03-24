package content

import "time"

type Draft struct {
	ID          string    `json:"id"`
	Channel     string    `json:"channel"`
	Hook        string    `json:"hook"`
	Body        string    `json:"body"`
	Rationale   string    `json:"rationale"`
	SourceTrend string    `json:"sourceTrend"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}
