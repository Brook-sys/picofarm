package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Brook-sys/picofarm/internal/service"
)

// CalculatePricing is stateless: no settings, inventory, quote or hardware writes.
// Limit the whole body (including trailing whitespace), not merely the first JSON
// object. Unknown fields and additional JSON values are deliberately rejected.
func CalculatePricing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var limit *http.MaxBytesError
		if errors.As(err, &limit) {
			respondError(w, http.StatusRequestEntityTooLarge, "pricing request exceeds 128 KiB")
		} else {
			respondError(w, http.StatusBadRequest, "cannot read pricing request")
		}
		return
	}
	var req service.PricingCalculationRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid pricing JSON: "+err.Error())
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		respondError(w, http.StatusBadRequest, "pricing request must contain exactly one JSON object")
		return
	}
	result, err := service.CalculatePricing(req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, result)
}
