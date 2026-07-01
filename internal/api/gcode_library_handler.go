package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/service"
)

type GCodeLibraryHandler struct {
	service *service.GCodeLibraryService
}

func parseIntDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (h *GCodeLibraryHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := parseIntDefault(q.Get("page"), 1)
	pageSize := parseIntDefault(q.Get("page_size"), 50)
	res, err := h.service.List(r.Context(), service.GCodeLibraryListOptions{
		Query: q.Get("q"), Material: q.Get("material"), Profile: q.Get("profile"), Nozzle: q.Get("nozzle"), Layer: q.Get("layer"),
		TimeBucket: q.Get("time_bucket"), Usage: q.Get("usage"), Sort: q.Get("sort"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (h *GCodeLibraryHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()
	var parentID *uuid.UUID
	if raw := r.FormValue("parent_stl_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid parent STL ID")
			return
		}
		parentID = &parsed
	}
	item, err := h.service.UploadWithParent(r.Context(), header.Filename, file, parentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *GCodeLibraryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	var opts service.GCodeLibraryUpdateOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.Update(r.Context(), id, opts)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (h *GCodeLibraryHandler) SetParentSTL(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	var req struct {
		ParentSTLID *string `json:"parent_stl_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var parentID *uuid.UUID
	if req.ParentSTLID != nil && *req.ParentSTLID != "" {
		parsed, err := uuid.Parse(*req.ParentSTLID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid parent STL ID")
			return
		}
		parentID = &parsed
	}
	if err := h.service.SetParentSTL(r.Context(), id, parentID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GCodeLibraryHandler) SetDefaultForSTL(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	if err := h.service.SetDefaultForSTL(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GCodeLibraryHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.service.ListTags(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, tags)
}

func (h *GCodeLibraryHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tag, err := h.service.CreateTag(r.Context(), req.Name, req.Color)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, tag)
}

func (h *GCodeLibraryHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid tag ID")
		return
	}
	if err := h.service.DeleteTag(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GCodeLibraryHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	tagID, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid tag ID")
		return
	}
	if err := h.service.AddTag(r.Context(), fileID, tagID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GCodeLibraryHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	tagID, err := uuid.Parse(chi.URLParam(r, "tagID"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid tag ID")
		return
	}
	if err := h.service.RemoveTag(r.Context(), fileID, tagID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GCodeLibraryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *GCodeLibraryHandler) AddToQueue(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	var options service.GCodeQueueOptions
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&options)
	}
	item, err := h.service.AddToQueue(r.Context(), id, options)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}
