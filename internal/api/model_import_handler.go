package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
)

type ModelImportHandler struct {
	service *service.ModelImportService
}

func NewModelImportHandler(service *service.ModelImportService) *ModelImportHandler {
	return &ModelImportHandler{service: service}
}

func (h *ModelImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req service.ModelImportPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, err := h.service.Preview(r.Context(), req.URL)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, preview)
}

func (h *ModelImportHandler) Import(w http.ResponseWriter, r *http.Request) {
	var req service.ModelImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.Import(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, result)
}
