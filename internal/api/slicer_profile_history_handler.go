package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

func decodeProfileEditor(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2*service.SlicerProfileJSONLimit))
	if err := decoder.Decode(value); err != nil {
		respondError(w, http.StatusBadRequest, "invalid or oversized profile request")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		respondError(w, http.StatusBadRequest, "request must contain one JSON object")
		return false
	}
	return true
}
func (h *SlicerHandler) ValidateProfile(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerUploadProfileRequest
	if !decodeProfileEditor(w, r, &req) {
		return
	}
	respondJSON(w, http.StatusOK, h.service.ValidateProfile(r.Context(), req))
}
func (h *SlicerHandler) ProfileHistory(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ProfileHistory(r.Context(), chi.URLParam(r, "category"), chi.URLParam(r, "name"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
func (h *SlicerHandler) SaveProfileRevision(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerSaveProfileRequest
	if !decodeProfileEditor(w, r, &req) {
		return
	}
	result, err := h.service.SaveProfileRevision(r.Context(), req)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrSlicerProfileConflict) {
			status = http.StatusConflict
		}
		respondError(w, status, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
