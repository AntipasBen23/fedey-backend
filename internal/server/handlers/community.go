package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/community"
)

type CommunityHandler struct {
	service *community.Service
}

func NewCommunityHandler(service *community.Service) *CommunityHandler {
	return &CommunityHandler{service: service}
}

func (h *CommunityHandler) ListInbox(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list community inbox")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *CommunityHandler) CreateInboxItem(w http.ResponseWriter, r *http.Request) {
	var request community.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	item, err := h.service.Create(r.Context(), request)
	if errors.Is(err, community.ErrInvalidInboxInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create community inbox item")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *CommunityHandler) SyncXMentions(w http.ResponseWriter, r *http.Request) {
	created, err := h.service.SyncXMentions(r.Context())
	if errors.Is(err, community.ErrInvalidInboxInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync x mentions")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int{"created": created})
}

func (h *CommunityHandler) SyncLinkedInComments(w http.ResponseWriter, r *http.Request) {
	created, err := h.service.SyncLinkedInComments(r.Context())
	if errors.Is(err, community.ErrInvalidInboxInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync linkedin comments")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]int{"created": created})
}

func (h *CommunityHandler) DraftReply(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.DraftReply(r.Context(), r.PathValue("id"))
	if errors.Is(err, community.ErrInvalidInboxInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, community.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to draft reply")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (h *CommunityHandler) MarkReplied(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.MarkReplied(r.Context(), r.PathValue("id"))
	if errors.Is(err, community.ErrInvalidInboxInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, community.ErrItemNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to mark community reply sent")
		return
	}

	writeJSON(w, http.StatusOK, item)
}
