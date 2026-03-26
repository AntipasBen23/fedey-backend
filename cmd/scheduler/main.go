package main

import (
	"context"
	"log"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/automation"
	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/common/config"
	"github.com/AntipasBen23/fedey-backend/internal/community"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
	linkedin "github.com/AntipasBen23/fedey-backend/internal/platform/linkedin"
	x "github.com/AntipasBen23/fedey-backend/internal/platform/x"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
	postgresstorage "github.com/AntipasBen23/fedey-backend/internal/storage/postgres"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	interval, err := time.ParseDuration(cfg.AutomationInterval())
	if err != nil {
		log.Fatalf("invalid FEDEY_AUTOMATION_INTERVAL: %v", err)
	}

	pool, err := postgresstorage.OpenPool(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to initialize database pool: %v", err)
	}
	if pool != nil {
		defer pool.Close()
	}

	xClient := x.NewClient(cfg.XAPIBaseURL(), cfg.XAccessToken(), cfg.XUserID())
	xAccountService := xaccounts.NewService(
		xaccounts.NewRepository(pool),
		xClient,
		cfg.XClientID(),
		cfg.XRedirectURI(),
		cfg.WebAppURL(),
	)
	linkedinClient := linkedin.NewClient(cfg.LinkedInAPIBaseURL())
	linkedinAccountService := linkedinaccounts.NewService(
		linkedinaccounts.NewRepository(pool),
		linkedinClient,
		cfg.LinkedInClientID(),
		cfg.LinkedInClientSecret(),
		cfg.LinkedInRedirectURI(),
		cfg.WebAppURL(),
	)

	experimentService := experiments.NewService(experiments.NewRepository(pool))
	brandMemoryService := brandmemory.NewService(brandmemory.NewRepository(pool))
	trendService := trends.NewService(trends.NewRepository(pool))
	contentService := content.NewService(content.NewRepository(pool), brandMemoryService, trendService, experimentService)
	publishingService := publishing.NewService(publishing.NewRepository(pool), contentService, xClient, xAccountService, linkedinClient, linkedinAccountService)
	communityService := community.NewService(community.NewRepository(pool), brandMemoryService, xClient, xAccountService)
	automationService := automation.NewService(automation.NewRepository(pool), contentService, publishingService, communityService, automation.Settings{
		Interval: cfg.AutomationInterval(),
		Windows:  publishing.ParseWindows(cfg.PublishWindows()),
	})

	log.Printf("scheduler started with interval %s", interval)
	runOnce(ctx, automationService)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		runOnce(ctx, automationService)
	}
}

func runOnce(ctx context.Context, automationService *automation.Service) {
	run, err := automationService.Run(ctx, "scheduler")
	if err != nil {
		log.Printf("automation run failed: %v", err)
		return
	}

	log.Printf(
		"automation run completed: published=%d drafts=%d schedules=%d replies=%d",
		run.PostsPublished,
		run.DraftsGenerated,
		run.SchedulesCreated,
		run.RepliesDrafted,
	)
}
