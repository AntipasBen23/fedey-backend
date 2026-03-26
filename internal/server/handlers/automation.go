package handlers

import (
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/automation"
)

type AutomationHandler struct {
	service *automation.Service
}

func NewAutomationHandler(service *automation.Service) *AutomationHandler {
	return &AutomationHandler{service: service}
}

func (h *AutomationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list automation runs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AutomationHandler) RunOnce(w http.ResponseWriter, r *http.Request) {
	run, err := h.service.Run(r.Context(), "manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to run automation")
		return
	}

	writeJSON(w, http.StatusCreated, run)
}
