package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
)

type TimelapseRepository struct {
	db *sql.DB
}

func (r *TimelapseRepository) Create(ctx context.Context, t *model.Timelapse) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO timelapses (id, printer_id, camera_id, print_job_id, status, frames_path, video_path, frame_count, started_at, completed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.PrinterID, t.CameraID, t.PrintJobID, t.Status, t.FramesPath, t.VideoPath, t.FrameCount, t.StartedAt, t.CompletedAt, t.CreatedAt, t.UpdatedAt)
	return err
}

func (r *TimelapseRepository) List(ctx context.Context, printerID *uuid.UUID) ([]model.Timelapse, error) {
	query := `SELECT id, printer_id, camera_id, print_job_id, status, frames_path, video_path, frame_count, started_at, completed_at, created_at, updated_at FROM timelapses`
	args := []interface{}{}
	if printerID != nil {
		query += ` WHERE printer_id = ?`
		args = append(args, *printerID)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []model.Timelapse
	for rows.Next() {
		var t model.Timelapse
		if err := scanRow(rows, &t.ID, &t.PrinterID, &t.CameraID, &t.PrintJobID, &t.Status, &t.FramesPath, &t.VideoPath, &t.FrameCount, &t.StartedAt, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}
