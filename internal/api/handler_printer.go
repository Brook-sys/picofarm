package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/validation"
	"github.com/go-chi/chi/v5"
)

type PrinterHandler struct {
	service *service.PrinterService
}

// List returns all printers.
func (h *PrinterHandler) List(w http.ResponseWriter, r *http.Request) {
	printers, err := h.service.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if printers == nil {
		printers = []model.Printer{}
	}

	respondJSON(w, http.StatusOK, printers)
}

// Create creates a new printer.
func (h *PrinterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var printer model.Printer
	if err := json.NewDecoder(r.Body).Decode(&printer); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	v := validation.New()
	v.Required("name", printer.Name)
	v.MaxLength("name", printer.Name, 255)
	v.MaxLength("model", printer.Model, 255)
	v.MaxLength("manufacturer", printer.Manufacturer, 255)
	v.NoControlChars("name", printer.Name)
	v.NonNegative("cost_per_hour_cents", printer.CostPerHourCents)
	if err := v.Error(); err != nil {
		respondValidationError(w, err)
		return
	}

	if err := h.service.Create(r.Context(), &printer); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, printer)
}

// Get returns a printer by ID.
func (h *PrinterHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	printer, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if printer == nil {
		respondError(w, http.StatusNotFound, "printer not found")
		return
	}

	respondJSON(w, http.StatusOK, printer)
}

// Update updates a printer.
func (h *PrinterHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	printer, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if printer == nil {
		respondError(w, http.StatusNotFound, "printer not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(printer); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	printer.ID = id

	if err := h.service.Update(r.Context(), printer); err != nil {
		if _, ok := err.(*validation.ValidationError); ok {
			respondValidationError(w, err)
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, printer)
}

// Delete removes a printer.
func (h *PrinterHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EmergencyStop cancels all active prints on every connected printer.
func (h *PrinterHandler) EmergencyStop(w http.ResponseWriter, r *http.Request) {
	errs := h.service.EmergencyStop(r.Context())
	if len(errs) > 0 {
		respondJSON(w, http.StatusMultiStatus, map[string]interface{}{
			"errors": errs,
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetPrintSpeed sets the print speed profile or feed rate on a printer.
func (h *PrinterHandler) SetPrintSpeed(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Level   int `json:"level"`
		Percent int `json:"percent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Percent > 0 {
		if err := h.service.SetFeedRate(r.Context(), id, req.Percent); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.service.SetPrintSpeed(r.Context(), id, req.Level); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterHandler) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	caps, err := h.service.GetCapabilities(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, caps)
}

// SetFanSpeed sets a fan speed (0-100%) on a printer.
func (h *PrinterHandler) SetFanSpeed(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Fan   string `json:"fan"`
		Speed int    `json:"speed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetFanSpeed(r.Context(), id, req.Fan, req.Speed); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetLEDMode controls the chamber light.
func (h *PrinterHandler) SetLEDMode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetLEDMode(r.Context(), id, req.Mode); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SkipObject excludes an object from the current print.
func (h *PrinterHandler) SkipObject(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		ObjectID string `json:"object_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SkipObject(r.Context(), id, req.ObjectID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Jog moves an axis by a relative distance.
func (h *PrinterHandler) Jog(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Axis     string  `json:"axis"`
		Distance float64 `json:"distance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.Jog(r.Context(), id, req.Axis, req.Distance); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetTemperature sets a heater target temperature.
func (h *PrinterHandler) SetTemperature(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Heater     string  `json:"heater"`
		TargetTemp float64 `json:"target_temp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetTemperature(r.Context(), id, req.Heater, req.TargetTemp); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AMSLoad loads filament from a specific AMS slot.
func (h *PrinterHandler) AMSLoad(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		AMSID  string `json:"ams_id"`
		SlotID string `json:"slot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.AMSLoad(r.Context(), id, req.AMSID, req.SlotID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AMSUnload unloads the current AMS filament.
func (h *PrinterHandler) AMSUnload(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := h.service.AMSUnload(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AMSRefresh triggers RFID re-read.
func (h *PrinterHandler) AMSRefresh(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := h.service.AMSRefresh(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetAMSFilamentBackup toggles AMS backup mode.
func (h *PrinterHandler) SetAMSFilamentBackup(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetAMSFilamentBackup(r.Context(), id, req.Enabled); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PlateCleared confirms the build plate is clear.
func (h *PrinterHandler) PlateCleared(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := h.service.PlateCleared(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetMaintenanceMode enables or disables maintenance mode for a printer.
func (h *PrinterHandler) GetDefault(w http.ResponseWriter, r *http.Request) {
	p, err := h.service.GetDefault(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if p == nil {
		respondError(w, http.StatusNotFound, "no default printer set")
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *PrinterHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := h.service.SetDefault(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterHandler) Reconnect(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := h.service.ReconnectAsync(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *PrinterHandler) RunMacro(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	var req struct {
		Macro string `json:"macro"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.RunMacro(r.Context(), id, req.Macro); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterHandler) ListMacros(w http.ResponseWriter, r *http.Request) {
	macros, err := h.service.ListMacros(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, macros)
}

func (h *PrinterHandler) CreateMacro(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title   string `json:"title"`
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	macro, err := h.service.CreateMacro(r.Context(), req.Title, req.Command)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, macro)
}

func (h *PrinterHandler) UpdateMacro(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "macroID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid macro ID")
		return
	}
	var req model.PrinterMacro
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id
	macro, err := h.service.UpdateMacro(r.Context(), &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, macro)
}

func (h *PrinterHandler) DeleteMacro(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "macroID"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid macro ID")
		return
	}
	if err := h.service.DeleteMacro(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterHandler) SetMaintenanceMode(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	var req struct {
		MaintenanceMode bool `json:"maintenance_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	printer, err := h.service.SetMaintenanceMode(r.Context(), id, req.MaintenanceMode)
	if err != nil {
		if err.Error() == "printer not found" {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, printer)
}

// GetState returns the real-time state of a printer.
func (h *PrinterHandler) GetState(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	state, err := h.service.GetState(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, state)
}

// GetAllStates returns the real-time state of all printers.
func (h *PrinterHandler) GetAllStates(w http.ResponseWriter, r *http.Request) {
	states := h.service.GetAllStates(r.Context())
	respondJSON(w, http.StatusOK, states)
}

// Discover scans the network for printers.
func (h *PrinterHandler) Discover(w http.ResponseWriter, r *http.Request) {
	slog.Info("starting printer discovery")

	ctx := context.Background()
	printers, err := h.service.DiscoverPrinters(ctx)
	if err != nil {
		slog.Error("discovery failed", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	slog.Info("discovery complete", "found", len(printers))

	if printers == nil {
		printers = []printer.DiscoveredPrinter{}
	}

	respondJSON(w, http.StatusOK, printers)
}

// ListJobs returns all print jobs for a printer.
func (h *PrinterHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	jobs, err := h.service.ListJobs(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if jobs == nil {
		jobs = []model.PrintJob{}
	}

	respondJSON(w, http.StatusOK, jobs)
}

// GetJobStats returns job statistics for a printer.
func (h *PrinterHandler) GetJobStats(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	stats, err := h.service.GetJobStats(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// GetPrinterAnalytics returns comprehensive analytics for a printer.
func (h *PrinterHandler) GetPrinterAnalytics(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	analytics, err := h.service.GetPrinterAnalytics(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, analytics)
}

// MaterialHandler handles material endpoints.
