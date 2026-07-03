package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type PrintJobRepository struct {
	db *sql.DB
}

// Create inserts a new print job and records the initial "queued" event.
func (r *PrintJobRepository) Create(ctx context.Context, j *model.PrintJob) error {
	j.ID = uuid.New()
	j.CreatedAt = time.Now()
	if j.AttemptNumber == 0 {
		j.AttemptNumber = 1
	}
	j.Status = model.PrintJobStatusQueued // Always start as queued
	j.AutoDispatchEnabled = true          // Default to enabled

	outcomeJSON, _ := json.Marshal(j.Outcome)
	snapshotJSON, _ := json.Marshal(j.MaterialSnapshot)

	// Insert the job record
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO print_jobs (id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at, recipe_id, attempt_number, parent_job_id, estimated_seconds, material_snapshot, priority, auto_dispatch_enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, j.ID, j.DesignID, j.PrinterID, j.MaterialSpoolID, j.ProjectID, j.TaskID, j.Status, j.Progress, j.StartedAt, j.CompletedAt, outcomeJSON, j.Notes, j.CreatedAt, j.RecipeID, j.AttemptNumber, j.ParentJobID, j.EstimatedSeconds, snapshotJSON, j.Priority, j.AutoDispatchEnabled)
	if err != nil {
		return err
	}

	// Record the initial queued event
	status := model.PrintJobStatusQueued
	event := model.NewJobEvent(j.ID, model.JobEventQueued, &status)
	return r.AppendEvent(ctx, event)
}

// GetByID retrieves a print job by ID with current status computed from events.
func (r *PrintJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.PrintJob, error) {
	var j model.PrintJob
	var outcomeJSON, snapshotJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at,
		       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
		FROM print_jobs WHERE id = ?
	`, id), &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
		&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if outcomeJSON != nil {
		json.Unmarshal(outcomeJSON, &j.Outcome)
	}
	if snapshotJSON != nil {
		json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
	}

	// Get current status from latest event
	currentStatus, currentProgress, err := r.GetCurrentStatus(ctx, id)
	if err == nil && currentStatus != nil {
		j.Status = *currentStatus
		if currentProgress != nil {
			j.Progress = *currentProgress
		}
	}

	return &j, nil
}

// GetByIDWithEvents retrieves a print job with its full event timeline.
func (r *PrintJobRepository) GetByIDWithEvents(ctx context.Context, id uuid.UUID) (*model.PrintJob, error) {
	j, err := r.GetByID(ctx, id)
	if err != nil || j == nil {
		return j, err
	}

	events, err := r.GetEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	j.Events = events

	return j, nil
}

// List retrieves print jobs with optional filters.
func (r *PrintJobRepository) List(ctx context.Context, printerID *uuid.UUID, status *model.PrintJobStatus) ([]model.PrintJob, error) {
	query := `SELECT pj.id, pj.design_id, pj.printer_id, pj.material_spool_id, pj.project_id, pj.task_id, pj.status, pj.progress, pj.started_at, pj.completed_at, pj.outcome, pj.notes, pj.created_at,
	                 pj.recipe_id, pj.attempt_number, pj.parent_job_id, pj.failure_category, pj.estimated_seconds, pj.actual_seconds, pj.material_used_grams, pj.cost_cents, pj.printer_time_cost_cents, pj.material_cost_cents, pj.material_snapshot, pj.priority, pj.auto_dispatch_enabled
	          FROM print_jobs pj WHERE 1=1`
	args := []interface{}{}

	if printerID != nil {
		query += " AND pj.printer_id = ?"
		args = append(args, *printerID)
	}
	if status != nil {
		query += " AND pj.status = ?"
		args = append(args, *status)
	}
	query += ` ORDER BY pj.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListCompletedSince retrieves completed print jobs that were completed on or after the given time.
func (r *PrintJobRepository) ListCompletedSince(ctx context.Context, since time.Time) ([]model.PrintJob, error) {
	query := `SELECT pj.id, pj.design_id, pj.printer_id, pj.material_spool_id, pj.project_id, pj.task_id, pj.status, pj.progress, pj.started_at, pj.completed_at, pj.outcome, pj.notes, pj.created_at,
	                 pj.recipe_id, pj.attempt_number, pj.parent_job_id, pj.failure_category, pj.estimated_seconds, pj.actual_seconds, pj.material_used_grams, pj.cost_cents, pj.printer_time_cost_cents, pj.material_cost_cents, pj.material_snapshot, pj.priority, pj.auto_dispatch_enabled
	          FROM print_jobs pj WHERE pj.status = ? AND pj.completed_at >= ? ORDER BY pj.created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, model.PrintJobStatusCompleted, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListByDesign retrieves all print jobs for a design.
func (r *PrintJobRepository) ListByDesign(ctx context.Context, designID uuid.UUID) ([]model.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at,
		       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
		FROM print_jobs WHERE design_id = ? ORDER BY created_at DESC
	`, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// Update updates denormalized fields on a print job.
// NOTE: Status changes should go through AppendEvent, not Update.
// This method is for updating computed/summary fields only.
func (r *PrintJobRepository) Update(ctx context.Context, j *model.PrintJob) error {
	outcomeJSON, _ := json.Marshal(j.Outcome)
	snapshotJSON, _ := json.Marshal(j.MaterialSnapshot)

	_, err := r.db.ExecContext(ctx, `
		UPDATE print_jobs SET printer_id = ?, material_spool_id = ?, status = ?, progress = ?, started_at = ?, completed_at = ?, outcome = ?, notes = ?,
		       failure_category = ?, actual_seconds = ?, material_used_grams = ?, cost_cents = ?, printer_time_cost_cents = ?, material_cost_cents = ?, material_snapshot = ?,
		       priority = ?, auto_dispatch_enabled = ?
		WHERE id = ?
	`, j.PrinterID, j.MaterialSpoolID, j.Status, j.Progress, j.StartedAt, j.CompletedAt, outcomeJSON, j.Notes,
		j.FailureCategory, j.ActualSeconds, j.MaterialUsedGrams, j.CostCents, j.PrinterTimeCostCents, j.MaterialCostCents, snapshotJSON,
		j.Priority, j.AutoDispatchEnabled, j.ID)
	return err
}

// AppendEvent records a new event for a job. Events are immutable once created.
func (r *PrintJobRepository) AppendEvent(ctx context.Context, e *model.JobEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = e.CreatedAt
	}

	metadataJSON, _ := json.Marshal(e.Metadata)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO job_events (id, job_id, event_type, occurred_at, status, progress, printer_id, error_code, error_message, actor_type, actor_id, metadata, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, e.ID, e.JobID, e.EventType, e.OccurredAt, e.Status, e.Progress, e.PrinterID, e.ErrorCode, e.ErrorMessage, e.ActorType, e.ActorID, metadataJSON, e.CreatedAt)

	if err != nil {
		return err
	}

	// Update denormalized status on print_jobs table for query efficiency
	if e.Status != nil {
		_, err = r.db.ExecContext(ctx, `UPDATE print_jobs SET status = ?, progress = COALESCE(?, progress) WHERE id = ?`, *e.Status, e.Progress, e.JobID)
	} else if e.Progress != nil {
		_, err = r.db.ExecContext(ctx, `UPDATE print_jobs SET progress = ? WHERE id = ?`, *e.Progress, e.JobID)
	}

	return err
}

// GetEvents retrieves all events for a job in chronological order.
func (r *PrintJobRepository) GetEvents(ctx context.Context, jobID uuid.UUID) ([]model.JobEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_id, event_type, occurred_at, status, progress, printer_id, error_code, error_message, actor_type, actor_id, metadata, created_at
		FROM job_events WHERE job_id = ? ORDER BY occurred_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.JobEvent
	for rows.Next() {
		var e model.JobEvent
		var metadataJSON []byte
		if err := scanRow(rows, &e.ID, &e.JobID, &e.EventType, &e.OccurredAt, &e.Status, &e.Progress, &e.PrinterID, &e.ErrorCode, &e.ErrorMessage, &e.ActorType, &e.ActorID, &metadataJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetCurrentStatus retrieves the current status and progress for a job from the latest event.
func (r *PrintJobRepository) GetCurrentStatus(ctx context.Context, jobID uuid.UUID) (*model.PrintJobStatus, *float64, error) {
	var status model.PrintJobStatus
	var progress *float64
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT status, progress FROM job_events
		WHERE job_id = ? AND status IS NOT NULL
		ORDER BY occurred_at DESC LIMIT 1
	`, jobID), &status, &progress)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &status, progress, nil
}

// GetRetryChain retrieves all jobs in a retry chain (original + all retries).
func (r *PrintJobRepository) GetRetryChain(ctx context.Context, jobID uuid.UUID) ([]model.PrintJob, error) {
	// First, find the root job (the one with no parent)
	rootID := jobID
	for {
		var parentID *uuid.UUID
		err := scanRow(r.db.QueryRowContext(ctx, `SELECT parent_job_id FROM print_jobs WHERE id = ?`, rootID), &parentID)
		if err != nil {
			return nil, err
		}
		if parentID == nil {
			break
		}
		rootID = *parentID
	}

	// Now get all jobs in the chain
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE chain AS (
			SELECT id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at,
			       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
			FROM print_jobs WHERE id = ?
			UNION ALL
			SELECT pj.id, pj.design_id, pj.printer_id, pj.material_spool_id, pj.project_id, pj.task_id, pj.status, pj.progress, pj.started_at, pj.completed_at, pj.outcome, pj.notes, pj.created_at,
			       pj.recipe_id, pj.attempt_number, pj.parent_job_id, pj.failure_category, pj.estimated_seconds, pj.actual_seconds, pj.material_used_grams, pj.cost_cents, pj.printer_time_cost_cents, pj.material_cost_cents, pj.material_snapshot, pj.priority, pj.auto_dispatch_enabled
			FROM print_jobs pj INNER JOIN chain c ON pj.parent_job_id = c.id
		)
		SELECT * FROM chain ORDER BY attempt_number ASC
	`, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListByRecipe retrieves all print jobs for a recipe/template.
func (r *PrintJobRepository) ListByRecipe(ctx context.Context, recipeID uuid.UUID) ([]model.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at,
		       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
		FROM print_jobs WHERE recipe_id = ? ORDER BY created_at DESC
	`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListByProject retrieves all print jobs for a project.
func (r *PrintJobRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at,
		       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
		FROM print_jobs WHERE project_id = ? ORDER BY created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListByTask retrieves all print jobs for a task.
func (r *PrintJobRepository) ListByTask(ctx context.Context, taskID uuid.UUID) ([]model.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, design_id, printer_id, material_spool_id, project_id, task_id, status, progress, started_at, completed_at, outcome, notes, created_at,
		       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
		FROM print_jobs WHERE task_id = ? ORDER BY created_at DESC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.TaskID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// JobStats contains job statistics for a project.
type JobStats struct {
	Total     int `json:"total"`
	Queued    int `json:"queued"`
	Assigned  int `json:"assigned"`
	Printing  int `json:"printing"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

// GetProjectJobStats retrieves job statistics for a project.
func (r *PrintJobRepository) GetProjectJobStats(ctx context.Context, projectID uuid.UUID) (*JobStats, error) {
	stats := &JobStats{}
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM print_jobs WHERE project_id = ? GROUP BY status
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status model.PrintJobStatus
		var count int
		if err := scanRow(rows, &status, &count); err != nil {
			return nil, err
		}
		stats.Total += count
		switch status {
		case model.PrintJobStatusQueued:
			stats.Queued = count
		case model.PrintJobStatusAssigned:
			stats.Assigned = count
		case model.PrintJobStatusPrinting, model.PrintJobStatusUploaded:
			stats.Printing += count
		case model.PrintJobStatusCompleted:
			stats.Completed = count
		case model.PrintJobStatusFailed:
			stats.Failed = count
		case model.PrintJobStatusCancelled:
			stats.Cancelled = count
		}
	}
	return stats, rows.Err()
}

// GetPrinterJobStats retrieves job statistics for a printer.
func (r *PrintJobRepository) GetPrinterJobStats(ctx context.Context, printerID uuid.UUID) (*JobStats, error) {
	stats := &JobStats{}
	rows, err := r.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM print_jobs WHERE printer_id = ? GROUP BY status
	`, printerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status model.PrintJobStatus
		var count int
		if err := scanRow(rows, &status, &count); err != nil {
			return nil, err
		}
		stats.Total += count
		switch status {
		case model.PrintJobStatusQueued:
			stats.Queued = count
		case model.PrintJobStatusAssigned:
			stats.Assigned = count
		case model.PrintJobStatusPrinting, model.PrintJobStatusUploaded:
			stats.Printing += count
		case model.PrintJobStatusCompleted:
			stats.Completed = count
		case model.PrintJobStatusFailed:
			stats.Failed = count
		case model.PrintJobStatusCancelled:
			stats.Cancelled = count
		}
	}
	return stats, rows.Err()
}

// ListQueued retrieves queued jobs ordered by priority DESC, created_at ASC.
// Only returns jobs with auto_dispatch_enabled = true.
func (r *PrintJobRepository) ListQueued(ctx context.Context) ([]model.PrintJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, design_id, printer_id, material_spool_id, project_id, status, progress, started_at, completed_at, outcome, notes, created_at,
		       recipe_id, attempt_number, parent_job_id, failure_category, estimated_seconds, actual_seconds, material_used_grams, cost_cents, printer_time_cost_cents, material_cost_cents, material_snapshot, priority, auto_dispatch_enabled
		FROM print_jobs
		WHERE status = 'queued' AND auto_dispatch_enabled = 1
		ORDER BY priority DESC, created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.PrintJob
	for rows.Next() {
		var j model.PrintJob
		var outcomeJSON, snapshotJSON []byte
		if err := scanRow(rows, &j.ID, &j.DesignID, &j.PrinterID, &j.MaterialSpoolID, &j.ProjectID, &j.Status, &j.Progress, &j.StartedAt, &j.CompletedAt, &outcomeJSON, &j.Notes, &j.CreatedAt,
			&j.RecipeID, &j.AttemptNumber, &j.ParentJobID, &j.FailureCategory, &j.EstimatedSeconds, &j.ActualSeconds, &j.MaterialUsedGrams, &j.CostCents, &j.PrinterTimeCostCents, &j.MaterialCostCents, &snapshotJSON, &j.Priority, &j.AutoDispatchEnabled); err != nil {
			return nil, err
		}
		if outcomeJSON != nil {
			json.Unmarshal(outcomeJSON, &j.Outcome)
		}
		if snapshotJSON != nil {
			json.Unmarshal(snapshotJSON, &j.MaterialSnapshot)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdatePriority updates a job's priority.
func (r *PrintJobRepository) UpdatePriority(ctx context.Context, id uuid.UUID, priority int) error {
	_, err := r.db.ExecContext(ctx, `UPDATE print_jobs SET priority = ? WHERE id = ?`, priority, id)
	return err
}

func (r *PrintJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE print_jobs SET parent_job_id = NULL WHERE parent_job_id = ?`, id); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM print_jobs WHERE id = ?`, id)
	return err
}

func (r *PrintJobRepository) DeleteByProject(ctx context.Context, projectID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM print_jobs WHERE project_id = ?`, projectID)
	return err
}

func (r *PrintJobRepository) DeleteByPrinter(ctx context.Context, printerID uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM queue_items WHERE source_type = 'print_job' AND source_id IN (SELECT id FROM print_jobs WHERE printer_id = ?)`, printerID); err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM queue_items WHERE assigned_printer_id = ? AND source_type != 'print_job' AND status IN ('done', 'failed', 'cancelled')`, printerID); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM print_jobs WHERE printer_id = ?`, printerID)
	return err
}

// FileRepository handles file metadata database operations.
