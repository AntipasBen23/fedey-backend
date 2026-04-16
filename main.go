package main

import (
"log"
"os"

"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/handlers"
	"github.com/AntipasBen23/fedey-backend/worker"
	"github.com/gin-contrib/cors"
"github.com/gin-gonic/gin"
"github.com/joho/godotenv"
)

func main() {
_ = godotenv.Load()
database.InitDB()
worker.StartScheduler()

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

		// Trends & Social Listening
		v1.GET("/trends", handlers.GetTrendsHandler)
		v1.POST("/trends/react", handlers.ReactToTrendHandler)
		v1.POST("/trends/react/schedule", handlers.ScheduleReactionHandler)

		// Analytics
		v1.GET("/analytics", handlers.GetAnalyticsHandler)
		v1.POST("/analytics/sync", handlers.SyncAnalyticsHandler)

		// Posts Management
		v1.PUT("/posts/:id", handlers.UpdatePostHandler)
		v1.DELETE("/posts/:id", handlers.DeletePostHandler)

		// Conversational AI
		v1.POST("/chat", handlers.ChatWithFurciHandler)

		// Script Engine (video scripts, carousels, threads)
		v1.POST("/scripts/generate", handlers.GenerateScriptHandler)

		// Carousel Image Generation (DALL-E 3) and Carousel Design (FFmpeg)
		v1.POST("/carousel/images", handlers.GenerateCarouselImagesHandler)
		v1.POST("/carousel/design", handlers.GenerateCarouselDesignHandler)

		// Plan management
		v1.GET("/plan", handlers.GetPlanHandler)
		v1.POST("/plan/upgrade", handlers.UpgradePlanHandler)

		// Video Generation — template (FFmpeg, free) and AI (Runway ML, pro only)
		v1.POST("/video/template", handlers.GenerateTemplateVideoHandler)
		v1.POST("/video/generate", handlers.GenerateVideoHandler)
		v1.GET("/video/status/:taskId", handlers.GetVideoStatusHandler)
		v1.DELETE("/video/task/:taskId", handlers.CancelVideoTaskHandler)
	}

port := os.Getenv("PORT")
if port == "" {
port = "8080"
}

log.Printf("Server starting on port %s", port)
r.Run(":" + port)
}
