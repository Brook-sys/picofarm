package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
)

type PrintArchiveRepository struct {
	db *sql.DB
}

func (r *PrintArchiveRepository) Create(ctx context.Context, a *model.PrintArchive) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	tags, _ := json.Marshal(a.Tags)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO print_archives (id, job_id, printer_id, status, start_time, end_time, duration_seconds, filament_used_grams, cost_cents, thumbnail_file_id, notes, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, a.ID, a.JobID, a.PrinterID, a.Status, a.StartTime, a.EndTime, a.DurationSeconds, a.FilamentUsedGrams, a.CostCents, a.ThumbnailFileID, a.Notes, string(tags), a.CreatedAt, a.UpdatedAt)
	return err
}

func (r *PrintArchiveRepository) List(ctx context.Context, printerID *uuid.UUID, status string) ([]model.PrintArchive, error) {
	query := `SELECT id, job_id, printer_id, status, start_time, end_time, duration_seconds, filament_used_grams, cost_cents, thumbnail_file_id, notes, tags, created_at, updated_at FROM print_archives`
	args := []interface{}{}
	if printerID != nil {
		query += ` WHERE printer_id = ?`
		args = append(args, *printerID)
	}
	if status != "" {
		if len(args) > 0 {
			query += ` AND status = ?`
		} else {
			query += ` WHERE status = ?`
		}
		args = append(args, status)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.PrintArchive
	for rows.Next() {
		var a model.PrintArchive
		var tags string
		if err := scanRow(rows, &a.ID, &a.JobID, &a.PrinterID, &a.Status, &a.StartTime, &a.EndTime, &a.DurationSeconds, &a.FilamentUsedGrams, &a.CostCents, &a.ThumbnailFileID, &a.Notes, &tags, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(tags), &a.Tags)
		list = append(list, a)
	}
	return list, rows.Err()
}
