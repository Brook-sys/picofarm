package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSettingsHidesAndProtectsSlicerJobs(t *testing.T) {
	env := newTestEnv(t)
	const snapshot = `[{"id":"private-job"}]`
	if err := env.services.Settings.Set(t.Context(), "slicer_jobs_v1", snapshot); err != nil {
		t.Fatal(err)
	}
	if err := env.services.Settings.Set(t.Context(), "ordinary_setting", "visible"); err != nil {
		t.Fatal(err)
	}
	h := &SettingsHandler{service: env.services.Settings}
	router := chi.NewRouter()
	router.Get("/settings", h.List)
	router.Get("/settings/{key}", h.Get)
	router.Put("/settings/{key}", h.Set)
	router.Delete("/settings/{key}", h.Delete)
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(method, "/settings/slicer_jobs_v1", strings.NewReader(`{"value":"overwritten"}`)))
		if rr.Code != 403 {
			t.Fatalf("%s status=%d body=%s", method, rr.Code, rr.Body.String())
		}
	}
	saved, err := env.services.Settings.Get(t.Context(), "slicer_jobs_v1")
	if err != nil || saved == nil || saved.Value != snapshot {
		t.Fatalf("snapshot modified: %+v %v", saved, err)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest("GET", "/settings", nil))
	if rr.Code != 200 || strings.Contains(rr.Body.String(), "slicer_jobs_v1") || strings.Contains(rr.Body.String(), "private-job") || !strings.Contains(rr.Body.String(), "ordinary_setting") {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest("GET", "/settings/ordinary_setting", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
}
