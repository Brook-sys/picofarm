package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/mercadolivre"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/Brook-sys/picofarm/internal/storage"
)

func TestSalesChannelAdapters_DescriptorsExposeCapabilities(t *testing.T) {
	repos := openSalesChannelAdapterRepos(t)

	providers := []saleschannel.Provider{
		NewEtsySalesChannelProvider(NewEtsyService(repos.Etsy, "client-id", "http://localhost/callback", &SettingsService{repo: repos.Settings})),
		NewSquarespaceSalesChannelProvider(NewSquarespaceService(repos.Squarespace)),
		NewShopifySalesChannelProvider(NewShopifyService(repos.Shopify, nil, nil)),
		NewMercadoLivreSalesChannelProvider(),
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
	want := []saleschannel.ChannelID{saleschannel.ChannelEtsy, saleschannel.ChannelSquarespace, saleschannel.ChannelShopify, saleschannel.ChannelMercadoLivre}
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

type fakeMercadoLivreClient struct {
	user *mercadolivre.User
	err  error
}

func (f fakeMercadoLivreClient) GetCurrentUser(context.Context) (*mercadolivre.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
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
