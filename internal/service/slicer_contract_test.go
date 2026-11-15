package service

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/storage"
)

func TestSlicerRejectsUnsupportedArtifactBeforeLookup(t *testing.T) {
	s := newSlicerTestService(t)
	if _, err := s.SliceSTL(context.Background(), SlicerSliceRequest{ExportType: "3mf"}); err == nil || !strings.Contains(err.Error(), "only imports gcode") {
		t.Fatalf("expected unsupported artifact error, got %v", err)
	}
}

func TestSlicerOptionsExplicitBooleanContract(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		writeSlicerOptions(writer, enabled, enabled, enabled, enabled, enabled, enabled)
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/slice", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		if err := req.ParseMultipartForm(1024); err != nil {
			t.Fatal(err)
		}
		defer req.MultipartForm.RemoveAll()
		want := "false"
		if enabled {
			want = "true"
		}
		for _, key := range []string{"arrange", "orient", "enableSupport", "brimType", "printSequenceByObject", "multicolorOnePlate"} {
			if got := req.FormValue(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		for _, legacy := range []string{"brim_type", "print_sequence_by_object"} {
			if _, exists := req.MultipartForm.Value[legacy]; exists {
				t.Errorf("unexpected legacy upstream key %s", legacy)
			}
		}
	}
}

func TestSlicerPreviewAndSliceForwardSameOptions(t *testing.T) {
	ctx := context.Background()
	s := newSlicerTestService(t)
	store := storage.NewLocalStorage(t.TempDir())
	s.storage = store
	path, hash, size, err := store.Save("synthetic.stl", strings.NewReader("solid synthetic\nendsolid synthetic\n"))
	if err != nil {
		t.Fatal(err)
	}
	file := &model.File{OriginalName: "synthetic.stl", StoragePath: path, Hash: hash, SizeBytes: size, ContentType: "model/stl"}
	if err := s.files.Create(ctx, file); err != nil {
		t.Fatal(err)
	}
	stl := &model.STLLibraryFile{FileID: file.ID, DisplayName: "Synthetic"}
	if err := s.stls.Create(ctx, stl); err != nil {
		t.Fatal(err)
	}
	forms := map[string]map[string][]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		defer r.MultipartForm.RemoveAll()
		forms[r.URL.Path] = r.MultipartForm.Value
		uploaded, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		uploaded.Close()
		if header.Filename != "synthetic.stl" {
			t.Errorf("filename = %q", header.Filename)
		}
		if r.URL.Path == "/slice/preview" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"usesSupport":false,"printTime":120,"filamentUsedG":2,"filamentUsedMm":700,"thumbnail":""}`))
		} else {
			// Stop before importing an artifact: this test exercises the outbound contract.
			http.Error(w, "synthetic rejection", http.StatusUnprocessableEntity)
		}
	}))
	defer server.Close()
	t.Setenv(SlicerConnectionURLEnv, server.URL)
	overrides := map[string]map[string]any{"preset": {"layer_height": "0.24"}}
	preview, err := s.PreviewSTL(ctx, SlicerPreviewRequest{STLFileID: stl.ID, Printer: "machine", Preset: "process", Filament: "filament", BrimType: true, PrintSequenceByObject: true, Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	if preview.PrintTime != 120 {
		t.Fatalf("preview = %+v", preview)
	}
	_, err = s.SliceSTL(ctx, SlicerSliceRequest{STLFileID: stl.ID, Printer: "machine", Preset: "process", Filament: "filament", BrimType: true, PrintSequenceByObject: true, Overrides: overrides})
	if err == nil || !strings.Contains(err.Error(), "422") {
		t.Fatalf("expected synthetic rejection, got %v", err)
	}
	sliceForm := forms["/slice"]
	delete(sliceForm, "exportType")
	if !reflect.DeepEqual(forms["/slice/preview"], sliceForm) {
		t.Errorf("preview/slice differ: %v / %v", forms["/slice/preview"], sliceForm)
	}
	var decoded map[string]map[string]any
	if err := json.Unmarshal([]byte(sliceForm["overrides"][0]), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, overrides) {
		t.Errorf("overrides = %v", decoded)
	}
	if sliceForm["enableSupport"][0] != "false" {
		t.Error("disabled support must be explicit")
	}
}
