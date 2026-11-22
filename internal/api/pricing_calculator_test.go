package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/service"
)

const pricingValidJSON = `{"currency":"USD","quantity":2,"production":{"mode":"proportional","grams":"100","hours":"1"},"material_cost_per_kg":"10","target":{"anchor":"price","value":"10"}}`

func TestPricingHTTPStrictDecoding(t *testing.T) {
	cases := []struct {
		name, body string
		status     int
	}{
		{"valid", pricingValidJSON, 200}, {"unknown", strings.Replace(pricingValidJSON, `"quantity":2`, `"quantity":2,"surprise":1`, 1), 400},
		{"nested unknown", strings.Replace(pricingValidJSON, `"hours":"1"`, `"hours":"1","surprise":1`, 1), 400},
		{"numeric decimal", strings.Replace(pricingValidJSON, `"grams":"100"`, `"grams":100`, 1), 400},
		{"fraction quantity", strings.Replace(pricingValidJSON, `"quantity":2`, `"quantity":2.1`, 1), 400},
		{"trailing value", pricingValidJSON + ` {}`, 400}, {"trailing garbage", pricingValidJSON + ` abc`, 400}, {"empty", "", 400}, {"null", "null", 400}, {"array", "[]", 400},
		{"oversize", pricingValidJSON + strings.Repeat(" ", 128<<10), 413},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			CalculatePricing(w, httptest.NewRequest(http.MethodPost, "/api/pricing/calculate", strings.NewReader(tc.body)))
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("pricing response must not be cached")
			}
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if tc.status == 200 {
				var result service.PricingCalculationResult
				if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Base.Price != "10.00" || result.Base.Gain != "8.00" {
					t.Fatalf("%+v", result)
				}
			}
		})
	}
}
func TestPricingRouterAuthentication(t *testing.T) {
	router := NewRouterWithOptions(&service.Services{}, nil, RouterOptions{ApiToken: "test-pricing-token", StaticDir: t.TempDir()})
	for _, authenticated := range []bool{false, true} {
		req := httptest.NewRequest(http.MethodPost, "/api/pricing/calculate", strings.NewReader(pricingValidJSON))
		if authenticated {
			req.Header.Set("Authorization", "Bearer test-pricing-token")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		expected := http.StatusUnauthorized
		if authenticated {
			expected = 200
		}
		if w.Code != expected {
			t.Fatalf("authenticated=%v status=%d body=%s", authenticated, w.Code, w.Body.String())
		}
	}
}
