package handlers

import (
	"net/http"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeHealth(w)
}

func HealthV1(w http.ResponseWriter, _ *http.Request) {
	writeHealth(w)
}

func writeHealth(w http.ResponseWriter) {
	response := healthResponse{
		Status:    "ok",
		Service:   "fedey-backend-api",
		Version:   "v0",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, response)
}
