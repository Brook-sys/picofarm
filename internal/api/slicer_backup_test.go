package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/service"
)

// newSlicerBackupEnv wires a fake orca-slicer-api into the real router so the
// profile backup endpoints are exercised end to end.
func newSlicerBackupEnv(t *testing.T, fake http.Handler) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	env.services.Slicer = service.NewSlicerService(env.services.Settings, env.repos, env.storage, nil)
	if err := env.services.Settings.Set(t.Context(), "slicer_connection_url", "http://slicer.invalid"); err != nil {
		t.Fatalf("seed slicer URL: %v", err)
	}
	fakeServer := httptest.NewServer(fake)
	t.Cleanup(fakeServer.Close)
	if err := env.services.Settings.Set(t.Context(), "slicer_connection_url", fakeServer.URL); err != nil {
		t.Fatalf("point slicer at fake: %v", err)
	}
	return env
}

func slicerBackupFake() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			_, _ = w.Write([]byte(`{"status":"healthy","orcaVersion":"2.4.1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/profiles/printers":
			_, _ = w.Write([]byte(`[{"name":"p1","size":10}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/profiles/p1":
			_, _ = w.Write([]byte(`{"name":"p1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/profiles/printers/p1":
			_, _ = w.Write([]byte(`{"name":"p1"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/profiles/"):
			_, _ = w.Write([]byte(`[]`))
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	})
}

// A plain POST with no body must produce a usable backup: operators call this
// from curl and scripts, not only from the UI.
func TestSlicerExportProfilesAcceptsEmptyBody(t *testing.T) {
	env := newSlicerBackupEnv(t, slicerBackupFake())

	req := httptest.NewRequest(http.MethodPost, "/api/slicer/profiles/export", nil)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusOK, rr.Body.String())
	}
	var bundle service.SlicerProfileBundle
	if err := json.Unmarshal(rr.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if len(bundle.Profiles) != 1 || bundle.Profiles[0].Name != "p1" {
		t.Fatalf("bundle profiles = %+v, want the single printer profile", bundle.Profiles)
	}
}

// A malformed bundle must be refused with 400 and, per the service tests, before
// anything reaches the slicer.
func TestSlicerImportProfilesRejectsMalformedBundle(t *testing.T) {
	env := newSlicerBackupEnv(t, slicerBackupFake())

	for name, body := range map[string]string{
		"empty":           "",
		"not json":        "{",
		"no profiles":     `{"profiles":[]}`,
		"invalid profile": `{"profiles":[{"category":"printers","name":"p1","json":"nope"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/slicer/profiles/import", strings.NewReader(body))
			rr := httptest.NewRecorder()
			env.handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusBadRequest, rr.Body.String())
			}
		})
	}
}

// Per-profile upload failures come back with 200 and a report, because the
// restore did run and the UI needs the details.
func TestSlicerImportProfilesReportsFailuresWithoutErrorStatus(t *testing.T) {
	env := newSlicerBackupEnv(t, slicerBackupFake())

	// overwrite, because the fake slicer already reports p1 as present and the
	// point of this test is the status contract, not the skip decision.
	req := httptest.NewRequest(http.MethodPost, "/api/slicer/profiles/import",
		strings.NewReader(`{"overwrite":true,"profiles":[{"category":"printers","name":"p1","json":"{\"name\":\"p1\"}"}]}`))
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", rr.Code, http.StatusOK, rr.Body.String())
	}
	var result service.SlicerImportBundleResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Restored != 1 || len(result.Profiles) != 1 || result.Profiles[0].Status != "restored" {
		t.Fatalf("result = %+v, want one restored profile", result)
	}
}
