package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type ProjectSupplyRepository struct {
	db *sql.DB
}

// Create inserts a new project supply.
func (r *ProjectSupplyRepository) Create(ctx context.Context, s *model.ProjectSupply) error {
	s.ID = uuid.New()
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO project_supplies (id, project_id, name, unit_cost_cents, quantity, notes, material_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.ProjectID, s.Name, s.UnitCostCents, s.Quantity, s.Notes, s.MaterialID, s.CreatedAt, s.UpdatedAt)
	return err
}

// ListByProject retrieves all supplies for a project.
func (r *ProjectSupplyRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.ProjectSupply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, unit_cost_cents, quantity, notes, material_id, created_at, updated_at
		FROM project_supplies WHERE project_id = ? ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var supplies []model.ProjectSupply
	for rows.Next() {
		var s model.ProjectSupply
		if err := scanRow(rows, &s.ID, &s.ProjectID, &s.Name, &s.UnitCostCents, &s.Quantity, &s.Notes, &s.MaterialID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		supplies = append(supplies, s)
	}
	return supplies, rows.Err()
}

// Delete removes a project supply by ID.
func (r *ProjectSupplyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM project_supplies WHERE id = ?`, id)
	return err
}
