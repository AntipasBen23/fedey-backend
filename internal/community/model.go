package community

import "time"

type Status string

const (
	StatusPending Status = "pending"
	StatusDrafted Status = "drafted"
	StatusReplied Status = "replied"
)

type Item struct {
	ID            string    `json:"id"`
	Platform      string    `json:"platform"`
	Author        string    `json:"author"`
	Message       string    `json:"message"`
	Sentiment     string    `json:"sentiment"`
	ReplyDraft    string    `json:"replyDraft,omitempty"`
	LinkedPostRef string    `json:"linkedPostRef"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	RepliedAt     time.Time `json:"repliedAt,omitempty"`
}

type CreateInput struct {
	Platform      string `json:"platform"`
	Author        string `json:"author"`
	Message       string `json:"message"`
	Sentiment     string `json:"sentiment"`
	LinkedPostRef string `json:"linkedPostRef"`
}
