package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Brook-sys/picofarm/internal/gcode"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
)

func TestQueueService_saveThumbnail_repairsMissingPhysicalFile(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, nil, nil)
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

func TestQueueService_saveThumbnail_reusesExistingWhenSamePath(t *testing.T) {
	db, _ := openFileTestDB(t)
	repos := repository.NewRepositories(db)
	store := storage.NewLocalStorage(t.TempDir())
	service := NewQueueService(repos, store, nil, nil)
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
