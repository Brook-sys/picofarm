package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/gcode"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
)

func TestQueueService_saveThumbnail_repairsMissingPhysicalFile(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, printer.NewManager(), nil)
	ctx := context.Background()

	thumbnail := &gcode.Thumbnail{MimeType: "image/png", Data: []byte("png-data")}
	storagePath, hash, size, err := store.Save("old-thumbnail.png", bytes.NewReader(thumbnail.Data))
	if err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: hash, OriginalName: "old-thumbnail.png", ContentType: "image/png", SizeBytes: size, StoragePath: storagePath}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.GetFullPath(storagePath)); err != nil {
		t.Fatal(err)
	}

	id := service.saveThumbnail(ctx, "new.gcode", thumbnail)
	if id == nil || *id != file.ID {
		t.Fatalf("expected repaired existing ID %v, got %v", file.ID, id)
	}

	updated, err := repos.Files.GetByID(ctx, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.StoragePath == storagePath {
		t.Fatal("expected storage_path to be updated to the newly saved file")
	}
	reader, err := store.Get(updated.StoragePath)
	if err != nil {
		t.Fatalf("expected repaired file to exist: %v", err)
	}
	reader.Close()
	if filepath.Base(updated.StoragePath) != "new-thumbnail.png" {
		t.Fatalf("unexpected thumbnail filename: %s", updated.StoragePath)
	}
}

func TestQueueService_FindNextReadyForPrinter_ReturnsQueuedLibraryItem(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Macro Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}
	otherPrinter := &model.Printer{Name: "Other Printer"}
	if err := repos.Printers.Create(ctx, otherPrinter); err != nil {
		t.Fatal(err)
	}

	file := &model.File{Hash: "queued-hash", OriginalName: "queued.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "queued.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	doneItem := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "done.gcode", DisplayName: "Done", Status: model.QueueItemStatusDone, AssignedPrinterID: &printerObj.ID, Priority: 100}
	if err := repos.QueueItems.Create(ctx, doneItem); err != nil {
		t.Fatal(err)
	}
	otherPrinterItem := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "other.gcode", DisplayName: "Other", Status: model.QueueItemStatusQueued, AssignedPrinterID: &otherPrinter.ID, Priority: 90}
	if err := repos.QueueItems.Create(ctx, otherPrinterItem); err != nil {
		t.Fatal(err)
	}
	readyItem := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "ready.gcode", DisplayName: "Ready", Status: model.QueueItemStatusQueued, AssignedPrinterID: &printerObj.ID, Priority: 80}
	if err := repos.QueueItems.Create(ctx, readyItem); err != nil {
		t.Fatal(err)
	}

	got, err := service.FindNextReadyForPrinter(ctx, printerObj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected a queued library item for the macro printer")
	}
	if got.ID != readyItem.ID {
		t.Fatalf("expected ready item %s, got %s", readyItem.ID, got.ID)
	}
}

func TestQueueService_handlePrinterStatus_DoesNotRecoverFailedStart(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Macro Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}

	file := &model.File{Hash: "false-failed-hash", OriginalName: "elegoo_logo.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "elegoo_logo.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	item := &model.QueueItem{
		SourceType:        model.QueueSourceLibrary,
		FileID:            file.ID,
		FileName:          "elegoo_logo.gcode",
		DisplayName:       "Elegoo Logo",
		Status:            model.QueueItemStatusFailed,
		AssignedPrinterID: &printerObj.ID,
		FailedAttempts:    1,
		Notes:             "Start failed: printer rejected upload",
	}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	item.UpdatedAt = time.Now().Add(-5 * time.Second)
	if err := repos.QueueItems.Update(ctx, item); err != nil {
		t.Fatal(err)
	}

	service.handlePrinterStatus(
		&model.PrinterState{PrinterID: printerObj.ID, Status: model.PrinterStatusPrinting, CurrentFile: "gcodes/elegoo_logo.gcode"},
		&model.PrinterState{PrinterID: printerObj.ID, Status: model.PrinterStatusIdle},
	)

	updated, err := repos.QueueItems.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.QueueItemStatusFailed {
		t.Fatalf("expected failed start to remain failed, got %s", updated.Status)
	}
	if updated.FailedAttempts != 1 {
		t.Fatalf("expected failure count to remain recorded, got %d", updated.FailedAttempts)
	}
	if updated.Notes != item.Notes {
		t.Fatalf("expected failure reason to remain visible, got %q", updated.Notes)
	}
}

func TestQueueService_handlePrinterStatus_DoesNotRecoverOldFailedItem(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Macro Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}

	file := &model.File{Hash: "old-failed-hash", OriginalName: "elegoo_logo.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "elegoo_logo.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "elegoo_logo.gcode", DisplayName: "Elegoo Logo", Status: model.QueueItemStatusFailed, AssignedPrinterID: &printerObj.ID, FailedAttempts: 1}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE queue_items SET updated_at = ? WHERE id = ?`, time.Now().Add(-2*queueStartStatusGrace), item.ID); err != nil {
		t.Fatal(err)
	}

	service.handlePrinterStatus(
		&model.PrinterState{PrinterID: printerObj.ID, Status: model.PrinterStatusPrinting, CurrentFile: "elegoo_logo.gcode"},
		&model.PrinterState{PrinterID: printerObj.ID, Status: model.PrinterStatusIdle},
	)

	updated, err := repos.QueueItems.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.QueueItemStatusFailed {
		t.Fatalf("expected old failed item to remain failed, got %s", updated.Status)
	}
}

func TestQueueService_StartAllowsManualRestartAfterStartFailure(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "manual-restart-hash", OriginalName: "manual.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "manual.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "manual.gcode", DisplayName: "Manual", Status: model.QueueItemStatusFailed, StartFailed: true, Notes: "Start failed: printer disconnected"}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	err := service.Start(ctx, item.ID)
	if err == nil || !strings.Contains(err.Error(), "missing printer") {
		t.Fatalf("expected manual restart to reach preflight, got %v", err)
	}
	stored, err := repos.QueueItems.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.QueueItemStatusFailed || stored.Notes != item.Notes {
		t.Fatalf("expected failed start to remain unchanged after preflight rejection, got status=%s notes=%q", stored.Status, stored.Notes)
	}
}

func TestQueueService_StartRejectsRestartAfterPhysicalFailure(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "physical-failure-hash", OriginalName: "failed.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "failed.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "failed.gcode", DisplayName: "Failed", Status: model.QueueItemStatusFailed, Progress: 35, Notes: "Printer error"}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	err := service.Start(ctx, item.ID)
	if err == nil || !strings.Contains(err.Error(), "cannot be started again") {
		t.Fatalf("expected physical failure restart rejection, got %v", err)
	}
	stored, err := repos.QueueItems.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.QueueItemStatusFailed || stored.Progress != 35 {
		t.Fatalf("expected failed item to remain unchanged, got status=%s progress=%v", stored.Status, stored.Progress)
	}
}

func TestQueueService_UpdateNotesPreservesStartFailureMarker(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "preserve-marker-hash", OriginalName: "preserve.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "preserve.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "preserve.gcode", DisplayName: "Preserve", Status: model.QueueItemStatusFailed, StartFailed: true, Notes: "Start failed: disconnected"}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	updated, err := service.Update(ctx, item.ID, QueueCreateOptions{Notes: "Operator changed this note"})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.StartFailed {
		t.Fatal("expected common update to preserve the internal start-failure marker")
	}
}

func TestQueueService_StartRejectsForgedStartFailureNote(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "forged-start-hash", OriginalName: "forged.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "forged.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "forged.gcode", DisplayName: "Forged", Status: model.QueueItemStatusFailed, Notes: "Start failed: forged by update"}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	err := service.Start(ctx, item.ID)
	if err == nil || !strings.Contains(err.Error(), "cannot be started again") {
		t.Fatalf("expected forged start-failure note to be rejected, got %v", err)
	}
}

func TestQueueService_SetStatusRejectsReopeningTerminalItem(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "terminal-status-hash", OriginalName: "terminal.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "terminal.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "terminal.gcode", DisplayName: "Terminal", Status: model.QueueItemStatusFailed}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	for _, status := range []model.QueueItemStatus{model.QueueItemStatusDraft, model.QueueItemStatusQueued, model.QueueItemStatusReady, model.QueueItemStatusBlocked, model.QueueItemStatusPrinting, model.QueueItemStatusPaused, model.QueueItemStatusDone, model.QueueItemStatusCancelled} {
		if err := service.SetStatus(ctx, item.ID, status); err == nil {
			t.Fatalf("expected terminal-to-%s transition to be rejected", status)
		}
	}
}

func TestQueueService_Start_RejectsSecondActiveItemForPrinter(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Single Active Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}

	storagePath, hash, size, err := store.Save("candidate.gcode", strings.NewReader("G28\n"))
	if err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: hash, OriginalName: "candidate.gcode", ContentType: "text/x-gcode", SizeBytes: size, StoragePath: storagePath}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	active := &model.QueueItem{
		SourceType:        model.QueueSourceLibrary,
		FileID:            file.ID,
		FileName:          "active.gcode",
		DisplayName:       "Active",
		Status:            model.QueueItemStatusPrinting,
		AssignedPrinterID: &printerObj.ID,
	}
	if err := repos.QueueItems.Create(ctx, active); err != nil {
		t.Fatal(err)
	}
	candidate := &model.QueueItem{
		SourceType:        model.QueueSourceLibrary,
		FileID:            file.ID,
		FileName:          "candidate.gcode",
		DisplayName:       "Candidate",
		Status:            model.QueueItemStatusQueued,
		AssignedPrinterID: &printerObj.ID,
	}
	if err := repos.QueueItems.Create(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	err = service.Start(ctx, candidate.ID)
	if err == nil {
		t.Fatal("expected second active queue item to be rejected")
	}
	if !strings.Contains(err.Error(), "already has an active queue item") {
		t.Fatalf("expected active queue item error, got %q", err)
	}

	updated, err := repos.QueueItems.GetByID(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.QueueItemStatusQueued {
		t.Fatalf("expected candidate to remain queued, got %s", updated.Status)
	}
}

func TestQueueService_SetStatus_RejectsSecondActiveItemForPrinter(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Single Active Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: "status-active-hash", OriginalName: "status.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "status.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	active := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "active.gcode", DisplayName: "Active", Status: model.QueueItemStatusPrinting, AssignedPrinterID: &printerObj.ID}
	if err := repos.QueueItems.Create(ctx, active); err != nil {
		t.Fatal(err)
	}
	candidate := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "candidate.gcode", DisplayName: "Candidate", Status: model.QueueItemStatusQueued, AssignedPrinterID: &printerObj.ID}
	if err := repos.QueueItems.Create(ctx, candidate); err != nil {
		t.Fatal(err)
	}

	err := service.SetStatus(ctx, candidate.ID, model.QueueItemStatusPrinting)
	if err == nil {
		t.Fatal("expected second active queue item status to be rejected")
	}
	if !strings.Contains(err.Error(), "already has an active queue item") {
		t.Fatalf("expected active queue item error, got %q", err)
	}

	updated, err := repos.QueueItems.GetByID(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.QueueItemStatusQueued {
		t.Fatalf("expected candidate to remain queued, got %s", updated.Status)
	}
}

func TestQueueService_reserveStartedClearsOnlyPreviousStartFailure(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "manual-start-hash", OriginalName: "manual.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "manual.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "manual.gcode", DisplayName: "Manual", Status: model.QueueItemStatusFailed, StartFailed: true, Notes: "Operator note\nStart failed: printer disconnected\nKeep dry"}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	if err := service.reserveStarted(ctx, item); err != nil {
		t.Fatal(err)
	}
	stored, err := repos.QueueItems.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.QueueItemStatusPrinting {
		t.Fatalf("expected manual start to reserve printing, got %s", stored.Status)
	}
	if stored.StartFailed {
		t.Fatal("expected manual start to clear start-failure marker")
	}
	if stored.Notes != "Operator note\nKeep dry" {
		t.Fatalf("expected only previous start failure to be cleared, got %q", stored.Notes)
	}
}

func TestQueueService_rollbackFailedStartPersistsReason(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()
	file := &model.File{Hash: "broken-start-hash", OriginalName: "broken.gcode", ContentType: "text/x-gcode", SizeBytes: 1, StoragePath: "broken.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "broken.gcode", DisplayName: "Broken", Status: model.QueueItemStatusPrinting, Notes: "Operator note\nStart failed: previous error"}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	startErr := errors.New("printer rejected upload")
	err := service.rollbackFailedStart(ctx, item, startErr)
	if !errors.Is(err, startErr) {
		t.Fatalf("expected original start error, got %v", err)
	}
	stored, err := repos.QueueItems.GetByID(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.QueueItemStatusFailed {
		t.Fatalf("expected failed status, got %s", stored.Status)
	}
	if !stored.StartFailed {
		t.Fatal("expected failed start marker to be persisted")
	}
	if stored.Notes != "Operator note\nStart failed: printer rejected upload" {
		t.Fatalf("expected persisted start failure reason without losing operator notes, got %q", stored.Notes)
	}
	if stored.WastedGrams != 0 {
		t.Fatalf("expected failed start not to record material waste, got %v", stored.WastedGrams)
	}
}

func TestQueueService_rollbackFailedStartReportsRollbackError(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Rollback Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: "rollback-error-hash", OriginalName: "rollback.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "rollback.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	item := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "rollback.gcode", DisplayName: "Rollback", Status: model.QueueItemStatusPrinting, AssignedPrinterID: &printerObj.ID}
	if err := repos.QueueItems.Create(ctx, item); err != nil {
		t.Fatal(err)
	}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()
	startErr := fmt.Errorf("printer rejected start")
	err := service.rollbackFailedStart(cancelledCtx, item, startErr)
	if err == nil {
		t.Fatal("expected combined start and rollback error")
	}
	if !errors.Is(err, startErr) {
		t.Fatalf("expected start error to remain discoverable, got %q", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected rollback error to remain discoverable, got %q", err)
	}
	if !strings.Contains(err.Error(), "rollback queue reservation") {
		t.Fatalf("expected rollback context in error, got %q", err)
	}

	stored, getErr := repos.QueueItems.GetByID(ctx, item.ID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if stored.Status != model.QueueItemStatusPrinting {
		t.Fatalf("expected failed rollback to leave persisted reservation visible, got %s", stored.Status)
	}
}

func TestQueueService_Update_RejectsReassigningActiveItemToBusyPrinter(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()

	printerA := &model.Printer{Name: "Printer A"}
	printerB := &model.Printer{Name: "Printer B"}
	if err := repos.Printers.Create(ctx, printerA); err != nil {
		t.Fatal(err)
	}
	if err := repos.Printers.Create(ctx, printerB); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: "reassign-active-hash", OriginalName: "reassign.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "reassign.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	moving := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "moving.gcode", DisplayName: "Moving", Status: model.QueueItemStatusPrinting, AssignedPrinterID: &printerA.ID}
	busy := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "busy.gcode", DisplayName: "Busy", Status: model.QueueItemStatusPaused, AssignedPrinterID: &printerB.ID}
	if err := repos.QueueItems.Create(ctx, moving); err != nil {
		t.Fatal(err)
	}
	if err := repos.QueueItems.Create(ctx, busy); err != nil {
		t.Fatal(err)
	}

	_, err := service.Update(ctx, moving.ID, QueueCreateOptions{AssignedPrinterID: &printerB.ID})
	if err == nil {
		t.Fatal("expected active item reassignment to busy printer to be rejected")
	}
	if !strings.Contains(err.Error(), "already has an active queue item") {
		t.Fatalf("expected active queue item error, got %q", err)
	}

	updated, err := repos.QueueItems.GetByID(ctx, moving.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.AssignedPrinterID == nil || *updated.AssignedPrinterID != printerA.ID {
		t.Fatalf("expected item to remain assigned to printer A, got %v", updated.AssignedPrinterID)
	}
}

func TestQueueService_handlePrinterStatus_DoesNotRecoverFailedItemWhenPrinterAlreadyHasActiveItem(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	service := NewQueueService(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), nil)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Recovery Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: "recovery-conflict-hash", OriginalName: "target.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "target.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	active := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "active.gcode", DisplayName: "Active", Status: model.QueueItemStatusPrinting, AssignedPrinterID: &printerObj.ID}
	failed := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "target.gcode", DisplayName: "Target", Status: model.QueueItemStatusFailed, AssignedPrinterID: &printerObj.ID, FailedAttempts: 1}
	if err := repos.QueueItems.Create(ctx, active); err != nil {
		t.Fatal(err)
	}
	if err := repos.QueueItems.Create(ctx, failed); err != nil {
		t.Fatal(err)
	}

	service.handlePrinterStatus(
		&model.PrinterState{PrinterID: printerObj.ID, Status: model.PrinterStatusPrinting, CurrentFile: "target.gcode"},
		&model.PrinterState{PrinterID: printerObj.ID, Status: model.PrinterStatusIdle},
	)

	updated, err := repos.QueueItems.GetByID(ctx, failed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != model.QueueItemStatusFailed {
		t.Fatalf("expected conflicting failed item to remain failed, got %s", updated.Status)
	}
	if updated.FailedAttempts != 1 {
		t.Fatalf("expected failure count to remain unchanged, got %d", updated.FailedAttempts)
	}
}

func TestQueueRepository_RejectsSecondActiveInsertForPrinter(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Insert Guard Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: "insert-guard-hash", OriginalName: "insert.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "insert.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	first := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "first.gcode", DisplayName: "First", Status: model.QueueItemStatusPrinting, AssignedPrinterID: &printerObj.ID}
	second := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "second.gcode", DisplayName: "Second", Status: model.QueueItemStatusPaused, AssignedPrinterID: &printerObj.ID}
	if err := repos.QueueItems.Create(ctx, first); err != nil {
		t.Fatalf("expected first active insert to succeed: %v", err)
	}
	if err := repos.QueueItems.Create(ctx, second); err == nil {
		t.Fatal("expected database to reject second active insert for printer")
	}
}

func TestQueueRepository_RejectsSecondActiveItemForPrinter(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	ctx := context.Background()

	printerObj := &model.Printer{Name: "Database Guard Printer"}
	if err := repos.Printers.Create(ctx, printerObj); err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: "database-guard-hash", OriginalName: "guard.gcode", ContentType: "text/x-gcode", SizeBytes: 128, StoragePath: "guard.gcode"}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	first := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "first.gcode", DisplayName: "First", Status: model.QueueItemStatusQueued, AssignedPrinterID: &printerObj.ID}
	second := &model.QueueItem{SourceType: model.QueueSourceLibrary, FileID: file.ID, FileName: "second.gcode", DisplayName: "Second", Status: model.QueueItemStatusQueued, AssignedPrinterID: &printerObj.ID}
	if err := repos.QueueItems.Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := repos.QueueItems.Create(ctx, second); err != nil {
		t.Fatal(err)
	}

	first.Status = model.QueueItemStatusPrinting
	if err := repos.QueueItems.Update(ctx, first); err != nil {
		t.Fatalf("expected first item to become active: %v", err)
	}
	second.Status = model.QueueItemStatusPaused
	if err := repos.QueueItems.Update(ctx, second); err == nil {
		t.Fatal("expected database to reject second active item for printer")
	}
}

func TestQueueService_saveThumbnail_reusesExistingWhenSamePath(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, printer.NewManager(), nil)
	ctx := context.Background()

	thumbnail := &gcode.Thumbnail{MimeType: "image/png", Data: []byte("png-data")}
	storagePath, hash, size, err := store.Save("dup-thumbnail.png", bytes.NewReader(thumbnail.Data))
	if err != nil {
		t.Fatal(err)
	}
	file := &model.File{Hash: hash, OriginalName: "dup-thumbnail.png", ContentType: "image/png", SizeBytes: size, StoragePath: storagePath}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}

	id := service.saveThumbnail(ctx, "dup.gcode", thumbnail)
	if id == nil || *id != file.ID {
		t.Fatalf("expected same ID %v, got %v", file.ID, id)
	}
}
