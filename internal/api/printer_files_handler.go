package api

import (
	"encoding/json"
	"io"
	"net/http"
	"path"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PrinterFileHandler struct {
	service *service.PrinterFileService
}

func NewPrinterFileHandler(service *service.PrinterFileService) *PrinterFileHandler {
	return &PrinterFileHandler{service: service}
}

func (h *PrinterFileHandler) List(w http.ResponseWriter, r *http.Request) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	files, err := h.service.List(r.Context(), printerID, r.URL.Query().Get("path"))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, files)
}

func (h *PrinterFileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	if err := h.service.Upload(r.Context(), printerID, r.FormValue("path"), file, header); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterFileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	printerID, filePath, ok := h.parseFileAction(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), printerID, filePath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterFileHandler) StartPrint(w http.ResponseWriter, r *http.Request) {
	printerID, filePath, ok := h.parseFileAction(w, r)
	if !ok {
		return
	}
	if err := h.service.StartPrint(r.Context(), printerID, filePath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterFileHandler) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	printerID, filePath, ok := h.parseFileAction(w, r)
	if !ok {
		return
	}
	if err := h.service.CreateDirectory(r.Context(), printerID, filePath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterFileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	printerID, oldPath, newPath, ok := h.parseTwoPathAction(w, r)
	if !ok {
		return
	}
	if err := h.service.Rename(r.Context(), printerID, oldPath, newPath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterFileHandler) Move(w http.ResponseWriter, r *http.Request) {
	printerID, oldPath, newPath, ok := h.parseTwoPathAction(w, r)
	if !ok {
		return
	}
	if err := h.service.Move(r.Context(), printerID, oldPath, newPath); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PrinterFileHandler) Metadata(w http.ResponseWriter, r *http.Request) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}
	metadata, err := h.service.Metadata(r.Context(), printerID, filePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, metadata)
}

func (h *PrinterFileHandler) Thumbnail(w http.ResponseWriter, r *http.Request) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	thumbPath := r.URL.Query().Get("path")
	if thumbPath == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}
	body, err := h.service.Thumbnail(r.Context(), printerID, thumbPath)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", "image/png")
	_, _ = io.Copy(w, body)
}

func (h *PrinterFileHandler) Download(w http.ResponseWriter, r *http.Request) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return
	}
	body, err := h.service.Download(r.Context(), printerID, filePath)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+path.Base(filePath)+`"`)
	if _, err := io.Copy(w, body); err != nil {
		return
	}
}

func (h *PrinterFileHandler) parseTwoPathAction(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, string, bool) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return uuid.Nil, "", "", false
	}
	var req struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" || req.NewPath == "" {
		respondError(w, http.StatusBadRequest, "path and new_path are required")
		return uuid.Nil, "", "", false
	}
	return printerID, req.Path, req.NewPath, true
}

func (h *PrinterFileHandler) parseFileAction(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	printerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return uuid.Nil, "", false
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		respondError(w, http.StatusBadRequest, "path is required")
		return uuid.Nil, "", false
	}
	return printerID, req.Path, true
}
