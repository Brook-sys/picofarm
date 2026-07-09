package olx

import (
	"context"
	"encoding/json"
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
