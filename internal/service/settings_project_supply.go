package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Brook-sys/picofarm/internal/crypto"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type SettingsService struct {
	repo *repository.SettingsRepository
}

// sensitiveKeys lists settings that should be encrypted at rest.
var sensitiveKeys = map[string]bool{
	"anthropic_api_key":     true,
	"etsy_client_id":        true,
	"etsy_access_token":     true,
	"etsy_refresh_token":    true,
	"bambu_cloud_token":     true,
	"bambu_cloud_password":  true,
	"thingiverse_api_token": true,
}

// isSensitive checks if a key should be encrypted.
func isSensitive(key string) bool {
	return sensitiveKeys[key]
}

// Get retrieves a setting by key, decrypting if necessary.
func (s *SettingsService) Get(ctx context.Context, key string) (*repository.Setting, error) {
	setting, err := s.repo.Get(ctx, key)
	if err != nil || setting == nil {
		return setting, err
	}

	// Decrypt sensitive values
	if isSensitive(key) && crypto.IsEncrypted(setting.Value) {
		decrypted, err := crypto.Decrypt(setting.Value)
		if err != nil {
			slog.Warn("failed to decrypt setting", "key", key, "error", err)
			// Return original value if decryption fails (might be unencrypted legacy data)
			return setting, nil
		}
		setting.Value = decrypted
	}

	return setting, nil
}

// Set creates or updates a setting, encrypting sensitive values.
func (s *SettingsService) Set(ctx context.Context, key, value string) error {
	// Encrypt sensitive values
	if isSensitive(key) && value != "" {
		encrypted, err := crypto.Encrypt(value)
		if err != nil {
			slog.Warn("failed to encrypt setting, storing unencrypted", "key", key, "error", err)
			// Fall back to storing unencrypted if encryption fails
		} else {
			value = encrypted
		}
	}

	return s.repo.Set(ctx, key, value)
}

// List retrieves all settings, decrypting sensitive values.
func (s *SettingsService) List(ctx context.Context) ([]repository.Setting, error) {
	settings, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Decrypt sensitive values
	for i := range settings {
		if isSensitive(settings[i].Key) && crypto.IsEncrypted(settings[i].Value) {
			decrypted, err := crypto.Decrypt(settings[i].Value)
			if err != nil {
				slog.Warn("failed to decrypt setting", "key", settings[i].Key, "error", err)
				continue
			}
			settings[i].Value = decrypted
		}
	}

	return settings, nil
}

// Delete removes a setting.
func (s *SettingsService) Delete(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}

// ProjectSupplyService handles project supply business logic.
type ProjectSupplyService struct {
	repo         *repository.ProjectSupplyRepository
	materialRepo *repository.MaterialRepository
}

// Create creates a new project supply.
func (s *ProjectSupplyService) Create(ctx context.Context, supply *model.ProjectSupply) error {
	// If material_id is set and name is empty, auto-populate from the material
	if supply.MaterialID != nil && *supply.MaterialID != uuid.Nil && supply.Name == "" {
		mat, err := s.materialRepo.GetByID(ctx, *supply.MaterialID)
		if err != nil {
			return fmt.Errorf("failed to look up material: %w", err)
		}
		if mat != nil {
			supply.Name = mat.Name
			if supply.UnitCostCents == 0 {
				supply.UnitCostCents = int(mat.CostPerKg * 100) // CostPerKg repurposed as per-unit $ for supplies
			}
		}
	}
	if supply.Name == "" {
		return fmt.Errorf("supply name is required")
	}
	if supply.Quantity < 1 {
		supply.Quantity = 1
	}
	return s.repo.Create(ctx, supply)
}

// ListByProject retrieves all supplies for a project.
func (s *ProjectSupplyService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]model.ProjectSupply, error) {
	return s.repo.ListByProject(ctx, projectID)
}

// Delete removes a project supply.
func (s *ProjectSupplyService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
