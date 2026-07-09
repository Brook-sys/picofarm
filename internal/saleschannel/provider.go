package saleschannel

import (
	"context"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// Provider exposes one external commerce channel through PicoFarm's canonical
// sales-channel contract.
type Provider interface {
	Descriptor() ProviderDescriptor
	Status(ctx context.Context) (ConnectionStatus, error)
	Sync(ctx context.Context, kind SyncKind) (SyncResult, error)
	ListOrders(ctx context.Context, filter OrderFilter) ([]ExternalOrder, error)
	GetOrder(ctx context.Context, externalID string) (*ExternalOrder, error)
	ProcessOrder(ctx context.Context, externalID string) (*model.Order, error)
	ListProducts(ctx context.Context, filter ProductFilter) ([]ExternalProduct, error)
	LinkProduct(ctx context.Context, externalProductID string, projectID uuid.UUID, sku string) error
	UnlinkProduct(ctx context.Context, externalProductID string, projectID uuid.UUID) error
}
