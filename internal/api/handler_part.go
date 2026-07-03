package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/google/uuid"
)

type PartHandler struct {
	service       *service.PartService
	designService *service.DesignService
}

// ListByProject returns all parts for a project.
func (h *PartHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid project ID")
		return
	}

	parts, err := h.service.ListByProject(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if parts == nil {
		parts = []model.Part{}
	}

	respondJSON(w, http.StatusOK, parts)
}

// Create creates a new part. Supports JSON body or multipart/form-data with an optional file.
func (h *PartHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid project ID")
		return
	}

	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		h.createWithFile(w, r, projectID)
		return
	}

	var req struct {
		model.Part
		GCodeFileID *uuid.UUID `json:"gcode_file_id"`
		STLFileID   *uuid.UUID `json:"stl_file_id"`
		Notes       string     `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	part := req.Part
	part.ProjectID = projectID

	if err := h.service.Create(r.Context(), &part); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.GCodeFileID != nil && h.designService != nil {
		design, err := h.designService.CreateFromGCodeLibrary(r.Context(), part.ID, *req.GCodeFileID, req.Notes)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, map[string]interface{}{"part": part, "design": design})
		return
	}
	if req.STLFileID != nil && h.designService != nil {
		design, err := h.designService.CreateFromSTLLibrary(r.Context(), part.ID, *req.STLFileID, req.Notes)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, map[string]interface{}{"part": part, "design": design})
		return
	}

	respondJSON(w, http.StatusCreated, part)
}

// createWithFile handles multipart part creation with an optional file attachment.
func (h *PartHandler) createWithFile(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	quantity := 1
	if q := r.FormValue("quantity"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil {
			quantity = parsed
		}
	}

	part := model.Part{
		ProjectID:   projectID,
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Quantity:    quantity,
	}

	if err := h.service.Create(r.Context(), &part); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if id := r.FormValue("gcode_file_id"); id != "" && h.designService != nil {
		gcodeID, err := uuid.Parse(id)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid gcode file ID")
			return
		}
		design, err := h.designService.CreateFromGCodeLibrary(r.Context(), part.ID, gcodeID, r.FormValue("notes"))
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, map[string]interface{}{"part": part, "design": design})
		return
	}

	// Check for optional file
	file, header, err := r.FormFile("file")
	if err != nil {
		// No file provided — return just the part
		respondJSON(w, http.StatusCreated, part)
		return
	}
	defer file.Close()

	if h.designService == nil {
		respondJSON(w, http.StatusCreated, part)
		return
	}

	notes := r.FormValue("notes")
	design, err := h.designService.Create(r.Context(), part.ID, header.Filename, file, notes)
	if err != nil {
		// Part was created but design failed — still return the part
		slog.Error("failed to create design for new part", "error", err, "part_id", part.ID)
		respondJSON(w, http.StatusCreated, map[string]interface{}{
			"part":         part,
			"design_error": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"part":   part,
		"design": design,
	})
}

// Get returns a part by ID.
func (h *PartHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid part ID")
		return
	}

	part, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if part == nil {
		respondError(w, http.StatusNotFound, "part not found")
		return
	}

	respondJSON(w, http.StatusOK, part)
}

// Update updates a part.
func (h *PartHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid part ID")
		return
	}

	part, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if part == nil {
		respondError(w, http.StatusNotFound, "part not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(part); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	part.ID = id

	if err := h.service.Update(r.Context(), part); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, part)
}

// Delete removes a part.
func (h *PartHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid part ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DesignHandler handles design endpoints.
