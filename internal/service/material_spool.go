package service

import (
	"context"
	"fmt"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type MaterialService struct {
	repo *repository.MaterialRepository
}

// Create creates a new material.
func (s *MaterialService) Create(ctx context.Context, m *model.Material) error {
	if m.Name == "" {
		return fmt.Errorf("material name is required")
	}
	return s.repo.Create(ctx, m)
}

// GetByID retrieves a material by ID.
func (s *MaterialService) GetByID(ctx context.Context, id uuid.UUID) (*model.Material, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all materials.
func (s *MaterialService) List(ctx context.Context) ([]model.Material, error) {
	return s.repo.List(ctx)
}

// ListByType retrieves all materials of a given type.
func (s *MaterialService) ListByType(ctx context.Context, matType model.MaterialType) ([]model.Material, error) {
	return s.repo.ListByType(ctx, matType)
}

// Update updates an existing material.
func (s *MaterialService) Update(ctx context.Context, m *model.Material) error {
	if m.Name == "" {
		return fmt.Errorf("material name is required")
	}
	return s.repo.Update(ctx, m)
}

// Delete removes a material by ID.
func (s *MaterialService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// SpoolService handles spool business logic.
type SpoolService struct {
	repo *repository.SpoolRepository
}

// Create creates a new spool.
func (s *SpoolService) Create(ctx context.Context, sp *model.MaterialSpool) error {
	if sp.MaterialID == uuid.Nil {
		return fmt.Errorf("material ID is required")
	}
	if sp.RemainingWeight == 0 {
		sp.RemainingWeight = sp.InitialWeight
	}
	if err := s.repo.Create(ctx, sp); err != nil {
		return err
	}
	if sp.DefaultForMaterial {
		return s.repo.SetDefaultForMaterial(ctx, sp.ID)
	}
	return s.repo.EnsureDefaultForMaterialID(ctx, sp.MaterialID)
}

// GetByID retrieves a spool by ID.
func (s *SpoolService) GetByID(ctx context.Context, id uuid.UUID) (*model.MaterialSpool, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all spools.
func (s *SpoolService) List(ctx context.Context) ([]model.MaterialSpool, error) {
	return s.repo.List(ctx)
}

// Update updates a spool.
func (s *SpoolService) Update(ctx context.Context, sp *model.MaterialSpool) error {
	if sp.MaterialID == uuid.Nil {
		return fmt.Errorf("material ID is required")
	}
	if err := s.repo.Update(ctx, sp); err != nil {
		return err
	}
	if sp.DefaultForMaterial {
		return s.repo.SetDefaultForMaterial(ctx, sp.ID)
	}
	return s.repo.EnsureDefaultForMaterialID(ctx, sp.MaterialID)
}

// Delete deletes a spool by ID.
func (s *SpoolService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// PrintJobService handles print job business logic.
// Jobs are immutable once created - state changes are recorded as events.
