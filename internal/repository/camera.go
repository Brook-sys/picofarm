package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
)

type CameraRepository struct {
	db *sql.DB
}

func (r *CameraRepository) Create(ctx context.Context, c *model.Camera) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO cameras (id, printer_id, name, type, url, enabled, token, token_expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, c.ID, c.PrinterID, c.Name, c.Type, c.URL, c.Enabled, c.Token, c.TokenExpiresAt, c.CreatedAt, c.UpdatedAt)
	return err
}

func (r *CameraRepository) List(ctx context.Context, printerID *uuid.UUID, enabled *bool) ([]model.Camera, error) {
	query := `SELECT id, printer_id, name, type, url, enabled, token, token_expires_at, created_at, updated_at FROM cameras`
	args := []interface{}{}
	if printerID != nil {
		query += ` WHERE printer_id = ?`
		args = append(args, *printerID)
	}
	if enabled != nil {
		if len(args) > 0 {
			query += ` AND enabled = ?`
		} else {
			query += ` WHERE enabled = ?`
		}
		args = append(args, *enabled)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Camera
	for rows.Next() {
		var c model.Camera
		if err := scanRow(rows, &c.ID, &c.PrinterID, &c.Name, &c.Type, &c.URL, &c.Enabled, &c.Token, &c.TokenExpiresAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *CameraRepository) Update(ctx context.Context, c *model.Camera) error {
	c.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE cameras SET name=?, type=?, url=?, enabled=?, token=?, token_expires_at=?, updated_at=? WHERE id=?
	`, c.Name, c.Type, c.URL, c.Enabled, c.Token, c.TokenExpiresAt, c.UpdatedAt, c.ID)
	return err
}

func (r *CameraRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM cameras WHERE id=?`, id)
	return err
}
