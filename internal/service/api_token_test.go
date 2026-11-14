package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

// openTokenTestService builds the token service over a real in-memory schema,
// so these tests exercise the same SQL the production path runs.
func openTokenTestService(t *testing.T) *APITokenService {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewAPITokenService(repository.NewAPITokenRepository(db))
}

func TestAPITokenService_GenerateReturnsPlaintextOnce(t *testing.T) {
	svc := openTokenTestService(t)
	ctx := context.Background()

	plaintext, token, err := svc.Generate(ctx, "Integração Etsy", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(plaintext) != 64 {
		t.Errorf("plaintext length = %d, want 64 hex chars", len(plaintext))
	}
	if token.Prefix != plaintext[:8] {
		t.Errorf("prefix = %q, want first 8 chars of plaintext", token.Prefix)
	}
	// The stored row must not be reversible into the plaintext.
	if token.TokenHash == plaintext {
		t.Fatal("token row stores the plaintext as its own hash")
	}
	if token.TokenHash != HashToken(plaintext) {
		t.Error("stored hash does not match SHA-256 of the plaintext")
	}
	if len(token.Scopes) != 1 || token.Scopes[0] != ScopeFull {
		t.Errorf("scopes = %v, want [*] by default", token.Scopes)
	}

	// No read path returns the plaintext: List must expose the hash-free row.
	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	if list[0].TokenHash == plaintext || list[0].Prefix == plaintext {
		t.Error("list response leaked the full plaintext")
	}
}

func TestAPITokenService_Validate(t *testing.T) {
	svc := openTokenTestService(t)
	ctx := context.Background()

	plaintext, generated, err := svc.Generate(ctx, "slicer", []string{"/api/slicer"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	resolved, err := svc.Validate(ctx, plaintext)
	if err != nil {
		t.Fatalf("validate valid token: %v", err)
	}
	if resolved == nil || resolved.ID != generated.ID {
		t.Fatalf("validate resolved %v, want id %s", resolved, generated.ID)
	}
	if !ScopeAllows(resolved.Scopes, "/api/slicer/slice") {
		t.Errorf("scope should cover /api/slicer/slice, got %v", resolved.Scopes)
	}

	if _, err := svc.Validate(ctx, ""); err == nil {
		t.Error("empty token must not validate")
	}
	if _, err := svc.Validate(ctx, "deadbeef"); err == nil {
		t.Error("unknown token must not validate")
	}

	if err := svc.Revoke(ctx, generated.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// Revocation must be immediate: the in-memory cache is invalidated, so the
	// very next request fails even though the row still exists.
	if _, err := svc.Validate(ctx, plaintext); err == nil {
		t.Error("revoked token must stop validating immediately")
	}
}

func TestScopeAllows(t *testing.T) {
	cases := []struct {
		name   string
		scopes []string
		path   string
		want   bool
	}{
		{"full wildcard allows anything", []string{"*"}, "/api/printers", true},
		{"exact prefix", []string{"/api/slicer"}, "/api/slicer", true},
		{"sub path", []string{"/api/slicer"}, "/api/slicer/slice", true},
		{"different route rejected", []string{"/api/slicer"}, "/api/printers", false},
		{"sibling prefix not matched", []string{"/api/slicer"}, "/api/slicer-extra", false},
		{"multiple scopes, second matches", []string{"/api/printers", "/api/materials"}, "/api/materials/1", true},
		{"empty scope list allows nothing", nil, "/api/printers", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScopeAllows(tc.scopes, tc.path); got != tc.want {
				t.Errorf("ScopeAllows(%v, %q) = %v, want %v", tc.scopes, tc.path, got, tc.want)
			}
		})
	}
}

func TestAPITokenService_NameAndScopeValidation(t *testing.T) {
	svc := openTokenTestService(t)
	ctx := context.Background()

	if _, _, err := svc.Generate(ctx, "   ", nil); err == nil {
		t.Error("blank name must be rejected")
	}
	if _, _, err := svc.Generate(ctx, strings.Repeat("a", 65), nil); err == nil {
		t.Error("over-long name must be rejected")
	}
	if _, _, err := svc.Generate(ctx, "inje<script>", nil); err == nil {
		t.Error("unsafe characters must be rejected")
	}
	if _, _, err := svc.Generate(ctx, "escopa-ruim", []string{"slicer"}); err == nil {
		t.Error("scope that is not a /api prefix must be rejected")
	}
	// The rejected attempt above must not reserve the name either.
	if _, _, err := svc.Generate(ctx, "slicer", []string{"*"}); err != nil {
		t.Fatalf("first valid generate: %v", err)
	}

	// A trailing slash is normalized, not rejected: operators type it.
	_, trailing, err := svc.Generate(ctx, "normalizado", []string{"/api/slicer/"})
	if err != nil {
		t.Fatalf("generate with trailing slash: %v", err)
	}
	if len(trailing.Scopes) != 1 || trailing.Scopes[0] != "/api/slicer" {
		t.Errorf("scopes = %v, want [/api/slicer]", trailing.Scopes)
	}

	// Duplicated and empty entries collapse into the distinct list.
	_, dedup, err := svc.Generate(ctx, "dedup", []string{"", "/api/x", "/api/x/"})
	if err != nil {
		t.Fatalf("generate with duplicated scopes: %v", err)
	}
	if len(dedup.Scopes) != 1 || dedup.Scopes[0] != "/api/x" {
		t.Errorf("scopes = %v, want [/api/x]", dedup.Scopes)
	}

	if _, _, err := svc.Generate(ctx, "slicer", nil); err == nil {
		t.Error("duplicate active name must be rejected")
	}
}

func TestAPITokenService_CountsAndRevocationErrors(t *testing.T) {
	svc := openTokenTestService(t)
	ctx := context.Background()

	active, revoked, err := svc.Counts(ctx)
	if err != nil {
		t.Fatalf("counts on empty store: %v", err)
	}
	if active != 0 || revoked != 0 {
		t.Errorf("counts = (%d, %d), want (0, 0)", active, revoked)
	}

	_, first, err := svc.Generate(ctx, "primeiro", nil)
	if err != nil {
		t.Fatalf("generate first: %v", err)
	}
	if _, _, err := svc.Generate(ctx, "segundo", nil); err != nil {
		t.Fatalf("generate second: %v", err)
	}
	if err := svc.Revoke(ctx, first.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	active, revoked, err = svc.Counts(ctx)
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	if active != 1 || revoked != 1 {
		t.Errorf("counts = (%d, %d), want (1, 1)", active, revoked)
	}

	// Revoking twice is not a new state change.
	if err := svc.Revoke(ctx, first.ID); err == nil {
		t.Error("second revoke must fail")
	}
	// Revoking an unknown id must fail, not silently succeed.
	if err := svc.Revoke(ctx, uuid.New()); err == nil {
		t.Error("unknown revoke must fail")
	}
	// A revoked name is reusable, which is how the page recreates a token.
	reused, _, err := svc.Generate(ctx, "primeiro", nil)
	if err != nil {
		t.Fatalf("reuse revoked name: %v", err)
	}
	if reused == "" {
		t.Error("expected plaintext back on reuse")
	}
}

// Validate caches results; this proves the cache cannot outlive a revocation.
func TestAPITokenService_CacheDoesNotOutliveRevocation(t *testing.T) {
	svc := openTokenTestService(t)
	ctx := context.Background()

	plaintext, generated, err := svc.Generate(ctx, "cache", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := svc.Validate(ctx, plaintext); err != nil {
		t.Fatalf("first validate: %v", err)
	}
	if err := svc.Revoke(ctx, generated.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := svc.Validate(ctx, plaintext)
		if err != nil {
			break // revoked: good
		}
		if time.Now().After(deadline) {
			t.Fatal("cache kept a revoked token valid for over 2s")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A touch of last_used_at must not fail the request, even though the write is
// async: the shared SQLite connection serializes writes.
func TestAPITokenService_ValidateTouchesLastUsed(t *testing.T) {
	svc := openTokenTestService(t)
	ctx := context.Background()

	plaintext, _, err := svc.Generate(ctx, "uso", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := svc.Validate(ctx, plaintext); err != nil {
		t.Fatalf("validate: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		list, err := svc.List(ctx)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(list) == 1 && list[0].LastUsedAt != nil {
			return // recorded
		}
		if time.Now().After(deadline) {
			t.Fatal("last_used_at was never recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The model must never serialize the hash, whatever field order it is built with.
func TestAPITokenModelNeverSerializesHash(t *testing.T) {
	token := model.APIToken{Name: "x", TokenHash: "segredo", Prefix: "abcd1234"}
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "segredo") {
		t.Error("json output contains the token hash")
	}
	if !strings.Contains(string(data), "abcd1234") {
		t.Error("json output should still carry the display prefix")
	}
}
