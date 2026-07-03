package api

import (
	"net/http"
	"time"

	"github.com/Brook-sys/picofarm/internal/service"
)

type StatsHandler struct {
	service *service.StatsService
}

// GetFinancialSummary returns aggregated financial statistics.
// Accepts an optional "period" query param (30d, 60d, 90d, 12m) to filter by time range.
func (h *StatsHandler) GetFinancialSummary(w http.ResponseWriter, r *http.Request) {
	var since *time.Time
	if period := r.URL.Query().Get("period"); period != "" {
		t := parsePeriodTime(period)
		since = &t
	}

	summary, err := h.service.GetFinancialSummary(r.Context(), since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetTimeSeries returns time-series data for revenue, expenses, and profit.
func (h *StatsHandler) GetTimeSeries(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	data, err := h.service.GetTimeSeriesData(r.Context(), period)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, data)
}

// GetExpensesByCategory returns expense totals grouped by category.
func (h *StatsHandler) GetExpensesByCategory(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	data, err := h.service.GetExpensesByCategory(r.Context(), period)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if data == nil {
		data = []service.CategoryBreakdown{}
	}

	respondJSON(w, http.StatusOK, data)
}

// GetUsage returns aggregated printer and filament usage stats.
func (h *StatsHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	var since *time.Time
	if period := r.URL.Query().Get("period"); period != "" {
		t := parsePeriodTime(period)
		since = &t
	}
	data, err := h.service.GetUsageStats(r.Context(), since)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, data)
}

// GetSalesByChannel returns sales totals grouped by channel.
func (h *StatsHandler) GetSalesByChannel(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	data, err := h.service.GetSalesByChannel(r.Context(), period)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if data == nil {
		data = []service.ChannelBreakdown{}
	}

	respondJSON(w, http.StatusOK, data)
}

// GetSalesByProject returns sales aggregated by project.
func (h *StatsHandler) GetSalesByProject(w http.ResponseWriter, r *http.Request) {
	data, err := h.service.GetSalesByProject(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if data == nil {
		data = []service.ProjectSales{}
	}

	respondJSON(w, http.StatusOK, data)
}

// TemplateHandler handles template endpoints.
