package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/brandmemory"
)

type BrandMemoryHandler struct {
	service *brandmemory.Service
}

func NewBrandMemoryHandler(service *brandmemory.Service) *BrandMemoryHandler {
	return &BrandMemoryHandler{service: service}
}

func (h *BrandMemoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	profile, err := h.service.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load brand memory")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *BrandMemoryHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var request brandmemory.UpsertInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	profile, err := h.service.Upsert(r.Context(), request)
	if errors.Is(err, brandmemory.ErrInvalidProfileInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update brand memory")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}
