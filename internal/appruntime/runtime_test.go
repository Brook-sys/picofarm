package appruntime

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/service"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	dir := t.TempDir()
	return Config{
		ListenAddress: "127.0.0.1:0", DatabasePath: filepath.Join(dir, "picofarm.db"),
		UploadDir: filepath.Join(dir, "uploads"), StaticDir: filepath.Join(dir, "static"),
		ReconnectConfig: service.DefaultPrinterReconnectConfig(), ShutdownTimeout: time.Second,
	}
}

func TestRuntimeLifecycle(t *testing.T) {
	// An explicit static root must win over the host process environment.
	t.Setenv("STATIC_DIR", filepath.Join(t.TempDir(), "not-the-static-root"))
	cfg := testConfig(t)
	if err := os.MkdirAll(cfg.StaticDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.StaticDir, "index.html"), []byte("runtime-static"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app, err := Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Shutdown(context.Background()) })
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(app.URL() + "/health")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health: %d", response.StatusCode)
	}
	response, err = client.Get(app.URL() + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || string(body) != "runtime-static" {
		t.Fatalf("static override: %q, %v", body, readErr)
	}
	cancel()
	select {
	case <-app.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("context cancellation did not stop runtime")
	}
	if err := app.Wait(); err != nil {
		t.Fatal(err)
	}
	// Repeated and concurrent cleanup must not close service stop channels twice.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := app.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	// All DB/service ownership has been released, allowing a fresh runtime.
	next, err := Start(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := next.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestBindFailureHasNoDatabaseSideEffects(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg := testConfig(t)
	cfg.ListenAddress = listener.Addr().String()
	if app, err := Start(context.Background(), cfg); err == nil || app != nil {
		t.Fatalf("Start = %v, %v", app, err)
	}
	if _, err := os.Stat(cfg.DatabasePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database touched before bind: %v", err)
	}
}

func TestInvalidStartup(t *testing.T) {
	cfg := testConfig(t)
	cfg.ReconnectConfig.Interval = time.Millisecond
	if _, err := Start(context.Background(), cfg); err == nil {
		t.Fatal("accepted invalid reconnect config")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Start(ctx, testConfig(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled startup: %v", err)
	}
	cfg = testConfig(t)
	cfg.DatabasePath = t.TempDir() // opening a directory as SQLite must fail
	if _, err := Start(context.Background(), cfg); err == nil {
		t.Fatal("accepted invalid database path")
	}
}

func TestConfigFromEnv(t *testing.T) {
	for key, value := range map[string]string{
		"PORT": "", "DATABASE_PATH": filepath.Join(t.TempDir(), "db"), "UPLOAD_DIR": "", "STATIC_DIR": "",
		"PRINTER_RECONNECT_INTERVAL": "10s", "PRINTER_RECONNECT_MAX_CONCURRENT": "4",
	} {
		t.Setenv(key, value)
	}
	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddress != ":8084" || cfg.UploadDir != "./uploads" || cfg.StaticDir != "./web/dist" {
		t.Fatalf("defaults: %+v", cfg)
	}
	t.Setenv("PORT", "9010")
	t.Setenv("UPLOAD_DIR", "custom-uploads")
	t.Setenv("STATIC_DIR", "custom-static")
	cfg, err = ConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddress != ":9010" || cfg.UploadDir != "custom-uploads" || cfg.StaticDir != "custom-static" {
		t.Fatalf("overrides: %+v", cfg)
	}
	t.Setenv("PRINTER_RECONNECT_INTERVAL", "bad")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("accepted invalid reconnect environment")
	}
}
