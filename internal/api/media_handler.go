package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

type CameraHandler struct {
	service *service.CameraService
}

func (h *CameraHandler) List(w http.ResponseWriter, r *http.Request) {
	var printerID *uuid.UUID
	if raw := r.URL.Query().Get("printer_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid printer_id")
			return
		}
		printerID = &id
	}
	cameras, err := h.service.List(r.Context(), printerID, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cameras == nil {
		cameras = []model.Camera{}
	}
	respondJSON(w, http.StatusOK, cameras)
}

func (h *CameraHandler) Create(w http.ResponseWriter, r *http.Request) {
	var camera model.Camera
	if err := json.NewDecoder(r.Body).Decode(&camera); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.Create(r.Context(), &camera); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, camera)
}

func (h *CameraHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid camera ID")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type TimelapseHandler struct {
	service *service.TimelapseService
}

func (h *TimelapseHandler) List(w http.ResponseWriter, r *http.Request) {
	var printerID *uuid.UUID
	if raw := r.URL.Query().Get("printer_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid printer_id")
			return
		}
		printerID = &id
	}
	items, err := h.service.List(r.Context(), printerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.Timelapse{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *TimelapseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var item model.Timelapse
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.Create(r.Context(), &item); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

type PrintArchiveHandler struct {
	service *service.PrintArchiveService
}

func (h *PrintArchiveHandler) List(w http.ResponseWriter, r *http.Request) {
	var printerID *uuid.UUID
	if raw := r.URL.Query().Get("printer_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid printer_id")
			return
		}
		printerID = &id
	}
	items, err := h.service.List(r.Context(), printerID, r.URL.Query().Get("status"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.PrintArchive{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *PrintArchiveHandler) Create(w http.ResponseWriter, r *http.Request) {
	var item model.PrintArchive
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.Create(r.Context(), &item); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *PrintArchiveHandler) Compare(w http.ResponseWriter, r *http.Request) {
	a, err := uuid.Parse(r.URL.Query().Get("a"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid archive a")
		return
	}
	b, err := uuid.Parse(r.URL.Query().Get("b"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid archive b")
		return
	}
	result, err := h.service.Compare(r.Context(), a, b)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *PrintArchiveHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.List(r.Context(), nil, r.URL.Query().Get("status"))
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=print-log.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "status", "duration_seconds", "filament_used_grams", "cost_cents", "created_at"})
	for _, item := range items {
		_ = writer.Write([]string{item.ID.String(), item.Status, fmtInt(item.DurationSeconds), fmtFloat(item.FilamentUsedGrams), fmtInt(item.CostCents), item.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
	}
	writer.Flush()
}

func fmtInt(v int) string       { return strconv.Itoa(v) }
func fmtFloat(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
