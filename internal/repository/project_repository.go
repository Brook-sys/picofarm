package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

type ProjectRepository struct {
	db *sql.DB
}

// Create inserts a new project.
func (r *ProjectRepository) Create(ctx context.Context, p *model.Project) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if p.Source == "" {
		p.Source = "manual"
	}

	tagsJSON := marshalStringArray(p.Tags)
	allowedPrinterIDsJSON, _ := json.Marshal(p.AllowedPrinterIDs)
	defaultSettingsJSON, _ := json.Marshal(p.DefaultSettings)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, target_date, tags, template_id, source, external_order_id, customer_notes, sku, price_cents, printer_type, allowed_printer_ids, default_settings, notes, source_url, source_provider, source_author, source_license, source_description, cover_file_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ID, p.Name, p.Description, p.TargetDate, tagsJSON, p.TemplateID, p.Source, p.ExternalOrderID, p.CustomerNotes, p.SKU, p.PriceCents, p.PrinterType, allowedPrinterIDsJSON, defaultSettingsJSON, p.Notes, p.SourceURL, p.SourceProvider, p.SourceAuthor, p.SourceLicense, p.SourceDescription, p.CoverFileID, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetByID retrieves a project by ID.
func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	var p model.Project
	var tagsJSON, allowedPrinterIDsJSON, defaultSettingsJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, description, target_date, tags, template_id, source, external_order_id, customer_notes, sku, price_cents, printer_type, allowed_printer_ids, default_settings, notes, source_url, source_provider, source_author, source_license, source_description, cover_file_id, created_at, updated_at
		FROM projects WHERE id = ?
	`, id), &p.ID, &p.Name, &p.Description, &p.TargetDate, &tagsJSON, &p.TemplateID, &p.Source, &p.ExternalOrderID, &p.CustomerNotes, &p.SKU, &p.PriceCents, &p.PrinterType, &allowedPrinterIDsJSON, &defaultSettingsJSON, &p.Notes, &p.SourceURL, &p.SourceProvider, &p.SourceAuthor, &p.SourceLicense, &p.SourceDescription, &p.CoverFileID, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Tags = unmarshalStringArray(tagsJSON)
	if allowedPrinterIDsJSON != nil {
		json.Unmarshal(allowedPrinterIDsJSON, &p.AllowedPrinterIDs)
	}
	if defaultSettingsJSON != nil {
		json.Unmarshal(defaultSettingsJSON, &p.DefaultSettings)
	}
	return &p, nil
}

// List retrieves all projects.
func (r *ProjectRepository) List(ctx context.Context) ([]model.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, target_date, tags, template_id, source, external_order_id, customer_notes, sku, price_cents, printer_type, allowed_printer_ids, default_settings, notes, source_url, source_provider, source_author, source_license, source_description, cover_file_id, created_at, updated_at
		FROM projects ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		var tagsJSON, allowedPrinterIDsJSON, defaultSettingsJSON []byte
		if err := scanRow(rows, &p.ID, &p.Name, &p.Description, &p.TargetDate, &tagsJSON, &p.TemplateID, &p.Source, &p.ExternalOrderID, &p.CustomerNotes, &p.SKU, &p.PriceCents, &p.PrinterType, &allowedPrinterIDsJSON, &defaultSettingsJSON, &p.Notes, &p.SourceURL, &p.SourceProvider, &p.SourceAuthor, &p.SourceLicense, &p.SourceDescription, &p.CoverFileID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Tags = unmarshalStringArray(tagsJSON)
		if allowedPrinterIDsJSON != nil {
			json.Unmarshal(allowedPrinterIDsJSON, &p.AllowedPrinterIDs)
		}
		if defaultSettingsJSON != nil {
			json.Unmarshal(defaultSettingsJSON, &p.DefaultSettings)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Update updates a project.
func (r *ProjectRepository) Update(ctx context.Context, p *model.Project) error {
	p.UpdatedAt = time.Now()
	tagsJSON := marshalStringArray(p.Tags)
	allowedPrinterIDsJSON, _ := json.Marshal(p.AllowedPrinterIDs)
	defaultSettingsJSON, _ := json.Marshal(p.DefaultSettings)

	_, err := r.db.ExecContext(ctx, `
		UPDATE projects SET name = ?, description = ?, target_date = ?, tags = ?, template_id = ?, source = ?, external_order_id = ?, customer_notes = ?, sku = ?, price_cents = ?, printer_type = ?, allowed_printer_ids = ?, default_settings = ?, notes = ?, source_url = ?, source_provider = ?, source_author = ?, source_license = ?, source_description = ?, cover_file_id = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.Description, p.TargetDate, tagsJSON, p.TemplateID, p.Source, p.ExternalOrderID, p.CustomerNotes, p.SKU, p.PriceCents, p.PrinterType, allowedPrinterIDsJSON, defaultSettingsJSON, p.Notes, p.SourceURL, p.SourceProvider, p.SourceAuthor, p.SourceLicense, p.SourceDescription, p.CoverFileID, p.UpdatedAt, p.ID)
	return err
}

// Delete removes a project.
func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	return err
}

// ListByTemplateID retrieves all projects created from a given template (legacy).
func (r *ProjectRepository) ListByTemplateID(ctx context.Context, templateID uuid.UUID) ([]model.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, description, target_date, tags, template_id, source, external_order_id, customer_notes, sku, price_cents, printer_type, allowed_printer_ids, default_settings, notes, source_url, source_provider, source_author, source_license, source_description, cover_file_id, created_at, updated_at
		FROM projects WHERE template_id = ? ORDER BY created_at DESC
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []model.Project
	for rows.Next() {
		var p model.Project
		var tagsJSON, allowedPrinterIDsJSON, defaultSettingsJSON []byte
		if err := scanRow(rows, &p.ID, &p.Name, &p.Description, &p.TargetDate, &tagsJSON, &p.TemplateID, &p.Source, &p.ExternalOrderID, &p.CustomerNotes, &p.SKU, &p.PriceCents, &p.PrinterType, &allowedPrinterIDsJSON, &defaultSettingsJSON, &p.Notes, &p.SourceURL, &p.SourceProvider, &p.SourceAuthor, &p.SourceLicense, &p.SourceDescription, &p.CoverFileID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Tags = unmarshalStringArray(tagsJSON)
		if allowedPrinterIDsJSON != nil {
			json.Unmarshal(allowedPrinterIDsJSON, &p.AllowedPrinterIDs)
		}
		if defaultSettingsJSON != nil {
			json.Unmarshal(defaultSettingsJSON, &p.DefaultSettings)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// GetBySKU retrieves a project by SKU.
func (r *ProjectRepository) GetBySKU(ctx context.Context, sku string) (*model.Project, error) {
	var p model.Project
	var tagsJSON, allowedPrinterIDsJSON, defaultSettingsJSON []byte
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, description, target_date, tags, template_id, source, external_order_id, customer_notes, sku, price_cents, printer_type, allowed_printer_ids, default_settings, notes, source_url, source_provider, source_author, source_license, source_description, cover_file_id, created_at, updated_at
		FROM projects WHERE sku = ?
	`, sku), &p.ID, &p.Name, &p.Description, &p.TargetDate, &tagsJSON, &p.TemplateID, &p.Source, &p.ExternalOrderID, &p.CustomerNotes, &p.SKU, &p.PriceCents, &p.PrinterType, &allowedPrinterIDsJSON, &defaultSettingsJSON, &p.Notes, &p.SourceURL, &p.SourceProvider, &p.SourceAuthor, &p.SourceLicense, &p.SourceDescription, &p.CoverFileID, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Tags = unmarshalStringArray(tagsJSON)
	if allowedPrinterIDsJSON != nil {
		json.Unmarshal(allowedPrinterIDsJSON, &p.AllowedPrinterIDs)
	}
	if defaultSettingsJSON != nil {
		json.Unmarshal(defaultSettingsJSON, &p.DefaultSettings)
	}
	return &p, nil
}

// PartRepository handles part database operations.
