package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

func TestSlicerJobsHandlerValidation(t *testing.T) {
	env := newTestEnv(t)
	slicer := service.NewSlicerService(env.services.Settings, env.repos, env.storage, nil)
	jobs, err := service.NewSlicerJobsService(context.Background(), env.services.Settings, slicer)
	if err != nil {
		t.Fatal(err)
	}
	defer jobs.Close()
	handler := NewSlicerJobsHandler(jobs)
	router := chi.NewRouter()
	router.Post("/jobs", handler.Submit)
	router.Get("/jobs", handler.List)
	router.Get("/jobs/{id}", handler.Get)
	router.Post("/jobs/{id}/cancel", handler.Cancel)
	router.Delete("/jobs/{id}", handler.Delete)
	cases := []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/jobs", "", 200},
		{"GET", "/jobs/missing", "", 404},
		{"POST", "/jobs/missing/cancel", "", 404},
		{"DELETE", "/jobs/missing", "", 404},
		{"POST", "/jobs", "{}", 400},
		{"POST", "/jobs", "{", 400},
		{"POST", "/jobs", `{"unknown":1}`, 400},
		{"POST", "/jobs", `{} {}`, 400},
		{"POST", "/jobs", strings.Repeat("x", (64<<10)+1), 400},
		{"POST", "/jobs", `{"stl_file_id":"00000000-0000-0000-0000-000000000001"}`, 400}, // missing key
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path+tc.body[:min(12, len(tc.body))], func(t *testing.T) {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if rr.Code != tc.status {
				t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(`{"stl_file_id":"00000000-0000-0000-0000-000000000001"}`))
	req.Header.Set("Idempotency-Key", "test-job")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}
