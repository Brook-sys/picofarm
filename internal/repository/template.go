package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type TemplateRepository struct {
	db *sql.DB
}

// Create inserts a new template.
func (r *TemplateRepository) Create(ctx context.Context, t *model.Template) error {
	t.ID = uuid.New()
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	if t.Tags == nil {
		t.Tags = []string{}
	}
	if t.PostProcessChecklist == nil {
		t.PostProcessChecklist = []string{}
	}
	if t.QuantityPerOrder == 0 {
		t.QuantityPerOrder = 1
	}
	if t.Version == 0 {
		t.Version = 1
	}
	if t.PrintProfile == "" {
		t.PrintProfile = model.PrintProfileStandard
	}

	tagsJSON := marshalStringArray(t.Tags)
	checklistJSON, _ := json.Marshal(t.PostProcessChecklist)
	constraintsJSON, _ := json.Marshal(t.PrinterConstraints)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO templates (id, name, description, sku, tags, material_type, estimated_material_grams, preferred_printer_id, allow_any_printer, quantity_per_order, post_process_checklist, is_active, printer_constraints, print_profile, estimated_print_seconds, labor_minutes, sale_price_cents, material_cost_per_gram_cents, version, archived_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Name, t.Description, t.SKU, tagsJSON, t.MaterialType, t.EstimatedMaterialGrams, t.PreferredPrinterID, t.AllowAnyPrinter, t.QuantityPerOrder, checklistJSON, t.IsActive, constraintsJSON, t.PrintProfile, t.EstimatedPrintSeconds, t.LaborMinutes, t.SalePriceCents, t.MaterialCostPerGramCents, t.Version, t.ArchivedAt, t.CreatedAt, t.UpdatedAt)
	return err
}

// GetByID retrieves a template by ID.
func (r *TemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Template, error) {
	var t model.Template
	var tagsJSON, checklistJSON, constraintsJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, description, sku, tags, material_type, estimated_material_grams, preferred_printer_id, allow_any_printer, quantity_per_order, post_process_checklist, is_active, COALESCE(printer_constraints, '{}'), COALESCE(print_profile, 'standard'), COALESCE(estimated_print_seconds, 0), COALESCE(labor_minutes, 0), COALESCE(sale_price_cents, 0), COALESCE(material_cost_per_gram_cents, 0), COALESCE(version, 1), archived_at, created_at, updated_at
		FROM templates WHERE id = ?
	`, id), &t.ID, &t.Name, &t.Description, &t.SKU, &tagsJSON, &t.MaterialType, &t.EstimatedMaterialGrams, &t.PreferredPrinterID, &t.AllowAnyPrinter, &t.QuantityPerOrder, &checklistJSON, &t.IsActive, &constraintsJSON, &t.PrintProfile, &t.EstimatedPrintSeconds, &t.LaborMinutes, &t.SalePriceCents, &t.MaterialCostPerGramCents, &t.Version, &t.ArchivedAt, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Tags = unmarshalStringArray(tagsJSON)
	if checklistJSON != nil {
		json.Unmarshal(checklistJSON, &t.PostProcessChecklist)
	}
	if len(constraintsJSON) > 2 {
		var constraints model.PrinterConstraints
		if err := json.Unmarshal(constraintsJSON, &constraints); err == nil {
			t.PrinterConstraints = &constraints
		}
	}
	return &t, nil
}

// GetBySKU retrieves a template by SKU.
func (r *TemplateRepository) GetBySKU(ctx context.Context, sku string) (*model.Template, error) {
	var t model.Template
	var tagsJSON, checklistJSON, constraintsJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, description, sku, tags, material_type, estimated_material_grams, preferred_printer_id, allow_any_printer, quantity_per_order, post_process_checklist, is_active, COALESCE(printer_constraints, '{}'), COALESCE(print_profile, 'standard'), COALESCE(estimated_print_seconds, 0), COALESCE(labor_minutes, 0), COALESCE(sale_price_cents, 0), COALESCE(material_cost_per_gram_cents, 0), COALESCE(version, 1), archived_at, created_at, updated_at
		FROM templates WHERE sku = ?
	`, sku), &t.ID, &t.Name, &t.Description, &t.SKU, &tagsJSON, &t.MaterialType, &t.EstimatedMaterialGrams, &t.PreferredPrinterID, &t.AllowAnyPrinter, &t.QuantityPerOrder, &checklistJSON, &t.IsActive, &constraintsJSON, &t.PrintProfile, &t.EstimatedPrintSeconds, &t.LaborMinutes, &t.SalePriceCents, &t.MaterialCostPerGramCents, &t.Version, &t.ArchivedAt, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Tags = unmarshalStringArray(tagsJSON)
	if checklistJSON != nil {
		json.Unmarshal(checklistJSON, &t.PostProcessChecklist)
	}
	if len(constraintsJSON) > 2 {
		var constraints model.PrinterConstraints
		if err := json.Unmarshal(constraintsJSON, &constraints); err == nil {
			t.PrinterConstraints = &constraints
		}
	}
	return &t, nil
}

// List retrieves all templates with optional active filter.
func (r *TemplateRepository) List(ctx context.Context, activeOnly bool) ([]model.Template, error) {
	query := `SELECT id, name, description, sku, tags, material_type, estimated_material_grams, preferred_printer_id, allow_any_printer, quantity_per_order, post_process_checklist, is_active, COALESCE(printer_constraints, '{}'), COALESCE(print_profile, 'standard'), COALESCE(estimated_print_seconds, 0), COALESCE(labor_minutes, 0), COALESCE(sale_price_cents, 0), COALESCE(material_cost_per_gram_cents, 0), COALESCE(version, 1), archived_at, created_at, updated_at FROM templates`
	if activeOnly {
		query += ` WHERE is_active = 1 AND archived_at IS NULL`
	}
	query += ` ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []model.Template
	for rows.Next() {
		var t model.Template
		var tagsJSON, checklistJSON, constraintsJSON []byte
		if err := scanRow(rows, &t.ID, &t.Name, &t.Description, &t.SKU, &tagsJSON, &t.MaterialType, &t.EstimatedMaterialGrams, &t.PreferredPrinterID, &t.AllowAnyPrinter, &t.QuantityPerOrder, &checklistJSON, &t.IsActive, &constraintsJSON, &t.PrintProfile, &t.EstimatedPrintSeconds, &t.LaborMinutes, &t.SalePriceCents, &t.MaterialCostPerGramCents, &t.Version, &t.ArchivedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Tags = unmarshalStringArray(tagsJSON)
		if checklistJSON != nil {
			json.Unmarshal(checklistJSON, &t.PostProcessChecklist)
		}
		if len(constraintsJSON) > 2 {
			var constraints model.PrinterConstraints
			if err := json.Unmarshal(constraintsJSON, &constraints); err == nil {
				t.PrinterConstraints = &constraints
			}
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

// Update updates a template.
func (r *TemplateRepository) Update(ctx context.Context, t *model.Template) error {
	t.UpdatedAt = time.Now()
	tagsJSON := marshalStringArray(t.Tags)
	checklistJSON, _ := json.Marshal(t.PostProcessChecklist)
	constraintsJSON, _ := json.Marshal(t.PrinterConstraints)

	_, err := r.db.ExecContext(ctx, `
		UPDATE templates SET name = ?, description = ?, sku = ?, tags = ?, material_type = ?, estimated_material_grams = ?, preferred_printer_id = ?, allow_any_printer = ?, quantity_per_order = ?, post_process_checklist = ?, is_active = ?, printer_constraints = ?, print_profile = ?, estimated_print_seconds = ?, labor_minutes = ?, sale_price_cents = ?, material_cost_per_gram_cents = ?, version = ?, archived_at = ?, updated_at = ?
		WHERE id = ?
	`, t.Name, t.Description, t.SKU, tagsJSON, t.MaterialType, t.EstimatedMaterialGrams, t.PreferredPrinterID, t.AllowAnyPrinter, t.QuantityPerOrder, checklistJSON, t.IsActive, constraintsJSON, t.PrintProfile, t.EstimatedPrintSeconds, t.LaborMinutes, t.SalePriceCents, t.MaterialCostPerGramCents, t.Version, t.ArchivedAt, t.UpdatedAt, t.ID)
	return err
}

// Delete removes a template.
func (r *TemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	return err
}

// AddDesign adds a design to a template.
func (r *TemplateRepository) AddDesign(ctx context.Context, td *model.TemplateDesign) error {
	td.ID = uuid.New()
	td.CreatedAt = time.Now()
	if td.Quantity == 0 {
		td.Quantity = 1
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO template_designs (id, template_id, design_id, is_primary, quantity, sequence_order, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, td.ID, td.TemplateID, td.DesignID, td.IsPrimary, td.Quantity, td.SequenceOrder, td.Notes, td.CreatedAt)
	return err
}

// RemoveDesign removes a design from a template.
func (r *TemplateRepository) RemoveDesign(ctx context.Context, templateID, designID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM template_designs WHERE template_id = ? AND design_id = ?`, templateID, designID)
	return err
}

// GetDesigns retrieves all designs for a template.
func (r *TemplateRepository) GetDesigns(ctx context.Context, templateID uuid.UUID) ([]model.TemplateDesign, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT td.id, td.template_id, td.design_id, td.is_primary, td.quantity, td.sequence_order, td.notes, td.created_at,
		       d.id, d.part_id, d.version, d.file_id, d.file_name, d.file_hash, d.file_size_bytes, d.file_type, d.notes, d.slice_profile, d.created_at
		FROM template_designs td
		JOIN designs d ON d.id = td.design_id
		WHERE td.template_id = ?
		ORDER BY td.sequence_order ASC, td.created_at ASC
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var designs []model.TemplateDesign
	for rows.Next() {
		var td model.TemplateDesign
		var d model.Design
		if err := scanRow(rows,
			&td.ID, &td.TemplateID, &td.DesignID, &td.IsPrimary, &td.Quantity, &td.SequenceOrder, &td.Notes, &td.CreatedAt,
			&d.ID, &d.PartID, &d.Version, &d.FileID, &d.FileName, &d.FileHash, &d.FileSizeBytes, &d.FileType, &d.Notes, &d.SliceProfile, &d.CreatedAt,
		); err != nil {
			return nil, err
		}
		td.Design = &d
		designs = append(designs, td)
	}
	return designs, rows.Err()
}

// CreateRecipeMaterial inserts a new material requirement.
func (r *TemplateRepository) CreateRecipeMaterial(ctx context.Context, m *model.RecipeMaterial) error {
	m.ID = uuid.New()
	m.CreatedAt = time.Now()

	colorSpecJSON, _ := json.Marshal(m.ColorSpec)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO recipe_materials (id, recipe_id, material_type, color_spec, weight_grams, ams_position, sequence_order, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, m.ID, m.RecipeID, m.MaterialType, colorSpecJSON, m.WeightGrams, m.AMSPosition, m.SequenceOrder, m.Notes, m.CreatedAt)
	return err
}

// GetRecipeMaterials retrieves all materials for a recipe.
func (r *TemplateRepository) GetRecipeMaterials(ctx context.Context, recipeID uuid.UUID) ([]model.RecipeMaterial, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, recipe_id, material_type, color_spec, weight_grams, ams_position, sequence_order, notes, created_at
		FROM recipe_materials WHERE recipe_id = ? ORDER BY sequence_order ASC
	`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []model.RecipeMaterial
	for rows.Next() {
		var m model.RecipeMaterial
		var colorSpecJSON []byte
		if err := scanRow(rows, &m.ID, &m.RecipeID, &m.MaterialType, &colorSpecJSON, &m.WeightGrams, &m.AMSPosition, &m.SequenceOrder, &m.Notes, &m.CreatedAt); err != nil {
			return nil, err
		}
		if len(colorSpecJSON) > 2 {
			var colorSpec model.ColorSpec
			if err := json.Unmarshal(colorSpecJSON, &colorSpec); err == nil {
				m.ColorSpec = &colorSpec
			}
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}

// GetRecipeMaterialByID retrieves a single material by ID.
func (r *TemplateRepository) GetRecipeMaterialByID(ctx context.Context, id uuid.UUID) (*model.RecipeMaterial, error) {
	var m model.RecipeMaterial
	var colorSpecJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, recipe_id, material_type, color_spec, weight_grams, ams_position, sequence_order, notes, created_at
		FROM recipe_materials WHERE id = ?
	`, id), &m.ID, &m.RecipeID, &m.MaterialType, &colorSpecJSON, &m.WeightGrams, &m.AMSPosition, &m.SequenceOrder, &m.Notes, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(colorSpecJSON) > 2 {
		var colorSpec model.ColorSpec
		if err := json.Unmarshal(colorSpecJSON, &colorSpec); err == nil {
			m.ColorSpec = &colorSpec
		}
	}
	return &m, nil
}

// UpdateRecipeMaterial updates a material requirement.
func (r *TemplateRepository) UpdateRecipeMaterial(ctx context.Context, m *model.RecipeMaterial) error {
	colorSpecJSON, _ := json.Marshal(m.ColorSpec)

	_, err := r.db.ExecContext(ctx, `
		UPDATE recipe_materials SET material_type = ?, color_spec = ?, weight_grams = ?, ams_position = ?, sequence_order = ?, notes = ?
		WHERE id = ?
	`, m.MaterialType, colorSpecJSON, m.WeightGrams, m.AMSPosition, m.SequenceOrder, m.Notes, m.ID)
	return err
}

// DeleteRecipeMaterial removes a material requirement.
func (r *TemplateRepository) DeleteRecipeMaterial(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM recipe_materials WHERE id = ?`, id)
	return err
}

// AddRecipeSupply inserts a new supply item for a recipe.
func (r *TemplateRepository) AddRecipeSupply(ctx context.Context, s *model.RecipeSupply) error {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO recipe_supplies (id, recipe_id, name, unit_cost_cents, quantity, material_id, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.RecipeID, s.Name, s.UnitCostCents, s.Quantity, s.MaterialID, s.Notes, s.CreatedAt)
	return err
}

// GetRecipeSupplies retrieves all supplies for a recipe.
func (r *TemplateRepository) GetRecipeSupplies(ctx context.Context, recipeID uuid.UUID) ([]model.RecipeSupply, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, recipe_id, name, unit_cost_cents, quantity, material_id, notes, created_at
		FROM recipe_supplies WHERE recipe_id = ? ORDER BY created_at ASC
	`, recipeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var supplies []model.RecipeSupply
	for rows.Next() {
		var s model.RecipeSupply
		if err := scanRow(rows, &s.ID, &s.RecipeID, &s.Name, &s.UnitCostCents, &s.Quantity, &s.MaterialID, &s.Notes, &s.CreatedAt); err != nil {
			return nil, err
		}
		supplies = append(supplies, s)
	}
	return supplies, rows.Err()
}

// UpdateRecipeSupply updates a supply item.
func (r *TemplateRepository) UpdateRecipeSupply(ctx context.Context, s *model.RecipeSupply) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE recipe_supplies SET name = ?, unit_cost_cents = ?, quantity = ?, notes = ?
		WHERE id = ?
	`, s.Name, s.UnitCostCents, s.Quantity, s.Notes, s.ID)
	return err
}

// DeleteRecipeSupply removes a supply item.
func (r *TemplateRepository) DeleteRecipeSupply(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM recipe_supplies WHERE id = ?`, id)
	return err
}

// GetRecipeSupplyByID retrieves a single supply by ID.
func (r *TemplateRepository) GetRecipeSupplyByID(ctx context.Context, id uuid.UUID) (*model.RecipeSupply, error) {
	var s model.RecipeSupply
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, recipe_id, name, unit_cost_cents, quantity, material_id, notes, created_at
		FROM recipe_supplies WHERE id = ?
	`, id), &s.ID, &s.RecipeID, &s.Name, &s.UnitCostCents, &s.Quantity, &s.MaterialID, &s.Notes, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindCompatiblePrinters queries printers matching recipe constraints.
func (r *TemplateRepository) FindCompatiblePrinters(ctx context.Context, recipeID uuid.UUID) ([]model.Printer, error) {
	// Get the recipe to check its constraints
	template, err := r.GetByID(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, nil
	}

	// Start with all printers
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, model, manufacturer, connection_type, connection_uri, api_key, status, build_volume, nozzle_diameter, location, notes, created_at, updated_at
		FROM printers ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allPrinters []model.Printer
	for rows.Next() {
		var p model.Printer
		var buildVolumeJSON []byte
		if err := scanRow(rows, &p.ID, &p.Name, &p.Model, &p.Manufacturer, &p.ConnectionType, &p.ConnectionURI, &p.APIKey, &p.Status, &buildVolumeJSON, &p.NozzleDiameter, &p.Location, &p.Notes, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if buildVolumeJSON != nil {
			json.Unmarshal(buildVolumeJSON, &p.BuildVolume)
		}
		allPrinters = append(allPrinters, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If no constraints, return all printers
	if template.PrinterConstraints == nil {
		return allPrinters, nil
	}

	constraints := template.PrinterConstraints

	// Filter printers based on constraints
	var compatible []model.Printer
	for _, p := range allPrinters {
		// Check bed size
		if constraints.MinBedSize != nil && p.BuildVolume != nil {
			if p.BuildVolume.X < constraints.MinBedSize.X ||
				p.BuildVolume.Y < constraints.MinBedSize.Y ||
				p.BuildVolume.Z < constraints.MinBedSize.Z {
				continue
			}
		}

		// Check nozzle diameter
		if len(constraints.NozzleDiameters) > 0 {
			found := false
			for _, d := range constraints.NozzleDiameters {
				if p.NozzleDiameter == d {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		compatible = append(compatible, p)
	}

	return compatible, nil
}

// BambuCloudRepository handles Bambu Cloud auth token storage.
