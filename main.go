package main

import (
"log"
"os"

"github.com/AntipasBen23/fedey-backend/database"
"github.com/AntipasBen23/fedey-backend/handlers"
"github.com/gin-contrib/cors"
"github.com/gin-gonic/gin"
"github.com/joho/godotenv"
)

func main() {
_ = godotenv.Load()
database.InitDB()

r := gin.Default()

r.Use(cors.New(cors.Config{
AllowOrigins:     []string{"*"},
AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
ExposeHeaders:    []string{"Content-Length"},
AllowCredentials: true,
}))

v1 := r.Group("/v1")
{
v1.POST("/analyze", handlers.AnalyzeHandler)
v1.POST("/auth/callback", handlers.AuthCallbackHandler)
v1.DELETE("/auth/disconnect", handlers.DisconnectHandler)
v1.POST("/strategy", handlers.StrategyHandler)
v1.POST("/strategy/refine", handlers.StrategyRefineHandler)
v1.POST("/calendar", handlers.CalendarHandler)
v1.POST("/calendar/approve", handlers.ApproveCalendarHandler)
v1.GET("/dashboard", handlers.GetDashboardHandler)
v1.POST("/settings/autopilot", handlers.ToggleAutoPilotHandler)
v1.POST("/revise", handlers.ReviseHandler)
}

port := os.Getenv("PORT")
if port == "" {
port = "8080"
}

log.Printf("Server starting on port %s", port)
r.Run(":" + port)
}
