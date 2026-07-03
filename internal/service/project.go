package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

type ProjectService struct {
	repo         *repository.ProjectRepository
	printJobRepo *repository.PrintJobRepository
	printerRepo  *repository.PrinterRepository
	spoolRepo    *repository.SpoolRepository
	templateRepo *repository.TemplateRepository
	designRepo   *repository.DesignRepository
	saleRepo     *repository.SaleRepository
	partRepo     *repository.PartRepository
	supplyRepo   *repository.ProjectSupplyRepository
	repos        *repository.Repositories // For transaction support
	printerMgr   *printer.Manager
	hub          *realtime.Hub
	storage      storage.Storage
}

// Create creates a new project.
func (s *ProjectService) Create(ctx context.Context, p *model.Project) error {
	if p.Name == "" {
		return fmt.Errorf("project name is required")
	}
	return s.repo.Create(ctx, p)
}

// GetByID retrieves a project by ID.
func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all projects.
func (s *ProjectService) List(ctx context.Context) ([]model.Project, error) {
	return s.repo.List(ctx)
}

// Update updates a project.
func (s *ProjectService) Update(ctx context.Context, p *model.Project) error {
	return s.repo.Update(ctx, p)
}

// Delete removes a project and clears nullable FK references that would block deletion.
func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repos.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Clear nullable FK references that would block deletion (RESTRICT)
		for _, stmt := range []string{
			`UPDATE print_jobs SET project_id = NULL WHERE project_id = ?`,
			`UPDATE sales SET project_id = NULL WHERE project_id = ?`,
			`UPDATE etsy_receipts SET project_id = NULL WHERE project_id = ?`,
			`UPDATE order_items SET project_id = NULL WHERE project_id = ?`,
		} {
			if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
				return err
			}
		}
		// tasks, parts, and project_supplies cascade via FK constraints
		_, err := tx.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
		return err
	})
}

// GetJobStats retrieves job statistics for a project.
func (s *ProjectService) GetJobStats(ctx context.Context, projectID uuid.UUID) (*repository.JobStats, error) {
	return s.printJobRepo.GetProjectJobStats(ctx, projectID)
}

// ListJobs retrieves all print jobs for a project.
func (s *ProjectService) ListJobs(ctx context.Context, projectID uuid.UUID) ([]model.PrintJob, error) {
	return s.printJobRepo.ListByProject(ctx, projectID)
}

// GetProjectSummary computes a derived analytics summary for a project.
// All values are computed from jobs and sales — nothing is stored.
func (s *ProjectService) GetProjectSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectSummary, error) {
	// Fetch jobs
	jobs, err := s.printJobRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Fetch sales
	sales, err := s.saleRepo.List(ctx, &projectID)
	if err != nil {
		return nil, err
	}

	summary := &model.ProjectSummary{}

	// Revenue from sales
	for _, sale := range sales {
		summary.TotalRevenueCents += sale.GrossCents
		summary.TotalFeesCents += sale.FeesCents
		summary.NetRevenueCents += sale.NetCents
		summary.SalesCount++
	}

	// Cost and performance from jobs
	summary.JobCount = len(jobs)
	for _, job := range jobs {
		if job.Status == model.PrintJobStatusCompleted {
			summary.CompletedCount++
		}
		if job.Status == model.PrintJobStatusFailed {
			summary.FailedCount++
		}

		// Cost breakdown (from completed jobs with snapshots)
		if job.PrinterTimeCostCents != nil {
			summary.PrinterTimeCostCents += *job.PrinterTimeCostCents
		}
		if job.MaterialCostCents != nil {
			summary.MaterialCostCents += *job.MaterialCostCents
		}
		if job.CostCents != nil {
			summary.TotalCostCents += *job.CostCents
		}

		// Print time
		if job.ActualSeconds != nil && *job.ActualSeconds > 0 {
			summary.TotalPrintSeconds += *job.ActualSeconds
		}

		// Material
		if job.MaterialUsedGrams != nil {
			summary.TotalMaterialGrams += *job.MaterialUsedGrams
		}
	}

	// Derived metrics
	if summary.CompletedCount+summary.FailedCount > 0 {
		summary.SuccessRate = float64(summary.CompletedCount) / float64(summary.CompletedCount+summary.FailedCount) * 100
	}
	if summary.CompletedCount > 0 {
		summary.AvgPrintSeconds = summary.TotalPrintSeconds / summary.CompletedCount
	}

	// Estimated material cost, grams, and print time from slice profiles
	if s.partRepo != nil && s.designRepo != nil {
		parts, err := s.partRepo.ListByProject(ctx, projectID)
		if err == nil {
			for _, part := range parts {
				designs, err := s.designRepo.ListByPart(ctx, part.ID)
				if err != nil || len(designs) == 0 {
					continue
				}
				// Latest design is first (ordered by version DESC)
				latest := designs[0]
				if latest.SliceProfile != nil {
					var profile model.SliceProfileData
					if json.Unmarshal(latest.SliceProfile, &profile) == nil {
						if profile.WeightGrams > 0 {
							// Default cost: $19.99/kg
							costPerKg := 19.99
							costCents := int(profile.WeightGrams / 1000.0 * costPerKg * 100)
							summary.EstimatedMaterialCostCents += costCents * part.Quantity
							summary.EstimatedMaterialGrams += profile.WeightGrams * float64(part.Quantity)
						}
						if profile.PrintTimeSeconds > 0 {
							summary.EstimatedPrintSeconds += profile.PrintTimeSeconds * part.Quantity
						}
					}
				}
			}
		}
	}

	// Supply costs
	if s.supplyRepo != nil {
		supplies, err := s.supplyRepo.ListByProject(ctx, projectID)
		if err == nil {
			for _, supply := range supplies {
				summary.SupplyCostCents += supply.UnitCostCents * supply.Quantity
			}
		}
	}

	// Include estimated material and supply costs in total
	summary.TotalCostCents += summary.EstimatedMaterialCostCents + summary.SupplyCostCents

	// UnitCostCents is the per-unit cost of production
	summary.UnitCostCents = summary.TotalCostCents

	// TotalCostCents is total COGS: per-unit cost × number of sales
	if summary.SalesCount > 1 {
		summary.TotalCostCents = summary.UnitCostCents * summary.SalesCount
	}

	summary.GrossProfitCents = summary.NetRevenueCents - summary.TotalCostCents
	if summary.NetRevenueCents > 0 {
		summary.GrossMarginPercent = float64(summary.GrossProfitCents) / float64(summary.NetRevenueCents) * 100
	}

	// Profit per hour: (profit from one sale) / (print time for one unit in hours)
	printSeconds := summary.TotalPrintSeconds
	if printSeconds <= 0 {
		printSeconds = summary.EstimatedPrintSeconds
	}
	if printSeconds > 0 && summary.SalesCount > 0 {
		profitPerSale := float64(summary.GrossProfitCents) / float64(summary.SalesCount)
		hours := float64(printSeconds) / 3600.0
		summary.ProfitPerHourCents = int(profitPerSale / hours)
	}

	return summary, nil
}

// StartProductionResult contains the result of starting production.
type StartProductionResult struct {
	JobsStarted int               `json:"jobs_started"`
	JobsSkipped int               `json:"jobs_skipped"`
	FailedJobs  []StartJobFailure `json:"failed_jobs,omitempty"`
}

// StartJobFailure represents a job that failed to start.
type StartJobFailure struct {
	JobID  uuid.UUID `json:"job_id"`
	Reason string    `json:"reason"`
}

// StartProduction auto-assigns resources and starts all queued jobs for a project.
func (s *ProjectService) StartProduction(ctx context.Context, projectID uuid.UUID) (*StartProductionResult, error) {
	project, err := s.repo.GetByID(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	if project == nil {
		return nil, fmt.Errorf("project not found")
	}

	// Get all jobs for this project
	jobs, err := s.printJobRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project jobs: %w", err)
	}

	// Get template constraints if this project has a template
	var template *model.Template
	if project.TemplateID != nil {
		template, err = s.templateRepo.GetByID(ctx, *project.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("failed to get template: %w", err)
		}
	}

	// Get available printers and spools
	printers, err := s.printerRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get printers: %w", err)
	}

	spools, err := s.spoolRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get spools: %w", err)
	}

	result := &StartProductionResult{}

	for _, job := range jobs {
		// Skip jobs that are not in queued status
		if job.Status != model.PrintJobStatusQueued {
			result.JobsSkipped++
			continue
		}

		// Find an idle printer
		var selectedPrinter *model.Printer
		for i := range printers {
			p := &printers[i]
			if p.Status != model.PrinterStatusIdle {
				continue
			}
			// Check template constraints
			if template != nil {
				if template.PreferredPrinterID != nil && !template.AllowAnyPrinter {
					if p.ID != *template.PreferredPrinterID {
						continue
					}
				}
				// TODO: Check printer constraints (HasEnclosure/HasAMS) once printer model supports them
			}
			selectedPrinter = p
			break
		}

		if selectedPrinter == nil {
			result.FailedJobs = append(result.FailedJobs, StartJobFailure{
				JobID:  job.ID,
				Reason: "no idle printer available",
			})
			continue
		}

		// Find a spool with matching material and enough weight
		var selectedSpool *model.MaterialSpool
		for i := range spools {
			sp := &spools[i]
			if sp.Status != model.SpoolStatusInUse && sp.Status != model.SpoolStatusNew && sp.Status != model.SpoolStatusLow {
				continue
			}
			// Check if spool has enough material (at least 50g as minimum)
			if sp.RemainingWeight < 50 {
				continue
			}
			// TODO: Check material type constraints from template if specified
			selectedSpool = sp
			break
		}

		if selectedSpool == nil {
			result.FailedJobs = append(result.FailedJobs, StartJobFailure{
				JobID:  job.ID,
				Reason: "no suitable spool available",
			})
			continue
		}

		// Assign resources to the job
		job.PrinterID = &selectedPrinter.ID
		job.MaterialSpoolID = &selectedSpool.ID

		if err := s.printJobRepo.Update(ctx, &job); err != nil {
			result.FailedJobs = append(result.FailedJobs, StartJobFailure{
				JobID:  job.ID,
				Reason: fmt.Sprintf("failed to assign resources: %v", err),
			})
			continue
		}

		// Record assignment event
		assignedStatus := model.PrintJobStatusAssigned
		event := model.NewJobEvent(job.ID, model.JobEventAssigned, &assignedStatus).
			WithPrinter(selectedPrinter.ID).
			WithActor(model.ActorSystem, "start_production")
		if err := s.printJobRepo.AppendEvent(ctx, event); err != nil {
			slog.Error("failed to record assignment event", "job_id", job.ID, "error", err)
		}

		// Get design and start the job
		design, err := s.designRepo.GetByID(ctx, job.DesignID)
		if err != nil || design == nil {
			result.FailedJobs = append(result.FailedJobs, StartJobFailure{
				JobID:  job.ID,
				Reason: "design not found",
			})
			continue
		}

		// Send to printer
		if err := s.printerMgr.StartJob(selectedPrinter.ID, design.FileName, s.storage.GetFullPath(design.FileName)); err != nil {
			// Record failure event
			failedStatus := model.PrintJobStatusFailed
			failEvent := model.NewJobEvent(job.ID, model.JobEventFailed, &failedStatus).
				WithError("UPLOAD_FAILED", err.Error()).
				WithActor(model.ActorSystem, "start_production")
			s.printJobRepo.AppendEvent(ctx, failEvent)

			result.FailedJobs = append(result.FailedJobs, StartJobFailure{
				JobID:  job.ID,
				Reason: fmt.Sprintf("failed to start print: %v", err),
			})
			continue
		}

		// Record uploaded event
		uploadedStatus := model.PrintJobStatusUploaded
		uploadEvent := model.NewJobEvent(job.ID, model.JobEventUploaded, &uploadedStatus).
			WithPrinter(selectedPrinter.ID).
			WithActor(model.ActorSystem, "start_production")
		s.printJobRepo.AppendEvent(ctx, uploadEvent)

		// Update started_at timestamp
		now := time.Now()
		job.StartedAt = &now
		s.printJobRepo.Update(ctx, &job) //nolint:errcheck // best-effort timestamp update

		// Mark printer as printing
		selectedPrinter.Status = model.PrinterStatusPrinting
		result.JobsStarted++
	}

	s.hub.Broadcast(realtime.Event{
		Type: "production_started",
		Data: map[string]interface{}{
			"project_id":   projectID,
			"jobs_started": result.JobsStarted,
		},
	})

	return result, nil
}

// PartService handles part business logic.
