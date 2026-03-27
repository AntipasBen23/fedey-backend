package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/AntipasBen23/fedey-backend/internal/content"
	"github.com/AntipasBen23/fedey-backend/internal/publishing"
)

type PublishingHandler struct {
	service *publishing.Service
}

func NewPublishingHandler(service *publishing.Service) *PublishingHandler {
	return &PublishingHandler{service: service}
}

func (h *PublishingHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list publishing schedules")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type createScheduleRequest struct {
	DraftID      string `json:"draftId"`
	VariantLabel string `json:"variantLabel"`
	Channel      string `json:"channel"`
	QueueProfile string `json:"queueProfile"`
	ScheduledFor string `json:"scheduledFor"`
}

func (h *PublishingHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var request createScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	scheduledFor, err := time.Parse(time.RFC3339, request.ScheduledFor)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid scheduledFor timestamp")
		return
	}

	item, err := h.service.Create(r.Context(), publishing.CreateInput{
		DraftID:      request.DraftID,
		VariantLabel: request.VariantLabel,
		Channel:      request.Channel,
		QueueProfile: request.QueueProfile,
		ScheduledFor: scheduledFor,
	})
	if errors.Is(err, publishing.ErrInvalidScheduleInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, content.ErrDraftNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create publishing schedule")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *PublishingHandler) MarkPublished(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.MarkPublished(r.Context(), r.PathValue("id"))
	if errors.Is(err, publishing.ErrInvalidScheduleInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, publishing.ErrScheduleNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark publishing schedule as published")
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (h *PublishingHandler) SyncPerformance(w http.ResponseWriter, r *http.Request) {
	count, err := h.service.SyncPublishedPerformance(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync publishing performance")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "synced",
		"metricsRecorded": count,
	})
}
