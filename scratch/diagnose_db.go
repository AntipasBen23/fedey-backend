package main

import (
	"fmt"
	"log"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/models"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("../.env")
	database.InitDB()

	if database.DB == nil {
		log.Fatal("Database not initialized")
	}

	fmt.Println("\n--- USER LIST ---")
	var users []models.User
	database.DB.Find(&users)
	for _, u := range users {
		fmt.Printf("ID: %d | Email: %s | Name: %s\n", u.ID, u.Email, u.Name)
	}

	fmt.Println("\n--- SOCIAL ACCOUNTS ---")
	var accounts []models.SocialAccount
	database.DB.Find(&accounts)
	for _, a := range accounts {
		fmt.Printf("ID: %d | UserID: %d | Platform: %s\n", a.ID, a.UserID, a.Platform)
	}
	
	fmt.Println("\n--- DIAGNOSIS SUMMARY ---")
	if len(users) > 1 {
		fmt.Printf("CRITICAL: You have %d users. My backfill assigned everything to User ID %d.\n", len(users), users[0].ID)
		fmt.Println("If you are logged in as a different ID, you won't see your accounts.")
	} else if len(users) == 1 {
		fmt.Println("You only have one user. The issue might be something else.")
	} else {
		fmt.Println("No users found.")
	}
}
