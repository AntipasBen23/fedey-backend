package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	"github.com/AntipasBen23/fedey-backend/internal/security/tokens"
	"github.com/AntipasBen23/fedey-backend/internal/server"
	postgresstorage "github.com/AntipasBen23/fedey-backend/internal/storage/postgres"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := postgresstorage.OpenPool(ctx, cfg.DatabaseURL())
	if err != nil {
		log.Fatalf("failed to initialize database pool: %v", err)
	}
	if pool != nil {
		defer pool.Close()
	}

	tokenCipher, err := resolveTokenCipher(cfg.EncryptionKey())
	if err != nil {
		log.Fatalf("failed to initialize token encryption: %v", err)
	}

	xClient := x.NewClient(cfg.XAPIBaseURL(), cfg.XAccessToken(), cfg.XUserID())
	xAccountRepository := xaccounts.NewRepository(pool, tokenCipher)
	xAccountService := xaccounts.NewService(
		xAccountRepository,
		xClient,
		cfg.XClientID(),
		cfg.XRedirectURI(),
		cfg.WebAppURL(),
	)
	linkedinClient := linkedin.NewClient(cfg.LinkedInAPIBaseURL())
	linkedinAccountService := linkedinaccounts.NewService(
		linkedinaccounts.NewRepository(pool, tokenCipher),
		linkedinClient,
		cfg.LinkedInClientID(),
		cfg.LinkedInClientSecret(),
		cfg.LinkedInRedirectURI(),
		cfg.WebAppURL(),
	)

	experimentRepository := experiments.NewRepository(pool)
	experimentService := experiments.NewService(experimentRepository)
	brandMemoryRepository := brandmemory.NewRepository(pool)
	brandMemoryService := brandmemory.NewService(brandMemoryRepository)
	trendRepository := trends.NewRepository(pool)
	trendService := trends.NewService(trendRepository)
	contentRepository := content.NewRepository(pool)
	contentService := content.NewService(contentRepository, brandMemoryService, trendService, experimentService)
	publishingRepository := publishing.NewRepository(pool)
	publishingService := publishing.NewService(publishingRepository, contentService, xClient, xAccountService, linkedinClient, linkedinAccountService)
	communityRepository := community.NewRepository(pool)
	communityService := community.NewService(communityRepository, brandMemoryService, publishingService, xClient, xAccountService, linkedinClient, linkedinAccountService)
	automationRepository := automation.NewRepository(pool)
	automationService := automation.NewService(automationRepository, brandMemoryService, trendService, contentService, publishingService, communityService, automation.Settings{
		Interval: cfg.AutomationInterval(),
		Windows:  publishing.ParseWindows(cfg.PublishWindows()),
	})

	httpServer := &http.Server{
		Addr: cfg.APIAddress(),
		Handler: server.NewRouter(server.Dependencies{
			ExperimentService:  experimentService,
			BrandMemoryService: brandMemoryService,
			TrendService:       trendService,
			ContentService:     contentService,
			PublishingService:  publishingService,
			CommunityService:   communityService,
			AutomationService:  automationService,
			XAccountService:    xAccountService,
			LinkedInService:    linkedinAccountService,
		}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		waitForShutdown(httpServer)
	}()

	log.Printf("api server listening on %s", cfg.APIAddress())
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("api server failed: %v", err)
	}

	<-shutdownDone
}

func resolveTokenCipher(key string) (tokens.Cipher, error) {
	if key == "" {
		return tokens.NewNoopCipher(), nil
	}

	return tokens.NewAESCipher(key)
}

func waitForShutdown(httpServer *http.Server) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	log.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		return
	}

	log.Println("api server stopped gracefully")
}
