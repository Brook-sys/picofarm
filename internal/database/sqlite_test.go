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
