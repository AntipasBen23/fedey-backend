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

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/common/config"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
	"github.com/AntipasBen23/fedey-backend/internal/server"
	postgresstorage "github.com/AntipasBen23/fedey-backend/internal/storage/postgres"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
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

	experimentRepository := experiments.NewRepository(pool)
	experimentService := experiments.NewService(experimentRepository)
	brandMemoryRepository := brandmemory.NewRepository(pool)
	brandMemoryService := brandmemory.NewService(brandMemoryRepository)
	trendRepository := trends.NewRepository(pool)
	trendService := trends.NewService(trendRepository)
	contentRepository := content.NewRepository(pool)
	contentService := content.NewService(contentRepository, brandMemoryService, trendService, experimentService)
	publishingRepository := publishing.NewRepository(pool)
	publishingService := publishing.NewService(publishingRepository, contentService)

	httpServer := &http.Server{
		Addr: cfg.APIAddress(),
		Handler: server.NewRouter(server.Dependencies{
			ExperimentService:  experimentService,
			BrandMemoryService: brandMemoryService,
			TrendService:       trendService,
			ContentService:     contentService,
			PublishingService:  publishingService,
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
