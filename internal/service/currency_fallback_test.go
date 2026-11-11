package service

import (
	"context"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

func newCurrencyFallbackServices(t *testing.T) (*repository.Repositories, *SettingsService, *SaleService, *StatsService) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repos := repository.NewRepositories(db)
	settings := &SettingsService{repo: repos.Settings}
	return repos, settings,
		&SaleService{repo: repos.Sales, settings: settings},
		&StatsService{
			expenseRepo:  repos.Expenses,
			saleRepo:     repos.Sales,
			printJobRepo: repos.PrintJobs,
			queueRepo:    repos.QueueItems,
			spoolRepo:    repos.Spools,
			materialRepo: repos.Materials,
			settings:     settings,
		}
}

func TestSaleServiceCreateUsesConfiguredCurrencyAsFallback(t *testing.T) {
	repos, settings, sales, _ := newCurrencyFallbackServices(t)
	ctx := context.Background()
	if err := settings.Set(ctx, CurrencySettingKey, "BRL"); err != nil {
		t.Fatal(err)
	}

	sale := &model.Sale{OccurredAt: time.Now(), GrossCents: 2500, Quantity: 1}
	if err := sales.Create(ctx, sale); err != nil {
		t.Fatal(err)
	}
	stored, err := repos.Sales.GetByID(ctx, sale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Currency != "BRL" {
		t.Fatalf("expected BRL fallback, got %q", stored.Currency)
	}
}

func TestSaleServiceCreatePreservesExplicitCurrency(t *testing.T) {
	_, settings, sales, _ := newCurrencyFallbackServices(t)
	ctx := context.Background()
	if err := settings.Set(ctx, CurrencySettingKey, "BRL"); err != nil {
		t.Fatal(err)
	}

	sale := &model.Sale{OccurredAt: time.Now(), GrossCents: 2500, Quantity: 1, Currency: "EUR"}
	if err := sales.Create(ctx, sale); err != nil {
		t.Fatal(err)
	}
	if sale.Currency != "EUR" {
		t.Fatalf("expected explicit EUR to be preserved, got %q", sale.Currency)
	}
}

func TestSaleServiceUpdatePreservesStoredCurrencyWhenOmitted(t *testing.T) {
	repos, settings, sales, _ := newCurrencyFallbackServices(t)
	ctx := context.Background()
	if err := settings.Set(ctx, CurrencySettingKey, "BRL"); err != nil {
		t.Fatal(err)
	}
	sale := &model.Sale{OccurredAt: time.Now(), GrossCents: 1000, Quantity: 1, Currency: "EUR"}
	if err := sales.Create(ctx, sale); err != nil {
		t.Fatal(err)
	}
	update := *sale
	update.Currency = ""
	update.GrossCents = 1200
	if err := sales.Update(ctx, &update); err != nil {
		t.Fatal(err)
	}
	stored, err := repos.Sales.GetByID(ctx, sale.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Currency != "EUR" {
		t.Fatalf("expected stored EUR to be preserved, got %q", stored.Currency)
	}
}

func TestStatsServiceFinancialSummaryExcludesOtherCurrencies(t *testing.T) {
	repos, settings, sales, stats := newCurrencyFallbackServices(t)
	ctx := context.Background()
	if err := settings.Set(ctx, CurrencySettingKey, "BRL"); err != nil {
		t.Fatal(err)
	}
	for _, sale := range []*model.Sale{
		{OccurredAt: time.Now(), GrossCents: 1000, NetCents: 900, Quantity: 1, Currency: "BRL"},
		{OccurredAt: time.Now(), GrossCents: 5000, NetCents: 4500, Quantity: 1, Currency: "USD"},
	} {
		if err := sales.Create(ctx, sale); err != nil {
			t.Fatal(err)
		}
	}
	for _, expense := range []*model.Expense{
		{OccurredAt: time.Now(), TotalCents: 300, Status: model.ExpenseStatusConfirmed, Currency: "BRL"},
		{OccurredAt: time.Now(), TotalCents: 700, Status: model.ExpenseStatusConfirmed, Currency: "USD"},
	} {
		if err := repos.Expenses.Create(ctx, expense); err != nil {
			t.Fatal(err)
		}
	}

	summary, err := stats.GetFinancialSummary(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalSalesGrossCents != 1000 || summary.TotalSalesNetCents != 900 || summary.SalesCount != 1 {
		t.Fatalf("unexpected sales summary: %#v", summary)
	}
	if summary.TotalExpensesCents != 300 || summary.ConfirmedExpenseCount != 1 {
		t.Fatalf("unexpected expense summary: %#v", summary)
	}
}

func TestRepositoryAggregatesFilterByCurrency(t *testing.T) {
	repos, _, sales, _ := newCurrencyFallbackServices(t)
	ctx := context.Background()
	now := time.Now()
	for _, sale := range []*model.Sale{
		{OccurredAt: now, GrossCents: 1000, NetCents: 900, Quantity: 1, Currency: "BRL"},
		{OccurredAt: now, GrossCents: 5000, NetCents: 4500, Quantity: 1, Currency: "USD"},
	} {
		if err := sales.Create(ctx, sale); err != nil {
			t.Fatal(err)
		}
	}
	gross, net, _, count, err := repos.Sales.GetTotalsByDateRange(ctx, now.Add(-time.Hour), now.Add(time.Hour), "BRL")
	if err != nil {
		t.Fatal(err)
	}
	if gross != 1000 || net != 900 || count != 1 {
		t.Fatalf("expected BRL-only aggregate, got gross=%d net=%d count=%d", gross, net, count)
	}
}

func TestExpenseCurrencyFallbackUsesConfiguredCurrency(t *testing.T) {
	repos, settings, _, _ := newCurrencyFallbackServices(t)
	ctx := context.Background()
	if err := settings.Set(ctx, CurrencySettingKey, "BRL"); err != nil {
		t.Fatal(err)
	}
	expense := &model.Expense{OccurredAt: time.Now(), TotalCents: 500}
	service := &ExpenseService{repo: repos.Expenses, settingsRepo: repos.Settings}
	if err := service.applyCurrencyFallback(ctx, expense); err != nil {
		t.Fatal(err)
	}
	if expense.Currency != "BRL" {
		t.Fatalf("expected BRL fallback, got %q", expense.Currency)
	}
}
