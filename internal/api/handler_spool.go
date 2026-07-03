package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

type SpoolHandler struct {
	service *service.SpoolService
}

// List returns all spools.
func (h *SpoolHandler) List(w http.ResponseWriter, r *http.Request) {
	spools, err := h.service.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if spools == nil {
		spools = []model.MaterialSpool{}
	}

	respondJSON(w, http.StatusOK, spools)
}

// Create creates a new spool.
func (h *SpoolHandler) Create(w http.ResponseWriter, r *http.Request) {
	var spool model.MaterialSpool
	if err := json.NewDecoder(r.Body).Decode(&spool); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Create(r.Context(), &spool); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, spool)
}

// Get returns a spool by ID.
func (h *SpoolHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid spool ID")
		return
	}

	spool, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if spool == nil {
		respondError(w, http.StatusNotFound, "spool not found")
		return
	}

	respondJSON(w, http.StatusOK, spool)
}

// Update updates a spool.
func (h *SpoolHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid spool ID")
		return
	}
	var spool model.MaterialSpool
	if err := json.NewDecoder(r.Body).Decode(&spool); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	spool.ID = id
	if err := h.service.Update(r.Context(), &spool); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, spool)
}

// Delete deletes a spool by ID.
func (h *SpoolHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid spool ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PrintJobHandler handles print job endpoints.
