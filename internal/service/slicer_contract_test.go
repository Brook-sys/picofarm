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
			_, _ = w.Write([]byte(`{"usesSupport":false,"printTime":120,"filamentUsedG":2,"filamentUsedMm":700,"thumbnail":"","quantity":3,"toolpaths":{"version":2,"mode":"full","geometryComplete":true,"sourceComplete":true,"warnings":["bounded"],"limits":{"segments":250000},"bounds":{"min":[0,1,2],"max":[3,4,5]},"layers":[{"index":0,"z":0.2,"segments":[{"from":[0,0,0.2],"to":[1,2,0.2],"extrusion":0.3,"feedrateMmSec":20,"widthMm":0.42,"heightMm":null,"feature":"support","object":"copy 1","tool":2}]}]}}`))
		} else {
			// Stop before importing an artifact: this test exercises the outbound contract.
			http.Error(w, "synthetic rejection", http.StatusUnprocessableEntity)
		}
	}))
	defer server.Close()
	t.Setenv(SlicerConnectionURLEnv, server.URL)
	overrides := map[string]map[string]any{"preset": {"layer_height": "0.24"}}
	zero, angle := 0.0, -45.5
	preview, err := s.PreviewSTL(ctx, SlicerPreviewRequest{Quantity: 3, Arrange: true, Rotate: &angle, RotateX: &zero, STLFileID: stl.ID, Printer: "machine", Preset: "process", Filament: "filament", BrimType: true, PrintSequenceByObject: true, Overrides: overrides})
	if err != nil {
		t.Fatal(err)
	}
	if preview.PrintTime != 120 {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Quantity != 3 || preview.Toolpaths == nil || preview.Toolpaths.Layers[0].Segments[0].Object != "copy 1" || preview.Toolpaths.Bounds.Max[2] != 5 || preview.Toolpaths.Layers[0].Segments[0].Width == nil || *preview.Toolpaths.Layers[0].Segments[0].Width != 0.42 || preview.Toolpaths.Layers[0].Segments[0].Height != nil {
		t.Fatalf("preview geometry lost: %+v", preview)
	}
	encodedPreview, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encodedPreview), `"heightMm":null`) || !strings.Contains(string(encodedPreview), `"geometryComplete":true`) || preview.Toolpaths.Version != 2 || preview.Toolpaths.Mode != "full" {
		t.Fatalf("full preview metadata lost in proxy JSON: %s", encodedPreview)
	}
	_, err = s.SliceSTL(ctx, SlicerSliceRequest{Quantity: 3, Arrange: true, Rotate: &angle, RotateX: &zero, STLFileID: stl.ID, Printer: "machine", Preset: "process", Filament: "filament", BrimType: true, PrintSequenceByObject: true, Overrides: overrides})
	if err == nil || !strings.Contains(err.Error(), "422") {
		t.Fatalf("expected synthetic rejection, got %v", err)
	}
	sliceForm := forms["/slice"]
	for key, want := range map[string]string{"quantity": "3", "rotate": "-45.5", "rotateX": "0"} {
		if sliceForm[key][0] != want {
			t.Errorf("%s = %v, want %s", key, sliceForm[key], want)
		}
	}
	if _, exists := sliceForm["rotateY"]; exists {
		t.Error("absent rotation must stay absent")
	}
	previewForm := forms["/slice/preview"]
	if got := previewForm["toolpathMode"]; !reflect.DeepEqual(got, []string{"full"}) {
		t.Errorf("toolpathMode = %v, want full", got)
	}
	delete(previewForm, "toolpathMode")
	if !reflect.DeepEqual(previewForm, sliceForm) {
		t.Errorf("preview/slice differ: %v / %v", previewForm, sliceForm)
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
