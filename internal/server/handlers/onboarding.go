package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/onboarding"
)

type OnboardingHandler struct {
	service *onboarding.Service
}

func NewOnboardingHandler(service *onboarding.Service) *OnboardingHandler {
	return &OnboardingHandler{service: service}
}

func (h *OnboardingHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list onboarding sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *OnboardingHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var request onboarding.CreateSessionInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	item, err := h.service.CreateSession(r.Context(), request)
	if errors.Is(err, onboarding.ErrInvalidSessionInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create onboarding session")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *OnboardingHandler) AnswerQuestion(w http.ResponseWriter, r *http.Request) {
	var request onboarding.AnswerQuestionInput
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	request.SessionID = r.PathValue("id")

	item, err := h.service.AnswerQuestion(r.Context(), request)
	if errors.Is(err, onboarding.ErrInvalidSessionInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, onboarding.ErrSessionNotFound) || errors.Is(err, onboarding.ErrQuestionNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to answer onboarding question")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *OnboardingHandler) RunAudit(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.RunAudit(r.Context(), r.PathValue("id"))
	if errors.Is(err, onboarding.ErrInvalidSessionInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, onboarding.ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to run onboarding audit")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *OnboardingHandler) Activate(w http.ResponseWriter, r *http.Request) {
	item, err := h.service.Activate(r.Context(), r.PathValue("id"))
	if errors.Is(err, onboarding.ErrInvalidSessionInput) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, onboarding.ErrSessionNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to activate onboarding session")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
