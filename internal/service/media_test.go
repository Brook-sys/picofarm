package service

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

func newCameraServiceTest(t *testing.T) (*sql.DB, *repository.Repositories, *CameraService) {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repos := repository.NewRepositories(db)
	return db, repos, &CameraService{repo: repos.Cameras, printerRepo: repos.Printers}
}

func TestCameraService_ListDiscoversMoonrakerWebcamsForPrinter(t *testing.T) {
	_, repos, svc := newCameraServiceTest(t)

	moonraker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/webcams/list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"webcams":[{"name":"Mesa","enabled":true,"stream_url":"http://100.84.153.105/webcam/?action=stream"}]}}`))
	}))
	t.Cleanup(moonraker.Close)

	printer := &model.Printer{
		Name:           "Klipper",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  moonraker.URL,
	}
	if err := repos.Printers.Create(context.Background(), printer); err != nil {
		t.Fatalf("create printer: %v", err)
	}

	cameras, err := svc.List(context.Background(), &printer.ID, nil)
	if err != nil {
		t.Fatalf("list cameras: %v", err)
	}
	if len(cameras) != 1 {
		t.Fatalf("expected 1 discovered camera, got %d", len(cameras))
	}
	if cameras[0].PrinterID == nil || *cameras[0].PrinterID != printer.ID {
		t.Fatalf("expected camera linked to printer %s, got %#v", printer.ID, cameras[0].PrinterID)
	}
	if cameras[0].Name != "Mesa" {
		t.Fatalf("expected camera name Mesa, got %q", cameras[0].Name)
	}
	if cameras[0].URL != "http://100.84.153.105/webcam/?action=stream" {
		t.Fatalf("unexpected stream url: %q", cameras[0].URL)
	}
	if !cameras[0].Enabled {
		t.Fatal("expected discovered camera to be enabled")
	}
}

func TestCameraService_ListResolvesRelativeMoonrakerWebcamURLAgainstFluiddURL(t *testing.T) {
	_, repos, svc := newCameraServiceTest(t)

	moonraker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"webcams":[{"name":"Default","enabled":true,"stream_url":"/webcam/?action=stream"}]}}`))
	}))
	t.Cleanup(moonraker.Close)

	printer := &model.Printer{
		Name:           "Klipper",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  moonraker.URL,
		FluiddURL:      "http://100.84.153.105",
	}
	if err := repos.Printers.Create(context.Background(), printer); err != nil {
		t.Fatalf("create printer: %v", err)
	}

	cameras, err := svc.List(context.Background(), &printer.ID, nil)
	if err != nil {
		t.Fatalf("list cameras: %v", err)
	}
	if len(cameras) != 1 {
		t.Fatalf("expected 1 discovered camera, got %d", len(cameras))
	}
	if cameras[0].URL != "http://100.84.153.105/webcam/?action=stream" {
		t.Fatalf("unexpected resolved stream url: %q", cameras[0].URL)
	}
}

func TestCameraService_ListKeepsManualCameraAndSkipsDuplicateMoonrakerURL(t *testing.T) {
	_, repos, svc := newCameraServiceTest(t)

	moonraker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"webcams":[{"name":"Moonraker","enabled":true,"stream_url":"http://100.84.153.105/webcam/?action=stream"}]}}`))
	}))
	t.Cleanup(moonraker.Close)

	printer := &model.Printer{
		Name:           "Klipper",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  moonraker.URL,
	}
	if err := repos.Printers.Create(context.Background(), printer); err != nil {
		t.Fatalf("create printer: %v", err)
	}

	manual := &model.Camera{
		PrinterID: &printer.ID,
		Name:      "Manual",
		Type:      "mjpeg",
		URL:       "http://100.84.153.105/webcam/?action=stream",
	}
	if err := svc.Create(context.Background(), manual); err != nil {
		t.Fatalf("create manual camera: %v", err)
	}

	cameras, err := svc.List(context.Background(), &printer.ID, nil)
	if err != nil {
		t.Fatalf("list cameras: %v", err)
	}
	if len(cameras) != 1 {
		t.Fatalf("expected duplicate Moonraker camera to be skipped, got %d", len(cameras))
	}
	if cameras[0].Name != "Manual" {
		t.Fatalf("expected manual camera to win, got %q", cameras[0].Name)
	}
}

func TestCameraService_ListDiscoversMoonrakerWebcamsWithoutPrinterFilter(t *testing.T) {
	_, repos, svc := newCameraServiceTest(t)

	moonraker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/webcams/list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"webcams":[{"name":"USB","enabled":true,"stream_url":"/webcam?action=stream"}]}}`))
	}))
	t.Cleanup(moonraker.Close)

	printer := &model.Printer{
		Name:           "NP4",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  moonraker.URL,
		FluiddURL:      "http://100.84.153.105",
	}
	if err := repos.Printers.Create(context.Background(), printer); err != nil {
		t.Fatalf("create printer: %v", err)
	}

	cameras, err := svc.List(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("list cameras: %v", err)
	}
	if len(cameras) != 1 {
		t.Fatalf("expected 1 discovered camera in global list, got %d", len(cameras))
	}
	if cameras[0].PrinterID == nil || *cameras[0].PrinterID != printer.ID {
		t.Fatalf("expected global discovered camera linked to printer %s, got %#v", printer.ID, cameras[0].PrinterID)
	}
	if cameras[0].URL != "http://100.84.153.105/webcam?action=stream" {
		t.Fatalf("unexpected global resolved stream url: %q", cameras[0].URL)
	}
}
