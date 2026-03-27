package performance

import "time"

type Snapshot struct {
	ID             string    `json:"id"`
	Platform       string    `json:"platform"`
	ExternalPostID string    `json:"externalPostId"`
	AuthorRef      string    `json:"authorRef"`
	ContentPreview string    `json:"contentPreview"`
	LikeCount      int       `json:"likeCount"`
	ReplyCount     int       `json:"replyCount"`
	QuoteCount     int       `json:"quoteCount"`
	CommentCount   int       `json:"commentCount"`
	CapturedAt     time.Time `json:"capturedAt"`
}

type SyncResult struct {
	XSnapshots        int `json:"xSnapshots"`
	LinkedInSnapshots int `json:"linkedinSnapshots"`
}
