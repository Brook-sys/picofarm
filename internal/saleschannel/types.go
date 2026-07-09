package saleschannel

import (
	"time"

	"github.com/google/uuid"
)

// ChannelID identifies an external sales channel provider.
type ChannelID string

const (
	ChannelEtsy        ChannelID = "etsy"
	ChannelSquarespace ChannelID = "squarespace"
	ChannelShopify     ChannelID = "shopify"
)

// Capability describes an optional feature supported by a provider.
type Capability string

const (
	CapabilityOAuth          Capability = "oauth"
	CapabilityAPIKey         Capability = "api_key"
	CapabilityOrdersRead     Capability = "orders_read"
	CapabilityProductsRead   Capability = "products_read"
	CapabilityInventoryWrite Capability = "inventory_write"
	CapabilityWebhooks       Capability = "webhooks"
)

var validCapabilities = map[Capability]struct{}{
	CapabilityOAuth:          {},
	CapabilityAPIKey:         {},
	CapabilityOrdersRead:     {},
	CapabilityProductsRead:   {},
	CapabilityInventoryWrite: {},
	CapabilityWebhooks:       {},
}

// IsValid reports whether the capability is known by PicoFarm.
func (c Capability) IsValid() bool {
	_, ok := validCapabilities[c]
	return ok
}

// ProviderDescriptor is the frontend/API-visible summary of a channel provider.
type ProviderDescriptor struct {
	ID           ChannelID    `json:"id"`
	DisplayName  string       `json:"display_name"`
	Description  string       `json:"description,omitempty"`
	Capabilities []Capability `json:"capabilities"`
	AuthType     string       `json:"auth_type"`
	DocsURL      string       `json:"docs_url,omitempty"`
}

// Supports reports whether the descriptor declares a capability.
func (d ProviderDescriptor) Supports(capability Capability) bool {
	for _, candidate := range d.Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

// ConnectionStatus is the canonical status returned by a provider.
type ConnectionStatus struct {
	Channel           ChannelID  `json:"channel"`
	Connected         bool       `json:"connected"`
	DisplayName       string     `json:"display_name,omitempty"`
	AccountID         string     `json:"account_id,omitempty"`
	LastOrderSyncAt   *time.Time `json:"last_order_sync_at,omitempty"`
	LastProductSyncAt *time.Time `json:"last_product_sync_at,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
}

// SyncKind selects the class of external data to sync.
type SyncKind string

const (
	SyncOrders   SyncKind = "orders"
	SyncProducts SyncKind = "products"
	SyncAll      SyncKind = "all"
)

// SyncResult summarizes a sync operation without exposing provider secrets.
type SyncResult struct {
	Channel      ChannelID `json:"channel"`
	Kind         SyncKind  `json:"kind"`
	TotalFetched int       `json:"total_fetched"`
	Created      int       `json:"created"`
	Updated      int       `json:"updated"`
	Skipped      int       `json:"skipped"`
	Errors       int       `json:"errors"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
}

// OrderFilter constrains external order listing.
type OrderFilter struct {
	Channel   ChannelID
	Processed *bool
	Status    string
	Limit     int
	Offset    int
}

// ExternalOrder is a provider-neutral imported order/receipt.
type ExternalOrder struct {
	ID              uuid.UUID           `json:"id"`
	Channel         ChannelID           `json:"channel"`
	ExternalOrderID string              `json:"external_order_id"`
	OrderID         *uuid.UUID          `json:"order_id,omitempty"`
	OrderNumber     string              `json:"order_number"`
	CustomerName    string              `json:"customer_name"`
	CustomerEmail   string              `json:"customer_email,omitempty"`
	TotalCents      int                 `json:"total_cents"`
	Currency        string              `json:"currency"`
	Status          string              `json:"status,omitempty"`
	IsProcessed     bool                `json:"is_processed"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Items           []ExternalOrderItem `json:"items,omitempty"`
}

// ExternalOrderItem is a provider-neutral imported line item.
type ExternalOrderItem struct {
	ID                 uuid.UUID  `json:"id"`
	ExternalOrderID    uuid.UUID  `json:"external_order_id"`
	ExternalLineItemID string     `json:"external_line_item_id"`
	SKU                string     `json:"sku,omitempty"`
	Title              string     `json:"title"`
	Quantity           int        `json:"quantity"`
	UnitPriceCents     int        `json:"unit_price_cents"`
	Currency           string     `json:"currency"`
	ProjectID          *uuid.UUID `json:"project_id,omitempty"`
}

// ProductFilter constrains external product/listing listing.
type ProductFilter struct {
	Channel ChannelID
	Linked  *bool
	Status  string
	Limit   int
	Offset  int
}

// ExternalProduct is a provider-neutral imported product/listing.
type ExternalProduct struct {
	ID                uuid.UUID                `json:"id"`
	Channel           ChannelID                `json:"channel"`
	ExternalProductID string                   `json:"external_product_id"`
	Title             string                   `json:"title"`
	Description       string                   `json:"description,omitempty"`
	URL               string                   `json:"url,omitempty"`
	Status            string                   `json:"status,omitempty"`
	IsVisible         bool                     `json:"is_visible"`
	PriceCents        int                      `json:"price_cents,omitempty"`
	Currency          string                   `json:"currency,omitempty"`
	Variants          []ExternalProductVariant `json:"variants,omitempty"`
}

// ExternalProductVariant is a provider-neutral product variant/SKU.
type ExternalProductVariant struct {
	ID                uuid.UUID `json:"id"`
	ExternalProductID uuid.UUID `json:"external_product_id"`
	ExternalVariantID string    `json:"external_variant_id"`
	SKU               string    `json:"sku,omitempty"`
	Title             string    `json:"title"`
	PriceCents        int       `json:"price_cents,omitempty"`
	Currency          string    `json:"currency,omitempty"`
	StockQuantity     *int      `json:"stock_quantity,omitempty"`
}
