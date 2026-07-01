package repository

import (
	"context"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestTimelapseRepository_CreateList(t *testing.T) {
	db := openTestDB(t)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS timelapses (
		id TEXT PRIMARY KEY, printer_id TEXT, camera_id TEXT, print_job_id TEXT, status TEXT,
		frames_path TEXT, video_path TEXT, frame_count INTEGER, started_at TEXT, completed_at TEXT,
		created_at TEXT, updated_at TEXT
	)`)
	repo := &TimelapseRepository{db: db}

	item := &model.Timelapse{Status: "capturing", FramesPath: "/tmp/frames", FrameCount: 3}
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatalf("create: %v", err)
	}
	items, err := repo.List(context.Background(), nil)
	if err != nil || len(items) != 1 {
		t.Fatalf("list: %v len=%d", err, len(items))
	}
	if items[0].Status != "capturing" || items[0].FrameCount != 3 {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}
