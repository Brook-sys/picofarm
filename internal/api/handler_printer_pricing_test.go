package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestPrinterPricingAPI(t *testing.T) {
	env := newTestEnv(t)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		rr := httptest.NewRecorder()
		env.handler.ServeHTTP(rr, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rr
	}
	rr := request(http.MethodPost, "/api/printers", `{"name":"Manual","connection_type":"manual","cost_per_hour_cents":123,"pricing_config":{"currency":"BRL","rate_mode":"direct","direct_rate_per_hour":"1.23","includes_energy":true}}`)
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Code, rr.Body.String())
	}
	var p model.Printer
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	path := "/api/printers/" + p.ID.String()
	rr = request(http.MethodPatch, path, `{"notes":"legacy client","cost_per_hour_cents":200}`)
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.PricingConfig == nil || p.PricingConfig.Currency != "BRL" || p.PricingConfig.DirectRatePerHour != "1.23" || !p.PricingConfig.IncludesEnergy {
		t.Fatalf("omitted profile overwritten: %+v", p.PricingConfig)
	}
	rr = request(http.MethodPatch, path, `{"pricing_config":{"currency":"","rate_mode":"direct","direct_rate_per_hour":"0"}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatal("update must reject missing currency", rr.Code, rr.Body.String())
	}
	rr = request(http.MethodPost, "/api/printers", `{"name":"Invalid","connection_type":"manual","pricing_config":{"currency":"BRL","rate_mode":"direct","direct_rate_per_hour":"1e3"}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatal("create must reject exponent", rr.Code)
	}
	rr = request(http.MethodPatch, path, `{"pricing_config":null}`)
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code, rr.Body.String())
	}
	rr = request(http.MethodGet, path, "")
	var cleared model.Printer
	if err := json.Unmarshal(rr.Body.Bytes(), &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.PricingConfig != nil || cleared.CostPerHourCents != 200 {
		t.Fatal("clear or legacy update failed")
	}
}
