package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// material_type and material_color are nullable columns, and a row written
// outside the app (a restored backup, a migration, or hand editing) can leave
// them NULL. Scanning NULL into the plain string fields used to fail the whole
// listing with "converting NULL to string is unsupported", taking the entire
// Files screen down for a single bad row. COALESCE in the query makes the
// listing degrade to an empty material instead.
func TestGCodeLibraryRepository_SurvivesNullMaterialColumns(t *testing.T) {
	db := openTestDB(t)
	repo := &GCodeLibraryRepository{db: db}

	fileID := uuid.New()
	if _, err := db.Exec(`INSERT INTO files (id, hash, original_name, content_type, size_bytes, storage_path)
		VALUES (?, 'hash-null-material', 'null-material.gcode', 'text/x-gcode', 10, '/tmp/null-material.gcode')`, fileID); err != nil {
		t.Fatalf("insert file: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO gcode_files (id, file_id, display_name, material_type, material_color)
		VALUES (?, ?, 'legacy row', NULL, NULL)`, uuid.New(), fileID); err != nil {
		t.Fatalf("insert gcode with NULL material columns: %v", err)
	}

	files, total, err := repo.List(context.Background(), GCodeLibraryListOptions{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("List with a NULL material row: %v", err)
	}
	if total != 1 || len(files) != 1 {
		t.Fatalf("expected the row to still be listed, got total=%d len=%d", total, len(files))
	}
	if files[0].MaterialType != "" || files[0].MaterialColor != "" {
		t.Fatalf("expected NULL material columns to degrade to empty strings, got %q/%q", files[0].MaterialType, files[0].MaterialColor)
	}

	// The same columns are read by the per-STL listing used to attach G-codes
	// to their parent model.
	children, err := repo.ListByParentSTL(context.Background(), fileID)
	if err != nil {
		t.Fatalf("ListByParentSTL with a NULL material row: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("expected no children for an unrelated STL, got %d", len(children))
	}
}
