package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/gcode"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

type QueueBoardItem struct {
	Item      model.QueueItem       `json:"item"`
	File      *model.File           `json:"file,omitempty"`
	Printer   *model.Printer        `json:"printer,omitempty"`
	Spool     *model.MaterialSpool  `json:"spool,omitempty"`
	Material  *model.Material       `json:"material,omitempty"`
	Preflight *PreflightCheckResult `json:"preflight,omitempty"`
	Column    string                `json:"column"`
	BlockedBy []string              `json:"blocked_by,omitempty"`
}

type QueueSummary struct {
	ReadyCount         int     `json:"ready_count"`
	BlockedCount       int     `json:"blocked_count"`
	ActiveCount        int     `json:"active_count"`
	EstimatedSeconds   int     `json:"estimated_seconds"`
	TotalFilamentGrams float64 `json:"total_filament_grams"`
}

type QueueResponse struct {
	Items   []QueueBoardItem `json:"items"`
	Summary QueueSummary     `json:"summary"`
}

const queueStartStatusGrace = 30 * time.Second

type QueueCreateOptions struct {
	DisplayName            string     `json:"display_name"`
	AssignedPrinterID      *uuid.UUID `json:"assigned_printer_id,omitempty"`
	AssignedSpoolID        *uuid.UUID `json:"assigned_spool_id,omitempty"`
	ClearAssignedPrinterID bool       `json:"-"`
	ClearAssignedSpoolID   bool       `json:"-"`
	ClearThumbnailFileID   bool       `json:"-"`
	MaterialType           string     `json:"material_type,omitempty"`
	MaterialColor          string     `json:"material_color,omitempty"`
	FilamentGrams          *float64   `json:"filament_grams,omitempty"`
	EstimatedSeconds       *int       `json:"estimated_seconds,omitempty"`
	LayerHeight            *float64   `json:"layer_height,omitempty"`
	NozzleDiameter         *float64   `json:"nozzle_diameter,omitempty"`
	BedTemp                *float64   `json:"bed_temp,omitempty"`
	NozzleTemp             *float64   `json:"nozzle_temp,omitempty"`
	ThumbnailFileID        *uuid.UUID `json:"thumbnail_file_id,omitempty"`
	Notes                  string     `json:"notes,omitempty"`
}

type QueueService struct {
	repo          *repository.QueueItemRepository
	fileRepo      *repository.FileRepository
	printJobRepo  *repository.PrintJobRepository
	designRepo    *repository.DesignRepository
	printerRepo   *repository.PrinterRepository
	spoolRepo     *repository.SpoolRepository
	materialRepo  *repository.MaterialRepository
	settingsRepo  *repository.SettingsRepository
	libraryRepo   *repository.GCodeLibraryRepository
	storage       storage.Storage
	printerMgr    *printer.Manager
	hub           *realtime.Hub
	notifications *NotificationService
}

func NewQueueService(repos *repository.Repositories, store storage.Storage, printerMgr *printer.Manager, hub *realtime.Hub) *QueueService {
	svc := &QueueService{
		repo:         repos.QueueItems,
		fileRepo:     repos.Files,
		printJobRepo: repos.PrintJobs,
		designRepo:   repos.Designs,
		printerRepo:  repos.Printers,
		spoolRepo:    repos.Spools,
		materialRepo: repos.Materials,
		settingsRepo: repos.Settings,
		libraryRepo:  repos.GCodeLibrary,
		storage:      store,
		printerMgr:   printerMgr,
		hub:          hub,
	}
	if printerMgr != nil {
		printerMgr.OnStatusChange(svc.handlePrinterStatus)
	}
	return svc
}

func (s *QueueService) SetNotificationService(notifications *NotificationService) {
	s.notifications = notifications
}

func (s *QueueService) List(ctx context.Context) (*QueueResponse, error) {
	s.reconcileStaleActiveItems(ctx)
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	resp := &QueueResponse{Items: []QueueBoardItem{}}
	for _, item := range items {
		board := s.buildItem(ctx, item)
		resp.Items = append(resp.Items, board)
		resp.Summary.EstimatedSeconds += valueInt(item.EstimatedSeconds)
		resp.Summary.TotalFilamentGrams += valueFloat(item.FilamentGrams)
		switch board.Column {
		case "ready":
			resp.Summary.ReadyCount++
		case "blocked":
			resp.Summary.BlockedCount++
		case "active":
			resp.Summary.ActiveCount++
		}
	}
	return resp, nil
}

func (s *QueueService) DefaultPrinterID(ctx context.Context) *uuid.UUID {
	if s.printerRepo == nil {
		return nil
	}
	ps, err := s.printerRepo.List(ctx)
	if err != nil || len(ps) == 0 {
		return nil
	}
	if len(ps) == 1 {
		return &ps[0].ID
	}
	if s.settingsRepo != nil {
		setting, err := s.settingsRepo.Get(ctx, "default_printer_id")
		if err == nil && setting != nil && setting.Value != "" {
			if id, err := uuid.Parse(setting.Value); err == nil {
				for _, p := range ps {
					if p.ID == id {
						return &id
					}
				}
			}
		}
	}
	return nil
}

func (s *QueueService) CreateFromUpload(ctx context.Context, filename string, reader io.Reader, opts QueueCreateOptions) (*model.QueueItem, error) {
	if strings.ToLower(filepath.Ext(filename)) != ".gcode" {
		return nil, fmt.Errorf("only .gcode files can be added to the queue")
	}
	storagePath, hash, size, err := s.storage.Save(filename, reader)
	if err != nil {
		return nil, err
	}
	// Always analyze the file we just received (before dedup logic)
	tmpFile := &model.File{Hash: hash, OriginalName: filename, ContentType: "text/x-gcode", SizeBytes: size, StoragePath: storagePath}
	metadata, thumbnailFileID := s.analyzeFile(ctx, tmpFile)

	file, err := s.fileRepo.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if file != nil {
		if file.StoragePath != storagePath {
			if existingReader, err := s.storage.Get(file.StoragePath); err == nil {
				existingReader.Close()
				_ = s.storage.Delete(storagePath)
			} else {
				file.OriginalName = filename
				file.ContentType = "text/x-gcode"
				file.SizeBytes = size
				file.StoragePath = storagePath
				if err := s.fileRepo.Update(ctx, file); err != nil {
					return nil, err
				}
			}
		}
	} else {
		file = tmpFile
		if err := s.fileRepo.Create(ctx, file); err != nil {
			return nil, err
		}
	}
	if opts.AssignedPrinterID == nil {
		opts.AssignedPrinterID = s.DefaultPrinterID(ctx)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceUpload, FileID: file.ID, FileName: filename, DisplayName: opts.DisplayName, Status: model.QueueItemStatusQueued, AssignedPrinterID: opts.AssignedPrinterID, AssignedSpoolID: opts.AssignedSpoolID, MaterialType: opts.MaterialType, MaterialColor: opts.MaterialColor, FilamentGrams: opts.FilamentGrams, EstimatedSeconds: opts.EstimatedSeconds, Notes: opts.Notes, Metadata: metadata, ThumbnailFileID: thumbnailFileID}
	if item.DisplayName == "" {
		item.DisplayName = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	s.applyMetadata(item, metadata)
	s.applyDefaultSpool(ctx, item)
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *QueueService) CreateFromPrintJob(ctx context.Context, jobID uuid.UUID, opts QueueCreateOptions) (*model.QueueItem, error) {
	job, err := s.printJobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("print job not found")
	}
	design, err := s.designRepo.GetByID(ctx, job.DesignID)
	if err != nil {
		return nil, err
	}
	if design == nil {
		return nil, fmt.Errorf("design not found")
	}
	if design.FileType != model.FileTypeGCODE {
		return nil, fmt.Errorf("print job design is not a .gcode file")
	}
	var metadata *model.GCodeMetadata
	var thumbnailFileID *uuid.UUID
	file, _ := s.fileRepo.GetByID(ctx, design.FileID)
	if file != nil {
		metadata, thumbnailFileID = s.analyzeFile(ctx, file)
	}
	printerID := job.PrinterID
	if printerID == nil {
		printerID = s.DefaultPrinterID(ctx)
	}
	item := &model.QueueItem{SourceType: model.QueueSourcePrintJob, SourceID: &job.ID, FileID: design.FileID, FileName: design.FileName, DisplayName: opts.DisplayName, Status: model.QueueItemStatusQueued, AssignedPrinterID: printerID, AssignedSpoolID: job.MaterialSpoolID, MaterialType: opts.MaterialType, MaterialColor: opts.MaterialColor, FilamentGrams: job.MaterialUsedGrams, EstimatedSeconds: job.EstimatedSeconds, Notes: opts.Notes, Metadata: metadata, ThumbnailFileID: thumbnailFileID}
	if item.DisplayName == "" {
		item.DisplayName = strings.TrimSuffix(design.FileName, filepath.Ext(design.FileName))
	}
	s.applyOptions(item, opts)
	s.applyMetadata(item, metadata)
	s.applyDefaultSpool(ctx, item)
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *QueueService) Update(ctx context.Context, id uuid.UUID, opts QueueCreateOptions) (*model.QueueItem, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("queue item not found")
	}
	s.applyUpdateOptions(item, opts)
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *QueueService) Delete(ctx context.Context, id uuid.UUID) error {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item != nil && item.SourceType == model.QueueSourcePrintJob && item.SourceID != nil {
		if job, err := s.printJobRepo.GetByID(ctx, *item.SourceID); err == nil && job != nil {
			job.Status = model.PrintJobStatus("queue_canceled")
			_ = s.printJobRepo.Update(ctx, job)
		}
	}
	return s.repo.Delete(ctx, id)
}

func (s *QueueService) UpdatePriority(ctx context.Context, id uuid.UUID, priority int) error {
	return s.repo.UpdatePriority(ctx, id, priority)
}

// FindNextReadyForPrinter returns the highest-priority queued file that can be started on the printer.
func (s *QueueService) FindNextReadyForPrinter(ctx context.Context, printerID uuid.UUID) (*model.QueueItem, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		if item.AssignedPrinterID == nil || *item.AssignedPrinterID != printerID {
			continue
		}
		if item.Status != model.QueueItemStatusQueued && item.Status != model.QueueItemStatusReady {
			continue
		}

		preflight, err := s.PreflightCheck(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if preflight.Ready {
			return &item, nil
		}
	}

	return nil, nil
}

func (s *QueueService) PreflightCheck(ctx context.Context, id uuid.UUID) (*PreflightCheckResult, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("queue item not found")
	}
	result := &PreflightCheckResult{Ready: true}
	if item.AssignedPrinterID == nil {
		result.Ready = false
		result.Errors = append(result.Errors, "missing printer")
	}
	if _, err := s.fileRepo.GetByID(ctx, item.FileID); err != nil {
		result.Ready = false
		result.Errors = append(result.Errors, "file not found")
	}
	if item.AssignedPrinterID != nil {
		printerData, err := s.printerRepo.GetByID(ctx, *item.AssignedPrinterID)
		if err != nil || printerData == nil {
			result.Ready = false
			result.Errors = append(result.Errors, "printer not found")
		}
		if printerData.RestrictGCodeModel && item.Metadata != nil && item.Metadata.PrinterModel != "" && !strings.EqualFold(strings.TrimSpace(item.Metadata.PrinterModel), strings.TrimSpace(printerData.Model)) {
			result.Ready = false
			result.Errors = append(result.Errors, fmt.Sprintf("gcode printer model %q does not match printer model %q", item.Metadata.PrinterModel, printerData.Model))
		}
		printerState, _ := s.printerMgr.GetState(*item.AssignedPrinterID)
		if printerState != nil {
			result.AMSState = printerState.AMS
			if printerState.Status != model.PrinterStatusIdle {
				result.Ready = false
				result.Errors = append(result.Errors, fmt.Sprintf("printer is %s", printerState.Status))
			}
		}
	}
	if item.AssignedSpoolID != nil {
		spool, err := s.spoolRepo.GetByID(ctx, *item.AssignedSpoolID)
		if err != nil || spool == nil {
			result.Ready = false
			result.Errors = append(result.Errors, "spool not found")
		} else if item.FilamentGrams != nil && spool.RemainingWeight < *item.FilamentGrams {
			result.Ready = false
			result.Errors = append(result.Errors, "not enough filament on spool")
		}
	}
	return result, nil
}

func (s *QueueService) Start(ctx context.Context, id uuid.UUID) error {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("queue item not found")
	}
	preflight, err := s.PreflightCheck(ctx, id)
	if err != nil {
		return err
	}
	if !preflight.Ready {
		return fmt.Errorf("%s", strings.Join(preflight.Errors, ", "))
	}
	file, err := s.fileRepo.GetByID(ctx, item.FileID)
	if err != nil || file == nil {
		return fmt.Errorf("file not found")
	}
	if err := s.printerMgr.StartJob(*item.AssignedPrinterID, item.FileName, s.storage.GetFullPath(file.StoragePath)); err != nil {
		item.FailedAttempts++
		item.Status = model.QueueItemStatusFailed
		_ = s.repo.Update(ctx, item)
		return err
	}
	return s.markStarted(ctx, item)
}

func (s *QueueService) markStarted(ctx context.Context, item *model.QueueItem) error {
	item.Status = model.QueueItemStatusPrinting
	item.Progress = 0
	if err := s.repo.Update(ctx, item); err != nil {
		return err
	}
	if s.hub != nil {
		s.hub.Broadcast(realtime.Event{Type: "queue_item_started", Data: item})
	}
	s.dispatchQueueNotification(ctx, item, "print.started", "info", "Print started")
	return nil
}

func (s *QueueService) recoverFalseFailedStart(ctx context.Context, item *model.QueueItem) error {
	if item.FailedAttempts > 0 {
		item.FailedAttempts--
	}
	item.WastedGrams = 0
	if strings.Contains(item.Notes, "Cancelled on printer") {
		item.Notes = strings.TrimSpace(strings.ReplaceAll(item.Notes, "Cancelled on printer", ""))
	}
	return s.markStarted(ctx, item)
}

func (s *QueueService) SetStatus(ctx context.Context, id uuid.UUID, status model.QueueItemStatus) error {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if item == nil {
		return fmt.Errorf("queue item not found")
	}
	if status == model.QueueItemStatusCancelled {
		item.WastedGrams += queueItemAttemptWaste(*item)
		item.FailedAttempts++
		item.Status = model.QueueItemStatusFailed
		if item.Notes == "" {
			item.Notes = "Cancelled"
		} else if !strings.Contains(strings.ToLower(item.Notes), "cancel") {
			item.Notes += "\nCancelled"
		}
	} else {
		item.Status = status
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return err
	}
	switch status {
	case model.QueueItemStatusCancelled:
		s.dispatchQueueNotification(ctx, item, "print.cancelled", "warning", "Print cancelled")
	case model.QueueItemStatusFailed:
		s.dispatchQueueNotification(ctx, item, "print.failed", "error", "Print failed")
	case model.QueueItemStatusDone:
		s.dispatchQueueNotification(ctx, item, "print.completed", "success", "Print completed")
	}
	return nil
}

func (s *QueueService) dispatchQueueNotification(ctx context.Context, item *model.QueueItem, eventType string, severity string, title string) {
	if s.notifications == nil || item == nil {
		return
	}
	message := fmt.Sprintf("File: %s", item.FileName)
	data := map[string]any{"queue_item_id": item.ID.String(), "file_name": item.FileName, "status": item.Status}
	if item.AssignedPrinterID != nil && s.printerRepo != nil {
		printer, err := s.printerRepo.GetByID(ctx, *item.AssignedPrinterID)
		if err == nil && printer != nil {
			data["printer_name"] = printer.Name
			data["printer_model"] = printer.Model
		}
	}
	if item.Progress > 0 {
		data["progress"] = item.Progress
		message = fmt.Sprintf("%s\nProgress: %.1f%%", message, item.Progress)
	}
	if item.FilamentGrams != nil {
		data["filament_grams"] = *item.FilamentGrams
	}
	if item.WastedGrams > 0 {
		data["wasted_grams"] = item.WastedGrams
	}
	if item.Notes != "" {
		data["notes"] = item.Notes
		message = fmt.Sprintf("%s\nNotes: %s", message, item.Notes)
	}
	s.notifications.Dispatch(ctx, model.NotificationEvent{Type: eventType, Severity: severity, Title: title, Message: message, Timestamp: time.Now().UTC(), PrinterID: item.AssignedPrinterID, Data: data})
}

func (s *QueueService) dispatchPrinterStatusNotification(ctx context.Context, newState *model.PrinterState, oldState *model.PrinterState) {
	if s.notifications == nil || newState == nil || oldState == nil || newState.Status == oldState.Status {
		return
	}
	eventType := ""
	severity := "info"
	title := ""
	switch newState.Status {
	case model.PrinterStatusOffline:
		eventType = "printer.offline"
		severity = "warning"
		title = "Printer offline"
	case model.PrinterStatusError:
		eventType = "printer.error"
		severity = "error"
		title = "Printer error"
	case model.PrinterStatusIdle, model.PrinterStatusPrinting, model.PrinterStatusPaused:
		if oldState.Status == model.PrinterStatusOffline || oldState.Status == model.PrinterStatusError {
			eventType = "printer.online"
			severity = "success"
			title = "Printer online"
		}
	}
	if eventType == "" {
		return
	}
	printerID := newState.PrinterID
	data := map[string]any{"status": newState.Status, "previous_status": oldState.Status}
	if s.printerRepo != nil {
		printer, err := s.printerRepo.GetByID(ctx, printerID)
		if err == nil && printer != nil {
			data["printer_name"] = printer.Name
			data["printer_model"] = printer.Model
		}
	}
	s.notifications.Dispatch(ctx, model.NotificationEvent{Type: eventType, Severity: severity, Title: title, Message: fmt.Sprintf("Printer status changed to %s", newState.Status), Timestamp: time.Now().UTC(), PrinterID: &printerID, Data: data})
}

func currentFileMatchesQueueItem(state *model.PrinterState, item model.QueueItem) bool {
	currentFile := filepath.Base(strings.TrimSpace(state.CurrentFile))
	if currentFile == "" {
		return false
	}
	return currentFile == filepath.Base(strings.TrimSpace(item.FileName))
}

func queueItemAttemptWaste(item model.QueueItem) float64 {
	if item.FilamentGrams == nil {
		return 0
	}
	progress := item.Progress
	if progress <= 0 && (item.Status == model.QueueItemStatusPrinting || item.Status == model.QueueItemStatusPaused) {
		progress = 1
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	return *item.FilamentGrams * progress / 100
}

func (s *QueueService) reconcileStaleActiveItems(ctx context.Context) {
	if s.printerMgr == nil {
		return
	}
	items, err := s.repo.List(ctx)
	if err != nil {
		return
	}
	for _, item := range items {
		if item.AssignedPrinterID == nil || (item.Status != model.QueueItemStatusPrinting && item.Status != model.QueueItemStatusPaused) {
			continue
		}
		state, err := s.printerMgr.GetState(*item.AssignedPrinterID)
		if err != nil || state == nil || state.Status == model.PrinterStatusPrinting || state.Status == model.PrinterStatusPaused {
			continue
		}
		if time.Since(item.UpdatedAt) <= queueStartStatusGrace {
			continue
		}
		oldState := &model.PrinterState{PrinterID: *item.AssignedPrinterID, Status: model.PrinterStatusPrinting, Progress: item.Progress}
		s.handlePrinterStatus(state, oldState)
	}
}

func (s *QueueService) handlePrinterStatus(newState *model.PrinterState, oldState *model.PrinterState) {
	if newState == nil {
		return
	}
	s.dispatchPrinterStatusNotification(context.Background(), newState, oldState)
	wasActive := oldState != nil && (oldState.Status == model.PrinterStatusPrinting || oldState.Status == model.PrinterStatusPaused)
	isActive := newState.Status == model.PrinterStatusPrinting || newState.Status == model.PrinterStatusPaused
	if !wasActive && !isActive {
		return
	}
	ctx := context.Background()
	items, err := s.repo.List(ctx)
	if err != nil {
		return
	}
	for _, item := range items {
		if item.AssignedPrinterID == nil || *item.AssignedPrinterID != newState.PrinterID {
			continue
		}
		if item.Status == model.QueueItemStatusFailed && isActive && currentFileMatchesQueueItem(newState, item) && time.Since(item.UpdatedAt) <= queueStartStatusGrace {
			if err := s.recoverFalseFailedStart(ctx, &item); err == nil {
				continue
			}
		}
		if item.Status != model.QueueItemStatusPrinting && item.Status != model.QueueItemStatusPaused {
			continue
		}
		if isActive {
			item.Progress = newState.Progress
			if err := s.repo.Update(ctx, &item); err == nil && s.hub != nil {
				s.hub.Broadcast(realtime.Event{Type: "queue_item_updated", Data: item})
			}
			continue
		}
		consumedProgress := item.Progress
		switch newState.Status {
		case model.PrinterStatusIdle:
			lastProgress := item.Progress
			if oldState != nil && oldState.Progress > lastProgress {
				lastProgress = oldState.Progress
			}
			if lastProgress < 99 {
				item.Progress = lastProgress
				consumedProgress = lastProgress
				item.WastedGrams += queueItemAttemptWaste(item)
				item.FailedAttempts++
				item.Status = model.QueueItemStatusFailed
				if item.Notes == "" {
					item.Notes = "Cancelled on printer"
				} else if !strings.Contains(strings.ToLower(item.Notes), "cancel") {
					item.Notes += "\nCancelled on printer"
				}
			} else {
				item.Status = model.QueueItemStatusDone
				consumedProgress = 100
				if (item.SourceType == model.QueueSourceLibrary || item.SourceType == model.QueueSourceProject) && item.SourceID != nil && s.libraryRepo != nil {
					_ = s.libraryRepo.IncrementPrintCount(ctx, *item.SourceID)
				}
			}
		case model.PrinterStatusError, model.PrinterStatusOffline:
			item.WastedGrams += queueItemAttemptWaste(item)
			item.FailedAttempts++
			item.Status = model.QueueItemStatusFailed
		default:
			continue
		}
		if item.Status == model.QueueItemStatusDone {
			item.Progress = 0
		}
		if err := s.repo.Update(ctx, &item); err == nil {
			if s.hub != nil {
				s.hub.Broadcast(realtime.Event{Type: "queue_item_updated", Data: item})
			}
			switch item.Status {
			case model.QueueItemStatusDone:
				s.dispatchQueueNotification(ctx, &item, "print.completed", "success", "Print completed")
			case model.QueueItemStatusFailed:
				s.dispatchQueueNotification(ctx, &item, "print.failed", "error", "Print failed")
			}
		}
		if item.AssignedSpoolID != nil {
			spool, err := s.spoolRepo.GetByID(ctx, *item.AssignedSpoolID)
			if err == nil && spool != nil && item.FilamentGrams != nil {
				used := *item.FilamentGrams * consumedProgress / 100.0
				if used < 0 {
					used = 0
				}
				if used > *item.FilamentGrams {
					used = *item.FilamentGrams
				}
				spool.RemainingWeight -= used
				if spool.RemainingWeight < 0 {
					spool.RemainingWeight = 0
				}
				_ = s.spoolRepo.Update(ctx, spool)
			}
		}
	}
}

func (s *QueueService) buildItem(ctx context.Context, item model.QueueItem) QueueBoardItem {
	board := QueueBoardItem{Item: item, Column: "blocked"}
	if file, err := s.fileRepo.GetByID(ctx, item.FileID); err == nil {
		board.File = file
	}
	if item.AssignedPrinterID != nil {
		if printerData, err := s.printerRepo.GetByID(ctx, *item.AssignedPrinterID); err == nil {
			board.Printer = printerData
		}
	}
	if item.AssignedSpoolID != nil {
		if spool, err := s.spoolRepo.GetByID(ctx, *item.AssignedSpoolID); err == nil {
			board.Spool = spool
			if spool != nil {
				if material, err := s.materialRepo.GetByID(ctx, spool.MaterialID); err == nil {
					board.Material = material
				}
			}
		}
	}
	if item.Status == model.QueueItemStatusPrinting || item.Status == model.QueueItemStatusPaused {
		board.Column = "active"
		board.Item.Progress = item.Progress
		return board
	}
	if item.Status == model.QueueItemStatusDone {
		board.Column = "done"
		return board
	}
	if item.Status == model.QueueItemStatusFailed || item.Status == model.QueueItemStatusCancelled {
		board.Column = "ready"
		return board
	}
	preflight, err := s.PreflightCheck(ctx, item.ID)
	if err == nil {
		board.Preflight = preflight
		if !preflight.Ready {
			board.BlockedBy = append(board.BlockedBy, preflight.Errors...)
		}
	}
	if len(board.BlockedBy) == 0 {
		board.Column = "ready"
	}
	return board
}

func (s *QueueService) analyzeFile(ctx context.Context, file *model.File) (*model.GCodeMetadata, *uuid.UUID) {
	reader, err := s.storage.Get(file.StoragePath)
	if err != nil {
		return nil, nil
	}
	defer reader.Close()
	analysis, err := gcode.Analyze(reader)
	if err != nil || analysis == nil {
		return nil, nil
	}
	var thumbnailFileID, originalThumbnailFileID *uuid.UUID
	if analysis.Thumbnail != nil {
		thumbnailFileID = s.saveThumbnail(ctx, file.OriginalName, analysis.Thumbnail)
		originalThumbnailFileID = thumbnailFileID
	}
	metadata := analysisToMetadata(analysis)
	if metadata != nil {
		metadata.ThumbnailFileID = thumbnailFileID
		metadata.OriginalThumbnailFileID = originalThumbnailFileID
	}
	return metadata, thumbnailFileID
}

func (s *QueueService) saveThumbnail(ctx context.Context, sourceName string, thumbnail *gcode.Thumbnail) *uuid.UUID {
	if thumbnail == nil || len(thumbnail.Data) == 0 {
		return nil
	}
	ext := ".png"
	if thumbnail.MimeType == "image/jpeg" {
		ext = ".jpg"
	}
	name := strings.TrimSuffix(sourceName, filepath.Ext(sourceName)) + "-thumbnail" + ext
	storagePath, hash, size, err := s.storage.Save(name, bytes.NewReader(thumbnail.Data))
	if err != nil {
		return nil
	}
	if existing, err := s.fileRepo.GetByHash(ctx, hash); err == nil && existing != nil {
		if existing.StoragePath == storagePath {
			return &existing.ID
		}
		if reader, err := s.storage.Get(existing.StoragePath); err == nil {
			reader.Close()
			_ = s.storage.Delete(storagePath)
			return &existing.ID
		}

		existing.OriginalName = name
		existing.ContentType = thumbnail.MimeType
		existing.SizeBytes = size
		existing.StoragePath = storagePath
		if err := s.fileRepo.Update(ctx, existing); err == nil {
			return &existing.ID
		}
	}
	file := &model.File{Hash: hash, OriginalName: name, ContentType: thumbnail.MimeType, SizeBytes: size, StoragePath: storagePath}
	if err := s.fileRepo.Create(ctx, file); err != nil {
		return nil
	}
	return &file.ID
}

func (s *QueueService) applyUpdateOptions(item *model.QueueItem, opts QueueCreateOptions) {
	if opts.DisplayName != "" {
		item.DisplayName = opts.DisplayName
	}
	if opts.ClearAssignedPrinterID {
		item.AssignedPrinterID = nil
	} else if opts.AssignedPrinterID != nil {
		item.AssignedPrinterID = opts.AssignedPrinterID
	}
	if opts.ClearAssignedSpoolID {
		item.AssignedSpoolID = nil
	} else if opts.AssignedSpoolID != nil {
		item.AssignedSpoolID = opts.AssignedSpoolID
	}
	if opts.ClearThumbnailFileID {
		item.ThumbnailFileID = nil
	} else if opts.ThumbnailFileID != nil {
		item.ThumbnailFileID = opts.ThumbnailFileID
	}
	if opts.Notes != "" {
		item.Notes = opts.Notes
	}
}

func (s *QueueService) applyOptions(item *model.QueueItem, opts QueueCreateOptions) {
	if opts.DisplayName != "" {
		item.DisplayName = opts.DisplayName
	}
	if opts.AssignedPrinterID != nil {
		item.AssignedPrinterID = opts.AssignedPrinterID
	}
	if opts.AssignedSpoolID != nil {
		item.AssignedSpoolID = opts.AssignedSpoolID
	}
	if opts.MaterialType != "" {
		item.MaterialType = opts.MaterialType
	}
	if opts.MaterialColor != "" {
		item.MaterialColor = opts.MaterialColor
	}
	if opts.FilamentGrams != nil {
		item.FilamentGrams = opts.FilamentGrams
	}
	if opts.EstimatedSeconds != nil {
		item.EstimatedSeconds = opts.EstimatedSeconds
	}
	if opts.LayerHeight != nil {
		item.LayerHeight = opts.LayerHeight
	}
	if opts.NozzleDiameter != nil {
		item.NozzleDiameter = opts.NozzleDiameter
	}
	if opts.BedTemp != nil {
		item.BedTemp = opts.BedTemp
	}
	if opts.NozzleTemp != nil {
		item.NozzleTemp = opts.NozzleTemp
	}
	if opts.ThumbnailFileID != nil {
		item.ThumbnailFileID = opts.ThumbnailFileID
	}
	if opts.Notes != "" {
		item.Notes = opts.Notes
	}
}

func (s *QueueService) applyMetadata(item *model.QueueItem, metadata *model.GCodeMetadata) {
	if metadata == nil {
		return
	}
	if item.MaterialType == "" {
		item.MaterialType = metadata.MaterialType
	}
	if item.MaterialColor == "" {
		item.MaterialColor = metadata.MaterialColor
	}
	if item.FilamentName == "" {
		item.FilamentName = metadata.FilamentName
	}
	if item.FilamentGrams == nil {
		item.FilamentGrams = metadata.FilamentGrams
	}
	if item.EstimatedSeconds == nil {
		item.EstimatedSeconds = metadata.EstimatedSeconds
	}
	if item.LayerHeight == nil {
		item.LayerHeight = metadata.LayerHeight
	}
	if item.NozzleDiameter == nil {
		item.NozzleDiameter = metadata.NozzleDiameter
	}
	if item.BedTemp == nil {
		item.BedTemp = metadata.BedTemp
	}
	if item.NozzleTemp == nil {
		item.NozzleTemp = metadata.NozzleTemp
	}
}

func (s *QueueService) applyDefaultSpool(ctx context.Context, item *model.QueueItem) {
	if item.AssignedSpoolID != nil || s.spoolRepo == nil || strings.TrimSpace(item.MaterialType) == "" {
		return
	}
	spool, err := s.spoolRepo.GetDefaultForMaterialType(ctx, item.MaterialType)
	if err != nil || spool == nil {
		return
	}
	item.AssignedSpoolID = &spool.ID
}

func normalizeMaterialType(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if idx := strings.Index(v, "#"); idx >= 0 {
		v = strings.TrimSpace(v[:idx])
	}
	v = strings.ReplaceAll(v, " ", "")
	v = strings.ReplaceAll(v, "-", "")
	v = strings.ReplaceAll(v, "_", "")
	switch {
	case strings.Contains(v, "petg"):
		return "petg"
	case strings.Contains(v, "abs"):
		return "abs"
	case strings.Contains(v, "asa"):
		return "asa"
	case strings.Contains(v, "tpu") || strings.Contains(v, "flex"):
		return "tpu"
	case strings.Contains(v, "pla"):
		return "pla"
	default:
		return v
	}
}

func analysisToMetadata(analysis *gcode.Analysis) *model.GCodeMetadata {
	if analysis == nil {
		return nil
	}
	metadata := &model.GCodeMetadata{
		Slicer:             analysis.Slicer,
		MaterialType:       normalizeMaterialType(analysis.MaterialType),
		PrintSettingsID:    analysis.PrintSettingsID,
		PrinterSettingsID:  analysis.PrinterSettingsID,
		FilamentSettingsID: analysis.FilamentSettingsID,
		PrinterModel:       analysis.PrinterModel,
		FilamentName:       analysis.FilamentName,
		FilamentGrams:      analysis.FilamentGrams,
		EstimatedSeconds:   analysis.EstimatedSeconds,
		LayerHeight:        analysis.LayerHeight,
		NozzleDiameter:     analysis.NozzleDiameter,
		BedTemp:            analysis.BedTemp,
		NozzleTemp:         analysis.NozzleTemp,
		Raw:                map[string]interface{}{},
	}
	for key, value := range analysis.Raw {
		metadata.Raw[key] = value
	}
	return metadata
}

func valueInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func valueFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
