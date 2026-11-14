package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

func TestAPITokenRepository_CRUD(t *testing.T) {
	db := openTestDB(t)
	repo := &APITokenRepository{db: db}
	ctx := context.Background()

	token := &model.APIToken{
		Name:      "Integração Etsy",
		Prefix:    "abcd1234",
		TokenHash: "hash-do-token-1",
		Scopes:    []string{"*"},
	}
	if err := repo.Create(ctx, token); err != nil {
		t.Fatalf("create: %v", err)
	}
	if token.ID == uuid.Nil {
		t.Fatal("expected ID assigned")
	}
	if token.CreatedAt.IsZero() {
		t.Error("expected created_at assigned")
	}
	if token.LastUsedAt != nil || token.RevokedAt != nil {
		t.Error("new token must not start with last_used_at/revoked_at set")
	}

	got, err := repo.GetByID(ctx, token.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got == nil {
		t.Fatal("expected token back")
	}
	if len(got.Scopes) != 1 || got.Scopes[0] != "*" {
		t.Errorf("scopes = %v, want [*]", got.Scopes)
	}

	byHash, err := repo.GetByHash(ctx, "hash-do-token-1")
	if err != nil {
		t.Fatalf("get by hash: %v", err)
	}
	if byHash == nil || byHash.ID != token.ID {
		t.Fatalf("get by hash returned %v, want id %s", byHash, token.ID)
	}

	if err := repo.TouchLastUsed(ctx, token.ID, time.Now()); err != nil {
		t.Fatalf("touch last used: %v", err)
	}
	got, _ = repo.GetByID(ctx, token.ID)
	if got.LastUsedAt == nil {
		t.Error("expected last_used_at after touch")
	}

	if err := repo.Revoke(ctx, token.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got, _ = repo.GetByID(ctx, token.ID)
	if got.RevokedAt == nil {
		t.Error("expected revoked_at after revoke")
	}
	// Revoking twice must not resurrect or double-write the row.
	if err := repo.Revoke(ctx, token.ID); err == nil {
		t.Error("expected second revoke to report no rows affected")
	}
}

func TestAPITokenRepository_ListOrderAndMissing(t *testing.T) {
	db := openTestDB(t)
	repo := &APITokenRepository{db: db}
	ctx := context.Background()

	first := &model.APIToken{Name: "primeira", Prefix: "p1", TokenHash: "h1", Scopes: []string{"*"}}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	// Newest first: force a later created_at so the ordering is deterministic
	// even when both rows are created inside the same clock tick.
	time.Sleep(2 * time.Millisecond)
	second := &model.APIToken{Name: "segunda", Prefix: "p2", TokenHash: "h2", Scopes: []string{"*"}}
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("create second: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].ID != second.ID {
		t.Errorf("expected newest first, got %s", list[0].Name)
	}

	missing, err := repo.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("get missing returned error: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for a missing id")
	}

	if err := repo.Revoke(ctx, uuid.New()); err == nil {
		t.Error("expected revoking a missing id to report no rows affected")
	}
}

// A name may only be reused after the active row is revoked: that is what lets
// the page recreate a token that was retired.
func TestAPITokenRepository_ActiveNameIsUnique(t *testing.T) {
	db := openTestDB(t)
	repo := &APITokenRepository{db: db}
	ctx := context.Background()

	first := &model.APIToken{Name: "automação", Prefix: "p1", TokenHash: "h1", Scopes: []string{"*"}}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	second := &model.APIToken{Name: "automação", Prefix: "p2", TokenHash: "h2", Scopes: []string{"*"}}
	if err := repo.Create(ctx, second); err == nil {
		t.Fatal("expected duplicate active name to be rejected")
	}

	if err := repo.Revoke(ctx, first.ID); err != nil {
		t.Fatalf("revoke first: %v", err)
	}
	third := &model.APIToken{Name: "automação", Prefix: "p3", TokenHash: "h3", Scopes: []string{"*"}}
	if err := repo.Create(ctx, third); err != nil {
		t.Fatalf("recreate name after revoke: %v", err)
	}
}
