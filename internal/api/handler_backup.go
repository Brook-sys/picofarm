package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

type BackupHandler struct {
	service *service.BackupService
}

// List returns all available backups.
func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	backups, err := h.service.ListBackups(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, backups)
}

// Create creates a new backup.
func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	backup, err := h.service.CreateBackup(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, backup)
}

// Delete removes a backup.
func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "backup name required")
		return
	}

	if err := h.service.DeleteBackup(r.Context(), name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Restore restores the database from a backup.
// WARNING: This will restart the application!
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "backup name required")
		return
	}

	if err := h.service.RestoreBackup(r.Context(), name); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Database restored. Please restart the application.",
	})
}

// GetConfig returns the backup configuration.
func (h *BackupHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	config := h.service.GetConfig(r.Context())
	respondJSON(w, http.StatusOK, config)
}

// UpdateConfig updates the backup configuration.
func (h *BackupHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var config service.BackupConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.UpdateConfig(r.Context(), config); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, config)
}
