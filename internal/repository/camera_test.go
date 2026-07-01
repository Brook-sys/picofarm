package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
)

func TestCameraRepository_CRUD(t *testing.T) {
	db := openTestDB(t)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS cameras (
		id TEXT PRIMARY KEY, printer_id TEXT, name TEXT, type TEXT, url TEXT,
		enabled BOOLEAN, token TEXT, token_expires_at TEXT, created_at TEXT, updated_at TEXT
	)`)
	repo := &CameraRepository{db: db}

	c := &model.Camera{Name: "TestCam", Type: "mjpeg", URL: "http://example.com/stream"}
	if err := repo.Create(context.Background(), c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == uuid.Nil {
		t.Error("expected ID assigned")
	}

	list, err := repo.List(context.Background(), nil, nil)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	c.Name = "Updated"
	if err := repo.Update(context.Background(), c); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := repo.Delete(context.Background(), c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
