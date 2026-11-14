package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

// APITokenHandler exposes the named-token lifecycle. It returns the plaintext
// exactly once, in the Create response; there is no endpoint that can read it
// back afterwards.
type APITokenHandler struct {
	service *service.APITokenService
	// authEnabled mirrors RouterOptions.ApiToken != "": whether the shared token
	// is currently enforcing auth. The page shows it so an operator can tell
	// "auth is off" from "auth is on and this token works".
	authEnabled bool
}

// NewAPITokenHandler builds the handler.
func NewAPITokenHandler(svc *service.APITokenService) *APITokenHandler {
	return &APITokenHandler{service: svc}
}

// SetAuthEnabled records whether the shared API_TOKEN is configured. It only
// changes what Status reports, never what is accepted.
func (h *APITokenHandler) SetAuthEnabled(enabled bool) { h.authEnabled = enabled }

type createAPITokenRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// Create mints a token. The plaintext appears only in this response.
func (h *APITokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAPITokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	plaintext, token, err := h.service.Generate(r.Context(), req.Name, req.Scopes)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"token":     plaintext,
		"api_token": token,
	})
}

// List returns every token, newest first. The stored hash is never included.
func (h *APITokenHandler) List(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.service.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tokens == nil {
		// An empty list must serialize as [], not null: the page iterates it.
		tokens = []model.APIToken{}
	}
	respondJSON(w, http.StatusOK, tokens)
}

// Revoke retires a token. The row is kept; the credential stops working at once.
func (h *APITokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid token ID")
		return
	}
	if err := h.service.Revoke(r.Context(), id); err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Status reports counts and whether the shared token is enforcing auth.
func (h *APITokenHandler) Status(w http.ResponseWriter, r *http.Request) {
	active, revoked, err := h.service.Counts(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"auth_enabled":            h.authEnabled,
		"global_token_configured": h.authEnabled,
		"active_count":            active,
		"revoked_count":           revoked,
	})
}
