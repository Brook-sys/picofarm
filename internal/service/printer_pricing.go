package service

import (
	"math/big"
	"regexp"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/validation"
)

var printerPricingDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})(\.[0-9]{1,9})?$`)

// Decimal values are finite, nonnegative, at most 9 integer and 9 fractional
// digits. Empty values mean unknown, never zero; only the active rate inputs
// are required. Arithmetic here stays exact, without float conversion.
func validatePrinterPricing(p *model.Printer) error {
	c := p.PricingConfig
	if c == nil {
		return nil
	}
	var errors []validation.FieldError
	add := func(field, message string) {
		errors = append(errors, validation.FieldError{Field: "pricing_config." + field, Message: message})
	}
	supported := false
	for _, currency := range strings.Fields("BRL USD EUR GBP CAD AUD JPY CHF NZD MXN ARS CLP COP INR CNY SEK NOK DKK PLN CZK") {
		if c.Currency == currency {
			supported = true
			break
		}
	}
	if !supported {
		add("currency", "select a supported currency")
	}
	if c.RateMode != "direct" && c.RateMode != "derived" {
		add("rate_mode", "must be direct or derived")
	}
	values := map[string]string{
		"direct_rate_per_hour": c.DirectRatePerHour, "purchase_price": c.PurchasePrice,
		"residual_value": c.ResidualValue, "productive_lifetime_hours": c.ProductiveLifetimeHours,
		"maintenance_per_hour": c.MaintenancePerHour, "average_power_watts": c.AveragePowerWatts,
		"energy_tariff": c.EnergyTariff,
	}
	for field, value := range values {
		if value != "" && !printerPricingDecimal.MatchString(value) {
			add(field, "must be a nonnegative decimal with up to 9 integer and 9 fractional digits")
		}
	}
	require := func(field string) {
		if values[field] == "" {
			add(field, "is required for this rate mode")
		}
	}
	if c.RateMode == "direct" {
		require("direct_rate_per_hour")
	}
	if c.RateMode == "derived" {
		require("purchase_price")
		require("residual_value")
		require("productive_lifetime_hours")
		if printerPricingDecimal.MatchString(c.ProductiveLifetimeHours) {
			if hours, ok := new(big.Rat).SetString(c.ProductiveLifetimeHours); ok && hours.Sign() == 0 {
				add("productive_lifetime_hours", "must be greater than zero")
			}
		}
		if printerPricingDecimal.MatchString(c.PurchasePrice) && printerPricingDecimal.MatchString(c.ResidualValue) {
			purchase, purchaseOK := new(big.Rat).SetString(c.PurchasePrice)
			residual, residualOK := new(big.Rat).SetString(c.ResidualValue)
			if purchaseOK && residualOK && residual.Cmp(purchase) > 0 {
				add("residual_value", "must not exceed purchase price")
			}
		}
		if c.IncludesEnergy {
			add("includes_energy", "derived depreciation excludes energy")
		}
		if c.IncludesMaintenance {
			add("includes_maintenance", "derived depreciation excludes maintenance")
		}
	}
	if len(errors) != 0 {
		return &validation.ValidationError{Errors: errors}
	}
	return nil
}
