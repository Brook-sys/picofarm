package repository

import (
	"context"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestPrintArchiveRepository_CreateListFilters(t *testing.T) {
	db := openTestDB(t)
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS print_archives (
		id TEXT PRIMARY KEY, job_id TEXT, printer_id TEXT, status TEXT, start_time TEXT, end_time TEXT,
		duration_seconds INTEGER, filament_used_grams REAL, cost_cents INTEGER, thumbnail_file_id TEXT,
		notes TEXT, tags TEXT, created_at TEXT, updated_at TEXT
	)`)
	repo := &PrintArchiveRepository{db: db}

	archive := &model.PrintArchive{Status: "completed", DurationSeconds: 120, FilamentUsedGrams: 12.5, CostCents: 42, Tags: []string{"test"}}
	if err := repo.Create(context.Background(), archive); err != nil {
		t.Fatalf("create: %v", err)
	}

	all, err := repo.List(context.Background(), nil, "")
	if err != nil || len(all) != 1 {
		t.Fatalf("list all: %v len=%d", err, len(all))
	}
	if len(all[0].Tags) != 1 || all[0].Tags[0] != "test" {
		t.Fatalf("tags not round-tripped: %#v", all[0].Tags)
	}

	completed, err := repo.List(context.Background(), nil, "completed")
	if err != nil || len(completed) != 1 {
		t.Fatalf("list completed: %v len=%d", err, len(completed))
	}
	failed, err := repo.List(context.Background(), nil, "failed")
	if err != nil || len(failed) != 0 {
		t.Fatalf("list failed: %v len=%d", err, len(failed))
	}
}
