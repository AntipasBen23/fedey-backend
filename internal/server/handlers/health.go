package handlers

import (
	"encoding/json"
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
