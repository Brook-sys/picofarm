package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/google/uuid"
)

func TestSalesChannelHandler_ProcessExternalOrder(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	connection := seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelEtsy)
	project := seedSalesChannelProject(t, ctx, env, "Dragon", "DRAGON-RED")
	order := &saleschannel.ExternalOrder{
		ConnectionID:    connection.ID,
		Channel:         saleschannel.ChannelEtsy,
		ExternalOrderID: "etsy-2001",
		OrderNumber:     "#2001",
		CustomerName:    "Ada Lovelace",
		CustomerEmail:   "ada@example.test",
		TotalCents:      2599,
		Currency:        "USD",
		Status:          "paid",
		Items: []saleschannel.ExternalOrderItem{
			{ExternalLineItemID: "line-1", SKU: "DRAGON-RED", Title: "Red Dragon", Quantity: 2, UnitPriceCents: 1299, Currency: "USD", ProjectID: &project.ID},
		},
	}
	seedExternalOrder(t, ctx, env, order)

	req := httptest.NewRequest(http.MethodPost, "/api/sales-channels/orders/"+order.ID.String()+"/process", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response struct {
		Order model.Order `json:"order"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Order.ID == uuid.Nil || response.Order.Source != model.OrderSourceEtsy || response.Order.SourceOrderID != "etsy-2001" {
		t.Fatalf("unexpected processed order: %#v", response.Order)
	}
	if len(response.Order.Items) != 1 || response.Order.Items[0].ProjectID == nil || *response.Order.Items[0].ProjectID != project.ID {
		t.Fatalf("expected processed order item linked to project, got %#v", response.Order.Items)
	}

	processed := true
	orders, err := env.repos.SalesChannels.ListExternalOrders(ctx, saleschannel.OrderFilter{Processed: &processed})
	if err != nil {
		t.Fatalf("list processed external orders: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != order.ID || orders[0].OrderID == nil || *orders[0].OrderID != response.Order.ID {
		t.Fatalf("expected external order to be marked processed and linked, got %#v", orders)
	}
}

func TestSalesChannelHandler_LinkAndUnlinkExternalProduct(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	connection := seedSalesChannelConnection(t, ctx, env, saleschannel.ChannelSquarespace)
	project := seedSalesChannelProject(t, ctx, env, "Benchy", "BENCHY")
	product := &saleschannel.ExternalProduct{
		ConnectionID:      connection.ID,
		Channel:           saleschannel.ChannelSquarespace,
		ExternalProductID: "sq-product-1",
		Title:             "Benchy listing",
		Status:            "active",
		IsVisible:         true,
		Currency:          "USD",
		Variants: []saleschannel.ExternalProductVariant{
			{ExternalVariantID: "variant-1", SKU: "BENCHY", Title: "Default", PriceCents: 1200, Currency: "USD"},
		},
	}
	seedExternalProduct(t, ctx, env, product)

	body := map[string]any{
		"project_id":          project.ID.String(),
		"external_variant_id": product.Variants[0].ID.String(),
		"sku":                 "BENCHY",
		"sync_inventory":      true,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/sales-channels/products/"+product.ID.String()+"/link", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("link status = %d, body = %s", rr.Code, rr.Body.String())
	}
	links, err := env.repos.SalesChannels.ListProductLinks(ctx, product.ID)
	if err != nil {
		t.Fatalf("list product links: %v", err)
	}
	if len(links) != 1 || links[0].ProjectID != project.ID || links[0].ExternalVariantID == nil || *links[0].ExternalVariantID != product.Variants[0].ID || !links[0].SyncInventory {
		t.Fatalf("unexpected links after link: %#v", links)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/sales-channels/products/"+product.ID.String()+"/link", bytes.NewReader(payload))
	rr = httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("unlink status = %d, body = %s", rr.Code, rr.Body.String())
	}
	links, err = env.repos.SalesChannels.ListProductLinks(ctx, product.ID)
	if err != nil {
		t.Fatalf("list product links after unlink: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected links to be removed, got %#v", links)
	}
}

func seedSalesChannelProject(t *testing.T, ctx context.Context, env *testEnv, name string, sku string) *model.Project {
	t.Helper()
	project := &model.Project{Name: name, SKU: sku}
	if err := env.repos.Projects.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}
