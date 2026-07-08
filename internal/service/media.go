package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type CameraService struct {
	repo        *repository.CameraRepository
	printerRepo *repository.PrinterRepository
}

func (s *CameraService) Create(ctx context.Context, c *model.Camera) error {
	c.Name = strings.TrimSpace(c.Name)
	c.URL = strings.TrimSpace(c.URL)
	c.Type = strings.TrimSpace(c.Type)
	if c.Name == "" {
		return fmt.Errorf("camera name is required")
	}
	if c.URL == "" {
		return fmt.Errorf("camera url is required")
	}
	parsed, err := url.Parse(c.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid camera url")
	}
	if c.Type == "" {
		c.Type = "mjpeg"
	}
	switch c.Type {
	case "mjpeg", "rtsp", "webrtc", "snapshot":
	default:
		return fmt.Errorf("unsupported camera type")
	}
	c.Enabled = true
	return s.repo.Create(ctx, c)
}

func (s *CameraService) List(ctx context.Context, printerID *uuid.UUID, enabled *bool) ([]model.Camera, error) {
	cameras, err := s.repo.List(ctx, printerID, enabled)
	if err != nil {
		return nil, err
	}
	if s.printerRepo == nil {
		return cameras, nil
	}
	if printerID != nil {
		return s.withDiscoveredMoonrakerWebcams(ctx, cameras, *printerID, enabled)
	}

	printers, err := s.printerRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, printer := range printers {
		cameras, err = s.withDiscoveredMoonrakerWebcams(ctx, cameras, printer.ID, enabled)
		if err != nil {
			return nil, err
		}
	}
	return cameras, nil
}

func (s *CameraService) Update(ctx context.Context, c *model.Camera) error {
	return s.repo.Update(ctx, c)
}

func (s *CameraService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *CameraService) MintToken(ctx context.Context, camera *model.Camera, days int) error {
	if days <= 0 || days > 365 {
		return fmt.Errorf("token duration must be between 1 and 365 days")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	expires := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	camera.Token = hex.EncodeToString(buf)
	camera.TokenExpiresAt = &expires
	return s.repo.Update(ctx, camera)
}

func (s *CameraService) withDiscoveredMoonrakerWebcams(ctx context.Context, cameras []model.Camera, printerID uuid.UUID, enabled *bool) ([]model.Camera, error) {
	printer, err := s.printerRepo.GetByID(ctx, printerID)
	if err != nil || printer == nil || printer.ConnectionType != model.ConnectionTypeMoonraker || printer.ConnectionURI == "" {
		return cameras, err
	}

	discovered, err := s.discoverMoonrakerWebcams(ctx, printer)
	if err != nil {
		slog.Debug("CameraService: failed to discover Moonraker webcams", "printer_id", printerID, "error", err)
		return cameras, nil
	}

	existingURLs := make(map[string]struct{}, len(cameras))
	for _, camera := range cameras {
		existingURLs[strings.TrimRight(camera.URL, "/")] = struct{}{}
	}

	for _, camera := range discovered {
		if enabled != nil && camera.Enabled != *enabled {
			continue
		}
		key := strings.TrimRight(camera.URL, "/")
		if _, exists := existingURLs[key]; exists {
			continue
		}
		existingURLs[key] = struct{}{}
		cameras = append(cameras, camera)
	}

	return cameras, nil
}

func (s *CameraService) discoverMoonrakerWebcams(ctx context.Context, printer *model.Printer) ([]model.Camera, error) {
	endpoint, err := url.JoinPath(strings.TrimRight(printer.ConnectionURI, "/"), "/server/webcams/list")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("moonraker webcams list failed: %s", resp.Status)
	}

	var payload moonrakerWebcamsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	webcams := payload.Result.Webcams
	if webcams == nil {
		webcams = payload.Webcams
	}

	cameras := make([]model.Camera, 0, len(webcams))
	now := time.Now()
	for _, webcam := range webcams {
		streamURL := firstNonEmptyCamera(webcam.StreamURL, webcam.URL, webcam.SnapshotURL)
		if strings.TrimSpace(streamURL) == "" {
			continue
		}
		resolvedURL := resolveCameraURL(streamURL, printer)
		if resolvedURL == "" {
			continue
		}
		name := strings.TrimSpace(webcam.Name)
		if name == "" {
			name = "Moonraker webcam"
		}
		typ := strings.TrimSpace(webcam.Type)
		if typ == "" {
			typ = "mjpeg"
		}
		cameraPrinterID := printer.ID
		cameras = append(cameras, model.Camera{
			ID:        uuid.New(),
			PrinterID: &cameraPrinterID,
			Name:      name,
			Type:      typ,
			URL:       resolvedURL,
			Enabled:   webcam.Enabled == nil || *webcam.Enabled,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return cameras, nil
}

func resolveCameraURL(raw string, printer *model.Printer) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() && parsed.Host != "" {
		return parsed.String()
	}
	baseRaw := firstNonEmptyCamera(printer.FluiddURL, printer.ConnectionURI)
	base, err := url.Parse(baseRaw)
	if err != nil || base.Host == "" {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

func firstNonEmptyCamera(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type moonrakerWebcamsResponse struct {
	Result struct {
		Webcams []moonrakerWebcam `json:"webcams"`
	} `json:"result"`
	Webcams []moonrakerWebcam `json:"webcams"`
}

type moonrakerWebcam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	StreamURL   string `json:"stream_url"`
	SnapshotURL string `json:"snapshot_url"`
	URL         string `json:"url"`
	Enabled     *bool  `json:"enabled"`
}

type TimelapseService struct {
	repo *repository.TimelapseRepository
}

func (s *TimelapseService) Create(ctx context.Context, t *model.Timelapse) error {
	if t.Status == "" {
		t.Status = "pending"
	}
	return s.repo.Create(ctx, t)
}

func (s *TimelapseService) List(ctx context.Context, printerID *uuid.UUID) ([]model.Timelapse, error) {
	return s.repo.List(ctx, printerID)
}

type PrintArchiveService struct {
	repo *repository.PrintArchiveRepository
}

func (s *PrintArchiveService) Create(ctx context.Context, a *model.PrintArchive) error {
	a.Status = strings.TrimSpace(a.Status)
	if a.Status == "" {
		return fmt.Errorf("archive status is required")
	}
	switch a.Status {
	case "completed", "failed", "cancelled":
	default:
		return fmt.Errorf("unsupported archive status")
	}
	if a.DurationSeconds < 0 || a.FilamentUsedGrams < 0 || a.CostCents < 0 {
		return fmt.Errorf("archive metrics cannot be negative")
	}
	return s.repo.Create(ctx, a)
}

func (s *PrintArchiveService) List(ctx context.Context, printerID *uuid.UUID, status string) ([]model.PrintArchive, error) {
	return s.repo.List(ctx, printerID, status)
}

func (s *PrintArchiveService) Compare(ctx context.Context, aID uuid.UUID, bID uuid.UUID) (map[string]interface{}, error) {
	archives, err := s.repo.List(ctx, nil, "")
	if err != nil {
		return nil, err
	}
	var left, right *model.PrintArchive
	for i := range archives {
		if archives[i].ID == aID {
			left = &archives[i]
		}
		if archives[i].ID == bID {
			right = &archives[i]
		}
	}
	if left == nil || right == nil {
		return nil, fmt.Errorf("archive not found")
	}
	return map[string]interface{}{
		"a": left,
		"b": right,
		"differences": map[string]bool{
			"status":              left.Status != right.Status,
			"printer_id":          fmt.Sprint(left.PrinterID) != fmt.Sprint(right.PrinterID),
			"duration_seconds":    left.DurationSeconds != right.DurationSeconds,
			"filament_used_grams": left.FilamentUsedGrams != right.FilamentUsedGrams,
			"cost_cents":          left.CostCents != right.CostCents,
		},
	}, nil
}
