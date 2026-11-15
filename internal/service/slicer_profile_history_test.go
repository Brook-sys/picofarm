package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlicerProfilePreflight(t *testing.T) {
	cases := []struct {
		raw, category string
		valid         bool
	}{
		{`null`, "presets", false}, {`[]`, "presets", false}, {`{}`, "presets", false},
		{`{"name":"q","type":"filament"}`, "presets", false},
		{`{"name":"q","inherits":false}`, "presets", false},
		{`{"name":"q","inherits":"q"}`, "presets", false},
		{`{"name":"q","layer_height":"-1"}`, "presets", true},
		{`{"name":"q","layer_height":"-2"}`, "presets", false},
		{`{"name":"q","nozzle_temperature":["501"]}`, "filaments", false},
		{`{"name":"q","nozzle_diameter":[]}`, "printers", false},
		{`{"name":"q","layer_height":"0.2","inherits":"parent"}`, "presets", true},
		{`{"name":"q","nozzle_temperature":["200",210]}`, "filaments", true},
		{`{"name":"q"}`, "unknown", false},
		{strings.Repeat(" ", SlicerProfileJSONLimit+1), "presets", false},
	}
	for _, tc := range cases {
		t.Run(tc.category+tc.raw[:min(len(tc.raw), 40)], func(t *testing.T) {
			v := ValidateSlicerProfile(SlicerUploadProfileRequest{Category: tc.category, JSON: tc.raw})
			if v.Valid != tc.valid {
				t.Fatalf("got %+v", v)
			}
		})
	}
}

func TestSlicerEditorSnapshotsConflictRestoreAndUploadFailure(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)
	fake := newFakeSlicer()
	server := httptest.NewServer(fake)
	defer server.Close()
	t.Setenv(SlicerConnectionURLEnv, server.URL)
	initial, err := s.ProfileHistory(ctx, "presets", "q1")
	if err != nil {
		t.Fatal(err)
	}
	proposed := `{"name":"q1","layer_height":"0.3"}`
	saved, err := s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: proposed, ExpectedHash: initial.CurrentHash})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Revisions) != 2 {
		t.Fatalf("expected preimage and proposal: %+v", saved)
	}
	if saved.Revisions[1].JSON != initial.CurrentJSON {
		t.Fatal("preimage lost")
	}
	_, err = s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: proposed, ExpectedHash: initial.CurrentHash})
	if !errors.Is(err, ErrSlicerProfileConflict) {
		t.Fatalf("expected conflict: %v", err)
	}
	restored, err := s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: saved.Revisions[1].JSON, ExpectedHash: saved.CurrentHash})
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Revisions) != 4 || restored.CurrentHash != initial.CurrentHash {
		t.Fatal("restore must append, not rewrite history")
	}
	fake.mu.Lock()
	fake.failingUpload = "q1"
	fake.mu.Unlock()
	_, err = s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: proposed, ExpectedHash: restored.CurrentHash})
	if err == nil {
		t.Fatal("expected upload failure")
	}
	failed, err := s.ProfileHistory(ctx, "presets", "q1")
	if err != nil {
		t.Fatal(err)
	}
	if len(failed.Revisions) != 6 || failed.CurrentHash != restored.CurrentHash {
		t.Fatal("failure lost snapshots or changed current profile")
	}
	if fake.uploadCount() != 2 {
		t.Fatalf("unexpected uploads: %d", fake.uploadCount())
	}
}

func TestSlicerSemanticHashIgnoresWhitespaceAndKeyOrder(t *testing.T) {
	a, _ := slicerSemanticHash(`{"a":1,"b":2}`)
	b, _ := slicerSemanticHash("{\n\"b\":2,\"a\":1}")
	if a != b {
		t.Fatal("formatting alone must not conflict")
	}
}
