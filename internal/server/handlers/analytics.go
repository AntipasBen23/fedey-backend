package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/experiments"
)

type AnalyticsHandler struct {
	service *experiments.Service
}

func NewAnalyticsHandler(service *experiments.Service) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	var request experiments.RecordMetricInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	err := h.service.RecordMetric(r.Context(), request)
	if errors.Is(err, experiments.ErrInvalidMetricInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, experiments.ErrExperimentNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record analytics event")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
	})
}
