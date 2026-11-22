package model

// PrinterPricingConfig is an explicit, self-contained costing profile. Monetary
// values are major currency units, not cents. Nil means unconfigured, not free.
// In direct mode, includes flags identify costs already contained in the rate.
// In derived mode the depreciation rate excludes energy and maintenance, so both
// flags must be false. Consumers must not add included costs a second time.
type PrinterPricingConfig struct {
	Currency                string `json:"currency"`
	RateMode                string `json:"rate_mode"`
	DirectRatePerHour       string `json:"direct_rate_per_hour"`
	PurchasePrice           string `json:"purchase_price"`
	ResidualValue           string `json:"residual_value"`
	ProductiveLifetimeHours string `json:"productive_lifetime_hours"`
	MaintenancePerHour      string `json:"maintenance_per_hour"`
	AveragePowerWatts       string `json:"average_power_watts"`
	EnergyTariff            string `json:"energy_tariff"`
	IncludesEnergy          bool   `json:"includes_energy"`
	IncludesMaintenance     bool   `json:"includes_maintenance"`
}
