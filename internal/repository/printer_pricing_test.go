package repository

import (
	"context"
	"reflect"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestPrinterPricingRoundTrip(t *testing.T) {
	db := openTestDB(t)
	repo := &PrinterRepository{db: db}
	ctx := context.Background()
	p := &model.Printer{Name: "Manual", ConnectionType: model.ConnectionTypeManual, CostPerHourCents: 123}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PricingConfig != nil {
		t.Fatal("legacy profile inferred")
	}
	p.PricingConfig = &model.PrinterPricingConfig{Currency: "BRL", RateMode: "direct", DirectRatePerHour: "1.230001", IncludesEnergy: true}
	if err := repo.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.PricingConfig, p.PricingConfig) || got.CostPerHourCents != 123 {
		t.Fatalf("round trip: %+v", got)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !reflect.DeepEqual(list[0].PricingConfig, p.PricingConfig) {
		t.Fatal("list profile mismatch")
	}
	p.PricingConfig = nil
	if err := repo.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil || got.PricingConfig != nil {
		t.Fatal("clear failed", err)
	}
	// Create path also persists the typed profile.
	p.PricingConfig = &model.PrinterPricingConfig{Currency: "JPY", RateMode: "direct", DirectRatePerHour: "0"}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil || !reflect.DeepEqual(got.PricingConfig, p.PricingConfig) {
		t.Fatal("create profile mismatch", err)
	}
}
