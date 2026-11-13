package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeSlicer is a stand-in for the orca-slicer-api container. It serves the
// handful of routes the export/import flow touches and records every upload so
// the tests can assert on what was really sent.
type fakeSlicer struct {
	mu sync.Mutex
	// profiles maps category -> profile name -> raw JSON.
	profiles map[string]map[string]string
	// failingRead names a profile whose read answers with a server error.
	failingRead string
	// failingUpload names a profile whose upload is rejected.
	failingUpload string
	uploads       []uploadedProfile
}

type uploadedProfile struct {
	Category string
	Name     string
	JSON     string
}

func newFakeSlicer() *fakeSlicer {
	return &fakeSlicer{profiles: map[string]map[string]string{
		"printers":  {"p1": `{"name":"p1","layer_height":"0.2"}`},
		"presets":   {"q1": `{"name":"q1","speed":"120"}`},
		"filaments": {},
	}}
}

func (f *fakeSlicer) uploadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.uploads)
}

func (f *fakeSlicer) uploaded() []uploadedProfile {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]uploadedProfile(nil), f.uploads...)
}

func (f *fakeSlicer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		writeJSON(w, http.StatusOK, map[string]any{"status": "healthy", "orcaVersion": "2.4.1"})
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/profiles/"):
		rest := strings.TrimPrefix(r.URL.Path, "/profiles/")
		parts := strings.SplitN(rest, "/", 2)
		category := parts[0]
		if len(parts) == 1 {
			names := make([]map[string]any, 0, len(f.profiles[category]))
			for name := range f.profiles[category] {
				names = append(names, map[string]any{"name": name, "size": len(f.profiles[category][name])})
			}
			writeJSON(w, http.StatusOK, names)
			return
		}
		name := parts[1]
		if name == f.failingRead {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "boom"})
			return
		}
		body, ok := f.profiles[category][name]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "Profile not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/upload"):
		category := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/profiles/"), "/upload")
		name, body, err := readUpload(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": err.Error()})
			return
		}
		if name == f.failingUpload {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "profile rejected"})
			return
		}
		if f.profiles[category] == nil {
			f.profiles[category] = map[string]string{}
		}
		f.profiles[category][name] = body
		f.uploads = append(f.uploads, uploadedProfile{Category: category, Name: name, JSON: body})
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "not found"})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// readUpload pulls the profile name and file content out of a multipart upload.
func readUpload(r *http.Request) (string, string, error) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return "", "", fmt.Errorf("invalid multipart form")
	}
	reader := multipart.NewReader(r.Body, params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return "", "", fmt.Errorf("invalid multipart form")
		}
		if err != nil {
			return "", "", fmt.Errorf("invalid multipart form")
		}
		content, err := io.ReadAll(part)
		if err != nil {
			return "", "", err
		}
		if part.FormName() == "name" {
			name := string(content)
			// The file part follows the name field.
			filePart, err := reader.NextPart()
			if err != nil {
				return "", "", fmt.Errorf("invalid multipart form")
			}
			fileContent, err := io.ReadAll(filePart)
			if err != nil {
				return "", "", err
			}
			return name, string(fileContent), nil
		}
	}
}

func newSlicerBackupTestService(t *testing.T, fake *fakeSlicer) (*SlicerService, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	s := newSlicerTestService(t)
	if err := s.settings.Set(context.Background(), "slicer_connection_url", server.URL); err != nil {
		t.Fatalf("seed slicer URL: %v", err)
	}
	return s, server
}

// The slicer has no export endpoint, so the bundle has to be composed from the
// profile list plus one read per profile. The raw JSON must survive the round
// trip byte for byte, otherwise a restore pushes a re-serialized profile.
func TestSlicerExportProfilesComposesBundleFromListAndReads(t *testing.T) {
	ctx := context.Background()
	fake := newFakeSlicer()
	s, server := newSlicerBackupTestService(t, fake)

	bundle, err := s.ExportProfiles(ctx, SlicerExportRequest{})
	if err != nil {
		t.Fatalf("ExportProfiles: %v", err)
	}
	if bundle.Version != slicerProfileBundleVersion {
		t.Fatalf("bundle version = %d, want %d", bundle.Version, slicerProfileBundleVersion)
	}
	if bundle.ExportedAt == "" {
		t.Fatal("bundle has no exported_at timestamp")
	}
	if bundle.Source != server.URL {
		t.Fatalf("bundle source = %q, want %q", bundle.Source, server.URL)
	}
	if bundle.SlicerVersion != "2.4.1" {
		t.Fatalf("bundle slicer version = %q, want 2.4.1", bundle.SlicerVersion)
	}
	if len(bundle.Profiles) != 2 {
		t.Fatalf("bundle has %d profiles, want 2", len(bundle.Profiles))
	}
	byKey := map[string]SlicerBundleProfile{}
	for _, profile := range bundle.Profiles {
		byKey[profile.Category+"/"+profile.Name] = profile
	}
	printer, ok := byKey["printers/p1"]
	if !ok {
		t.Fatal("bundle is missing the printers/p1 profile")
	}
	if printer.JSON != fake.profiles["printers"]["p1"] {
		t.Fatalf("bundle JSON = %q, want the exact profile bytes", printer.JSON)
	}
	if _, ok := byKey["presets/q1"]; !ok {
		t.Fatal("bundle is missing the presets/q1 profile")
	}
}

// An unknown category must fail loudly. Exporting nothing looks exactly like an
// empty slicer, and that is how a backup silently stops covering a profile.
func TestSlicerExportProfilesRejectsUnknownCategory(t *testing.T) {
	s, _ := newSlicerBackupTestService(t, newFakeSlicer())

	if _, err := s.ExportProfiles(context.Background(), SlicerExportRequest{Categories: []string{"machines"}}); err == nil {
		t.Fatal("expected an error for an unknown category")
	}
}

// A partial export is worse than no export: the bundle would look complete and
// get treated as a backup during a restore.
func TestSlicerExportProfilesFailsWhenAProfileReadFails(t *testing.T) {
	ctx := context.Background()
	fake := newFakeSlicer()
	fake.failingRead = "p1"
	s, _ := newSlicerBackupTestService(t, fake)

	if _, err := s.ExportProfiles(ctx, SlicerExportRequest{}); err == nil {
		t.Fatal("expected the export to fail when a profile cannot be read")
	} else if !strings.Contains(err.Error(), "p1") {
		t.Fatalf("error = %q, want it to name the profile that failed", err)
	}
}

// Restoring must not destroy a working profile that already exists unless the
// operator explicitly asked for it.
func TestSlicerImportProfilesSkipsExistingWithoutOverwrite(t *testing.T) {
	ctx := context.Background()
	fake := newFakeSlicer()
	s, _ := newSlicerBackupTestService(t, fake)

	result, err := s.ImportProfiles(ctx, SlicerImportBundleRequest{Profiles: []SlicerBundleProfile{
		{Category: "printers", Name: "p1", JSON: `{"name":"p1","layer_height":"0.3"}`},
		{Category: "printers", Name: "p2", JSON: `{"name":"p2","layer_height":"0.2"}`},
	}})
	if err != nil {
		t.Fatalf("ImportProfiles: %v", err)
	}
	if result.Restored != 1 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("result = %d restored / %d skipped / %d failed, want 1/1/0", result.Restored, result.Skipped, result.Failed)
	}
	uploads := fake.uploaded()
	if len(uploads) != 1 {
		t.Fatalf("slicer received %d uploads, want 1", len(uploads))
	}
	if uploads[0].Name != "p2" {
		t.Fatalf("uploaded %q, want p2 (p1 must be skipped)", uploads[0].Name)
	}
	if uploads[0].JSON != `{"name":"p2","layer_height":"0.2"}` {
		t.Fatalf("uploaded JSON = %q, want the bundle bytes", uploads[0].JSON)
	}
	if fake.profiles["printers"]["p1"] != `{"name":"p1","layer_height":"0.2"}` {
		t.Fatal("the existing profile was overwritten even though overwrite was not requested")
	}
}

func TestSlicerImportProfilesOverwritesWhenAsked(t *testing.T) {
	ctx := context.Background()
	fake := newFakeSlicer()
	s, _ := newSlicerBackupTestService(t, fake)

	result, err := s.ImportProfiles(ctx, SlicerImportBundleRequest{Overwrite: true, Profiles: []SlicerBundleProfile{
		{Category: "printers", Name: "p1", JSON: `{"name":"p1","layer_height":"0.3"}`},
	}})
	if err != nil {
		t.Fatalf("ImportProfiles: %v", err)
	}
	if result.Restored != 1 || result.Skipped != 0 {
		t.Fatalf("result = %d restored / %d skipped, want 1/0", result.Restored, result.Skipped)
	}
	if fake.profiles["printers"]["p1"] != `{"name":"p1","layer_height":"0.3"}` {
		t.Fatal("the existing profile was not overwritten")
	}
}

// A malformed bundle must be rejected before a single upload: half a restore is
// worse than none, because the operator believes it ran.
func TestSlicerImportProfilesRejectsInvalidBundleBeforeUploading(t *testing.T) {
	cases := map[string][]SlicerBundleProfile{
		"invalid json":     {{Category: "printers", Name: "p1", JSON: "not json"}},
		"missing name":     {{Category: "printers", Name: "  ", JSON: `{"name":"p1"}`}},
		"missing json":     {{Category: "printers", Name: "p1", JSON: ""}},
		"unknown category": {{Category: "machines", Name: "p1", JSON: `{"name":"p1"}`}},
		"duplicate entry": {
			{Category: "printers", Name: "p1", JSON: `{"name":"p1"}`},
			{Category: "printers", Name: "p1", JSON: `{"name":"p1"}`},
		},
	}
	for name, profiles := range cases {
		t.Run(name, func(t *testing.T) {
			fake := newFakeSlicer()
			s, _ := newSlicerBackupTestService(t, fake)

			if _, err := s.ImportProfiles(context.Background(), SlicerImportBundleRequest{Profiles: profiles}); err == nil {
				t.Fatal("expected the bundle to be rejected")
			}
			if fake.uploadCount() != 0 {
				t.Fatalf("slicer received %d uploads, want 0", fake.uploadCount())
			}
		})
	}
}

// One rejected profile must not hide the ones that worked: the result reports
// every profile individually.
func TestSlicerImportProfilesReportsPerProfileFailure(t *testing.T) {
	ctx := context.Background()
	fake := newFakeSlicer()
	fake.failingUpload = "bad"
	s, _ := newSlicerBackupTestService(t, fake)

	result, err := s.ImportProfiles(ctx, SlicerImportBundleRequest{Profiles: []SlicerBundleProfile{
		{Category: "printers", Name: "good", JSON: `{"name":"good"}`},
		{Category: "printers", Name: "bad", JSON: `{"name":"bad"}`},
	}})
	if err != nil {
		t.Fatalf("ImportProfiles returned an error instead of a per-profile report: %v", err)
	}
	if result.Restored != 1 || result.Failed != 1 {
		t.Fatalf("result = %d restored / %d failed, want 1/1", result.Restored, result.Failed)
	}
	statuses := map[string]string{}
	for _, profile := range result.Profiles {
		statuses[profile.Name] = profile.Status
	}
	if statuses["good"] != "restored" || statuses["bad"] != "failed" {
		t.Fatalf("statuses = %v, want good=restored and bad=failed", statuses)
	}
}

// The category list is the one thing an operator types by hand, so the common
// aliases must work and duplicates must collapse. Anything else is rejected with
// the offending name, because exporting nothing looks like an empty slicer.
func TestResolveSlicerCategoriesNormalizesAliasesAndDropsDuplicates(t *testing.T) {
	resolved, err := resolveSlicerCategories([]string{"printer", "printers", "filament"})
	if err != nil {
		t.Fatalf("resolveSlicerCategories: %v", err)
	}
	want := []string{"printers", "filaments"}
	if len(resolved) != len(want) {
		t.Fatalf("resolved = %v, want %v", resolved, want)
	}
	for i, category := range want {
		if resolved[i] != category {
			t.Fatalf("resolved = %v, want %v", resolved, want)
		}
	}
	all, err := resolveSlicerCategories(nil)
	if err != nil {
		t.Fatalf("resolveSlicerCategories with no categories: %v", err)
	}
	if len(all) != len(slicerProfileCategories) {
		t.Fatalf("default categories = %v, want %v", all, slicerProfileCategories)
	}
}
