package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/validation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// respondJSON sends a JSON response.
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode JSON response", "error", err)
		}
	}
	// Flush if the ResponseWriter supports it
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// respondError sends an error response.
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// respondValidationError sends a validation error response.
func respondValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(*validation.ValidationError); ok {
		respondJSON(w, http.StatusBadRequest, ve)
		return
	}
	respondError(w, http.StatusBadRequest, err.Error())
}

// parseUUID parses a UUID from URL parameter.
func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, param))
}

// ProjectHandler handles project endpoints.
