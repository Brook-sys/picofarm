package appruntime

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStaticEnvironmentFallback(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("environment-static"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STATIC_DIR", root)
	cfg := testConfig(t)
	cfg.StaticDir = "" // existing API callers do not need to populate the new option
	app, err := Start(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	response, err := (&http.Client{Timeout: time.Second}).Get(app.URL() + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "environment-static" {
		t.Fatalf("STATIC_DIR fallback: %q", body)
	}
	if os.Getenv("STATIC_DIR") != root {
		t.Fatal("runtime mutated process environment")
	}
}
