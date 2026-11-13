package service

import (
	"context"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

// newArchiveTestService builds a PrintJobService backed by a real in-memory
// database, wiring only the collaborators the archiving path needs.
func newArchiveTestService(t *testing.T) (*PrintJobService, *repository.Repositories, *model.PrintJob) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repos := repository.NewRepositories(db)
	svc := &PrintJobService{
		repo:        repos.PrintJobs,
		printerRepo: repos.Printers,
		archiveRepo: repos.PrintArchives,
		hub:         realtime.NewHub(),
	}

	ctx := context.Background()
	design := createArchiveTestDesign(t, repos)
	// 1 cent per second of printer time, so cost assertions stay readable.
	printer := &model.Printer{Name: "P1", Model: "X1", ConnectionType: "bambu", CostPerHourCents: 3600}
	if err := repos.Printers.Create(ctx, printer); err != nil {
		t.Fatalf("create printer: %v", err)
	}

	now := time.Now()
	job := &model.PrintJob{
		DesignID:  design.ID,
		PrinterID: &printer.ID,
		Status:    model.PrintJobStatusPrinting,
		StartedAt: &now,
		Notes:     "benchy",
	}
	if err := repos.PrintJobs.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	return svc, repos, job
}

// createArchiveTestDesign creates the parent chain print_jobs requires
// (project -> part -> file -> design).
func createArchiveTestDesign(t *testing.T, repos *repository.Repositories) *model.Design {
	t.Helper()
	ctx := context.Background()

	project := &model.Project{Name: "Archive Test Project"}
	if err := repos.Projects.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	part := &model.Part{ProjectID: project.ID, Name: "Archive Test Part"}
	if err := repos.Parts.Create(ctx, part); err != nil {
		t.Fatalf("create part: %v", err)
	}
	// files.hash is UNIQUE, so each call needs its own hash.
	hash := uuid.NewString()
	file := &model.File{
		Hash:         hash,
		OriginalName: "archive.3mf",
		ContentType:  "application/3mf",
		SizeBytes:    2048,
		StoragePath:  "ar/ch/archivehash/archive.3mf",
	}
	if err := repos.Files.Create(ctx, file); err != nil {
		t.Fatalf("create file: %v", err)
	}
	design := &model.Design{
		PartID:        part.ID,
		FileName:      "archive.3mf",
		FileID:        file.ID,
		FileHash:      file.Hash,
		FileSizeBytes: file.SizeBytes,
		FileType:      "3mf",
	}
	if err := repos.Designs.Create(ctx, design); err != nil {
		t.Fatalf("create design: %v", err)
	}
	return design
}

// TestArchiveJobSnapshotsTerminalJob walks the real RecordFailure path and
// checks that the archive carries the job's timeline and metrics.
func TestArchiveJobSnapshotsTerminalJob(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	startedAt := time.Now().Add(-10 * time.Minute)
	job.StartedAt = &startedAt
	grams := 12.5
	cost := 420
	job.MaterialUsedGrams = &grams
	job.CostCents = &cost
	if err := svc.repo.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := svc.RecordFailure(ctx, job.ID, model.FailureMechanical, "JAM", "extruder jam"); err != nil {
		t.Fatalf("record failure: %v", err)
	}

	archives, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %d, want 1", len(archives))
	}
	a := archives[0]
	if a.Status != string(model.PrintJobStatusFailed) {
		t.Errorf("status = %q, want failed", a.Status)
	}
	if a.JobID == nil || *a.JobID != job.ID {
		t.Errorf("job_id = %v, want %v", a.JobID, job.ID)
	}
	if a.PrinterID == nil || *a.PrinterID != *job.PrinterID {
		t.Errorf("printer_id = %v, want %v", a.PrinterID, *job.PrinterID)
	}
	// RecordFailure stamps CompletedAt with "now", so the fallback duration is
	// the span between the job's start and the failure detection.
	if a.DurationSeconds < 590 || a.DurationSeconds > 610 {
		t.Errorf("duration = %d, want ~600 (start/end fallback)", a.DurationSeconds)
	}
	if a.FilamentUsedGrams != 12.5 || a.CostCents != 420 {
		t.Errorf("metrics = %v g / %d cents, want 12.5 g / 420 cents", a.FilamentUsedGrams, a.CostCents)
	}
	if a.StartTime == nil || a.EndTime == nil {
		t.Errorf("start/end not carried over: %v / %v", a.StartTime, a.EndTime)
	}
	if len(a.Tags) != 1 || a.Tags[0] != "auto" {
		t.Errorf("tags = %v, want [auto]", a.Tags)
	}
	if a.Notes != "benchy" {
		t.Errorf("notes = %q, want benchy", a.Notes)
	}
}

// TestRecordOutcomeArchivesCompletedJob covers the success path, where costs
// are computed by the service and must reach the archive.
func TestRecordOutcomeArchivesCompletedJob(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	actualTime := 600
	err := svc.RecordOutcome(ctx, job.ID, &model.PrintOutcome{
		Success:      true,
		ActualTime:   &actualTime,
		MaterialUsed: 0,
	})
	if err != nil {
		t.Fatalf("record outcome: %v", err)
	}

	archives, err := repos.PrintArchives.List(ctx, nil, string(model.PrintJobStatusCompleted))
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %d, want 1 completed archive", len(archives))
	}
	// Printer cost is 1 cent/second: 600s -> 600 cents, with no material cost
	// (no spool assigned).
	if archives[0].CostCents != 600 {
		t.Errorf("cost = %d, want 600", archives[0].CostCents)
	}
	if archives[0].DurationSeconds != 600 {
		t.Errorf("duration = %d, want 600", archives[0].DurationSeconds)
	}
}

// TestArchiveJobIsIdempotent re-archives the same job and expects a single row.
func TestArchiveJobIsIdempotent(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	if err := svc.RecordFailure(ctx, job.ID, model.FailureUnknown, "ERR", "boom"); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	// Repeated archiving attempts (backfill, retries) must not duplicate rows.
	svc.ArchiveJob(ctx, job.ID)
	svc.ArchiveJob(ctx, job.ID)

	archives, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %d, want 1 (job must not be archived twice)", len(archives))
	}
}

// TestArchiveJobSkipsNonTerminalJob makes sure mid-flight jobs never leak into
// the archive just because something called the archiver.
func TestArchiveJobSkipsNonTerminalJob(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)

	svc.ArchiveJob(context.Background(), job.ID)

	archives, err := repos.PrintArchives.List(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 0 {
		t.Fatalf("archives = %d, want 0 for a non-terminal job", len(archives))
	}
}

// TestArchiveJobPrefersActualSeconds checks that a recorded duration wins over
// the wall-clock span.
func TestArchiveJobPrefersActualSeconds(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	startedAt := time.Now().Add(-2 * time.Hour)
	actual := 111
	job.StartedAt = &startedAt
	job.ActualSeconds = &actual
	if err := svc.repo.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := svc.RecordFailure(ctx, job.ID, model.FailureUnknown, "ERR", "boom"); err != nil {
		t.Fatalf("record failure: %v", err)
	}

	archives, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %d, want 1", len(archives))
	}
	if archives[0].DurationSeconds != 111 {
		t.Errorf("duration = %d, want 111 (actual_seconds wins)", archives[0].DurationSeconds)
	}
}

// TestCancelArchivesJob exercises the cancellation transition end to end.
func TestCancelArchivesJob(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	// PrinterID is nil so Cancel skips the printer manager, which a unit test
	// cannot construct.
	job.PrinterID = nil
	if err := svc.repo.Update(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	if err := svc.Cancel(ctx, job.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	archives, err := repos.PrintArchives.List(ctx, nil, string(model.PrintJobStatusCancelled))
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %d, want 1 cancelled archive", len(archives))
	}
	if archives[0].EndTime == nil {
		t.Error("cancelled archive has no end time")
	}
}

// TestManualArchiveSurvivesAutoArchive guards the unique index contract: manual
// entries (no job link) must not collide with an automatic one.
func TestManualArchiveSurvivesAutoArchive(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	if err := svc.RecordFailure(ctx, job.ID, model.FailureUnknown, "ERR", "boom"); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	manual := &model.PrintArchive{Status: "completed", DurationSeconds: 60, Tags: []string{"manual"}}
	if _, err := repos.PrintArchives.CreateIfAbsent(ctx, manual); err != nil {
		t.Fatalf("manual archive: %v", err)
	}

	archives, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 2 {
		t.Fatalf("archives = %d, want 2 (auto + manual)", len(archives))
	}
}

// TestBackfillArchivesHistoricalJobs simulates jobs that reached a terminal
// status before the archiving producer existed (terminal event, no archive) and
// checks that the startup backfill picks them up exactly once.
func TestBackfillArchivesHistoricalJobs(t *testing.T) {
	svc, repos, _ := newArchiveTestService(t)
	ctx := context.Background()

	// Two jobs closed by an event only, bypassing the service transitions that
	// now archive automatically.
	var jobIDs []uuid.UUID
	for _, status := range []model.PrintJobStatus{model.PrintJobStatusCompleted, model.PrintJobStatusFailed} {
		job := &model.PrintJob{DesignID: designIDFrom(t, repos), Status: model.PrintJobStatusPrinting}
		if err := repos.PrintJobs.Create(ctx, job); err != nil {
			t.Fatalf("create job: %v", err)
		}
		terminal := status
		event := model.NewJobEvent(job.ID, model.JobEventCompleted, &terminal)
		if err := repos.PrintJobs.AppendEvent(ctx, event); err != nil {
			t.Fatalf("append terminal event: %v", err)
		}
		jobIDs = append(jobIDs, job.ID)
	}

	// A third job that is still printing must not be backfilled.
	active := &model.PrintJob{DesignID: designIDFrom(t, repos), Status: model.PrintJobStatusPrinting}
	if err := repos.PrintJobs.Create(ctx, active); err != nil {
		t.Fatalf("create active job: %v", err)
	}

	svc.backfillArchives()

	archives, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 2 {
		t.Fatalf("archives = %d, want 2 (terminal jobs backfilled, active job skipped)", len(archives))
	}
	archived := make(map[string]bool, len(archives))
	for _, a := range archives {
		if a.JobID == nil {
			t.Fatal("backfilled archive has no job link")
		}
		archived[a.JobID.String()] = true
	}
	for _, id := range jobIDs {
		if !archived[id.String()] {
			t.Errorf("historical job %s was not backfilled", id)
		}
	}
	if archived[active.ID.String()] {
		t.Error("active job was archived")
	}

	// Second run must be a no-op.
	svc.backfillArchives()
	again, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(again) != 2 {
		t.Fatalf("archives = %d after second backfill, want 2", len(again))
	}
}

// designIDFrom returns a valid design id for a job fixture, creating the parent
// chain once per call set. Job creation needs a real design FK.
func designIDFrom(t *testing.T, repos *repository.Repositories) uuid.UUID {
	t.Helper()
	return createArchiveTestDesign(t, repos).ID
}

// TestArchiveSurvivesJobDeletion pins the ON DELETE SET NULL contract: history
// must outlive the job it snapshotted, and deleting a job must never fail.
func TestArchiveSurvivesJobDeletion(t *testing.T) {
	svc, repos, job := newArchiveTestService(t)
	ctx := context.Background()

	if err := svc.RecordFailure(ctx, job.ID, model.FailureUnknown, "ERR", "boom"); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	if err := repos.PrintJobs.Delete(ctx, job.ID); err != nil {
		t.Fatalf("delete job: %v", err)
	}

	archives, err := repos.PrintArchives.List(ctx, nil, "")
	if err != nil {
		t.Fatalf("list archives: %v", err)
	}
	if len(archives) != 1 {
		t.Fatalf("archives = %d, want the archive to survive job deletion", len(archives))
	}
	if archives[0].JobID != nil {
		t.Errorf("job_id = %v, want nil after job deletion", archives[0].JobID)
	}
}
