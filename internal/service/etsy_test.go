package service

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/etsy"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

func openEtsyTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestEtsyService_Configure_valid(t *testing.T) {
	db := openEtsyTestDB(t)
	// Use exported field via repository.NewRepositories to get properly wired repos
	repos := repository.NewRepositories(db)
	etsyRepo := repos.Etsy
	settingsRepo := repos.Settings

	settingsSvc := &SettingsService{repo: settingsRepo}
	svc := NewEtsyService(etsyRepo, "", "", settingsSvc)

	if svc.IsConfigured() {
		t.Fatal("expected service to not be configured initially")
	}

	err := svc.Configure(context.Background(), "test-client-id", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}

	if !svc.IsConfigured() {
		t.Fatal("expected service to be configured after Configure()")
	}

	// Verify settings were persisted
	setting, err := settingsSvc.Get(context.Background(), "etsy_client_id")
	if err != nil {
		t.Fatalf("Get etsy_client_id: %v", err)
	}
	if setting == nil || setting.Value != "test-client-id" {
		t.Fatalf("expected etsy_client_id to be 'test-client-id', got %v", setting)
	}

	uriSetting, err := settingsSvc.Get(context.Background(), "etsy_redirect_uri")
	if err != nil {
		t.Fatalf("Get etsy_redirect_uri: %v", err)
	}
	if uriSetting == nil || uriSetting.Value != "http://localhost:8080/callback" {
		t.Fatalf("expected etsy_redirect_uri to be 'http://localhost:8080/callback', got %v", uriSetting)
	}
}

func TestEtsyService_Configure_emptyClientID(t *testing.T) {
	db := openEtsyTestDB(t)
	repos := repository.NewRepositories(db)

	settingsSvc := &SettingsService{repo: repos.Settings}
	svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

	err := svc.Configure(context.Background(), "", "http://localhost:8080/callback")
	if err == nil {
		t.Fatal("expected error for empty client_id")
	}

	if svc.IsConfigured() {
		t.Fatal("expected service to remain unconfigured after empty client_id")
	}
}

func TestEtsyService_Configure_defaultRedirectURI(t *testing.T) {
	db := openEtsyTestDB(t)
	repos := repository.NewRepositories(db)

	settingsSvc := &SettingsService{repo: repos.Settings}
	svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

	err := svc.Configure(context.Background(), "test-client-id", "")
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}

	if !svc.IsConfigured() {
		t.Fatal("expected service to be configured")
	}

	// Verify default redirect URI was used
	uriSetting, err := settingsSvc.Get(context.Background(), "etsy_redirect_uri")
	if err != nil {
		t.Fatalf("Get etsy_redirect_uri: %v", err)
	}
	if uriSetting == nil || uriSetting.Value != etsy.DefaultRedirectURI {
		t.Fatalf("expected default redirect URI on port 8084, got %v", uriSetting)
	}
}

// Configure must never persist a redirect URI that cannot produce a valid
// OAuth request. Otherwise the failure only surfaces at Etsy, with a message
// the operator never sees.
func TestEtsyService_Configure_invalidRedirectURI(t *testing.T) {
	db := openEtsyTestDB(t)
	repos := repository.NewRepositories(db)
	settingsSvc := &SettingsService{repo: repos.Settings}
	svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

	cases := []struct {
		name string
		uri  string
	}{
		{"not a url", "not-a-url"},
		{"ftp scheme", "ftp://example.com/callback"},
		{"no host", "http:///callback"},
		{"whitespace padding", " http://localhost:8084/api/integrations/etsy/callback "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.Configure(context.Background(), "test-client-id", tc.uri)
			if !errors.Is(err, ErrInvalidRedirectURI) {
				t.Fatalf("expected ErrInvalidRedirectURI for %q, got %v", tc.uri, err)
			}
			// A rejected configuration must not leave the service half-configured.
			if svc.IsConfigured() {
				t.Fatal("service must not be configured after a rejected redirect URI")
			}
			setting, err := settingsSvc.Get(context.Background(), EtsyRedirectURIKey)
			if err != nil {
				t.Fatalf("Get %s: %v", EtsyRedirectURIKey, err)
			}
			if setting != nil {
				t.Fatalf("rejected redirect URI was persisted: %v", setting.Value)
			}
		})
	}
}

// Configure is the only way out of the "configured but not connected" dead end:
// a keystring or redirect URI that is wrong must be replaceable without hand
// editing the database or deleting the whole integration.
func TestEtsyService_Configure_replacesPreviousValues(t *testing.T) {
	db := openEtsyTestDB(t)
	repos := repository.NewRepositories(db)
	settingsSvc := &SettingsService{repo: repos.Settings}
	svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

	if err := svc.Configure(context.Background(), "old-key", etsy.DefaultRedirectURI); err != nil {
		t.Fatalf("initial Configure: %v", err)
	}
	if err := svc.Configure(context.Background(), "new-key", "http://localhost:9099/api/integrations/etsy/callback"); err != nil {
		t.Fatalf("second Configure: %v", err)
	}

	if got := svc.RedirectURI(); got != "http://localhost:9099/api/integrations/etsy/callback" {
		t.Fatalf("expected in-memory client to use the new redirect URI, got %q", got)
	}
	setting, err := settingsSvc.Get(context.Background(), "etsy_redirect_uri")
	if err != nil {
		t.Fatalf("Get etsy_redirect_uri: %v", err)
	}
	if setting == nil || setting.Value != "http://localhost:9099/api/integrations/etsy/callback" {
		t.Fatalf("expected persisted redirect URI to be replaced, got %v", setting)
	}
	keySetting, err := settingsSvc.Get(context.Background(), "etsy_client_id")
	if err != nil {
		t.Fatalf("Get etsy_client_id: %v", err)
	}
	if keySetting == nil || keySetting.Value != "new-key" {
		t.Fatalf("expected client ID to be replaced, got %v", keySetting)
	}
}

// Older builds persisted port 8080 as the default redirect URI, which is not
// the port this app listens on. The authorization then failed at Etsy with
// "The requested redirect URL is not permitted." and nothing in the UI could
// correct it. The repair rewrites just that value and touches nothing else.
func TestEtsyService_RepairLegacyRedirectURI(t *testing.T) {
	legacy := etsy.LegacyRedirectURI
	fixed := etsy.DefaultRedirectURI

	t.Run("rewrites only the legacy default", func(t *testing.T) {
		db := openEtsyTestDB(t)
		repos := repository.NewRepositories(db)
		settingsSvc := &SettingsService{repo: repos.Settings}
		svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)
		if err := settingsSvc.Set(context.Background(), EtsyRedirectURIKey, legacy); err != nil {
			t.Fatalf("Set: %v", err)
		}

		if err := svc.RepairLegacyRedirectURI(context.Background()); err != nil {
			t.Fatalf("RepairLegacyRedirectURI: %v", err)
		}
		setting, _ := settingsSvc.Get(context.Background(), EtsyRedirectURIKey)
		if setting == nil || setting.Value != fixed {
			t.Fatalf("expected legacy URI to be rewritten to %q, got %v", fixed, setting)
		}
	})

	t.Run("leaves an operator choice alone", func(t *testing.T) {
		db := openEtsyTestDB(t)
		repos := repository.NewRepositories(db)
		settingsSvc := &SettingsService{repo: repos.Settings}
		svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)
		custom := "http://192.0.2.10:8084/api/integrations/etsy/callback"
		if err := settingsSvc.Set(context.Background(), EtsyRedirectURIKey, custom); err != nil {
			t.Fatalf("Set: %v", err)
		}

		if err := svc.RepairLegacyRedirectURI(context.Background()); err != nil {
			t.Fatalf("RepairLegacyRedirectURI: %v", err)
		}
		setting, _ := settingsSvc.Get(context.Background(), EtsyRedirectURIKey)
		if setting == nil || setting.Value != custom {
			t.Fatalf("expected custom URI to be preserved, got %v", setting)
		}
	})

	t.Run("no setting and empty setting are no-ops", func(t *testing.T) {
		db := openEtsyTestDB(t)
		repos := repository.NewRepositories(db)
		settingsSvc := &SettingsService{repo: repos.Settings}
		svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

		if err := svc.RepairLegacyRedirectURI(context.Background()); err != nil {
			t.Fatalf("RepairLegacyRedirectURI with no setting: %v", err)
		}
		if err := settingsSvc.Set(context.Background(), EtsyRedirectURIKey, ""); err != nil {
			t.Fatalf("Set empty: %v", err)
		}
		if err := svc.RepairLegacyRedirectURI(context.Background()); err != nil {
			t.Fatalf("RepairLegacyRedirectURI with empty setting: %v", err)
		}
		setting, _ := settingsSvc.Get(context.Background(), EtsyRedirectURIKey)
		if setting == nil || setting.Value != "" {
			t.Fatalf("expected empty value to be left for the startup default, got %v", setting)
		}
	})
}

// The OAuth state must record the redirect URI that produced the authorization:
// Etsy requires the identical value in the token exchange, so a reconfiguration
// mid-flow can be detected instead of surfacing as an opaque Etsy error.
func TestEtsyService_StartOAuth_recordsRedirectURIInState(t *testing.T) {
	db := openEtsyTestDB(t)
	repos := repository.NewRepositories(db)
	settingsSvc := &SettingsService{repo: repos.Settings}
	svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

	redirect := "http://localhost:9099/api/integrations/etsy/callback"
	if err := svc.Configure(context.Background(), "test-client-id", redirect); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	authURL, err := svc.StartOAuth(context.Background())
	if err != nil {
		t.Fatalf("StartOAuth: %v", err)
	}
	if !strings.Contains(authURL, url.QueryEscape(redirect)) {
		t.Fatalf("expected authorization URL to carry the configured redirect URI, got %q", authURL)
	}

	stateParam, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse auth URL: %v", err)
	}
	stored, err := repos.Etsy.GetOAuthState(context.Background(), stateParam.Query().Get("state"))
	if err != nil {
		t.Fatalf("GetOAuthState: %v", err)
	}
	if stored == nil {
		t.Fatal("expected OAuth state to be persisted")
	}
	if stored.RedirectURI != redirect {
		t.Fatalf("expected persisted state to record %q, got %q", redirect, stored.RedirectURI)
	}
}

// A callback whose recorded redirect URI no longer matches the current client
// is a configuration change mid-flow: fail fast with both values named, and
// consume the state so a retry starts from a clean slate.
func TestEtsyService_HandleCallback_redirectURIMismatch(t *testing.T) {
	db := openEtsyTestDB(t)
	repos := repository.NewRepositories(db)
	settingsSvc := &SettingsService{repo: repos.Settings}
	svc := NewEtsyService(repos.Etsy, "", "", settingsSvc)

	current := etsy.DefaultRedirectURI
	if err := svc.Configure(context.Background(), "test-client-id", current); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	stale := "http://localhost:9099/api/integrations/etsy/callback"
	if err := repos.Etsy.SaveOAuthState(context.Background(), &model.EtsyOAuthState{
		State:        "stale-state",
		CodeVerifier: "verifier",
		RedirectURI:  stale,
	}); err != nil {
		t.Fatalf("SaveOAuthState: %v", err)
	}

	_, err := svc.HandleCallback(context.Background(), "stale-state", "dummy-code")
	if err == nil {
		t.Fatal("expected error for a redirect URI mismatch")
	}
	if !strings.Contains(err.Error(), "redirect URI changed") {
		t.Fatalf("expected the error to explain the redirect URI change, got %v", err)
	}
	if !strings.Contains(err.Error(), stale) || !strings.Contains(err.Error(), current) {
		t.Fatalf("expected both redirect URIs in the error, got %v", err)
	}

	leftover, _ := repos.Etsy.GetOAuthState(context.Background(), "stale-state")
	if leftover != nil {
		t.Fatal("expected the mismatched state to be consumed")
	}
}

func TestEtsyService_Configure_noSettingsService(t *testing.T) {
	svc := NewEtsyService(nil, "", "", nil)

	err := svc.Configure(context.Background(), "test-client-id", "http://localhost:8080/callback")
	if err != nil {
		t.Fatalf("Configure without settingsSvc returned error: %v", err)
	}

	if !svc.IsConfigured() {
		t.Fatal("expected service to be configured even without settingsSvc")
	}
}
