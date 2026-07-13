package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type QueueItemRepository struct {
	db *sql.DB
}

func (r *QueueItemRepository) Create(ctx context.Context, item *model.QueueItem) error {
	item.ID = uuid.New()
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt
	if item.SourceType == "" {
		item.SourceType = model.QueueSourceUpload
	}
	if item.Status == "" {
		item.Status = model.QueueItemStatusQueued
	}
	metadata := marshalQueueMetadata(item.Metadata)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO queue_items (
			id, source_type, source_id, project_id, file_id, file_name, display_name, status, priority, progress, wasted_grams, failed_attempts,
			assigned_printer_id, assigned_spool_id, material_type, material_color, filament_name, filament_grams,
			estimated_seconds, layer_height, nozzle_diameter, bed_temp, nozzle_temp,
			thumbnail_file_id, metadata_json, notes, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.SourceType, item.SourceID, item.ProjectID, item.FileID, item.FileName, item.DisplayName, item.Status, item.Priority, item.Progress, item.WastedGrams, item.FailedAttempts,
		item.AssignedPrinterID, item.AssignedSpoolID, item.MaterialType, item.MaterialColor, item.FilamentName, item.FilamentGrams,
		item.EstimatedSeconds, item.LayerHeight, item.NozzleDiameter, item.BedTemp, item.NozzleTemp,
		item.ThumbnailFileID, metadata, item.Notes, item.CreatedAt, item.UpdatedAt)
	return err
}

func (r *QueueItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.QueueItem, error) {
	var item model.QueueItem
	var metadata sql.NullString
	err := scanRow(r.db.QueryRowContext(ctx, queueItemSelect()+` WHERE id = ?`, id), scanQueueItem(&item, &metadata)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.Metadata = unmarshalQueueMetadata(metadata)
	return &item, nil
}

func (r *QueueItemRepository) List(ctx context.Context) ([]model.QueueItem, error) {
	rows, err := r.db.QueryContext(ctx, queueItemSelect()+` ORDER BY priority DESC, created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.QueueItem{}
	for rows.Next() {
		var item model.QueueItem
		var metadata sql.NullString
		if err := scanRow(rows, scanQueueItem(&item, &metadata)...); err != nil {
			return nil, err
		}
		item.Metadata = unmarshalQueueMetadata(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *QueueItemRepository) GetActiveByPrinter(ctx context.Context, printerID, excludeID uuid.UUID) (*model.QueueItem, error) {
	var item model.QueueItem
	var metadata sql.NullString
	err := scanRow(r.db.QueryRowContext(ctx, queueItemSelect()+`
		WHERE assigned_printer_id = ? AND id != ? AND status IN ('printing', 'paused')
		ORDER BY updated_at DESC LIMIT 1`, printerID, excludeID), scanQueueItem(&item, &metadata)...)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	item.Metadata = unmarshalQueueMetadata(metadata)
	return &item, nil
}

func (r *QueueItemRepository) ListTerminalByPrinter(ctx context.Context, printerID uuid.UUID) ([]model.QueueItem, error) {
	rows, err := r.db.QueryContext(ctx, queueItemSelect()+` WHERE assigned_printer_id = ? AND source_type != 'print_job' AND status IN ('done', 'failed', 'cancelled') ORDER BY updated_at DESC`, printerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQueueItems(rows)
}

func (r *QueueItemRepository) ListTerminalDirect(ctx context.Context) ([]model.QueueItem, error) {
	rows, err := r.db.QueryContext(ctx, queueItemSelect()+` WHERE source_type != 'print_job' AND (status IN ('done', 'failed', 'cancelled') OR COALESCE(wasted_grams, 0) > 0) ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQueueItems(rows)
}

func scanQueueItems(rows *sql.Rows) ([]model.QueueItem, error) {
	items := []model.QueueItem{}
	for rows.Next() {
		var item model.QueueItem
		var metadata sql.NullString
		if err := scanRow(rows, scanQueueItem(&item, &metadata)...); err != nil {
			return nil, err
		}
		item.Metadata = unmarshalQueueMetadata(metadata)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *QueueItemRepository) Update(ctx context.Context, item *model.QueueItem) error {
	item.UpdatedAt = time.Now()
	metadata := marshalQueueMetadata(item.Metadata)
	_, err := r.db.ExecContext(ctx, `
		UPDATE queue_items SET
			source_type = ?, source_id = ?, project_id = ?, file_id = ?, file_name = ?, display_name = ?, status = ?, priority = ?, progress = ?, wasted_grams = ?, failed_attempts = ?,
			assigned_printer_id = ?, assigned_spool_id = ?, material_type = ?, material_color = ?, filament_name = ?, filament_grams = ?,
			estimated_seconds = ?, layer_height = ?, nozzle_diameter = ?, bed_temp = ?, nozzle_temp = ?,
			thumbnail_file_id = ?, metadata_json = ?, notes = ?, updated_at = ?
		WHERE id = ?
	`, item.SourceType, item.SourceID, item.ProjectID, item.FileID, item.FileName, item.DisplayName, item.Status, item.Priority, item.Progress, item.WastedGrams, item.FailedAttempts,
		item.AssignedPrinterID, item.AssignedSpoolID, item.MaterialType, item.MaterialColor, item.FilamentName, item.FilamentGrams,
		item.EstimatedSeconds, item.LayerHeight, item.NozzleDiameter, item.BedTemp, item.NozzleTemp,
		item.ThumbnailFileID, metadata, item.Notes, item.UpdatedAt, item.ID)
	return err
}

func (r *QueueItemRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM queue_items WHERE id = ?`, id)
	return err
}

func (r *QueueItemRepository) DeleteBySourcePrintJob(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM queue_items WHERE source_type = 'print_job' AND source_id = ?`, jobID.String())
	return err
}

func (r *QueueItemRepository) DeleteByProjectPrintJobs(ctx context.Context, projectID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM queue_items 
		WHERE source_type = 'print_job' 
		AND source_id IN (SELECT id FROM print_jobs WHERE project_id = ?)
	`, projectID.String())
	return err
}

func (r *QueueItemRepository) UpdatePriority(ctx context.Context, id uuid.UUID, priority int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE queue_items SET priority = ?, updated_at = ? WHERE id = ?`, priority, time.Now(), id)
	return err
}

func queueItemSelect() string {
	return `SELECT id, source_type, source_id, project_id, file_id, file_name, display_name, status, priority, progress, COALESCE(wasted_grams, 0), COALESCE(failed_attempts, 0),
		assigned_printer_id, assigned_spool_id, material_type, material_color, COALESCE(filament_name, ''), filament_grams,
		estimated_seconds, layer_height, nozzle_diameter, bed_temp, nozzle_temp,
		thumbnail_file_id, metadata_json, notes, created_at, updated_at FROM queue_items`
}

func scanQueueItem(item *model.QueueItem, metadata *sql.NullString) []any {
	return []any{
		&item.ID, &item.SourceType, &item.SourceID, &item.ProjectID, &item.FileID, &item.FileName, &item.DisplayName, &item.Status, &item.Priority, &item.Progress, &item.WastedGrams, &item.FailedAttempts,
		&item.AssignedPrinterID, &item.AssignedSpoolID, &item.MaterialType, &item.MaterialColor, &item.FilamentName, &item.FilamentGrams,
		&item.EstimatedSeconds, &item.LayerHeight, &item.NozzleDiameter, &item.BedTemp, &item.NozzleTemp,
		&item.ThumbnailFileID, metadata, &item.Notes, &item.CreatedAt, &item.UpdatedAt,
	}
}

func marshalQueueMetadata(metadata *model.GCodeMetadata) string {
	if metadata == nil {
		return "{}"
	}
	b, _ := json.Marshal(metadata)
	return string(b)
}

func unmarshalQueueMetadata(data sql.NullString) *model.GCodeMetadata {
	if !data.Valid || data.String == "" {
		return nil
	}
	var metadata model.GCodeMetadata
	if err := json.Unmarshal([]byte(data.String), &metadata); err != nil {
		return nil
	}
	return &metadata
}
