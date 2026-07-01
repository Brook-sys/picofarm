package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
)

type ThingiverseSettingsHandler struct {
	settings *service.SettingsService
}

func NewThingiverseSettingsHandler(settings *service.SettingsService) *ThingiverseSettingsHandler {
	return &ThingiverseSettingsHandler{settings: settings}
}

func (h *ThingiverseSettingsHandler) GetToken(w http.ResponseWriter, r *http.Request) {
	setting, err := h.settings.Get(r.Context(), "thingiverse_api_token")
	if err != nil || setting == nil {
		respondJSON(w, http.StatusOK, map[string]string{"token": ""})
		return
	}
	// Always return masked token
	respondJSON(w, http.StatusOK, map[string]string{"token": "********"})
}

func (h *ThingiverseSettingsHandler) SetToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.settings.Set(r.Context(), "thingiverse_api_token", req.Token); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
