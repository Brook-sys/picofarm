package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type PartRepository struct {
	db *sql.DB
}

// Create inserts a new part.
func (r *PartRepository) Create(ctx context.Context, p *model.Part) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	if p.Status == "" {
		p.Status = model.PartStatusDesign
	}
	if p.Quantity == 0 {
		p.Quantity = 1
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO parts (id, project_id, name, description, quantity, status, material_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.ProjectID, p.Name, p.Description, p.Quantity, p.Status, p.MaterialType, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetByID retrieves a part by ID.
func (r *PartRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Part, error) {
	var p model.Part
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, description, quantity, status, material_type, created_at, updated_at
		FROM parts WHERE id = ?
	`, id), &p.ID, &p.ProjectID, &p.Name, &p.Description, &p.Quantity, &p.Status, &p.MaterialType, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// ListByProject retrieves all parts for a project.
func (r *PartRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.Part, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, description, quantity, status, material_type, created_at, updated_at
		FROM parts WHERE project_id = ? ORDER BY created_at ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parts []model.Part
	for rows.Next() {
		var p model.Part
		if err := scanRow(rows, &p.ID, &p.ProjectID, &p.Name, &p.Description, &p.Quantity, &p.Status, &p.MaterialType, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

// Update updates a part.
func (r *PartRepository) Update(ctx context.Context, p *model.Part) error {
	p.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE parts SET name = ?, description = ?, quantity = ?, status = ?, material_type = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Description, p.Quantity, p.Status, p.MaterialType, p.UpdatedAt, p.ID)
	return err
}

// Delete removes a part.
func (r *PartRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// delete dependent queue items linked to print jobs of this part's designs
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM queue_items 
		WHERE source_type = 'print_job' 
		AND source_id IN (
			SELECT j.id FROM print_jobs j 
			JOIN designs d ON d.id = j.design_id 
			WHERE d.part_id = ?
		)
	`, id); err != nil {
		return err
	}
	// delete job events (cascade on job delete should handle, but be explicit)
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM job_events 
		WHERE job_id IN (
			SELECT j.id FROM print_jobs j 
			JOIN designs d ON d.id = j.design_id 
			WHERE d.part_id = ?
		)
	`, id); err != nil {
		return err
	}
	// delete print jobs
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM print_jobs 
		WHERE design_id IN (SELECT id FROM designs WHERE part_id = ?)
	`, id); err != nil {
		return err
	}
	// delete template links
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM template_designs
		WHERE design_id IN (SELECT id FROM designs WHERE part_id = ?)
	`, id); err != nil {
		return err
	}
	// delete design tags
	if _, err := r.db.ExecContext(ctx, `
		DELETE FROM design_tags 
		WHERE design_id IN (SELECT id FROM designs WHERE part_id = ?)
	`, id); err != nil {
		return err
	}
	// delete part tags
	if _, err := r.db.ExecContext(ctx, `DELETE FROM part_tags WHERE part_id = ?`, id); err != nil {
		return err
	}
	// unlink task checklist items
	if _, err := r.db.ExecContext(ctx, `UPDATE task_checklist_items SET part_id = NULL WHERE part_id = ?`, id); err != nil {
		return err
	}
	// delete designs
	if _, err := r.db.ExecContext(ctx, `DELETE FROM designs WHERE part_id = ?`, id); err != nil {
		return err
	}
	// delete part
	_, err := r.db.ExecContext(ctx, `DELETE FROM parts WHERE id = ?`, id)
	return err
}

// TaskRepository handles task database operations.
