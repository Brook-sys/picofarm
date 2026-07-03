package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/validation"
	"github.com/google/uuid"
)

type TemplateHandler struct {
	service *service.TemplateService
}

// List returns all templates.
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"

	templates, err := h.service.List(r.Context(), activeOnly)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if templates == nil {
		templates = []model.Template{}
	}

	respondJSON(w, http.StatusOK, templates)
}

// Create creates a new template.
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var template model.Template
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input
	v := validation.New()
	v.Required("name", template.Name)
	v.MaxLength("name", template.Name, 255)
	v.MaxLength("description", template.Description, 5000)
	v.NoControlChars("name", template.Name)
	v.NonNegative("estimated_print_seconds", template.EstimatedPrintSeconds)
	v.NonNegativeFloat("estimated_material_grams", template.EstimatedMaterialGrams)
	if err := v.Error(); err != nil {
		respondValidationError(w, err)
		return
	}

	if err := h.service.Create(r.Context(), &template); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, template)
}

// Get returns a template by ID with its designs.
func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	template, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if template == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}

	respondJSON(w, http.StatusOK, template)
}

// Update updates a template.
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	template, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if template == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(template); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	template.ID = id

	if err := h.service.Update(r.Context(), template); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, template)
}

// Delete removes a template.
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddDesignRequest represents the request body for adding a design to a template.
type AddDesignRequest struct {
	DesignID  string `json:"design_id"`
	Quantity  int    `json:"quantity"`
	IsPrimary bool   `json:"is_primary"`
	Notes     string `json:"notes"`
}

// AddDesign adds a design to a template.
func (h *TemplateHandler) AddDesign(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	var req AddDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	designID, err := uuid.Parse(req.DesignID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}

	td := &model.TemplateDesign{
		TemplateID: templateID,
		DesignID:   designID,
		Quantity:   req.Quantity,
		IsPrimary:  req.IsPrimary,
		Notes:      req.Notes,
	}

	if err := h.service.AddDesign(r.Context(), td); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, td)
}

// RemoveDesign removes a design from a template.
func (h *TemplateHandler) RemoveDesign(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	designID, err := parseUUID(r, "designId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid design ID")
		return
	}

	if err := h.service.RemoveDesign(r.Context(), templateID, designID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// InstantiateRequest represents the request body for instantiating a template.
type InstantiateRequest struct {
	OrderQuantity   int    `json:"order_quantity"`
	CustomerNotes   string `json:"customer_notes"`
	ExternalOrderID string `json:"external_order_id"`
	Source          string `json:"source"`
	MaterialSpoolID string `json:"material_spool_id"`
}

// Instantiate creates a project from a template.
func (h *TemplateHandler) Instantiate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	var req InstantiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	opts := service.CreateFromTemplateOptions{
		OrderQuantity:   req.OrderQuantity,
		CustomerNotes:   req.CustomerNotes,
		ExternalOrderID: req.ExternalOrderID,
		Source:          req.Source,
	}

	if req.MaterialSpoolID != "" {
		spoolID, err := uuid.Parse(req.MaterialSpoolID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid material spool ID")
			return
		}
		opts.MaterialSpoolID = &spoolID
	}

	project, jobs, err := h.service.CreateProjectFromTemplate(r.Context(), templateID, opts)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"project": project,
		"jobs":    jobs,
	})
}

// ListMaterials returns all materials for a template/recipe.
func (h *TemplateHandler) ListMaterials(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	materials, err := h.service.ListMaterials(r.Context(), templateID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if materials == nil {
		materials = []model.RecipeMaterial{}
	}

	respondJSON(w, http.StatusOK, materials)
}

// AddMaterialRequest represents the request body for adding a material.
type AddMaterialRequest struct {
	MaterialType  string           `json:"material_type"`
	ColorSpec     *model.ColorSpec `json:"color_spec,omitempty"`
	WeightGrams   float64          `json:"weight_grams"`
	AMSPosition   *int             `json:"ams_position,omitempty"`
	SequenceOrder int              `json:"sequence_order"`
	Notes         string           `json:"notes"`
}

// AddMaterial adds a material to a template/recipe.
func (h *TemplateHandler) AddMaterial(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	var req AddMaterialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	material := &model.RecipeMaterial{
		RecipeID:      templateID,
		MaterialType:  model.MaterialType(req.MaterialType),
		ColorSpec:     req.ColorSpec,
		WeightGrams:   req.WeightGrams,
		AMSPosition:   req.AMSPosition,
		SequenceOrder: req.SequenceOrder,
		Notes:         req.Notes,
	}

	if err := h.service.AddMaterial(r.Context(), material); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, material)
}

// UpdateMaterial updates a material in a template/recipe.
func (h *TemplateHandler) UpdateMaterial(w http.ResponseWriter, r *http.Request) {
	materialID, err := parseUUID(r, "materialId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid material ID")
		return
	}

	material, err := h.service.GetMaterial(r.Context(), materialID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if material == nil {
		respondError(w, http.StatusNotFound, "material not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(material); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	material.ID = materialID

	if err := h.service.UpdateMaterial(r.Context(), material); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, material)
}

// RemoveMaterial removes a material from a template/recipe.
func (h *TemplateHandler) RemoveMaterial(w http.ResponseWriter, r *http.Request) {
	materialID, err := parseUUID(r, "materialId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid material ID")
		return
	}

	if err := h.service.RemoveMaterial(r.Context(), materialID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCompatiblePrinters returns printers that match recipe constraints.
func (h *TemplateHandler) GetCompatiblePrinters(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	printers, err := h.service.FindCompatiblePrinters(r.Context(), templateID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if printers == nil {
		printers = []model.Printer{}
	}

	respondJSON(w, http.StatusOK, printers)
}

// GetCompatibleSpools returns spools that match recipe material requirements.
func (h *TemplateHandler) GetCompatibleSpools(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	spools, err := h.service.FindCompatibleSpools(r.Context(), templateID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if spools == nil {
		spools = []service.CompatibleSpool{}
	}

	respondJSON(w, http.StatusOK, spools)
}

// GetCostEstimate returns the cost breakdown for a recipe.
func (h *TemplateHandler) GetCostEstimate(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	estimate, err := h.service.CalculateRecipeCost(r.Context(), templateID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, estimate)
}

// ValidatePrinter checks if a printer meets recipe constraints.
func (h *TemplateHandler) ValidatePrinter(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	printerID, err := parseUUID(r, "printerId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid printer ID")
		return
	}

	result, err := h.service.ValidatePrinterForRecipe(r.Context(), templateID, printerID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ListSupplies returns all supply items for a recipe.
func (h *TemplateHandler) ListSupplies(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	supplies, err := h.service.ListSupplies(r.Context(), templateID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if supplies == nil {
		supplies = []model.RecipeSupply{}
	}

	respondJSON(w, http.StatusOK, supplies)
}

// AddSupply adds a supply item to a recipe.
func (h *TemplateHandler) AddSupply(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	var supply model.RecipeSupply
	if err := json.NewDecoder(r.Body).Decode(&supply); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	supply.RecipeID = templateID

	if err := h.service.AddSupply(r.Context(), &supply); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, supply)
}

// UpdateSupply updates a supply item in a recipe.
func (h *TemplateHandler) UpdateSupply(w http.ResponseWriter, r *http.Request) {
	supplyID, err := parseUUID(r, "supplyId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid supply ID")
		return
	}

	supply, err := h.service.GetSupply(r.Context(), supplyID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if supply == nil {
		respondError(w, http.StatusNotFound, "supply not found")
		return
	}

	if err := json.NewDecoder(r.Body).Decode(supply); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	supply.ID = supplyID

	if err := h.service.UpdateSupply(r.Context(), supply); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, supply)
}

// RemoveSupply removes a supply item from a recipe.
func (h *TemplateHandler) RemoveSupply(w http.ResponseWriter, r *http.Request) {
	supplyID, err := parseUUID(r, "supplyId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid supply ID")
		return
	}

	if err := h.service.RemoveSupply(r.Context(), supplyID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAnalytics returns aggregated performance analytics for a template.
func (h *TemplateHandler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	templateID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	analytics, err := h.service.GetTemplateAnalytics(r.Context(), templateID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, analytics)
}

// SettingsHandler handles settings endpoints.
