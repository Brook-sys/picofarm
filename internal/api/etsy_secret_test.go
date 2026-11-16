package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/go-chi/chi/v5"
)

func TestSettingsProtectsEtsySharedSecret(t *testing.T) {
	env := newTestEnv(t)
	h := &SettingsHandler{service: env.services.Settings}
	router := chi.NewRouter()
	router.Get("/api/settings", h.List)
	router.Get("/api/settings/{key}", h.Get)
	router.Put("/api/settings/{key}", h.Set)
	router.Delete("/api/settings/{key}", h.Delete)
	for _, secret := range []string{"short", "test-shared-secret-long"} {
		if err := env.services.Settings.Set(t.Context(), service.EtsySharedSecretKey, secret); err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"GET", "PUT", "DELETE"} {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(method, "/api/settings/etsy_shared_secret", strings.NewReader(`{"value":""}`)))
			if rr.Code != 403 || strings.Contains(rr.Body.String(), secret) {
				t.Fatalf("%s: %d %s", method, rr.Code, rr.Body.String())
			}
		}
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest("GET", "/api/settings", nil))
		if rr.Code != 200 || strings.Contains(rr.Body.String(), "etsy_shared_secret") || strings.Contains(rr.Body.String(), secret) {
			t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
		}
		saved, err := env.services.Settings.Get(t.Context(), service.EtsySharedSecretKey)
		if err != nil || saved == nil || saved.Value != secret {
			t.Fatal("secret modified through generic API")
		}
	}
}

func TestEtsyConfigureSharedSecret(t *testing.T) {
	env := newTestEnv(t)
	h := NewEtsyHandler(env.services.Etsy, env.services.Orders)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{"client_id":"test-key"}`, 400},
		{`{"client_id":"key:secret","shared_secret":"test-secret"}`, 400},
		{`{"client_id":"test-key","shared_secret":"test-secret"}`, 200},
		{`{"client_id":"test-key"}`, 200},
		{`{"client_id":"different-key"}`, 400},
	} {
		rr := httptest.NewRecorder()
		h.Configure(rr, httptest.NewRequest("PUT", "/api/integrations/etsy/configure", strings.NewReader(tc.body)))
		if rr.Code != tc.code || strings.Contains(rr.Body.String(), "test-secret") {
			t.Fatalf("configure: %d %s", rr.Code, rr.Body.String())
		}
		status := httptest.NewRecorder()
		h.GetStatus(status, httptest.NewRequest("GET", "/api/integrations/etsy/status", nil))
		if status.Code != 200 || strings.Contains(status.Body.String(), "test-secret") || strings.Contains(status.Body.String(), "shared_secret") {
			t.Fatalf("status leaked credentials: %d", status.Code)
		}
	}
}
