package onboarding

import "time"

type SessionStatus string

const (
	StatusDraft            SessionStatus = "draft"
	StatusInterview        SessionStatus = "interview"
	StatusReady            SessionStatus = "ready"
	StatusAuditReady       SessionStatus = "audit_ready"
	StatusAwaitingApproval SessionStatus = "awaiting_approval"
	StatusActivated        SessionStatus = "activated"
)

type Session struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	JobDescription  string         `json:"jobDescription"`
	AccountMode     string         `json:"accountMode"`
	Objective       string         `json:"objective"`
	PrimaryPlatform string         `json:"primaryPlatform"`
	BrandName       string         `json:"brandName"`
	Audience        string         `json:"audience"`
	VoiceSummary    string         `json:"voiceSummary"`
	Constraints     []string       `json:"constraints"`
	ReviewMode      string         `json:"reviewMode"`
	ApprovalStatus  string         `json:"approvalStatus"`
	Questions       []Question     `json:"questions"`
	Audit           AuditReport    `json:"audit"`
	Activation      ActivationPlan `json:"activation"`
	Status          SessionStatus  `json:"status"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type Question struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"sessionId"`
	Prompt     string    `json:"prompt"`
	Category   string    `json:"category"`
	Answer     string    `json:"answer,omitempty"`
	Required   bool      `json:"required"`
	CreatedAt  time.Time `json:"createdAt"`
	AnsweredAt time.Time `json:"answeredAt,omitempty"`
}

type AuditReport struct {
	Status              string    `json:"status"`
	ConnectedPlatforms  []string  `json:"connectedPlatforms"`
	PostsReviewed       int       `json:"postsReviewed"`
	ReplyPatterns       []string  `json:"replyPatterns"`
	ContentPatterns     []string  `json:"contentPatterns"`
	Recommendations     []string  `json:"recommendations"`
	PerformanceInsights []string  `json:"performanceInsights"`
	LastRunAt           time.Time `json:"lastRunAt,omitempty"`
}

type ActivationPlan struct {
	Status          string            `json:"status"`
	BrandMemorySync bool              `json:"brandMemorySync"`
	WeekPlan        []ActivationItem  `json:"weekPlan"`
	Drafts          []ActivationDraft `json:"drafts"`
	Summary         string            `json:"summary"`
	GeneratedAt     time.Time         `json:"generatedAt,omitempty"`
}

type ActivationItem struct {
	Day        string `json:"day"`
	Channel    string `json:"channel"`
	Focus      string `json:"focus"`
	Format     string `json:"format"`
	Hypothesis string `json:"hypothesis"`
}

type ActivationDraft struct {
	DraftID        string    `json:"draftId"`
	Channel        string    `json:"channel"`
	Hook           string    `json:"hook"`
	Rationale      string    `json:"rationale"`
	ScheduleID     string    `json:"scheduleId,omitempty"`
	ScheduleStatus string    `json:"scheduleStatus,omitempty"`
	ScheduledFor   time.Time `json:"scheduledFor,omitempty"`
}

type CreateSessionInput struct {
	Title           string   `json:"title"`
	JobDescription  string   `json:"jobDescription"`
	AccountMode     string   `json:"accountMode"`
	Objective       string   `json:"objective"`
	PrimaryPlatform string   `json:"primaryPlatform"`
	BrandName       string   `json:"brandName"`
	Audience        string   `json:"audience"`
	Constraints     []string `json:"constraints"`
	ReviewMode      string   `json:"reviewMode"`
}

type AnswerQuestionInput struct {
	SessionID  string `json:"sessionId"`
	QuestionID string `json:"questionId"`
	Answer     string `json:"answer"`
}

type UpdateReviewModeInput struct {
	SessionID  string `json:"sessionId"`
	ReviewMode string `json:"reviewMode"`
}

type UpdateActivationPlanInput struct {
	SessionID string           `json:"sessionId"`
	WeekPlan  []ActivationItem `json:"weekPlan"`
}
