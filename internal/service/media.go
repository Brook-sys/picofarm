package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

type CameraService struct {
	repo *repository.CameraRepository
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
	return s.repo.List(ctx, printerID, enabled)
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
