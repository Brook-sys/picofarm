package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/mercadolivre"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/olx"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/Brook-sys/picofarm/internal/shopee"
	"github.com/Brook-sys/picofarm/internal/storage"
)

func TestSalesChannelAdapters_DescriptorsExposeCapabilities(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)

	providers := []saleschannel.Provider{
		NewEtsySalesChannelProvider(NewEtsyService(repos.Etsy, "client-id", "http://localhost/callback", &SettingsService{repo: repos.Settings})),
		NewSquarespaceSalesChannelProvider(NewSquarespaceService(repos.Squarespace)),
		NewShopifySalesChannelProvider(NewShopifyService(repos.Shopify, nil, nil)),
		NewMercadoLivreSalesChannelProvider(),
		NewShopeeSalesChannelProvider(),
		NewOLXSalesChannelProvider(),
	}

	want := map[saleschannel.ChannelID]saleschannel.ProviderDescriptor{
		saleschannel.ChannelEtsy: {
			ID:           saleschannel.ChannelEtsy,
			DisplayName:  "Etsy",
			AuthType:     "oauth",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead, saleschannel.CapabilityInventoryWrite, saleschannel.CapabilityWebhooks},
		},
		saleschannel.ChannelSquarespace: {
			ID:           saleschannel.ChannelSquarespace,
			DisplayName:  "Squarespace",
			AuthType:     "api_key",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityAPIKey, saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead},
		},
		saleschannel.ChannelShopify: {
			ID:           saleschannel.ChannelShopify,
			DisplayName:  "Shopify",
			AuthType:     "oauth",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead},
		},
		saleschannel.ChannelMercadoLivre: {
			ID:           saleschannel.ChannelMercadoLivre,
			DisplayName:  "Mercado Livre",
			AuthType:     "oauth",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead, saleschannel.CapabilityInventoryWrite, saleschannel.CapabilityWebhooks},
		},
		saleschannel.ChannelShopee: {
			ID:           saleschannel.ChannelShopee,
			DisplayName:  "Shopee",
			AuthType:     "oauth",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead},
		},
		saleschannel.ChannelOLX: {
			ID:           saleschannel.ChannelOLX,
			DisplayName:  "OLX Brasil",
			AuthType:     "manual",
			Capabilities: []saleschannel.Capability{saleschannel.CapabilityProductsRead, saleschannel.CapabilityWebhooks},
		},
	}

	for _, provider := range providers {
		descriptor := provider.Descriptor()
		expected := want[descriptor.ID]
		if descriptor.DisplayName != expected.DisplayName {
			t.Fatalf("%s display name: expected %q, got %q", descriptor.ID, expected.DisplayName, descriptor.DisplayName)
		}
		if descriptor.AuthType != expected.AuthType {
			t.Fatalf("%s auth type: expected %q, got %q", descriptor.ID, expected.AuthType, descriptor.AuthType)
		}
		if !reflect.DeepEqual(descriptor.Capabilities, expected.Capabilities) {
			t.Fatalf("%s capabilities: expected %v, got %v", descriptor.ID, expected.Capabilities, descriptor.Capabilities)
		}
	}
}

func TestSalesChannelAdapters_StatusMapsLegacyIntegrations(t *testing.T) {
	ctx := context.Background()
	repos := openSalesChannelAdapterRepos(t)
	settings := &SettingsService{repo: repos.Settings}

	etsyProvider := NewEtsySalesChannelProvider(NewEtsyService(repos.Etsy, "client-id", "http://localhost/callback", settings))
	squarespaceProvider := NewSquarespaceSalesChannelProvider(NewSquarespaceService(repos.Squarespace))
	shopifyProvider := NewShopifySalesChannelProvider(NewShopifyService(repos.Shopify, nil, nil))

	etsyStatus, err := etsyProvider.Status(ctx)
	if err != nil {
		t.Fatalf("etsy disconnected status: %v", err)
	}
	if etsyStatus.Connected {
		t.Fatalf("expected Etsy to start disconnected")
	}

	now := time.Now().UTC()
	if err := repos.Etsy.SaveIntegration(ctx, &model.EtsyIntegration{
		ShopID:         12345,
		ShopName:       "Dragon Forge",
		UserID:         67890,
		AccessToken:    "test-access-token",
		RefreshToken:   "test-refresh-token",
		TokenExpiresAt: now.Add(time.Hour),
		Scopes:         []string{"transactions_r", "listings_r"},
		IsActive:       true,
		LastSyncAt:     &now,
	}); err != nil {
		t.Fatalf("save Etsy integration: %v", err)
	}
	if err := repos.Squarespace.SaveIntegration(ctx, &model.SquarespaceIntegration{
		SiteID:            "site-123",
		SiteTitle:         "Dragon Store",
		APIKey:            "test-api-key",
		IsActive:          true,
		LastOrderSyncAt:   &now,
		LastProductSyncAt: &now,
	}); err != nil {
		t.Fatalf("save Squarespace integration: %v", err)
	}
	if err := repos.Shopify.SaveCredentials(ctx, &model.ShopifyCredentials{
		ShopDomain:  "dragon-store.myshopify.com",
		AccessToken: "test-shopify-token",
	}); err != nil {
		t.Fatalf("save Shopify credentials: %v", err)
	}

	assertStatus(t, etsyProvider, saleschannel.ChannelEtsy, "12345", "Dragon Forge")
	assertStatus(t, squarespaceProvider, saleschannel.ChannelSquarespace, "site-123", "Dragon Store")
	assertStatus(t, shopifyProvider, saleschannel.ChannelShopify, "dragon-store.myshopify.com", "dragon-store.myshopify.com")
}

func TestOLXSalesChannelProvider_StatusUsesConfiguredAPIKey(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	settings := &SettingsService{repo: repos.Settings}
	ctx := context.Background()
	if err := settings.Set(ctx, "olx_api_key", "fake-olx-api-key"); err != nil {
		t.Fatalf("save OLX API key: %v", err)
	}
	provider := NewOLXSalesChannelProviderWithSettings(settings, fakeOLXClient{
		accountID: "olx-account-123",
		name:      "PicoFarm OLX",
	})

	status, err := provider.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Connected {
		t.Fatalf("expected OLX to be connected")
	}
	if status.Channel != saleschannel.ChannelOLX {
		t.Fatalf("expected channel %q, got %q", saleschannel.ChannelOLX, status.Channel)
	}
	if status.AccountID != "olx-account-123" || status.DisplayName != "PicoFarm OLX" {
		t.Fatalf("unexpected OLX status: %+v", status)
	}
}

func TestOLXSalesChannelProvider_StatusSanitizesClientErrors(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	settings := &SettingsService{repo: repos.Settings}
	ctx := context.Background()
	if err := settings.Set(ctx, "olx_api_key", "fake-olx-api-key"); err != nil {
		t.Fatalf("save OLX API key: %v", err)
	}
	provider := NewOLXSalesChannelProviderWithSettings(settings, fakeOLXClient{
		err: errors.New("olx api_key=super-secret token=also-secret"),
	})

	status, err := provider.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Connected {
		t.Fatalf("expected OLX to remain disconnected")
	}
	if strings.Contains(status.LastError, "super-secret") || strings.Contains(status.LastError, "also-secret") {
		t.Fatalf("expected sanitized OLX status error, got %q", status.LastError)
	}
}

func TestMercadoLivreSalesChannelProvider_StatusUsesInjectedClient(t *testing.T) {
	provider := NewMercadoLivreSalesChannelProviderWithClient(fakeMercadoLivreClient{
		user: &mercadolivre.User{ID: 123456789, Nickname: "PICO_TEST_USER", SiteID: "MLB"},
	})

	status, err := provider.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Connected {
		t.Fatalf("expected Mercado Livre to be connected")
	}
	if status.Channel != saleschannel.ChannelMercadoLivre {
		t.Fatalf("expected channel %q, got %q", saleschannel.ChannelMercadoLivre, status.Channel)
	}
	if status.AccountID != "123456789" || status.DisplayName != "PICO_TEST_USER" {
		t.Fatalf("unexpected Mercado Livre status: %+v", status)
	}
}

func TestMercadoLivreSalesChannelProvider_StatusSanitizesClientErrors(t *testing.T) {
	provider := NewMercadoLivreSalesChannelProviderWithClient(fakeMercadoLivreClient{
		err: errors.New("access_token=super-secret-token client_secret=also-secret"),
	})

	status, err := provider.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Connected {
		t.Fatalf("expected Mercado Livre to remain disconnected")
	}
	if status.LastError != "access_token=[REDACTED] client_secret=[REDACTED]" {
		t.Fatalf("expected sanitized error, got %q", status.LastError)
	}
}

func TestMercadoLivreSalesChannelProvider_SyncOrdersUpsertsExternalOrdersIdempotently(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	ctx := context.Background()
	provider := NewMercadoLivreSalesChannelProviderWithRepository(fakeMercadoLivreClient{
		user:  &mercadolivre.User{ID: 123456789, Nickname: "PICO_TEST_USER", SiteID: "MLB"},
		order: fakeMercadoLivreOrder(),
	}, repos.SalesChannels)

	result, err := provider.Sync(ctx, saleschannel.SyncOrders)
	if err != nil {
		t.Fatalf("sync orders: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 1 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected first sync result: %+v", result)
	}

	orders, err := repos.SalesChannels.ListExternalOrders(ctx, saleschannel.OrderFilter{Channel: saleschannel.ChannelMercadoLivre})
	if err != nil {
		t.Fatalf("list stored orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 stored order, got %d", len(orders))
	}
	firstID := orders[0].ID
	if orders[0].ExternalOrderID != "2000000001" || orders[0].CustomerName != "TEST_BUYER" || orders[0].TotalCents != 12990 {
		t.Fatalf("unexpected stored order: %+v", orders[0])
	}
	if len(orders[0].Items) != 1 || orders[0].Items[0].SKU != "DRAGON-RED" {
		t.Fatalf("unexpected stored items: %+v", orders[0].Items)
	}

	result, err = provider.Sync(ctx, saleschannel.SyncOrders)
	if err != nil {
		t.Fatalf("sync orders second: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 0 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected second sync result: %+v", result)
	}

	orders, err = repos.SalesChannels.ListExternalOrders(ctx, saleschannel.OrderFilter{Channel: saleschannel.ChannelMercadoLivre})
	if err != nil {
		t.Fatalf("list stored orders second: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != firstID {
		t.Fatalf("expected idempotent upsert to keep one row with ID %s, got %+v", firstID, orders)
	}
}

func TestMercadoLivreSalesChannelProvider_SyncProductsUpsertsExternalProductsIdempotently(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	ctx := context.Background()
	provider := NewMercadoLivreSalesChannelProviderWithRepository(fakeMercadoLivreClient{
		user: &mercadolivre.User{ID: 123456789, Nickname: "PICO_TEST_USER", SiteID: "MLB"},
		item: fakeMercadoLivreItem(),
	}, repos.SalesChannels)

	result, err := provider.Sync(ctx, saleschannel.SyncProducts)
	if err != nil {
		t.Fatalf("sync products: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 1 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected first sync result: %+v", result)
	}

	products, err := repos.SalesChannels.ListExternalProducts(ctx, saleschannel.ProductFilter{Channel: saleschannel.ChannelMercadoLivre})
	if err != nil {
		t.Fatalf("list stored products: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 stored product, got %d", len(products))
	}
	firstID := products[0].ID
	if products[0].ExternalProductID != "MLB123456789" || products[0].Title != "Printed Dragon Miniature" || products[0].PriceCents != 12990 {
		t.Fatalf("unexpected stored product: %+v", products[0])
	}
	if len(products[0].Variants) != 1 || products[0].Variants[0].SKU != "DRAGON-RED" || products[0].Variants[0].StockQuantity == nil || *products[0].Variants[0].StockQuantity != 12 {
		t.Fatalf("unexpected stored variants: %+v", products[0].Variants)
	}

	result, err = provider.Sync(ctx, saleschannel.SyncProducts)
	if err != nil {
		t.Fatalf("sync products second: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 0 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected second sync result: %+v", result)
	}

	products, err = repos.SalesChannels.ListExternalProducts(ctx, saleschannel.ProductFilter{Channel: saleschannel.ChannelMercadoLivre})
	if err != nil {
		t.Fatalf("list stored products second: %v", err)
	}
	if len(products) != 1 || products[0].ID != firstID {
		t.Fatalf("expected idempotent upsert to keep one row with ID %s, got %+v", firstID, products)
	}
}

func TestShopeeSalesChannelProvider_SyncOrdersUpsertsExternalOrdersIdempotently(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	ctx := context.Background()
	provider := NewShopeeSalesChannelProviderWithRepository(fakeShopeeClient{
		shop:  &shopee.Shop{ID: 987654321, Name: "PicoFarm Shopee"},
		order: fakeShopeeOrder(),
	}, repos.SalesChannels)

	result, err := provider.Sync(ctx, saleschannel.SyncOrders)
	if err != nil {
		t.Fatalf("sync orders: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 1 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected first sync result: %+v", result)
	}

	orders, err := repos.SalesChannels.ListExternalOrders(ctx, saleschannel.OrderFilter{Channel: saleschannel.ChannelShopee})
	if err != nil {
		t.Fatalf("list stored orders: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 stored order, got %d", len(orders))
	}
	firstID := orders[0].ID
	if orders[0].ExternalOrderID != "250709ABC123" || orders[0].CustomerName != "Shopee Buyer" || orders[0].TotalCents != 8990 || orders[0].Currency != "BRL" {
		t.Fatalf("unexpected stored order: %+v", orders[0])
	}
	if orders[0].RawJSON == "" {
		t.Fatalf("expected raw_json to be preserved internally")
	}
	if len(orders[0].Items) != 1 || orders[0].Items[0].SKU != "MODEL-RED" || orders[0].Items[0].ExternalLineItemID != "1001:2002" {
		t.Fatalf("unexpected stored items: %+v", orders[0].Items)
	}

	result, err = provider.Sync(ctx, saleschannel.SyncOrders)
	if err != nil {
		t.Fatalf("sync orders second: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 0 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected second sync result: %+v", result)
	}
	orders, err = repos.SalesChannels.ListExternalOrders(ctx, saleschannel.OrderFilter{Channel: saleschannel.ChannelShopee})
	if err != nil {
		t.Fatalf("list stored orders second: %v", err)
	}
	if len(orders) != 1 || orders[0].ID != firstID {
		t.Fatalf("expected idempotent upsert to keep one row with ID %s, got %+v", firstID, orders)
	}
}

func TestShopeeSalesChannelProvider_SyncProductsUpsertsExternalProductsIdempotently(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	ctx := context.Background()
	provider := NewShopeeSalesChannelProviderWithRepository(fakeShopeeClient{
		shop: &shopee.Shop{ID: 987654321, Name: "PicoFarm Shopee"},
		item: fakeShopeeItem(),
	}, repos.SalesChannels)

	result, err := provider.Sync(ctx, saleschannel.SyncProducts)
	if err != nil {
		t.Fatalf("sync products: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 1 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected first sync result: %+v", result)
	}

	products, err := repos.SalesChannels.ListExternalProducts(ctx, saleschannel.ProductFilter{Channel: saleschannel.ChannelShopee})
	if err != nil {
		t.Fatalf("list stored products: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 stored product, got %d", len(products))
	}
	firstID := products[0].ID
	if products[0].ExternalProductID != "1001" || products[0].Title != "Dragon Miniature" || products[0].PriceCents != 8990 {
		t.Fatalf("unexpected stored product: %+v", products[0])
	}
	if products[0].RawJSON == "" {
		t.Fatalf("expected raw_json to be preserved internally")
	}
	if len(products[0].Variants) != 1 || products[0].Variants[0].ExternalVariantID != "2002" || products[0].Variants[0].SKU != "MODEL-RED" || products[0].Variants[0].StockQuantity == nil || *products[0].Variants[0].StockQuantity != 7 {
		t.Fatalf("unexpected stored variants: %+v", products[0].Variants)
	}

	result, err = provider.Sync(ctx, saleschannel.SyncProducts)
	if err != nil {
		t.Fatalf("sync products second: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 0 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected second sync result: %+v", result)
	}
	products, err = repos.SalesChannels.ListExternalProducts(ctx, saleschannel.ProductFilter{Channel: saleschannel.ChannelShopee})
	if err != nil {
		t.Fatalf("list stored products second: %v", err)
	}
	if len(products) != 1 || products[0].ID != firstID {
		t.Fatalf("expected idempotent upsert to keep one row with ID %s, got %+v", firstID, products)
	}
}

func TestOLXSalesChannelProvider_SyncProductsUpsertsExternalProductsIdempotently(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)
	ctx := context.Background()
	settings := &SettingsService{repo: repos.Settings}
	_ = settings.Set(ctx, "olx_api_key", "fake-olx-key")
	provider := NewOLXSalesChannelProviderWithRepository(settings, fakeOLXClient{accountID: "olx-account-123", name: "PicoFarm OLX"}, repos.SalesChannels)

	result, err := provider.Sync(ctx, saleschannel.SyncProducts)
	if err != nil {
		t.Fatalf("sync products: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 1 || result.Updated != 0 || result.Skipped != 0 {
		t.Fatalf("unexpected first sync result: %+v", result)
	}

	products, err := repos.SalesChannels.ListExternalProducts(ctx, saleschannel.ProductFilter{Channel: saleschannel.ChannelOLX})
	if err != nil {
		t.Fatalf("list stored products: %v", err)
	}
	if len(products) != 1 {
		t.Fatalf("expected 1 stored product, got %d", len(products))
	}
	firstID := products[0].ID
	if products[0].ExternalProductID != "olx-ad-1" || products[0].Title != "Miniatura dragão 3D" || products[0].PriceCents != 12990 || products[0].Currency != "BRL" {
		t.Fatalf("unexpected stored product: %+v", products[0])
	}

	result, err = provider.Sync(ctx, saleschannel.SyncProducts)
	if err != nil {
		t.Fatalf("sync products second: %v", err)
	}
	if result.TotalFetched != 1 || result.Created != 0 || result.Updated != 1 || result.Skipped != 0 {
		t.Fatalf("unexpected second sync result: %+v", result)
	}

	products, err = repos.SalesChannels.ListExternalProducts(ctx, saleschannel.ProductFilter{Channel: saleschannel.ChannelOLX})
	if err != nil {
		t.Fatalf("list stored products second: %v", err)
	}
	if len(products) != 1 || products[0].ID != firstID {
		t.Fatalf("expected idempotent upsert to keep one row with ID %s, got %+v", firstID, products)
	}
}

func TestShopeeSalesChannelProvider_SyncSanitizesRateLimitErrors(t *testing.T) {
	provider := NewShopeeSalesChannelProviderWithRepository(fakeShopeeClient{
		err: errors.New("429 rate limit sign=abcdef access_token=secret-token partner_key=secret-key"),
	}, openSalesChannelAdapterRepos(t).SalesChannels)

	_, err := provider.Sync(context.Background(), saleschannel.SyncOrders)
	if err == nil {
		t.Fatalf("expected sync error")
	}
	if strings.Contains(err.Error(), "abcdef") || strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("expected sanitized sync error, got %q", err.Error())
	}
}

func TestServicesWireSalesChannelRegistryWithInitialAdapters(t *testing.T) {
	t.Run("default services", func(t *testing.T) {
		repos := openSalesChannelAdapterRepos(t)
		store := storage.NewLocalStorage(t.TempDir())
		services := NewServices(repos, store, printer.NewManager(), realtime.NewHub())

		assertInitialSalesChannelRegistry(t, services)
	})

	t.Run("configured services", func(t *testing.T) {
		repos := openSalesChannelAdapterRepos(t)
		store := storage.NewLocalStorage(t.TempDir())
		services := NewServicesWithConfig(repos, store, printer.NewManager(), realtime.NewHub(), ServicesConfig{
			Etsy: EtsyConfig{ClientID: "client-id", RedirectURI: "http://localhost/callback"},
		})

		assertInitialSalesChannelRegistry(t, services)
	})
}

func assertInitialSalesChannelRegistry(t *testing.T, services *Services) {
	t.Helper()
	if services.SalesChannels == nil {
		t.Fatalf("expected sales-channel registry to be wired")
	}

	descriptors := services.SalesChannels.Descriptors()
	got := make([]saleschannel.ChannelID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		got = append(got, descriptor.ID)
	}
	want := []saleschannel.ChannelID{saleschannel.ChannelEtsy, saleschannel.ChannelSquarespace, saleschannel.ChannelShopify, saleschannel.ChannelMercadoLivre, saleschannel.ChannelShopee, saleschannel.ChannelOLX}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected provider order %v, got %v", want, got)
	}
}

func assertStatus(t *testing.T, provider saleschannel.Provider, channel saleschannel.ChannelID, accountID, displayName string) {
	t.Helper()
	status, err := provider.Status(context.Background())
	if err != nil {
		t.Fatalf("%s status: %v", channel, err)
	}
	if status.Channel != channel {
		t.Fatalf("expected channel %q, got %q", channel, status.Channel)
	}
	if !status.Connected {
		t.Fatalf("expected %s to be connected", channel)
	}
	if status.AccountID != accountID {
		t.Fatalf("%s account ID: expected %q, got %q", channel, accountID, status.AccountID)
	}
	if status.DisplayName != displayName {
		t.Fatalf("%s display name: expected %q, got %q", channel, displayName, status.DisplayName)
	}
}

type fakeOLXClient struct {
	accountID string
	name      string
	err       error
}

func (f fakeOLXClient) ValidateAPIKey(context.Context, string) (string, string, error) {
	if f.err != nil {
		return "", "", f.err
	}
	return f.accountID, f.name, nil
}

func (f fakeOLXClient) ListAds(context.Context, string) ([]olx.Ad, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []olx.Ad{{
		ID:          "olx-ad-1",
		Title:       "Miniatura dragão 3D",
		Description: "Impresso sob demanda",
		URL:         "https://www.olx.com.br/item/olx-ad-1",
		Status:      "active",
		PriceCents:  12990,
		Currency:    "BRL",
		Visible:     true,
	}}, nil
}

type fakeMercadoLivreClient struct {
	user  *mercadolivre.User
	order *mercadolivre.Order
	item  *mercadolivre.Item
	err   error
}

func (f fakeMercadoLivreClient) GetCurrentUser(context.Context) (*mercadolivre.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f fakeMercadoLivreClient) GetOrder(context.Context, string) (*mercadolivre.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.order, nil
}

func (f fakeMercadoLivreClient) ListOrders(context.Context) ([]*mercadolivre.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.order == nil {
		return []*mercadolivre.Order{}, nil
	}
	return []*mercadolivre.Order{f.order}, nil
}

func (f fakeMercadoLivreClient) ListItems(context.Context) ([]*mercadolivre.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.item == nil {
		return []*mercadolivre.Item{}, nil
	}
	return []*mercadolivre.Item{f.item}, nil
}

func (f fakeMercadoLivreClient) GetItem(context.Context, string) (*mercadolivre.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.item, nil
}

func fakeMercadoLivreOrder() *mercadolivre.Order {
	order := &mercadolivre.Order{
		ID:          2000000001,
		Status:      "paid",
		TotalAmount: 129.90,
		CurrencyID:  "BRL",
	}
	order.Buyer.Nickname = "TEST_BUYER"
	order.Buyer.FirstName = "Test"
	order.Buyer.LastName = "Buyer"
	order.Buyer.Email = "buyer@example.test"
	item := mercadolivre.OrderItem{Quantity: 1, UnitPrice: 129.90, CurrencyID: "BRL"}
	item.Item.ID = "MLB123456789"
	item.Item.Title = "Printed Dragon Miniature"
	item.Item.SKU = "DRAGON-RED"
	order.Items = []mercadolivre.OrderItem{item}
	return order
}

func fakeMercadoLivreItem() *mercadolivre.Item {
	return &mercadolivre.Item{
		ID:                "MLB123456789",
		Title:             "Printed Dragon Miniature",
		Status:            "active",
		Permalink:         "https://produto.mercadolivre.com.br/MLB-123456789-printed-dragon-miniature",
		Price:             129.90,
		CurrencyID:        "BRL",
		AvailableQuantity: 12,
		SKU:               "DRAGON-RED",
	}
}

type fakeShopeeClient struct {
	shop  *shopee.Shop
	order *shopee.Order
	item  *shopee.Item
	err   error
}

func (f fakeShopeeClient) GetShop(context.Context) (*shopee.Shop, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.shop, nil
}

func (f fakeShopeeClient) ListOrders(context.Context) ([]*shopee.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.order == nil {
		return []*shopee.Order{}, nil
	}
	return []*shopee.Order{f.order}, nil
}

func (f fakeShopeeClient) ListItems(context.Context) ([]*shopee.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.item == nil {
		return []*shopee.Item{}, nil
	}
	return []*shopee.Item{f.item}, nil
}

func fakeShopeeOrder() *shopee.Order {
	return &shopee.Order{
		SN:          "250709ABC123",
		Status:      "READY_TO_SHIP",
		Currency:    "BRL",
		TotalAmount: 89.90,
		BuyerName:   "Shopee Buyer",
		Recipient:   "Recipient Example",
		ShippingID:  "SHIP123",
		RawJSON:     `{"order_sn":"250709ABC123"}`,
		CreatedAt:   time.Unix(1700000000, 0).UTC(),
		UpdatedAt:   time.Unix(1700000600, 0).UTC(),
		OrderItems: []shopee.OrderItem{{
			ItemID:    1001,
			ModelID:   2002,
			ItemSKU:   "ITEM-DRAGON",
			ModelSKU:  "MODEL-RED",
			Title:     "Dragon Miniature - Red",
			Quantity:  1,
			UnitPrice: 89.90,
			Currency:  "BRL",
		}},
	}
}

func fakeShopeeItem() *shopee.Item {
	stock := 7
	return &shopee.Item{
		ID:       1001,
		Title:    "Dragon Miniature",
		Status:   "NORMAL",
		URL:      "https://shopee.com.br/product/987654321/1001",
		Price:    89.90,
		Currency: "BRL",
		SKU:      "ITEM-DRAGON",
		Stock:    &stock,
		ImageURL: "https://example.test/dragon.jpg",
		RawJSON:  `{"item_id":1001}`,
		ModelList: []shopee.Model{{
			ID:       2002,
			SKU:      "MODEL-RED",
			Title:    "Red",
			Price:    89.90,
			Currency: "BRL",
			Stock:    &stock,
		}},
	}
}

func openSalesChannelAdapterRepos(t *testing.T) *repository.Repositories {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewRepositories(db)
}
