package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type PrinterRepository struct {
	db *sql.DB
}

// Create inserts a new printer.
func (r *PrinterRepository) Create(ctx context.Context, p *model.Printer) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	if p.Status == "" {
		p.Status = model.PrinterStatusOffline
	}
	if p.MinMaterialPercent == 0 {
		p.MinMaterialPercent = 10 // Default 10%
	}
	if !p.RestrictGCodeModel {
		p.RestrictGCodeModel = true
	}

	buildVolumeJSON, _ := json.Marshal(p.BuildVolume)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO printers (id, name, model, manufacturer, connection_type, connection_uri, fluidd_url, api_key, serial_number, status, build_volume, nozzle_diameter, location, notes, min_material_percent, cost_per_hour_cents, purchase_price_cents, maintenance_mode, restrict_gcode_model, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.Model, p.Manufacturer, p.ConnectionType, p.ConnectionURI, p.FluiddURL, p.APIKey, p.SerialNumber, p.Status, buildVolumeJSON, p.NozzleDiameter, p.Location, p.Notes, p.MinMaterialPercent, p.CostPerHourCents, p.PurchasePriceCents, p.MaintenanceMode, p.RestrictGCodeModel, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetByID retrieves a printer by ID.
func (r *PrinterRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Printer, error) {
	var p model.Printer
	var buildVolumeJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, model, manufacturer, connection_type, connection_uri, fluidd_url, api_key, serial_number, status, build_volume, nozzle_diameter, location, notes, min_material_percent, cost_per_hour_cents, purchase_price_cents, maintenance_mode, restrict_gcode_model, created_at, updated_at
		FROM printers WHERE id = ?
	`, id), &p.ID, &p.Name, &p.Model, &p.Manufacturer, &p.ConnectionType, &p.ConnectionURI, &p.FluiddURL, &p.APIKey, &p.SerialNumber, &p.Status, &buildVolumeJSON, &p.NozzleDiameter, &p.Location, &p.Notes, &p.MinMaterialPercent, &p.CostPerHourCents, &p.PurchasePriceCents, &p.MaintenanceMode, &p.RestrictGCodeModel, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if buildVolumeJSON != nil {
		json.Unmarshal(buildVolumeJSON, &p.BuildVolume)
	}
	return &p, nil
}

// List retrieves all printers.
func (r *PrinterRepository) List(ctx context.Context) ([]model.Printer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, model, manufacturer, connection_type, connection_uri, fluidd_url, api_key, serial_number, status, build_volume, nozzle_diameter, location, notes, min_material_percent, cost_per_hour_cents, purchase_price_cents, maintenance_mode, restrict_gcode_model, created_at, updated_at
		FROM printers ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var printers []model.Printer
	for rows.Next() {
		var p model.Printer
		var buildVolumeJSON []byte
		if err := scanRow(rows, &p.ID, &p.Name, &p.Model, &p.Manufacturer, &p.ConnectionType, &p.ConnectionURI, &p.FluiddURL, &p.APIKey, &p.SerialNumber, &p.Status, &buildVolumeJSON, &p.NozzleDiameter, &p.Location, &p.Notes, &p.MinMaterialPercent, &p.CostPerHourCents, &p.PurchasePriceCents, &p.MaintenanceMode, &p.RestrictGCodeModel, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if buildVolumeJSON != nil {
			json.Unmarshal(buildVolumeJSON, &p.BuildVolume)
		}
		printers = append(printers, p)
	}
	return printers, rows.Err()
}

// Update updates a printer.
func (r *PrinterRepository) Update(ctx context.Context, p *model.Printer) error {
	p.UpdatedAt = time.Now()
	buildVolumeJSON, _ := json.Marshal(p.BuildVolume)

	_, err := r.db.ExecContext(ctx, `
		UPDATE printers SET name = ?, model = ?, manufacturer = ?, connection_type = ?, connection_uri = ?, fluidd_url = ?, api_key = ?, serial_number = ?, status = ?, build_volume = ?, nozzle_diameter = ?, location = ?, notes = ?, min_material_percent = ?, cost_per_hour_cents = ?, purchase_price_cents = ?, maintenance_mode = ?, restrict_gcode_model = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Model, p.Manufacturer, p.ConnectionType, p.ConnectionURI, p.FluiddURL, p.APIKey, p.SerialNumber, p.Status, buildVolumeJSON, p.NozzleDiameter, p.Location, p.Notes, p.MinMaterialPercent, p.CostPerHourCents, p.PurchasePriceCents, p.MaintenanceMode, p.RestrictGCodeModel, p.UpdatedAt, p.ID)
	return err
}

// UpdateStatus updates only the printer status.
func (r *PrinterRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.PrinterStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE printers SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

// Delete removes a printer.
func (r *PrinterRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM printers WHERE id = ?`, id)
	return err
}

// PrinterUtilizationData holds raw data for computing utilization metrics.
type PrinterUtilizationData struct {
	CompletedSeconds int
	FailedSeconds    int
	CompletedJobs    int
	FailedJobs       int
	TotalJobs        int
}

// GetPrinterUtilizationData retrieves utilization data for a printer since a given time.
func (r *PrinterRepository) GetPrinterUtilizationData(ctx context.Context, printerID uuid.UUID, since time.Time) (*PrinterUtilizationData, error) {
	var data PrinterUtilizationData
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(completed_seconds), 0),
			COALESCE(SUM(failed_seconds), 0),
			COALESCE(SUM(completed_jobs), 0),
			COALESCE(SUM(failed_jobs), 0),
			COALESCE(SUM(total_jobs), 0)
		FROM (
			SELECT
				CASE WHEN status = 'completed' AND COALESCE(json_extract(outcome, '$.success'), 1) != 0 THEN COALESCE(actual_seconds, CAST(strftime('%s', completed_at) - strftime('%s', started_at) AS INTEGER), estimated_seconds, 0) ELSE 0 END AS completed_seconds,
				CASE WHEN status = 'failed' OR COALESCE(json_extract(outcome, '$.success'), 1) = 0 THEN COALESCE(actual_seconds, CAST(strftime('%s', completed_at) - strftime('%s', started_at) AS INTEGER), estimated_seconds, 0) ELSE 0 END AS failed_seconds,
				CASE WHEN status = 'completed' AND COALESCE(json_extract(outcome, '$.success'), 1) != 0 THEN 1 ELSE 0 END AS completed_jobs,
				CASE WHEN status = 'failed' OR COALESCE(json_extract(outcome, '$.success'), 1) = 0 THEN 1 ELSE 0 END AS failed_jobs,
				1 AS total_jobs
			FROM print_jobs
			WHERE printer_id = ? AND created_at >= ?
			UNION ALL
			SELECT
				CASE WHEN status = 'done' THEN COALESCE(estimated_seconds, CAST(strftime('%s', updated_at) - strftime('%s', created_at) AS INTEGER), 0) ELSE 0 END,
				CASE WHEN status IN ('failed', 'cancelled') THEN COALESCE(estimated_seconds, CAST(strftime('%s', updated_at) - strftime('%s', created_at) AS INTEGER), 0) ELSE 0 END,
				CASE WHEN status = 'done' THEN 1 ELSE 0 END,
				CASE WHEN status IN ('failed', 'cancelled') THEN 1 ELSE 0 END,
				1
			FROM queue_items
			WHERE assigned_printer_id = ? AND created_at >= ? AND source_type != 'print_job' AND status IN ('done', 'failed', 'cancelled')
		)
	`, printerID, since, printerID, since).Scan(&data.CompletedSeconds, &data.FailedSeconds, &data.CompletedJobs, &data.FailedJobs, &data.TotalJobs)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// PrinterHealthData holds raw data for computing health metrics.
type PrinterHealthData struct {
	TotalJobs          int
	CompletedJobs      int
	FailedJobs         int
	TotalSeconds       int
	TotalCostCents     int
	TotalMaterialGrams float64
}

// GetPrinterHealthData retrieves lifetime health data for a printer.
func (r *PrinterRepository) GetPrinterHealthData(ctx context.Context, printerID uuid.UUID) (*PrinterHealthData, error) {
	var data PrinterHealthData
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(total_jobs), 0),
			COALESCE(SUM(completed_jobs), 0),
			COALESCE(SUM(failed_jobs), 0),
			COALESCE(SUM(total_seconds), 0),
			COALESCE(SUM(cost_cents), 0),
			COALESCE(SUM(material_grams), 0)
		FROM (
			SELECT
				1 AS total_jobs,
				CASE WHEN status = 'completed' AND COALESCE(json_extract(outcome, '$.success'), 1) != 0 THEN 1 ELSE 0 END AS completed_jobs,
				CASE WHEN status = 'failed' OR COALESCE(json_extract(outcome, '$.success'), 1) = 0 THEN 1 ELSE 0 END AS failed_jobs,
				CASE WHEN status = 'completed' AND COALESCE(json_extract(outcome, '$.success'), 1) != 0 THEN COALESCE(actual_seconds, CAST(strftime('%s', completed_at) - strftime('%s', started_at) AS INTEGER), estimated_seconds, 0) ELSE 0 END AS total_seconds,
				COALESCE(cost_cents, 0) AS cost_cents,
				COALESCE(material_used_grams, 0) AS material_grams
			FROM print_jobs
			WHERE printer_id = ?
			UNION ALL
			SELECT
				1,
				CASE WHEN status = 'done' THEN 1 ELSE 0 END,
				CASE WHEN status IN ('failed', 'cancelled') THEN 1 ELSE 0 END,
				CASE WHEN status = 'done' THEN COALESCE(estimated_seconds, CAST(strftime('%s', updated_at) - strftime('%s', created_at) AS INTEGER), 0) ELSE 0 END,
				0,
				COALESCE(filament_grams, 0)
			FROM queue_items
			WHERE assigned_printer_id = ? AND source_type != 'print_job' AND status IN ('done', 'failed', 'cancelled')
		)
	`, printerID, printerID).Scan(&data.TotalJobs, &data.CompletedJobs, &data.FailedJobs, &data.TotalSeconds, &data.TotalCostCents, &data.TotalMaterialGrams)
	if err != nil {
		return nil, err
	}
	return &data, nil
}

// GetPrinterFailureBreakdown retrieves failure category counts for a printer.
func (r *PrinterRepository) GetPrinterFailureBreakdown(ctx context.Context, printerID uuid.UUID) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(failure_category, 'unknown'), COUNT(*)
		FROM print_jobs
		WHERE printer_id = ? AND (status = 'failed' OR COALESCE(json_extract(outcome, '$.success'), 1) = 0)
		GROUP BY failure_category
	`, printerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		breakdown[category] = count
	}
	return breakdown, rows.Err()
}

// GetPrinterRevenueAttribution computes attributed revenue for a printer.
// This traces: Printer → Jobs → Projects → Sales with proportional attribution.
func (r *PrinterRepository) GetPrinterRevenueAttribution(ctx context.Context, printerID uuid.UUID) (int, error) {
	var revenueCents int
	err := r.db.QueryRowContext(ctx, `
		WITH printer_project_jobs AS (
			SELECT project_id, COUNT(*) as printer_jobs
			FROM print_jobs
			WHERE printer_id = ? AND project_id IS NOT NULL AND status = 'completed'
			GROUP BY project_id
		),
		project_total_jobs AS (
			SELECT project_id, COUNT(*) as total_jobs
			FROM print_jobs
			WHERE project_id IS NOT NULL AND status = 'completed'
			GROUP BY project_id
		),
		project_sales AS (
			SELECT project_id, SUM(gross_cents) as gross
			FROM sales
			WHERE project_id IS NOT NULL
			GROUP BY project_id
		)
		SELECT COALESCE(SUM(
			CAST(ps.gross AS REAL) * CAST(ppj.printer_jobs AS REAL) / CAST(ptj.total_jobs AS REAL)
		), 0)
		FROM printer_project_jobs ppj
		JOIN project_total_jobs ptj ON ppj.project_id = ptj.project_id
		JOIN project_sales ps ON ppj.project_id = ps.project_id
	`, printerID).Scan(&revenueCents)
	if err != nil {
		return 0, err
	}
	return revenueCents, nil
}

// MaterialRepository handles material database operations.
