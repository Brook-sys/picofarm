package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type SaleService struct {
	repo     *repository.SaleRepository
	taskRepo *repository.TaskRepository
}

// Create creates a new sale.
func (s *SaleService) Create(ctx context.Context, sale *model.Sale) error {
	// Calculate net if not provided
	if sale.NetCents == 0 {
		sale.NetCents = sale.GrossCents - sale.FeesCents - sale.ShippingCostCents
	}
	return s.repo.Create(ctx, sale)
}

// GetByID retrieves a sale by ID.
func (s *SaleService) GetByID(ctx context.Context, id uuid.UUID) (*model.Sale, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all sales.
func (s *SaleService) List(ctx context.Context, projectID *uuid.UUID) ([]model.Sale, error) {
	return s.repo.List(ctx, projectID)
}

// Update updates a sale.
func (s *SaleService) Update(ctx context.Context, sale *model.Sale) error {
	// Recalculate net
	sale.NetCents = sale.GrossCents - sale.FeesCents - sale.ShippingCostCents
	return s.repo.Update(ctx, sale)
}

// Delete deletes a sale.
func (s *SaleService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// GetProfitSummary calculates profit for a date range.
func (s *SaleService) GetProfitSummary(ctx context.Context, start, end time.Time) (grossCents, netCents, feesCents int, count int, err error) {
	return s.repo.GetTotalsByDateRange(ctx, start, end)
}

// WeekSummary holds aggregated sales totals for a single week.
type WeekSummary struct {
	GrossCents int `json:"gross_cents"`
	NetCents   int `json:"net_cents"`
	FeesCents  int `json:"fees_cents"`
	Count      int `json:"count"`
}

// WeeklyInsights holds this-week vs last-week comparison data.
type WeeklyInsights struct {
	ThisWeek            WeekSummary        `json:"this_week"`
	LastWeek            WeekSummary        `json:"last_week"`
	Channels            []ChannelBreakdown `json:"channels"`
	WeekStart           string             `json:"week_start"`
	WeekEnd             string             `json:"week_end"`
	PendingCount        int                `json:"pending_count"`
	PendingRevenueCents int                `json:"pending_revenue_cents"`
}

// GetWeeklyInsights returns this week's sales metrics with last week comparison.
func (s *SaleService) GetWeeklyInsights(ctx context.Context) (*WeeklyInsights, error) {
	now := time.Now().UTC()

	// Monday of this week
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	thisMonday := time.Date(now.Year(), now.Month(), now.Day()-int(weekday-time.Monday), 0, 0, 0, 0, time.UTC)
	thisSunday := thisMonday.AddDate(0, 0, 7) // exclusive end

	lastMonday := thisMonday.AddDate(0, 0, -7)

	// This week totals
	twGross, twNet, twFees, twCount, err := s.repo.GetTotalsByDateRange(ctx, thisMonday, thisSunday)
	if err != nil {
		return nil, fmt.Errorf("this week totals: %w", err)
	}

	// Last week totals
	lwGross, lwNet, lwFees, lwCount, err := s.repo.GetTotalsByDateRange(ctx, lastMonday, thisMonday)
	if err != nil {
		return nil, fmt.Errorf("last week totals: %w", err)
	}

	// Channel breakdown for this week
	channelRows, err := s.repo.GetSalesByChannel(ctx, thisMonday)
	if err != nil {
		return nil, fmt.Errorf("channel breakdown: %w", err)
	}
	channels := make([]ChannelBreakdown, len(channelRows))
	for i, r := range channelRows {
		channels[i] = ChannelBreakdown{Channel: r.Channel, Total: r.Total, Count: r.Count}
	}

	// Pending sales: tasks in pending/in_progress linked to priced projects
	var pendingCount, pendingRevenue int
	if s.taskRepo != nil {
		pendingCount, pendingRevenue, err = s.taskRepo.GetPendingSalesStats(ctx)
		if err != nil {
			return nil, fmt.Errorf("pending sales: %w", err)
		}
	}

	return &WeeklyInsights{
		ThisWeek:            WeekSummary{GrossCents: twGross, NetCents: twNet, FeesCents: twFees, Count: twCount},
		LastWeek:            WeekSummary{GrossCents: lwGross, NetCents: lwNet, FeesCents: lwFees, Count: lwCount},
		Channels:            channels,
		WeekStart:           thisMonday.Format("2006-01-02"),
		WeekEnd:             thisSunday.AddDate(0, 0, -1).Format("2006-01-02"),
		PendingCount:        pendingCount,
		PendingRevenueCents: pendingRevenue,
	}, nil
}

// FinancialSummary contains aggregated financial data.
type FinancialSummary struct {
	TotalExpensesCents     int     `json:"total_expenses_cents"`
	TotalSalesGrossCents   int     `json:"total_sales_gross_cents"`
	TotalSalesNetCents     int     `json:"total_sales_net_cents"`
	TotalFeesCents         int     `json:"total_fees_cents"`
	TotalMaterialCost      float64 `json:"total_material_cost"`
	TotalMaterialUsedGrams float64 `json:"total_material_used_grams"`
	TotalCOGSCents         int     `json:"total_cogs_cents"`
	NetProfitCents         int     `json:"net_profit_cents"`
	ConfirmedExpenseCount  int     `json:"confirmed_expense_count"`
	PendingExpenseCount    int     `json:"pending_expense_count"`
	SalesCount             int     `json:"sales_count"`
	CompletedPrintCount    int     `json:"completed_print_count"`
	SuccessfulPrintCount   int     `json:"successful_print_count"`
}

// StatsService handles financial statistics and aggregations.
type StatsService struct {
	expenseRepo    *repository.ExpenseRepository
	saleRepo       *repository.SaleRepository
	printJobRepo   *repository.PrintJobRepository
	queueRepo      *repository.QueueItemRepository
	spoolRepo      *repository.SpoolRepository
	projectService *ProjectService
}

type UsageStats struct {
	TotalPrints         int     `json:"total_prints"`
	SuccessfulPrints    int     `json:"successful_prints"`
	FailedPrints        int     `json:"failed_prints"`
	TotalPrintHours     float64 `json:"total_print_hours"`
	TotalFilamentUsed   float64 `json:"total_filament_used_grams"`
	TotalFilamentWasted float64 `json:"total_filament_wasted_grams"`
	SpoolsInUse         int     `json:"spools_in_use"`
}

func (s *StatsService) GetUsageStats(ctx context.Context, since *time.Time) (*UsageStats, error) {
	stats := &UsageStats{}
	inPeriod := func(t time.Time) bool {
		return since == nil || !t.Before(*since)
	}
	if s.printJobRepo != nil {
		jobs, err := s.printJobRepo.List(ctx, nil, nil)
		if err != nil {
			return nil, err
		}
		for _, job := range jobs {
			if job.CompletedAt != nil && !inPeriod(*job.CompletedAt) {
				continue
			}
			if job.CompletedAt == nil && !inPeriod(job.CreatedAt) {
				continue
			}
			stats.TotalPrints++
			if job.Outcome != nil && job.Outcome.Success {
				stats.SuccessfulPrints++
			} else if job.Outcome != nil && !job.Outcome.Success {
				stats.FailedPrints++
			}
			if job.ActualSeconds != nil {
				stats.TotalPrintHours += float64(*job.ActualSeconds) / 3600.0
			}
			if job.MaterialUsedGrams != nil {
				stats.TotalFilamentUsed += *job.MaterialUsedGrams
				if job.Outcome != nil && !job.Outcome.Success {
					stats.TotalFilamentWasted += *job.MaterialUsedGrams
				}
			}
		}
	}
	if s.queueRepo != nil {
		items, err := s.queueRepo.ListTerminalDirect(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !inPeriod(item.UpdatedAt) {
				continue
			}
			currentAttemptDone := item.Status == model.QueueItemStatusDone
			currentAttemptFailed := item.Status == model.QueueItemStatusFailed || item.Status == model.QueueItemStatusCancelled
			failedAttempts := item.FailedAttempts
			if currentAttemptFailed && failedAttempts == 0 {
				failedAttempts = 1
			}
			stats.TotalPrints += failedAttempts
			if currentAttemptDone {
				stats.TotalPrints++
				stats.SuccessfulPrints++
			}
			stats.FailedPrints += failedAttempts
			consumedGrams := queueItemConsumedGrams(item)
			if item.EstimatedSeconds != nil {
				stats.TotalPrintHours += float64(queueItemConsumedSeconds(item)) / 3600.0
			}
			stats.TotalFilamentWasted += item.WastedGrams
			stats.TotalFilamentUsed += item.WastedGrams
			if currentAttemptDone {
				stats.TotalFilamentUsed += consumedGrams
			} else if currentAttemptFailed && item.WastedGrams == 0 {
				stats.TotalFilamentWasted += consumedGrams
				stats.TotalFilamentUsed += consumedGrams
			}
		}
	}
	if s.spoolRepo != nil {
		spools, err := s.spoolRepo.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, spool := range spools {
			if spool.Status == model.SpoolStatusInUse {
				stats.SpoolsInUse++
			}
		}
	}
	return stats, nil
}

func queueItemConsumedGrams(item model.QueueItem) float64 {
	if item.FilamentGrams == nil {
		return 0
	}
	if item.Status == model.QueueItemStatusDone {
		return *item.FilamentGrams
	}
	progress := item.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	if progress == 0 {
		progress = queueItemElapsedProgress(item)
	}
	return *item.FilamentGrams * progress / 100
}

func queueItemConsumedSeconds(item model.QueueItem) int {
	if item.EstimatedSeconds == nil {
		return 0
	}
	if item.Status == model.QueueItemStatusDone {
		return *item.EstimatedSeconds
	}
	progress := item.Progress
	if progress <= 0 {
		progress = queueItemElapsedProgress(item)
	}
	if progress > 100 {
		progress = 100
	}
	return int(float64(*item.EstimatedSeconds) * progress / 100)
}

func queueItemElapsedProgress(item model.QueueItem) float64 {
	if item.EstimatedSeconds == nil || *item.EstimatedSeconds <= 0 {
		return 0
	}
	elapsed := item.UpdatedAt.Sub(item.CreatedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	progress := elapsed / float64(*item.EstimatedSeconds) * 100
	return min(progress, 100)
}

// GetFinancialSummary returns aggregated financial data.
// If since is non-nil, only data from that time onward is included.
func (s *StatsService) GetFinancialSummary(ctx context.Context, since *time.Time) (*FinancialSummary, error) {
	summary := &FinancialSummary{}

	// Get expense totals
	confirmedStatus := model.ExpenseStatusConfirmed
	var expenses []model.Expense
	var err error
	if since != nil {
		expenses, err = s.expenseRepo.ListSince(ctx, &confirmedStatus, *since)
	} else {
		expenses, err = s.expenseRepo.List(ctx, &confirmedStatus)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get expenses: %w", err)
	}
	for _, exp := range expenses {
		summary.TotalExpensesCents += exp.TotalCents
		summary.ConfirmedExpenseCount++
	}

	// Get pending expense count
	pendingStatus := model.ExpenseStatusPending
	var pendingExpenses []model.Expense
	if since != nil {
		pendingExpenses, err = s.expenseRepo.ListSince(ctx, &pendingStatus, *since)
	} else {
		pendingExpenses, err = s.expenseRepo.List(ctx, &pendingStatus)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get pending expenses: %w", err)
	}
	summary.PendingExpenseCount = len(pendingExpenses)

	// Get sales totals
	var sales []model.Sale
	if since != nil {
		sales, err = s.saleRepo.ListSince(ctx, *since)
	} else {
		sales, err = s.saleRepo.List(ctx, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sales: %w", err)
	}
	for _, sale := range sales {
		summary.TotalSalesGrossCents += sale.GrossCents
		summary.TotalSalesNetCents += sale.NetCents
		summary.TotalFeesCents += sale.FeesCents
		summary.SalesCount++
	}

	// Get print job stats (completed jobs with outcomes)
	var jobs []model.PrintJob
	if since != nil {
		jobs, err = s.printJobRepo.ListCompletedSince(ctx, *since)
	} else {
		completedStatus := model.PrintJobStatusCompleted
		jobs, err = s.printJobRepo.List(ctx, nil, &completedStatus)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get print jobs: %w", err)
	}
	for _, job := range jobs {
		summary.CompletedPrintCount++
		if job.Outcome != nil {
			if job.Outcome.Success {
				summary.SuccessfulPrintCount++
			}
		}
	}

	if s.queueRepo != nil {
		items, err := s.queueRepo.ListTerminalDirect(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get queue items: %w", err)
		}
		for _, item := range items {
			if item.Status == model.QueueItemStatusDone {
				summary.TotalMaterialUsedGrams += queueItemConsumedGrams(item)
			}
		}
	}

	// Aggregate material and cost data from sales-linked project summaries
	if s.projectService != nil {
		projectIDs := make(map[uuid.UUID]bool)
		for _, sale := range sales {
			if sale.ProjectID != nil {
				projectIDs[*sale.ProjectID] = true
			}
		}

		for pid := range projectIDs {
			ps, err := s.projectService.GetProjectSummary(ctx, pid)
			if err != nil || ps == nil {
				continue
			}
			// Material grams: actual from jobs + estimated from slice profiles
			summary.TotalMaterialUsedGrams += ps.TotalMaterialGrams + ps.EstimatedMaterialGrams
			// Material cost: actual from jobs + estimated from profiles
			summary.TotalMaterialCost += float64(ps.MaterialCostCents+ps.EstimatedMaterialCostCents) / 100.0
			// Total COGS includes material + printer time + supplies (already × sales count)
			summary.TotalCOGSCents += ps.TotalCostCents
		}
	}

	// Net profit: sales net revenue - total COGS
	summary.NetProfitCents = summary.TotalSalesNetCents - summary.TotalCOGSCents

	return summary, nil
}

// TimeSeriesPoint represents a single data point in a time series.
type TimeSeriesPoint struct {
	Date     string `json:"date"`
	Revenue  int    `json:"revenue"`
	Expenses int    `json:"expenses"`
	Profit   int    `json:"profit"`
}

// TimeSeriesData represents the full time series response.
type TimeSeriesData struct {
	Points []TimeSeriesPoint `json:"points"`
	Period string            `json:"period"`
}

// CategoryBreakdown represents an expense category breakdown.
type CategoryBreakdown struct {
	Category string `json:"category"`
	Total    int    `json:"total"`
	Count    int    `json:"count"`
}

// ChannelBreakdown represents a sales channel breakdown.
type ChannelBreakdown struct {
	Channel string `json:"channel"`
	Total   int    `json:"total"`
	Count   int    `json:"count"`
}

// parsePeriod converts a period string to (since time, strftime format).
func parsePeriod(period string) (time.Time, string) {
	now := time.Now()
	switch period {
	case "90d":
		return now.AddDate(0, 0, -90), "%Y-W%W"
	case "12m":
		return now.AddDate(-1, 0, 0), "%Y-%m"
	default: // "30d"
		return now.AddDate(0, 0, -30), "%Y-%m-%d"
	}
}

// GetTimeSeriesData returns aligned revenue, expenses, and profit time series data.
func (s *StatsService) GetTimeSeriesData(ctx context.Context, period string) (*TimeSeriesData, error) {
	since, strftimeFmt := parsePeriod(period)

	revenueSeries, err := s.saleRepo.GetSalesOverTime(ctx, since, strftimeFmt)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue series: %w", err)
	}

	expenseSeries, err := s.expenseRepo.GetExpensesOverTime(ctx, since, strftimeFmt)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense series: %w", err)
	}

	// Merge into aligned date buckets
	allDates := make(map[string]bool)
	revenueMap := make(map[string]int)
	expenseMap := make(map[string]int)

	for _, r := range revenueSeries {
		allDates[r.DateBucket] = true
		revenueMap[r.DateBucket] = r.Total
	}
	for _, e := range expenseSeries {
		allDates[e.DateBucket] = true
		expenseMap[e.DateBucket] = e.Total
	}

	// Sort dates
	dates := make([]string, 0, len(allDates))
	for d := range allDates {
		dates = append(dates, d)
	}
	sortStrings(dates)

	points := make([]TimeSeriesPoint, len(dates))
	for i, d := range dates {
		rev := revenueMap[d]
		exp := expenseMap[d]
		points[i] = TimeSeriesPoint{
			Date:     d,
			Revenue:  rev,
			Expenses: exp,
			Profit:   rev - exp,
		}
	}

	return &TimeSeriesData{
		Points: points,
		Period: period,
	}, nil
}

// GetExpensesByCategory returns expense totals grouped by category.
func (s *StatsService) GetExpensesByCategory(ctx context.Context, period string) ([]CategoryBreakdown, error) {
	since, _ := parsePeriod(period)
	rows, err := s.expenseRepo.GetExpensesByCategory(ctx, since)
	if err != nil {
		return nil, err
	}

	result := make([]CategoryBreakdown, len(rows))
	for i, r := range rows {
		result[i] = CategoryBreakdown{
			Category: r.Category,
			Total:    r.Total,
			Count:    r.Count,
		}
	}
	return result, nil
}

// GetSalesByChannel returns sales totals grouped by channel.
func (s *StatsService) GetSalesByChannel(ctx context.Context, period string) ([]ChannelBreakdown, error) {
	since, _ := parsePeriod(period)
	rows, err := s.saleRepo.GetSalesByChannel(ctx, since)
	if err != nil {
		return nil, err
	}

	result := make([]ChannelBreakdown, len(rows))
	for i, r := range rows {
		result[i] = ChannelBreakdown{
			Channel: r.Channel,
			Total:   r.Total,
			Count:   r.Count,
		}
	}
	return result, nil
}

// ProjectSales represents aggregated sales data for a project.
type ProjectSales struct {
	ProjectID             string `json:"project_id"`
	ProjectName           string `json:"project_name"`
	GrossCents            int    `json:"gross_cents"`
	NetCents              int    `json:"net_cents"`
	Count                 int    `json:"count"`
	AvgCents              int    `json:"avg_cents"`
	UnitCostCents         int    `json:"unit_cost_cents"`
	TotalCOGS             int    `json:"total_cogs_cents"`
	ProfitCents           int    `json:"profit_cents"`
	EstimatedPrintSeconds int    `json:"estimated_print_seconds"`
	TotalPrintSeconds     int    `json:"total_print_seconds"`
	FirstSale             string `json:"first_sale"`
	LastSale              string `json:"last_sale"`
}

// GetSalesByProject returns sales aggregated by project.
func (s *StatsService) GetSalesByProject(ctx context.Context) ([]ProjectSales, error) {
	rows, err := s.saleRepo.GetSalesByProject(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]ProjectSales, len(rows))
	for i, r := range rows {
		ps := ProjectSales{
			ProjectID:   r.ProjectID,
			ProjectName: r.ProjectName,
			GrossCents:  r.GrossCents,
			NetCents:    r.NetCents,
			Count:       r.Count,
			AvgCents:    r.AvgCents,
			FirstSale:   r.FirstSale,
			LastSale:    r.LastSale,
		}

		// Enrich with cost data from project summary
		if s.projectService != nil && r.ProjectID != "" {
			projectID, err := uuid.Parse(r.ProjectID)
			if err == nil {
				summary, err := s.projectService.GetProjectSummary(ctx, projectID)
				if err == nil && summary != nil {
					ps.UnitCostCents = summary.UnitCostCents
					ps.TotalCOGS = summary.TotalCostCents
					ps.ProfitCents = ps.NetCents - ps.TotalCOGS
					ps.EstimatedPrintSeconds = summary.EstimatedPrintSeconds
					ps.TotalPrintSeconds = summary.TotalPrintSeconds
				}
			}
		}

		result[i] = ps
	}
	return result, nil
}

// sortStrings sorts a slice of strings in ascending order.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// TemplateService handles template business logic.
