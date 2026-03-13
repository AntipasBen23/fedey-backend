package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/experiments"
)

type ExperimentsHandler struct {
	service *experiments.Service
}

func NewExperimentsHandler(service *experiments.Service) *ExperimentsHandler {
	return &ExperimentsHandler{service: service}
}

func (h *ExperimentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request experiments.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	created, err := h.service.Create(r.Context(), request)
	if errors.Is(err, experiments.ErrInvalidExperimentInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create experiment")
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (h *ExperimentsHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list experiments")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

type patchStatusRequest struct {
	Status experiments.Status `json:"status"`
}

func (h *ExperimentsHandler) PatchStatus(w http.ResponseWriter, r *http.Request) {
	experimentID := r.PathValue("id")

	var request patchStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	updated, err := h.service.UpdateStatus(r.Context(), experimentID, request.Status)
	if errors.Is(err, experiments.ErrInvalidStatus) ||
		errors.Is(err, experiments.ErrInvalidExperimentInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, experiments.ErrExperimentNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update experiment")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}
