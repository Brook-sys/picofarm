package repository

import (
	"context"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestPrintArchiveRepository_CreateListFilters(t *testing.T) {
	db := openTestDB(t)
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

// TestPrintArchiveRepository_CreateIfAbsentIsIdempotent pins the contract the
// automatic archiving relies on: the same job twice, one row.
func TestPrintArchiveRepository_CreateIfAbsentIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	repo := &PrintArchiveRepository{db: db}
	printJobRepo := &PrintJobRepository{db: db}
	ctx := context.Background()

	_, design := setupPrintJobTestData(t, db)
	job := &model.PrintJob{DesignID: design.ID, Status: model.PrintJobStatusQueued}
	if err := printJobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}

	first := &model.PrintArchive{JobID: &job.ID, Status: "failed", DurationSeconds: 30, Tags: []string{"auto"}}
	inserted, err := repo.CreateIfAbsent(ctx, first)
	if err != nil || !inserted {
		t.Fatalf("first insert: inserted=%v err=%v", inserted, err)
	}

	second := &model.PrintArchive{JobID: &job.ID, Status: "failed", DurationSeconds: 999, Tags: []string{"auto"}}
	inserted, err = repo.CreateIfAbsent(ctx, second)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if inserted {
		t.Error("second insert reported a new row; the unique index on job_id was ignored")
	}

	all, err := repo.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("archives = %d, want 1", len(all))
	}
	if all[0].DurationSeconds != 30 {
		t.Errorf("duration = %d, want 30 (first snapshot kept)", all[0].DurationSeconds)
	}
}

// TestPrintArchiveRepository_ManualArchiveHasNoJobLink covers the other side of
// the contract: entries without a job (manual archives) are never deduplicated.
func TestPrintArchiveRepository_ManualArchiveHasNoJobLink(t *testing.T) {
	db := openTestDB(t)
	repo := &PrintArchiveRepository{db: db}
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		inserted, err := repo.CreateIfAbsent(ctx, &model.PrintArchive{Status: "completed", DurationSeconds: 60})
		if err != nil || !inserted {
			t.Fatalf("insert %d: inserted=%v err=%v", i, inserted, err)
		}
	}

	all, err := repo.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("archives = %d, want 2 manual entries", len(all))
	}
	for _, a := range all {
		if a.JobID != nil {
			t.Errorf("job_id = %v, want nil for a manual archive", a.JobID)
		}
		if len(a.Tags) != 0 {
			t.Errorf("tags = %v, want empty (nil serialized as [])", a.Tags)
		}
	}
}
