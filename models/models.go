package models

import (
	"time"

	"gorm.io/gorm"
)

// SocialAccount stores OAuth tokens and platform info
type SocialAccount struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	UserID           uint           `gorm:"uniqueIndex:idx_user_platform_context;not null" json:"userId"`
	Platform         string         `gorm:"uniqueIndex:idx_user_platform_context" json:"platform"`
	AccountType      string         `gorm:"uniqueIndex:idx_user_platform_context" json:"accountType"` // 'old' or 'new'
	AccessToken      string         `json:"accessToken"`
	AutoPilotEnabled bool           `gorm:"default:false" json:"autoPilotEnabled"`
	GhostModeEnabled bool           `gorm:"default:false" json:"ghostModeEnabled"`
	SchedulingConfig string         `json:"schedulingConfig"` // JSON string for Mode, Staggering, etc.
	FollowerCount    int            `json:"followerCount"`     // Last synced follower count
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// ContentCalendar stores generated content plans
type ContentCalendar struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	UserID         uint           `gorm:"index;not null" json:"userId"`
	ProductSummary string         `json:"productSummary"`
	ContentJSON    string         `json:"contentJson"` // Stores the plan as JSON string
	Status         string         `gorm:"default:'draft'" json:"status"` // 'draft' or 'scheduled'
	DayCount       int            `gorm:"default:3" json:"dayCount"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// ScheduledPost represents a discrete posting event for the background worker
type ScheduledPost struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"index;not null" json:"userId"`
	AccountID     uint           `json:"accountId"`
	Platform      string         `json:"platform"`
	Content       string         `json:"content"`
	ContentType   string         `json:"contentType"`   // tweet|thread|carousel|video_script|linkedin_post
	Script        string         `json:"script"`         // Full script (thread tweets or video scene script)
	SlidesJSON    string         `json:"slidesJson"`     // JSON array of carousel slide copy strings
	ImageURLsJSON string         `json:"imageUrlsJson"`  // JSON array of DALL-E generated image URLs
	VideoURL      string         `json:"videoUrl"`       // Runway ML generated video URL
	Hashtags      string         `json:"hashtags"`       // Space-separated hashtags
	CTAText       string         `json:"ctaText"`
	Day           int            `json:"day"`
	ScheduledAt   time.Time      `json:"scheduledAt"`
	Status        string         `gorm:"default:'pending'" json:"status"` // 'pending', 'posted', 'failed'
	ExternalID    string         `json:"externalId"`    // The ID from X/LinkedIn after posting
	FailureReason string         `json:"failureReason"` // Error detail when status = 'failed'
	AIReasoning   string         `json:"aiReasoning"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Analytics     *PostAnalytics `gorm:"foreignKey:ScheduledPostID" json:"analytics,omitempty"`
}

// PostAnalytics stores engagement data for a specific post
type PostAnalytics struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ScheduledPostID uint           `gorm:"index" json:"scheduledPostId"`
	Likes           int            `json:"likes"`
	Reposts         int            `json:"reposts"`
	Comments        int            `json:"comments"`
	Impressions     int            `json:"impressions"`
	EngagementRate  float64        `json:"engagementRate"`
	ImpactScore     int            `json:"impactScore"` // 1-100 calculated by AI
	SyncTime        time.Time      `json:"syncTime"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserStrategy stores the active growth strategy
type UserStrategy struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	UserID             uint           `gorm:"index;not null" json:"userId"`
	IdentityAudit      string         `json:"identityAudit"`
	TrendMonitoring    string         `json:"trendMonitoring"` // JSON string
	GrowthExperiments  string         `json:"growthExperiments"` // JSON string
	AnalyticsReporting string         `json:"analyticsReporting"` // JSON string
	PreferredStartHour int            `gorm:"default:9" json:"preferredStartHour"`
	PreferredStagger   string         `gorm:"default:'smart'" json:"preferredStagger"`
	PreferredMode      string         `gorm:"default:'smart'" json:"preferredMode"`
	SaveAsDefault      bool           `gorm:"default:false" json:"saveAsDefault"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// FollowerSnapshot stores one daily follower count reading per platform.
// Used to power the follower growth chart on the dashboard.
type FollowerSnapshot struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_snapshot_date_platform;not null" json:"userId"`
	Date      string    `gorm:"uniqueIndex:idx_user_snapshot_date_platform" json:"date"` // "2026-04-16"
	Platform  string    `gorm:"uniqueIndex:idx_user_snapshot_date_platform" json:"platform"`
	Count     int       `json:"count"`
	CreatedAt time.Time `json:"createdAt"`
}

type UserPreferences struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Config    string         `json:"config"`
	// Plan controls feature access: "free" (default) or "pro"
	Plan      string         `gorm:"default:'free'" json:"plan"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// ── Auth models ───────────────────────────────────────────────────────────────

// User is the core identity record for app authentication.
type User struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	Name               string         `json:"name"`
	Username           string         `gorm:"uniqueIndex" json:"username"`
	Email              string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash       string         `json:"-"` // empty for Google-only accounts
	GoogleID           string         `gorm:"index" json:"-"`
	IsVerified         bool           `gorm:"default:false" json:"isVerified"`
	Plan               string         `gorm:"default:'free'" json:"plan"`
	LoginAttempts      int            `gorm:"default:0" json:"-"`
	LockedUntil        *time.Time     `json:"-"`
	LastLoginAt        *time.Time     `json:"lastLoginAt"`
	// Onboarding state — persisted server-side so localStorage is not needed
	JobDescription     string         `json:"jobDescription"`
	PlatformContext    string         `json:"platformContext"`    // JSON: {platform, accountType}
	LastOnboardingStep string         `json:"lastOnboardingStep"` // e.g. "/hire", "/strategy", "completed"
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// PageView records a single page visit for analytics.
type PageView struct {
	ID         uint      `gorm:"primaryKey"`
	Path       string    `gorm:"index;not null"`
	Referrer   string
	DeviceType string    // "mobile" | "tablet" | "desktop"
	SessionID  string    `gorm:"index"`
	UserID     *uint     `gorm:"index"` // nil = anonymous visitor
	CreatedAt  time.Time `gorm:"index"`
}

// PageVisit is the enriched visit record with geo-IP, visitor fingerprint, and identity.
type PageVisit struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	VisitorKey string    `gorm:"index;not null" json:"visitorKey"`
	UserID     *uint     `gorm:"index" json:"userId"`
	UserEmail  string    `gorm:"default:''" json:"userEmail"`
	Path       string    `gorm:"not null" json:"path"`
	Referrer   string    `gorm:"default:''" json:"referrer"`
	Country    string    `gorm:"index;default:''" json:"country"`
	Region     string    `gorm:"default:''" json:"region"`
	State      string    `gorm:"default:''" json:"state"`
	City       string    `gorm:"default:''" json:"city"`
	UserAgent  string    `gorm:"default:''" json:"userAgent"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
}

// EmailVerification holds a one-time 6-digit code for verifying a new user's email.
type EmailVerification struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Code      string    `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Used      bool      `gorm:"default:false"`
	CreatedAt time.Time
}

// RefreshToken stores server-side opaque refresh tokens for JWT rotation.
type RefreshToken struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Token     string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time
}

// PasswordResetToken is a short-lived token for resetting a forgotten password.
type PasswordResetToken struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	Token     string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	Used      bool      `gorm:"default:false"`
	CreatedAt time.Time
}

// EngagementEvent tracks find-and-reply social listening events
type EngagementEvent struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"index;not null" json:"userId"`
	AccountID       uint           `json:"accountId"`
	Platform        string         `json:"platform"`
	Type            string         `json:"type"` // 'mention', 'niche_discovery', 'comment_on_self'
	ExternalPostID  string         `json:"externalPostId"`
	AuthorHandle    string         `json:"authorHandle"`
	OriginalContent string         `json:"originalContent"`
	ProposedReply   string         `json:"proposedReply"`
	Status          string         `gorm:"default:'pending'" json:"status"` // 'pending', 'sent', 'ignored'
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}
