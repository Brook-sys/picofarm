package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/google/uuid"
)

type PrintJobHandler struct {
	service *service.PrintJobService
}

// List returns all print jobs.
func (h *PrintJobHandler) List(w http.ResponseWriter, r *http.Request) {
	var printerID *uuid.UUID
	if pidStr := r.URL.Query().Get("printer_id"); pidStr != "" {
		if pid, err := uuid.Parse(pidStr); err == nil {
			printerID = &pid
		}
	}

	var status *model.PrintJobStatus
	if s := r.URL.Query().Get("status"); s != "" {
		ps := model.PrintJobStatus(s)
		status = &ps
	}

	jobs, err := h.service.List(r.Context(), printerID, status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if jobs == nil {
		jobs = []model.PrintJob{}
	}

	respondJSON(w, http.StatusOK, jobs)
}

// ListByDesign returns all print jobs for a design.
func (h *PrintJobHandler) ListByDesign(w http.ResponseWriter, r *http.Request) {
	designID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}

	jobs, err := h.service.ListByDesign(r.Context(), designID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if jobs == nil {
		jobs = []model.PrintJob{}
	}

	respondJSON(w, http.StatusOK, jobs)
}

// Create creates a new print job.
func (h *PrintJobHandler) Create(w http.ResponseWriter, r *http.Request) {
	var job model.PrintJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Create(r.Context(), &job); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, job)
}

// Get returns a print job by ID.
func (h *PrintJobHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid print job ID")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrintJobHandler) DeleteByProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid project ID")
		return
	}
	if err := h.service.DeleteByProject(r.Context(), projectID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrintJobHandler) DeleteByPrinter(w http.ResponseWriter, r *http.Request) {
	printerID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := h.service.DeleteByPrinter(r.Context(), printerID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrintJobHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if job == nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// Update updates a print job.
func (h *PrintJobHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if job == nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(job); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	job.ID = id

	if err := h.service.Update(r.Context(), job); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// PreflightCheck validates a job is ready to start.
func (h *PrintJobHandler) PreflightCheck(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	result, err := h.service.PreflightCheck(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// Start sends the job to the printer.
func (h *PrintJobHandler) Start(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.service.Start(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Pause pauses the print job.
func (h *PrintJobHandler) Pause(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.service.Pause(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Resume resumes the print job.
func (h *PrintJobHandler) Resume(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.service.Resume(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Cancel cancels the print job.
func (h *PrintJobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	if err := h.service.Cancel(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RecordOutcome records the outcome of a completed print job.
func (h *PrintJobHandler) RecordOutcome(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	var outcome model.PrintOutcome
	if err := json.NewDecoder(r.Body).Decode(&outcome); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.RecordOutcome(r.Context(), id, &outcome); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get updated job to return
	job, _ := h.service.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, job)
}

// GetWithEvents returns a print job with its full event timeline.
func (h *PrintJobHandler) GetWithEvents(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.service.GetByIDWithEvents(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if job == nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// GetEvents returns all events for a print job.
func (h *PrintJobHandler) GetEvents(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	events, err := h.service.GetEvents(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if events == nil {
		events = []model.JobEvent{}
	}

	respondJSON(w, http.StatusOK, events)
}

// GetRetryChain returns all jobs in a retry chain.
func (h *PrintJobHandler) GetRetryChain(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	chain, err := h.service.GetRetryChain(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if chain == nil {
		chain = []model.PrintJob{}
	}

	respondJSON(w, http.StatusOK, chain)
}

// RetryRequest represents the request body for retrying a job.
type RetryRequest struct {
	PrinterID       string `json:"printer_id,omitempty"`
	MaterialSpoolID string `json:"material_spool_id,omitempty"`
	FailureCategory string `json:"failure_category,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

// Retry creates a new job from a failed job.
func (h *PrintJobHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	var req RetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body is optional
		req = RetryRequest{}
	}

	retryReq := &service.RetryRequest{
		Notes: req.Notes,
	}

	if req.PrinterID != "" {
		printerID, err := uuid.Parse(req.PrinterID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid printer ID")
			return
		}
		retryReq.PrinterID = &printerID
	}

	if req.MaterialSpoolID != "" {
		spoolID, err := uuid.Parse(req.MaterialSpoolID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid material spool ID")
			return
		}
		retryReq.MaterialSpoolID = &spoolID
	}

	if req.FailureCategory != "" {
		category := model.FailureCategory(req.FailureCategory)
		retryReq.FailureCategory = &category
	}

	newJob, err := h.service.Retry(r.Context(), id, retryReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, newJob)
}

// RecordFailureRequest represents the request body for recording a failure.
type RecordFailureRequest struct {
	FailureCategory string `json:"failure_category"`
	ErrorCode       string `json:"error_code"`
	ErrorMessage    string `json:"error_message"`
}

// RecordFailure records a failure for a job.
func (h *PrintJobHandler) RecordFailure(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	var req RecordFailureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	category := model.FailureCategory(req.FailureCategory)
	if category == "" {
		category = model.FailureUnknown
	}

	if err := h.service.RecordFailure(r.Context(), id, category, req.ErrorCode, req.ErrorMessage); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return updated job
	job, _ := h.service.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, job)
}

// MarkAsScrap marks a failed job as scrap (no retry intended).
func (h *PrintJobHandler) MarkAsScrap(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	var req service.ScrapRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	if err := h.service.MarkAsScrap(r.Context(), id, &req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return updated job
	job, _ := h.service.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, job)
}

// ListByRecipe returns all print jobs for a recipe.
func (h *PrintJobHandler) ListByRecipe(w http.ResponseWriter, r *http.Request) {
	recipeID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recipe ID")
		return
	}

	jobs, err := h.service.ListByRecipe(r.Context(), recipeID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if jobs == nil {
		jobs = []model.PrintJob{}
	}

	respondJSON(w, http.StatusOK, jobs)
}

// UpdatePriority updates a job's priority in the queue.
func (h *PrintJobHandler) UpdatePriority(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	var req struct {
		Priority int `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.UpdatePriority(r.Context(), id, req.Priority); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// FileHandler handles file endpoints.
