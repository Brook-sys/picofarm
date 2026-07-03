package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/service"
)

type ProjectSupplyHandler struct {
	service *service.ProjectSupplyService
}

// List retrieves all supplies for a project.
func (h *ProjectSupplyHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid project ID")
		return
	}

	supplies, err := h.service.ListByProject(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if supplies == nil {
		supplies = []model.ProjectSupply{}
	}
	respondJSON(w, http.StatusOK, supplies)
}

// Create creates a new project supply.
func (h *ProjectSupplyHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid project ID")
		return
	}

	var supply model.ProjectSupply
	if err := json.NewDecoder(r.Body).Decode(&supply); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	supply.ProjectID = projectID

	if err := h.service.Create(r.Context(), &supply); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, supply)
}

// Delete removes a project supply.
func (h *ProjectSupplyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	supplyID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid supply ID")
		return
	}

	if err := h.service.Delete(r.Context(), supplyID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BackupHandler handles database backup HTTP requests.
