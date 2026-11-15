package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSlicerEditorPinsDestinationAcrossConfigChange(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)
	fake := newFakeSlicer()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/profiles/presets/q1" {
			once.Do(func() { close(entered); <-release })
		}
		fake.ServeHTTP(w, r)
	}))
	defer a.Close()
	var mu sync.Mutex
	callsB := 0
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callsB++
		mu.Unlock()
		http.Error(w, "wrong destination", 500)
	}))
	defer b.Close()
	t.Setenv(SlicerConnectionURLEnv, "")
	if _, err := s.SetConfig(ctx, SlicerConfig{ConnectionURL: a.URL}); err != nil {
		t.Fatal(err)
	}
	hash, _ := slicerSemanticHash(`{"name":"q1","speed":"120"}`)
	done := make(chan error, 1)
	go func() {
		_, err := s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: `{"name":"q1","speed":"150"}`, ExpectedHash: hash})
		done <- err
	}()
	<-entered
	if _, err := s.SetConfig(ctx, SlicerConfig{ConnectionURL: b.URL}); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("save deadlocked")
	}
	mu.Lock()
	defer mu.Unlock()
	if callsB != 0 || fake.uploadCount() != 1 {
		t.Fatalf("destination changed mid-operation: B=%d uploadsA=%d", callsB, fake.uploadCount())
	}
}

func TestSlicerMutationsSerializeWithEditor(t *testing.T) {
	for _, kind := range []string{"upload", "import-url", "source", "bundle"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s := newSlicerTestService(t)
			entered := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			var mu sync.Mutex
			current := `{"name":"q1","speed":"120"}`
			writes := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					mu.Lock()
					raw := current
					mu.Unlock()
					once.Do(func() { close(entered); <-release })
					io.WriteString(w, raw)
					return
				}
				var raw string
				if strings.HasSuffix(r.URL.Path, "/upload") {
					if err := r.ParseMultipartForm(2 << 20); err != nil {
						t.Error(err)
						return
					}
					file, _, err := r.FormFile("file")
					if err != nil {
						t.Error(err)
						return
					}
					defer file.Close()
					data, _ := io.ReadAll(file)
					raw = string(data)
				} else {
					raw = `{"name":"q1","speed":"200"}`
				}
				mu.Lock()
				current = raw
				writes = append(writes, raw)
				mu.Unlock()
				json.NewEncoder(w).Encode(map[string]bool{"ok": true})
			}))
			defer server.Close()
			t.Setenv(SlicerConnectionURLEnv, server.URL)
			hash, _ := slicerSemanticHash(current)
			editorDone := make(chan error, 1)
			mutationDone := make(chan error, 1)
			proposal := `{"name":"q1","speed":"150"}`
			go func() {
				_, err := s.SaveProfileRevision(ctx, SlicerSaveProfileRequest{Category: "presets", Name: "q1", JSON: proposal, ExpectedHash: hash})
				editorDone <- err
			}()
			<-entered
			started := make(chan struct{})
			go func() {
				close(started)
				var err error
				switch kind {
				case "upload":
					_, err = s.UploadProfileJSON(ctx, SlicerUploadProfileRequest{Category: "presets", Name: "q1", JSON: `{"name":"q1","speed":"200"}`})
				case "import-url":
					_, err = s.ImportProfile(ctx, SlicerImportRequest{Category: "presets", Name: "q1", URL: "https://example.com/profile.json"})
				case "source":
					_, err = s.UpdateProfileFromSource(ctx, "presets", "q1")
				case "bundle":
					_, err = s.ImportProfiles(ctx, SlicerImportBundleRequest{Overwrite: true, Profiles: []SlicerBundleProfile{{Category: "presets", Name: "q1", JSON: `{"name":"q1","speed":"200"}`}}})
				}
				mutationDone <- err
			}()
			<-started
			close(release)
			for _, done := range []chan error{editorDone, mutationDone} {
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("mutation deadlocked")
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if len(writes) != 2 || writes[0] != proposal {
				t.Fatalf("mutation interleaved with editor preimage: %v", writes)
			}
			history, err := s.settings.repo.ListSlicerRevisions(ctx, func() string { v, _ := s.profileHistorySource(ctx); return v }(), "presets", "q1")
			if err != nil {
				t.Fatal(err)
			}
			if len(history) != 2 || history[1].JSON != `{"name":"q1","speed":"120"}` {
				t.Fatalf("lost preimage: %+v", history)
			}
		})
	}
}
