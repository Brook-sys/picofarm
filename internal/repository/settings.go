package repository

import (
	"context"
	"database/sql"
	"time"
)

type SettingsRepository struct {
	db *sql.DB
}

// Setting represents a single key-value setting.
type Setting struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Get retrieves a setting by key. Returns nil if not found.
func (r *SettingsRepository) Get(ctx context.Context, key string) (*Setting, error) {
	var s Setting
	var updatedAt string
	err := r.db.QueryRowContext(ctx, `SELECT key, value, updated_at FROM settings WHERE key = ?`, key).
		Scan(&s.Key, &s.Value, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// Set upserts a setting. Creates it if it doesn't exist, updates it if it does.
func (r *SettingsRepository) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, time.Now().UTC().Format(time.RFC3339))
	return err
}

// List retrieves all settings.
func (r *SettingsRepository) List(ctx context.Context) ([]Setting, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value, updated_at FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var s Setting
		var updatedAt string
		if err := rows.Scan(&s.Key, &s.Value, &updatedAt); err != nil {
			return nil, err
		}
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		settings = append(settings, s)
	}
	return settings, nil
}

// Delete removes a setting by key.
func (r *SettingsRepository) Delete(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}
