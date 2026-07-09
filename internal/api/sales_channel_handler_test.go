package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
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
