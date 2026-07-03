package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/google/uuid"
)

type DesignHandler struct {
	service *service.DesignService
}

// ListByPart returns all designs for a part.
func (h *DesignHandler) ListByPart(w http.ResponseWriter, r *http.Request) {
	partID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid part ID")
		return
	}

	designs, err := h.service.ListByPart(r.Context(), partID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if designs == nil {
		designs = []model.Design{}
	}

	respondJSON(w, http.StatusOK, designs)
}

// Create uploads a new design version.
func (h *DesignHandler) Create(w http.ResponseWriter, r *http.Request) {
	partID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid part ID")
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/form-data") {
		// JSON path: link existing G-code library file
		var req struct {
			GCodeFileID uuid.UUID `json:"gcode_file_id"`
			STLFileID   uuid.UUID `json:"stl_file_id"`
			Notes       string    `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.GCodeFileID == uuid.Nil && req.STLFileID == uuid.Nil {
			respondError(w, http.StatusBadRequest, "gcode_file_id or stl_file_id is required")
			return
		}
		var design *model.Design
		var err error
		if req.STLFileID != uuid.Nil {
			design, err = h.service.CreateFromSTLLibrary(r.Context(), partID, req.STLFileID, req.Notes)
		} else {
			design, err = h.service.CreateFromGCodeLibrary(r.Context(), partID, req.GCodeFileID, req.Notes)
		}
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, design)
		return
	}

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	notes := r.FormValue("notes")

	design, err := h.service.Create(r.Context(), partID, header.Filename, file, notes)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, design)
}

// Get returns a design by ID.
func (h *DesignHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}

	design, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if design == nil {
		respondError(w, http.StatusNotFound, "design not found")
		return
	}

	respondJSON(w, http.StatusOK, design)
}

// Download returns the design file.
func (h *DesignHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DesignHandler) Download(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}

	reader, design, err := h.service.GetFile(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Disposition", "attachment; filename="+design.FileName)
	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, reader) //nolint:errcheck // best-effort streaming to HTTP client
}

// OpenExternal opens a design file in an external application.
func (h *DesignHandler) OpenExternal(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}

	var req struct {
		App string `json:"app"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		json.NewDecoder(r.Body).Decode(&req)
	}

	if err := h.service.OpenInExternalApp(r.Context(), id, req.App); err != nil {
		if err.Error() == "design not found" || err.Error() == "file not found" {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PrinterHandler handles printer endpoints.
