package api

import (
	"io"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
)

type FileHandler struct {
	service *service.FileService
}

// Get returns a file by ID.
func (h *FileHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	reader, file, err := h.service.GetReader(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+file.OriginalName)
	w.Header().Set("Content-Type", file.ContentType)
	io.Copy(w, reader) //nolint:errcheck // best-effort streaming to HTTP client
}

// Upload saves a small image file and returns its file record.
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	if contentType != "image/png" && contentType != "image/jpeg" {
		respondError(w, http.StatusBadRequest, "only PNG and JPG images are supported")
		return
	}
	created, err := h.service.Upload(r.Context(), header.Filename, file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

// ExpenseHandler handles expense endpoints.
