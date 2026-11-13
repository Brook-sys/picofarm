package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

// slicerBundleBodyLimit caps a profile bundle request. Profiles are a few
// kilobytes each, so this is generous while still refusing an absurd body
// before it is buffered in memory.
const slicerBundleBodyLimit = 8 << 20

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
	// The handler answers with the effective configuration, not the request,
	// because the environment can override the value that was just stored.
	effective, err := h.service.SetConfig(r.Context(), cfg)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, effective)
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

func (h *SlicerHandler) SetDefaultProfile(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerDefaultProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.SetDefaultProfile(r.Context(), req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"ok": true})
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

// ExportProfiles answers a JSON bundle of every selected profile. An empty body
// is allowed and means "all categories", so a plain POST from a script or curl
// gets a usable backup.
func (h *SlicerHandler) ExportProfiles(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerExportRequest
	if body, err := io.ReadAll(io.LimitReader(r.Body, slicerBundleBodyLimit)); err == nil && len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	bundle, err := h.service.ExportProfiles(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bundle)
}

// ImportProfiles restores a bundle. A malformed bundle is rejected before
// anything is uploaded; per-profile upload failures come back in the result with
// 200, because the restore did run and the caller needs the details.
func (h *SlicerHandler) ImportProfiles(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, slicerBundleBodyLimit))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		respondError(w, http.StatusBadRequest, "request body is required")
		return
	}
	var req service.SlicerImportBundleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.ImportProfiles(r.Context(), req)
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

func (h *SlicerHandler) PreviewSTL(w http.ResponseWriter, r *http.Request) {
	var req service.SlicerPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.service.PreviewSTL(r.Context(), req)
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
