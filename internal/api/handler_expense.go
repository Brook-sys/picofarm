package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

type ExpenseHandler struct {
	service *service.ExpenseService
}

// List returns all expenses.
func (h *ExpenseHandler) List(w http.ResponseWriter, r *http.Request) {
	var status *model.ExpenseStatus
	if s := r.URL.Query().Get("status"); s != "" {
		es := model.ExpenseStatus(s)
		status = &es
	}

	expenses, err := h.service.List(r.Context(), status)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if expenses == nil {
		expenses = []model.Expense{}
	}

	respondJSON(w, http.StatusOK, expenses)
}

// Get returns an expense by ID with its items.
func (h *ExpenseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid expense ID")
		return
	}

	expense, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if expense == nil {
		respondError(w, http.StatusNotFound, "expense not found")
		return
	}

	respondJSON(w, http.StatusOK, expense)
}

// UploadReceipt handles receipt file upload.
func (h *ExpenseHandler) UploadReceipt(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	expense, err := h.service.UploadReceipt(r.Context(), header.Filename, data)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, expense)
}

// Confirm confirms an expense and applies inventory changes.
func (h *ExpenseHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid expense ID")
		return
	}

	var req service.ConfirmExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.ConfirmExpense(r.Context(), id, &req); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return updated expense
	expense, _ := h.service.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, expense)
}

// Retry re-triggers AI parsing for a failed or stuck expense.
func (h *ExpenseHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid expense ID")
		return
	}

	expense, err := h.service.RetryParse(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	respondJSON(w, http.StatusOK, expense)
}

// Delete deletes an expense.
func (h *ExpenseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid expense ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SaleHandler handles sale endpoints.
