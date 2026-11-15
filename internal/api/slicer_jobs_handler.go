package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

type SlicerJobsHandler struct{ service *service.SlicerJobsService }

func NewSlicerJobsHandler(s *service.SlicerJobsService) *SlicerJobsHandler {
	return &SlicerJobsHandler{service: s}
}

func (h *SlicerJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.service.List())
}
func (h *SlicerJobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.Get(chi.URLParam(r, "id"))
	if err != nil {
		slicerJobError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, job)
}
func (h *SlicerJobsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var req service.SlicerSliceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid slicer job request")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		respondError(w, http.StatusBadRequest, "Expected one JSON object")
		return
	}
	job, err := h.service.Submit(req, r.Header.Get("Idempotency-Key"))
	if err != nil {
		slicerJobError(w, err)
		return
	}
	respondJSON(w, http.StatusAccepted, job)
}
func (h *SlicerJobsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	job, err := h.service.Cancel(chi.URLParam(r, "id"))
	if err != nil {
		slicerJobError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, job)
}
func (h *SlicerJobsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Delete(chi.URLParam(r, "id")); err != nil {
		slicerJobError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func slicerJobError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrSlicerJobInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrSlicerJobNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrSlicerJobConflict):
		status = http.StatusConflict
	case errors.Is(err, service.ErrSlicerJobsFull):
		status = http.StatusTooManyRequests
	case errors.Is(err, service.ErrSlicerJobsUnavailable):
		status = http.StatusServiceUnavailable
	}
	respondError(w, status, err.Error())
}
