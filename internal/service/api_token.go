package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

// ScopeFull grants access to every route. Any other scope is a route prefix
// under /api (for example "/api/slicer"), matched exactly or as a path segment,
// so "/api/slicer" does not open "/api/slicer-something-else".
const ScopeFull = "*"

const (
	// tokenBytes is the entropy of a generated token, in bytes. 32 bytes rendered
	// as hex yields 64 characters, the same shape used by camera access tokens.
	tokenBytes = 32
	// maxTokenNameLen bounds the display name so lists and logs stay readable.
	maxTokenNameLen = 64
	// tokenCacheTTL is how long a validation result is reused. SQLite serializes
	// writes on a single connection, so re-querying on every request would put
	// authentication in the same queue as user writes.
	tokenCacheTTL = 30 * time.Second
	// tokenNegativeCacheTTL is shorter: an unknown token must not be cached for
	// as long as a valid one, or a revoked-then-recreated name would lag.
	tokenNegativeCacheTTL = 10 * time.Second
	// tokenTouchMinGap throttles last_used_at writes per token. Without it, every
	// authenticated request would be a write transaction.
	tokenTouchMinGap = time.Minute
)

var (
	// ErrTokenNotFound is returned for an unknown, expired or revoked token. It
	// deliberately does not distinguish the cases: the caller only learns that
	// the credential is not acceptable.
	ErrTokenNotFound = errors.New("api token not found")
	// ErrInvalidTokenName means the name is empty, too long or uses characters
	// that are unsafe to display.
	ErrInvalidTokenName = errors.New("invalid api token name")
	// ErrInvalidScope means a scope is neither "*" nor a /api route prefix.
	ErrInvalidScope = errors.New("invalid api token scope")
)

// APITokenService mints, lists, revokes and validates named API tokens.
//
// Only a SHA-256 hash of each token is stored, so the plaintext exists twice:
// once in the Generate response, and once in the client that saved it. The
// service caches validation results in memory and invalidates the whole cache on
// every create/revoke, which keeps revocation immediate without paying a SQLite
// read per request.
type APITokenService struct {
	repo *repository.APITokenRepository

	mu    sync.RWMutex
	cache map[string]cachedToken  // key: SHA-256 hex of the presented token
	touch map[uuid.UUID]time.Time // last last_used_at write per token
}

type cachedToken struct {
	id       uuid.UUID
	scopes   []string
	revoked  bool
	loadedAt time.Time
}

// NewAPITokenService builds the service over a token repository.
func NewAPITokenService(repo *repository.APITokenRepository) *APITokenService {
	return &APITokenService{
		repo:  repo,
		cache: make(map[string]cachedToken),
		touch: make(map[uuid.UUID]time.Time),
	}
}

// Generate mints a new named token and returns its plaintext exactly once: the
// stored row keeps only the hash, so there is no second chance to read it.
func (s *APITokenService) Generate(ctx context.Context, name string, scopes []string) (string, *model.APIToken, error) {
	normalizedName, normalizedScopes, err := normalizeAPIToken(name, scopes)
	if err != nil {
		return "", nil, err
	}

	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate token entropy: %w", err)
	}
	plaintext := hex.EncodeToString(buf)

	token := &model.APIToken{
		Name:      normalizedName,
		Prefix:    plaintext[:8],
		TokenHash: HashToken(plaintext),
		Scopes:    normalizedScopes,
	}
	if err := s.repo.Create(ctx, token); err != nil {
		if isUniqueConstraintError(err) {
			return "", nil, fmt.Errorf("api token name %q is already in use", normalizedName)
		}
		return "", nil, fmt.Errorf("create api token: %w", err)
	}

	// A brand new hash can never be cached, but dropping everything also clears
	// negative entries, so the new token validates immediately.
	s.invalidateAll()
	return plaintext, token, nil
}

// List returns every token, newest first, with hash values already blanked by
// the repository contract (they are never part of the API response anyway).
func (s *APITokenService) List(ctx context.Context) ([]model.APIToken, error) {
	tokens, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	return tokens, nil
}

// Revoke retires a token. Rows are kept so the UI can show what was retired and
// so a replayed hash still resolves to a rejecting row.
func (s *APITokenService) Revoke(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Revoke(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("revoke api token: %w", err)
	}
	s.invalidateID(id)
	return nil
}

// Counts reports how many tokens are active and revoked.
func (s *APITokenService) Counts(ctx context.Context) (active int, revoked int, err error) {
	tokens, err := s.repo.List(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("count api tokens: %w", err)
	}
	for _, t := range tokens {
		if t.RevokedAt != nil {
			revoked++
			continue
		}
		active++
	}
	return active, revoked, nil
}

// Validate resolves a presented token to the scopes it grants. It returns
// ErrTokenNotFound for anything not currently acceptable, without saying which
// of unknown/revoked applied.
func (s *APITokenService) Validate(ctx context.Context, token string) (*model.APIToken, error) {
	if token == "" {
		return nil, ErrTokenNotFound
	}
	hash := HashToken(token)

	if cached, ok := s.lookupCache(hash); ok {
		if cached.revoked {
			return nil, ErrTokenNotFound
		}
		s.touchUsage(ctx, cached.id)
		return &model.APIToken{ID: cached.id, Scopes: cached.scopes}, nil
	}

	stored, err := s.repo.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("look up api token: %w", err)
	}
	if stored == nil {
		s.storeNegative(hash)
		return nil, ErrTokenNotFound
	}

	entry := cachedToken{id: stored.ID, scopes: stored.Scopes, revoked: stored.RevokedAt != nil}
	s.storeCache(hash, entry)
	if entry.revoked {
		return nil, ErrTokenNotFound
	}
	s.touchUsage(ctx, stored.ID)
	return &model.APIToken{ID: stored.ID, Scopes: stored.Scopes}, nil
}

// HashToken returns the SHA-256 hex of a token plaintext. Exported so the auth
// middleware and tests hash the same way the service stores.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ScopeAllows reports whether a scope list grants access to a route path.
func ScopeAllows(scopes []string, path string) bool {
	for _, scope := range scopes {
		if scope == ScopeFull {
			return true
		}
		if path == scope || strings.HasPrefix(path, scope+"/") {
			return true
		}
	}
	return false
}

func (s *APITokenService) lookupCache(hash string) (cachedToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.cache[hash]
	if !ok {
		return cachedToken{}, false
	}
	ttl := tokenCacheTTL
	if entry.revoked {
		// A revoked entry is only cached while it is the truthful answer; once it
		// expires we re-read so a recreated token with the same hash is honored.
		ttl = tokenNegativeCacheTTL
	}
	if time.Since(entry.loadedAt) > ttl {
		return cachedToken{}, false
	}
	return entry, true
}

func (s *APITokenService) storeCache(hash string, entry cachedToken) {
	entry.loadedAt = time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = make(map[string]cachedToken)
	}
	s.cache[hash] = entry
}

func (s *APITokenService) storeNegative(hash string) {
	s.storeCache(hash, cachedToken{revoked: true})
}

func (s *APITokenService) invalidateAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]cachedToken)
	// The touch map is pruned here too. It only ever gains entries (one per
	// validated token), so without this it would grow for the life of the
	// process on an instance that mints and retires tokens continuously.
	s.pruneTouchLocked(time.Now().Add(-2 * tokenTouchMinGap))
}

// invalidateID drops the cached entries for one token. It takes the write lock
// for the whole operation: reading the map under RLock and then deleting under
// Lock would race with an invalidateAll that swaps in a fresh map, leaving the
// revoked entry behind.
func (s *APITokenService) invalidateID(id uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for hash, entry := range s.cache {
		if entry.id == id {
			delete(s.cache, hash)
		}
	}
	delete(s.touch, id)
}

// pruneTouchLocked drops throttle timestamps older than the cutoff, so a token
// that goes quiet stops occupying the map. Callers must hold s.mu.
func (s *APITokenService) pruneTouchLocked(cutoff time.Time) {
	for id, at := range s.touch {
		if at.Before(cutoff) {
			delete(s.touch, id)
		}
	}
}

// touchUsage records last_used_at at most once per token per tokenTouchMinGap.
// The write happens on a detached context so a cancelled request cannot take the
// bookkeeping down with it.
func (s *APITokenService) touchUsage(ctx context.Context, id uuid.UUID) {
	s.mu.Lock()
	if last, ok := s.touch[id]; ok && time.Since(last) < tokenTouchMinGap {
		s.mu.Unlock()
		return
	}
	s.touch[id] = time.Now()
	s.mu.Unlock()

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	go func() {
		defer cancel()
		if err := s.repo.TouchLastUsed(writeCtx, id, time.Now()); err != nil {
			slog.Warn("failed to record api token usage", "token_id", id, "error", err)
		}
	}()
}

// isUniqueConstraintError detects the UNIQUE index rejection for an active
// token name. glebarez/go-sqlite surfaces it as an error string rather than a
// typed sentinel, and there is no errno cross-driver alias to rely on.
func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

// normalizeAPIToken trims and validates a name and a scope list.
func normalizeAPIToken(name string, scopes []string) (string, []string, error) {
	normalizedName := strings.TrimSpace(name)
	if normalizedName == "" || len(normalizedName) > maxTokenNameLen {
		return "", nil, ErrInvalidTokenName
	}
	for _, r := range normalizedName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			continue
		}
		switch r {
		case '-', '_', '.', '(', ')', '/':
			continue
		}
		return "", nil, ErrInvalidTokenName
	}

	if len(scopes) == 0 {
		scopes = []string{ScopeFull}
	}
	seen := make(map[string]struct{}, len(scopes))
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if scope != ScopeFull {
			scope = strings.TrimRight(scope, "/")
			if !strings.HasPrefix(scope, "/api/") {
				return "", nil, fmt.Errorf("%w: %q must be \"%s\" or a /api path prefix", ErrInvalidScope, scope, ScopeFull)
			}
		}
		if _, dup := seen[scope]; dup {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		normalized = []string{ScopeFull}
	}
	return normalizedName, normalized, nil
}
