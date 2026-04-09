package models

import (
	"time"

	"gorm.io/gorm"
)

// SocialAccount stores OAuth tokens and platform info
type SocialAccount struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Platform    string         `gorm:"uniqueIndex:idx_platform_context" json:"platform"`
	AccountType string         `gorm:"uniqueIndex:idx_platform_context" json:"accountType"` // 'old' or 'new'
	AccessToken string         `json:"accessToken"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// ContentCalendar stores generated 14-day plans
type ContentCalendar struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ProductSummary string         `json:"productSummary"`
	ContentJSON    string         `json:"contentJson"` // Stores the full 14-day plan as JSON string
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
