package automation

import (
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/publishing"
)

type Run struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	DraftsGenerated  int       `json:"draftsGenerated"`
	SchedulesCreated int       `json:"schedulesCreated"`
	PostsPublished   int       `json:"postsPublished"`
	MentionsSynced   int       `json:"mentionsSynced"`
	RepliesDrafted   int       `json:"repliesDrafted"`
	TriggeredBy      string    `json:"triggeredBy"`
	Notes            string    `json:"notes"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Settings struct {
	Interval string              `json:"interval"`
	Windows  []publishing.Window `json:"windows"`
}
