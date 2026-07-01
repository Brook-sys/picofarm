package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

type QueueHandler struct {
	service *service.QueueService
}

func (h *QueueHandler) List(w http.ResponseWriter, r *http.Request) {
	queue, err := h.service.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, queue)
}

func (h *QueueHandler) Upload(w http.ResponseWriter, r *http.Request) {
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

	var opts service.QueueCreateOptions
	if dn := r.FormValue("display_name"); dn != "" {
		opts.DisplayName = dn
	}
	if notes := r.FormValue("notes"); notes != "" {
		opts.Notes = notes
	}
	if pid := r.FormValue("printer_id"); pid != "" {
		if u, err := uuid.Parse(pid); err == nil {
			opts.AssignedPrinterID = &u
		}
	}
	if sid := r.FormValue("spool_id"); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			opts.AssignedSpoolID = &u
		}
	}
	if mt := r.FormValue("material_type"); mt != "" {
		opts.MaterialType = mt
	}
	if mc := r.FormValue("material_color"); mc != "" {
		opts.MaterialColor = mc
	}

	item, err := h.service.CreateFromUpload(r.Context(), header.Filename, file, opts)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *QueueHandler) FromPrintJob(w http.ResponseWriter, r *http.Request) {
	jobID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid print job id")
		return
	}
	var opts service.QueueCreateOptions
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
		opts = service.QueueCreateOptions{}
	}
	item, err := h.service.CreateFromPrintJob(r.Context(), jobID, opts)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

func (h *QueueHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid queue id")
		return
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	opts, err := decodeQueueUpdateOptions(raw)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	item, err := h.service.Update(r.Context(), id, opts)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (h *QueueHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid queue id")
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func decodeQueueUpdateOptions(raw map[string]json.RawMessage) (service.QueueCreateOptions, error) {
	var opts service.QueueCreateOptions
	if v, ok := raw["display_name"]; ok {
		json.Unmarshal(v, &opts.DisplayName)
	}
	if v, ok := raw["notes"]; ok {
		json.Unmarshal(v, &opts.Notes)
	}
	if v, ok := raw["assigned_printer_id"]; ok {
		var s *string
		if err := json.Unmarshal(v, &s); err != nil {
			return opts, err
		}
		if s == nil || *s == "" {
			opts.ClearAssignedPrinterID = true
		} else if id, err := uuid.Parse(*s); err == nil {
			opts.AssignedPrinterID = &id
		} else {
			return opts, err
		}
	}
	if v, ok := raw["assigned_spool_id"]; ok {
		var s *string
		if err := json.Unmarshal(v, &s); err != nil {
			return opts, err
		}
		if s == nil || *s == "" {
			opts.ClearAssignedSpoolID = true
		} else if id, err := uuid.Parse(*s); err == nil {
			opts.AssignedSpoolID = &id
		} else {
			return opts, err
		}
	}
	if v, ok := raw["thumbnail_file_id"]; ok {
		var s *string
		if err := json.Unmarshal(v, &s); err != nil {
			return opts, err
		}
		if s == nil || *s == "" {
			opts.ClearThumbnailFileID = true
		} else if id, err := uuid.Parse(*s); err == nil {
			opts.ThumbnailFileID = &id
		} else {
			return opts, err
		}
	}
	return opts, nil
}

func (h *QueueHandler) UpdatePriority(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid queue id")
		return
	}
	var req struct {
		Priority int `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.service.UpdatePriority(r.Context(), id, req.Priority); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *QueueHandler) Preflight(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid queue id")
		return
	}
	res, err := h.service.PreflightCheck(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (h *QueueHandler) Start(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid queue id")
		return
	}
	if err := h.service.Start(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *QueueHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid queue id")
		return
	}
	var req struct {
		Status model.QueueItemStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := h.service.SetStatus(r.Context(), id, req.Status); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
