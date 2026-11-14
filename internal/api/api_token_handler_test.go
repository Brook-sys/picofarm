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

// newTokenTestEnv builds a route-aware environment with explicit router options,
// which is what the auth matrix needs: newTestEnv always uses NewRouter and
// therefore the ambient environment.
func newTokenTestEnv(t *testing.T, opts RouterOptions) http.Handler {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := storage.NewLocalStorage(t.TempDir())
	repos := repository.NewRepositories(db)
	hub := realtime.NewHub()
	printerMgr := printer.NewManager()
	services := service.NewServicesWithConfig(repos, store, printerMgr, hub, service.ServicesConfig{})

	return NewRouterWithOptions(services, hub, opts)
}

func doTokenRequest(t *testing.T, handler http.Handler, method, path, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if token != "" {
		req.Header.Set("X-API-Token", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// With auth off (the documented local-development default), the whole /api
// surface stays reachable: this is what keeps the tokens page usable.
func TestAPITokenHandler_AuthDisabledByDefault(t *testing.T) {
	handler := newTokenTestEnv(t, RouterOptions{})

	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("list with auth off: status = %d, want 200", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens/status", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("status with auth off: status = %d, want 200", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{"name":"off"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create with auth off: status = %d, want 201", rec.Code)
	}
}

// The matrix that matters when auth is on: nothing works without a credential,
// the shared token works everywhere, and a scoped token is refused where its
// scope does not reach.
func TestAPITokenHandler_AuthMatrix(t *testing.T) {
	handler := newTokenTestEnv(t, RouterOptions{ApiToken: "senha-global"})

	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("list without token: status = %d, want 401", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens", "errada", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("list with wrong token: status = %d, want 401", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens", "senha-global", ""); rec.Code != http.StatusOK {
		t.Errorf("list with shared token: status = %d, want 200", rec.Code)
	}

	// A named minted through the accepted shared token.
	rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "senha-global", `{"name":"slicer","scopes":["/api/slicer"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create scoped token: status = %d, want 201", rec.Code)
	}
	var created struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Token == "" {
		t.Fatal("create response must carry the plaintext exactly once")
	}

	// The scoped token reads its own route, and is refused outside it.
	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/slicer/profiles/printers", created.Token, ""); rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Errorf("scoped token on /api/slicer/profiles: got %d, wanted the route itself to answer", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/materials", created.Token, ""); rec.Code != http.StatusForbidden {
		t.Errorf("scoped token outside its scope: status = %d, want 403", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodGet, "/api/materials", created.Token, ""); rec.Code == http.StatusOK {
		t.Error("scoped token was allowed on a route its scope does not cover")
	}
}

func TestAPITokenHandler_CRUD(t *testing.T) {
	handler := newTokenTestEnv(t, RouterOptions{})

	// Empty list serializes as [], not null: the page iterates it.
	rec := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens", "", "")
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Errorf("empty list body = %q, want []", body)
	}

	rec = doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{"name":"Integração Etsy"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want 201", rec.Code)
	}
	var created struct {
		Token    string `json:"token"`
		APIToken struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			Prefix  string   `json:"prefix"`
			Scopes  []string `json:"scopes"`
			Hash    string   `json:"token_hash"`
			Revoked bool     `json:"revoked,omitempty"`
		} `json:"api_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Token == "" {
		t.Error("plaintext missing from create response")
	}
	if created.APIToken.ID == "" {
		t.Error("created token row is missing its id")
	}
	if created.APIToken.Hash != "" {
		t.Error("create response exposed the stored hash")
	}
	if len(created.APIToken.Scopes) != 1 || created.APIToken.Scopes[0] != "*" {
		t.Errorf("scopes = %v, want [*] by default", created.APIToken.Scopes)
	}

	// The plaintext never appears again in any response.
	relisted := doTokenRequest(t, handler, http.MethodGet, "/api/api-tokens", "", "")
	if strings.Contains(relisted.Body.String(), created.Token) {
		t.Error("list response leaked the plaintext")
	}
	var listed []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Prefix string `json:"prefix"`
	}
	if err := json.NewDecoder(relisted.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.APIToken.ID {
		t.Fatalf("list = %+v, want the created token", listed)
	}
	if listed[0].Prefix != created.APIToken.Prefix {
		t.Error("list changed the display prefix")
	}

	if rec := doTokenRequest(t, handler, http.MethodDelete, "/api/api-tokens/"+created.APIToken.ID, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke: status = %d, want 204", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodDelete, "/api/api-tokens/"+created.APIToken.ID, "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("second revoke: status = %d, want 404", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodDelete, "/api/api-tokens/nao-e-uuid", "", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("revoke with bad id: status = %d, want 400", rec.Code)
	}
}

func TestAPITokenHandler_Status(t *testing.T) {
	off := newTokenTestEnv(t, RouterOptions{})
	rec := doTokenRequest(t, off, http.MethodGet, "/api/api-tokens/status", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status: status = %d, want 200", rec.Code)
	}
	var status struct {
		AuthEnabled           bool `json:"auth_enabled"`
		GlobalTokenConfigured bool `json:"global_token_configured"`
		ActiveCount           int  `json:"active_count"`
		RevokedCount          int  `json:"revoked_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.AuthEnabled {
		t.Error("auth_enabled must be false when no API_TOKEN is configured")
	}
	if status.GlobalTokenConfigured {
		t.Error("global_token_configured must be false when no API_TOKEN is configured")
	}

	on := newTokenTestEnv(t, RouterOptions{ApiToken: "senha"})
	rec = doTokenRequest(t, on, http.MethodGet, "/api/api-tokens/status", "senha", "")
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status with auth on: %v", err)
	}
	if !status.AuthEnabled || !status.GlobalTokenConfigured {
		t.Error("status must report auth on when API_TOKEN is set")
	}
}

func TestAPITokenHandler_RejectsBadRequests(t *testing.T) {
	handler := newTokenTestEnv(t, RouterOptions{})

	if rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body: status = %d, want 400", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{"name":""}`); rec.Code != http.StatusBadRequest {
		t.Errorf("blank name: status = %d, want 400", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{"name":"x","scopes":["raiz"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("scope that is not a /api prefix: status = %d, want 400", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{"name":"dup"}`); rec.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d, want 201", rec.Code)
	}
	if rec := doTokenRequest(t, handler, http.MethodPost, "/api/api-tokens", "", `{"name":"dup"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("duplicate name: status = %d, want 400", rec.Code)
	}
}
