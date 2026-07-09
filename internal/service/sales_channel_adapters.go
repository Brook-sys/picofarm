package service

import (
	"context"
	"fmt"

	"github.com/Brook-sys/picofarm/internal/mercadolivre"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
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

// ShopeeSalesChannelProvider is the conservative MVP descriptor for Shopee.
// Live auth, sync, inventory, and webhook behavior are added in later SHP cards.
type ShopeeSalesChannelProvider struct{}

// NewShopeeSalesChannelProvider creates a Shopee provider descriptor skeleton.
func NewShopeeSalesChannelProvider() *ShopeeSalesChannelProvider {
	return &ShopeeSalesChannelProvider{}
}

func (p *ShopeeSalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return saleschannel.ProviderDescriptor{
		ID:          saleschannel.ChannelShopee,
		DisplayName: "Shopee",
		Description: "Shopee Open Platform MVP integration for OAuth shop authorization plus signed V2 order and product reads.",
		Capabilities: []saleschannel.Capability{
			saleschannel.CapabilityOAuth,
			saleschannel.CapabilityOrdersRead,
			saleschannel.CapabilityProductsRead,
		},
		AuthType: "oauth",
		DocsURL:  "docs/SALES_CHANNELS.md#shopee-mvp-and-capability-matrix",
	}
}

func (p *ShopeeSalesChannelProvider) Status(context.Context) (saleschannel.ConnectionStatus, error) {
	return saleschannel.ConnectionStatus{Channel: saleschannel.ChannelShopee}, nil
}

func (p *ShopeeSalesChannelProvider) Sync(ctx context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
	return saleschannel.SyncResult{Channel: saleschannel.ChannelShopee, Kind: kind}, errSalesChannelReadModelPending(saleschannel.ChannelShopee, fmt.Sprintf("sync_%s", kind))
}

func (p *ShopeeSalesChannelProvider) ListOrders(context.Context, saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopee, "orders")
}

func (p *ShopeeSalesChannelProvider) GetOrder(context.Context, string) (*saleschannel.ExternalOrder, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopee, "order")
}

func (p *ShopeeSalesChannelProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopee, "process_order")
}

func (p *ShopeeSalesChannelProvider) ListProducts(context.Context, saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelShopee, "products")
}

func (p *ShopeeSalesChannelProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelShopee, "link_product")
}

func (p *ShopeeSalesChannelProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelShopee, "unlink_product")
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

// MercadoLivreClient is the Mercado Livre client surface the provider needs.
// Tests can provide fakes without network or credentials.
type MercadoLivreClient interface {
	GetCurrentUser(ctx context.Context) (*mercadolivre.User, error)
	ListOrders(ctx context.Context) ([]*mercadolivre.Order, error)
	GetOrder(ctx context.Context, externalOrderID string) (*mercadolivre.Order, error)
	ListItems(ctx context.Context) ([]*mercadolivre.Item, error)
	GetItem(ctx context.Context, itemID string) (*mercadolivre.Item, error)
}

// MercadoLivreSalesChannelProvider exposes the approved Mercado Livre MVP
// contract and delegates provider validation to an injected fakeable client.
// Operational sync/list/link methods remain fail-closed until follow-up cards.
type MercadoLivreSalesChannelProvider struct {
	client MercadoLivreClient
	repo   *repository.SalesChannelRepository
}

// NewMercadoLivreSalesChannelProvider creates the Mercado Livre provider shell.
func NewMercadoLivreSalesChannelProvider() *MercadoLivreSalesChannelProvider {
	return &MercadoLivreSalesChannelProvider{}
}

// NewMercadoLivreSalesChannelProviderWithClient creates a Mercado Livre provider with an injected client.
func NewMercadoLivreSalesChannelProviderWithClient(client MercadoLivreClient) *MercadoLivreSalesChannelProvider {
	return &MercadoLivreSalesChannelProvider{client: client}
}

// NewMercadoLivreSalesChannelProviderWithRepository creates a Mercado Livre provider with client and repository for sync.
func NewMercadoLivreSalesChannelProviderWithRepository(client MercadoLivreClient, repo *repository.SalesChannelRepository) *MercadoLivreSalesChannelProvider {
	return &MercadoLivreSalesChannelProvider{client: client, repo: repo}
}

func (p *MercadoLivreSalesChannelProvider) Descriptor() saleschannel.ProviderDescriptor {
	return saleschannel.ProviderDescriptor{
		ID:          saleschannel.ChannelMercadoLivre,
		DisplayName: "Mercado Livre",
		Description: "Mercado Livre marketplace integration for OAuth, orders, listings, inventory, and notifications. Live sync is implemented in follow-up cards.",
		Capabilities: []saleschannel.Capability{
			saleschannel.CapabilityOAuth,
			saleschannel.CapabilityOrdersRead,
			saleschannel.CapabilityProductsRead,
			saleschannel.CapabilityInventoryWrite,
			saleschannel.CapabilityWebhooks,
		},
		AuthType: "oauth",
		DocsURL:  "docs/SALES_CHANNELS.md#mercado-livre-discovery-matrix",
	}
}

func (p *MercadoLivreSalesChannelProvider) Status(ctx context.Context) (saleschannel.ConnectionStatus, error) {
	status := saleschannel.ConnectionStatus{Channel: saleschannel.ChannelMercadoLivre}
	if p.client == nil {
		return status, nil
	}
	user, err := p.client.GetCurrentUser(ctx)
	if err != nil {
		status.LastError = saleschannel.SanitizeErrorMessage(err.Error())
		return status, nil
	}
	status.Connected = true
	status.AccountID = fmt.Sprintf("%d", user.ID)
	status.DisplayName = user.Nickname
	return status, nil
}

func (p *MercadoLivreSalesChannelProvider) Sync(ctx context.Context, kind saleschannel.SyncKind) (saleschannel.SyncResult, error) {
	result := saleschannel.SyncResult{Channel: saleschannel.ChannelMercadoLivre, Kind: kind}
	if kind != saleschannel.SyncOrders && kind != saleschannel.SyncProducts && kind != saleschannel.SyncAll {
		return result, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, string(kind))
	}
	if p.client == nil || p.repo == nil {
		return result, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "sync")
	}
	connection, err := p.upsertConnection(ctx)
	if err != nil {
		return result, err
	}
	if kind == saleschannel.SyncOrders || kind == saleschannel.SyncAll {
		if err := p.syncOrders(ctx, connection, &result); err != nil {
			return result, err
		}
	}
	if kind == saleschannel.SyncProducts || kind == saleschannel.SyncAll {
		if err := p.syncProducts(ctx, connection, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (p *MercadoLivreSalesChannelProvider) syncOrders(ctx context.Context, connection *saleschannel.Connection, result *saleschannel.SyncResult) error {
	orders, err := p.client.ListOrders(ctx)
	if err != nil {
		return fmt.Errorf("mercado_livre list orders: %w", err)
	}
	for _, order := range orders {
		if order == nil {
			result.Skipped++
			continue
		}
		external := mercadoLivreExternalOrder(order)
		external.ConnectionID = connection.ID
		stored, err := p.repo.GetExternalOrderByProviderID(ctx, connection.ID, external.ExternalOrderID)
		if err != nil {
			return err
		}
		if err := p.repo.UpsertExternalOrder(ctx, &external); err != nil {
			return err
		}
		result.TotalFetched++
		if stored == nil {
			result.Created++
		} else {
			result.Updated++
		}
	}
	return nil
}

func (p *MercadoLivreSalesChannelProvider) syncProducts(ctx context.Context, connection *saleschannel.Connection, result *saleschannel.SyncResult) error {
	items, err := p.client.ListItems(ctx)
	if err != nil {
		return fmt.Errorf("mercado_livre list items: %w", err)
	}
	for _, item := range items {
		if item == nil {
			result.Skipped++
			continue
		}
		external := mercadoLivreExternalProduct(item)
		external.ConnectionID = connection.ID
		stored, err := p.repo.GetExternalProductByProviderID(ctx, connection.ID, external.ExternalProductID)
		if err != nil {
			return err
		}
		if err := p.repo.UpsertExternalProduct(ctx, &external); err != nil {
			return err
		}
		result.TotalFetched++
		if stored == nil {
			result.Created++
		} else {
			result.Updated++
		}
	}
	return nil
}

func (p *MercadoLivreSalesChannelProvider) ListOrders(ctx context.Context, filter saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	if p.client == nil {
		return nil, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "orders")
	}
	order, err := p.client.GetOrder(ctx, filter.Status)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return []saleschannel.ExternalOrder{}, nil
	}
	return []saleschannel.ExternalOrder{mercadoLivreExternalOrder(order)}, nil
}

func (p *MercadoLivreSalesChannelProvider) GetOrder(ctx context.Context, externalID string) (*saleschannel.ExternalOrder, error) {
	if p.client == nil {
		return nil, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "order")
	}
	order, err := p.client.GetOrder(ctx, externalID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, nil
	}
	external := mercadoLivreExternalOrder(order)
	return &external, nil
}

func mercadoLivreExternalOrder(order *mercadolivre.Order) saleschannel.ExternalOrder {
	external := saleschannel.ExternalOrder{
		Channel:         saleschannel.ChannelMercadoLivre,
		ExternalOrderID: fmt.Sprintf("%d", order.ID),
		OrderNumber:     fmt.Sprintf("%d", order.ID),
		CustomerName:    mercadoLivreBuyerName(order),
		CustomerEmail:   order.Buyer.Email,
		TotalCents:      int(order.TotalAmount * 100),
		Currency:        order.CurrencyID,
		Status:          order.Status,
		RawJSON:         "{}",
	}
	for _, item := range order.Items {
		external.Items = append(external.Items, saleschannel.ExternalOrderItem{
			ExternalLineItemID: item.Item.ID,
			SKU:                item.Item.SKU,
			Title:              item.Item.Title,
			Quantity:           item.Quantity,
			UnitPriceCents:     int(item.UnitPrice * 100),
			Currency:           item.CurrencyID,
		})
	}
	return external
}

func mercadoLivreBuyerName(order *mercadolivre.Order) string {
	if order.Buyer.Nickname != "" {
		return order.Buyer.Nickname
	}
	name := order.Buyer.FirstName
	if order.Buyer.LastName != "" {
		if name != "" {
			name += " "
		}
		name += order.Buyer.LastName
	}
	return name
}

func mercadoLivreExternalProduct(item *mercadolivre.Item) saleschannel.ExternalProduct {
	stock := item.AvailableQuantity
	product := saleschannel.ExternalProduct{
		Channel:           saleschannel.ChannelMercadoLivre,
		ExternalProductID: item.ID,
		Title:             item.Title,
		URL:               item.Permalink,
		Status:            item.Status,
		IsVisible:         item.Status == "active",
		PriceCents:        int(item.Price * 100),
		Currency:          item.CurrencyID,
		RawJSON:           "{}",
	}
	product.Variants = []saleschannel.ExternalProductVariant{{
		ExternalVariantID: item.ID,
		SKU:               item.SKU,
		Title:             item.Title,
		PriceCents:        product.PriceCents,
		Currency:          product.Currency,
		StockQuantity:     &stock,
	}}
	return product
}

func (p *MercadoLivreSalesChannelProvider) upsertConnection(ctx context.Context) (*saleschannel.Connection, error) {
	if p.repo == nil {
		return nil, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "connection")
	}
	user, err := p.client.GetCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("mercado_livre user: %w", err)
	}
	connection := &saleschannel.Connection{
		Channel:      saleschannel.ChannelMercadoLivre,
		AccountID:    fmt.Sprintf("%d", user.ID),
		DisplayName:  user.Nickname,
		Status:       saleschannel.ConnectionStatusConnected,
		Capabilities: []saleschannel.Capability{saleschannel.CapabilityOAuth, saleschannel.CapabilityOrdersRead, saleschannel.CapabilityProductsRead, saleschannel.CapabilityInventoryWrite, saleschannel.CapabilityWebhooks},
	}
	if err := p.repo.UpsertConnection(ctx, connection); err != nil {
		return nil, err
	}
	return connection, nil
}

func (p *MercadoLivreSalesChannelProvider) ProcessOrder(context.Context, string) (*model.Order, error) {
	return nil, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "process_order")
}

func (p *MercadoLivreSalesChannelProvider) ListProducts(ctx context.Context, filter saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	if p.repo != nil {
		filter.Channel = saleschannel.ChannelMercadoLivre
		return p.repo.ListExternalProducts(ctx, filter)
	}
	if p.client == nil {
		return nil, errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "products")
	}
	items, err := p.client.ListItems(ctx)
	if err != nil {
		return nil, err
	}
	products := make([]saleschannel.ExternalProduct, 0, len(items))
	for _, item := range items {
		if item != nil {
			products = append(products, mercadoLivreExternalProduct(item))
		}
	}
	return products, nil
}

func (p *MercadoLivreSalesChannelProvider) LinkProduct(context.Context, string, uuid.UUID, string) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "link_product")
}

func (p *MercadoLivreSalesChannelProvider) UnlinkProduct(context.Context, string, uuid.UUID) error {
	return errSalesChannelReadModelPending(saleschannel.ChannelMercadoLivre, "unlink_product")
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
