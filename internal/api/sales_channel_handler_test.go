package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
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
	wantIDs := []saleschannel.ChannelID{saleschannel.ChannelEtsy, saleschannel.ChannelSquarespace, saleschannel.ChannelShopify}
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

func newSalesChannelTestRouter(t *testing.T, providers ...saleschannel.Provider) http.Handler {
	t.Helper()
	registry := saleschannel.NewRegistry()
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			t.Fatalf("register provider: %v", err)
		}
	}
	handler := NewSalesChannelHandler(registry, nil)
	router := chi.NewRouter()
	router.Route("/api/sales-channels", func(r chi.Router) {
		r.Post("/{channel}/sync", handler.Sync)
	})
	return router
}

type fakeSalesChannelProvider struct {
	descriptor saleschannel.ProviderDescriptor
}

func (p *fakeSalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return p.descriptor
}

func (p *fakeSalesChannelProvider) Status(context.Context) (saleschannel.ConnectionStatus, error) {
	return saleschannel.ConnectionStatus{Channel: p.descriptor.ID, Connected: true}, nil
}

func (p *fakeSalesChannelProvider) Sync(_ context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
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
