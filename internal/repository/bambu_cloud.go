package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/Brook-sys/picofarm/internal/crypto"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type BambuCloudRepository struct {
	db *sql.DB
}

// Upsert stores or updates Bambu Cloud auth credentials.
// Only one row is expected (single account). Tokens are encrypted before storage.
func (r *BambuCloudRepository) Upsert(ctx context.Context, auth *model.BambuCloudAuth) error {
	if auth.ID == uuid.Nil {
		auth.ID = uuid.New()
	}
	auth.UpdatedAt = time.Now()
	if auth.CreatedAt.IsZero() {
		auth.CreatedAt = auth.UpdatedAt
	}

	// Encrypt access token before storing
	accessToken := auth.AccessToken
	if accessToken != "" {
		if encrypted, err := crypto.Encrypt(accessToken); err == nil {
			accessToken = encrypted
		} else {
			slog.Warn("failed to encrypt bambu cloud access token", "error", err)
		}
	}

	// Encrypt refresh token before storing
	refreshToken := auth.RefreshToken
	if refreshToken != "" {
		if encrypted, err := crypto.Encrypt(refreshToken); err == nil {
			refreshToken = encrypted
		} else {
			slog.Warn("failed to encrypt bambu cloud refresh token", "error", err)
		}
	}

	// Delete existing then insert (simpler than upsert for single-row table)
	_, _ = r.db.ExecContext(ctx, `DELETE FROM bambu_cloud_auth`)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO bambu_cloud_auth (id, email, access_token, refresh_token, mqtt_username, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, auth.ID, auth.Email, accessToken, refreshToken, auth.MQTTUsername, auth.ExpiresAt, auth.CreatedAt, auth.UpdatedAt)
	return err
}

// Get retrieves the stored Bambu Cloud auth (if any).
// Tokens are decrypted before returning.
func (r *BambuCloudRepository) Get(ctx context.Context) (*model.BambuCloudAuth, error) {
	var auth model.BambuCloudAuth
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, email, access_token, refresh_token, mqtt_username, expires_at, created_at, updated_at
		FROM bambu_cloud_auth LIMIT 1
	`), &auth.ID, &auth.Email, &auth.AccessToken, &auth.RefreshToken, &auth.MQTTUsername, &auth.ExpiresAt, &auth.CreatedAt, &auth.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Decrypt access token
	if decrypted, err := crypto.Decrypt(auth.AccessToken); err == nil {
		auth.AccessToken = decrypted
	}

	// Decrypt refresh token
	if auth.RefreshToken != "" {
		if decrypted, err := crypto.Decrypt(auth.RefreshToken); err == nil {
			auth.RefreshToken = decrypted
		}
	}

	return &auth, nil
}

// Delete removes the stored Bambu Cloud auth.
func (r *BambuCloudRepository) Delete(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM bambu_cloud_auth`)
	return err
}

// SettingsRepository handles settings key-value storage.
