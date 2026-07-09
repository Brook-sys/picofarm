package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/go-chi/chi/v5"
)

// SalesChannelHandler exposes provider-neutral sales-channel endpoints.
type SalesChannelHandler struct {
	registry *saleschannel.Registry
}

// NewSalesChannelHandler creates a provider-neutral sales-channel handler.
func NewSalesChannelHandler(registry *saleschannel.Registry) *SalesChannelHandler {
	return &SalesChannelHandler{registry: registry}
}

type salesChannelResponse struct {
	Descriptor saleschannel.ProviderDescriptor `json:"descriptor"`
	Status     saleschannel.ConnectionStatus   `json:"status"`
}

type salesChannelsListResponse struct {
	Channels []salesChannelResponse `json:"channels"`
}

// List returns all registered sales channels with descriptors and current status.
// GET /api/sales-channels
func (h *SalesChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel registry unavailable")
		return
	}

	channels := make([]salesChannelResponse, 0)
	for _, descriptor := range h.registry.Descriptors() {
		provider, err := h.registry.Get(descriptor.ID)
		if err != nil {
			slog.Error("failed to resolve sales-channel provider", "channel", descriptor.ID, "error", err)
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		status, err := provider.Status(r.Context())
		if err != nil {
			slog.Error("failed to get sales-channel status", "channel", descriptor.ID, "error", err)
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		channels = append(channels, salesChannelResponse{Descriptor: descriptor, Status: status})
	}

	respondJSON(w, http.StatusOK, salesChannelsListResponse{Channels: channels})
}

// Get returns one registered sales channel with descriptor and current status.
// GET /api/sales-channels/{channel}
func (h *SalesChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.registry == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel registry unavailable")
		return
	}

	channel := saleschannel.ChannelID(chi.URLParam(r, "channel"))
	provider, err := h.registry.Get(channel)
	if err != nil {
		if errors.Is(err, saleschannel.ErrProviderNotFound) {
			respondError(w, http.StatusNotFound, "sales channel not found")
			return
		}
		slog.Error("failed to resolve sales-channel provider", "channel", channel, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	descriptor := provider.Descriptor()
	status, err := provider.Status(r.Context())
	if err != nil {
		slog.Error("failed to get sales-channel status", "channel", descriptor.ID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, salesChannelResponse{Descriptor: descriptor, Status: status})
}
