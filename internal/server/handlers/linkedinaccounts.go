package handlers

import (
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/linkedinaccounts"
)

type LinkedInAccountsHandler struct {
	service *linkedinaccounts.Service
}

func NewLinkedInAccountsHandler(service *linkedinaccounts.Service) *LinkedInAccountsHandler {
	return &LinkedInAccountsHandler{service: service}
}

func (h *LinkedInAccountsHandler) Status(w http.ResponseWriter, r *http.Request) {
	account, err := h.service.GetActive(r.Context())
	if errors.Is(err, linkedinaccounts.ErrAccountNotConnected) {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load linkedin connection status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connected": true,
		"account": map[string]any{
			"provider":    account.Provider,
			"memberId":    account.MemberID,
			"displayName": account.DisplayName,
			"authorUrn":   account.AuthorURN,
			"scopes":      account.Scopes,
			"expiresAt":   account.ExpiresAt,
			"connectedAt": account.ConnectedAt,
		},
	})
}

func (h *LinkedInAccountsHandler) StartConnect(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.service.StartAuth(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start linkedin oauth flow")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *LinkedInAccountsHandler) Callback(w http.ResponseWriter, r *http.Request) {
	redirectURL, err := h.service.HandleCallback(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	if errors.Is(err, linkedinaccounts.ErrOAuthStateNotFound) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete linkedin oauth flow")
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}
