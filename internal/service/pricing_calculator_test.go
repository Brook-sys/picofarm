package service

import (
	"strings"
	"testing"
)

func pricingFixture() PricingCalculationRequest {
	return PricingCalculationRequest{Currency: "USD", Quantity: 3, Production: PricingProduction{Mode: "proportional", Grams: "100", Hours: "1"}, MaterialCostPerKG: "10", Machine: &PricingMachine{Mode: "direct", HourlyRate: "0", IncludesEnergy: true, IncludesMaintenance: true}, Labor: &PricingLabor{Minutes: "0", HourlyRate: "0"}, Extras: []PricingExtra{}, Reserve: "0", Shipping: &PricingShipping{Cost: "0", Charged: "0"}, Fees: []PricingFee{}, Target: PricingTarget{Anchor: "price", Value: "10"}}
}
func TestPricingFractionalArithmetic(t *testing.T) {
	r := pricingFixture()
	r.Production.Grams = "0.1"
	r.MaterialCostPerKG = "0.1"
	r.Target = PricingTarget{Anchor: "margin", Value: "33.333333333"}
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalGrams != "0.300000" || got.Base.Price != "0.00" || got.Base.MarginPercent != nil {
		t.Fatalf("%+v", got)
	}
	// Exact decimal ties, including negative gain, round away from zero.
	r = pricingFixture()
	r.Quantity = 1
	r.Production.Grams = "0"
	r.Reserve = "0.005"
	r.Target.Value = "0"
	got, err = CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base.Gain != "-0.01" || got.Base.MarginPercent != nil {
		t.Fatalf("%+v", got.Base)
	}
}
func TestPricingFeesAnchorsAndShipping(t *testing.T) {
	for _, anchor := range []string{"price", "gain", "margin"} {
		t.Run(anchor, func(t *testing.T) {
			r := pricingFixture()
			r.Shipping = &PricingShipping{Cost: "5", Charged: "4"}
			r.Fees = []PricingFee{{Percent: "10", Fixed: "1", Scope: "unit", Base: "total"}}
			// Costs 8; fees=.1*P+.4+3; gain=.9*P-7.4.
			value := map[string]string{"price": "20", "gain": "10.6", "margin": "53"}[anchor]
			r.Target = PricingTarget{Anchor: anchor, Value: value}
			got, err := CalculatePricing(r)
			if err != nil {
				t.Fatal(err)
			}
			if got.Base.Price != "20.00" || got.Base.Gain != "10.60" || got.Base.Fees != "5.40" || got.MinimumSalePrice != "8.23" {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestPricingActualGainUsesRoundedCharge(t *testing.T) {
	r := pricingFixture()
	r.Quantity = 1
	r.Production.Grams = "100"
	r.Target = PricingTarget{Anchor: "margin", Value: "33"}
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	// Ideal 1 / .67 = 1.492537..., actual currency charge = 1.49.
	if got.Base.Price != "1.49" || got.Base.Gain != "0.49" || *got.Base.MarginPercent != "32.89" {
		t.Fatalf("%+v", got.Base)
	}
	r.Shipping = &PricingShipping{Cost: "0", Charged: "10"}
	r.Target = PricingTarget{Anchor: "price", Value: "0"}
	got, err = CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.MinimumSalePrice != "0.00" || got.Base.Gain != "9.00" {
		t.Fatalf("%+v", got)
	}
}

func TestPricingBatchConservativeRemainder(t *testing.T) {
	r := pricingFixture()
	r.Quantity = 5
	r.Production = PricingProduction{Mode: "batches", UnitsPerBatch: 2, Grams: "100", Hours: "0.333333333"}
	r.Extras = []PricingExtra{{Label: "setup", Amount: "2", Scope: "batch"}, {Label: "unit", Amount: "1", Scope: "unit"}, {Label: "project", Amount: "3", Scope: "project"}, {Label: "order", Amount: "4", Scope: "order"}}
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.EffectivePlates != "3" || got.TotalGrams != "300.000000" || got.Breakdown.Extras != "18.00" || !strings.Contains(strings.Join(got.Warnings, ","), "full_batch_remainder") {
		t.Fatalf("%+v", got)
	}
}
func TestPricingDerivedMachine(t *testing.T) {
	r := pricingFixture()
	r.Machine = &PricingMachine{Mode: "derived", Acquisition: "1000", Residual: "100", LifetimeHours: "300", MaintenanceHourly: "0.5"}
	r.Energy = &PricingEnergy{PowerWatts: "200", TariffPerKWH: "0.3"}
	r.Labor = &PricingLabor{Minutes: "10", HourlyRate: "12"}
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Breakdown.Machine != "9.00" || got.Breakdown.Maintenance != "1.50" || got.Breakdown.Energy != "0.18" || got.Breakdown.Labor != "6.00" {
		t.Fatalf("%+v", got)
	}
}
func TestPricingDiscountAndMinimumCharge(t *testing.T) {
	r := pricingFixture()
	r.MinCharge = "20"
	r.Target = PricingTarget{Anchor: "gain", Value: "0"}
	r.DiscountPercent = "90"
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base.Price != "20.00" || got.Discounted.Price != "2.00" || got.Discounted.Gain != "-1.00" {
		t.Fatalf("%+v", got)
	}
	r.Target = PricingTarget{Anchor: "price", Value: "1"}
	got, err = CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base.Price != "1.00" || !strings.Contains(strings.Join(got.Warnings, ","), "price_below_minimum_charge") {
		t.Fatalf("%+v", got)
	}
}
func TestPricingMissingWarningsAndJPY(t *testing.T) {
	r := PricingCalculationRequest{Currency: "JPY", Quantity: 1, Production: PricingProduction{Mode: "batches", UnitsPerBatch: 1}, Target: PricingTarget{Anchor: "price", Value: "1.5"}}
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base.Price != "2" || !got.Estimate || len(got.Warnings) < 8 {
		t.Fatalf("%+v", got)
	}
	r = pricingFixture()
	r.Production = PricingProduction{Mode: "batches", UnitsPerBatch: 3, Grams: "100", Hours: "1"}
	got, err = CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Estimate {
		t.Fatalf("explicit zero inputs must not warn: %+v", got.Warnings)
	}
}
func TestPricingMinimumChargeQuantizedBeforeEnforcement(t *testing.T) {
	for _, tc := range []struct{ currency, floor, want, manual string }{{"USD", "1.004", "1.01", "1.00"}, {"JPY", "1.4", "2", "1"}} {
		for _, anchor := range []string{"gain", "margin", "price"} {
			t.Run(tc.currency+"/"+anchor, func(t *testing.T) {
				r := pricingFixture()
				r.Currency = tc.currency
				r.Quantity = 1
				r.Production.Grams = "0"
				r.MinCharge = tc.floor
				r.Target = PricingTarget{Anchor: anchor, Value: "0"}
				if anchor == "price" {
					r.Target.Value = tc.floor
				}
				got, err := CalculatePricing(r)
				if err != nil {
					t.Fatal(err)
				}
				want, warning := tc.want, "minimum_charge_applied"
				if anchor == "price" {
					want, warning = tc.manual, "price_below_minimum_charge"
				}
				if got.Base.Price != want || !strings.Contains(strings.Join(got.Warnings, ","), warning) {
					t.Fatalf("%+v", got)
				}
			})
		}
	}
	// A nonbinding floor does not alter the target-derived charge.
	r := pricingFixture()
	r.MinCharge = "1.004"
	r.Target = PricingTarget{Anchor: "gain", Value: "3"}
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base.Price != "6.00" || got.Base.Gain != "3.00" {
		t.Fatalf("%+v", got)
	}
}

func TestPricingMinimumCeilsWithoutLoss(t *testing.T) {
	for _, tc := range []struct{ currency, cost, want string }{{"USD", "1.005", "1.01"}, {"USD", "0.014", "0.02"}, {"USD", "1.01", "1.01"}, {"JPY", "1.005", "2"}} {
		r := pricingFixture()
		r.Quantity = 1
		r.Currency = tc.currency
		r.Production.Grams = "0"
		r.Reserve = tc.cost
		got, err := CalculatePricing(r)
		if err != nil {
			t.Fatal(err)
		}
		if got.MinimumSalePrice != tc.want {
			t.Fatalf("cost=%s got=%s want=%s", tc.cost, got.MinimumSalePrice, tc.want)
		}
		r.Target.Value = got.MinimumSalePrice
		charged, err := CalculatePricing(r)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(charged.Warnings, ","), "negative_gain") {
			t.Fatalf("minimum loses money: %+v", charged)
		}
	}
}
func TestPricingSignedTargetsAndLossWarnings(t *testing.T) {
	for _, tc := range []struct{ anchor, value, price, gain string }{{"gain", "-1", "2.00", "-1.00"}, {"margin", "-50", "2.00", "-1.00"}} {
		r := pricingFixture()
		r.Target = PricingTarget{Anchor: tc.anchor, Value: tc.value}
		got, err := CalculatePricing(r)
		if err != nil {
			t.Fatal(err)
		}
		if got.Base.Price != tc.price || got.Base.Gain != tc.gain || !strings.Contains(strings.Join(got.Warnings, ","), "negative_gain") {
			t.Fatalf("%+v", got)
		}
	}
	for _, target := range []PricingTarget{{Anchor: "price", Value: "-1"}, {Anchor: "gain", Value: "-4"}, {Anchor: "gain", Value: "-1000000001"}, {Anchor: "margin", Value: "--1"}} {
		r := pricingFixture()
		r.Target = target
		r.MinCharge = "10"
		if _, err := CalculatePricing(r); err == nil {
			t.Fatalf("accepted %+v", target)
		}
	}
	// A loss smaller than a displayed cent must still warn.
	r := pricingFixture()
	r.Quantity = 1
	r.Production.Grams = "0"
	r.Reserve = "1.001"
	r.Target.Value = "1"
	r.DiscountPercent = "0"
	got, err := CalculatePricing(r)
	if err != nil {
		t.Fatal(err)
	}
	warnings := strings.Join(got.Warnings, ",")
	if !strings.Contains(warnings, "negative_gain") || !strings.Contains(warnings, "discounted_loss") {
		t.Fatalf("%+v", got)
	}
}

func TestPricingRejectsInvalidInputs(t *testing.T) {
	cases := map[string]func(*PricingCalculationRequest){
		"currency": func(r *PricingCalculationRequest) { r.Currency = "ZZZ" }, "quantity": func(r *PricingCalculationRequest) { r.Quantity = 0 }, "mode": func(r *PricingCalculationRequest) { r.Production.Mode = "auto" },
		"fraction syntax": func(r *PricingCalculationRequest) { r.Reserve = "1/3" }, "exponent": func(r *PricingCalculationRequest) { r.Reserve = "1e3" }, "negative": func(r *PricingCalculationRequest) { r.Reserve = "-1" }, "large": func(r *PricingCalculationRequest) { r.Reserve = "1000000001" }, "long": func(r *PricingCalculationRequest) { r.Reserve = strings.Repeat("1", 100) },
		"double energy": func(r *PricingCalculationRequest) { r.Energy = &PricingEnergy{PowerWatts: "0"} }, "double maintenance": func(r *PricingCalculationRequest) { r.Machine.MaintenanceHourly = "0" }, "zero lifetime": func(r *PricingCalculationRequest) {
			r.Machine = &PricingMachine{Mode: "derived", Acquisition: "10", Residual: "0", LifetimeHours: "0"}
		},
		"fee denominator": func(r *PricingCalculationRequest) {
			r.Fees = []PricingFee{{Percent: "50", Fixed: "0", Scope: "order", Base: "goods"}, {Percent: "50", Fixed: "0", Scope: "order", Base: "goods"}}
		}, "margin denominator": func(r *PricingCalculationRequest) { r.Target = PricingTarget{Anchor: "margin", Value: "100"} },
		"batch scope": func(r *PricingCalculationRequest) {
			r.Extras = []PricingExtra{{Label: "x", Amount: "1", Scope: "batch"}}
		}, "many extras": func(r *PricingCalculationRequest) { r.Extras = make([]PricingExtra, 51) }, "many fees": func(r *PricingCalculationRequest) { r.Fees = make([]PricingFee, 51) }, "discount": func(r *PricingCalculationRequest) { r.DiscountPercent = "101" }, "missing target": func(r *PricingCalculationRequest) { r.Target.Value = "" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r := pricingFixture()
			change(&r)
			if _, err := CalculatePricing(r); err == nil {
				t.Fatal("accepted invalid request")
			}
		})
	}
}
