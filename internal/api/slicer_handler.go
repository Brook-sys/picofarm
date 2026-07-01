package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/Brook-sys/picofarm/internal/service"
)

type SlicerHandler struct {
	service *service.SlicerService
}

func NewSlicerHandler(service *service.SlicerService) *SlicerHandler {
	return &SlicerHandler{service: service}
}

func (h *SlicerHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.service.GetConfig(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

func (h *SlicerHandler) SetConfig(w http.ResponseWriter, r *http.Request) {
	var cfg service.SlicerConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetConfig(r.Context(), cfg); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

func (h *SlicerHandler) Health(w http.ResponseWriter, r *http.Request) {
	health, err := h.service.Health(r.Context())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, health)
}

func (h *SlicerHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.service.Status(r.Context())
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (h *SlicerHandler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.service.ListProfiles(r.Context(), chi.URLParam(r, "category"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, profiles)
}

func (h *SlicerHandler) GetProfileJSON(w http.ResponseWriter, r *http.Request) {
	profile, err := h.service.GetProfileJSON(r.Context(), chi.URLParam(r, "category"), chi.URLParam(r, "name"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(profile)
}

func (h *SlicerHandler) ImportProfile(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.ImportProfile(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *SlicerHandler) UploadProfileJSON(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerUploadProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.UploadProfileJSON(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *SlicerHandler) UpdateProfileFromSource(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.UpdateProfileFromSource(r.Context(), chi.URLParam(r, "category"), chi.URLParam(r, "name"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *SlicerHandler) ResolveProfiles(w http.ResponseWriter, r *http.Request) {
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.ResolveProfiles(r.Context(), payload)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *SlicerHandler) SliceSTL(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerSliceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.SliceSTL(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
