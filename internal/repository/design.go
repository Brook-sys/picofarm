package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type DesignRepository struct {
	db *sql.DB
}

// Create inserts a new design version.
func (r *DesignRepository) Create(ctx context.Context, d *model.Design) error {
	d.ID = uuid.New()
	d.CreatedAt = time.Now()

	// Get next version number for this part
	var maxVersion int
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) FROM designs WHERE part_id = ?
	`, d.PartID), &maxVersion)
	if err != nil {
		return err
	}
	d.Version = maxVersion + 1

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO designs (id, part_id, version, file_id, file_name, file_hash, file_size_bytes, file_type, notes, slice_profile, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.PartID, d.Version, d.FileID, d.FileName, d.FileHash, d.FileSizeBytes, d.FileType, d.Notes, d.SliceProfile, d.CreatedAt)
	return err
}

// GetByID retrieves a design by ID.
func (r *DesignRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Design, error) {
	var d model.Design
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, part_id, version, file_id, file_name, file_hash, file_size_bytes, file_type, notes, slice_profile, created_at
		FROM designs WHERE id = ?
	`, id), &d.ID, &d.PartID, &d.Version, &d.FileID, &d.FileName, &d.FileHash, &d.FileSizeBytes, &d.FileType, &d.Notes, &d.SliceProfile, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &d, err
}

// ListByPart retrieves all designs for a part.
func (r *DesignRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM designs WHERE id = ?`, id)
	return err
}

func (r *DesignRepository) ProjectHasFileDesign(ctx context.Context, projectID, fileID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM designs d
		JOIN parts p ON p.id = d.part_id
		WHERE p.project_id = ? AND d.file_id = ?
	`, projectID, fileID).Scan(&count)
	return count > 0, err
}

func (r *DesignRepository) ListByPart(ctx context.Context, partID uuid.UUID) ([]model.Design, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, part_id, version, file_id, file_name, file_hash, file_size_bytes, file_type, notes, slice_profile, created_at
		FROM designs WHERE part_id = ? ORDER BY version DESC
	`, partID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var designs []model.Design
	for rows.Next() {
		var d model.Design
		if err := scanRow(rows, &d.ID, &d.PartID, &d.Version, &d.FileID, &d.FileName, &d.FileHash, &d.FileSizeBytes, &d.FileType, &d.Notes, &d.SliceProfile, &d.CreatedAt); err != nil {
			return nil, err
		}
		designs = append(designs, d)
	}
	return designs, rows.Err()
}

// PrinterRepository handles printer database operations.
