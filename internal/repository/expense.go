package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type ExpenseRepository struct {
	db *sql.DB
}

// Create inserts a new expense.
func (r *ExpenseRepository) Create(ctx context.Context, e *model.Expense) error {
	e.ID = uuid.New()
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	if e.Status == "" {
		e.Status = model.ExpenseStatusPending
	}
	if e.Currency == "" {
		e.Currency = "USD"
	}
	if e.Category == "" {
		e.Category = model.ExpenseCategoryOther
	}

	rawAIJSON, _ := json.Marshal(e.RawAIResponse)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO expenses (id, occurred_at, vendor, subtotal_cents, tax_cents, shipping_cents, total_cents, currency, category, notes, receipt_file_id, receipt_file_path, status, raw_ocr_text, raw_ai_response, confidence, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.OccurredAt, e.Vendor, e.SubtotalCents, e.TaxCents, e.ShippingCents, e.TotalCents, e.Currency, e.Category, e.Notes, e.ReceiptFileID, e.ReceiptFilePath, e.Status, e.RawOCRText, rawAIJSON, e.Confidence, e.CreatedAt, e.UpdatedAt)
	return err
}

// GetByID retrieves an expense by ID.
func (r *ExpenseRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Expense, error) {
	var e model.Expense
	var rawAIJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, occurred_at, vendor, subtotal_cents, tax_cents, shipping_cents, total_cents, currency, category, notes, receipt_file_id, receipt_file_path, status, raw_ocr_text, raw_ai_response, confidence, created_at, updated_at
		FROM expenses WHERE id = ?
	`, id), &e.ID, &e.OccurredAt, &e.Vendor, &e.SubtotalCents, &e.TaxCents, &e.ShippingCents, &e.TotalCents, &e.Currency, &e.Category, &e.Notes, &e.ReceiptFileID, &e.ReceiptFilePath, &e.Status, &e.RawOCRText, &rawAIJSON, &e.Confidence, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if rawAIJSON != nil {
		e.RawAIResponse = rawAIJSON
	}
	return &e, err
}

// List retrieves all expenses.
func (r *ExpenseRepository) List(ctx context.Context, status *model.ExpenseStatus) ([]model.Expense, error) {
	query := `SELECT id, occurred_at, vendor, subtotal_cents, tax_cents, shipping_cents, total_cents, currency, category, notes, receipt_file_id, receipt_file_path, status, confidence, created_at, updated_at FROM expenses`
	args := []interface{}{}

	if status != nil {
		query += ` WHERE status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY occurred_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []model.Expense
	for rows.Next() {
		var e model.Expense
		if err := scanRow(rows, &e.ID, &e.OccurredAt, &e.Vendor, &e.SubtotalCents, &e.TaxCents, &e.ShippingCents, &e.TotalCents, &e.Currency, &e.Category, &e.Notes, &e.ReceiptFileID, &e.ReceiptFilePath, &e.Status, &e.Confidence, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

// ListSince retrieves expenses with the given status that occurred on or after the given time.
func (r *ExpenseRepository) ListSince(ctx context.Context, status *model.ExpenseStatus, since time.Time) ([]model.Expense, error) {
	query := `SELECT id, occurred_at, vendor, subtotal_cents, tax_cents, shipping_cents, total_cents, currency, category, notes, receipt_file_id, receipt_file_path, status, confidence, created_at, updated_at FROM expenses WHERE occurred_at >= ?`
	args := []interface{}{since}

	if status != nil {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY occurred_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var expenses []model.Expense
	for rows.Next() {
		var e model.Expense
		if err := scanRow(rows, &e.ID, &e.OccurredAt, &e.Vendor, &e.SubtotalCents, &e.TaxCents, &e.ShippingCents, &e.TotalCents, &e.Currency, &e.Category, &e.Notes, &e.ReceiptFileID, &e.ReceiptFilePath, &e.Status, &e.Confidence, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}

// Update updates an expense.
func (r *ExpenseRepository) Update(ctx context.Context, e *model.Expense) error {
	return r.UpdateTx(ctx, r.db, e)
}

// UpdateTx updates an expense using the provided DBTX (supports transactions).
func (r *ExpenseRepository) UpdateTx(ctx context.Context, db DBTX, e *model.Expense) error {
	e.UpdatedAt = time.Now()
	rawAIJSON, _ := json.Marshal(e.RawAIResponse)

	_, err := db.ExecContext(ctx, `
		UPDATE expenses SET occurred_at = ?, vendor = ?, subtotal_cents = ?, tax_cents = ?, shipping_cents = ?, total_cents = ?, currency = ?, category = ?, notes = ?, status = ?, raw_ocr_text = ?, raw_ai_response = ?, confidence = ?, updated_at = ?
		WHERE id = ?
	`, e.OccurredAt, e.Vendor, e.SubtotalCents, e.TaxCents, e.ShippingCents, e.TotalCents, e.Currency, e.Category, e.Notes, e.Status, e.RawOCRText, rawAIJSON, e.Confidence, e.UpdatedAt, e.ID)
	return err
}

// Delete deletes an expense.
func (r *ExpenseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM expenses WHERE id = ?`, id)
	return err
}

// ExpenseItemRepository handles expense item database operations (embedded in ExpenseRepository).

// CreateItem inserts a new expense item.
func (r *ExpenseRepository) CreateItem(ctx context.Context, item *model.ExpenseItem) error {
	item.ID = uuid.New()
	item.CreatedAt = time.Now()
	if item.ActionTaken == "" {
		item.ActionTaken = model.ExpenseItemActionNone
	}

	metadataJSON, _ := json.Marshal(item.Metadata)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO expense_items (id, expense_id, description, quantity, unit_price_cents, total_price_cents, sku, vendor_item_id, category, metadata, matched_spool_id, matched_material_id, confidence, action_taken, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.ExpenseID, item.Description, item.Quantity, item.UnitPriceCents, item.TotalPriceCents, item.SKU, item.VendorItemID, item.Category, metadataJSON, item.MatchedSpoolID, item.MatchedMaterialID, item.Confidence, item.ActionTaken, item.CreatedAt)
	return err
}

// GetItemsByExpenseID retrieves all items for an expense.
func (r *ExpenseRepository) GetItemsByExpenseID(ctx context.Context, expenseID uuid.UUID) ([]model.ExpenseItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, expense_id, description, quantity, unit_price_cents, total_price_cents, sku, vendor_item_id, category, metadata, matched_spool_id, matched_material_id, confidence, action_taken, created_at
		FROM expense_items WHERE expense_id = ? ORDER BY created_at
	`, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ExpenseItem
	for rows.Next() {
		var item model.ExpenseItem
		var metadataJSON []byte
		if err := scanRow(rows, &item.ID, &item.ExpenseID, &item.Description, &item.Quantity, &item.UnitPriceCents, &item.TotalPriceCents, &item.SKU, &item.VendorItemID, &item.Category, &metadataJSON, &item.MatchedSpoolID, &item.MatchedMaterialID, &item.Confidence, &item.ActionTaken, &item.CreatedAt); err != nil {
			return nil, err
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &item.Metadata)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateItem updates an expense item.
func (r *ExpenseRepository) UpdateItem(ctx context.Context, item *model.ExpenseItem) error {
	return r.UpdateItemTx(ctx, r.db, item)
}

// UpdateItemTx updates an expense item using the provided DBTX (supports transactions).
func (r *ExpenseRepository) UpdateItemTx(ctx context.Context, db DBTX, item *model.ExpenseItem) error {
	metadataJSON, _ := json.Marshal(item.Metadata)

	_, err := db.ExecContext(ctx, `
		UPDATE expense_items SET description = ?, quantity = ?, unit_price_cents = ?, total_price_cents = ?, sku = ?, vendor_item_id = ?, category = ?, metadata = ?, matched_spool_id = ?, matched_material_id = ?, confidence = ?, action_taken = ?
		WHERE id = ?
	`, item.Description, item.Quantity, item.UnitPriceCents, item.TotalPriceCents, item.SKU, item.VendorItemID, item.Category, metadataJSON, item.MatchedSpoolID, item.MatchedMaterialID, item.Confidence, item.ActionTaken, item.ID)
	return err
}

// DeleteItem deletes an expense item by ID.
func (r *ExpenseRepository) DeleteItem(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM expense_items WHERE id = ?`, id)
	return err
}

// DeleteItemsByExpenseID deletes all expense items for an expense.
func (r *ExpenseRepository) DeleteItemsByExpenseID(ctx context.Context, expenseID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM expense_items WHERE expense_id = ?`, expenseID)
	return err
}

// SaleRepository handles sale database operations.
