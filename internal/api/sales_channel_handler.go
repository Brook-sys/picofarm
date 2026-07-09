package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

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

type salesChannelSyncRequest struct {
	Kind saleschannel.SyncKind `json:"kind"`
}

type salesChannelSyncResponse struct {
	Result saleschannel.SyncResult `json:"result"`
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
	provider, ok := h.providerFromRequest(w, r)
	if !ok {
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

// Sync runs a provider-neutral sync for one registered sales channel.
// POST /api/sales-channels/{channel}/sync
func (h *SalesChannelHandler) Sync(w http.ResponseWriter, r *http.Request) {
	provider, ok := h.providerFromRequest(w, r)
	if !ok {
		return
	}

	var req salesChannelSyncRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
	}
	if req.Kind == "" {
		req.Kind = saleschannel.SyncAll
	}
	if !validSyncKind(req.Kind) {
		respondError(w, http.StatusBadRequest, "invalid sync kind")
		return
	}

	descriptor := provider.Descriptor()
	if !descriptorSupportsSyncKind(descriptor, req.Kind) {
		respondError(w, http.StatusBadRequest, "sales channel does not support requested sync kind")
		return
	}

	startedAt := time.Now().UTC()
	result, err := provider.Sync(r.Context(), req.Kind)
	finishedAt := time.Now().UTC()
	result.Channel = descriptor.ID
	result.Kind = req.Kind
	result.StartedAt = startedAt
	result.FinishedAt = finishedAt
	if err != nil {
		slog.Error("failed to sync sales channel", "channel", descriptor.ID, "kind", req.Kind, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, salesChannelSyncResponse{Result: result})
}

func (h *SalesChannelHandler) providerFromRequest(w http.ResponseWriter, r *http.Request) (saleschannel.Provider, bool) {
	if h == nil || h.registry == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel registry unavailable")
		return nil, false
	}

	channel := saleschannel.ChannelID(chi.URLParam(r, "channel"))
	provider, err := h.registry.Get(channel)
	if err != nil {
		if errors.Is(err, saleschannel.ErrProviderNotFound) {
			respondError(w, http.StatusNotFound, "sales channel not found")
			return nil, false
		}
		slog.Error("failed to resolve sales-channel provider", "channel", channel, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}

	return provider, true
}

func validSyncKind(kind saleschannel.SyncKind) bool {
	switch kind {
	case saleschannel.SyncOrders, saleschannel.SyncProducts, saleschannel.SyncAll:
		return true
	default:
		return false
	}
}

func descriptorSupportsSyncKind(descriptor saleschannel.ProviderDescriptor, kind saleschannel.SyncKind) bool {
	switch kind {
	case saleschannel.SyncOrders:
		return descriptor.Supports(saleschannel.CapabilityOrdersRead)
	case saleschannel.SyncProducts:
		return descriptor.Supports(saleschannel.CapabilityProductsRead)
	case saleschannel.SyncAll:
		return descriptor.Supports(saleschannel.CapabilityOrdersRead) || descriptor.Supports(saleschannel.CapabilityProductsRead)
	default:
		return false
	}
}
