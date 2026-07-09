package saleschannel

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type fakeProvider struct {
	descriptor ProviderDescriptor
}

func (p fakeProvider) Descriptor() ProviderDescriptor {
	return p.descriptor
}

func (p fakeProvider) Status(context.Context) (ConnectionStatus, error) {
	return ConnectionStatus{Channel: p.descriptor.ID, Connected: true}, nil
}

func (p fakeProvider) Sync(context.Context, SyncKind) (SyncResult, error) {
	return SyncResult{Channel: p.descriptor.ID}, nil
}

func (p fakeProvider) ListOrders(context.Context, OrderFilter) ([]ExternalOrder, error) {
	return nil, nil
}

func (p fakeProvider) GetOrder(context.Context, string) (*ExternalOrder, error) {
	return nil, nil
}

func (p fakeProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, nil
}

func (p fakeProvider) ListProducts(context.Context, ProductFilter) ([]ExternalProduct, error) {
	return nil, nil
}

func (p fakeProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return nil
}

func (p fakeProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return nil
}

func TestRegistryRegisterRejectsDuplicateChannelIDs(t *testing.T) {
	registry := NewRegistry()
	provider := fakeProvider{descriptor: ProviderDescriptor{ID: ChannelEtsy, DisplayName: "Etsy"}}

	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	err := registry.Register(provider)
	if !errors.Is(err, ErrProviderAlreadyRegistered) {
		t.Fatalf("expected ErrProviderAlreadyRegistered, got %v", err)
	}
}

func TestRegistryGetReturnsProviderOrMissingError(t *testing.T) {
	registry := NewRegistry()
	provider := fakeProvider{descriptor: ProviderDescriptor{ID: ChannelSquarespace, DisplayName: "Squarespace"}}

	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	got, err := registry.Get(ChannelSquarespace)
	if err != nil {
		t.Fatalf("get registered provider: %v", err)
	}
	if got.Descriptor().ID != ChannelSquarespace {
		t.Fatalf("expected squarespace provider, got %q", got.Descriptor().ID)
	}

	_, err = registry.Get(ChannelID("missing"))
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestRegistryDescriptorsAreReturnedInRegistrationOrder(t *testing.T) {
	registry := NewRegistry()
	providers := []fakeProvider{
		{descriptor: ProviderDescriptor{ID: ChannelEtsy, DisplayName: "Etsy", Capabilities: []Capability{CapabilityOAuth, CapabilityOrdersRead}}},
		{descriptor: ProviderDescriptor{ID: ChannelSquarespace, DisplayName: "Squarespace", Capabilities: []Capability{CapabilityAPIKey, CapabilityProductsRead}}},
		{descriptor: ProviderDescriptor{ID: ChannelShopify, DisplayName: "Shopify", Capabilities: []Capability{CapabilityOAuth}}},
		{descriptor: ProviderDescriptor{ID: ChannelShopee, DisplayName: "Shopee", Capabilities: []Capability{CapabilityOAuth, CapabilityOrdersRead, CapabilityProductsRead}}},
		{descriptor: ProviderDescriptor{ID: ChannelOLX, DisplayName: "OLX Brasil", Capabilities: []Capability{CapabilityProductsRead, CapabilityWebhooks}}},
	}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			t.Fatalf("register %s: %v", provider.Descriptor().ID, err)
		}
	}

	descriptors := registry.Descriptors()
	gotIDs := make([]ChannelID, 0, len(descriptors))
	for _, descriptor := range descriptors {
		gotIDs = append(gotIDs, descriptor.ID)
	}

	wantIDs := []ChannelID{ChannelEtsy, ChannelSquarespace, ChannelShopify, ChannelShopee, ChannelOLX}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("expected descriptor IDs %v, got %v", wantIDs, gotIDs)
	}

	if !descriptors[0].Supports(CapabilityOAuth) {
		t.Fatalf("expected Etsy descriptor to support OAuth")
	}
	if descriptors[1].Supports(CapabilityInventoryWrite) {
		t.Fatalf("did not expect Squarespace descriptor to support inventory writes")
	}
}

func TestCapabilityValidationRejectsUnknownValues(t *testing.T) {
	registry := NewRegistry()
	provider := fakeProvider{descriptor: ProviderDescriptor{
		ID:           ChannelID("custom"),
		DisplayName:  "Custom",
		Capabilities: []Capability{Capability("unknown")},
	}}

	err := registry.Register(provider)
	if !errors.Is(err, ErrInvalidCapability) {
		t.Fatalf("expected ErrInvalidCapability, got %v", err)
	}
}
