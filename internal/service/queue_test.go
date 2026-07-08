package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

func TestQueueService_handlePrinterStatus_RecoversFalseFailedWhenPrinterStartsFile(t *testing.T) {
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
		WastedGrams:       0.0032,
		Notes:             "Cancelled on printer",
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
	if updated.Status != model.QueueItemStatusPrinting {
		t.Fatalf("expected false failed item to be recovered as printing, got %s", updated.Status)
	}
	if updated.Progress != 0 {
		t.Fatalf("expected progress reset to 0, got %v", updated.Progress)
	}
	if updated.FailedAttempts != 0 {
		t.Fatalf("expected false failure count to be reverted, got %d", updated.FailedAttempts)
	}
	if updated.WastedGrams != 0 {
		t.Fatalf("expected false waste to be reverted, got %v", updated.WastedGrams)
	}
	if updated.Notes != "" {
		t.Fatalf("expected false cancellation note to be cleared, got %q", updated.Notes)
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
