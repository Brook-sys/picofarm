package api

import (
	"bufio"
	"context"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Brook-sys/picofarm/internal/service"
	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request_id"
)

// GetRequestID retrieves the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker interface for WebSocket support.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Flush implements http.Flusher interface.
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// SecurityHeaders adds conservative browser hardening headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Cross-Origin-Resource-Policy", "same-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// RequestLogger is middleware that logs HTTP requests with timing and correlation IDs.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate or extract request ID
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()[:8] // Short ID for readability
		}

		// Add request ID to response headers
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to context
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		r = r.WithContext(ctx)

		// Wrap response writer to capture status
		wrapped := wrapResponseWriter(w)

		// Log request start (debug level to reduce noise)
		slog.Debug("request started",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Log request completion
		level := slog.LevelInfo
		if wrapped.status >= 500 {
			level = slog.LevelError
		} else if wrapped.status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(r.Context(), level, "request completed",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration_ms", duration.Milliseconds(),
		)
	})
}

// LogError logs an error with request context.
func LogError(ctx context.Context, msg string, err error, args ...any) {
	allArgs := make([]any, 0, len(args)+4)
	allArgs = append(allArgs, "error", err)
	if requestID := GetRequestID(ctx); requestID != "" {
		allArgs = append(allArgs, "request_id", requestID)
	}
	allArgs = append(allArgs, args...)
	slog.Error(msg, allArgs...)
}

// LogInfo logs an info message with request context.
func LogInfo(ctx context.Context, msg string, args ...any) {
	allArgs := make([]any, 0, len(args)+2)
	if requestID := GetRequestID(ctx); requestID != "" {
		allArgs = append(allArgs, "request_id", requestID)
	}
	allArgs = append(allArgs, args...)
	slog.Info(msg, allArgs...)
}

// LogWarn logs a warning with request context.
func LogWarn(ctx context.Context, msg string, args ...any) {
	allArgs := make([]any, 0, len(args)+2)
	if requestID := GetRequestID(ctx); requestID != "" {
		allArgs = append(allArgs, "request_id", requestID)
	}
	allArgs = append(allArgs, args...)
	slog.Warn(msg, allArgs...)
}

// ApiTokenAuth validates the API token if auth is enabled in router options.
// With auth disabled (local development), every request is allowed: that default
// is documented in docs/SECURITY_MODEL.md and the token page relies on it to
// stay reachable.
//
// When auth is enabled, two credential sources are accepted: the shared token
// from the environment (API_TOKEN), and named tokens minted through
// POST /api/api-tokens. The environment token grants full access; a named token
// is restricted to the route prefixes declared in its scopes.
func ApiTokenAuth(expectedToken string, requireAuth bool, validate TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Auth disabled: nothing to check.
			if expectedToken == "" && (!requireAuth || validate == nil) {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-API-Token")
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			// Constant-time comparison against the shared token: a plain != would
			// leak how many leading bytes matched.
			if expectedToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}

			if validate != nil {
				scopes, ok := validate(r.Context(), token)
				if !ok {
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}
				if !service.ScopeAllows(scopes, r.URL.Path) {
					http.Error(w, "Forbidden: token scope does not cover this route", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

// TokenValidator resolves a presented token to the scopes it grants. It returns
// ok=false when the token is unknown, revoked or empty, so callers cannot tell
// those cases apart.
type TokenValidator func(ctx context.Context, token string) (scopes []string, ok bool)
