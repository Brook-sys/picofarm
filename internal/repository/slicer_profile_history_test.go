package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSlicerRecoveryStorageImmutableBoundedAndIsolated(t *testing.T) {
	db := openTestDB(t)
	r := &SettingsRepository{db: db}
	ctx := context.Background()
	raw := `{"name":"q"}`
	if err := r.AppendSlicerRevision(ctx, "source-a", "presets", "q", raw); err != nil {
		t.Fatal(err)
	}
	if err := r.AppendSlicerRevision(ctx, "source-a", "presets", "q", raw); err != nil {
		t.Fatal(err)
	}
	rows, err := r.ListSlicerRevisions(ctx, "source-a", "presets", "q")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(raw))
	if len(rows) != 2 || rows[0].ID <= rows[1].ID || rows[0].Hash != hex.EncodeToString(digest[:]) || rows[1].JSON != raw {
		t.Fatalf("snapshots altered: %+v", rows)
	}
	other, err := r.ListSlicerRevisions(ctx, "source-b", "presets", "q")
	if err != nil || len(other) != 0 {
		t.Fatal("source isolation failed")
	}
	if err := r.AppendSlicerRevision(ctx, "source-a", "presets", "q", strings.Repeat("x", 1<<20+1)); err == nil {
		t.Fatal("oversized snapshot accepted")
	}
	// A valid first snapshot followed by an invalid proposal must roll back both.
	if err := r.AppendSlicerRevisions(ctx, "source-a", "presets", "q", raw, strings.Repeat("x", 1<<20+1)); err == nil {
		t.Fatal("invalid pair accepted")
	}
	var pairCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM slicer_profile_history`).Scan(&pairCount); err != nil || pairCount != 2 {
		t.Fatal("partial pair committed")
	}
	// Artificially fill quota without consuming memory in the test process.
	if _, err := db.Exec(`UPDATE slicer_profile_history SET json=CAST(zeroblob(33554428) AS TEXT)`); err != nil {
		t.Fatal(err)
	}
	// Four bytes fit individually, but the eight-byte pair must be atomic.
	if err := r.AppendSlicerRevisions(ctx, "source-a", "presets", "q", "12345", "67890"); err == nil {
		t.Fatal("over-quota pair accepted")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM slicer_profile_history`).Scan(&pairCount); err != nil || pairCount != 2 {
		t.Fatal("quota failure committed a partial pair")
	}
	if _, err := db.Exec(`UPDATE slicer_profile_history SET json=CAST(zeroblob(33554432) AS TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := r.AppendSlicerRevision(ctx, "source-a", "presets", "q", raw); err == nil {
		t.Fatal("quota exceeded without refusal")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM slicer_profile_history`).Scan(&count); err != nil || count != 2 {
		t.Fatal("quota refusal deleted or appended a snapshot")
	}
}
