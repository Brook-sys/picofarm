package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/repository"
)

func newFilamentColorsSettingsService(t *testing.T) *SettingsService {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &SettingsService{repo: repository.NewRepositories(db).Settings}
}

// The files screen reads this setting raw and applies its own defaults, so a
// malformed value must be refused here instead of silently degrading the badge
// to a neutral color.
func TestSettingsServiceFilamentColorsRejectsMalformedValues(t *testing.T) {
	settings := newFilamentColorsSettingsService(t)

	cases := []struct {
		name  string
		value string
	}{
		{"not json", "pla=#22c55e"},
		{"json array", `["pla","#22c55e"]`},
		{"json scalar", `"#22c55e"`},
		{"non-string color", `{"pla":37}`},
		{"color without hash", `{"pla":"22c55e"}`},
		{"css color name", `{"pla":"rebeccapurple"}`},
		{"empty type", `{"":"#22c55e"}`},
		{"type that normalizes away", `{"#":"#22c55e"}`},
		{"conflicting normalized keys", `{"pla":"#22c55e","PLA-CF":"#3b82f6"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := settings.Set(context.Background(), FilamentTypeColorsSettingKey, tc.value)
			if !errors.Is(err, ErrInvalidFilamentTypeColors) {
				t.Fatalf("expected ErrInvalidFilamentTypeColors for %s, got %v", tc.name, err)
			}
			stored, err := settings.Get(context.Background(), FilamentTypeColorsSettingKey)
			if err != nil {
				t.Fatalf("Get after rejection: %v", err)
			}
			if stored != nil {
				t.Fatalf("rejected value was persisted: %v", stored.Value)
			}
		})
	}
}

// Keys must be stored the same way the G-code parser normalizes material_type,
// otherwise a configured color can never match anything on the files screen.
func TestSettingsServiceFilamentColorsNormalizesKeys(t *testing.T) {
	settings := newFilamentColorsSettingsService(t)
	ctx := context.Background()

	err := settings.Set(ctx, FilamentTypeColorsSettingKey, `{"  PETG ":"#3B82F6","PLA-CF":"#22c55e","TPU":"#ec4899"}`)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	stored, err := settings.Get(ctx, FilamentTypeColorsSettingKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored == nil {
		t.Fatal("expected stored value")
	}
	expected := `{"petg":"#3b82f6","pla":"#22c55e","tpu":"#ec4899"}`
	if stored.Value != expected {
		t.Fatalf("expected normalized keys/lowercase colors %s, got %s", expected, stored.Value)
	}
}

func TestSettingsServiceFilamentColorsAcceptsEmptyObjectAndEmptyValue(t *testing.T) {
	settings := newFilamentColorsSettingsService(t)
	ctx := context.Background()

	if err := settings.Set(ctx, FilamentTypeColorsSettingKey, "  "); err != nil {
		t.Fatalf("Set empty: %v", err)
	}
	// The generic settings store keeps an (empty) row; the front end treats an
	// absent and an empty value identically and falls back to its defaults.
	stored, err := settings.Get(ctx, FilamentTypeColorsSettingKey)
	if err != nil {
		t.Fatalf("Get after empty: %v", err)
	}
	if stored == nil || stored.Value != "" {
		t.Fatalf("expected the value to be cleared, got %v", stored)
	}

	if err := settings.Set(ctx, FilamentTypeColorsSettingKey, "{}"); err != nil {
		t.Fatalf("Set empty object: %v", err)
	}
	stored, err = settings.Get(ctx, FilamentTypeColorsSettingKey)
	if err != nil {
		t.Fatalf("Get after empty object: %v", err)
	}
	if stored == nil || stored.Value != "{}" {
		t.Fatalf("expected {} to be stored, got %v", stored)
	}
}
