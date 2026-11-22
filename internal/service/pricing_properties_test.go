package service

import (
	"math/big"
	"testing"
)

// Independently check the advertised minimum against the returned attainable
// charge. A rounded-to-nearest analytical threshold may still lose money.
func TestPricingAdvertisedMinimumDoesNotLoseMoney(t *testing.T) {
	for _, currency := range []string{"BRL", "JPY", "CLP"} {
		for _, reserve := range []string{"0.001", "0.005", "1.005", "20.999999999"} {
			for _, percent := range []string{"0", "3.9", "33.333333333", "99.9"} {
				r := PricingCalculationRequest{Currency: currency, Quantity: 1, Production: PricingProduction{Mode: "batches", UnitsPerBatch: 1, Grams: "0", Hours: "0"}, MaterialCostPerKG: "0", Machine: &PricingMachine{Mode: "direct", HourlyRate: "0", IncludesEnergy: true, IncludesMaintenance: true}, Labor: &PricingLabor{Minutes: "0", HourlyRate: "0"}, Reserve: reserve, Extras: []PricingExtra{}, Shipping: &PricingShipping{Cost: "0", Charged: "0"}, Fees: []PricingFee{{Percent: percent, Fixed: "0.005", Scope: "order", Base: "goods"}}, Target: PricingTarget{Anchor: "price", Value: "0"}}
				got, err := CalculatePricing(r)
				if err != nil {
					t.Fatal(err)
				}
				minimum, ok := new(big.Rat).SetString(got.MinimumSalePrice)
				if !ok {
					t.Fatal("invalid minimum")
				}
				fee, _ := new(big.Rat).SetString(percent)
				fee.Quo(fee, big.NewRat(100, 1))
				cost, _ := new(big.Rat).SetString(reserve)
				cost.Add(cost, big.NewRat(5, 1000))
				recovered := new(big.Rat).Mul(minimum, new(big.Rat).Sub(big.NewRat(1, 1), fee))
				if recovered.Cmp(cost) < 0 {
					t.Fatalf("%s reserve=%s fee=%s minimum %s loses money", currency, reserve, percent, got.MinimumSalePrice)
				}
			}
		}
	}
}

func BenchmarkPricingCalculatorBoundedInputs(b *testing.B) {
	r := PricingCalculationRequest{Currency: "BRL", Quantity: 1000, Production: PricingProduction{Mode: "batches", UnitsPerBatch: 25, Grams: "250.123456789", Hours: "4.123456789"}, MaterialCostPerKG: "123.123456789", Machine: &PricingMachine{Mode: "derived", Acquisition: "2500", Residual: "100", LifetimeHours: "10000", MaintenanceHourly: "0.123456789"}, Energy: &PricingEnergy{PowerWatts: "150", TariffPerKWH: "0.923456789"}, Labor: &PricingLabor{Minutes: "2.5", HourlyRate: "25"}, Reserve: "10", Shipping: &PricingShipping{Cost: "20", Charged: "25"}, Target: PricingTarget{Anchor: "margin", Value: "25"}, DiscountPercent: "5"}
	for i := 0; i < 50; i++ {
		r.Extras = append(r.Extras, PricingExtra{Label: "bounded fixture", Amount: "0.123456789", Scope: "unit"})
		r.Fees = append(r.Fees, PricingFee{Percent: "0.123456789", Fixed: "0.123456789", Scope: "order", Base: "total"})
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := CalculatePricing(r); err != nil {
			b.Fatal(err)
		}
	}
}

func TestPricingCostIncreasePreservesEachTargetMeaning(t *testing.T) {
	for _, anchor := range []string{"price", "gain", "margin"} {
		r := PricingCalculationRequest{Currency: "BRL", Quantity: 1, Production: PricingProduction{Mode: "proportional", Grams: "100", Hours: "0"}, MaterialCostPerKG: "100", Reserve: "0", Target: PricingTarget{Anchor: anchor, Value: map[string]string{"price": "40", "gain": "10", "margin": "50"}[anchor]}}
		before, err := CalculatePricing(r)
		if err != nil {
			t.Fatal(err)
		}
		r.MaterialCostPerKG = "200"
		after, err := CalculatePricing(r)
		if err != nil {
			t.Fatal(err)
		}
		switch anchor {
		case "price":
			if before.Base.Price != after.Base.Price || after.Base.Gain != "20.00" {
				t.Fatalf("price anchor: %+v", after.Base)
			}
		case "gain":
			if after.Base.Gain != "10.00" || after.Base.Price != "30.00" {
				t.Fatalf("gain anchor: %+v", after.Base)
			}
		case "margin":
			if after.Base.MarginPercent == nil || *after.Base.MarginPercent != "50.00" || after.Base.Price != "40.00" {
				t.Fatalf("margin anchor: %+v", after.Base)
			}
		}
	}
}
