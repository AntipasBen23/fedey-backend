package handlers

import (
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/content"
)

type ContentHandler struct {
	service *content.Service
}

func NewContentHandler(service *content.Service) *ContentHandler {
	return &ContentHandler{service: service}
}

func (h *ContentHandler) ListDrafts(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list content drafts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
	})
}

func (h *ContentHandler) GenerateDrafts(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.Generate(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate content drafts")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"items": items,
	})
}

func (h *ContentHandler) GenerateVariants(w http.ResponseWriter, r *http.Request) {
	draftID := r.PathValue("id")

	item, err := h.service.GenerateVariants(r.Context(), draftID)
	if errors.Is(err, content.ErrInvalidVariantRequest) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, content.ErrDraftNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate content variants")
		return
	}

	writeJSON(w, http.StatusCreated, item)
}
