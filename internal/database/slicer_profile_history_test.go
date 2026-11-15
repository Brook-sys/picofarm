package database

import (
	"path/filepath"
	"testing"
)

func TestSlicerHistoryFreshAndUpgrade(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM slicer_profile_history`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("fresh table: %d %v", count, err)
	}
	if _, err := db.Exec(`DROP TABLE slicer_profile_history`); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO slicer_profile_history(source_hash,category,name,hash,json) VALUES('source','presets','q','hash','{}')`); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM slicer_profile_history`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("upgrade discarded snapshots: %d %v", count, err)
	}
}
