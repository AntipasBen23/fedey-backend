package server

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/server/handlers"
	"github.com/AntipasBen23/fedey-backend/internal/trends"
)

type Dependencies struct {
	ExperimentService  *experiments.Service
	BrandMemoryService *brandmemory.Service
	TrendService       *trends.Service
	ContentService     *content.Service
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, deps)
	return mux
}

func registerRoutes(mux *http.ServeMux, deps Dependencies) {
	experimentsHandler := handlers.NewExperimentsHandler(deps.ExperimentService)
	analyticsHandler := handlers.NewAnalyticsHandler(deps.ExperimentService)
	brandMemoryHandler := handlers.NewBrandMemoryHandler(deps.BrandMemoryService)
	trendsHandler := handlers.NewTrendsHandler(deps.TrendService)
	strategyHandler := handlers.NewStrategyHandler(deps.BrandMemoryService, deps.TrendService)
	contentHandler := handlers.NewContentHandler(deps.ContentService)

	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.HandleFunc("GET /v1/health", handlers.HealthV1)
	mux.HandleFunc("GET /v1/strategy/snapshot", strategyHandler.GetSnapshot)
	mux.HandleFunc("GET /v1/brand-memory", brandMemoryHandler.Get)
	mux.HandleFunc("PUT /v1/brand-memory", brandMemoryHandler.Upsert)
	mux.HandleFunc("GET /v1/trends", trendsHandler.List)
	mux.HandleFunc("POST /v1/trends", trendsHandler.Create)
	mux.HandleFunc("GET /v1/content/drafts", contentHandler.ListDrafts)
	mux.HandleFunc("POST /v1/content/drafts/generate", contentHandler.GenerateDrafts)
	mux.HandleFunc("POST /v1/content/drafts/{id}/variants/generate", contentHandler.GenerateVariants)

	mux.HandleFunc("POST /v1/experiments", experimentsHandler.Create)
	mux.HandleFunc("GET /v1/experiments", experimentsHandler.List)
	mux.HandleFunc("PATCH /v1/experiments/{id}/status", experimentsHandler.PatchStatus)
	mux.HandleFunc("POST /v1/analytics/events", analyticsHandler.RecordEvent)
}
