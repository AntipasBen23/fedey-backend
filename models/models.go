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
	ID          uint           `gorm:"primaryKey" json:"id"`
	AccountID   uint           `json:"accountId"`
	Platform    string         `json:"platform"`
	Content     string         `json:"content"`
	Day         int            `json:"day"`
	ScheduledAt time.Time      `json:"scheduledAt"`
	Status      string         `gorm:"default:'pending'" json:"status"` // 'pending', 'posted', 'failed'
	AIReasoning string         `json:"aiReasoning"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserStrategy stores the active growth strategy
type UserStrategy struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	IdentityAudit      string         `json:"identityAudit"`
	TrendMonitoring    string         `json:"trendMonitoring"` // JSON string
	GrowthExperiments  string         `json:"growthExperiments"` // JSON string
	AnalyticsReporting string         `json:"analyticsReporting"` // JSON string
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}
