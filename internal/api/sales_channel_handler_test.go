package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestSalesChannelHandler_ListReturnsDescriptorsAndStatus(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := env.repos.Etsy.SaveIntegration(ctx, &model.EtsyIntegration{
		ShopID:         12345,
		ShopName:       "Dragon Forge",
		UserID:         67890,
		AccessToken:    "",
		RefreshToken:   "",
		TokenExpiresAt: now.Add(time.Hour),
		Scopes:         []string{"transactions_r", "listings_r"},
		IsActive:       true,
		LastSyncAt:     &now,
	}); err != nil {
		t.Fatalf("save Etsy integration: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sales-channels", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got salesChannelsListResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	ids := make([]saleschannel.ChannelID, 0, len(got.Channels))
	for _, channel := range got.Channels {
		ids = append(ids, channel.Descriptor.ID)
	}
	wantIDs := []saleschannel.ChannelID{saleschannel.ChannelEtsy, saleschannel.ChannelSquarespace, saleschannel.ChannelShopify, saleschannel.ChannelMercadoLivre, saleschannel.ChannelShopee, saleschannel.ChannelOLX}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("expected channel order %v, got %v", wantIDs, ids)
	}

	etsy := got.Channels[0]
	if etsy.Descriptor.DisplayName != "Etsy" {
		t.Fatalf("expected Etsy display name, got %q", etsy.Descriptor.DisplayName)
	}
	if !etsy.Status.Connected {
		t.Fatalf("expected Etsy status to be connected")
	}
	if etsy.Status.AccountID != "12345" || etsy.Status.DisplayName != "Dragon Forge" {
		t.Fatalf("unexpected Etsy status: %+v", etsy.Status)
	}

	shopify := got.Channels[2]
	if shopify.Status.Connected {
		t.Fatalf("expected Shopify to start disconnected")
	}
	if shopify.Descriptor.Supports(saleschannel.CapabilityProductsRead) {
		t.Fatalf("Shopify should not advertise product sync capability yet")
	}

	shopee := got.Channels[4]
	if shopee.Status.Connected {
		t.Fatalf("expected Shopee to start disconnected")
	}
	if !shopee.Descriptor.Supports(saleschannel.CapabilityOrdersRead) || !shopee.Descriptor.Supports(saleschannel.CapabilityProductsRead) {
		t.Fatalf("expected Shopee descriptor to advertise read-only MVP capabilities: %+v", shopee.Descriptor.Capabilities)
	}
	if shopee.Descriptor.Supports(saleschannel.CapabilityInventoryWrite) || shopee.Descriptor.Supports(saleschannel.CapabilityWebhooks) {
		t.Fatalf("Shopee should not advertise post-MVP gated capabilities yet: %+v", shopee.Descriptor.Capabilities)
	}
}

func TestSalesChannelHandler_GetReturnsSingleChannel(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sales-channels/squarespace", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got salesChannelResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Descriptor.ID != saleschannel.ChannelSquarespace {
		t.Fatalf("expected Squarespace descriptor, got %q", got.Descriptor.ID)
	}
	if got.Status.Channel != saleschannel.ChannelSquarespace {
		t.Fatalf("expected Squarespace status, got %q", got.Status.Channel)
	}
}

func TestSalesChannelHandler_GetUnknownChannelReturns404(t *testing.T) {
	env := newTestEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sales-channels/not-a-channel", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSalesChannelHandler_SyncDispatchesProviderNeutralSync(t *testing.T) {
	provider := &fakeSalesChannelProvider{
		descriptor: saleschannel.ProviderDescriptor{
			ID:           saleschannel.ChannelEtsy,
			DisplayName:  "Etsy",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOrdersRead},
		},
	}
	router := newSalesChannelTestRouter(t, provider)

	body := bytes.NewBufferString(`{"kind":"orders"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sales-channels/etsy/sync", body)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var got salesChannelSyncResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Result.Channel != saleschannel.ChannelEtsy {
		t.Fatalf("expected Etsy sync result, got %q", got.Result.Channel)
	}
	if got.Result.Kind != saleschannel.SyncOrders {
		t.Fatalf("expected orders sync kind, got %q", got.Result.Kind)
	}
	if got.Result.StartedAt.IsZero() || got.Result.FinishedAt.IsZero() {
		t.Fatalf("expected sync timestamps to be populated: %+v", got.Result)
	}
}

func TestSalesChannelHandler_SyncRejectsUnsupportedCapability(t *testing.T) {
	provider := &fakeSalesChannelProvider{
		descriptor: saleschannel.ProviderDescriptor{
			ID:           saleschannel.ChannelShopify,
			DisplayName:  "Shopify",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOrdersRead},
		},
	}
	router := newSalesChannelTestRouter(t, provider)

	body := bytes.NewBufferString(`{"kind":"products"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sales-channels/shopify/sync", body)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSalesChannelHandler_ListSyncRunsRedactsStoredErrors(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	connection := seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelEtsy)
	finishedAt := time.Date(2026, 7, 9, 12, 5, 0, 0, time.UTC)
	rawLastError := "etsy failed access_token=secret-token refresh_token=refresh-secret Bearer bearer-secret"
	run := &saleschannel.SyncRun{
		ConnectionID: connection.ID,
		Channel:      saleschannel.ChannelEtsy,
		Kind:         saleschannel.SyncOrders,
		Status:       saleschannel.SyncRunStatusFailed,
		TotalFetched: 7,
		Created:      3,
		Updated:      1,
		Skipped:      2,
		Errors:       1,
		LastError:    rawLastError,
		StartedAt:    time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		FinishedAt:   &finishedAt,
	}
	if err := env.repos.SalesChannels.CreateSyncRun(ctx, run); err != nil {
		t.Fatalf("create sync run: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sales-channels/sync-runs?channel=etsy&kind=orders", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Runs []saleschannel.SyncRun `json:"runs"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Runs) != 1 {
		t.Fatalf("expected 1 sync run, got %d: %#v", len(response.Runs), response.Runs)
	}
	got := response.Runs[0]
	if got.LastError == "" || got.LastError != saleschannel.SanitizeErrorMessage(rawLastError) {
		t.Fatalf("expected sanitized last_error, got %q", got.LastError)
	}
	for _, secret := range []string{"secret-token", "refresh-secret", "bearer-secret"} {
		if strings.Contains(got.LastError, secret) || strings.Contains(rr.Body.String(), secret) {
			t.Fatalf("sync run response leaked secret %q in body %s", secret, rr.Body.String())
		}
	}
}

func TestSalesChannelHandler_SyncRecordsSanitizedFailureRun(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelEtsy)
	provider := &fakeSalesChannelProvider{
		descriptor: saleschannel.ProviderDescriptor{
			ID:           saleschannel.ChannelEtsy,
			DisplayName:  "Etsy",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOrdersRead},
		},
		syncError: errors.New("oauth failed access_token=secret-token client_secret=client-secret code=oauth-code"),
	}
	router := newSalesChannelTestRouterWithRepo(t, env.repos.SalesChannels, provider)

	body := bytes.NewBufferString(`{"kind":"orders"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sales-channels/etsy/sync", body)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rr.Code, rr.Body.String())
	}
	for _, secret := range []string{"secret-token", "client-secret", "oauth-code"} {
		if strings.Contains(rr.Body.String(), secret) {
			t.Fatalf("sync error response leaked secret %q in body %s", secret, rr.Body.String())
		}
	}
	runs, err := env.repos.SalesChannels.ListSyncRuns(ctx, saleschannel.SyncRunFilter{Channel: saleschannel.ChannelEtsy, Kind: saleschannel.SyncOrders})
	if err != nil {
		t.Fatalf("list sync runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 failure run, got %d: %#v", len(runs), runs)
	}
	if runs[0].Status != saleschannel.SyncRunStatusFailed || runs[0].Errors != 1 || runs[0].FinishedAt == nil {
		t.Fatalf("unexpected failure run: %#v", runs[0])
	}
	for _, secret := range []string{"secret-token", "client-secret", "oauth-code"} {
		if strings.Contains(runs[0].LastError, secret) {
			t.Fatalf("stored sync error leaked secret %q in %q", secret, runs[0].LastError)
		}
	}
}

func newSalesChannelTestRouter(t *testing.T, providers ...saleschannel.Provider) http.Handler {
	return newSalesChannelTestRouterWithRepo(t, nil, providers...)
}

func TestSalesChannelHandler_MercadoLivreWebhookStoresEventAndListsMetadata(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelMercadoLivre)
	provider := &fakeSalesChannelProvider{
		descriptor: saleschannel.ProviderDescriptor{
			ID:           saleschannel.ChannelMercadoLivre,
			DisplayName:  "Mercado Livre",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityWebhooks},
		},
	}
	router := newSalesChannelTestRouterWithRepo(t, env.repos.SalesChannels, provider)

	payload := `{"_id":"orders:/orders/2000000001:1","topic":"orders","resource":"/orders/2000000001","user_id":123,"access_token":"secret-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sales-channels/mercado_livre/webhook", strings.NewReader(payload))
	req.Header.Set("X-Request-Signature", "Bearer bearer-secret")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
	for _, secret := range []string{"secret-token", "bearer-secret"} {
		if strings.Contains(rr.Body.String(), secret) {
			t.Fatalf("webhook response leaked secret %q in body %s", secret, rr.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/sales-channels/mercado_livre/webhook-events?topic=orders", nil)
	listRR := httptest.NewRecorder()
	router.ServeHTTP(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("expected 200 listing events, got %d: %s", listRR.Code, listRR.Body.String())
	}
	for _, secret := range []string{"secret-token", "bearer-secret"} {
		if strings.Contains(listRR.Body.String(), secret) {
			t.Fatalf("webhook list leaked secret %q in body %s", secret, listRR.Body.String())
		}
	}
	var response struct {
		Events []saleschannel.WebhookEvent `json:"events"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode webhook events: %v", err)
	}
	if len(response.Events) != 1 {
		t.Fatalf("expected 1 event, got %d: %#v", len(response.Events), response.Events)
	}
	got := response.Events[0]
	if got.Channel != saleschannel.ChannelMercadoLivre || got.Topic != "orders" || got.ResourcePath != "/orders/2000000001" {
		t.Fatalf("unexpected event metadata: %#v", got)
	}
	if got.Payload != "" || got.Signature != "" {
		t.Fatalf("payload/signature should be omitted from listing: %#v", got)
	}
}

func newSalesChannelTestRouterWithRepo(t *testing.T, repo *repository.SalesChannelRepository, providers ...saleschannel.Provider) http.Handler {
	t.Helper()
	registry := saleschannel.NewRegistry()
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			t.Fatalf("register provider: %v", err)
		}
	}
	handler := NewSalesChannelHandler(registry, repo)
	router := chi.NewRouter()
	router.Route("/api/sales-channels", func(r chi.Router) {
		r.Get("/sync-runs", handler.ListSyncRuns)
		r.Post("/{channel}/webhook", handler.ReceiveWebhook)
		r.Get("/{channel}/webhook-events", handler.ListWebhookEvents)
		r.Post("/{channel}/sync", handler.Sync)
	})
	return router
}

type fakeSalesChannelProvider struct {
	descriptor saleschannel.ProviderDescriptor
	syncError  error
}

func (p *fakeSalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return p.descriptor
}

func (p *fakeSalesChannelProvider) Status(context.Context) (saleschannel.ConnectionStatus, error) {
	return saleschannel.ConnectionStatus{Channel: p.descriptor.ID, Connected: true}, nil
}

func (p *fakeSalesChannelProvider) Sync(_ context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
	if p.syncError != nil {
		return saleschannel.SyncResult{}, p.syncError
	}
	return saleschannel.SyncResult{Channel: p.descriptor.ID, Kind: kind, TotalFetched: 3}, nil
}

func (p *fakeSalesChannelProvider) ListOrders(context.Context, saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	return nil, nil
}

func (p *fakeSalesChannelProvider) GetOrder(context.Context, string) (*saleschannel.ExternalOrder, error) {
	return nil, nil
}

func (p *fakeSalesChannelProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, nil
}

func (p *fakeSalesChannelProvider) ListProducts(context.Context, saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	return nil, nil
}

func (p *fakeSalesChannelProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return nil
}

func (p *fakeSalesChannelProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return nil
}
