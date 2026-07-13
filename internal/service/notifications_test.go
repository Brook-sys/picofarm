package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostMultipartSendsAttachmentAndCaptionTogether(t *testing.T) {
	t.Parallel()

	var gotCaption string
	var gotFilename string
	var gotData string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		gotCaption = r.FormValue("caption")
		file, header, err := r.FormFile("photo")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		gotFilename = header.Filename
		data, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		gotData = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	service := &NotificationService{client: server.Client()}
	err := service.postMultipart(context.Background(), server.URL, map[string]string{"caption": "Print started"}, "photo", notificationAttachment{
		Filename:    "preview.png",
		ContentType: "image/png",
		Data:        []byte("png-data"),
	})
	if err != nil {
		t.Fatalf("postMultipart: %v", err)
	}
	if gotCaption != "Print started" {
		t.Fatalf("caption = %q", gotCaption)
	}
	if gotFilename != "preview.png" {
		t.Fatalf("filename = %q", gotFilename)
	}
	if gotData != "png-data" {
		t.Fatalf("data = %q", gotData)
	}
}

func TestTruncateRunesPreservesUTF8(t *testing.T) {
	t.Parallel()
	got := truncateRunes(strings.Repeat("á", 6), 5)
	if got != "áááá…" {
		t.Fatalf("truncateRunes = %q", got)
	}
}
