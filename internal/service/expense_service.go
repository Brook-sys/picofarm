package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/receipt"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/storage"
	"github.com/google/uuid"
)

type ExpenseService struct {
	repo         *repository.ExpenseRepository
	materialRepo *repository.MaterialRepository
	spoolRepo    *repository.SpoolRepository
	fileRepo     *repository.FileRepository
	settingsRepo *repository.SettingsRepository
	repos        *repository.Repositories // For transaction support
	storage      storage.Storage
	parser       *receipt.Parser
}

// initParser initializes the receipt parser, reading the API key from
// the settings DB first, then falling back to the ANTHROPIC_API_KEY env var.
func (s *ExpenseService) initParser(ctx context.Context) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if s.settingsRepo != nil {
		if setting, err := s.settingsRepo.Get(ctx, "anthropic_api_key"); err == nil && setting != nil && setting.Value != "" {
			apiKey = setting.Value
		}
	}
	s.parser = receipt.NewParserWithKey(apiKey)
}

// UploadReceipt uploads a receipt file and starts AI parsing.
func (s *ExpenseService) UploadReceipt(ctx context.Context, filename string, data []byte) (*model.Expense, error) {
	// Initialize parser lazily
	if s.parser == nil {
		s.initParser(ctx)
	}

	// Store the file using Save with a bytes reader
	reader := bytes.NewReader(data)
	storagePath, _, _, err := s.storage.Save(filename, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to store receipt: %w", err)
	}

	// Create initial expense record
	expense := &model.Expense{
		OccurredAt:      time.Now(),
		Status:          model.ExpenseStatusPending,
		ReceiptFilePath: storagePath,
		Category:        model.ExpenseCategoryOther,
	}

	if err := s.repo.Create(ctx, expense); err != nil {
		return nil, fmt.Errorf("failed to create expense: %w", err)
	}

	// Parse receipt asynchronously if API key is configured
	if s.parser.HasAPIKey() {
		go func() {
			parseCtx := context.Background()
			s.parseReceiptAsync(parseCtx, expense.ID, storagePath, data)
		}()
	}

	return expense, nil
}

// parseReceiptAsync parses a receipt and updates the expense record.
func (s *ExpenseService) parseReceiptAsync(ctx context.Context, expenseID uuid.UUID, _ string, data []byte) {
	slog.Info("starting receipt parsing", "expense_id", expenseID)

	// Detect content type
	contentType := "image/jpeg"
	if len(data) >= 4 {
		if data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
			contentType = "image/png"
		} else if data[0] == '%' && data[1] == 'P' && data[2] == 'D' && data[3] == 'F' {
			contentType = "application/pdf"
		}
	}

	parsed, err := s.parser.ParseFromBytes(ctx, data, contentType)
	if err != nil {
		slog.Error("failed to parse receipt", "expense_id", expenseID, "error", err)
		// Surface the error to the user by updating the expense record
		if expense, getErr := s.repo.GetByID(ctx, expenseID); getErr == nil && expense != nil {
			expense.Notes = fmt.Sprintf("Parse failed: %s", err.Error())
			expense.Status = model.ExpenseStatusRejected
			_ = s.repo.Update(ctx, expense)
		}
		return
	}

	slog.Info("receipt parsed successfully", "expense_id", expenseID, "vendor", parsed.Vendor, "total_cents", parsed.TotalCents)

	// Update expense with parsed data
	expense, err := s.repo.GetByID(ctx, expenseID)
	if err != nil || expense == nil {
		slog.Error("failed to get expense for update", "expense_id", expenseID, "error", err)
		return
	}

	expense.Vendor = parsed.Vendor
	expense.SubtotalCents = parsed.SubtotalCents
	expense.TaxCents = parsed.TaxCents
	expense.ShippingCents = parsed.ShippingCents
	expense.TotalCents = parsed.TotalCents
	expense.Currency = parsed.Currency
	expense.Confidence = parsed.Confidence
	expense.RawOCRText = parsed.RawText

	// Determine primary category based on items
	hasFilament := false
	for _, item := range parsed.Items {
		if item.IsFilament {
			hasFilament = true
			break
		}
	}
	if hasFilament {
		expense.Category = model.ExpenseCategoryFilament
	}

	// Store raw AI response
	rawJSON, _ := json.Marshal(parsed)
	expense.RawAIResponse = rawJSON

	// Parse date
	if parsed.Date != "" {
		if t, err := time.Parse("2006-01-02", parsed.Date); err == nil {
			expense.OccurredAt = t
		}
	}

	if err := s.repo.Update(ctx, expense); err != nil {
		slog.Error("failed to update expense", "expense_id", expenseID, "error", err)
		return
	}

	// Create expense items and auto-create materials + spools for filament
	for _, item := range parsed.Items {
		expenseItem := &model.ExpenseItem{
			ExpenseID:       expenseID,
			Description:     item.Description,
			Quantity:        item.Quantity,
			UnitPriceCents:  item.UnitPriceCents,
			TotalPriceCents: item.TotalPriceCents,
			Category:        item.Category,
			Confidence:      item.Confidence,
		}

		if item.IsFilament && item.Filament != nil {
			expenseItem.Metadata = item.Filament
		}

		if err := s.repo.CreateItem(ctx, expenseItem); err != nil {
			slog.Error("failed to create expense item", "expense_id", expenseID, "error", err)
			continue
		}

		// Auto-create material + spools for filament items
		if item.IsFilament && item.Filament != nil {
			materialID, err := s.findOrCreateMaterial(ctx, item.Filament, item.UnitPriceCents)
			if err != nil {
				slog.Error("failed to find/create material", "expense_id", expenseID, "description", item.Description, "error", err)
				continue
			}

			weightGrams := item.Filament.WeightGrams
			if weightGrams == 0 {
				weightGrams = 1000
			}

			quantity := int(item.Quantity)
			if quantity < 1 {
				quantity = 1
			}

			for i := 0; i < quantity; i++ {
				spool := &model.MaterialSpool{
					MaterialID:      materialID,
					InitialWeight:   weightGrams,
					RemainingWeight: weightGrams,
					PurchaseDate:    &expense.OccurredAt,
					PurchaseCost:    float64(item.TotalPriceCents) / 100.0 / float64(quantity),
					Status:          model.SpoolStatusNew,
					Notes:           fmt.Sprintf("From receipt: %s", expense.Vendor),
				}

				if err := s.spoolRepo.Create(ctx, spool); err != nil {
					slog.Error("failed to create spool", "expense_id", expenseID, "error", err)
					continue
				}

				if i == 0 {
					expenseItem.MatchedSpoolID = &spool.ID
					expenseItem.MatchedMaterialID = &materialID
					expenseItem.ActionTaken = model.ExpenseItemActionCreatedSpool
				}
			}

			if err := s.repo.UpdateItem(ctx, expenseItem); err != nil {
				slog.Error("failed to update expense item", "expense_id", expenseID, "error", err)
			}

			slog.Info("auto-created material + spools", "expense_id", expenseID, "material_id", materialID, "quantity", quantity, "description", item.Description)
		} else if !item.IsFilament && item.Category != "shipping" {
			// Auto-create supply material for non-filament, non-shipping items
			materialID, err := s.findOrCreateSupplyMaterial(ctx, item.Description, parsed.Vendor, item.UnitPriceCents)
			if err != nil {
				slog.Error("failed to find/create supply material", "expense_id", expenseID, "description", item.Description, "error", err)
				continue
			}

			expenseItem.MatchedMaterialID = &materialID
			expenseItem.ActionTaken = model.ExpenseItemActionCreatedSupply

			if err := s.repo.UpdateItem(ctx, expenseItem); err != nil {
				slog.Error("failed to update expense item with supply material", "expense_id", expenseID, "error", err)
			}

			slog.Info("auto-created supply material", "expense_id", expenseID, "material_id", materialID, "description", item.Description)
		}
	}

	// Auto-confirm the expense since materials were processed
	expense.Status = model.ExpenseStatusConfirmed
	if err := s.repo.Update(ctx, expense); err != nil {
		slog.Error("failed to auto-confirm expense", "expense_id", expenseID, "error", err)
	}

	slog.Info("expense auto-confirmed", "expense_id", expenseID, "items", len(parsed.Items))
}

// findOrCreateMaterial finds an existing material matching the filament metadata,
// or creates a new one. Returns the material ID.
// filamentColorHex maps common filament color names to hex values.
var filamentColorHex = map[string]string{
	"black":       "#000000",
	"white":       "#FFFFFF",
	"red":         "#FF0000",
	"blue":        "#0000FF",
	"green":       "#008000",
	"yellow":      "#FFFF00",
	"orange":      "#FF8C00",
	"purple":      "#800080",
	"pink":        "#FF69B4",
	"gray":        "#808080",
	"grey":        "#808080",
	"silver":      "#C0C0C0",
	"gold":        "#FFD700",
	"brown":       "#8B4513",
	"beige":       "#F5DEB3",
	"ivory":       "#FFFFF0",
	"cream":       "#FFFDD0",
	"cyan":        "#00FFFF",
	"magenta":     "#FF00FF",
	"teal":        "#008080",
	"navy":        "#000080",
	"olive":       "#808000",
	"maroon":      "#800000",
	"coral":       "#FF7F50",
	"salmon":      "#FA8072",
	"turquoise":   "#40E0D0",
	"lavender":    "#E6E6FA",
	"lilac":       "#C8A2C8",
	"mint":        "#3EB489",
	"jade":        "#00A86B",
	"transparent": "#E0E0E0",
	"natural":     "#F5F0E1",
	"matte black": "#1A1A1A",
	"matte white": "#F0F0F0",
	"charcoal":    "#36454F",
	"dark grey":   "#555555",
	"dark gray":   "#555555",
	"light grey":  "#BBBBBB",
	"light gray":  "#BBBBBB",
	"dark blue":   "#00008B",
	"light blue":  "#ADD8E6",
	"sky blue":    "#87CEEB",
	"dark green":  "#006400",
	"light green": "#90EE90",
	"dark red":    "#8B0000",
	"bambu green": "#00AE42",
	"jade white":  "#E8E0D8",
	"arctic blue": "#6CB4EE",
}

func (s *ExpenseService) findOrCreateMaterial(ctx context.Context, fm *model.FilamentMetadata, unitPriceCents int) (uuid.UUID, error) {
	matType := model.MaterialType(strings.ToLower(fm.MaterialType))
	manufacturer := fm.Brand
	color := fm.Color

	// Try to find existing material
	existing, err := s.materialRepo.FindByTypeManufacturerColor(ctx, matType, manufacturer, color)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find material: %w", err)
	}
	if existing != nil {
		return existing.ID, nil
	}

	// Build a descriptive name
	name := manufacturer
	if name != "" {
		name += " "
	}
	name += strings.ToUpper(string(matType))
	if color != "" {
		name += " - " + color
	}

	// Calculate cost per kg from the unit price
	weightKg := fm.WeightGrams / 1000.0
	if weightKg <= 0 {
		weightKg = 1.0
	}
	costPerKg := float64(unitPriceCents) / 100.0 / weightKg

	// Resolve color hex: use AI-provided value, then fallback to color name lookup
	colorHex := fm.ColorHex
	if colorHex == "" && color != "" {
		colorHex = filamentColorHex[strings.ToLower(color)]
	}

	mat := &model.Material{
		Name:         name,
		Type:         matType,
		Manufacturer: manufacturer,
		Color:        color,
		ColorHex:     colorHex,
		Density:      1.24, // reasonable default for PLA/PETG
		CostPerKg:    costPerKg,
	}

	if err := s.materialRepo.Create(ctx, mat); err != nil {
		return uuid.Nil, fmt.Errorf("create material: %w", err)
	}

	return mat.ID, nil
}

// findOrCreateSupplyMaterial finds or creates a supply-type material for non-filament receipt items.
func (s *ExpenseService) findOrCreateSupplyMaterial(ctx context.Context, description string, vendor string, unitPriceCents int) (uuid.UUID, error) {
	// Try to find existing supply material by type + name
	existing, err := s.materialRepo.FindByTypeAndName(ctx, model.MaterialTypeSupply, description)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find supply material: %w", err)
	}
	if existing != nil {
		return existing.ID, nil
	}

	// Create new supply material
	mat := &model.Material{
		Name:         description,
		Type:         model.MaterialTypeSupply,
		Manufacturer: vendor,
		CostPerKg:    float64(unitPriceCents) / 100.0, // repurposed as per-unit cost
	}

	if err := s.materialRepo.Create(ctx, mat); err != nil {
		return uuid.Nil, fmt.Errorf("create supply material: %w", err)
	}

	return mat.ID, nil
}

// GetByID retrieves an expense by ID with its items.
func (s *ExpenseService) GetByID(ctx context.Context, id uuid.UUID) (*model.Expense, error) {
	expense, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if expense == nil {
		return nil, nil
	}

	// Load items
	items, err := s.repo.GetItemsByExpenseID(ctx, id)
	if err != nil {
		return nil, err
	}
	expense.Items = items

	return expense, nil
}

// List retrieves all expenses.
func (s *ExpenseService) List(ctx context.Context, status *model.ExpenseStatus) ([]model.Expense, error) {
	return s.repo.List(ctx, status)
}

// ConfirmExpenseRequest contains the data to confirm an expense.
type ConfirmExpenseRequest struct {
	Items []ConfirmExpenseItem `json:"items"`
}

// ConfirmExpenseItem contains the user's decisions for each expense item.
type ConfirmExpenseItem struct {
	ItemID      uuid.UUID       `json:"item_id"`
	CreateSpool bool            `json:"create_spool"`
	MaterialID  *uuid.UUID      `json:"material_id,omitempty"`  // Use existing material
	NewMaterial *model.Material `json:"new_material,omitempty"` // Create new material
	WeightGrams float64         `json:"weight_grams,omitempty"`
	DiameterMM  float64         `json:"diameter_mm,omitempty"`
}

// ConfirmExpense confirms an expense and applies inventory changes.
// All database operations are wrapped in a transaction for atomicity.
func (s *ExpenseService) ConfirmExpense(ctx context.Context, id uuid.UUID, req *ConfirmExpenseRequest) error {
	expense, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if expense == nil {
		return fmt.Errorf("expense not found")
	}

	if expense.Status != model.ExpenseStatusPending {
		return fmt.Errorf("expense is not pending")
	}

	// Execute all inventory changes within a transaction
	return s.repos.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Process each item
		for _, confirmItem := range req.Items {
			// Find the expense item
			var expenseItem *model.ExpenseItem
			for i := range expense.Items {
				if expense.Items[i].ID == confirmItem.ItemID {
					expenseItem = &expense.Items[i]
					break
				}
			}
			if expenseItem == nil {
				continue
			}

			if confirmItem.CreateSpool {
				var materialID uuid.UUID

				// Create new material if specified
				if confirmItem.NewMaterial != nil {
					if err := s.materialRepo.CreateTx(ctx, tx, confirmItem.NewMaterial); err != nil {
						return fmt.Errorf("failed to create material: %w", err)
					}
					materialID = confirmItem.NewMaterial.ID
				} else if confirmItem.MaterialID != nil {
					materialID = *confirmItem.MaterialID
				} else {
					continue // No material specified, skip
				}

				// Determine weight
				weightGrams := confirmItem.WeightGrams
				if weightGrams == 0 && expenseItem.Metadata != nil {
					weightGrams = expenseItem.Metadata.WeightGrams
				}
				if weightGrams == 0 {
					weightGrams = 1000 // Default 1kg
				}

				// Create spools (one per quantity)
				quantity := int(expenseItem.Quantity)
				if quantity < 1 {
					quantity = 1
				}

				for i := 0; i < quantity; i++ {
					spool := &model.MaterialSpool{
						MaterialID:      materialID,
						InitialWeight:   weightGrams,
						RemainingWeight: weightGrams,
						PurchaseDate:    &expense.OccurredAt,
						PurchaseCost:    float64(expenseItem.TotalPriceCents) / 100.0 / float64(quantity),
						Status:          model.SpoolStatusNew,
						Notes:           fmt.Sprintf("From receipt: %s", expense.Vendor),
					}

					if err := s.spoolRepo.CreateTx(ctx, tx, spool); err != nil {
						return fmt.Errorf("failed to create spool: %w", err)
					}

					// Update expense item with matched spool
					if i == 0 {
						expenseItem.MatchedSpoolID = &spool.ID
						expenseItem.MatchedMaterialID = &materialID
						expenseItem.ActionTaken = model.ExpenseItemActionCreatedSpool
					}
				}

				if err := s.repo.UpdateItemTx(ctx, tx, expenseItem); err != nil {
					return fmt.Errorf("failed to update expense item: %w", err)
				}
			} else {
				expenseItem.ActionTaken = model.ExpenseItemActionSkipped
				if err := s.repo.UpdateItemTx(ctx, tx, expenseItem); err != nil {
					return fmt.Errorf("failed to update expense item: %w", err)
				}
			}
		}

		// Mark expense as confirmed
		expense.Status = model.ExpenseStatusConfirmed
		if err := s.repo.UpdateTx(ctx, tx, expense); err != nil {
			return fmt.Errorf("failed to confirm expense: %w", err)
		}

		return nil
	})
}

// RetryParse re-reads the stored receipt file and re-triggers AI parsing.
func (s *ExpenseService) RetryParse(ctx context.Context, id uuid.UUID) (*model.Expense, error) {
	// Re-initialize parser on retry so it picks up any new API key from settings
	s.initParser(ctx)

	expense, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if expense == nil {
		return nil, fmt.Errorf("expense not found")
	}
	if expense.ReceiptFilePath == "" {
		return nil, fmt.Errorf("no receipt file stored for this expense")
	}

	// Read the file back from storage
	reader, err := s.storage.Get(expense.ReceiptFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stored receipt: %w", err)
	}
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read stored receipt: %w", err)
	}

	// Delete any old expense items from a previous parse attempt
	_ = s.repo.DeleteItemsByExpenseID(ctx, id)

	// Reset expense to pending state
	expense.Status = model.ExpenseStatusPending
	expense.Notes = ""
	expense.Vendor = ""
	expense.SubtotalCents = 0
	expense.TaxCents = 0
	expense.ShippingCents = 0
	expense.TotalCents = 0
	expense.Confidence = 0
	expense.RawOCRText = ""
	expense.RawAIResponse = nil
	expense.Category = model.ExpenseCategoryOther
	if err := s.repo.Update(ctx, expense); err != nil {
		return nil, fmt.Errorf("failed to reset expense: %w", err)
	}

	// Re-trigger parsing if API key is configured
	if s.parser.HasAPIKey() {
		go func() {
			parseCtx := context.Background()
			s.parseReceiptAsync(parseCtx, expense.ID, expense.ReceiptFilePath, data)
		}()
	}

	return expense, nil
}

// Delete deletes an expense.
func (s *ExpenseService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// SaleService handles sales and revenue tracking.
