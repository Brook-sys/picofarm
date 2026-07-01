package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/service"
)

type STLLibraryHandler struct {
	service *service.STLLibraryService
}

func (h *STLLibraryHandler) Library(w http.ResponseWriter, r *http.Request) {
	res, err := h.service.Library(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (h *STLLibraryHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.service.List(r.Context(), service.STLLibraryListOptions{Query: q.Get("q"), Sort: q.Get("sort"), Page: parseIntDefault(q.Get("page"), 1), PageSize: parseIntDefault(q.Get("page_size"), 50)})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (h *STLLibraryHandler) Upload(w http.ResponseWriter, r *http.Request) {
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
	var thumbnail io.Reader
	if thumb, _, err := r.FormFile("thumbnail"); err == nil {
		thumbnail = thumb
		defer thumb.Close()
	}
	item, err := h.service.Upload(r.Context(), header.Filename, file, thumbnail)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *STLLibraryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	var opts service.STLLibraryUpdateOptions
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

func (h *STLLibraryHandler) UpdateThumbnail(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	thumbnail, _, err := r.FormFile("thumbnail")
	if err != nil {
		respondError(w, http.StatusBadRequest, "thumbnail required")
		return
	}
	defer thumbnail.Close()
	item, err := h.service.UpdateThumbnail(r.Context(), id, thumbnail)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (h *STLLibraryHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

func (h *STLLibraryHandler) AddTag(w http.ResponseWriter, r *http.Request) {
	fileID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}
	var payload struct {
		TagID string `json:"tag_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	tagID, err := uuid.Parse(payload.TagID)
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

func (h *STLLibraryHandler) RemoveTag(w http.ResponseWriter, r *http.Request) {
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
