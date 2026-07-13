package printer

import (
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

func writePrinterTestFile(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "model-*.gcode")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("G28\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func uploadedFilename(t *testing.T, r *http.Request) (string, map[string]string) {
	t.Helper()
	reader, err := r.MultipartReader()
	if err != nil {
		t.Fatalf("multipart reader: %v", err)
	}
	fields := map[string]string{}
	filename := ""
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next multipart part: %v", err)
		}
		data, _ := io.ReadAll(part)
		_, params, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		if remoteName := params["filename"]; remoteName != "" {
			filename = remoteName
			continue
		}
		fields[part.FormName()] = string(data)
	}
	return filename, fields
}

func TestMoonrakerStartJobUploadsAndStartsConfiguredFolder(t *testing.T) {
	localPath := writePrinterTestFile(t)
	var mu sync.Mutex
	var uploadName, startedName string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/server/files/upload":
			name, _ := uploadedFilename(t, r)
			mu.Lock()
			uploadName = name
			mu.Unlock()
			_, _ = io.WriteString(w, `{"result":{"item":{"path":"sda1/model.gcode"}}}`)
		case "/printer/print/start":
			mu.Lock()
			startedName = r.URL.Query().Get("filename")
			mu.Unlock()
			_, _ = io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewMoonrakerClient([16]byte{}, server.URL)
	if err := client.StartJob(PrintRequest{Filename: "model.gcode", LocalPath: localPath, RemoteDirectory: "sda1"}); err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if uploadName != "sda1/model.gcode" {
		t.Fatalf("uploaded filename = %q", uploadName)
	}
	if startedName != "sda1/model.gcode" {
		t.Fatalf("started filename = %q", startedName)
	}
}

func TestOctoPrintStartJobUploadsAndStartsConfiguredFolder(t *testing.T) {
	localPath := writePrinterTestFile(t)
	var uploadPath, uploadName, startedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/files/local" {
			name, fields := uploadedFilename(t, r)
			uploadName = name
			uploadPath = fields["path"]
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/files/local/") {
			startedPath = strings.TrimPrefix(r.URL.Path, "/api/files/local/")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewOctoPrintClient([16]byte{}, server.URL, "key")
	if err := client.StartJob(PrintRequest{Filename: "model.gcode", LocalPath: localPath, RemoteDirectory: "sda1"}); err != nil {
		t.Fatalf("StartJob: %v", err)
	}
	if uploadPath != "sda1" || uploadName != "model.gcode" {
		t.Fatalf("upload path/name = %q/%q", uploadPath, uploadName)
	}
	decoded, err := url.PathUnescape(startedPath)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "sda1/model.gcode" {
		t.Fatalf("started path = %q", decoded)
	}
}
