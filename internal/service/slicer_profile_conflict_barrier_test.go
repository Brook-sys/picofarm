package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSlicerEditorConflictsAfterInflightUpload(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	current := `{"name":"q1","speed":"120"}`
	oldHash, _ := slicerSemanticHash(current)
	uploads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			mu.Lock()
			defer mu.Unlock()
			io.WriteString(w, current)
			return
		}
		close(entered)
		<-release
		mu.Lock()
		uploads++
		current = `{"name":"q1","speed":"200"}`
		mu.Unlock()
		io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	t.Setenv(SlicerConnectionURLEnv, server.URL)
	uploadDone := make(chan error, 1)
	go func() {
		_, err := s.UploadProfileJSON(ctx, SlicerUploadProfileRequest{Category: "presets", Name: "q1", JSON: `{"name":"q1","speed":"200"}`})
		uploadDone <- err
	}()
	<-entered
	editorDone := make(chan error, 1)
	go func() {
		_, err := s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: `{"name":"q1","speed":"300"}`, ExpectedHash: oldHash})
		editorDone <- err
	}()
	close(release)
	select {
	case err := <-uploadDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("upload deadlocked")
	}
	select {
	case err := <-editorDone:
		if !errors.Is(err, ErrSlicerProfileConflict) {
			t.Fatalf("expected conflict after completed upload: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("editor deadlocked")
	}
	mu.Lock()
	defer mu.Unlock()
	if uploads != 1 {
		t.Fatal("editor overwrote concurrent upload")
	}
}
