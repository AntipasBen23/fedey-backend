package database

import (
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
	err = db.AutoMigrate(
		&models.SocialAccount{}, 
		&models.ContentCalendar{},
		&models.UserStrategy{},
		&models.ScheduledPost{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}

	log.Println("Database connection successful and schemas migrated")
	DB = db
}
