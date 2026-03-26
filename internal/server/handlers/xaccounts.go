package handlers

import (
	"errors"
	"net/http"

	"github.com/AntipasBen23/fedey-backend/internal/xaccounts"
)

type XAccountsHandler struct {
	service *xaccounts.Service
}

func NewXAccountsHandler(service *xaccounts.Service) *XAccountsHandler {
	return &XAccountsHandler{service: service}
}

func (h *XAccountsHandler) Status(w http.ResponseWriter, r *http.Request) {
	account, err := h.service.GetActive(r.Context())
	if errors.Is(err, xaccounts.ErrAccountNotConnected) {
		writeJSON(w, http.StatusOK, map[string]any{
			"connected": false,
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load x connection status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"connected": true,
		"account": map[string]any{
			"provider":    account.Provider,
			"userId":      account.UserID,
			"username":    account.Username,
			"scopes":      account.Scopes,
			"expiresAt":   account.ExpiresAt,
			"connectedAt": account.ConnectedAt,
		},
	})
}

func (h *XAccountsHandler) StartConnect(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.service.StartAuth(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start x oauth flow")
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *XAccountsHandler) Callback(w http.ResponseWriter, r *http.Request) {
	redirectURL, err := h.service.HandleCallback(
		r.Context(),
		r.URL.Query().Get("code"),
		r.URL.Query().Get("state"),
	)
	if errors.Is(err, xaccounts.ErrOAuthStateNotFound) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete x oauth flow")
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}
