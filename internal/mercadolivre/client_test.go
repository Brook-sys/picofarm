package mercadolivre

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestClient_WithHTTPClient_AllowsFakeTransport(t *testing.T) {
	called := false
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		if req.Header.Get("Authorization") == "" {
			t.Fatalf("expected Authorization header")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	c := NewClient("test-token").WithHTTPClient(&http.Client{Transport: rt})

	resp, err := c.do(context.Background(), http.MethodGet, "/users/me")
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Fatalf("expected transport to be called")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClient_GetCurrentUser_UsesBearerAndParsesFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/user.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/me" {
			t.Fatalf("path = %q, want /users/me", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fake-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer fake-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer ts.Close()

	oldBase := BaseURL
	defer func() { BaseURL = oldBase }()
	BaseURL = ts.URL

	user, err := NewClient("fake-token").WithHTTPClient(ts.Client()).GetCurrentUser(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if user.ID != 123456789 || user.Nickname != "PICO_TEST_USER" || user.SiteID != "MLB" {
		t.Fatalf("unexpected user: %+v", user)
	}
}

func TestClient_GetCurrentUser_ErrorDoesNotExposeBearer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad token"}`, http.StatusUnauthorized)
	}))
	defer ts.Close()

	oldBase := BaseURL
	defer func() { BaseURL = oldBase }()
	BaseURL = ts.URL

	_, err := NewClient("super-secret-token").WithHTTPClient(ts.Client()).GetCurrentUser(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := err.Error(); got == "" || got == "super-secret-token" {
		t.Fatalf("unexpected error: %q", got)
	}
}
