package service

import (
	"context"
	"fmt"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type TemplateService struct {
	repo           *repository.TemplateRepository
	projectRepo    *repository.ProjectRepository
	partRepo       *repository.PartRepository
	designRepo     *repository.DesignRepository
	printJobRepo   *repository.PrintJobRepository
	spoolRepo      *repository.SpoolRepository
	materialRepo   *repository.MaterialRepository
	printerRepo    *repository.PrinterRepository
	projectService *ProjectService
}

// Create creates a new template.
func (s *TemplateService) Create(ctx context.Context, t *model.Template) error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if t.MaterialType == "" {
		return fmt.Errorf("material type is required")
	}
	t.IsActive = true
	return s.repo.Create(ctx, t)
}

// GetByID retrieves a template by ID with its designs, materials, and supplies.
func (s *TemplateService) GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}

	// Load designs
	designs, err := s.repo.GetDesigns(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Designs = designs

	// Load materials
	materials, err := s.repo.GetRecipeMaterials(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Materials = materials

	// Load supplies
	supplies, err := s.repo.GetRecipeSupplies(ctx, id)
	if err != nil {
		return nil, err
	}
	t.Supplies = supplies

	return t, nil
}

// GetBySKU retrieves a template by SKU.
func (s *TemplateService) GetBySKU(ctx context.Context, sku string) (*model.Template, error) {
	return s.repo.GetBySKU(ctx, sku)
}

// List retrieves all templates.
func (s *TemplateService) List(ctx context.Context, activeOnly bool) ([]model.Template, error) {
	return s.repo.List(ctx, activeOnly)
}

// Update updates a template.
func (s *TemplateService) Update(ctx context.Context, t *model.Template) error {
	return s.repo.Update(ctx, t)
}

// Delete removes a template.
func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// AddDesign adds a design to a template.
func (s *TemplateService) AddDesign(ctx context.Context, td *model.TemplateDesign) error {
	if td.TemplateID == uuid.Nil {
		return fmt.Errorf("template ID is required")
	}
	if td.DesignID == uuid.Nil {
		return fmt.Errorf("design ID is required")
	}

	// Verify design exists
	design, err := s.designRepo.GetByID(ctx, td.DesignID)
	if err != nil {
		return err
	}
	if design == nil {
		return fmt.Errorf("design not found")
	}

	return s.repo.AddDesign(ctx, td)
}

// RemoveDesign removes a design from a template.
func (s *TemplateService) RemoveDesign(ctx context.Context, templateID, designID uuid.UUID) error {
	return s.repo.RemoveDesign(ctx, templateID, designID)
}

// GetDesigns retrieves all designs for a template.
func (s *TemplateService) GetDesigns(ctx context.Context, templateID uuid.UUID) ([]model.TemplateDesign, error) {
	return s.repo.GetDesigns(ctx, templateID)
}

// CreateFromTemplateOptions contains options for creating a project from a template.
type CreateFromTemplateOptions struct {
	OrderQuantity   int    // Multiplier for quantity_per_order
	ExternalOrderID string // For Etsy orders later
	CustomerNotes   string
	Source          string     // "manual", "etsy", "api"
	MaterialSpoolID *uuid.UUID // Optional spool override
}

// CreateProjectFromTemplate creates a new project from a template.
func (s *TemplateService) CreateProjectFromTemplate(ctx context.Context, templateID uuid.UUID, opts CreateFromTemplateOptions) (*model.Project, []model.PrintJob, error) {
	// Fetch template with designs
	template, err := s.GetByID(ctx, templateID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get template: %w", err)
	}
	if template == nil {
		return nil, nil, fmt.Errorf("template not found")
	}

	if !template.IsActive {
		return nil, nil, fmt.Errorf("template is not active")
	}

	// Set defaults
	if opts.OrderQuantity <= 0 {
		opts.OrderQuantity = 1
	}
	if opts.Source == "" {
		opts.Source = "manual"
	}

	// Create project
	project := &model.Project{
		Name:            template.Name,
		Description:     template.Description,
		Tags:            template.Tags,
		TemplateID:      &templateID,
		Source:          opts.Source,
		ExternalOrderID: opts.ExternalOrderID,
		CustomerNotes:   opts.CustomerNotes,
	}
	if err := s.projectRepo.Create(ctx, project); err != nil {
		return nil, nil, fmt.Errorf("failed to create project: %w", err)
	}

	var printJobs []model.PrintJob

	// For each template design, create parts and print jobs
	totalParts := template.QuantityPerOrder * opts.OrderQuantity
	for _, td := range template.Designs {
		for i := 0; i < totalParts; i++ {
			// Create part for this design
			partName := td.Design.FileName
			if td.Notes != "" {
				partName = td.Notes
			}

			part := &model.Part{
				ProjectID:   project.ID,
				Name:        partName,
				Description: fmt.Sprintf("From template: %s", template.Name),
				Quantity:    td.Quantity,
				Status:      model.PartStatusDesign,
			}
			if err := s.partRepo.Create(ctx, part); err != nil {
				return nil, nil, fmt.Errorf("failed to create part: %w", err)
			}

			// Create print job for each part quantity
			for j := 0; j < td.Quantity; j++ {
				job := &model.PrintJob{
					DesignID:  td.DesignID,
					ProjectID: &project.ID,
					RecipeID:  &templateID,
					Status:    model.PrintJobStatusQueued,
					Notes:     fmt.Sprintf("Part %d/%d for order", i+1, totalParts),
				}

				// Assign preferred printer if specified and not allowing any printer
				if template.PreferredPrinterID != nil && !template.AllowAnyPrinter {
					job.PrinterID = template.PreferredPrinterID
				}

				// Assign material spool if specified in options
				if opts.MaterialSpoolID != nil {
					job.MaterialSpoolID = opts.MaterialSpoolID
				}

				if err := s.printJobRepo.Create(ctx, job); err != nil {
					return nil, nil, fmt.Errorf("failed to create print job: %w", err)
				}
				printJobs = append(printJobs, *job)
			}
		}
	}

	return project, printJobs, nil
}

// GetByIDWithMaterials retrieves a template with its materials and supplies loaded.
func (s *TemplateService) GetByIDWithMaterials(ctx context.Context, id uuid.UUID) (*model.Template, error) {
	t, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}

	// Load materials
	materials, err := s.repo.GetRecipeMaterials(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading materials: %w", err)
	}
	t.Materials = materials

	// Load supplies
	supplies, err := s.repo.GetRecipeSupplies(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("loading supplies: %w", err)
	}
	t.Supplies = supplies

	return t, nil
}

// AddMaterial adds a material requirement to a recipe.
func (s *TemplateService) AddMaterial(ctx context.Context, m *model.RecipeMaterial) error {
	if m.RecipeID == uuid.Nil {
		return fmt.Errorf("recipe ID is required")
	}
	if m.MaterialType == "" {
		return fmt.Errorf("material type is required")
	}
	return s.repo.CreateRecipeMaterial(ctx, m)
}

// UpdateMaterial updates a material requirement.
func (s *TemplateService) UpdateMaterial(ctx context.Context, m *model.RecipeMaterial) error {
	return s.repo.UpdateRecipeMaterial(ctx, m)
}

// RemoveMaterial removes a material requirement from a recipe.
func (s *TemplateService) RemoveMaterial(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteRecipeMaterial(ctx, id)
}

// GetMaterial retrieves a single material by ID.
func (s *TemplateService) GetMaterial(ctx context.Context, id uuid.UUID) (*model.RecipeMaterial, error) {
	return s.repo.GetRecipeMaterialByID(ctx, id)
}

// ListMaterials retrieves all materials for a recipe.
func (s *TemplateService) ListMaterials(ctx context.Context, recipeID uuid.UUID) ([]model.RecipeMaterial, error) {
	return s.repo.GetRecipeMaterials(ctx, recipeID)
}

// AddSupply adds a supply item to a recipe.
func (s *TemplateService) AddSupply(ctx context.Context, supply *model.RecipeSupply) error {
	if supply.RecipeID == uuid.Nil {
		return fmt.Errorf("recipe ID is required")
	}
	// If material_id is set and name is empty, auto-populate from the material
	if supply.MaterialID != nil && *supply.MaterialID != uuid.Nil && supply.Name == "" {
		mat, err := s.materialRepo.GetByID(ctx, *supply.MaterialID)
		if err != nil {
			return fmt.Errorf("failed to look up material: %w", err)
		}
		if mat != nil {
			supply.Name = mat.Name
			if supply.UnitCostCents == 0 {
				supply.UnitCostCents = int(mat.CostPerKg * 100) // CostPerKg repurposed as per-unit $ for supplies
			}
		}
	}
	if supply.Name == "" {
		return fmt.Errorf("supply name is required")
	}
	if supply.Quantity < 1 {
		supply.Quantity = 1
	}
	return s.repo.AddRecipeSupply(ctx, supply)
}

// UpdateSupply updates a supply item.
func (s *TemplateService) UpdateSupply(ctx context.Context, supply *model.RecipeSupply) error {
	return s.repo.UpdateRecipeSupply(ctx, supply)
}

// RemoveSupply removes a supply item from a recipe.
func (s *TemplateService) RemoveSupply(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteRecipeSupply(ctx, id)
}

// GetSupply retrieves a single supply by ID.
func (s *TemplateService) GetSupply(ctx context.Context, id uuid.UUID) (*model.RecipeSupply, error) {
	return s.repo.GetRecipeSupplyByID(ctx, id)
}

// ListSupplies retrieves all supplies for a recipe.
func (s *TemplateService) ListSupplies(ctx context.Context, recipeID uuid.UUID) ([]model.RecipeSupply, error) {
	return s.repo.GetRecipeSupplies(ctx, recipeID)
}

// GetTemplateAnalytics returns aggregated performance metrics from all projects created from a template.
func (s *TemplateService) GetTemplateAnalytics(ctx context.Context, templateID uuid.UUID) (*model.TemplateAnalytics, error) {
	// Verify template exists
	template, err := s.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("template not found")
	}

	// Get all projects linked to this template
	projects, err := s.projectRepo.ListByTemplateID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}

	analytics := &model.TemplateAnalytics{
		TemplateID:             templateID,
		ProjectCount:           len(projects),
		EstimatedPrintSeconds:  template.EstimatedPrintSeconds,
		EstimatedMaterialGrams: template.EstimatedMaterialGrams,
	}

	if len(projects) == 0 {
		return analytics, nil
	}

	// Aggregate summaries from each project
	var marginSum float64
	var marginCount int
	for _, proj := range projects {
		if s.projectService == nil {
			continue
		}
		summary, err := s.projectService.GetProjectSummary(ctx, proj.ID)
		if err != nil || summary == nil {
			continue
		}

		analytics.TotalRevenueCents += summary.TotalRevenueCents
		analytics.TotalFeesCents += summary.TotalFeesCents
		analytics.NetRevenueCents += summary.NetRevenueCents
		analytics.TotalSalesCount += summary.SalesCount

		analytics.TotalCostCents += summary.TotalCostCents
		analytics.TotalPrinterTimeCost += summary.PrinterTimeCostCents
		analytics.TotalMaterialCost += summary.MaterialCostCents
		analytics.TotalSupplyCost += summary.SupplyCostCents

		analytics.TotalJobCount += summary.JobCount
		analytics.TotalCompleted += summary.CompletedCount
		analytics.TotalFailed += summary.FailedCount

		analytics.TotalPrintSeconds += summary.TotalPrintSeconds
		analytics.TotalMaterialGrams += summary.TotalMaterialGrams

		if summary.GrossMarginPercent != 0 {
			marginSum += summary.GrossMarginPercent
			marginCount++
		}
	}

	// Derived metrics
	if analytics.TotalCompleted+analytics.TotalFailed > 0 {
		analytics.SuccessRate = float64(analytics.TotalCompleted) / float64(analytics.TotalCompleted+analytics.TotalFailed) * 100
	}
	if analytics.TotalCompleted > 0 {
		analytics.AvgPrintSeconds = analytics.TotalPrintSeconds / analytics.TotalCompleted
	}
	if analytics.ProjectCount > 0 {
		analytics.AvgUnitCostCents = analytics.TotalCostCents / analytics.ProjectCount
		analytics.AvgMaterialGrams = analytics.TotalMaterialGrams / float64(analytics.ProjectCount)
	}
	if marginCount > 0 {
		analytics.AvgGrossMarginPercent = marginSum / float64(marginCount)
	}

	analytics.TotalGrossProfitCents = analytics.NetRevenueCents - analytics.TotalCostCents

	// Profit per hour across all projects
	if analytics.TotalPrintSeconds > 0 {
		hours := float64(analytics.TotalPrintSeconds) / 3600.0
		analytics.ProfitPerHourCents = int(float64(analytics.TotalGrossProfitCents) / hours)
	}

	// Get estimated cost for comparison
	costEstimate, err := s.CalculateRecipeCost(ctx, templateID)
	if err == nil && costEstimate != nil {
		analytics.EstimatedCostCents = costEstimate.TotalCostCents
	}

	return analytics, nil
}

// PrinterValidationResult contains the result of printer validation.
type PrinterValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidatePrinterForRecipe checks if a printer meets recipe constraints.
func (s *TemplateService) ValidatePrinterForRecipe(ctx context.Context, recipeID, printerID uuid.UUID) (*PrinterValidationResult, error) {
	template, err := s.repo.GetByID(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("recipe not found")
	}

	printer, err := s.printerRepo.GetByID(ctx, printerID)
	if err != nil {
		return nil, err
	}
	if printer == nil {
		return nil, fmt.Errorf("printer not found")
	}

	result := &PrinterValidationResult{Valid: true}

	if template.PrinterConstraints == nil {
		return result, nil
	}

	constraints := template.PrinterConstraints

	// Check bed size
	if constraints.MinBedSize != nil {
		if printer.BuildVolume == nil {
			result.Warnings = append(result.Warnings, "Printer build volume not configured")
		} else {
			if printer.BuildVolume.X < constraints.MinBedSize.X {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("X axis too small: need %.0fmm, have %.0fmm", constraints.MinBedSize.X, printer.BuildVolume.X))
			}
			if printer.BuildVolume.Y < constraints.MinBedSize.Y {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("Y axis too small: need %.0fmm, have %.0fmm", constraints.MinBedSize.Y, printer.BuildVolume.Y))
			}
			if printer.BuildVolume.Z < constraints.MinBedSize.Z {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("Z axis too small: need %.0fmm, have %.0fmm", constraints.MinBedSize.Z, printer.BuildVolume.Z))
			}
		}
	}

	// Check nozzle diameter
	if len(constraints.NozzleDiameters) > 0 {
		found := false
		for _, d := range constraints.NozzleDiameters {
			if printer.NozzleDiameter == d {
				found = true
				break
			}
		}
		if !found {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Incompatible nozzle: need one of %v mm, have %.2f mm", constraints.NozzleDiameters, printer.NozzleDiameter))
		}
	}

	// Check enclosure requirement
	if constraints.RequiresEnclosure {
		result.Warnings = append(result.Warnings, "Recipe requires enclosure - verify printer has enclosure")
	}

	// Check AMS requirement
	if constraints.RequiresAMS {
		result.Warnings = append(result.Warnings, "Recipe requires AMS - verify printer has AMS configured")
	}

	return result, nil
}

// FindCompatiblePrinters returns printers that match recipe constraints.
func (s *TemplateService) FindCompatiblePrinters(ctx context.Context, recipeID uuid.UUID) ([]model.Printer, error) {
	return s.repo.FindCompatiblePrinters(ctx, recipeID)
}

// CompatibleSpool represents a spool that matches recipe requirements.
type CompatibleSpool struct {
	Spool       model.MaterialSpool `json:"spool"`
	Material    model.Material      `json:"material"`
	MatchReason string              `json:"match_reason"`
}

// FindCompatibleSpools finds spools matching recipe material requirements.
func (s *TemplateService) FindCompatibleSpools(ctx context.Context, recipeID uuid.UUID) ([]CompatibleSpool, error) {
	materials, err := s.repo.GetRecipeMaterials(ctx, recipeID)
	if err != nil {
		return nil, err
	}

	// Get all available spools
	spools, err := s.spoolRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	// Get all materials for lookup
	allMaterials, err := s.materialRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	materialMap := make(map[uuid.UUID]model.Material)
	for _, m := range allMaterials {
		materialMap[m.ID] = m
	}

	var compatible []CompatibleSpool

	for _, rm := range materials {
		for _, spool := range spools {
			// Skip empty or archived spools
			if spool.Status == model.SpoolStatusEmpty || spool.Status == model.SpoolStatusArchived {
				continue
			}

			// Check if spool has enough material
			if spool.RemainingWeight < rm.WeightGrams {
				continue
			}

			material, ok := materialMap[spool.MaterialID]
			if !ok {
				continue
			}

			// Check material type match
			if material.Type != rm.MaterialType {
				continue
			}

			// Check color match based on color spec mode
			matchReason := fmt.Sprintf("Type match: %s", rm.MaterialType)

			if rm.ColorSpec != nil {
				switch rm.ColorSpec.Mode {
				case "exact":
					if rm.ColorSpec.Hex != "" && material.ColorHex != rm.ColorSpec.Hex {
						continue
					}
					if rm.ColorSpec.Name != "" && material.Color != rm.ColorSpec.Name {
						continue
					}
					matchReason += fmt.Sprintf(", Color: %s", material.Color)
				case "category":
					// Allow any color in the same category (would need color categorization)
					matchReason += fmt.Sprintf(", Color (any): %s", material.Color)
				case "any":
					matchReason += " (any color)"
				}
			}

			compatible = append(compatible, CompatibleSpool{
				Spool:       spool,
				Material:    material,
				MatchReason: matchReason,
			})
		}
	}

	return compatible, nil
}

// DefaultHourlyRateCents is the default machine time cost per hour in cents.
const DefaultHourlyRateCents = 500 // $5.00/hour

// DefaultLaborRateCents is the default manual labor cost per hour in cents.
const DefaultLaborRateCents = 1500 // $15.00/hour

// CalculateRecipeCost calculates the cost breakdown for a recipe.
func (s *TemplateService) CalculateRecipeCost(ctx context.Context, recipeID uuid.UUID) (*model.RecipeCostEstimate, error) {
	template, err := s.GetByIDWithMaterials(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, fmt.Errorf("recipe not found")
	}

	// Get all materials for cost lookup
	allMaterials, err := s.materialRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	materialMap := make(map[model.MaterialType]model.Material)
	for _, m := range allMaterials {
		materialMap[m.Type] = m
	}

	// Determine hourly rate: use preferred printer's rate if available
	hourlyRateCents := DefaultHourlyRateCents
	printerName := ""
	if template.PreferredPrinterID != nil {
		p, _ := s.printerRepo.GetByID(ctx, *template.PreferredPrinterID)
		if p != nil && p.CostPerHourCents > 0 {
			hourlyRateCents = p.CostPerHourCents
			printerName = p.Name
		}
	}

	estimate := &model.RecipeCostEstimate{
		EstimatedPrintTime: template.EstimatedPrintSeconds,
		HourlyRateCents:    hourlyRateCents,
		LaborRateCents:     DefaultLaborRateCents,
		LaborMinutes:       template.LaborMinutes,
		SalePriceCents:     template.SalePriceCents,
		PrinterName:        printerName,
	}

	// Calculate material costs
	for _, rm := range template.Materials {
		material, ok := materialMap[rm.MaterialType]
		if !ok {
			// Use a default cost if material not found
			material = model.Material{CostPerKg: 25.0} // $25/kg default
		}

		// Cost = (weight_grams / 1000) * cost_per_kg * 100 (to cents)
		costCents := int((rm.WeightGrams / 1000.0) * material.CostPerKg * 100)
		estimate.MaterialCostCents += costCents

		colorName := ""
		if rm.ColorSpec != nil {
			colorName = rm.ColorSpec.Name
		}

		estimate.MaterialBreakdown = append(estimate.MaterialBreakdown, model.RecipeMaterialCostBreakdown{
			MaterialType: string(rm.MaterialType),
			WeightGrams:  rm.WeightGrams,
			CostCents:    costCents,
			ColorName:    colorName,
		})
	}

	// If no materials defined but we have estimated grams, use that
	if len(template.Materials) == 0 && template.EstimatedMaterialGrams > 0 {
		material, ok := materialMap[template.MaterialType]
		if !ok {
			material = model.Material{CostPerKg: 25.0}
		}
		costCents := int((template.EstimatedMaterialGrams / 1000.0) * material.CostPerKg * 100)
		estimate.MaterialCostCents = costCents
		estimate.MaterialBreakdown = append(estimate.MaterialBreakdown, model.RecipeMaterialCostBreakdown{
			MaterialType: string(template.MaterialType),
			WeightGrams:  template.EstimatedMaterialGrams,
			CostCents:    costCents,
		})
	}

	// Calculate time cost (machine time) using actual printer rate
	if template.EstimatedPrintSeconds > 0 {
		hours := float64(template.EstimatedPrintSeconds) / 3600.0
		estimate.TimeCostCents = int(hours * float64(hourlyRateCents))
	}

	// Calculate labor cost (manual labor time)
	if template.LaborMinutes > 0 {
		hours := float64(template.LaborMinutes) / 60.0
		estimate.LaborCostCents = int(hours * float64(DefaultLaborRateCents))
	}

	// Calculate supply costs
	for _, supply := range template.Supplies {
		totalCents := supply.UnitCostCents * supply.Quantity
		estimate.SupplyCostCents += totalCents
		estimate.SupplyBreakdown = append(estimate.SupplyBreakdown, model.RecipeSupplyCostBreakdown{
			Name:          supply.Name,
			UnitCostCents: supply.UnitCostCents,
			Quantity:      supply.Quantity,
			TotalCents:    totalCents,
		})
	}

	// Total cost = material + machine time + labor + supplies
	estimate.TotalCostCents = estimate.MaterialCostCents + estimate.TimeCostCents + estimate.LaborCostCents + estimate.SupplyCostCents

	// Calculate gross margin if sale price is set
	if template.SalePriceCents > 0 {
		estimate.GrossMarginCents = template.SalePriceCents - estimate.TotalCostCents
		estimate.GrossMarginPercent = float64(estimate.GrossMarginCents) / float64(template.SalePriceCents) * 100.0

		// Profit per hour: margin / estimated print hours
		if template.EstimatedPrintSeconds > 0 {
			hours := float64(template.EstimatedPrintSeconds) / 3600.0
			estimate.ProfitPerHourCents = int(float64(estimate.GrossMarginCents) / hours)
		}
	}

	return estimate, nil
}

// SettingsService handles application settings.
