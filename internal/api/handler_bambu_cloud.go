package api

import (
	"encoding/json"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
)

type BambuCloudHandler struct {
	service *service.BambuCloudService
}

// Login authenticates with Bambu Cloud.
func (h *BambuCloudHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		respondError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	needsCode, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	status := "ok"
	if needsCode {
		status = "verify_code_required"
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": status})
}

// Verify completes login with a verification code.
func (h *BambuCloudHandler) Verify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Code == "" {
		respondError(w, http.StatusBadRequest, "email and code are required")
		return
	}

	if err := h.service.VerifyCode(r.Context(), req.Email, req.Code); err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Status returns the current Bambu Cloud auth status.
func (h *BambuCloudHandler) Status(w http.ResponseWriter, r *http.Request) {
	auth, err := h.service.GetStoredAuth(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if auth == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"connected": false,
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connected": true,
		"email":     auth.Email,
	})
}

// Devices fetches the list of printers from Bambu Cloud.
func (h *BambuCloudHandler) Devices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.service.GetDevices(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, devices)
}

// AddDevice creates a printer from a cloud device.
func (h *BambuCloudHandler) AddDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DevID string `json:"dev_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DevID == "" {
		respondError(w, http.StatusBadRequest, "dev_id is required")
		return
	}

	p, err := h.service.AddDevice(r.Context(), req.DevID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, p)
}

// Logout clears stored Bambu Cloud credentials.
func (h *BambuCloudHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context()); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ProjectSupplyHandler handles project supply HTTP requests.
