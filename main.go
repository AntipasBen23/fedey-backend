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
		// Explicit origins are required when AllowCredentials is true —
		// browsers reject "*" with credentials per the CORS spec.
		AllowOrigins:     []string{"https://furciai.com", "https://www.furciai.com", "http://localhost:3000", "https://furci-ai-admin.netlify.app"},
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
		v1.POST("/admin/setup", handlers.AdminSetupHandler) // one-time account creation
		v1.POST("/admin/login", handlers.AdminLoginHandler)

		// ── Admin protected routes ──────────────────────────────────────────
		admin := v1.Group("/admin", middleware.RequireAdmin())
		{
			admin.GET("/stats", handlers.AdminStatsHandler)
			admin.GET("/users", handlers.AdminListUsersHandler)
			admin.DELETE("/users/:id", handlers.AdminDeleteUserHandler)
			admin.PATCH("/users/:id/plan", handlers.AdminUpdateUserPlanHandler)
			admin.GET("/visitors", handlers.AdminVisitorSessionsHandler)
			admin.GET("/visitors/:key", handlers.AdminVisitorDetailHandler)
			admin.GET("/visitors/timeline", handlers.AdminVisitorTimelineHandler)
			admin.GET("/visitors/top-pages", handlers.AdminTopPagesHandler)
			admin.GET("/visitors/devices", handlers.AdminDevicesHandler)
			admin.GET("/activity", handlers.AdminActivityHandler)
			admin.POST("/reports/test", handlers.TestDailyReportHandler)
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

		// ── Protected user routes ───────────────────────────────────────────
		v1.GET("/user/me", middleware.RequireAuth(), handlers.GetMeHandler)
		v1.PATCH("/user/onboarding", middleware.RequireAuth(), handlers.UpdateOnboardingHandler)
		v1.PATCH("/user/username", middleware.RequireAuth(), handlers.SetUsernameHandler)

		// ── Protected app routes ────────────────────────────────────────────────
		v1.POST("/analyze", middleware.RequireAuth(), handlers.AnalyzeHandler)
		v1.POST("/auth/callback", middleware.RequireAuth(), handlers.AuthCallbackHandler)
		v1.DELETE("/auth/disconnect", middleware.RequireAuth(), handlers.DisconnectHandler)
		v1.POST("/strategy", middleware.RequireAuth(), handlers.StrategyHandler)
		v1.POST("/strategy/refine", middleware.RequireAuth(), handlers.StrategyRefineHandler)
		v1.POST("/calendar", middleware.RequireAuth(), handlers.CalendarHandler)
		v1.GET("/calendar/status", middleware.RequireAuth(), handlers.GetCalendarStatusHandler)
		v1.POST("/calendar/approve", middleware.RequireAuth(), handlers.ApproveCalendarHandler)
		v1.GET("/dashboard", middleware.RequireAuth(), handlers.GetDashboardHandler)
		v1.POST("/settings/autopilot", middleware.RequireAuth(), handlers.ToggleAutoPilotHandler)
		v1.POST("/revise", middleware.RequireAuth(), handlers.ReviseHandler)

		// Trends & Social Listening
		v1.GET("/trends", middleware.RequireAuth(), handlers.GetTrendsHandler)
		v1.POST("/trends/react", middleware.RequireAuth(), handlers.ReactToTrendHandler)
		v1.POST("/trends/react/schedule", middleware.RequireAuth(), handlers.ScheduleReactionHandler)

		// Analytics
		v1.GET("/analytics", middleware.RequireAuth(), handlers.GetAnalyticsHandler)
		v1.POST("/analytics/sync", middleware.RequireAuth(), handlers.SyncAnalyticsHandler)
		v1.GET("/analytics/peak-hours", middleware.RequireAuth(), handlers.GetPeakHoursHandler)

		// Posts Management
		v1.PUT("/posts/:id", middleware.RequireAuth(), handlers.UpdatePostHandler)
		v1.DELETE("/posts/:id", middleware.RequireAuth(), handlers.DeletePostHandler)
		v1.POST("/posts", middleware.RequireAuth(), handlers.CreatePostHandler)
		v1.POST("/posts/polish", middleware.RequireAuth(), handlers.AIPolishHandler)

		// Conversational AI
		v1.POST("/chat", middleware.RequireAuth(), handlers.ChatWithFurciHandler)

		// Engagement & Ghost Operator
		v1.GET("/engagements", middleware.RequireAuth(), handlers.GetEngagementsHandler)
		v1.POST("/engagements/:id/approve", middleware.RequireAuth(), handlers.ApproveEngagementHandler)
		v1.POST("/settings/ghost-mode", middleware.RequireAuth(), handlers.ToggleGhostModeHandler)

		// Script Engine
		v1.POST("/scripts/generate", middleware.RequireAuth(), handlers.GenerateScriptHandler)

		// Carousel
		v1.POST("/carousel/images", middleware.RequireAuth(), handlers.GenerateCarouselImagesHandler)
		v1.POST("/carousel/design", middleware.RequireAuth(), handlers.GenerateCarouselDesignHandler)

		// Plan management
		v1.GET("/plan", middleware.RequireAuth(), handlers.GetPlanHandler)
		v1.POST("/plan/upgrade", middleware.RequireAuth(), handlers.UpgradePlanHandler)

		// Video Generation
		v1.POST("/video/template", middleware.RequireAuth(), handlers.GenerateTemplateVideoHandler)
		v1.POST("/video/generate", middleware.RequireAuth(), handlers.GenerateVideoHandler)
		v1.GET("/video/status/:taskId", middleware.RequireAuth(), handlers.GetVideoStatusHandler)
		v1.DELETE("/video/task/:taskId", middleware.RequireAuth(), handlers.CancelVideoTaskHandler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	r.Run(":" + port)
}
