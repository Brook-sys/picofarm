package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/google/uuid"
)

func TestSalesChannelRepository_UpsertConnection(t *testing.T) {
	db := openTestDB(t)
	repo := NewSalesChannelRepository(db)
	ctx := context.Background()

	connection := &saleschannel.Connection{
		Channel:       saleschannel.ChannelEtsy,
		AccountID:     "shop-123",
		DisplayName:   "PicoFarm Etsy",
		Status:        saleschannel.ConnectionStatusConnected,
		Capabilities:  []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead},
		ConfigJSON:    `{"shop_id":"shop-123"}`,
		LastError:     "token=[REDACTED]",
		LastOrderSync: timePtr(time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)),
	}

	if err := repo.UpsertConnection(ctx, connection); err != nil {
		t.Fatalf("upsert connection: %v", err)
	}
	if connection.ID == uuid.Nil {
		t.Fatal("expected UpsertConnection to assign ID")
	}

	connection.DisplayName = "PicoFarm Etsy Updated"
	connection.Status = saleschannel.ConnectionStatusNeedsAttention
	connection.LastError = "oauth refresh failed: [REDACTED]"
	if err := repo.UpsertConnection(ctx, connection); err != nil {
		t.Fatalf("upsert existing connection: %v", err)
	}

	got, err := repo.GetConnection(ctx, connection.ID)
	if err != nil {
		t.Fatalf("get connection: %v", err)
	}
	if got == nil {
		t.Fatal("expected connection")
	}
	if got.DisplayName != "PicoFarm Etsy Updated" {
		t.Fatalf("display name = %q", got.DisplayName)
	}
	if got.Status != saleschannel.ConnectionStatusNeedsAttention {
		t.Fatalf("status = %q", got.Status)
	}
	if len(got.Capabilities) != 2 || got.Capabilities[0] != saleschannel.CapabilityOAuth || got.Capabilities[1] != saleschannel.CapabilityOrdersRead {
		t.Fatalf("capabilities = %#v", got.Capabilities)
	}
	if got.LastError != "oauth refresh failed: [REDACTED]" {
		t.Fatalf("last error = %q", got.LastError)
	}
	if got.LastOrderSync == nil || !got.LastOrderSync.Equal(*connection.LastOrderSync) {
		t.Fatalf("last order sync = %v", got.LastOrderSync)
	}
}

func TestSalesChannelRepository_SyncRunLifecycle(t *testing.T) {
	db := openTestDB(t)
	repo := NewSalesChannelRepository(db)
	ctx := context.Background()

	connection := testSalesChannelConnection(t, ctx, repo)
	run := &saleschannel.SyncRun{
		ConnectionID: connection.ID,
		Channel:      connection.Channel,
		Kind:         saleschannel.SyncOrders,
		Status:       saleschannel.SyncRunStatusRunning,
		StartedAt:    time.Date(2026, 7, 8, 11, 0, 0, 0, time.UTC),
	}
	if err := repo.CreateSyncRun(ctx, run); err != nil {
		t.Fatalf("create sync run: %v", err)
	}
	if run.ID == uuid.Nil {
		t.Fatal("expected CreateSyncRun to assign ID")
	}

	finishedAt := time.Date(2026, 7, 8, 11, 5, 0, 0, time.UTC)
	run.Status = saleschannel.SyncRunStatusSucceeded
	run.FinishedAt = &finishedAt
	run.TotalFetched = 4
	run.Created = 3
	run.Updated = 1
	if err := repo.FinishSyncRun(ctx, run); err != nil {
		t.Fatalf("finish sync run: %v", err)
	}

	runs, err := repo.ListSyncRuns(ctx, saleschannel.SyncRunFilter{ConnectionID: connection.ID})
	if err != nil {
		t.Fatalf("list sync runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 sync run, got %d", len(runs))
	}
	got := runs[0]
	if got.Status != saleschannel.SyncRunStatusSucceeded || got.TotalFetched != 4 || got.Created != 3 || got.Updated != 1 {
		t.Fatalf("unexpected run: %#v", got)
	}
	if got.FinishedAt == nil || !got.FinishedAt.Equal(finishedAt) {
		t.Fatalf("finished_at = %v", got.FinishedAt)
	}
}

func TestSalesChannelRepository_UpsertExternalOrderIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := NewSalesChannelRepository(db)
	ctx := context.Background()
	connection := testSalesChannelConnection(t, ctx, repo)

	order := &saleschannel.ExternalOrder{
		ConnectionID:    connection.ID,
		Channel:         connection.Channel,
		ExternalOrderID: "etsy-order-1",
		OrderNumber:     "#1001",
		CustomerName:    "Ada Lovelace",
		CustomerEmail:   "ada@example.test",
		TotalCents:      2599,
		Currency:        "USD",
		Status:          "paid",
		RawJSON:         `{"secret":"[REDACTED]"}`,
		Items: []saleschannel.ExternalOrderItem{
			{ExternalLineItemID: "line-1", SKU: "SKU-1", Title: "Widget", Quantity: 2, UnitPriceCents: 1299, Currency: "USD"},
		},
	}
	if err := repo.UpsertExternalOrder(ctx, order); err != nil {
		t.Fatalf("upsert order: %v", err)
	}
	firstID := order.ID

	order.CustomerName = "Ada Byron"
	order.TotalCents = 2699
	order.Items = append(order.Items, saleschannel.ExternalOrderItem{ExternalLineItemID: "line-2", SKU: "SKU-2", Title: "Addon", Quantity: 1, UnitPriceCents: 100, Currency: "USD"})
	if err := repo.UpsertExternalOrder(ctx, order); err != nil {
		t.Fatalf("upsert same order: %v", err)
	}
	if order.ID != firstID {
		t.Fatalf("expected idempotent upsert to keep ID %s, got %s", firstID, order.ID)
	}

	got, err := repo.GetExternalOrderByProviderID(ctx, connection.ID, "etsy-order-1")
	if err != nil {
		t.Fatalf("get external order: %v", err)
	}
	if got.CustomerName != "Ada Byron" || got.TotalCents != 2699 {
		t.Fatalf("order was not updated: %#v", got)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.Items))
	}
}

func TestSalesChannelRepository_UpsertExternalProductAndLink(t *testing.T) {
	db := openTestDB(t)
	repo := NewSalesChannelRepository(db)
	projectRepo := &ProjectRepository{db: db}
	ctx := context.Background()
	connection := testSalesChannelConnection(t, ctx, repo)
	project := testSalesChannelProject(t, ctx, projectRepo)

	product := &saleschannel.ExternalProduct{
		ConnectionID:      connection.ID,
		Channel:           connection.Channel,
		ExternalProductID: "listing-1",
		Title:             "Printed dragon",
		Status:            "active",
		IsVisible:         true,
		PriceCents:        1599,
		Currency:          "USD",
		RawJSON:           `{"listing":"listing-1"}`,
		Variants: []saleschannel.ExternalProductVariant{
			{ExternalVariantID: "variant-1", SKU: "DRAGON-RED", Title: "Red", PriceCents: 1599, Currency: "USD"},
		},
	}
	if err := repo.UpsertExternalProduct(ctx, product); err != nil {
		t.Fatalf("upsert product: %v", err)
	}
	if len(product.Variants) != 1 || product.Variants[0].ID == uuid.Nil {
		t.Fatalf("expected variant ID to be assigned: %#v", product.Variants)
	}

	link := &saleschannel.ProductLink{
		ConnectionID:      connection.ID,
		Channel:           connection.Channel,
		ExternalProductID: product.ID,
		ExternalVariantID: &product.Variants[0].ID,
		ProjectID:         project.ID,
		SKU:               "DRAGON-RED",
		SyncInventory:     true,
	}
	if err := repo.UpsertProductLink(ctx, link); err != nil {
		t.Fatalf("upsert product link: %v", err)
	}
	if link.ID == uuid.Nil {
		t.Fatal("expected link ID")
	}

	links, err := repo.ListProductLinks(ctx, product.ID)
	if err != nil {
		t.Fatalf("list product links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}
	if links[0].ProjectID != project.ID || links[0].SKU != "DRAGON-RED" || !links[0].SyncInventory {
		t.Fatalf("unexpected link: %#v", links[0])
	}
}

func testSalesChannelConnection(t *testing.T, ctx context.Context, repo *SalesChannelRepository) *saleschannel.Connection {
	t.Helper()
	connection := &saleschannel.Connection{
		Channel:      saleschannel.ChannelEtsy,
		AccountID:    "shop-123",
		DisplayName:  "PicoFarm Etsy",
		Status:       saleschannel.ConnectionStatusConnected,
		Capabilities: []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead},
	}
	if err := repo.UpsertConnection(ctx, connection); err != nil {
		t.Fatalf("upsert connection: %v", err)
	}
	return connection
}

func testSalesChannelProject(t *testing.T, ctx context.Context, repo *ProjectRepository) *model.Project {
	t.Helper()
	project := &model.Project{Name: "Dragon"}
	if err := repo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	return project
}

func timePtr(t time.Time) *time.Time {
	return &t
}
