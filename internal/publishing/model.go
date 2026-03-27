package publishing

import "time"

type Status string

const (
	StatusScheduled Status = "scheduled"
	StatusPublished Status = "published"
)

type Schedule struct {
	ID                  string             `json:"id"`
	DraftID             string             `json:"draftId"`
	VariantLabel        string             `json:"variantLabel,omitempty"`
	Channel             string             `json:"channel"`
	QueueProfile        string             `json:"queueProfile"`
	PlatformPostID      string             `json:"platformPostId,omitempty"`
	ScheduledFor        time.Time          `json:"scheduledFor"`
	Status              Status             `json:"status"`
	PublishedAt         time.Time          `json:"publishedAt,omitempty"`
	PerformanceSyncedAt time.Time          `json:"performanceSyncedAt,omitempty"`
	Timeline            []PerformancePoint `json:"timeline,omitempty"`
	CreatedAt           time.Time          `json:"createdAt"`
}

type Window struct {
	Hour   int    `json:"hour"`
	Minute int    `json:"minute"`
	Label  string `json:"label"`
}

type CreateInput struct {
	DraftID      string    `json:"draftId"`
	VariantLabel string    `json:"variantLabel"`
	Channel      string    `json:"channel"`
	QueueProfile string    `json:"queueProfile"`
	ScheduledFor time.Time `json:"scheduledFor"`
}

type PerformancePoint struct {
	CapturedAt      time.Time `json:"capturedAt"`
	Likes           int       `json:"likes"`
	Replies         int       `json:"replies"`
	Quotes          int       `json:"quotes"`
	Comments        int       `json:"comments"`
	TotalEngagement int       `json:"totalEngagement"`
}
