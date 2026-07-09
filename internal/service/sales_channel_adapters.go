package service

import (
	"context"
	"fmt"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/google/uuid"
)

// NewSalesChannelRegistry registers the initial legacy-backed sales-channel providers.
func NewSalesChannelRegistry(providers ...saleschannel.Provider) (*saleschannel.Registry, error) {
	registry := saleschannel.NewRegistry()
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// EtsySalesChannelProvider adapts the legacy Etsy service to the provider-neutral contract.
type EtsySalesChannelProvider struct {
	svc *EtsyService
}

// NewEtsySalesChannelProvider creates an Etsy sales-channel adapter.
func NewEtsySalesChannelProvider(svc *EtsyService) *EtsySalesChannelProvider {
	return &EtsySalesChannelProvider{svc: svc}
}

func (p *EtsySalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return saleschannel.ProviderDescriptor{
		ID:          saleschannel.ChannelEtsy,
		DisplayName: "Etsy",
		Description: "Etsy marketplace integration for OAuth, orders, listings, inventory, and webhooks.",
		Capabilities: []saleschannel.Capability{
			saleschannel.CapabilityOAuth,
			saleschannel.CapabilityOrdersRead,
			saleschannel.CapabilityProductsRead,
			saleschannel.CapabilityInventoryWrite,
			saleschannel.CapabilityWebhooks,
		},
		AuthType: "oauth",
		DocsURL:  "docs/SALES_CHANNELS.md#etsy",
	}
}

func (p *EtsySalesChannelProvider) Status(ctx context.Context) (saleschannel.ConnectionStatus, error) {
	status := saleschannel.ConnectionStatus{Channel: saleschannel.ChannelEtsy}
	if p == nil || p.svc == nil {
		return status, nil
	}
	integration, err := p.svc.GetStatus(ctx)
	if err != nil {
		return status, err
	}
	if integration == nil || !integration.IsActive {
		return status, nil
	}
	status.Connected = true
	status.DisplayName = integration.ShopName
	status.AccountID = fmt.Sprintf("%d", integration.ShopID)
	status.LastOrderSyncAt = integration.LastSyncAt
	status.LastProductSyncAt = integration.LastSyncAt
	return status, nil
}

func (p *EtsySalesChannelProvider) Sync(ctx context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
	result := saleschannel.SyncResult{Channel: saleschannel.ChannelEtsy, Kind: kind}
	if p == nil || p.svc == nil {
		return result, errSalesChannelProviderUnavailable(saleschannel.ChannelEtsy)
	}
	switch kind {
	case saleschannel.SyncOrders:
		legacy, err := p.svc.SyncReceipts(ctx)
		return saleschannelSyncResultFromLegacy(result, legacy), err
	case saleschannel.SyncProducts:
		legacy, err := p.svc.SyncListings(ctx)
		return saleschannelSyncResultFromLegacy(result, legacy), err
	case saleschannel.SyncAll:
		orders, err := p.svc.SyncReceipts(ctx)
		result = saleschannelSyncResultFromLegacy(result, orders)
		if err != nil {
			return result, err
		}
		products, err := p.svc.SyncListings(ctx)
		merged := saleschannelSyncResultFromLegacy(saleschannel.SyncResult{Channel: saleschannel.ChannelEtsy, Kind: kind}, products)
		result.TotalFetched += merged.TotalFetched
		result.Created += merged.Created
		result.Updated += merged.Updated
		result.Skipped += merged.Skipped
		result.Errors += merged.Errors
		return result, err
	default:
		return result, fmt.Errorf("unsupported Etsy sync kind: %s", kind)
	}
}

func (p *EtsySalesChannelProvider) ListOrders(context.Context, saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelEtsy, "orders")
}

func (p *EtsySalesChannelProvider) GetOrder(context.Context, string) (*saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelEtsy, "order")
}

func (p *EtsySalesChannelProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelEtsy, "process_order")
}

func (p *EtsySalesChannelProvider) ListProducts(context.Context, saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelEtsy, "products")
}

func (p *EtsySalesChannelProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelEtsy, "link_product")
}

func (p *EtsySalesChannelProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelEtsy, "unlink_product")
}

// SquarespaceSalesChannelProvider adapts the legacy Squarespace service to the provider-neutral contract.
type SquarespaceSalesChannelProvider struct {
	svc *SquarespaceService
}

// NewSquarespaceSalesChannelProvider creates a Squarespace sales-channel adapter.
func NewSquarespaceSalesChannelProvider(svc *SquarespaceService) *SquarespaceSalesChannelProvider {
	return &SquarespaceSalesChannelProvider{svc: svc}
}

func (p *SquarespaceSalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return saleschannel.ProviderDescriptor{
		ID:          saleschannel.ChannelSquarespace,
		DisplayName: "Squarespace",
		Description: "Squarespace Commerce integration for API-key based orders and products.",
		Capabilities: []saleschannel.Capability{
			saleschannel.CapabilityAPIKey,
			saleschannel.CapabilityOrdersRead,
			saleschannel.CapabilityProductsRead,
		},
		AuthType: "api_key",
		DocsURL:  "docs/SALES_CHANNELS.md#squarespace",
	}
}

func (p *SquarespaceSalesChannelProvider) Status(ctx context.Context) (saleschannel.ConnectionStatus, error) {
	status := saleschannel.ConnectionStatus{Channel: saleschannel.ChannelSquarespace}
	if p == nil || p.svc == nil {
		return status, nil
	}
	integration, err := p.svc.GetStatus(ctx)
	if err != nil {
		return status, err
	}
	if integration == nil || !integration.IsActive {
		return status, nil
	}
	status.Connected = true
	status.DisplayName = integration.SiteTitle
	status.AccountID = integration.SiteID
	status.LastOrderSyncAt = integration.LastOrderSyncAt
	status.LastProductSyncAt = integration.LastProductSyncAt
	return status, nil
}

func (p *SquarespaceSalesChannelProvider) Sync(ctx context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
	result := saleschannel.SyncResult{Channel: saleschannel.ChannelSquarespace, Kind: kind}
	if p == nil || p.svc == nil {
		return result, errSalesChannelProviderUnavailable(saleschannel.ChannelSquarespace)
	}
	switch kind {
	case saleschannel.SyncOrders:
		legacy, err := p.svc.SyncOrders(ctx)
		return saleschannelSyncResultFromLegacy(result, legacy), err
	case saleschannel.SyncProducts:
		legacy, err := p.svc.SyncProducts(ctx)
		return saleschannelSyncResultFromLegacy(result, legacy), err
	case saleschannel.SyncAll:
		orders, err := p.svc.SyncOrders(ctx)
		result = saleschannelSyncResultFromLegacy(result, orders)
		if err != nil {
			return result, err
		}
		products, err := p.svc.SyncProducts(ctx)
		merged := saleschannelSyncResultFromLegacy(saleschannel.SyncResult{Channel: saleschannel.ChannelSquarespace, Kind: kind}, products)
		result.TotalFetched += merged.TotalFetched
		result.Created += merged.Created
		result.Updated += merged.Updated
		result.Skipped += merged.Skipped
		result.Errors += merged.Errors
		return result, err
	default:
		return result, fmt.Errorf("unsupported Squarespace sync kind: %s", kind)
	}
}

func (p *SquarespaceSalesChannelProvider) ListOrders(context.Context, saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelSquarespace, "orders")
}

func (p *SquarespaceSalesChannelProvider) GetOrder(context.Context, string) (*saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelSquarespace, "order")
}

func (p *SquarespaceSalesChannelProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelSquarespace, "process_order")
}

func (p *SquarespaceSalesChannelProvider) ListProducts(context.Context, saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelSquarespace, "products")
}

func (p *SquarespaceSalesChannelProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelSquarespace, "link_product")
}

func (p *SquarespaceSalesChannelProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelSquarespace, "unlink_product")
}

// ShopifySalesChannelProvider adapts the partial legacy Shopify service to the provider-neutral contract.
type ShopifySalesChannelProvider struct {
	svc *ShopifyService
}

// NewShopifySalesChannelProvider creates a Shopify sales-channel adapter.
func NewShopifySalesChannelProvider(svc *ShopifyService) *ShopifySalesChannelProvider {
	return &ShopifySalesChannelProvider{svc: svc}
}

func (p *ShopifySalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return saleschannel.ProviderDescriptor{
		ID:          saleschannel.ChannelShopify,
		DisplayName: "Shopify",
		Description: "Partial Shopify integration for OAuth and order import. Product sync is not enabled yet.",
		Capabilities: []saleschannel.Capability{
			saleschannel.CapabilityOAuth,
			saleschannel.CapabilityOrdersRead,
		},
		AuthType: "oauth",
		DocsURL:  "docs/SALES_CHANNELS.md#shopify",
	}
}

func (p *ShopifySalesChannelProvider) Status(ctx context.Context) (saleschannel.ConnectionStatus, error) {
	status := saleschannel.ConnectionStatus{Channel: saleschannel.ChannelShopify}
	if p == nil || p.svc == nil {
		return status, nil
	}
	legacy, err := p.svc.GetStatus(ctx)
	if err != nil {
		return status, err
	}
	if legacy == nil || !legacy.Connected {
		return status, nil
	}
	status.Connected = true
	status.DisplayName = legacy.ShopDomain
	status.AccountID = legacy.ShopDomain
	status.LastOrderSyncAt = legacy.LastSyncAt
	return status, nil
}

func (p *ShopifySalesChannelProvider) Sync(ctx context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
	result := saleschannel.SyncResult{Channel: saleschannel.ChannelShopify, Kind: kind}
	if p == nil || p.svc == nil {
		return result, errSalesChannelProviderUnavailable(saleschannel.ChannelShopify)
	}
	if kind != saleschannel.SyncOrders && kind != saleschannel.SyncAll {
		return result, fmt.Errorf("unsupported Shopify sync kind: %s", kind)
	}
	legacy, err := p.svc.SyncOrders(ctx)
	return saleschannelSyncResultFromLegacy(result, legacy), err
}

func (p *ShopifySalesChannelProvider) ListOrders(context.Context, saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopify, "orders")
}

func (p *ShopifySalesChannelProvider) GetOrder(context.Context, string) (*saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopify, "order")
}

func (p *ShopifySalesChannelProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopify, "process_order")
}

func (p *ShopifySalesChannelProvider) ListProducts(context.Context, saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopify, "products")
}

func (p *ShopifySalesChannelProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelShopify, "link_product")
}

func (p *ShopifySalesChannelProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelShopify, "unlink_product")
}

func saleschannelSyncResultFromLegacy(result saleschannel.SyncResult, legacy *model.SyncResult) saleschannel.SyncResult {
	if legacy == nil {
		return result
	}
	result.TotalFetched = legacy.TotalFetched
	result.Created = legacy.Created
	result.Updated = legacy.Updated
	result.Skipped = legacy.Skipped
	result.Errors = legacy.Errors
	return result
}

func errSalesChannelProviderUnavailable(channel saleschannel.ChannelID) error {
	return fmt.Errorf("%s sales channel provider unavailable", channel)
}

func errSalesChannelReadModelPending(channel saleschannel.ChannelID, operation string) error {
	return fmt.Errorf("%s sales channel %s read model adapter is not implemented yet", channel, operation)
}
