package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/realtime"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/Brook-sys/picofarm/internal/storage"
)

func newCurrencyRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repos := repository.NewRepositories(db)
	hub := realtime.NewHub()
	services := service.NewServices(repos, storage.NewLocalStorage(t.TempDir()), printer.NewManager(), hub)
	return NewRouterWithOptions(services, hub, RouterOptions{})
}

func TestCurrencySettingRejectsUnsupportedCode(t *testing.T) {
	router := newCurrencyRouter(t)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/currency_code", strings.NewReader(`{"value":"XYZ"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", res.Code, res.Body.String())
	}
}

func TestPublicBusinessInfoAlwaysExposesCurrency(t *testing.T) {
	router := newCurrencyRouter(t)

	getCurrency := func() string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/public/business-info", nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		currency, _ := body["currency_code"].(string)
		return currency
	}

	if currency := getCurrency(); currency != "USD" {
		t.Fatalf("expected default USD, got %q", currency)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/settings/currency_code", strings.NewReader(`{"value":"BRL"}`))
	put.Header.Set("Content-Type", "application/json")
	putRes := httptest.NewRecorder()
	router.ServeHTTP(putRes, put)
	if putRes.Code != http.StatusOK {
		t.Fatalf("expected save 200, got %d: %s", putRes.Code, putRes.Body.String())
	}
	if currency := getCurrency(); currency != "BRL" {
		t.Fatalf("expected BRL after save, got %q", currency)
	}
}
