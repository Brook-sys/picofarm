package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/saleschannel"
	"github.com/google/uuid"
)

// SalesChannelRepository stores provider-neutral sales-channel state.
type SalesChannelRepository struct {
	db *sql.DB
}

// NewSalesChannelRepository creates a repository for canonical sales-channel storage.
func NewSalesChannelRepository(db *sql.DB) *SalesChannelRepository {
	return &SalesChannelRepository{db: db}
}

// UpsertConnection creates or updates a channel account connection.
func (r *SalesChannelRepository) UpsertConnection(ctx context.Context, connection *saleschannel.Connection) error {
	now := time.Now()
	if connection.ID == uuid.Nil {
		existing, err := r.connectionIDByProviderAccount(ctx, connection.Channel, connection.AccountID)
		if err != nil {
			return err
		}
		if existing != uuid.Nil {
			connection.ID = existing
		} else {
			connection.ID = uuid.New()
			connection.CreatedAt = now
		}
	}
	if connection.CreatedAt.IsZero() {
		connection.CreatedAt = now
	}
	connection.UpdatedAt = now
	if connection.Status == "" {
		connection.Status = saleschannel.ConnectionStatusDisconnected
	}
	capabilities, err := marshalCapabilities(connection.Capabilities)
	if err != nil {
		return err
	}
	if connection.ConfigJSON == "" {
		connection.ConfigJSON = "{}"
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO sales_channel_connections (
			id, channel, account_id, display_name, status, capabilities_json, config_json,
			last_order_sync_at, last_product_sync_at, last_error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(channel, account_id) DO UPDATE SET
			display_name = excluded.display_name,
			status = excluded.status,
			capabilities_json = excluded.capabilities_json,
			config_json = excluded.config_json,
			last_order_sync_at = excluded.last_order_sync_at,
			last_product_sync_at = excluded.last_product_sync_at,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at
	`, connection.ID, connection.Channel, connection.AccountID, connection.DisplayName, connection.Status, capabilities, connection.ConfigJSON, connection.LastOrderSync, connection.LastProductSync, connection.LastError, connection.CreatedAt, connection.UpdatedAt)
	return err
}

// GetConnection retrieves a connection by ID.
func (r *SalesChannelRepository) GetConnection(ctx context.Context, id uuid.UUID) (*saleschannel.Connection, error) {
	connection, err := r.scanConnection(r.db.QueryRowContext(ctx, `
		SELECT id, channel, account_id, display_name, status, capabilities_json, config_json,
			last_order_sync_at, last_product_sync_at, last_error, created_at, updated_at
		FROM sales_channel_connections WHERE id = ?
	`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return connection, err
}

func (r *SalesChannelRepository) connectionIDByProviderAccount(ctx context.Context, channel saleschannel.ChannelID, accountID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := scanRow(r.db.QueryRowContext(ctx, `SELECT id FROM sales_channel_connections WHERE channel = ? AND account_id = ?`, channel, accountID), &id)
	if err == sql.ErrNoRows {
		return uuid.Nil, nil
	}
	return id, err
}

// CreateSyncRun records a sync attempt start.
func (r *SalesChannelRepository) CreateSyncRun(ctx context.Context, run *saleschannel.SyncRun) error {
	now := time.Now()
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = now
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.Status == "" {
		run.Status = saleschannel.SyncRunStatusRunning
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sales_channel_sync_runs (
			id, connection_id, channel, kind, status, total_fetched, created_count,
			updated_count, skipped_count, error_count, last_error, started_at, finished_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.ID, run.ConnectionID, run.Channel, run.Kind, run.Status, run.TotalFetched, run.Created, run.Updated, run.Skipped, run.Errors, run.LastError, run.StartedAt, run.FinishedAt, run.CreatedAt)
	return err
}

// FinishSyncRun updates counters and terminal state for a sync attempt.
func (r *SalesChannelRepository) FinishSyncRun(ctx context.Context, run *saleschannel.SyncRun) error {
	if run.FinishedAt == nil {
		now := time.Now()
		run.FinishedAt = &now
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE sales_channel_sync_runs
		SET status = ?, total_fetched = ?, created_count = ?, updated_count = ?, skipped_count = ?,
			error_count = ?, last_error = ?, finished_at = ?
		WHERE id = ?
	`, run.Status, run.TotalFetched, run.Created, run.Updated, run.Skipped, run.Errors, run.LastError, run.FinishedAt, run.ID)
	return err
}

// ListSyncRuns lists sync attempts in newest-first order.
func (r *SalesChannelRepository) ListSyncRuns(ctx context.Context, filter saleschannel.SyncRunFilter) ([]saleschannel.SyncRun, error) {
	query := `
		SELECT id, connection_id, channel, kind, status, total_fetched, created_count, updated_count,
			skipped_count, error_count, last_error, started_at, finished_at, created_at
		FROM sales_channel_sync_runs WHERE 1=1
	`
	args := []any{}
	if filter.ConnectionID != uuid.Nil {
		query += " AND connection_id = ?"
		args = append(args, filter.ConnectionID)
	}
	if filter.Channel != "" {
		query += " AND channel = ?"
		args = append(args, filter.Channel)
	}
	if filter.Kind != "" {
		query += " AND kind = ?"
		args = append(args, filter.Kind)
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []saleschannel.SyncRun{}
	for rows.Next() {
		var run saleschannel.SyncRun
		if err := scanRow(rows, &run.ID, &run.ConnectionID, &run.Channel, &run.Kind, &run.Status, &run.TotalFetched, &run.Created, &run.Updated, &run.Skipped, &run.Errors, &run.LastError, &run.StartedAt, &run.FinishedAt, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// UpsertExternalOrder stores a provider order and replaces its current line items.
func (r *SalesChannelRepository) UpsertExternalOrder(ctx context.Context, order *saleschannel.ExternalOrder) error {
	now := time.Now()
	if order.ID == uuid.Nil {
		existing, err := r.externalOrderID(ctx, order.ConnectionID, order.ExternalOrderID)
		if err != nil {
			return err
		}
		if existing != uuid.Nil {
			order.ID = existing
		} else {
			order.ID = uuid.New()
			order.CreatedAt = now
		}
	}
	if order.CreatedAt.IsZero() {
		order.CreatedAt = now
	}
	order.UpdatedAt = now
	if order.RawJSON == "" {
		order.RawJSON = "{}"
	}
	if order.Currency == "" {
		order.Currency = "USD"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sales_channel_external_orders (
			id, connection_id, channel, external_order_id, order_id, order_number, customer_name,
			customer_email, total_cents, currency, status, is_processed, raw_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(connection_id, external_order_id) DO UPDATE SET
			order_id = excluded.order_id,
			order_number = excluded.order_number,
			customer_name = excluded.customer_name,
			customer_email = excluded.customer_email,
			total_cents = excluded.total_cents,
			currency = excluded.currency,
			status = excluded.status,
			is_processed = excluded.is_processed,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at
	`, order.ID, order.ConnectionID, order.Channel, order.ExternalOrderID, order.OrderID, order.OrderNumber, order.CustomerName, order.CustomerEmail, order.TotalCents, order.Currency, order.Status, order.IsProcessed, order.RawJSON, order.CreatedAt, order.UpdatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sales_channel_external_order_items WHERE external_order_id = ?`, order.ID); err != nil {
		return err
	}
	for i := range order.Items {
		item := &order.Items[i]
		if item.ID == uuid.Nil {
			item.ID = uuid.New()
		}
		item.ExternalOrderID = order.ID
		if item.Quantity == 0 {
			item.Quantity = 1
		}
		if item.Currency == "" {
			item.Currency = order.Currency
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sales_channel_external_order_items (
				id, external_order_id, external_line_item_id, sku, title, quantity, unit_price_cents, currency, project_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.ID, item.ExternalOrderID, item.ExternalLineItemID, item.SKU, item.Title, item.Quantity, item.UnitPriceCents, item.Currency, item.ProjectID, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetExternalOrderByProviderID retrieves a stored provider order and its line items.
func (r *SalesChannelRepository) GetExternalOrderByProviderID(ctx context.Context, connectionID uuid.UUID, externalOrderID string) (*saleschannel.ExternalOrder, error) {
	order, err := r.scanExternalOrder(r.db.QueryRowContext(ctx, `
		SELECT id, connection_id, channel, external_order_id, order_id, order_number, customer_name,
			customer_email, total_cents, currency, status, is_processed, raw_json, created_at, updated_at
		FROM sales_channel_external_orders WHERE connection_id = ? AND external_order_id = ?
	`, connectionID, externalOrderID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	items, err := r.listExternalOrderItems(ctx, order.ID)
	if err != nil {
		return nil, err
	}
	order.Items = items
	return order, nil
}

// ListExternalOrders returns provider-neutral imported orders with optional filters.
func (r *SalesChannelRepository) ListExternalOrders(ctx context.Context, filter saleschannel.OrderFilter) ([]saleschannel.ExternalOrder, error) {
	query := `
		SELECT id, connection_id, channel, external_order_id, order_id, order_number, customer_name,
			customer_email, total_cents, currency, status, is_processed, raw_json, created_at, updated_at
		FROM sales_channel_external_orders WHERE 1=1
	`
	args := []any{}
	if filter.Channel != "" {
		query += " AND channel = ?"
		args = append(args, filter.Channel)
	}
	if filter.Processed != nil {
		query += " AND is_processed = ?"
		if *filter.Processed {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	query += " ORDER BY updated_at DESC, created_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	orders, err := r.listExternalOrderRows(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	for i := range orders {
		items, err := r.listExternalOrderItems(ctx, orders[i].ID)
		if err != nil {
			return nil, err
		}
		orders[i].Items = items
	}
	return orders, nil
}

func (r *SalesChannelRepository) externalOrderID(ctx context.Context, connectionID uuid.UUID, externalOrderID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := scanRow(r.db.QueryRowContext(ctx, `SELECT id FROM sales_channel_external_orders WHERE connection_id = ? AND external_order_id = ?`, connectionID, externalOrderID), &id)
	if err == sql.ErrNoRows {
		return uuid.Nil, nil
	}
	return id, err
}

// UpsertExternalProduct stores a provider listing and replaces its current variants.
func (r *SalesChannelRepository) UpsertExternalProduct(ctx context.Context, product *saleschannel.ExternalProduct) error {
	now := time.Now()
	if product.ID == uuid.Nil {
		existing, err := r.externalProductID(ctx, product.ConnectionID, product.ExternalProductID)
		if err != nil {
			return err
		}
		if existing != uuid.Nil {
			product.ID = existing
		} else {
			product.ID = uuid.New()
		}
	}
	if product.RawJSON == "" {
		product.RawJSON = "{}"
	}
	if product.Currency == "" {
		product.Currency = "USD"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sales_channel_external_products (
			id, connection_id, channel, external_product_id, title, description, url, status,
			is_visible, price_cents, currency, raw_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(connection_id, external_product_id) DO UPDATE SET
			title = excluded.title,
			description = excluded.description,
			url = excluded.url,
			status = excluded.status,
			is_visible = excluded.is_visible,
			price_cents = excluded.price_cents,
			currency = excluded.currency,
			raw_json = excluded.raw_json,
			updated_at = excluded.updated_at
	`, product.ID, product.ConnectionID, product.Channel, product.ExternalProductID, product.Title, product.Description, product.URL, product.Status, product.IsVisible, product.PriceCents, product.Currency, product.RawJSON, now, now)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sales_channel_external_product_variants WHERE external_product_id = ?`, product.ID); err != nil {
		return err
	}
	for i := range product.Variants {
		variant := &product.Variants[i]
		if variant.ID == uuid.Nil {
			variant.ID = uuid.New()
		}
		variant.ExternalProductID = product.ID
		if variant.Currency == "" {
			variant.Currency = product.Currency
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sales_channel_external_product_variants (
				id, external_product_id, external_variant_id, sku, title, price_cents, currency, stock_quantity, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, variant.ID, variant.ExternalProductID, variant.ExternalVariantID, variant.SKU, variant.Title, variant.PriceCents, variant.Currency, variant.StockQuantity, now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SalesChannelRepository) externalProductID(ctx context.Context, connectionID uuid.UUID, externalProductID string) (uuid.UUID, error) {
	var id uuid.UUID
	err := scanRow(r.db.QueryRowContext(ctx, `SELECT id FROM sales_channel_external_products WHERE connection_id = ? AND external_product_id = ?`, connectionID, externalProductID), &id)
	if err == sql.ErrNoRows {
		return uuid.Nil, nil
	}
	return id, err
}

// ListExternalProducts returns provider-neutral imported products/listings with optional filters.
func (r *SalesChannelRepository) ListExternalProducts(ctx context.Context, filter saleschannel.ProductFilter) ([]saleschannel.ExternalProduct, error) {
	query := `
		SELECT id, connection_id, channel, external_product_id, title, description, url, status,
			is_visible, price_cents, currency, raw_json
		FROM sales_channel_external_products WHERE 1=1
	`
	args := []any{}
	if filter.Channel != "" {
		query += " AND channel = ?"
		args = append(args, filter.Channel)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.Linked != nil {
		if *filter.Linked {
			query += " AND EXISTS (SELECT 1 FROM sales_channel_product_links l WHERE l.external_product_id = sales_channel_external_products.id)"
		} else {
			query += " AND NOT EXISTS (SELECT 1 FROM sales_channel_product_links l WHERE l.external_product_id = sales_channel_external_products.id)"
		}
	}
	query += " ORDER BY title ASC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	products, err := r.listExternalProductRows(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	for i := range products {
		variants, err := r.listExternalProductVariants(ctx, products[i].ID)
		if err != nil {
			return nil, err
		}
		products[i].Variants = variants
	}
	return products, nil
}

// UpsertProductLink stores a mapping between a provider product and PicoFarm project.
func (r *SalesChannelRepository) UpsertProductLink(ctx context.Context, link *saleschannel.ProductLink) error {
	now := time.Now()
	if link.ID == uuid.Nil {
		link.ID = uuid.New()
		link.CreatedAt = now
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = now
	}
	link.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sales_channel_product_links (
			id, connection_id, channel, external_product_id, external_variant_id, project_id,
			sku, sync_inventory, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(external_product_id, external_variant_id, project_id) DO UPDATE SET
			connection_id = excluded.connection_id,
			channel = excluded.channel,
			sku = excluded.sku,
			sync_inventory = excluded.sync_inventory,
			updated_at = excluded.updated_at
	`, link.ID, link.ConnectionID, link.Channel, link.ExternalProductID, link.ExternalVariantID, link.ProjectID, link.SKU, link.SyncInventory, link.CreatedAt, link.UpdatedAt)
	return err
}

// ListProductLinks lists project links for a provider product.
func (r *SalesChannelRepository) ListProductLinks(ctx context.Context, externalProductID uuid.UUID) ([]saleschannel.ProductLink, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, connection_id, channel, external_product_id, external_variant_id, project_id,
			sku, sync_inventory, created_at, updated_at
		FROM sales_channel_product_links WHERE external_product_id = ? ORDER BY created_at ASC
	`, externalProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := []saleschannel.ProductLink{}
	for rows.Next() {
		var link saleschannel.ProductLink
		if err := scanRow(rows, &link.ID, &link.ConnectionID, &link.Channel, &link.ExternalProductID, &link.ExternalVariantID, &link.ProjectID, &link.SKU, &link.SyncInventory, &link.CreatedAt, &link.UpdatedAt); err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (r *SalesChannelRepository) scanConnection(row scannable) (*saleschannel.Connection, error) {
	var connection saleschannel.Connection
	var capabilitiesJSON string
	err := scanRow(row, &connection.ID, &connection.Channel, &connection.AccountID, &connection.DisplayName, &connection.Status, &capabilitiesJSON, &connection.ConfigJSON, &connection.LastOrderSync, &connection.LastProductSync, &connection.LastError, &connection.CreatedAt, &connection.UpdatedAt)
	if err != nil {
		return nil, err
	}
	connection.Capabilities, err = unmarshalCapabilities(capabilitiesJSON)
	if err != nil {
		return nil, err
	}
	return &connection, nil
}

func (r *SalesChannelRepository) scanExternalOrder(row scannable) (*saleschannel.ExternalOrder, error) {
	var order saleschannel.ExternalOrder
	err := scanRow(row, &order.ID, &order.ConnectionID, &order.Channel, &order.ExternalOrderID, &order.OrderID, &order.OrderNumber, &order.CustomerName, &order.CustomerEmail, &order.TotalCents, &order.Currency, &order.Status, &order.IsProcessed, &order.RawJSON, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *SalesChannelRepository) listExternalOrderRows(ctx context.Context, query string, args ...any) ([]saleschannel.ExternalOrder, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []saleschannel.ExternalOrder{}
	for rows.Next() {
		order, err := r.scanExternalOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}
	return orders, rows.Err()
}

func (r *SalesChannelRepository) scanExternalProduct(row scannable) (*saleschannel.ExternalProduct, error) {
	var product saleschannel.ExternalProduct
	err := scanRow(row, &product.ID, &product.ConnectionID, &product.Channel, &product.ExternalProductID, &product.Title, &product.Description, &product.URL, &product.Status, &product.IsVisible, &product.PriceCents, &product.Currency, &product.RawJSON)
	if err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *SalesChannelRepository) listExternalProductRows(ctx context.Context, query string, args ...any) ([]saleschannel.ExternalProduct, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []saleschannel.ExternalProduct{}
	for rows.Next() {
		product, err := r.scanExternalProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, *product)
	}
	return products, rows.Err()
}

func (r *SalesChannelRepository) listExternalProductVariants(ctx context.Context, externalProductID uuid.UUID) ([]saleschannel.ExternalProductVariant, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, external_product_id, external_variant_id, sku, title, price_cents, currency, stock_quantity
		FROM sales_channel_external_product_variants WHERE external_product_id = ? ORDER BY created_at ASC
	`, externalProductID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	variants := []saleschannel.ExternalProductVariant{}
	for rows.Next() {
		var variant saleschannel.ExternalProductVariant
		if err := scanRow(rows, &variant.ID, &variant.ExternalProductID, &variant.ExternalVariantID, &variant.SKU, &variant.Title, &variant.PriceCents, &variant.Currency, &variant.StockQuantity); err != nil {
			return nil, err
		}
		variants = append(variants, variant)
	}
	return variants, rows.Err()
}

func (r *SalesChannelRepository) listExternalOrderItems(ctx context.Context, externalOrderID uuid.UUID) ([]saleschannel.ExternalOrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, external_order_id, external_line_item_id, sku, title, quantity, unit_price_cents, currency, project_id
		FROM sales_channel_external_order_items WHERE external_order_id = ? ORDER BY created_at ASC
	`, externalOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []saleschannel.ExternalOrderItem{}
	for rows.Next() {
		var item saleschannel.ExternalOrderItem
		if err := scanRow(rows, &item.ID, &item.ExternalOrderID, &item.ExternalLineItemID, &item.SKU, &item.Title, &item.Quantity, &item.UnitPriceCents, &item.Currency, &item.ProjectID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func marshalCapabilities(capabilities []saleschannel.Capability) (string, error) {
	if capabilities == nil {
		capabilities = []saleschannel.Capability{}
	}
	data, err := json.Marshal(capabilities)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func unmarshalCapabilities(data string) ([]saleschannel.Capability, error) {
	if data == "" {
		return []saleschannel.Capability{}, nil
	}
	var capabilities []saleschannel.Capability
	if err := json.Unmarshal([]byte(data), &capabilities); err != nil {
		return nil, err
	}
	if capabilities == nil {
		capabilities = []saleschannel.Capability{}
	}
	return capabilities, nil
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	_ = tx.Rollback()
}
