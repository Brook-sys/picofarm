package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Brook-sys/picofarm/internal/saleschannel"
)

func TestSalesChannelHandler_ListExternalOrders(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	connection := seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelEtsy)

	seedExternalOrder(t, ctx, env, &saleschannel.ExternalOrder{
		ConnectionID:    connection.ID,
		Channel:         saleschannel.ChannelEtsy,
		ExternalOrderID: "etsy-1001",
		OrderNumber:     "#1001",
		CustomerName:    "Ada Lovelace",
		CustomerEmail:   "ada@example.test",
		TotalCents:      2599,
		Currency:        "USD",
		Status:          "paid",
		IsProcessed:     true,
		Items: []saleschannel.ExternalOrderItem{
			{ExternalLineItemID: "line-1", SKU: "DRAGON-RED", Title: "Red Dragon", Quantity: 2, UnitPriceCents: 1299, Currency: "USD"},
		},
	})
	seedExternalOrder(t, ctx, env, &saleschannel.ExternalOrder{
		ConnectionID:    connection.ID,
		Channel:         saleschannel.ChannelEtsy,
		ExternalOrderID: "etsy-1002",
		OrderNumber:     "#1002",
		CustomerName:    "Grace Hopper",
		TotalCents:      4200,
		Currency:        "USD",
		Status:          "open",
		Items: []saleschannel.ExternalOrderItem{
			{ExternalLineItemID: "line-2", SKU: "BENCHY", Title: "Benchy", Quantity: 1, UnitPriceCents: 4200, Currency: "USD"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sales-channels/orders?channel=etsy&processed=false", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Orders []saleschannel.ExternalOrder `json:"orders"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Orders) != 1 {
		t.Fatalf("expected 1 unprocessed order, got %d: %#v", len(response.Orders), response.Orders)
	}
	got := response.Orders[0]
	if got.Channel != saleschannel.ChannelEtsy || got.ExternalOrderID != "etsy-1002" || got.CustomerName != "Grace Hopper" {
		t.Fatalf("unexpected order: %#v", got)
	}
	if got.IsProcessed {
		t.Fatalf("expected unprocessed order: %#v", got)
	}
	if len(got.Items) != 1 || got.Items[0].SKU != "BENCHY" {
		t.Fatalf("expected order items in response, got %#v", got.Items)
	}
	if got.RawJSON != "" {
		t.Fatalf("raw_json must not be exposed by read-model response, got %q", got.RawJSON)
	}
}

func TestSalesChannelHandler_ListExternalProducts(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	connection := seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelSquarespace)

	seedExternalProduct(t, ctx, env, &saleschannel.ExternalProduct{
		ConnectionID:      connection.ID,
		Channel:           saleschannel.ChannelSquarespace,
		ExternalProductID: "product-1",
		Title:             "Articulated dragon",
		Description:       "A printed dragon",
		URL:               "https://example.test/products/dragon",
		Status:            "active",
		IsVisible:         true,
		PriceCents:        1599,
		Currency:          "USD",
		Variants: []saleschannel.ExternalProductVariant{
			{ExternalVariantID: "variant-1", SKU: "DRAGON-RED", Title: "Red", PriceCents: 1599, Currency: "USD"},
		},
	})
	seedExternalProduct(t, ctx, env, &saleschannel.ExternalProduct{
		ConnectionID:      connection.ID,
		Channel:           saleschannel.ChannelSquarespace,
		ExternalProductID: "product-2",
		Title:             "Hidden spare part",
		Status:            "draft",
		IsVisible:         false,
		Currency:          "USD",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/sales-channels/products?channel=squarespace&status=active", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Products []saleschannel.ExternalProduct `json:"products"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Products) != 1 {
		t.Fatalf("expected 1 active product, got %d: %#v", len(response.Products), response.Products)
	}
	got := response.Products[0]
	if got.Channel != saleschannel.ChannelSquarespace || got.ExternalProductID != "product-1" || got.Title != "Articulated dragon" {
		t.Fatalf("unexpected product: %#v", got)
	}
	if len(got.Variants) != 1 || got.Variants[0].SKU != "DRAGON-RED" {
		t.Fatalf("expected variants in response, got %#v", got.Variants)
	}
	if got.RawJSON != "" {
		t.Fatalf("raw_json must not be exposed by read-model response, got %q", got.RawJSON)
	}
}

func seedSalesChannelConnection(t *testing.T, ctx context.Context, env *testEnv, channel saleschannel.ChannelID) *saleschannel.Connection {
	t.Helper()
	connection := &saleschannel.Connection{
		Channel:      channel,
		AccountID:    string(channel) + "-account",
		DisplayName:  string(channel) + " test account",
		Status:       saleschannel.ConnectionStatusConnected,
		Capabilities: []saleschannel.Capability{saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead},
	}
	if err := env.repos.SalesChannels.UpsertConnection(ctx, connection); err != nil {
		t.Fatalf("upsert connection: %v", err)
	}
	return connection
}

func seedExternalOrder(t *testing.T, ctx context.Context, env *testEnv, order *saleschannel.ExternalOrder) {
	t.Helper()
	if err := env.repos.SalesChannels.UpsertExternalOrder(ctx, order); err != nil {
		t.Fatalf("upsert external order: %v", err)
	}
}

func seedExternalProduct(t *testing.T, ctx context.Context, env *testEnv, product *saleschannel.ExternalProduct) {
	t.Helper()
	if err := env.repos.SalesChannels.UpsertExternalProduct(ctx, product); err != nil {
		t.Fatalf("upsert external product: %v", err)
	}
}
