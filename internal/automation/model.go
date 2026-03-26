package automation

import "time"

type Run struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	DraftsGenerated  int       `json:"draftsGenerated"`
	SchedulesCreated int       `json:"schedulesCreated"`
	RepliesDrafted   int       `json:"repliesDrafted"`
	TriggeredBy      string    `json:"triggeredBy"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"createdAt"`
}
