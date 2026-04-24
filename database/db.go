package database

import (
	"fmt"
	"log"
	"os"

	"github.com/AntipasBen23/fedey-backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("DATABASE_URL not set, skipping database initialization")
		return
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-Migrate the schemas
	
	// LONG-TERM FIX: Ensure columns exist and are populated before enforcing NOT NULL
	// These manual steps prevent migration crashes when adding NOT NULL columns to existing data.
	db.Exec("ALTER TABLE social_accounts ADD COLUMN IF NOT EXISTS user_id bigint")
	db.Exec("ALTER TABLE content_calendars ADD COLUMN IF NOT EXISTS user_id bigint")
	db.Exec("ALTER TABLE scheduled_posts ADD COLUMN IF NOT EXISTS user_id bigint")
	db.Exec("ALTER TABLE user_strategies ADD COLUMN IF NOT EXISTS user_id bigint")
	db.Exec("ALTER TABLE follower_snapshots ADD COLUMN IF NOT EXISTS user_id bigint")
	db.Exec("ALTER TABLE engagement_events ADD COLUMN IF NOT EXISTS user_id bigint")
	
	// Backfill: Assign orphaned records to the first admin/user found in the DB.
	// If no user exists yet, it will remain NULL until the first user is created.
	backfillSQL := `
		UPDATE %s SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1) 
		WHERE user_id IS NULL OR user_id = 0
	`
	db.Exec(fmt.Sprintf(backfillSQL, "social_accounts"))
	db.Exec(fmt.Sprintf(backfillSQL, "content_calendars"))
	db.Exec(fmt.Sprintf(backfillSQL, "scheduled_posts"))
	db.Exec(fmt.Sprintf(backfillSQL, "user_strategies"))
	db.Exec(fmt.Sprintf(backfillSQL, "follower_snapshots"))
	db.Exec(fmt.Sprintf(backfillSQL, "engagement_events"))

	err = db.AutoMigrate(
		&models.SocialAccount{},
		&models.ContentCalendar{},
		&models.UserStrategy{},
		&models.ScheduledPost{},
		&models.PostAnalytics{},
		&models.UserPreferences{},
		&models.FollowerSnapshot{},
		&models.EngagementEvent{},
		&models.InteractionProfile{},
		&models.CompetitorProfile{},
		// Auth models
		&models.User{},
		&models.EmailVerification{},
		&models.RefreshToken{},
		&models.PasswordResetToken{},
		// Analytics
		&models.PageView{},
		&models.PageVisit{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}

	// Replace the full unique index on username with a partial one that only
	// enforces uniqueness for non-empty values, allowing multiple users to
	// have no username set yet (stored as '').
	db.Exec("DROP INDEX IF EXISTS idx_users_username")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE username != ''")

	log.Println("Database connection successful and schemas migrated")
	DB = db
}
