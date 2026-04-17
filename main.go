package main

import (
	"log"
	"os"

	"github.com/AntipasBen23/fedey-backend/database"
	"github.com/AntipasBen23/fedey-backend/handlers"
	"github.com/AntipasBen23/fedey-backend/middleware"
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
		// ── Page tracking (public) ─────────────────────────────────────────
		v1.POST("/track", handlers.TrackPageViewHandler)

		// ── Admin auth (public) ─────────────────────────────────────────────
		v1.POST("/admin/login", handlers.AdminLoginHandler)

		// ── Admin protected routes ──────────────────────────────────────────
		admin := v1.Group("/admin", middleware.RequireAdmin())
		{
			admin.GET("/stats", handlers.AdminStatsHandler)
			admin.GET("/users", handlers.AdminListUsersHandler)
			admin.DELETE("/users/:id", handlers.AdminDeleteUserHandler)
			admin.PATCH("/users/:id/plan", handlers.AdminUpdateUserPlanHandler)
			admin.GET("/visitors/timeline", handlers.AdminVisitorTimelineHandler)
			admin.GET("/visitors/top-pages", handlers.AdminTopPagesHandler)
			admin.GET("/visitors/devices", handlers.AdminDevicesHandler)
			admin.GET("/activity", handlers.AdminActivityHandler)
		}

		// ── Public auth routes ──────────────────────────────────────────────
		v1.POST("/user/register", handlers.RegisterHandler)
		v1.POST("/user/verify-email", handlers.VerifyEmailHandler)
		v1.POST("/user/resend-code", handlers.ResendCodeHandler)
		v1.POST("/user/login", handlers.LoginHandler)
		v1.POST("/user/refresh", handlers.RefreshTokenHandler)
		v1.POST("/user/google", handlers.GoogleAuthHandler)
		v1.POST("/user/forgot-password", handlers.ForgotPasswordHandler)
		v1.POST("/user/reset-password", handlers.ResetPasswordHandler)
		v1.POST("/user/logout", handlers.LogoutHandler)

		// ── Protected user route ────────────────────────────────────────────
		v1.GET("/user/me", middleware.RequireAuth(), handlers.GetMeHandler)

		// ── Existing routes (kept open for now — add RequireAuth() as needed) ─
		v1.POST("/analyze", handlers.AnalyzeHandler)
		v1.POST("/auth/callback", handlers.AuthCallbackHandler)
		v1.DELETE("/auth/disconnect", handlers.DisconnectHandler)
		v1.POST("/strategy", handlers.StrategyHandler)
		v1.POST("/strategy/refine", handlers.StrategyRefineHandler)
		v1.POST("/calendar", handlers.CalendarHandler)
		v1.GET("/calendar/status", handlers.GetCalendarStatusHandler)
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
		v1.GET("/analytics/peak-hours", handlers.GetPeakHoursHandler)

		// Posts Management
		v1.PUT("/posts/:id", handlers.UpdatePostHandler)
		v1.DELETE("/posts/:id", handlers.DeletePostHandler)
		v1.POST("/posts", handlers.CreatePostHandler)
		v1.POST("/posts/polish", handlers.AIPolishHandler)

		// Conversational AI
		v1.POST("/chat", handlers.ChatWithFurciHandler)

		// Engagement & Ghost Operator
		v1.GET("/engagements", handlers.GetEngagementsHandler)
		v1.POST("/engagements/:id/approve", handlers.ApproveEngagementHandler)
		v1.POST("/settings/ghost-mode", handlers.ToggleGhostModeHandler)

		// Script Engine
		v1.POST("/scripts/generate", handlers.GenerateScriptHandler)

		// Carousel
		v1.POST("/carousel/images", handlers.GenerateCarouselImagesHandler)
		v1.POST("/carousel/design", handlers.GenerateCarouselDesignHandler)

		// Plan management
		v1.GET("/plan", handlers.GetPlanHandler)
		v1.POST("/plan/upgrade", handlers.UpgradePlanHandler)

		// Video Generation
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
