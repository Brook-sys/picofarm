package database

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

func openRawTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	db := openTestDB(t)

	if err := RunMigrations(db); err != nil {
		t.Fatalf("first migration run: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second migration run should tolerate existing compatibility columns: %v", err)
	}
}

func TestRunMigrationsRejectsPreexistingActiveQueueConflicts(t *testing.T) {
	db := openRawTestDB(t)
	if _, err := db.Exec(`
		CREATE TABLE queue_items (
			id TEXT PRIMARY KEY,
			assigned_printer_id TEXT,
			status TEXT NOT NULL
		);
		INSERT INTO queue_items (id, assigned_printer_id, status) VALUES
			('first', 'printer-1', 'printing'),
			('second', 'printer-1', 'paused');
	`); err != nil {
		t.Fatalf("seed legacy conflict: %v", err)
	}

	err := RunMigrations(db)
	if err == nil {
		t.Fatal("expected migration to reject duplicate active queue items")
	}
	if !strings.Contains(err.Error(), "printer printer-1 has 2 active queue items") {
		t.Fatalf("expected actionable active queue conflict, got: %v", err)
	}
	var triggerCount int
	if scanErr := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND name LIKE 'trg_queue_items_single_active_%'`).Scan(&triggerCount); scanErr != nil {
		t.Fatal(scanErr)
	}
	if triggerCount != 0 {
		t.Fatalf("expected no guards to be installed over inconsistent data, got %d triggers", triggerCount)
	}
}

func TestRunMigrationsReportsUnexpectedCompatibilityFailures(t *testing.T) {
	db := openRawTestDB(t)

	if err := RunMigrations(db); err != nil {
		t.Fatalf("initial migration run: %v", err)
	}
	if _, err := db.Exec(`ALTER TABLE gcode_files RENAME COLUMN material_type TO material_type_broken`); err != nil {
		t.Fatalf("corrupt gcode_files schema: %v", err)
	}

	err := RunMigrations(db)
	if err == nil {
		t.Fatal("expected migration error for incompatible gcode_files schema")
	}
	if !strings.Contains(err.Error(), "normalize gcode file material metadata") {
		t.Fatalf("expected statement context in error, got: %v", err)
	}
}
