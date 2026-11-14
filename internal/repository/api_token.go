package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// APITokenRepository stores named API tokens. Only the SHA-256 hash of a token
// is persisted: a leaked database cannot be replayed as a live credential.
type APITokenRepository struct {
	db *sql.DB
}

// NewAPITokenRepository builds the repository over an open database.
func NewAPITokenRepository(db *sql.DB) *APITokenRepository {
	return &APITokenRepository{db: db}
}

// Create inserts a new token. ID and timestamps are assigned here when empty.
func (r *APITokenRepository) Create(ctx context.Context, t *model.APIToken) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	t.CreatedAt = time.Now()
	if t.LastUsedAt != nil {
		t.LastUsedAt = nil
	}
	if t.RevokedAt != nil {
		t.RevokedAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_tokens (id, name, prefix, token_hash, scopes_json, created_at, last_used_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Name, t.Prefix, t.TokenHash, marshalStringArray(t.Scopes), t.CreatedAt, t.LastUsedAt, t.RevokedAt)
	return err
}

// List returns every token, newest first, including revoked ones. The revoked
// rows are what makes "this name was already used" visible in the UI.
func (r *APITokenRepository) List(ctx context.Context) ([]model.APIToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, prefix, token_hash, scopes_json, created_at, last_used_at, revoked_at
		FROM api_tokens ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.APIToken
	for rows.Next() {
		var t model.APIToken
		var scopes string
		if err := scanRow(rows, &t.ID, &t.Name, &t.Prefix, &t.TokenHash, &scopes, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		t.Scopes = unmarshalStringArray([]byte(scopes))
		list = append(list, t)
	}
	return list, rows.Err()
}

// GetByID returns a token by ID, or (nil, nil) when it does not exist.
func (r *APITokenRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.APIToken, error) {
	return r.getOne(ctx, `WHERE id = ?`, id)
}

// GetByHash returns the token matching a SHA-256 hash. Callers are expected to
// hash the presented token first, so the stored value is never compared in
// plaintext.
func (r *APITokenRepository) GetByHash(ctx context.Context, tokenHash string) (*model.APIToken, error) {
	return r.getOne(ctx, `WHERE token_hash = ?`, tokenHash)
}

func (r *APITokenRepository) getOne(ctx context.Context, where string, arg any) (*model.APIToken, error) {
	var t model.APIToken
	var scopes string
	err := scanRow(r.db.QueryRowContext(ctx, `
		SELECT id, name, prefix, token_hash, scopes_json, created_at, last_used_at, revoked_at
		FROM api_tokens `+where+`
	`, arg), &t.ID, &t.Name, &t.Prefix, &t.TokenHash, &scopes, &t.CreatedAt, &t.LastUsedAt, &t.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Scopes = unmarshalStringArray([]byte(scopes))
	return &t, nil
}

// TouchLastUsed records the last successful authentication with this token.
func (r *APITokenRepository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, at, id)
	return err
}

// Revoke marks a token as revoked. Revoking is not deleting: the row stays so
// the UI can show what was retired, and a hash lookup keeps finding the row and
// rejecting it. Returns sql.ErrNoRows when the token does not exist.
func (r *APITokenRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, time.Now(), id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
