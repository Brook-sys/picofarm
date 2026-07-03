package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type SaleRepository struct {
	db *sql.DB
}

// Create inserts a new sale.
func (r *SaleRepository) Create(ctx context.Context, s *model.Sale) error {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	if s.Currency == "" {
		s.Currency = "USD"
	}
	if s.Channel == "" {
		s.Channel = model.SalesChannelOther
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sales (id, occurred_at, channel, platform, gross_cents, fees_cents, shipping_charged_cents, shipping_cost_cents, tax_collected_cents, net_cents, currency, project_id, customer_id, order_reference, customer_name, item_description, quantity, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.OccurredAt, s.Channel, s.Platform, s.GrossCents, s.FeesCents, s.ShippingChargedCents, s.ShippingCostCents, s.TaxCollectedCents, s.NetCents, s.Currency, s.ProjectID, s.CustomerID, s.OrderReference, s.CustomerName, s.ItemDescription, s.Quantity, s.Notes, s.CreatedAt, s.UpdatedAt)
	return err
}

// GetByID retrieves a sale by ID.
func (r *SaleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Sale, error) {
	var s model.Sale
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, occurred_at, channel, platform, gross_cents, fees_cents, shipping_charged_cents, shipping_cost_cents, tax_collected_cents, net_cents, currency, project_id, customer_id, order_reference, customer_name, item_description, quantity, notes, created_at, updated_at
		FROM sales WHERE id = ?
	`, id), &s.ID, &s.OccurredAt, &s.Channel, &s.Platform, &s.GrossCents, &s.FeesCents, &s.ShippingChargedCents, &s.ShippingCostCents, &s.TaxCollectedCents, &s.NetCents, &s.Currency, &s.ProjectID, &s.CustomerID, &s.OrderReference, &s.CustomerName, &s.ItemDescription, &s.Quantity, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

// List retrieves all sales.
func (r *SaleRepository) List(ctx context.Context, projectID *uuid.UUID) ([]model.Sale, error) {
	query := `SELECT id, occurred_at, channel, platform, gross_cents, fees_cents, shipping_charged_cents, shipping_cost_cents, tax_collected_cents, net_cents, currency, project_id, customer_id, order_reference, customer_name, item_description, quantity, notes, created_at, updated_at FROM sales`
	args := []interface{}{}

	if projectID != nil {
		query += ` WHERE project_id = ?`
		args = append(args, *projectID)
	}
	query += ` ORDER BY occurred_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sales []model.Sale
	for rows.Next() {
		var s model.Sale
		if err := scanRow(rows, &s.ID, &s.OccurredAt, &s.Channel, &s.Platform, &s.GrossCents, &s.FeesCents, &s.ShippingChargedCents, &s.ShippingCostCents, &s.TaxCollectedCents, &s.NetCents, &s.Currency, &s.ProjectID, &s.CustomerID, &s.OrderReference, &s.CustomerName, &s.ItemDescription, &s.Quantity, &s.Notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sales = append(sales, s)
	}
	return sales, rows.Err()
}

// ListSince retrieves sales that occurred on or after the given time.
func (r *SaleRepository) ListSince(ctx context.Context, since time.Time) ([]model.Sale, error) {
	query := `SELECT id, occurred_at, channel, platform, gross_cents, fees_cents, shipping_charged_cents, shipping_cost_cents, tax_collected_cents, net_cents, currency, project_id, customer_id, order_reference, customer_name, item_description, quantity, notes, created_at, updated_at FROM sales WHERE occurred_at >= ? ORDER BY occurred_at DESC`

	rows, err := r.db.QueryContext(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sales []model.Sale
	for rows.Next() {
		var s model.Sale
		if err := scanRow(rows, &s.ID, &s.OccurredAt, &s.Channel, &s.Platform, &s.GrossCents, &s.FeesCents, &s.ShippingChargedCents, &s.ShippingCostCents, &s.TaxCollectedCents, &s.NetCents, &s.Currency, &s.ProjectID, &s.CustomerID, &s.OrderReference, &s.CustomerName, &s.ItemDescription, &s.Quantity, &s.Notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sales = append(sales, s)
	}
	return sales, rows.Err()
}

// Update updates a sale.
func (r *SaleRepository) Update(ctx context.Context, s *model.Sale) error {
	s.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE sales SET occurred_at = ?, channel = ?, platform = ?, gross_cents = ?, fees_cents = ?, shipping_charged_cents = ?, shipping_cost_cents = ?, tax_collected_cents = ?, net_cents = ?, currency = ?, project_id = ?, customer_id = ?, order_reference = ?, customer_name = ?, item_description = ?, quantity = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`, s.OccurredAt, s.Channel, s.Platform, s.GrossCents, s.FeesCents, s.ShippingChargedCents, s.ShippingCostCents, s.TaxCollectedCents, s.NetCents, s.Currency, s.ProjectID, s.CustomerID, s.OrderReference, s.CustomerName, s.ItemDescription, s.Quantity, s.Notes, s.UpdatedAt, s.ID)
	return err
}

// Delete deletes a sale.
func (r *SaleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sales WHERE id = ?`, id)
	return err
}

// GetTotalsByDateRange calculates totals for a date range.
func (r *SaleRepository) GetTotalsByDateRange(ctx context.Context, start, end time.Time) (grossCents, netCents, feesCents int, count int, err error) {
	err = scanRow(r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(gross_cents), 0), COALESCE(SUM(net_cents), 0), COALESCE(SUM(fees_cents), 0), COUNT(*)
		FROM sales WHERE occurred_at >= ? AND occurred_at < ?
	`, start, end), &grossCents, &netCents, &feesCents, &count)
	return
}

// ProjectSalesRow represents aggregated sales data for a project.
type ProjectSalesRow struct {
	ProjectID   string
	ProjectName string
	GrossCents  int
	NetCents    int
	Count       int
	AvgCents    int
	FirstSale   string
	LastSale    string
}

// GetSalesByProject returns sales aggregated by project.
func (r *SaleRepository) GetSalesByProject(ctx context.Context) ([]ProjectSalesRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			COALESCE(s.project_id, ''),
			COALESCE(p.name, s.item_description),
			COALESCE(SUM(s.gross_cents), 0),
			COALESCE(SUM(s.net_cents), 0),
			COUNT(*),
			CAST(COALESCE(AVG(s.gross_cents), 0) AS INTEGER),
			MIN(s.occurred_at),
			MAX(s.occurred_at)
		FROM sales s
		LEFT JOIN projects p ON s.project_id = p.id
		GROUP BY COALESCE(s.project_id, s.item_description)
		ORDER BY SUM(s.gross_cents) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ProjectSalesRow
	for rows.Next() {
		var row ProjectSalesRow
		if err := rows.Scan(&row.ProjectID, &row.ProjectName, &row.GrossCents, &row.NetCents, &row.Count, &row.AvgCents, &row.FirstSale, &row.LastSale); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// TimeSeriesRow represents a single data point in a time series.
type TimeSeriesRow struct {
	DateBucket string
	Total      int
}

// ChannelBreakdownRow represents an aggregation by sales channel.
type ChannelBreakdownRow struct {
	Channel string
	Total   int
	Count   int
}

// GetSalesOverTime returns gross_cents grouped by date bucket.
func (r *SaleRepository) GetSalesOverTime(ctx context.Context, since time.Time, strftimeFmt string) ([]TimeSeriesRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT strftime(?, occurred_at) AS bucket, COALESCE(SUM(gross_cents), 0)
		FROM sales WHERE occurred_at >= ?
		GROUP BY bucket ORDER BY bucket
	`, strftimeFmt, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimeSeriesRow
	for rows.Next() {
		var row TimeSeriesRow
		if err := rows.Scan(&row.DateBucket, &row.Total); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetNetOverTime returns net_cents grouped by date bucket.
func (r *SaleRepository) GetNetOverTime(ctx context.Context, since time.Time, strftimeFmt string) ([]TimeSeriesRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT strftime(?, occurred_at) AS bucket, COALESCE(SUM(net_cents), 0)
		FROM sales WHERE occurred_at >= ?
		GROUP BY bucket ORDER BY bucket
	`, strftimeFmt, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimeSeriesRow
	for rows.Next() {
		var row TimeSeriesRow
		if err := rows.Scan(&row.DateBucket, &row.Total); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetSalesByChannel returns gross_cents and count grouped by channel.
func (r *SaleRepository) GetSalesByChannel(ctx context.Context, since time.Time) ([]ChannelBreakdownRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT channel, COALESCE(SUM(gross_cents), 0), COUNT(*)
		FROM sales WHERE occurred_at >= ?
		GROUP BY channel ORDER BY SUM(gross_cents) DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChannelBreakdownRow
	for rows.Next() {
		var row ChannelBreakdownRow
		if err := rows.Scan(&row.Channel, &row.Total, &row.Count); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// CategoryBreakdownRow represents an aggregation by expense category.
type CategoryBreakdownRow struct {
	Category string
	Total    int
	Count    int
}

// GetExpensesOverTime returns total_cents for confirmed expenses grouped by date bucket.
func (r *ExpenseRepository) GetExpensesOverTime(ctx context.Context, since time.Time, strftimeFmt string) ([]TimeSeriesRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT strftime(?, occurred_at) AS bucket, COALESCE(SUM(total_cents), 0)
		FROM expenses WHERE status = 'confirmed' AND occurred_at >= ?
		GROUP BY bucket ORDER BY bucket
	`, strftimeFmt, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TimeSeriesRow
	for rows.Next() {
		var row TimeSeriesRow
		if err := rows.Scan(&row.DateBucket, &row.Total); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetExpensesByCategory returns total_cents and count for confirmed expenses grouped by category.
func (r *ExpenseRepository) GetExpensesByCategory(ctx context.Context, since time.Time) ([]CategoryBreakdownRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(total_cents), 0), COUNT(*)
		FROM expenses WHERE status = 'confirmed' AND occurred_at >= ?
		GROUP BY category ORDER BY SUM(total_cents) DESC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CategoryBreakdownRow
	for rows.Next() {
		var row CategoryBreakdownRow
		if err := rows.Scan(&row.Category, &row.Total, &row.Count); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// TemplateRepository handles template database operations.
