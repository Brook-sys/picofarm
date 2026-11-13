package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/repository"
)

func newSlicerTestService(t *testing.T) *SlicerService {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repos := repository.NewRepositories(db)
	return NewSlicerService(&SettingsService{repo: repos.Settings}, repos, nil, nil)
}

// A deployment must be able to pin the slicer address through the environment:
// without that, whatever an older version wrote in settings silently wins over
// the container configuration.
func TestSlicerConnectionURLPrefersEnvironmentOverSetting(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)

	if err := s.settings.Set(ctx, "slicer_connection_url", "http://from-settings:8080"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	t.Setenv(SlicerConnectionURLEnv, "http://from-env:1234/")
	urlValue, source, err := s.effectiveConnectionURL(ctx)
	if err != nil {
		t.Fatalf("effectiveConnectionURL: %v", err)
	}
	if urlValue != "http://from-env:1234" {
		t.Fatalf("effective URL = %q, want the environment value (trailing slash trimmed)", urlValue)
	}
	if source != "env" {
		t.Fatalf("source = %q, want env", source)
	}

	cfg, err := s.GetConfig(ctx)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.ConnectionURL != "http://from-env:1234" || cfg.ConnectionURLSource != "env" {
		t.Fatalf("config = %q/%q, want the environment URL reported as coming from env", cfg.ConnectionURL, cfg.ConnectionURLSource)
	}
}

func TestSlicerConnectionURLFallsBackToSettingThenEmpty(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)

	urlValue, source, err := s.effectiveConnectionURL(ctx)
	if err != nil {
		t.Fatalf("effectiveConnectionURL with nothing configured: %v", err)
	}
	if urlValue != "" || source != "" {
		t.Fatalf("expected an empty URL with no source, got %q/%q", urlValue, source)
	}

	if err := s.settings.Set(ctx, "slicer_connection_url", "http://slicer.local/"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	urlValue, source, err = s.effectiveConnectionURL(ctx)
	if err != nil {
		t.Fatalf("effectiveConnectionURL: %v", err)
	}
	if urlValue != "http://slicer.local" || source != "settings" {
		t.Fatalf("expected the stored URL with source settings, got %q/%q", urlValue, source)
	}
}

// A malformed environment value must be reported instead of silently falling
// back to a stale setting, which would make the deployment look healthy while
// pointing at the wrong slicer.
func TestSlicerConnectionURLRejectsMalformedEnvironmentValue(t *testing.T) {
	s := newSlicerTestService(t)
	ctx := context.Background()

	if err := s.settings.Set(ctx, "slicer_connection_url", "http://from-settings:8080"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	t.Setenv(SlicerConnectionURLEnv, "slicer-without-scheme")

	if _, _, err := s.effectiveConnectionURL(ctx); err == nil {
		t.Fatal("expected an error for a malformed environment URL")
	}
}

// The URL is still stored while the environment overrides it, so removing the
// environment variable brings the stored value back rather than an empty config.
// SetConfig must also report the effective config: echoing the request back
// told the caller their edit took effect when the environment still won.
func TestSlicerSetConfigPersistsWhileEnvironmentOverrides(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)
	t.Setenv(SlicerConnectionURLEnv, "http://from-env:1234")

	effective, err := s.SetConfig(ctx, SlicerConfig{ConnectionURL: "http://stored:9000/"})
	if err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	if effective.ConnectionURL != "http://from-env:1234" || effective.ConnectionURLSource != "env" {
		t.Fatalf("SetConfig returned %q/%q, want the environment URL reported as coming from env", effective.ConnectionURL, effective.ConnectionURLSource)
	}
	stored, err := s.settings.Get(ctx, "slicer_connection_url")
	if err != nil {
		t.Fatalf("read setting: %v", err)
	}
	if stored == nil || stored.Value != "http://stored:9000" {
		t.Fatalf("stored value = %v, want http://stored:9000", stored)
	}
}

// Health and the other UI-facing calls must fail in seconds. With a single
// 60 minute client, a slicer that never answered kept the request (and the
// browser tab) waiting for an hour.
func TestSlicerQuickCallsFailFastOnUnresponsiveSlicer(t *testing.T) {
	release := make(chan struct{})
	hanging := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		hanging.Close()
	})

	s := newSlicerTestService(t)
	ctx := context.Background()
	if err := s.settings.Set(ctx, "slicer_connection_url", hanging.URL); err != nil {
		t.Fatalf("seed setting: %v", err)
	}

	start := time.Now()
	if _, err := s.Health(ctx); err == nil {
		t.Fatal("expected health to fail against an unresponsive slicer")
	}
	elapsed := time.Since(start)
	if elapsed > slicerTimeoutQuick+2*time.Second {
		t.Fatalf("health took %v, expected the quick deadline (%v) to bite", elapsed, slicerTimeoutQuick)
	}
}

// The three tiers must actually be distinct clients; a single shared client is
// the bug this guards against.
func TestSlicerServiceUsesDistinctDeadlines(t *testing.T) {
	s := newSlicerTestService(t)
	if s.quickClient.Timeout != slicerTimeoutQuick {
		t.Fatalf("quick client timeout = %v, want %v", s.quickClient.Timeout, slicerTimeoutQuick)
	}
	if s.profileClient.Timeout != slicerTimeoutProfile {
		t.Fatalf("profile client timeout = %v, want %v", s.profileClient.Timeout, slicerTimeoutProfile)
	}
	if s.sliceClient.Timeout != slicerTimeoutSlice {
		t.Fatalf("slice client timeout = %v, want %v", s.sliceClient.Timeout, slicerTimeoutSlice)
	}
}
