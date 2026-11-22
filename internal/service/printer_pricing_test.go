package service

import (
	"github.com/Brook-sys/picofarm/internal/model"
	"testing"
)

func TestPrinterPricingValidation(t *testing.T) {
	if err := validatePrinterPricing(&model.Printer{}); err != nil {
		t.Fatal(err)
	}
	valid := model.PrinterPricingConfig{Currency: "BRL", RateMode: "direct", DirectRatePerHour: "0"}
	for _, currency := range []string{"BRL", "USD", "EUR", "GBP", "CAD", "AUD", "JPY", "CHF", "NZD", "MXN", "ARS", "CLP", "COP", "INR", "CNY", "SEK", "NOK", "DKK", "PLN", "CZK"} {
		c := valid
		c.Currency = currency
		if err := validatePrinterPricing(&model.Printer{PricingConfig: &c}); err != nil {
			t.Fatal(currency, err)
		}
	}
	cases := map[string]func(*model.PrinterPricingConfig){
		"multiple currencies": func(c *model.PrinterPricingConfig) { c.Currency = "BRL USD" },
		"missing currency":    func(c *model.PrinterPricingConfig) { c.Currency = "" },
		"unknown currency":    func(c *model.PrinterPricingConfig) { c.Currency = "BTC" },
		"missing rate":        func(c *model.PrinterPricingConfig) { c.DirectRatePerHour = "" },
		"mode":                func(c *model.PrinterPricingConfig) { c.RateMode = "automatic" },
		"negative":            func(c *model.PrinterPricingConfig) { c.EnergyTariff = "-1" },
		"exponent":            func(c *model.PrinterPricingConfig) { c.AveragePowerWatts = "1e3" },
		"precision":           func(c *model.PrinterPricingConfig) { c.MaintenancePerHour = "0.0000000001" },
		"overflow":            func(c *model.PrinterPricingConfig) { c.PurchasePrice = "1000000000" },
		"NaN":                 func(c *model.PrinterPricingConfig) { c.ResidualValue = "NaN" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := valid
			mutate(&c)
			if validatePrinterPricing(&model.Printer{PricingConfig: &c}) == nil {
				t.Fatal("accepted invalid profile")
			}
		})
	}
	derived := model.PrinterPricingConfig{Currency: "BRL", RateMode: "derived", PurchasePrice: "2000.00", ResidualValue: "200", ProductiveLifetimeHours: "10000"}
	if err := validatePrinterPricing(&model.Printer{PricingConfig: &derived}); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"energy", "maintenance", "zero hours", "residual"} {
		c := derived
		switch field {
		case "energy":
			c.IncludesEnergy = true
		case "maintenance":
			c.IncludesMaintenance = true
		case "zero hours":
			c.ProductiveLifetimeHours = "0.000000"
		case "residual":
			c.ResidualValue = "2000.01"
		}
		if validatePrinterPricing(&model.Printer{PricingConfig: &c}) == nil {
			t.Fatal("accepted", field)
		}
	}
}
