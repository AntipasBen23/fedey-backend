package models

import (
	"time"

	"gorm.io/gorm"
)

// SocialAccount stores OAuth tokens and platform info
type SocialAccount struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Platform         string         `gorm:"uniqueIndex:idx_platform_context" json:"platform"`
	AccountType      string         `gorm:"uniqueIndex:idx_platform_context" json:"accountType"` // 'old' or 'new'
	AccessToken      string         `json:"accessToken"`
	AutoPilotEnabled bool           `gorm:"default:false" json:"autoPilotEnabled"`
	SchedulingConfig string         `json:"schedulingConfig"` // JSON string for Mode, Staggering, etc.
	FollowerCount    int            `json:"followerCount"`     // Last synced follower count
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// ContentCalendar stores generated content plans
type ContentCalendar struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
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

type UserPreferences struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Config    string         `json:"config"`
	// Plan controls feature access: "free" (default) or "pro"
	Plan      string         `gorm:"default:'free'" json:"plan"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
