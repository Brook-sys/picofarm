package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type FileRepository struct {
	db *sql.DB
}

// Create inserts a new file record.
func (r *FileRepository) Create(ctx context.Context, f *model.File) error {
	f.ID = uuid.New()
	f.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO files (id, hash, original_name, content_type, size_bytes, storage_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.Hash, f.OriginalName, f.ContentType, f.SizeBytes, f.StoragePath, f.CreatedAt)
	return err
}

// GetByID retrieves a file by ID.
func (r *FileRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.File, error) {
	var f model.File
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, hash, original_name, content_type, size_bytes, storage_path, created_at
		FROM files WHERE id = ?
	`, id), &f.ID, &f.Hash, &f.OriginalName, &f.ContentType, &f.SizeBytes, &f.StoragePath, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &f, err
}

// GetByHash retrieves a file by hash (for deduplication).
func (r *FileRepository) GetByHash(ctx context.Context, hash string) (*model.File, error) {
	var f model.File
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, hash, original_name, content_type, size_bytes, storage_path, created_at
		FROM files WHERE hash = ?
	`, hash), &f.ID, &f.Hash, &f.OriginalName, &f.ContentType, &f.SizeBytes, &f.StoragePath, &f.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &f, err
}

// Update updates file metadata while preserving the file ID and created_at.
func (r *FileRepository) Update(ctx context.Context, f *model.File) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE files SET original_name = ?, content_type = ?, size_bytes = ?, storage_path = ?
		WHERE id = ?
	`, f.OriginalName, f.ContentType, f.SizeBytes, f.StoragePath, f.ID)
	return err
}

// ExpenseRepository handles expense database operations.
