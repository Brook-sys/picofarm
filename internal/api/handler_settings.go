package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

type SettingsHandler struct {
	service *service.SettingsService
}

// List returns all settings (with sensitive values masked).
func (h *SettingsHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Mask sensitive values
	type maskedSetting struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		UpdatedAt string `json:"updated_at"`
	}
	result := make([]maskedSetting, 0, len(settings))
	for _, s := range settings {
		val := s.Value
		if isSensitiveKey(s.Key) && len(val) > 8 {
			val = val[:4] + "..." + val[len(val)-4:]
		}
		result = append(result, maskedSetting{
			Key:       s.Key,
			Value:     val,
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	respondJSON(w, http.StatusOK, result)
}

// Get returns a single setting (with sensitive values masked).
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "key is required")
		return
	}

	setting, err := h.service.Get(r.Context(), key)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if setting == nil {
		respondError(w, http.StatusNotFound, "setting not found")
		return
	}

	val := setting.Value
	if isSensitiveKey(key) && len(val) > 8 {
		val = val[:4] + "..." + val[len(val)-4:]
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"key":        setting.Key,
		"value":      val,
		"updated_at": setting.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Set creates or updates a setting.
func (h *SettingsHandler) Set(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "key is required")
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Set(r.Context(), key, req.Value); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Delete removes a setting.
func (h *SettingsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "key is required")
		return
	}

	if err := h.service.Delete(r.Context(), key); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// isSensitiveKey returns true for keys that should have their values masked in GET responses.
func isSensitiveKey(key string) bool {
	return strings.Contains(key, "api_key") || strings.Contains(key, "secret") || strings.Contains(key, "token") || strings.Contains(key, "password")
}

// BambuCloudHandler handles Bambu Cloud integration endpoints.
