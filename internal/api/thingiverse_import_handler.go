package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
)

type ThingiverseImportHandler struct {
	service *service.ThingiverseImportService
}

func NewThingiverseImportHandler(service *service.ThingiverseImportService) *ThingiverseImportHandler {
	return &ThingiverseImportHandler{service: service}
}

func (h *ThingiverseImportHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resolved, err := h.service.Resolve(r.Context(), req.URL)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, resolved)
}

func (h *ThingiverseImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
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

func (h *ThingiverseImportHandler) Import(w http.ResponseWriter, r *http.Request) {
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
