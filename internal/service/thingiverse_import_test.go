package service

import (
	"context"
	"testing"
)

func TestThingiverseImportService_ImportPreview_NoToken(t *testing.T) {
	svc := NewThingiverseImportService(nil, nil)

	_, err := svc.ImportPreview(context.Background(), ModelImportRequest{
		URL: "https://www.thingiverse.com/thing:7374617",
	})

	if err == nil {
		t.Fatal("expected error when token is not configured")
	}
}
