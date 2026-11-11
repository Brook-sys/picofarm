package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// insertTerminalJobWithPhases inserts a completed print job plus its phase_changed
// events using raw SQL, so the test controls completed_at/occurred_at exactly — the
// window filter under test depends on those timestamps.
func insertTerminalJobWithPhases(t *testing.T, db *sql.DB, jobID, designID, printerID uuid.UUID, start, printingStart, completedAt time.Time) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO print_jobs (id, design_id, printer_id, status, started_at, completed_at, attempt_number)
		VALUES (?, ?, ?, 'completed', ?, ?, 1)
	`, jobID.String(), designID.String(), printerID.String(), start.Format(time.RFC3339), completedAt.Format(time.RFC3339)); err != nil {
		t.Fatalf("insert print_jobs: %v", err)
	}

	phases := []struct {
		phase      string
		occurredAt time.Time
	}{
		{"Heating Bed", start},
		{"Homing", start.Add(5 * time.Minute)},
		{"Printing", printingStart},
	}
	for _, p := range phases {
		metadata := `{"new_phase":"` + p.phase + `"}`
		if _, err := db.Exec(`
			INSERT INTO job_events (id, job_id, event_type, occurred_at, metadata)
			VALUES (?, ?, 'phase_changed', ?, ?)
		`, uuid.New().String(), jobID.String(), p.occurredAt.Format(time.RFC3339), metadata); err != nil {
			t.Fatalf("insert job_events: %v", err)
		}
	}
}

func newTestPrinter(t *testing.T, repo *PrinterRepository, ctx context.Context, name string) *model.Printer {
	t.Helper()
	printer := &model.Printer{
		Name:           name,
		Model:          "Test",
		Manufacturer:   "Test",
		ConnectionType: "octoprint",
		ConnectionURI:  "http://localhost",
	}
	if err := repo.Create(ctx, printer); err != nil {
		t.Fatalf("create printer %q: %v", name, err)
	}
	return printer
}

// TestPrinterRepository_GetPrinterPreparationRecords covers the real repository path,
// which the unit test in internal/service cannot reach because it injects records
// directly. JobName used to be a truncated UUID there, so the contract is asserted here.
func TestPrinterRepository_GetPrinterPreparationRecords(t *testing.T) {
	db := openTestDB(t)
	repo := &PrinterRepository{db: db}
	ctx := context.Background()

	_, design := setupPrintJobTestData(t, db)
	printer := newTestPrinter(t, repo, ctx, "Preparation Test Printer")

	now := time.Now().UTC()
	// Two jobs inside the window.
	insertTerminalJobWithPhases(t, db, uuid.New(), design.ID, printer.ID, now.Add(-2*time.Hour), now.Add(-110*time.Minute), now.Add(-40*time.Minute))
	insertTerminalJobWithPhases(t, db, uuid.New(), design.ID, printer.ID, now.Add(-5*time.Hour), now.Add(-5*time.Hour+10*time.Minute), now.Add(-4*time.Hour))

	// One job outside the window, on another printer: must be invisible in both
	// the denominator and the event rows.
	other := newTestPrinter(t, repo, ctx, "Other Printer")
	oldStart := now.AddDate(0, 0, -400)
	insertTerminalJobWithPhases(t, db, uuid.New(), design.ID, other.ID, oldStart, oldStart.Add(10*time.Minute), oldStart.Add(time.Hour))

	records, totalJobs, err := repo.GetPrinterPreparationRecords(ctx, printer.ID)
	if err != nil {
		t.Fatalf("GetPrinterPreparationRecords: %v", err)
	}

	// The denominator must count only in-window jobs for this printer, otherwise
	// data_coverage_percent is inflated by jobs the numerator can never measure.
	if totalJobs != 2 {
		t.Errorf("totalJobs = %d, want 2 (in-window jobs for this printer only)", totalJobs)
	}
	if len(records) != 6 {
		t.Fatalf("len(records) = %d, want 6 phase events across the 2 in-window jobs", len(records))
	}

	for _, r := range records {
		// The contract: JobName is the design file name, not a UUID prefix.
		if r.JobName != design.FileName {
			t.Errorf("JobName = %q, want design file name %q", r.JobName, design.FileName)
		}
	}

	// A terminal job that never set completed_at must not contribute events. Its
	// absence from the denominator is what keeps the coverage percentage meaningful.
	noEndJob := uuid.New()
	if _, err := db.Exec(`
		INSERT INTO print_jobs (id, design_id, printer_id, status, started_at, completed_at, attempt_number)
		VALUES (?, ?, ?, 'cancelled', ?, NULL, 1)
	`, noEndJob.String(), design.ID.String(), printer.ID.String(), now.Add(-3*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("insert job without completed_at: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO job_events (id, job_id, event_type, occurred_at, metadata)
		VALUES (?, ?, 'phase_changed', ?, '{"new_phase":"Heating Bed"}')
	`, uuid.New().String(), noEndJob.String(), now.Add(-3*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("insert event for job without completed_at: %v", err)
	}

	records, totalJobs, err = repo.GetPrinterPreparationRecords(ctx, printer.ID)
	if err != nil {
		t.Fatalf("GetPrinterPreparationRecords after adding job without completed_at: %v", err)
	}
	if totalJobs != 2 {
		t.Errorf("totalJobs = %d after adding a cancelled job with no completed_at, want 2", totalJobs)
	}
	if len(records) != 6 {
		t.Errorf("len(records) = %d, want 6 (job without completed_at must not contribute events)", len(records))
	}
}
