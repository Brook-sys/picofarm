package olx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientValidateAPIKeyUsesBearerTokenAndParsesAccount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			t.Fatalf("expected /me, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fake-olx-key" {
			t.Fatalf("expected bearer auth, got %q", got)
		}
		if err := json.NewEncoder(w).Encode(map[string]string{
			"id":   "olx-account-123",
			"name": "PicoFarm OLX",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient().WithBaseURL(server.URL).WithHTTPClient(server.Client())
	accountID, name, err := client.ValidateAPIKey(context.Background(), "fake-olx-key")
	if err != nil {
		t.Fatalf("validate api key: %v", err)
	}
	if accountID != "olx-account-123" || name != "PicoFarm OLX" {
		t.Fatalf("unexpected account: %q %q", accountID, name)
	}
}

func TestClientValidateAPIKeyReturnsSanitizedProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid api_key=fake-olx-key token=hidden"}`, http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient().WithBaseURL(server.URL).WithHTTPClient(server.Client())
	_, _, err := client.ValidateAPIKey(context.Background(), "fake-olx-key")
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if got := err.Error(); got == "" || got == "invalid api_key=fake-olx-key token=hidden" {
		t.Fatalf("expected useful provider error, got %q", got)
	}
}

func TestClientListAdsUsesBearerTokenAndParsesListings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ads" {
			t.Fatalf("expected /ads, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fake-olx-key" {
			t.Fatalf("expected bearer auth, got %q", got)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Fatalf("expected page=1, got %q", got)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"ads": []map[string]any{{
				"id":          "olx-ad-1",
				"title":       "Miniatura dragão 3D",
				"description": "Impresso sob demanda",
				"url":         "https://www.olx.com.br/item/olx-ad-1",
				"status":      "active",
				"price":       129.90,
				"currency":    "BRL",
				"visible":     true,
			}},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient().WithBaseURL(server.URL).WithHTTPClient(server.Client())
	ads, err := client.ListAds(context.Background(), "fake-olx-key")
	if err != nil {
		t.Fatalf("list ads: %v", err)
	}
	if len(ads) != 1 {
		t.Fatalf("expected 1 ad, got %d", len(ads))
	}
	ad := ads[0]
	if ad.ID != "olx-ad-1" || ad.Title != "Miniatura dragão 3D" || ad.PriceCents != 12990 || ad.Currency != "BRL" || !ad.Visible {
		t.Fatalf("unexpected ad: %#v", ad)
	}
}

func TestClientListAdsReturnsUsefulHTTPStatusForEmptyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := NewClient().WithBaseURL(server.URL).WithHTTPClient(server.Client())
	_, err := client.ListAds(context.Background(), "fake-olx-key")
	if err == nil || (err.Error() != fmt.Sprintf("olx ads: %s", http.StatusText(http.StatusForbidden)) && err.Error() != "olx ads: 403 Forbidden") {
		t.Fatalf("expected useful forbidden error, got %v", err)
	}
}
