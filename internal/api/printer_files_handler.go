package api

import (
	"encoding/json"
	"net/http"

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
