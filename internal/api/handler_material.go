package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/validation"
)

type MaterialHandler struct {
	service *service.MaterialService
}

// List returns all materials.
func (h *MaterialHandler) List(w http.ResponseWriter, r *http.Request) {
	var materials []model.Material
	var err error

	if typeFilter := r.URL.Query().Get("type"); typeFilter != "" {
		materials, err = h.service.ListByType(r.Context(), model.MaterialType(typeFilter))
	} else {
		materials, err = h.service.List(r.Context())
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if materials == nil {
		materials = []model.Material{}
	}

	respondJSON(w, http.StatusOK, materials)
}

// Create creates a new material.
func (h *MaterialHandler) Create(w http.ResponseWriter, r *http.Request) {
	var material model.Material
	if err := json.NewDecoder(r.Body).Decode(&material); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	v := validation.New()
	v.Required("name", material.Name)
	v.MaxLength("name", material.Name, 255)
	v.Required("type", string(material.Type))
	v.MaxLength("type", string(material.Type), 50)
	v.MaxLength("manufacturer", material.Manufacturer, 255)
	v.MaxLength("color", material.Color, 100)
	v.NonNegativeFloat("density", material.Density)
	v.NonNegativeFloat("cost_per_kg", material.CostPerKg)
	if err := v.Error(); err != nil {
		respondValidationError(w, err)
		return
	}

	if err := h.service.Create(r.Context(), &material); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, material)
}

// Get returns a material by ID.
func (h *MaterialHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid material ID")
		return
	}

	material, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if material == nil {
		respondError(w, http.StatusNotFound, "material not found")
		return
	}

	respondJSON(w, http.StatusOK, material)
}

// Update updates a material.
func (h *MaterialHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid material ID")
		return
	}
	var material model.Material
	if err := json.NewDecoder(r.Body).Decode(&material); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	material.ID = id
	if err := h.service.Update(r.Context(), &material); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, material)
}

// Delete removes a material by ID.
func (h *MaterialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid material ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SpoolHandler handles spool endpoints.
