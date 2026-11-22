package service

import (
	"fmt"
	"math/big"
	"regexp"
)

// PricingCalculationRequest is a compute-only contract. All decimal inputs are
// base-10 strings (up to 9 fractional digits); only gain/margin target values
// may be negative. Counts are integers. Consumers must preserve decimal strings
// when formatting money: aggregate minor units can exceed JS safe integers.
// Grams/hours are per unit in proportional mode and per full plate in batches.
// Labor minutes are always per requested unit. Project/order extras occur once.
type PricingCalculationRequest struct {
	Currency          string            `json:"currency"`
	Quantity          int               `json:"quantity"`
	Production        PricingProduction `json:"production"`
	MaterialCostPerKG string            `json:"material_cost_per_kg,omitempty"`
	Machine           *PricingMachine   `json:"machine,omitempty"`
	Energy            *PricingEnergy    `json:"energy,omitempty"`
	Labor             *PricingLabor     `json:"labor,omitempty"`
	Extras            []PricingExtra    `json:"extras,omitempty"`
	MinCharge         string            `json:"min_charge,omitempty"`
	Reserve           string            `json:"reserve,omitempty"`
	Shipping          *PricingShipping  `json:"shipping,omitempty"`
	Fees              []PricingFee      `json:"fees,omitempty"`
	Target            PricingTarget     `json:"target"`
	DiscountPercent   string            `json:"discount_percent,omitempty"`
}
type PricingProduction struct {
	Mode          string `json:"mode"`
	UnitsPerBatch int    `json:"units_per_batch,omitempty"`
	Grams         string `json:"grams,omitempty"`
	Hours         string `json:"hours,omitempty"`
}
type PricingMachine struct {
	Mode                string `json:"mode"`
	HourlyRate          string `json:"hourly_rate,omitempty"`
	Acquisition         string `json:"acquisition,omitempty"`
	Residual            string `json:"residual,omitempty"`
	LifetimeHours       string `json:"lifetime_hours,omitempty"`
	MaintenanceHourly   string `json:"maintenance_hourly,omitempty"`
	IncludesEnergy      bool   `json:"includes_energy,omitempty"`
	IncludesMaintenance bool   `json:"includes_maintenance,omitempty"`
}
type PricingEnergy struct {
	PowerWatts   string `json:"power_watts,omitempty"`
	TariffPerKWH string `json:"tariff_per_kwh,omitempty"`
}
type PricingLabor struct {
	Minutes    string `json:"minutes,omitempty"`
	HourlyRate string `json:"hourly_rate,omitempty"`
}
type PricingExtra struct {
	Label  string `json:"label"`
	Amount string `json:"amount"`
	Scope  string `json:"scope"`
}
type PricingShipping struct {
	Cost    string `json:"cost,omitempty"`
	Charged string `json:"charged,omitempty"`
}
type PricingFee struct {
	Percent string `json:"percent"`
	Fixed   string `json:"fixed"`
	Scope   string `json:"scope"`
	Base    string `json:"base"`
}
type PricingTarget struct {
	Anchor string `json:"anchor"`
	Value  string `json:"value"`
}
type PricingScenario struct {
	Price         string  `json:"price"`
	PricePerUnit  string  `json:"price_per_unit"`
	Fees          string  `json:"fees"`
	Gain          string  `json:"gain"`
	GainPerUnit   string  `json:"gain_per_unit"`
	MarginPercent *string `json:"margin_percent"`
	RevenueTotal  string  `json:"revenue_total"`
	CostTotal     string  `json:"cost_total"`
	CostPerUnit   string  `json:"cost_per_unit"`
}
type PricingBreakdown struct {
	Material    string `json:"material"`
	Machine     string `json:"machine"`
	Maintenance string `json:"maintenance"`
	Energy      string `json:"energy"`
	Labor       string `json:"labor"`
	Extras      string `json:"extras"`
	Reserve     string `json:"reserve"`
	Shipping    string `json:"shipping"`
}
type PricingCalculationResult struct {
	Currency                string           `json:"currency"`
	Quantity                int              `json:"quantity"`
	EffectivePlates         string           `json:"effective_plates"`
	ProductionMultiplier    string           `json:"production_multiplier"`
	TotalGrams              string           `json:"total_grams"`
	TotalHours              string           `json:"total_hours"`
	Breakdown               PricingBreakdown `json:"breakdown"`
	MinimumSalePrice        string           `json:"minimum_sale_price"`
	MinimumSalePricePerUnit string           `json:"minimum_sale_price_per_unit"`
	Base                    PricingScenario  `json:"base"`
	Discounted              *PricingScenario `json:"discounted,omitempty"`
	Warnings                []string         `json:"warnings"`
	Estimate                bool             `json:"estimate"`
}

var pricingDecimal = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,9})?$`)
var pricingCurrencies = map[string]int{"BRL": 2, "USD": 2, "EUR": 2, "GBP": 2, "CAD": 2, "AUD": 2, "JPY": 0, "CHF": 2, "CNY": 2, "INR": 2, "MXN": 2, "ARS": 2, "CLP": 0, "COP": 2, "NZD": 2, "SEK": 2, "NOK": 2, "DKK": 2, "PLN": 2, "CZK": 2}

func pInt(n int) *big.Rat         { return new(big.Rat).SetInt64(int64(n)) }
func pAdd(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func pSub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func pMul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func pDiv(a, b *big.Rat) *big.Rat { return new(big.Rat).Quo(a, b) }

// Rat.FloatString rounds halves away from zero; never introduce binary floats.
func pFormat(a *big.Rat, digits int) string { return a.FloatString(digits) }

// pCeilMoney requires a nonnegative threshold. The quotient/remainder keeps
// repeating rationals exact and avoids returning a loss-making minimum charge.
func pCeilMoney(a *big.Rat, digits int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(digits)), nil)
	scaled := new(big.Rat).Mul(a, new(big.Rat).SetInt(scale))
	units, remainder := new(big.Int), new(big.Int)
	units.QuoRem(scaled.Num(), scaled.Denom(), remainder)
	if remainder.Sign() > 0 {
		units.Add(units, big.NewInt(1))
	}
	return new(big.Rat).SetFrac(units, scale).FloatString(digits)
}

// CalculatePricing solves gain = P*(1-r) - K, where P is goods total,
// r is the sum of fee fractions, K = costs + fixed fees + shipping-base
// percentage fees - shipping charged. Margin is gain/P (not revenue).
// Price/gain targets are total amounts, not per-unit; negative actual gain is
// permitted. Costs exclude sale-dependent fees until the scenario is evaluated.
// Goods prices are rounded to currency precision BEFORE evaluating actual fees,
// gain and margin. Other outputs are independently rounded for display, so rounded
// cost rows may not sum. Minimum sale prices round UP to a currency minor unit.
// Margin includes shipping gain in its numerator but uses goods price as its
// denominator: it is an estimate, not accounting net profit.
func CalculatePricing(req PricingCalculationRequest) (*PricingCalculationResult, error) {
	digits, ok := pricingCurrencies[req.Currency]
	if !ok {
		return nil, fmt.Errorf("unsupported currency")
	}
	if req.Quantity < 1 || req.Quantity > 1000000 {
		return nil, fmt.Errorf("quantity must be 1..1000000")
	}
	if len(req.Extras) > 50 || len(req.Fees) > 50 {
		return nil, fmt.Errorf("at most 50 extras and 50 fees")
	}
	out := &PricingCalculationResult{Currency: req.Currency, Quantity: req.Quantity, Warnings: []string{}}
	var firstErr error
	read := func(name, s string, required bool) *big.Rat {
		if s == "" {
			if required && firstErr == nil {
				firstErr = fmt.Errorf("%s is required", name)
			}
			return pInt(0)
		}
		if len(s) > 20 || !pricingDecimal.MatchString(s) {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s must be a decimal string with at most 10 integer and 9 fractional digits", name)
			}
			return pInt(0)
		}
		r, _ := new(big.Rat).SetString(s)
		if r.Cmp(pInt(1000000000)) > 0 && firstErr == nil {
			firstErr = fmt.Errorf("%s exceeds 1000000000", name)
		}
		return r
	}
	warn := func(code string) { out.Warnings = append(out.Warnings, code) }
	optional := func(name, s string) *big.Rat {
		if s == "" {
			warn("missing_" + name)
		}
		return read(name, s, false)
	}
	q := pInt(req.Quantity)
	mult := q
	plates := pInt(0)
	switch req.Production.Mode {
	case "proportional":
		if req.Production.UnitsPerBatch != 0 {
			return nil, fmt.Errorf("units_per_batch requires batches mode")
		}
		warn("proportional_production_estimate")
	case "batches":
		u := req.Production.UnitsPerBatch
		if u < 1 || u > 1000000 {
			return nil, fmt.Errorf("units_per_batch must be 1..1000000")
		}
		n := (req.Quantity + u - 1) / u
		plates = pInt(n)
		mult = plates
		if req.Quantity%u != 0 {
			warn("full_batch_remainder")
		}
	default:
		return nil, fmt.Errorf("production.mode must be proportional or batches")
	}
	grams := pMul(optional("grams", req.Production.Grams), mult)
	hours := pMul(optional("hours", req.Production.Hours), mult)
	material := pDiv(pMul(grams, optional("material_cost_per_kg", req.MaterialCostPerKG)), pInt(1000))
	machine, maintenance, energy, labor, extras := pInt(0), pInt(0), pInt(0), pInt(0), pInt(0)
	includesEnergy := false
	if m := req.Machine; m != nil {
		includesEnergy = m.IncludesEnergy
		includesMaintenance := m.IncludesMaintenance
		var rate *big.Rat
		switch m.Mode {
		case "direct":
			if m.Acquisition != "" || m.Residual != "" || m.LifetimeHours != "" {
				return nil, fmt.Errorf("derived fields require derived machine mode")
			}
			rate = optional("machine_hourly_rate", m.HourlyRate)
		case "derived":
			if m.HourlyRate != "" || m.IncludesEnergy || m.IncludesMaintenance {
				return nil, fmt.Errorf("derived machine cannot include direct rate or inclusion flags")
			}
			a := read("acquisition", m.Acquisition, true)
			r := read("residual", m.Residual, true)
			life := read("lifetime_hours", m.LifetimeHours, true)
			if life.Sign() <= 0 || a.Cmp(r) < 0 {
				return nil, fmt.Errorf("derived machine requires positive lifetime and residual <= acquisition")
			}
			rate = pDiv(pSub(a, r), life)
		default:
			return nil, fmt.Errorf("machine.mode must be direct or derived")
		}
		machine = pMul(rate, hours)
		if includesMaintenance {
			if m.MaintenanceHourly != "" {
				return nil, fmt.Errorf("maintenance already included in machine rate")
			}
		} else {
			maintenance = pMul(optional("maintenance_hourly", m.MaintenanceHourly), hours)
		}
	} else {
		warn("missing_machine")
		warn("missing_maintenance_hourly")
	}
	if includesEnergy {
		if req.Energy != nil {
			return nil, fmt.Errorf("energy already included in machine rate")
		}
	} else if e := req.Energy; e != nil {
		energy = pDiv(pMul(pMul(optional("power_watts", e.PowerWatts), optional("tariff_per_kwh", e.TariffPerKWH)), hours), pInt(1000))
	} else {
		warn("missing_energy")
		warn("missing_power_watts")
		warn("missing_tariff_per_kwh")
	}
	if l := req.Labor; l != nil {
		labor = pDiv(pMul(pMul(optional("labor_minutes", l.Minutes), optional("labor_hourly_rate", l.HourlyRate)), q), pInt(60))
	} else {
		warn("missing_labor")
	}
	for _, e := range req.Extras {
		if len(e.Label) == 0 || len(e.Label) > 120 {
			return nil, fmt.Errorf("extra label must contain 1..120 bytes")
		}
		factor := pInt(1)
		switch e.Scope {
		case "unit":
			factor = q
		case "batch":
			if req.Production.Mode != "batches" {
				return nil, fmt.Errorf("batch extras require batches mode")
			}
			factor = plates
		case "project", "order":
		default:
			return nil, fmt.Errorf("unsupported extra scope")
		}
		extras = pAdd(extras, pMul(read("extra.amount", e.Amount, true), factor))
	}
	reserve := optional("reserve", req.Reserve)
	shippingCost, shippingCharged := pInt(0), pInt(0)
	if s := req.Shipping; s != nil {
		shippingCost = optional("shipping_cost", s.Cost)
		shippingCharged = optional("shipping_charged", s.Charged)
	} else {
		warn("missing_shipping")
	}
	if req.Extras == nil {
		warn("missing_extras")
	}
	if req.Fees == nil {
		warn("missing_fees")
	}
	costs := pInt(0)
	for _, v := range []*big.Rat{material, machine, maintenance, energy, labor, extras, reserve, shippingCost} {
		costs = pAdd(costs, v)
	}
	r, fixed, shippingFees := pInt(0), pInt(0), pInt(0)
	for _, f := range req.Fees {
		percent := read("fee.percent", f.Percent, true)
		if percent.Cmp(pInt(100)) >= 0 {
			return nil, fmt.Errorf("fee percent must be <100")
		}
		fraction := pDiv(percent, pInt(100))
		r = pAdd(r, fraction)
		factor := pInt(1)
		switch f.Scope {
		case "unit":
			factor = q
		case "order":
		default:
			return nil, fmt.Errorf("unsupported fee scope")
		}
		fixed = pAdd(fixed, pMul(read("fee.fixed", f.Fixed, true), factor))
		switch f.Base {
		case "goods":
		case "total":
			shippingFees = pAdd(shippingFees, pMul(fraction, shippingCharged))
		default:
			return nil, fmt.Errorf("unsupported fee base")
		}
	}
	denominator := pSub(pInt(1), r)
	if denominator.Sign() <= 0 {
		return nil, fmt.Errorf("combined fee percent must be <100")
	}
	k := pSub(pAdd(pAdd(costs, fixed), shippingFees), shippingCharged)
	minimum := pDiv(k, denominator)
	if minimum.Sign() < 0 {
		minimum = pInt(0)
	}
	// Only gain/margin targets permit a bounded negative decimal; all costs and
	// explicit selling prices continue to use the unsigned reader.
	targetText := req.Target.Value
	negativeTarget := (req.Target.Anchor == "gain" || req.Target.Anchor == "margin") && len(targetText) > 0 && targetText[0] == '-'
	if negativeTarget {
		targetText = targetText[1:]
	}
	target := read("target.value", targetText, true)
	if negativeTarget {
		target = new(big.Rat).Neg(target)
	}
	var price *big.Rat
	switch req.Target.Anchor {
	case "price":
		price = target
	case "gain":
		price = pDiv(pAdd(k, target), denominator)
	case "margin":
		margin := pDiv(target, pInt(100))
		d := pSub(denominator, margin)
		if d.Sign() <= 0 {
			return nil, fmt.Errorf("margin plus fees must be <100 percent")
		}
		price = pDiv(k, d)
	default:
		return nil, fmt.Errorf("target.anchor must be price, gain or margin")
	}
	if price.Sign() < 0 {
		return nil, fmt.Errorf("target implies negative goods price; use price anchor instead")
	}
	floor := read("min_charge", req.MinCharge, false)
	// A configured floor must be attainable at the currency's precision;
	// nearest rounding after enforcement must never take the charge below it.
	floor, _ = new(big.Rat).SetString(pCeilMoney(floor, digits))
	if req.Target.Anchor != "price" && price.Cmp(floor) < 0 {
		price = floor
		warn("minimum_charge_applied")
	} else if req.Target.Anchor == "price" {
		actualPrice, _ := new(big.Rat).SetString(pFormat(price, digits))
		if actualPrice.Cmp(floor) < 0 {
			warn("price_below_minimum_charge")
		}
	}
	if price.Sign() < 0 {
		return nil, fmt.Errorf("target implies negative goods price; use price anchor instead")
	}
	discount := read("discount_percent", req.DiscountPercent, false)
	if discount.Cmp(pInt(100)) > 0 {
		return nil, fmt.Errorf("discount_percent must be <=100")
	}
	if firstErr != nil {
		return nil, firstErr
	}
	money := func(v *big.Rat) string { return pFormat(v, digits) }
	// Actual gain follows the currency-rounded charge, not the ideal target.
	price, _ = new(big.Rat).SetString(money(price))
	scenario := func(p *big.Rat) PricingScenario {
		p, _ = new(big.Rat).SetString(money(p))
		fees := pAdd(pAdd(pMul(p, r), fixed), shippingFees)
		totalCost := pAdd(costs, fees)
		revenue := pAdd(p, shippingCharged)
		gain := pSub(revenue, totalCost)
		var margin *string
		if p.Sign() != 0 {
			s := pFormat(pMul(pDiv(gain, p), pInt(100)), 2)
			margin = &s
		}
		return PricingScenario{Price: money(p), PricePerUnit: money(pDiv(p, q)), Fees: money(fees), Gain: money(gain), GainPerUnit: money(pDiv(gain, q)), MarginPercent: margin, RevenueTotal: money(revenue), CostTotal: money(totalCost), CostPerUnit: money(pDiv(totalCost, q))}
	}
	out.EffectivePlates = pFormat(plates, 0)
	out.ProductionMultiplier = pFormat(mult, 0)
	out.TotalGrams = pFormat(grams, 6)
	out.TotalHours = pFormat(hours, 6)
	out.Breakdown = PricingBreakdown{money(material), money(machine), money(maintenance), money(energy), money(labor), money(extras), money(reserve), money(shippingCost)}
	out.MinimumSalePrice = pCeilMoney(minimum, digits)
	out.MinimumSalePricePerUnit = pCeilMoney(pDiv(minimum, q), digits)
	out.Base = scenario(price)
	if pSub(pMul(price, denominator), k).Sign() < 0 {
		warn("negative_gain")
	}
	if req.DiscountPercent != "" {
		s := scenario(pMul(price, pSub(pInt(1), pDiv(discount, pInt(100)))))
		out.Discounted = &s
		discountedPrice, _ := new(big.Rat).SetString(s.Price)
		if pSub(pMul(discountedPrice, denominator), k).Sign() < 0 {
			warn("discounted_loss")
		}
	}
	out.Estimate = len(out.Warnings) > 0
	return out, nil
}
