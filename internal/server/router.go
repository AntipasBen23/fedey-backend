package server

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/experiments"
	"github.com/AntipasBen23/fedey-backend/internal/server/handlers"
)

type Dependencies struct {
	ExperimentService *experiments.Service
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, deps)
	return mux
}

func registerRoutes(mux *http.ServeMux, deps Dependencies) {
	experimentsHandler := handlers.NewExperimentsHandler(deps.ExperimentService)

	mux.HandleFunc("GET /healthz", handlers.Healthz)
	mux.HandleFunc("GET /v1/health", handlers.HealthV1)
	mux.HandleFunc("GET /v1/strategy/snapshot", handlers.StrategySnapshotV1)

	mux.HandleFunc("POST /v1/experiments", experimentsHandler.Create)
	mux.HandleFunc("GET /v1/experiments", experimentsHandler.List)
	mux.HandleFunc("PATCH /v1/experiments/{id}/status", experimentsHandler.PatchStatus)
}
