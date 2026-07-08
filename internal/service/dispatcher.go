package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type queueDispatchService interface {
	FindNextReadyForPrinter(ctx context.Context, printerID uuid.UUID) (*model.QueueItem, error)
	Start(ctx context.Context, id uuid.UUID) error
}

// DispatcherService orchestrates auto-dispatch of jobs to printers.
type DispatcherService struct {
	dispatchRepo    *repository.DispatchRepository
	settingsRepo    *repository.AutoDispatchSettingsRepository
	printJobRepo    *repository.PrintJobRepository
	printerRepo     *repository.PrinterRepository
	printJobSvc     *PrintJobService
	printerMgr      *printer.Manager
	hub             *realtime.Hub
	settingsService *SettingsService
	queueSvc        queueDispatchService

	mu              sync.Mutex
	cleanupStopCh   chan struct{}
	cleanupInterval time.Duration
}

// NewDispatcherService creates a new dispatcher service.
func NewDispatcherService(
	dispatchRepo *repository.DispatchRepository,
	settingsRepo *repository.AutoDispatchSettingsRepository,
	printJobRepo *repository.PrintJobRepository,
	printerRepo *repository.PrinterRepository,
	printJobSvc *PrintJobService,
	printerMgr *printer.Manager,
	hub *realtime.Hub,
	settingsService *SettingsService,
	queueSvc queueDispatchService,
) *DispatcherService {
	return &DispatcherService{
		dispatchRepo:    dispatchRepo,
		settingsRepo:    settingsRepo,
		printJobRepo:    printJobRepo,
		printerRepo:     printerRepo,
		printJobSvc:     printJobSvc,
		printerMgr:      printerMgr,
		hub:             hub,
		settingsService: settingsService,
		queueSvc:        queueSvc,
		cleanupInterval: 1 * time.Minute,
	}
}

// Init registers printer status callback and starts cleanup goroutine.
func (s *DispatcherService) Init() {
	s.printerMgr.OnStatusChange(s.handlePrinterStatusChange)
	s.printerMgr.OnMacroAutomation(s.handleMacroAutomationEvent)
	slog.Info("DispatcherService: registered for printer status changes")

	// Start cleanup goroutine
	s.cleanupStopCh = make(chan struct{})
	go s.cleanupLoop()
}

// Stop stops the cleanup goroutine.
func (s *DispatcherService) Stop() {
	if s.cleanupStopCh != nil {
		close(s.cleanupStopCh)
	}
}

// handleMacroAutomationEvent is called when a printer receives an empty-bed/ready macro event.
func (s *DispatcherService) handleMacroAutomationEvent(printerID uuid.UUID) {
	ctx := context.Background()
	settings, err := s.settingsRepo.Get(ctx, printerID)
	if err != nil {
		slog.Error("DispatcherService: failed to get settings for macro event", "printer_id", printerID, "error", err)
		return
	}

	if !settings.MacroAutoDispatchEnabled {
		slog.Debug("DispatcherService: ignoring macro event, feature disabled", "printer_id", printerID)
		return
	}

	slog.Info("DispatcherService: handling macro automation event", "printer_id", printerID)

	// In macro mode, this signal forces the idle transition
	if err := s.OnPrinterIdleMacro(printerID, settings); err != nil {
		slog.Error("DispatcherService: error handling macro automation idle transition", "printer_id", printerID, "error", err)
	}
}

// handlePrinterStatusChange is called when a printer's status changes.
func (s *DispatcherService) handlePrinterStatusChange(newState, oldState *model.PrinterState) {
	if newState == nil {
		return
	}

	// Check for transition to idle from an active state
	if newState.Status == model.PrinterStatusIdle {
		wasActive := oldState != nil && (oldState.Status == model.PrinterStatusPrinting || oldState.Status == model.PrinterStatusPaused)
		if wasActive {
			go func() {
				if err := s.OnPrinterIdle(newState.PrinterID); err != nil {
					slog.Error("DispatcherService: failed to handle printer idle", "printer_id", newState.PrinterID, "error", err)
				}
			}()
		}
	}
}

// OnPrinterIdleMacro handles the specific case of an empty bed signaled by macro.
func (s *DispatcherService) OnPrinterIdleMacro(printerID uuid.UUID, settings *model.AutoDispatchSettings) error {
	ctx := context.Background()

	// Skip if global dispatch is off, as it governs all automated assignments
	if !s.IsGloballyEnabled(ctx) {
		slog.Debug("DispatcherService: auto-dispatch globally disabled (macro ignored)")
		return nil
	}

	if s.queueSvc != nil {
		item, err := s.queueSvc.FindNextReadyForPrinter(ctx, printerID)
		if err != nil {
			return fmt.Errorf("failed to find next queue item: %w", err)
		}
		if item != nil {
			slog.Info("DispatcherService: starting macro queue item", "queue_item_id", item.ID, "printer_id", printerID)
			return s.queueSvc.Start(ctx, item.ID)
		}
	}

	job, err := s.FindNextJob(ctx, printerID)
	if err != nil {
		return fmt.Errorf("failed to find next job: %w", err)
	}

	if job != nil {
		// Create and auto-start the dispatch request because this was triggered explicitly by macro (implies operator confirmed empty)
		request, err := s.CreateDispatchRequest(ctx, job.ID, printerID)
		if err != nil {
			return fmt.Errorf("failed to create dispatch request: %w", err)
		}

		slog.Info("DispatcherService: created macro dispatch request", "request_id", request.ID, "job_id", job.ID, "printer_id", printerID)
		s.broadcastDispatchRequest(request)

		// In Macro Mode we assume the bed is clean, so we immediately confirm to trigger print
		return s.ConfirmDispatch(ctx, request.ID)
	}

	slog.Info("DispatcherService: macro queue empty", "printer_id", printerID)
	if settings.MacroEmptyQueueGcode != "" {
		slog.Info("DispatcherService: sending empty queue G-code", "printer_id", printerID, "gcode", settings.MacroEmptyQueueGcode)
		// Run macro/G-code
		if err := s.printerMgr.RunMacro(printerID, settings.MacroEmptyQueueGcode); err != nil {
			slog.Error("DispatcherService: failed to run empty queue G-code", "printer_id", printerID, "error", err)
		}
	}
	return nil
}

// OnPrinterIdle is called when a printer transitions to idle state.
// It checks if auto-dispatch is enabled and creates a dispatch request if appropriate.
func (s *DispatcherService) OnPrinterIdle(printerID uuid.UUID) error {
	ctx := context.Background()

	// Check if global auto-dispatch is enabled
	if !s.IsGloballyEnabled(ctx) {
		slog.Debug("DispatcherService: auto-dispatch globally disabled")
		return nil
	}

	// Check printer-specific settings
	settings, err := s.settingsRepo.Get(ctx, printerID)
	if err != nil {
		return fmt.Errorf("failed to get printer settings: %w", err)
	}
	if !settings.Enabled {
		slog.Debug("DispatcherService: auto-dispatch disabled for printer", "printer_id", printerID)
		return nil
	}

	// Check if there's already a pending request for this printer
	existing, err := s.dispatchRepo.GetPendingForPrinter(ctx, printerID)
	if err != nil {
		return fmt.Errorf("failed to check existing requests: %w", err)
	}
	if existing != nil {
		slog.Debug("DispatcherService: pending request already exists", "printer_id", printerID, "request_id", existing.ID)
		return nil
	}

	// Find next compatible job
	job, err := s.FindNextJob(ctx, printerID)
	if err != nil {
		return fmt.Errorf("failed to find next job: %w", err)
	}
	if job == nil {
		slog.Debug("DispatcherService: no compatible job found", "printer_id", printerID)
		return nil
	}

	// Create dispatch request
	request, err := s.CreateDispatchRequest(ctx, job.ID, printerID)
	if err != nil {
		return fmt.Errorf("failed to create dispatch request: %w", err)
	}

	slog.Info("DispatcherService: created dispatch request", "request_id", request.ID, "job_id", job.ID, "printer_id", printerID)

	// Broadcast WebSocket event
	s.broadcastDispatchRequest(request)

	return nil
}

// FindNextJob finds the highest-priority compatible job for a printer.
func (s *DispatcherService) FindNextJob(ctx context.Context, printerID uuid.UUID) (*model.PrintJob, error) {
	// Get all queued jobs with auto_dispatch_enabled, ordered by priority DESC, created_at ASC
	jobs, err := s.printJobRepo.ListQueued(ctx)
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		// Skip if job already has a pending dispatch request
		pendingReq, err := s.dispatchRepo.GetPendingForJob(ctx, job.ID)
		if err != nil {
			slog.Warn("DispatcherService: failed to check pending request for job", "job_id", job.ID, "error", err)
			continue
		}
		if pendingReq != nil {
			continue
		}

		// Found a compatible job
		return &job, nil
	}

	return nil, nil
}

// CreateDispatchRequest creates a new pending dispatch request.
func (s *DispatcherService) CreateDispatchRequest(ctx context.Context, jobID, printerID uuid.UUID) (*model.DispatchRequest, error) {
	// Get printer settings for timeout
	settings, err := s.settingsRepo.Get(ctx, printerID)
	if err != nil {
		return nil, err
	}

	request := &model.DispatchRequest{
		JobID:     jobID,
		PrinterID: printerID,
		ExpiresAt: time.Now().Add(time.Duration(settings.TimeoutMinutes) * time.Minute),
	}

	if err := s.dispatchRepo.Create(ctx, request); err != nil {
		return nil, err
	}

	// Enrich with job and printer data
	job, _ := s.printJobRepo.GetByID(ctx, jobID)
	printer, _ := s.printerRepo.GetByID(ctx, printerID)
	request.Job = job
	request.Printer = printer

	return request, nil
}

// ConfirmDispatch confirms a dispatch request and starts the job.
func (s *DispatcherService) ConfirmDispatch(ctx context.Context, requestID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, err := s.dispatchRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if request == nil {
		return fmt.Errorf("dispatch request not found")
	}
	if request.Status != model.DispatchPending {
		return fmt.Errorf("dispatch request is not pending")
	}

	// Update request status
	if err := s.dispatchRepo.UpdateStatus(ctx, requestID, model.DispatchConfirmed, ""); err != nil {
		return err
	}

	// Get the job
	job, err := s.printJobRepo.GetByID(ctx, request.JobID)
	if err != nil {
		return err
	}
	if job == nil {
		return fmt.Errorf("job not found")
	}

	// Assign printer to job
	job.PrinterID = &request.PrinterID
	if err := s.printJobRepo.Update(ctx, job); err != nil {
		return err
	}

	// Record assignment event
	status := model.PrintJobStatusAssigned
	event := model.NewJobEvent(job.ID, model.JobEventAssigned, &status).WithPrinter(request.PrinterID)
	if err := s.printJobRepo.AppendEvent(ctx, event); err != nil {
		slog.Warn("DispatcherService: failed to record assignment event", "job_id", job.ID, "error", err)
	}

	// Check if auto-start is enabled
	settings, err := s.settingsRepo.Get(ctx, request.PrinterID)
	if err != nil {
		return err
	}

	if settings.AutoStart {
		// Start the job
		if err := s.printJobSvc.Start(ctx, job.ID); err != nil {
			slog.Warn("DispatcherService: failed to auto-start job", "job_id", job.ID, "error", err)
			// Don't return error - the job is assigned, just not started
		}
	}

	// Broadcast confirmation
	s.broadcastDispatchConfirmed(request)

	return nil
}

// RejectDispatch rejects a dispatch request.
func (s *DispatcherService) RejectDispatch(ctx context.Context, requestID uuid.UUID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, err := s.dispatchRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if request == nil {
		return fmt.Errorf("dispatch request not found")
	}
	if request.Status != model.DispatchPending {
		return fmt.Errorf("dispatch request is not pending")
	}

	if err := s.dispatchRepo.UpdateStatus(ctx, requestID, model.DispatchRejected, reason); err != nil {
		return err
	}

	// Broadcast rejection
	s.broadcastDispatchRejected(request, reason)

	return nil
}

// SkipJob skips the current job and tries to find the next compatible one.
func (s *DispatcherService) SkipJob(ctx context.Context, requestID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	request, err := s.dispatchRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}
	if request == nil {
		return fmt.Errorf("dispatch request not found")
	}
	if request.Status != model.DispatchPending {
		return fmt.Errorf("dispatch request is not pending")
	}

	// Mark current request as rejected (skipped)
	if err := s.dispatchRepo.UpdateStatus(ctx, requestID, model.DispatchRejected, "skipped"); err != nil {
		return err
	}

	// Disable auto-dispatch for this specific job
	job, err := s.printJobRepo.GetByID(ctx, request.JobID)
	if err != nil {
		return err
	}
	if job != nil {
		job.AutoDispatchEnabled = false
		if err := s.printJobRepo.Update(ctx, job); err != nil {
			slog.Warn("DispatcherService: failed to disable auto-dispatch for job", "job_id", job.ID, "error", err)
		}
	}

	// Try to find the next job
	s.mu.Unlock() // Release lock before calling OnPrinterIdle
	if err := s.OnPrinterIdle(request.PrinterID); err != nil {
		slog.Warn("DispatcherService: failed to find next job after skip", "printer_id", request.PrinterID, "error", err)
	}
	s.mu.Lock()

	return nil
}

// ListPending returns all pending dispatch requests.
func (s *DispatcherService) ListPending(ctx context.Context) ([]model.DispatchRequest, error) {
	requests, err := s.dispatchRepo.ListPending(ctx)
	if err != nil {
		return nil, err
	}

	// Enrich with job and printer data
	for i := range requests {
		job, _ := s.printJobRepo.GetByID(ctx, requests[i].JobID)
		printer, _ := s.printerRepo.GetByID(ctx, requests[i].PrinterID)
		requests[i].Job = job
		requests[i].Printer = printer
	}

	return requests, nil
}

// GetSettings returns auto-dispatch settings for a printer.
func (s *DispatcherService) GetSettings(ctx context.Context, printerID uuid.UUID) (*model.AutoDispatchSettings, error) {
	return s.settingsRepo.Get(ctx, printerID)
}

// UpdateSettings updates auto-dispatch settings for a printer.
func (s *DispatcherService) UpdateSettings(ctx context.Context, settings *model.AutoDispatchSettings) error {
	return s.settingsRepo.Upsert(ctx, settings)
}

// IsGloballyEnabled returns whether auto-dispatch is globally enabled.
func (s *DispatcherService) IsGloballyEnabled(ctx context.Context) bool {
	setting, err := s.settingsService.Get(ctx, "auto_dispatch_enabled")
	if err != nil || setting == nil {
		return false // Disabled by default
	}
	return setting.Value == "true" || setting.Value == "1"
}

// SetGlobalEnabled enables or disables auto-dispatch globally.
func (s *DispatcherService) SetGlobalEnabled(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return s.settingsService.Set(ctx, "auto_dispatch_enabled", val)
}

// cleanupLoop periodically expires old dispatch requests.
func (s *DispatcherService) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.cleanupStopCh:
			return
		case <-ticker.C:
			ctx := context.Background()
			n, err := s.dispatchRepo.ExpireOld(ctx)
			if err != nil {
				slog.Warn("DispatcherService: failed to expire old requests", "error", err)
			} else if n > 0 {
				slog.Info("DispatcherService: expired old dispatch requests", "count", n)
				// Broadcast expiration events
				s.hub.Broadcast(model.BroadcastEvent{
					Type: "dispatch_expired",
					Data: map[string]interface{}{"count": n},
				})
			}
		}
	}
}

// broadcastDispatchRequest sends a dispatch_request WebSocket event.
func (s *DispatcherService) broadcastDispatchRequest(request *model.DispatchRequest) {
	s.hub.Broadcast(model.BroadcastEvent{
		Type: "dispatch_request",
		Data: request,
	})
}

// broadcastDispatchConfirmed sends a dispatch_confirmed WebSocket event.
func (s *DispatcherService) broadcastDispatchConfirmed(request *model.DispatchRequest) {
	s.hub.Broadcast(model.BroadcastEvent{
		Type: "dispatch_confirmed",
		Data: request,
	})
}

// broadcastDispatchRejected sends a dispatch_rejected WebSocket event.
func (s *DispatcherService) broadcastDispatchRejected(request *model.DispatchRequest, reason string) {
	s.hub.Broadcast(model.BroadcastEvent{
		Type: "dispatch_rejected",
		Data: map[string]interface{}{
			"request": request,
			"reason":  reason,
		},
	})
}
