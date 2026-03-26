package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/trends"
)

type TrendsHandler struct {
	service *trends.Service
}

func NewTrendsHandler(service *trends.Service) *TrendsHandler {
	return &TrendsHandler{service: service}
}

func (h *TrendsHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list trends")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *TrendsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request trends.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	item, err := h.service.Create(r.Context(), request)
	if errors.Is(err, trends.ErrInvalidSignalInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create trend signal")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *TrendsHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	var request trends.LiveIngestInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	items, err := h.service.IngestLive(r.Context(), request)
	if errors.Is(err, trends.ErrInvalidSignalInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to ingest live trend signals")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"items": items})
}
