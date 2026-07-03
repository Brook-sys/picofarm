package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/validation"
	"github.com/google/uuid"
)

type SaleHandler struct {
	service *service.SaleService
}

// List returns all sales.
func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request) {
	var projectID *uuid.UUID
	if pidStr := r.URL.Query().Get("project_id"); pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			projectID = &pid
		}
	}

	sales, err := h.service.List(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if sales == nil {
		sales = []model.Sale{}
	}

	respondJSON(w, http.StatusOK, sales)
}

// Create creates a new sale.
func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var sale model.Sale
	if err := json.NewDecoder(r.Body).Decode(&sale); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	v := validation.New()
	v.MaxLength("channel", string(sale.Channel), 100)
	v.MaxLength("platform", sale.Platform, 100)
	v.MaxLength("customer_name", sale.CustomerName, 255)
	v.MaxLength("order_reference", sale.OrderReference, 255)
	v.MaxLength("item_description", sale.ItemDescription, 1000)
	v.NonNegative("gross_cents", sale.GrossCents)
	v.NonNegative("fees_cents", sale.FeesCents)
	if err := v.Error(); err != nil {
		respondValidationError(w, err)
		return
	}

	if err := h.service.Create(r.Context(), &sale); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, sale)
}

// Get returns a sale by ID.
func (h *SaleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid sale ID")
		return
	}

	sale, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sale == nil {
		respondError(w, http.StatusNotFound, "sale not found")
		return
	}

	respondJSON(w, http.StatusOK, sale)
}

// Update updates a sale.
func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid sale ID")
		return
	}

	sale, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if sale == nil {
		respondError(w, http.StatusNotFound, "sale not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(sale); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	sale.ID = id

	if err := h.service.Update(r.Context(), sale); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, sale)
}

// Delete deletes a sale.
func (h *SaleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid sale ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetWeeklyInsights returns this-week vs last-week sales comparison.
func (h *SaleHandler) GetWeeklyInsights(w http.ResponseWriter, r *http.Request) {
	insights, err := h.service.GetWeeklyInsights(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, insights)
}

// parsePeriodTime converts a period string to a start time.
func parsePeriodTime(period string) time.Time {
	now := time.Now()
	switch period {
	case "1d":
		return now.AddDate(0, 0, -1)
	case "3d":
		return now.AddDate(0, 0, -3)
	case "7d":
		return now.AddDate(0, 0, -7)
	case "60d":
		return now.AddDate(0, 0, -60)
	case "90d":
		return now.AddDate(0, 0, -90)
	case "12m":
		return now.AddDate(-1, 0, 0)
	default: // "30d"
		return now.AddDate(0, 0, -30)
	}
}

// StatsHandler handles statistics endpoints.
