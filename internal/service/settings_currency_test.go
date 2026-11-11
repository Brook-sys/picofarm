package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/repository"
)

func newCurrencySettingsService(t *testing.T) *SettingsService {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &SettingsService{repo: repository.NewRepositories(db).Settings}
}

func TestSettingsServiceCurrencyDefaultsToUSD(t *testing.T) {
	settings := newCurrencySettingsService(t)

	currency, err := settings.GetCurrencyCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if currency != "USD" {
		t.Fatalf("expected USD default, got %q", currency)
	}
}

func TestSettingsServiceCurrencyNormalizesAndPersistsSupportedCode(t *testing.T) {
	settings := newCurrencySettingsService(t)
	ctx := context.Background()

	if err := settings.Set(ctx, CurrencySettingKey, " brl "); err != nil {
		t.Fatal(err)
	}
	currency, err := settings.GetCurrencyCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if currency != "BRL" {
		t.Fatalf("expected BRL, got %q", currency)
	}
}

func TestSettingsServiceCurrencyRejectsUnsupportedCode(t *testing.T) {
	settings := newCurrencySettingsService(t)

	err := settings.Set(context.Background(), CurrencySettingKey, "XYZ")
	if !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("expected ErrUnsupportedCurrency, got %v", err)
	}
}

func TestSettingsServiceCurrencyDeleteRestoresDefault(t *testing.T) {
	settings := newCurrencySettingsService(t)
	ctx := context.Background()

	if err := settings.Set(ctx, CurrencySettingKey, "EUR"); err != nil {
		t.Fatal(err)
	}
	if err := settings.Delete(ctx, CurrencySettingKey); err != nil {
		t.Fatal(err)
	}
	currency, err := settings.GetCurrencyCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if currency != "USD" {
		t.Fatalf("expected USD after delete, got %q", currency)
	}
}
