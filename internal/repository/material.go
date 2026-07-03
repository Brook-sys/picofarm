package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type MaterialRepository struct {
	db *sql.DB
}

// Create inserts a new material.
func (r *MaterialRepository) Create(ctx context.Context, m *model.Material) error {
	return r.CreateTx(ctx, r.db, m)
}

// CreateTx inserts a new material using the provided DBTX (supports transactions).
func (r *MaterialRepository) CreateTx(ctx context.Context, db DBTX, m *model.Material) error {
	m.ID = uuid.New()
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()

	printTempJSON, _ := json.Marshal(m.PrintTemp)
	bedTempJSON, _ := json.Marshal(m.BedTemp)

	_, err := db.ExecContext(ctx, `
		INSERT INTO materials (id, name, type, manufacturer, color, color_hex, density, cost_per_kg, print_temp, bed_temp, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.Name, m.Type, m.Manufacturer, m.Color, m.ColorHex, m.Density, m.CostPerKg, printTempJSON, bedTempJSON, m.Notes, m.CreatedAt, m.UpdatedAt)
	return err
}

// GetByID retrieves a material by ID.
func (r *MaterialRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Material, error) {
	var m model.Material
	var printTempJSON, bedTempJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, type, manufacturer, color, color_hex, density, cost_per_kg, print_temp, bed_temp, notes, low_threshold_grams, created_at, updated_at
		FROM materials WHERE id = ?
	`, id), &m.ID, &m.Name, &m.Type, &m.Manufacturer, &m.Color, &m.ColorHex, &m.Density, &m.CostPerKg, &printTempJSON, &bedTempJSON, &m.Notes, &m.LowThresholdGrams, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if printTempJSON != nil {
		json.Unmarshal(printTempJSON, &m.PrintTemp)
	}
	if bedTempJSON != nil {
		json.Unmarshal(bedTempJSON, &m.BedTemp)
	}
	return &m, nil
}

// List retrieves all materials.
func (r *MaterialRepository) List(ctx context.Context) ([]model.Material, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, manufacturer, color, color_hex, density, cost_per_kg, print_temp, bed_temp, notes, low_threshold_grams, created_at, updated_at
		FROM materials ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []model.Material
	for rows.Next() {
		var m model.Material
		var printTempJSON, bedTempJSON []byte
		if err := scanRow(rows, &m.ID, &m.Name, &m.Type, &m.Manufacturer, &m.Color, &m.ColorHex, &m.Density, &m.CostPerKg, &printTempJSON, &bedTempJSON, &m.Notes, &m.LowThresholdGrams, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if printTempJSON != nil {
			json.Unmarshal(printTempJSON, &m.PrintTemp)
		}
		if bedTempJSON != nil {
			json.Unmarshal(bedTempJSON, &m.BedTemp)
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}

// Delete removes a material by ID, clearing non-inventory references first.
func (r *MaterialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	var spoolCount int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_spools WHERE material_id = ? AND status != 'archived'`, id).Scan(&spoolCount); err != nil {
		return err
	}
	if spoolCount > 0 {
		return fmt.Errorf("material is used by %d spool(s) in inventory; remove those spools first", spoolCount)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE expense_items SET matched_material_id = NULL WHERE matched_material_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE project_supplies SET material_id = NULL WHERE material_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM material_spools WHERE material_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM materials WHERE id = ?`, id); err != nil {
		return err
	}

	return tx.Commit()
}

// Update updates an existing material.
func (r *MaterialRepository) Update(ctx context.Context, m *model.Material) error {
	printTempJSON, _ := json.Marshal(m.PrintTemp)
	bedTempJSON, _ := json.Marshal(m.BedTemp)
	m.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE materials SET name = ?, type = ?, manufacturer = ?, color = ?, color_hex = ?, density = ?,
			cost_per_kg = ?, print_temp = ?, bed_temp = ?, notes = ?, low_threshold_grams = ?, updated_at = ?
		WHERE id = ?
	`, m.Name, m.Type, m.Manufacturer, m.Color, m.ColorHex, m.Density, m.CostPerKg, printTempJSON, bedTempJSON, m.Notes, m.LowThresholdGrams, m.UpdatedAt, m.ID)
	return err
}

// FindByTypeManufacturerColor finds a material matching the given type, manufacturer, and color.
// Returns nil if no match is found.
func (r *MaterialRepository) FindByTypeManufacturerColor(ctx context.Context, matType model.MaterialType, manufacturer, color string) (*model.Material, error) {
	var m model.Material
	var printTempJSON, bedTempJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, type, manufacturer, color, color_hex, density, cost_per_kg, print_temp, bed_temp, notes, low_threshold_grams, created_at, updated_at
		FROM materials WHERE LOWER(type) = LOWER(?) AND LOWER(manufacturer) = LOWER(?) AND LOWER(color) = LOWER(?)
		LIMIT 1
	`, matType, manufacturer, color), &m.ID, &m.Name, &m.Type, &m.Manufacturer, &m.Color, &m.ColorHex, &m.Density, &m.CostPerKg, &printTempJSON, &bedTempJSON, &m.Notes, &m.LowThresholdGrams, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if printTempJSON != nil {
		json.Unmarshal(printTempJSON, &m.PrintTemp)
	}
	if bedTempJSON != nil {
		json.Unmarshal(bedTempJSON, &m.BedTemp)
	}
	return &m, nil
}

// FindByTypeAndName finds a material matching the given type and name (case-insensitive).
// Used for deduplicating supply materials.
func (r *MaterialRepository) FindByTypeAndName(ctx context.Context, matType model.MaterialType, name string) (*model.Material, error) {
	var m model.Material
	var printTempJSON, bedTempJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, type, manufacturer, color, color_hex, density, cost_per_kg, print_temp, bed_temp, notes, low_threshold_grams, created_at, updated_at
		FROM materials WHERE LOWER(type) = LOWER(?) AND LOWER(name) = LOWER(?)
		LIMIT 1
	`, matType, name), &m.ID, &m.Name, &m.Type, &m.Manufacturer, &m.Color, &m.ColorHex, &m.Density, &m.CostPerKg, &printTempJSON, &bedTempJSON, &m.Notes, &m.LowThresholdGrams, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if printTempJSON != nil {
		json.Unmarshal(printTempJSON, &m.PrintTemp)
	}
	if bedTempJSON != nil {
		json.Unmarshal(bedTempJSON, &m.BedTemp)
	}
	return &m, nil
}

// ListByType retrieves all materials of a given type, ordered by name.
func (r *MaterialRepository) ListByType(ctx context.Context, matType model.MaterialType) ([]model.Material, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, manufacturer, color, color_hex, density, cost_per_kg, print_temp, bed_temp, notes, low_threshold_grams, created_at, updated_at
		FROM materials WHERE LOWER(type) = LOWER(?) ORDER BY name ASC
	`, matType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []model.Material
	for rows.Next() {
		var m model.Material
		var printTempJSON, bedTempJSON []byte
		if err := scanRow(rows, &m.ID, &m.Name, &m.Type, &m.Manufacturer, &m.Color, &m.ColorHex, &m.Density, &m.CostPerKg, &printTempJSON, &bedTempJSON, &m.Notes, &m.LowThresholdGrams, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		if printTempJSON != nil {
			json.Unmarshal(printTempJSON, &m.PrintTemp)
		}
		if bedTempJSON != nil {
			json.Unmarshal(bedTempJSON, &m.BedTemp)
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}

// SpoolRepository handles material spool database operations.
