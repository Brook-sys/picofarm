package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

type PrintJobService struct {
	repo           *repository.PrintJobRepository
	printerRepo    *repository.PrinterRepository
	designRepo     *repository.DesignRepository
	spoolRepo      *repository.SpoolRepository
	materialRepo   *repository.MaterialRepository
	projectRepo    *repository.ProjectRepository
	queueRepo      *repository.QueueItemRepository
	printerMgr     *printer.Manager
	hub            *realtime.Hub
	storage        storage.Storage
	onJobCompleted func(ctx context.Context, job *model.PrintJob)
}

// SetOnJobCompleted sets a callback invoked when a print job completes successfully.
func (s *PrintJobService) SetOnJobCompleted(fn func(ctx context.Context, job *model.PrintJob)) {
	s.onJobCompleted = fn
}

// Create creates a new print job and records the initial "queued" event.
// Printer and spool can be nil - job will be queued pending assignment.
func (s *PrintJobService) Create(ctx context.Context, j *model.PrintJob) error {
	if j.DesignID == uuid.Nil {
		return fmt.Errorf("design ID is required")
	}
	// Printer and spool are optional - job can be created without assignment
	// The repository.Create already records the initial queued event
	return s.repo.Create(ctx, j)
}

// GetByID retrieves a print job by ID.
func (s *PrintJobService) GetByID(ctx context.Context, id uuid.UUID) (*model.PrintJob, error) {
	return s.repo.GetByID(ctx, id)
}

// GetByIDWithEvents retrieves a print job with its full event timeline.
func (s *PrintJobService) GetByIDWithEvents(ctx context.Context, id uuid.UUID) (*model.PrintJob, error) {
	return s.repo.GetByIDWithEvents(ctx, id)
}

// List retrieves print jobs.
func (s *PrintJobService) List(ctx context.Context, printerID *uuid.UUID, status *model.PrintJobStatus) ([]model.PrintJob, error) {
	return s.repo.List(ctx, printerID, status)
}

// ListByDesign retrieves print jobs for a design.
func (s *PrintJobService) ListByDesign(ctx context.Context, designID uuid.UUID) ([]model.PrintJob, error) {
	return s.repo.ListByDesign(ctx, designID)
}

// ListByRecipe retrieves print jobs for a recipe/template.
func (s *PrintJobService) ListByRecipe(ctx context.Context, recipeID uuid.UUID) ([]model.PrintJob, error) {
	return s.repo.ListByRecipe(ctx, recipeID)
}

// GetEvents retrieves all events for a job in chronological order.
func (s *PrintJobService) GetEvents(ctx context.Context, jobID uuid.UUID) ([]model.JobEvent, error) {
	return s.repo.GetEvents(ctx, jobID)
}

// GetRetryChain retrieves all jobs in a retry chain (original + all retries).
func (s *PrintJobService) GetRetryChain(ctx context.Context, jobID uuid.UUID) ([]model.PrintJob, error) {
	return s.repo.GetRetryChain(ctx, jobID)
}

// Start sends a print job to the printer and records the appropriate events.
func (s *PrintJobService) Start(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	// Verify job is in a startable state
	if job.Status.IsTerminal() {
		return fmt.Errorf("cannot start job in %s status", job.Status)
	}

	// Verify resources are assigned before starting
	if job.NeedsAssignment() {
		return fmt.Errorf("job needs printer and spool assignment before starting")
	}

	// Get design and file
	design, err := s.designRepo.GetByID(ctx, job.DesignID)
	if err != nil {
		return err
	}

	// Get printer
	printerData, err := s.printerRepo.GetByID(ctx, *job.PrinterID)
	if err != nil {
		return err
	}
	if printerData == nil {
		return fmt.Errorf("printer not found")
	}

	// Get printer state and validate material (if AMS available)
	printerState, _ := s.printerMgr.GetState(*job.PrinterID)
	if printerState != nil && printerState.AMS != nil {
		validation := s.ValidateMaterial(printerState.AMS, printerData.MinMaterialPercent)
		if len(validation.Errors) > 0 {
			return fmt.Errorf("material validation failed: %s", validation.Errors[0])
		}

		// Capture material snapshot before starting
		job.MaterialSnapshot = s.captureMaterialSnapshot(printerState.AMS)
	}

	// Record assignment event if printer wasn't already assigned
	if job.Status == model.PrintJobStatusQueued {
		assignedStatus := model.PrintJobStatusAssigned
		assignEvent := model.NewJobEvent(job.ID, model.JobEventAssigned, &assignedStatus).
			WithPrinter(*job.PrinterID).
			WithActor(model.ActorSystem, "print_service")
		if err := s.repo.AppendEvent(ctx, assignEvent); err != nil {
			return fmt.Errorf("failed to record assignment: %w", err)
		}
	}

	// Send to printer via manager
	if err := s.printerMgr.StartJob(*job.PrinterID, design.FileName, s.storage.GetFullPath(design.FileName)); err != nil {
		// Record failure event
		failedStatus := model.PrintJobStatusFailed
		failEvent := model.NewJobEvent(job.ID, model.JobEventFailed, &failedStatus).
			WithError("UPLOAD_FAILED", err.Error()).
			WithActor(model.ActorSystem, "print_service")
		s.repo.AppendEvent(ctx, failEvent)
		return fmt.Errorf("failed to start print: %w", err)
	}

	// Record uploaded event
	uploadedStatus := model.PrintJobStatusUploaded
	uploadEvent := model.NewJobEvent(job.ID, model.JobEventUploaded, &uploadedStatus).
		WithPrinter(*job.PrinterID).
		WithActor(model.ActorSystem, "print_service")
	if err := s.repo.AppendEvent(ctx, uploadEvent); err != nil {
		return fmt.Errorf("failed to record upload: %w", err)
	}

	// Update started_at timestamp and material snapshot
	now := time.Now()
	job.StartedAt = &now
	s.repo.Update(ctx, job) //nolint:errcheck // best-effort timestamp update

	// Broadcast update
	s.hub.Broadcast(realtime.Event{
		Type: "job_started",
		Data: job,
	})

	return nil
}

// ValidateMaterial validates the current AMS material state against thresholds.
// Returns warnings for low material and errors that should block job start.
func (s *PrintJobService) ValidateMaterial(ams *model.AMSState, minPercent int) *model.MaterialValidation {
	result := &model.MaterialValidation{Valid: true}

	if ams == nil {
		return result
	}

	// Find the currently selected tray
	var currentTray *model.AMSTray
	if ams.CurrentTray == "255" && ams.ExternalSpool != nil {
		currentTray = ams.ExternalSpool
	} else if ams.CurrentTray != "" {
		// Parse tray number (format: "X" where X is 0-15 for AMS trays)
		for _, unit := range ams.Units {
			for i := range unit.Trays {
				tray := &unit.Trays[i]
				// Calculate global tray ID: unit_id * 4 + tray_id
				globalID := unit.ID*4 + tray.ID
				if fmt.Sprintf("%d", globalID) == ams.CurrentTray {
					currentTray = tray
					break
				}
			}
			if currentTray != nil {
				break
			}
		}
	}

	if currentTray == nil {
		// No tray selected, can't validate
		return result
	}

	// Check if tray is empty
	if currentTray.Empty {
		result.Valid = false
		result.Errors = append(result.Errors, "selected tray is empty")
		return result
	}

	// Check remaining percentage against threshold
	if minPercent > 0 && currentTray.Remain < minPercent {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("material remaining (%d%%) is below minimum threshold (%d%%)", currentTray.Remain, minPercent))
	} else if currentTray.Remain < 20 {
		// Add warning for low material even if above threshold
		result.Warnings = append(result.Warnings, fmt.Sprintf("material low: %d%% remaining", currentTray.Remain))
	}

	return result
}

// captureMaterialSnapshot creates a snapshot of the current AMS state for a job.
func (s *PrintJobService) captureMaterialSnapshot(ams *model.AMSState) *model.MaterialSnapshot {
	if ams == nil {
		return nil
	}

	snapshot := &model.MaterialSnapshot{
		CapturedAt: time.Now(),
		AMSState:   ams,
	}

	// Find and record the currently selected tray info
	if ams.CurrentTray == "255" && ams.ExternalSpool != nil {
		snapshot.SelectedTray = 255
		snapshot.MaterialType = ams.ExternalSpool.MaterialType
		snapshot.Color = ams.ExternalSpool.Color
		snapshot.RemainPercent = ams.ExternalSpool.Remain
		snapshot.Brand = ams.ExternalSpool.Brand
	} else if ams.CurrentTray != "" {
		for _, unit := range ams.Units {
			for _, tray := range unit.Trays {
				globalID := unit.ID*4 + tray.ID
				if fmt.Sprintf("%d", globalID) == ams.CurrentTray {
					snapshot.SelectedTray = globalID
					snapshot.MaterialType = tray.MaterialType
					snapshot.Color = tray.Color
					snapshot.RemainPercent = tray.Remain
					snapshot.Brand = tray.Brand
					break
				}
			}
		}
	}

	return snapshot
}

// PreflightCheckResult contains the result of a preflight validation.
type PreflightCheckResult struct {
	Ready      bool                      `json:"ready"`
	Validation *model.MaterialValidation `json:"validation,omitempty"`
	AMSState   *model.AMSState           `json:"ams_state,omitempty"`
	Warnings   []string                  `json:"warnings,omitempty"`
	Errors     []string                  `json:"errors,omitempty"`
}

// PreflightCheck validates a job is ready to start.
// Returns current AMS state, validation results, and any warnings/errors.
func (s *PrintJobService) PreflightCheck(ctx context.Context, jobID uuid.UUID) (*PreflightCheckResult, error) {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job not found")
	}

	result := &PreflightCheckResult{Ready: true}

	// Check job state
	if job.Status.IsTerminal() {
		result.Ready = false
		result.Errors = append(result.Errors, fmt.Sprintf("job is in terminal state: %s", job.Status))
		return result, nil
	}

	// Check resource assignment
	if job.NeedsAssignment() {
		result.Ready = false
		result.Errors = append(result.Errors, "job needs printer and spool assignment")
		return result, nil
	}

	// Get printer
	printerData, err := s.printerRepo.GetByID(ctx, *job.PrinterID)
	if err != nil || printerData == nil {
		result.Ready = false
		result.Errors = append(result.Errors, "printer not found")
		return result, nil //nolint:nilerr // Validation result, not an error
	}

	// Get printer state including AMS
	printerState, _ := s.printerMgr.GetState(*job.PrinterID)
	if printerState == nil {
		result.Warnings = append(result.Warnings, "printer state unavailable")
		return result, nil
	}

	// Include AMS state in result
	result.AMSState = printerState.AMS

	// Validate material if AMS available
	if printerState.AMS != nil {
		validation := s.ValidateMaterial(printerState.AMS, printerData.MinMaterialPercent)
		result.Validation = validation
		result.Warnings = append(result.Warnings, validation.Warnings...)
		result.Errors = append(result.Errors, validation.Errors...)
		if !validation.Valid {
			result.Ready = false
		}
	}

	return result, nil
}

// AssignResources assigns a printer and spool to a job.
func (s *PrintJobService) AssignResources(ctx context.Context, jobID, printerID, spoolID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.Status.IsTerminal() {
		return fmt.Errorf("cannot assign resources to job in %s status", job.Status)
	}

	// Verify printer exists
	printer, err := s.printerRepo.GetByID(ctx, printerID)
	if err != nil {
		return fmt.Errorf("failed to get printer: %w", err)
	}
	if printer == nil {
		return fmt.Errorf("printer not found")
	}

	// Verify spool exists
	spool, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return fmt.Errorf("failed to get spool: %w", err)
	}
	if spool == nil {
		return fmt.Errorf("spool not found")
	}

	// Assign resources
	job.PrinterID = &printerID
	job.MaterialSpoolID = &spoolID

	if err := s.repo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	// Record assignment event
	assignedStatus := model.PrintJobStatusAssigned
	event := model.NewJobEvent(job.ID, model.JobEventAssigned, &assignedStatus).
		WithPrinter(printerID).
		WithActor(model.ActorSystem, "resource_assignment")
	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record assignment: %w", err)
	}

	return nil
}

// ListByProject retrieves print jobs for a project.
func (s *PrintJobService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.PrintJob, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *PrintJobService) Delete(ctx context.Context, id uuid.UUID) error {
	if s.queueRepo != nil {
		_ = s.queueRepo.DeleteBySourcePrintJob(ctx, id)
	}
	return s.repo.Delete(ctx, id)
}

func (s *PrintJobService) DeleteByProject(ctx context.Context, projectID uuid.UUID) error {
	if s.queueRepo != nil {
		_ = s.queueRepo.DeleteByProjectPrintJobs(ctx, projectID)
	}
	return s.repo.DeleteByProject(ctx, projectID)
}

func (s *PrintJobService) DeleteByPrinter(ctx context.Context, printerID uuid.UUID) error {
	return s.repo.DeleteByPrinter(ctx, printerID)
}

// RecordPrintingStarted records that the printer has begun printing (called by printer callbacks).
func (s *PrintJobService) RecordPrintingStarted(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	printingStatus := model.PrintJobStatusPrinting
	event := model.NewJobEvent(job.ID, model.JobEventStarted, &printingStatus).
		WithActor(model.ActorPrinter, "")
	if job.PrinterID != nil {
		event = event.WithPrinter(*job.PrinterID).
			WithActor(model.ActorPrinter, job.PrinterID.String())
	}

	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record printing started: %w", err)
	}

	// Update started_at if not already set
	if job.StartedAt == nil {
		now := time.Now()
		job.StartedAt = &now
		s.repo.Update(ctx, job) //nolint:errcheck // best-effort timestamp update
	}

	s.hub.Broadcast(realtime.Event{
		Type: "job_printing",
		Data: job,
	})

	return nil
}

// RecordProgress records a progress update for a job (called by printer callbacks).
func (s *PrintJobService) RecordProgress(ctx context.Context, id uuid.UUID, progress float64) error {
	event := model.NewJobEvent(id, model.JobEventProgress, nil).
		WithProgress(progress).
		WithActor(model.ActorPrinter, "")

	return s.repo.AppendEvent(ctx, event)
}

// Pause pauses a print job and records the event.
func (s *PrintJobService) Pause(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.Status != model.PrintJobStatusPrinting {
		return fmt.Errorf("can only pause printing jobs, current status: %s", job.Status)
	}

	if job.PrinterID == nil {
		return fmt.Errorf("job has no assigned printer")
	}

	if err := s.printerMgr.PauseJob(*job.PrinterID); err != nil {
		return err
	}

	// Record paused event
	pausedStatus := model.PrintJobStatusPaused
	event := model.NewJobEvent(job.ID, model.JobEventPaused, &pausedStatus).
		WithPrinter(*job.PrinterID).
		WithActor(model.ActorUser, "")

	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record pause: %w", err)
	}

	s.hub.Broadcast(realtime.Event{
		Type: "job_paused",
		Data: job,
	})

	return nil
}

// Resume resumes a paused print job and records the event.
func (s *PrintJobService) Resume(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.Status != model.PrintJobStatusPaused {
		return fmt.Errorf("can only resume paused jobs, current status: %s", job.Status)
	}

	if job.PrinterID == nil {
		return fmt.Errorf("job has no assigned printer")
	}

	if err := s.printerMgr.ResumeJob(*job.PrinterID); err != nil {
		return err
	}

	// Record resumed event (goes back to printing status)
	printingStatus := model.PrintJobStatusPrinting
	event := model.NewJobEvent(job.ID, model.JobEventResumed, &printingStatus).
		WithPrinter(*job.PrinterID).
		WithActor(model.ActorUser, "")

	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record resume: %w", err)
	}

	s.hub.Broadcast(realtime.Event{
		Type: "job_resumed",
		Data: job,
	})

	return nil
}

// Cancel cancels a print job and records the event.
func (s *PrintJobService) Cancel(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.Status.IsTerminal() {
		return fmt.Errorf("cannot cancel job in %s status", job.Status)
	}

	// Try to cancel on printer (may fail if not connected, but we still record cancellation)
	if job.PrinterID != nil {
		s.printerMgr.CancelJob(*job.PrinterID) //nolint:errcheck // best-effort printer cancel
	}

	// Record cancelled event
	cancelledStatus := model.PrintJobStatusCancelled
	event := model.NewJobEvent(job.ID, model.JobEventCancelled, &cancelledStatus).
		WithActor(model.ActorUser, "")
	if job.PrinterID != nil {
		event = event.WithPrinter(*job.PrinterID)
	}

	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record cancellation: %w", err)
	}

	// Update completed_at
	now := time.Now()
	job.CompletedAt = &now
	failureCategory := model.FailureUserCancelled
	job.FailureCategory = &failureCategory
	s.repo.Update(ctx, job) //nolint:errcheck // best-effort cancellation update

	s.hub.Broadcast(realtime.Event{
		Type: "job_cancelled",
		Data: job,
	})

	return nil
}

// Update updates denormalized fields on a print job.
// NOTE: For status changes, use the specific methods (Start, Pause, Cancel, RecordOutcome).
func (s *PrintJobService) Update(ctx context.Context, j *model.PrintJob) error {
	return s.repo.Update(ctx, j)
}

// RecordOutcome records the outcome of a completed print job.
// It records the completion/failure event, deducts material from the spool, and calculates costs.
func (s *PrintJobService) RecordOutcome(ctx context.Context, id uuid.UUID, outcome *model.PrintOutcome) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	// Get the spool and material to calculate cost (if assigned)
	var spool *model.MaterialSpool
	var material *model.Material
	if job.MaterialSpoolID != nil {
		spool, err = s.spoolRepo.GetByID(ctx, *job.MaterialSpoolID)
		if err != nil {
			return fmt.Errorf("failed to get spool: %w", err)
		}
	}
	if spool == nil && job.MaterialSpoolID != nil {
		return fmt.Errorf("spool not found")
	}

	// Get material for cost calculation (if spool is assigned)
	if spool != nil {
		material, err = s.materialRepo.GetByID(ctx, spool.MaterialID)
		if err != nil {
			return fmt.Errorf("failed to get material: %w", err)
		}
	}

	// Calculate material cost: (grams / 1000) * cost_per_kg
	if outcome.MaterialUsed > 0 && material != nil {
		outcome.MaterialCost = (outcome.MaterialUsed / 1000.0) * material.CostPerKg
	}

	// Compute printer time cost snapshot
	var printerTimeCostCents int
	if job.PrinterID != nil {
		printerObj, _ := s.printerRepo.GetByID(ctx, *job.PrinterID)
		if printerObj != nil && printerObj.CostPerHourCents > 0 {
			var durationSeconds int
			switch {
			case outcome.ActualTime != nil && *outcome.ActualTime > 0:
				durationSeconds = *outcome.ActualTime
			case job.ActualSeconds != nil && *job.ActualSeconds > 0:
				durationSeconds = *job.ActualSeconds
			case job.StartedAt != nil:
				durationSeconds = int(time.Since(*job.StartedAt).Seconds())
			}
			if durationSeconds > 0 {
				printerTimeCostCents = (durationSeconds * printerObj.CostPerHourCents) / 3600
			}
		}
	}

	// Update spool remaining weight if material was used
	if outcome.MaterialUsed > 0 && spool != nil {
		newWeight := spool.RemainingWeight - outcome.MaterialUsed
		if newWeight < 0 {
			newWeight = 0
		}
		spool.RemainingWeight = newWeight

		// Update spool status based on remaining weight
		if spool.RemainingWeight <= 0 {
			spool.Status = model.SpoolStatusEmpty
		} else if spool.RemainingWeight < 100 {
			spool.Status = model.SpoolStatusLow
		}

		if err := s.spoolRepo.Update(ctx, spool); err != nil {
			return fmt.Errorf("failed to update spool: %w", err)
		}
	}

	// Record the completion/failure event
	now := time.Now()
	var eventType model.JobEventType
	var status model.PrintJobStatus
	var errorCode, errorMessage string

	if outcome.Success {
		eventType = model.JobEventCompleted
		status = model.PrintJobStatusCompleted
	} else {
		eventType = model.JobEventFailed
		status = model.PrintJobStatusFailed
		errorCode = "PRINT_FAILED"
		errorMessage = outcome.FailureReason
	}

	event := model.NewJobEvent(job.ID, eventType, &status).
		WithActor(model.ActorSystem, "outcome_recorder")
	if !outcome.Success {
		event = event.WithError(errorCode, errorMessage)
	}
	if job.Progress > 0 {
		event = event.WithProgress(job.Progress)
	}

	// Add outcome data to metadata
	event.Metadata = map[string]interface{}{
		"material_used_grams": outcome.MaterialUsed,
		"material_cost":       outcome.MaterialCost,
		"quality_rating":      outcome.QualityRating,
		"actual_time":         outcome.ActualTime,
	}

	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record outcome event: %w", err)
	}

	// Persist cost snapshots
	materialCostCents := int(outcome.MaterialCost * 100)
	totalCostCents := materialCostCents + printerTimeCostCents

	// Update job with outcome data
	job.CompletedAt = &now
	job.Outcome = outcome
	job.MaterialUsedGrams = &outcome.MaterialUsed
	job.CostCents = &totalCostCents
	job.PrinterTimeCostCents = &printerTimeCostCents
	job.MaterialCostCents = &materialCostCents
	if outcome.ActualTime != nil {
		job.ActualSeconds = outcome.ActualTime
	}

	if err := s.repo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	// Broadcast the completed event
	wsEventType := "job_completed"
	if !outcome.Success {
		wsEventType = "job_failed"
	}
	s.hub.Broadcast(realtime.Event{
		Type: wsEventType,
		Data: job,
	})

	// Notify task service on successful completion
	if outcome.Success && s.onJobCompleted != nil {
		s.onJobCompleted(ctx, job)
	}

	return nil
}

// RetryRequest contains parameters for retrying a failed job.
type RetryRequest struct {
	PrinterID       *uuid.UUID             // Optional: use different printer
	MaterialSpoolID *uuid.UUID             // Optional: use different spool
	FailureCategory *model.FailureCategory // Classify why the original failed
	Notes           string                 // Notes for the retry
}

// Retry creates a new job from a failed job, linking them in a retry chain.
func (s *PrintJobService) Retry(ctx context.Context, id uuid.UUID, req *RetryRequest) (*model.PrintJob, error) {
	originalJob, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get original job: %w", err)
	}
	if originalJob == nil {
		return nil, fmt.Errorf("job not found")
	}

	// Only failed jobs can be retried
	if originalJob.Status != model.PrintJobStatusFailed && originalJob.Status != model.PrintJobStatusCancelled {
		return nil, fmt.Errorf("can only retry failed or cancelled jobs, current status: %s", originalJob.Status)
	}

	// Update the original job's failure category if provided
	if req != nil && req.FailureCategory != nil {
		originalJob.FailureCategory = req.FailureCategory
		s.repo.Update(ctx, originalJob) //nolint:errcheck // best-effort failure category update
	}

	// Create new job as retry
	newJob := &model.PrintJob{
		DesignID:         originalJob.DesignID,
		PrinterID:        originalJob.PrinterID,
		MaterialSpoolID:  originalJob.MaterialSpoolID,
		ProjectID:        originalJob.ProjectID,
		TaskID:           originalJob.TaskID,
		Notes:            originalJob.Notes,
		RecipeID:         originalJob.RecipeID,
		EstimatedSeconds: originalJob.EstimatedSeconds,
		AttemptNumber:    originalJob.AttemptNumber + 1,
		ParentJobID:      &originalJob.ID,
	}

	// Override with retry request values
	if req != nil {
		if req.PrinterID != nil {
			newJob.PrinterID = req.PrinterID
		}
		if req.MaterialSpoolID != nil {
			newJob.MaterialSpoolID = req.MaterialSpoolID
		}
		if req.Notes != "" {
			newJob.Notes = fmt.Sprintf("%s\n[Retry] %s", originalJob.Notes, req.Notes)
		}
	}

	if err := s.repo.Create(ctx, newJob); err != nil {
		return nil, fmt.Errorf("failed to create retry job: %w", err)
	}

	// Record retried event on original job
	retriedEvent := model.NewJobEvent(originalJob.ID, model.JobEventRetried, nil).
		WithActor(model.ActorUser, "").
		WithMetadata(map[string]interface{}{
			"retry_job_id": newJob.ID.String(),
		})
	s.repo.AppendEvent(ctx, retriedEvent)

	s.hub.Broadcast(realtime.Event{
		Type: "job_retried",
		Data: map[string]interface{}{
			"original_job": originalJob,
			"new_job":      newJob,
		},
	})

	return newJob, nil
}

// RecordFailure records a failure for a job (called by printer callbacks or error handlers).
func (s *PrintJobService) RecordFailure(ctx context.Context, id uuid.UUID, category model.FailureCategory, errorCode, errorMessage string) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	if job.Status.IsTerminal() {
		return fmt.Errorf("job already in terminal status: %s", job.Status)
	}

	// Record failed event
	failedStatus := model.PrintJobStatusFailed
	event := model.NewJobEvent(job.ID, model.JobEventFailed, &failedStatus).
		WithError(errorCode, errorMessage).
		WithActor(model.ActorPrinter, job.PrinterID.String())

	if err := s.repo.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to record failure: %w", err)
	}

	// Update job with failure info
	now := time.Now()
	job.CompletedAt = &now
	job.FailureCategory = &category
	s.repo.Update(ctx, job) //nolint:errcheck // best-effort failure update

	s.hub.Broadcast(realtime.Event{
		Type: "job_failed",
		Data: job,
	})

	return nil
}

// Init registers the printer status change callback to auto-detect job failures.
// Call this after services are created to enable automatic failure detection.
func (s *PrintJobService) Init() {
	s.printerMgr.OnStatusChange(s.handlePrinterStatusChange)
	slog.Info("PrintJobService: registered for printer status changes")
}

// handlePrinterStatusChange is called when any printer's status changes.
// It auto-detects job failures when a printer transitions to error state.
func (s *PrintJobService) handlePrinterStatusChange(newState *model.PrinterState, oldState *model.PrinterState) {
	if newState == nil {
		return
	}

	ctx := context.Background()

	// Detect failure: printer went from printing to error
	wasPrinting := oldState != nil && oldState.Status == model.PrinterStatusPrinting
	isError := newState.Status == model.PrinterStatusError

	if wasPrinting && isError {
		slog.Warn("printer failure detected", "printer_id", newState.PrinterID, "old_status", oldState.Status, "new_status", newState.Status)

		// Find active job for this printer
		job, err := s.GetActiveJobForPrinter(ctx, newState.PrinterID)
		if err != nil {
			slog.Error("failed to find active job for failed printer", "printer_id", newState.PrinterID, "error", err)
			return
		}
		if job == nil {
			slog.Debug("no active job found for failed printer", "printer_id", newState.PrinterID)
			return
		}

		// Auto-record the failure
		category := model.FailureUnknown
		errorCode := "PRINTER_ERROR"
		errorMessage := "Printer reported error during print"

		if err := s.RecordFailure(ctx, job.ID, category, errorCode, errorMessage); err != nil {
			slog.Error("failed to auto-record job failure", "job_id", job.ID, "error", err)
		} else {
			slog.Info("auto-recorded job failure", "job_id", job.ID, "printer_id", newState.PrinterID)
		}
	}
}

// GetActiveJobForPrinter finds the currently active (printing/paused) job for a printer.
func (s *PrintJobService) GetActiveJobForPrinter(ctx context.Context, printerID uuid.UUID) (*model.PrintJob, error) {
	// Get jobs for this printer that are in active states
	printingStatus := model.PrintJobStatusPrinting
	jobs, err := s.repo.List(ctx, &printerID, &printingStatus)
	if err != nil {
		return nil, err
	}
	if len(jobs) > 0 {
		return &jobs[0], nil
	}

	// Also check paused status
	pausedStatus := model.PrintJobStatusPaused
	jobs, err = s.repo.List(ctx, &printerID, &pausedStatus)
	if err != nil {
		return nil, err
	}
	if len(jobs) > 0 {
		return &jobs[0], nil
	}

	// Check uploaded status (job sent but not confirmed printing yet)
	uploadedStatus := model.PrintJobStatusUploaded
	jobs, err = s.repo.List(ctx, &printerID, &uploadedStatus)
	if err != nil {
		return nil, err
	}
	if len(jobs) > 0 {
		return &jobs[0], nil
	}

	return nil, nil
}

// MarkAsScrap marks a failed job as scrap (no retry intended).
// This is a user action to acknowledge the failure and move on.
type ScrapRequest struct {
	FailureCategory model.FailureCategory `json:"failure_category"`
	Notes           string                `json:"notes"`
}

func (s *PrintJobService) MarkAsScrap(ctx context.Context, id uuid.UUID, req *ScrapRequest) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	// Job must be in failed state to mark as scrap
	if job.Status != model.PrintJobStatusFailed {
		return fmt.Errorf("can only mark failed jobs as scrap, current status: %s", job.Status)
	}

	// Update job with scrap info
	if req != nil && req.FailureCategory != "" {
		job.FailureCategory = &req.FailureCategory
	}
	if req != nil && req.Notes != "" {
		if job.Notes != "" {
			job.Notes = job.Notes + "\n[Marked as Scrap] " + req.Notes
		} else {
			job.Notes = "[Marked as Scrap] " + req.Notes
		}
	} else {
		if job.Notes != "" {
			job.Notes += "\n[Marked as Scrap]"
		} else {
			job.Notes = "[Marked as Scrap]"
		}
	}

	// Record outcome as failed
	outcome := &model.PrintOutcome{
		Success:       false,
		FailureReason: "Marked as scrap by user",
	}
	if req != nil && req.Notes != "" {
		outcome.Notes = req.Notes
	}
	job.Outcome = outcome

	if err := s.repo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	s.hub.Broadcast(realtime.Event{
		Type: "job_updated",
		Data: job,
	})

	return nil
}

// UpdatePriority updates a job's priority in the queue.
func (s *PrintJobService) UpdatePriority(ctx context.Context, id uuid.UUID, priority int) error {
	return s.repo.UpdatePriority(ctx, id, priority)
}

// FileService handles file operations.
