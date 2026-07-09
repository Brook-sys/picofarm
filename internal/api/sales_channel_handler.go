package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SalesChannelHandler exposes provider-neutral sales-channel endpoints.
type SalesChannelHandler struct {
	registry *saleschannel.Registry
	repo     *repository.SalesChannelRepository
	orders   *service.OrderService
}

// NewSalesChannelHandler creates a provider-neutral sales-channel handler.
func NewSalesChannelHandler(registry *saleschannel.Registry, repo *repository.SalesChannelRepository, orders ...*service.OrderService) *SalesChannelHandler {
	handler := &SalesChannelHandler{registry: registry, repo: repo}
	if len(orders) > 0 {
		handler.orders = orders[0]
	}
	return handler
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

type salesChannelSyncRunsResponse struct {
	Runs []saleschannel.SyncRun `json:"runs"`
}

type salesChannelExternalOrdersResponse struct {
	Orders []saleschannel.ExternalOrder `json:"orders"`
}

type salesChannelExternalProductsResponse struct {
	Products []saleschannel.ExternalProduct `json:"products"`
}

type salesChannelProcessOrderResponse struct {
	Order model.Order `json:"order"`
}

type salesChannelLinkProductRequest struct {
	ProjectID         uuid.UUID  `json:"project_id"`
	ExternalVariantID *uuid.UUID `json:"external_variant_id,omitempty"`
	SKU               string     `json:"sku,omitempty"`
	SyncInventory     bool       `json:"sync_inventory"`
}

type salesChannelProductLinkResponse struct {
	Link saleschannel.ProductLink `json:"link"`
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

// ListExternalOrders returns provider-neutral imported orders from canonical storage.
// GET /api/sales-channels/orders
func (h *SalesChannelHandler) ListExternalOrders(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	filter := saleschannel.OrderFilter{
		Channel: saleschannel.ChannelID(r.URL.Query().Get("channel")),
		Status:  r.URL.Query().Get("status"),
		Limit:   parsePositiveIntQuery(r, "limit"),
		Offset:  parsePositiveIntQuery(r, "offset"),
	}
	if processed := r.URL.Query().Get("processed"); processed != "" {
		value, err := strconv.ParseBool(processed)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid processed filter")
			return
		}
		filter.Processed = &value
	}
	orders, err := h.repo.ListExternalOrders(r.Context(), filter)
	if err != nil {
		slog.Error("failed to list external sales-channel orders", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range orders {
		orders[i].RawJSON = ""
	}
	respondJSON(w, http.StatusOK, salesChannelExternalOrdersResponse{Orders: orders})
}

// ListExternalProducts returns provider-neutral imported products/listings from canonical storage.
// GET /api/sales-channels/products
func (h *SalesChannelHandler) ListExternalProducts(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	filter := saleschannel.ProductFilter{
		Channel: saleschannel.ChannelID(r.URL.Query().Get("channel")),
		Status:  r.URL.Query().Get("status"),
		Limit:   parsePositiveIntQuery(r, "limit"),
		Offset:  parsePositiveIntQuery(r, "offset"),
	}
	if linked := r.URL.Query().Get("linked"); linked != "" {
		value, err := strconv.ParseBool(linked)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid linked filter")
			return
		}
		filter.Linked = &value
	}
	products, err := h.repo.ListExternalProducts(r.Context(), filter)
	if err != nil {
		slog.Error("failed to list external sales-channel products", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range products {
		products[i].RawJSON = ""
	}
	respondJSON(w, http.StatusOK, salesChannelExternalProductsResponse{Products: products})
}

// ListSyncRuns returns provider-neutral sync-run observability from canonical storage.
// GET /api/sales-channels/sync-runs
func (h *SalesChannelHandler) ListSyncRuns(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	filter := saleschannel.SyncRunFilter{
		Channel: saleschannel.ChannelID(r.URL.Query().Get("channel")),
		Kind:    saleschannel.SyncKind(r.URL.Query().Get("kind")),
		Limit:   parsePositiveIntQuery(r, "limit"),
		Offset:  parsePositiveIntQuery(r, "offset"),
	}
	if connectionID := r.URL.Query().Get("connection_id"); connectionID != "" {
		parsed, err := uuid.Parse(connectionID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid connection_id filter")
			return
		}
		filter.ConnectionID = parsed
	}
	if filter.Kind != "" && !validSyncKind(filter.Kind) {
		respondError(w, http.StatusBadRequest, "invalid sync kind")
		return
	}
	runs, err := h.repo.ListSyncRuns(r.Context(), filter)
	if err != nil {
		slog.Error("failed to list sales-channel sync runs", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range runs {
		runs[i].LastError = saleschannel.SanitizeErrorMessage(runs[i].LastError)
	}
	respondJSON(w, http.StatusOK, salesChannelSyncRunsResponse{Runs: runs})
}

// ProcessExternalOrder converts a canonical external order into a PicoFarm order.
// POST /api/sales-channels/orders/{id}/process
func (h *SalesChannelHandler) ProcessExternalOrder(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil || h.orders == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel processing unavailable")
		return
	}
	externalOrderID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid external order ID")
		return
	}
	externalOrder, err := h.repo.GetExternalOrderByID(r.Context(), externalOrderID)
	if err != nil {
		slog.Error("failed to load external sales-channel order", "external_order_id", externalOrderID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if externalOrder == nil {
		respondError(w, http.StatusNotFound, "external order not found")
		return
	}
	if externalOrder.OrderID != nil || externalOrder.IsProcessed {
		respondError(w, http.StatusConflict, "external order already processed")
		return
	}

	items := make([]model.OrderItem, 0, len(externalOrder.Items))
	for _, item := range externalOrder.Items {
		items = append(items, model.OrderItem{
			SKU:       item.SKU,
			Quantity:  item.Quantity,
			ProjectID: item.ProjectID,
			Notes:     item.Title,
		})
	}
	order, err := h.orders.CreateFromExternalOrder(
		r.Context(),
		orderSourceForSalesChannel(externalOrder.Channel),
		externalOrder.ExternalOrderID,
		externalOrder.CustomerName,
		externalOrder.CustomerEmail,
		items,
	)
	if err != nil {
		slog.Error("failed to process external sales-channel order", "external_order_id", externalOrderID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.repo.MarkExternalOrderProcessed(r.Context(), externalOrder.ID, order.ID); err != nil {
		slog.Error("failed to mark external sales-channel order processed", "external_order_id", externalOrderID, "order_id", order.ID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if loaded, err := h.orders.GetByID(r.Context(), order.ID); err == nil && loaded != nil {
		order = loaded
	}
	respondJSON(w, http.StatusOK, salesChannelProcessOrderResponse{Order: *order})
}

// LinkExternalProduct links a canonical external product/listing to a PicoFarm project.
// POST /api/sales-channels/products/{id}/link
func (h *SalesChannelHandler) LinkExternalProduct(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	product, req, ok := h.externalProductLinkRequest(w, r)
	if !ok {
		return
	}
	link := saleschannel.ProductLink{
		ConnectionID:      product.ConnectionID,
		Channel:           product.Channel,
		ExternalProductID: product.ID,
		ExternalVariantID: req.ExternalVariantID,
		ProjectID:         req.ProjectID,
		SKU:               req.SKU,
		SyncInventory:     req.SyncInventory,
	}
	if err := h.repo.UpsertProductLink(r.Context(), &link); err != nil {
		slog.Error("failed to link external sales-channel product", "external_product_id", product.ID, "project_id", req.ProjectID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, salesChannelProductLinkResponse{Link: link})
}

// UnlinkExternalProduct removes a canonical external product/listing link.
// DELETE /api/sales-channels/products/{id}/link
func (h *SalesChannelHandler) UnlinkExternalProduct(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	product, req, ok := h.externalProductLinkRequest(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteProductLink(r.Context(), product.ID, req.ExternalVariantID, req.ProjectID); err != nil {
		slog.Error("failed to unlink external sales-channel product", "external_product_id", product.ID, "project_id", req.ProjectID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SalesChannelHandler) externalProductLinkRequest(w http.ResponseWriter, r *http.Request) (*saleschannel.ExternalProduct, salesChannelLinkProductRequest, bool) {
	externalProductID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid external product ID")
		return nil, salesChannelLinkProductRequest{}, false
	}
	var req salesChannelLinkProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return nil, req, false
	}
	if req.ProjectID == uuid.Nil {
		respondError(w, http.StatusBadRequest, "project_id is required")
		return nil, req, false
	}
	product, err := h.repo.GetExternalProductByID(r.Context(), externalProductID)
	if err != nil {
		slog.Error("failed to load external sales-channel product", "external_product_id", externalProductID, "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return nil, req, false
	}
	if product == nil {
		respondError(w, http.StatusNotFound, "external product not found")
		return nil, req, false
	}
	if req.ExternalVariantID != nil && !productHasVariant(product, *req.ExternalVariantID) {
		respondError(w, http.StatusBadRequest, "external variant does not belong to product")
		return nil, req, false
	}
	return product, req, true
}

func productHasVariant(product *saleschannel.ExternalProduct, variantID uuid.UUID) bool {
	for _, variant := range product.Variants {
		if variant.ID == variantID {
			return true
		}
	}
	return false
}

func orderSourceForSalesChannel(channel saleschannel.ChannelID) model.OrderSource {
	switch channel {
	case saleschannel.ChannelEtsy:
		return model.OrderSourceEtsy
	case saleschannel.ChannelSquarespace:
		return model.OrderSourceSquarespace
	case saleschannel.ChannelShopify:
		return model.OrderSourceShopify
	default:
		return model.OrderSource(fmt.Sprintf("sales_channel:%s", channel))
	}
}

// ReceiveWebhook accepts a provider webhook payload, stores it idempotently and sanitizes responses.
func (h *SalesChannelHandler) ReceiveWebhook(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	channel := saleschannel.ChannelID(chi.URLParam(r, "channel"))
	if channel == "" {
		respondError(w, http.StatusBadRequest, "channel is required")
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read webhook body")
		return
	}
	defer r.Body.Close()

	sig := r.Header.Get("X-Request-Signature")
	event := salesChannelWebhookEventFromRequest(channel, sig, body)
	if err := h.repo.UpsertWebhookEvent(r.Context(), &event); err != nil {
		slog.Error("failed to store sales-channel webhook event", "channel", channel, "error", err)
		respondError(w, http.StatusInternalServerError, "failed to store webhook event")
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// ListWebhookEvents lists stored inbound webhook metadata (payload and signature omitted).
func (h *SalesChannelHandler) ListWebhookEvents(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		respondError(w, http.StatusServiceUnavailable, "sales-channel storage unavailable")
		return
	}
	channel := saleschannel.ChannelID(chi.URLParam(r, "channel"))
	if channel == "" {
		respondError(w, http.StatusBadRequest, "channel is required")
		return
	}
	filter := saleschannel.WebhookEventFilter{
		Channel: channel,
		Topic:   r.URL.Query().Get("topic"),
		Limit:   parsePositiveIntQuery(r, "limit"),
		Offset:  parsePositiveIntQuery(r, "offset"),
	}
	events, err := h.repo.ListWebhookEvents(r.Context(), filter)
	if err != nil {
		slog.Error("failed to list sales-channel webhook events", "error", err)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range events {
		events[i].Payload = ""
		events[i].Signature = ""
	}
	type response struct {
		Events []saleschannel.WebhookEvent `json:"events"`
	}
	respondJSON(w, http.StatusOK, response{Events: events})
}

func salesChannelWebhookEventFromRequest(channel saleschannel.ChannelID, sig string, body []byte) saleschannel.WebhookEvent {
	event := saleschannel.WebhookEvent{
		Channel:         channel,
		ExternalEventID: computeExternalEventID(channel, sig, body),
		Payload:         string(body),
		Signature:       sig,
	}
	var payload struct {
		ID       string `json:"_id"`
		Topic    string `json:"topic"`
		Resource string `json:"resource"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		event.ExternalEventID = firstNonEmpty(payload.ID, event.ExternalEventID)
		event.Topic = payload.Topic
		event.ResourcePath = payload.Resource
	}
	return event
}

func computeExternalEventID(channel saleschannel.ChannelID, sig string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(string(channel)))
	h.Write([]byte(sig))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
	run := h.newSyncRun(r.Context(), descriptor.ID, req.Kind, startedAt)
	result, err := provider.Sync(r.Context(), req.Kind)
	finishedAt := time.Now().UTC()
	result.Channel = descriptor.ID
	result.Kind = req.Kind
	result.StartedAt = startedAt
	result.FinishedAt = finishedAt
	if err != nil {
		sanitized := saleschannel.SanitizeErrorMessage(err.Error())
		slog.Error("failed to sync sales channel", "channel", descriptor.ID, "kind", req.Kind, "error", sanitized)
		h.finishSyncRun(r.Context(), run, result, saleschannel.SyncRunStatusFailed, finishedAt, sanitized)
		respondError(w, http.StatusInternalServerError, sanitized)
		return
	}
	h.finishSyncRun(r.Context(), run, result, saleschannel.SyncRunStatusSucceeded, finishedAt, "")

	respondJSON(w, http.StatusOK, salesChannelSyncResponse{Result: result})
}

func (h *SalesChannelHandler) newSyncRun(ctx context.Context, channel saleschannel.ChannelID, kind saleschannel.SyncKind, startedAt time.Time) *saleschannel.SyncRun {
	if h == nil || h.repo == nil {
		return nil
	}
	run := &saleschannel.SyncRun{
		Channel:   channel,
		Kind:      kind,
		Status:    saleschannel.SyncRunStatusRunning,
		StartedAt: startedAt,
	}
	connection, err := h.repo.ConnectionForChannel(ctx, channel)
	if err != nil {
		slog.Warn("failed to resolve sales-channel connection for sync run", "channel", channel, "error", err)
		return nil
	}
	if connection == nil {
		return nil
	}
	run.ConnectionID = connection.ID
	if err := h.repo.CreateSyncRun(ctx, run); err != nil {
		slog.Warn("failed to create sales-channel sync run", "channel", channel, "kind", kind, "error", err)
		return nil
	}
	return run
}

func (h *SalesChannelHandler) finishSyncRun(ctx context.Context, run *saleschannel.SyncRun, result saleschannel.SyncResult, status saleschannel.SyncRunStatus, finishedAt time.Time, lastError string) {
	if h == nil || h.repo == nil || run == nil {
		return
	}
	run.Status = status
	run.TotalFetched = result.TotalFetched
	run.Created = result.Created
	run.Updated = result.Updated
	run.Skipped = result.Skipped
	run.Errors = result.Errors
	if status == saleschannel.SyncRunStatusFailed && run.Errors == 0 {
		run.Errors = 1
	}
	run.LastError = saleschannel.SanitizeErrorMessage(lastError)
	run.FinishedAt = &finishedAt
	if err := h.repo.FinishSyncRun(ctx, run); err != nil {
		slog.Warn("failed to finish sales-channel sync run", "run_id", run.ID, "error", err)
	}
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

func parsePositiveIntQuery(r *http.Request, key string) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
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
