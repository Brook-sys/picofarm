package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

type NotificationHandler struct {
	service *service.NotificationService
}

func NewNotificationHandler(service *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.service.ListChannels(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, channels)
}

func (h *NotificationHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var channel model.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.CreateChannel(r.Context(), &channel); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := h.service.GetChannel(r.Context(), channel.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

func (h *NotificationHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid channel ID")
		return
	}
	var channel model.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	channel.ID = id
	if err := h.service.UpdateChannel(r.Context(), &channel); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := h.service.GetChannel(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (h *NotificationHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid channel ID")
		return
	}
	if err := h.service.DeleteChannel(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) SendTest(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid channel ID")
		return
	}
	if err := h.service.SendTest(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	var channelID *uuid.UUID
	if raw := r.URL.Query().Get("channel_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid channel ID")
			return
		}
		channelID = &parsed
	}
	templates, err := h.service.ListTemplates(r.Context(), channelID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, templates)
}

func (h *NotificationHandler) UpsertTemplate(w http.ResponseWriter, r *http.Request) {
	var template model.NotificationTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.service.UpsertTemplate(r.Context(), &template); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, template)
}

func (h *NotificationHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}
	if err := h.service.DeleteTemplate(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var template model.NotificationTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, err := h.service.PreviewTemplate(r.Context(), template)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, preview)
}

func (h *NotificationHandler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	var channelID *uuid.UUID
	if raw := r.URL.Query().Get("channel_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid channel ID")
			return
		}
		channelID = &parsed
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	deliveries, err := h.service.ListDeliveries(r.Context(), channelID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, deliveries)
}
