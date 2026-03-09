package server

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/server/handlers"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux)
	return mux
}

func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", handlers.Healthz)
	mux.HandleFunc("/v1/health", handlers.HealthV1)
}
