package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type SpoolRepository struct {
	db *sql.DB
}

// Create inserts a new spool.
func (r *SpoolRepository) Create(ctx context.Context, s *model.MaterialSpool) error {
	return r.CreateTx(ctx, r.db, s)
}

// CreateTx inserts a new spool using the provided DBTX (supports transactions).
func (r *SpoolRepository) CreateTx(ctx context.Context, db DBTX, s *model.MaterialSpool) error {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	if s.Status == "" {
		s.Status = model.SpoolStatusNew
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO material_spools (id, material_id, initial_weight, remaining_weight, purchase_date, purchase_cost, location, status, default_for_material, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.MaterialID, s.InitialWeight, s.RemainingWeight, s.PurchaseDate, s.PurchaseCost, s.Location, s.Status, s.DefaultForMaterial, s.Notes, s.CreatedAt, s.UpdatedAt)
	return err
}

// GetByID retrieves a spool by ID.
func (r *SpoolRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.MaterialSpool, error) {
	var s model.MaterialSpool
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, material_id, initial_weight, remaining_weight, purchase_date, purchase_cost, location, status, COALESCE(default_for_material, FALSE), notes, created_at, updated_at
		FROM material_spools WHERE id = ?
	`, id), &s.ID, &s.MaterialID, &s.InitialWeight, &s.RemainingWeight, &s.PurchaseDate, &s.PurchaseCost, &s.Location, &s.Status, &s.DefaultForMaterial, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

// List retrieves all spools.
func (r *SpoolRepository) List(ctx context.Context) ([]model.MaterialSpool, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, material_id, initial_weight, remaining_weight, purchase_date, purchase_cost, location, status, COALESCE(default_for_material, FALSE), notes, created_at, updated_at
		FROM material_spools WHERE status != 'archived' ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spools []model.MaterialSpool
	for rows.Next() {
		var s model.MaterialSpool
		if err := scanRow(rows, &s.ID, &s.MaterialID, &s.InitialWeight, &s.RemainingWeight, &s.PurchaseDate, &s.PurchaseCost, &s.Location, &s.Status, &s.DefaultForMaterial, &s.Notes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		spools = append(spools, s)
	}
	return spools, rows.Err()
}

// Delete deletes a spool by ID and clears queue/job references to it.
func (r *SpoolRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE queue_items SET assigned_spool_id = NULL, updated_at = ? WHERE assigned_spool_id = ?`, time.Now(), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE print_jobs SET material_spool_id = NULL WHERE material_spool_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE expense_items SET matched_spool_id = NULL WHERE matched_spool_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM material_spools WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// Update updates a spool.
func (r *SpoolRepository) Update(ctx context.Context, s *model.MaterialSpool) error {
	s.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE material_spools SET
			material_id = ?,
			initial_weight = ?,
			remaining_weight = ?,
			purchase_date = ?,
			purchase_cost = ?,
			location = ?,
			status = ?,
			default_for_material = ?,
			notes = ?,
			updated_at = ?
		WHERE id = ?
	`, s.MaterialID, s.InitialWeight, s.RemainingWeight, s.PurchaseDate, s.PurchaseCost, s.Location, s.Status, s.DefaultForMaterial, s.Notes, s.UpdatedAt, s.ID)
	return err
}

func (r *SpoolRepository) SetDefaultForMaterial(ctx context.Context, spoolID uuid.UUID) error {
	spool, err := r.GetByID(ctx, spoolID)
	if err != nil || spool == nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE material_spools SET default_for_material = FALSE, updated_at = ? WHERE material_id IN (SELECT id FROM materials WHERE type = (SELECT type FROM materials WHERE id = ?))`, now, spool.MaterialID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE material_spools SET default_for_material = TRUE, updated_at = ? WHERE id = ?`, now, spoolID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *SpoolRepository) EnsureDefaultForMaterialID(ctx context.Context, materialID uuid.UUID) error {
	var defaultCount int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_spools WHERE default_for_material = TRUE AND status NOT IN ('empty', 'archived') AND material_id IN (SELECT id FROM materials WHERE type = (SELECT type FROM materials WHERE id = ?))`, materialID).Scan(&defaultCount); err != nil {
		return err
	}
	if defaultCount > 0 {
		return nil
	}
	var spoolID uuid.UUID
	err := r.db.QueryRowContext(ctx, `SELECT ms.id FROM material_spools ms JOIN materials m ON m.id = ms.material_id WHERE m.type = (SELECT type FROM materials WHERE id = ?) AND ms.status NOT IN ('empty', 'archived') ORDER BY ms.created_at ASC LIMIT 1`, materialID).Scan(&spoolID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return r.SetDefaultForMaterial(ctx, spoolID)
}

func (r *SpoolRepository) GetDefaultForMaterialType(ctx context.Context, materialType string) (*model.MaterialSpool, error) {
	var s model.MaterialSpool
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT ms.id, ms.material_id, ms.initial_weight, ms.remaining_weight, ms.purchase_date, ms.purchase_cost, ms.location, ms.status, COALESCE(ms.default_for_material, FALSE), ms.notes, ms.created_at, ms.updated_at
		FROM material_spools ms
		JOIN materials m ON m.id = ms.material_id
		WHERE LOWER(m.type) = LOWER(?) AND ms.default_for_material = TRUE AND ms.status NOT IN ('empty', 'archived')
		LIMIT 1
	`, materialType), &s.ID, &s.MaterialID, &s.InitialWeight, &s.RemainingWeight, &s.PurchaseDate, &s.PurchaseCost, &s.Location, &s.Status, &s.DefaultForMaterial, &s.Notes, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &s, err
}

// PrintJobRepository handles print job database operations.
