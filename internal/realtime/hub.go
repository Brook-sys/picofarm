package realtime

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Event represents a real-time event to broadcast.
// This is an alias for model.BroadcastEvent for backwards compatibility.
type Event = model.BroadcastEvent

// Client represents a connected WebSocket client.
type Client struct {
	conn *websocket.Conn
	send chan Event
}

// Hub manages WebSocket connections and broadcasts.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan Event
	register   chan *Client
	unregister chan *Client
	stop       chan struct{}
	stopped    bool

	// AuthToken is the shared API_TOKEN. When empty, the handshake does not ask
	// for a token (local development). When set, /ws is closed unless the client
	// presents it.
	AuthToken string
	// RequireAuth mirrors the router's REQUIRE_API_TOKEN: enforce a token even
	// without a shared one, using named tokens only.
	RequireAuth bool
	// AllowedOrigins lists accepted origin patterns (nhooyr syntax, e.g.
	// "https://picofarm.example.com" or "http://localhost:*"). Empty means the
	// loopback defaults, matching the CORS defaults in the router.
	AllowedOrigins []string
	// ValidateToken resolves a named token to its scopes. Nil in minimal setups.
	ValidateToken func(ctx context.Context, token string) ([]string, bool)
	// SubjectAllows is the scope check applied to a named token on /ws. Injected
	// to avoid a service import cycle; nil means "any valid token may connect".
	SubjectAllows func(scopes []string, subject string) bool
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stop:       make(chan struct{}),
	}
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	for {
		select {
		case <-h.stop:
			// Gracefully close all client connections
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				client.conn.Close(websocket.StatusGoingAway, "server shutting down") //nolint:errcheck // best-effort shutdown
				delete(h.clients, client)
			}
			h.stopped = true
			h.mu.Unlock()
			slog.Info("websocket hub stopped")
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Info("websocket client connected", "total_clients", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			slog.Info("websocket client disconnected", "total_clients", len(h.clients))

		case event := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- event:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(event Event) {
	select {
	case h.broadcast <- event:
	default:
		slog.Warn("broadcast channel full, dropping event", "type", event.Type)
	}
}

// HandleWebSocket handles WebSocket upgrade and connection.
//
// Two independent checks run before the upgrade, in this order:
//
//  1. Origin allowlist — always enforced. It falls back to loopback-only, the
//     same defaults the router uses for CORS. This is what stops a random web
//     page from opening a socket to a self-hosted instance.
//  2. Token — enforced only when the shared API_TOKEN is configured. Browsers
//     cannot set headers on a WebSocket, so the token is accepted as a query
//     parameter, as X-API-Token, or as a bearer Authorization header (the last
//     two cover non-browser clients).
//
// Named tokens may connect too, but only with a scope that covers "/ws".
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !h.originAllowed(r) {
		// Rejected before the upgrade, and before any token check, so an
		// attacker cannot tell "bad origin" from "bad token" by probing.
		slog.Debug("websocket origin rejected",
			"origin", r.Header.Get("Origin"), "host", r.Host)
		http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The library matches patterns against Origin's host only, so the
		// scheme-carrying ALLOWED_ORIGINS values are stripped here. originAllowed
		// above is the stricter, authoritative check.
		OriginPatterns: h.nhooyrOriginPatterns(),
	})
	if err != nil {
		slog.Debug("websocket accept rejected", "error", err, "origin", r.Header.Get("Origin"))
		return
	}

	if h.AuthToken != "" || h.RequireAuth {
		token := websocketToken(r)
		if token == "" {
			rejectUpgrade(conn, "missing token")
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(h.AuthToken)) == 1 {
			// shared token: full access
		} else if h.ValidateToken != nil {
			scopes, ok := h.ValidateToken(r.Context(), token)
			if !ok {
				rejectUpgrade(conn, "invalid token")
				return
			}
			if h.SubjectAllows != nil && !h.SubjectAllows(scopes, "/ws") {
				rejectUpgrade(conn, "token scope does not cover /ws")
				return
			}
		} else {
			rejectUpgrade(conn, "invalid token")
			return
		}
	}

	client := &Client{
		conn: conn,
		send: make(chan Event, 256),
	}

	h.register <- client

	// Start write and read pumps
	go h.writePump(client)
	h.readPump(client)
}

// originAllowed decides whether a request's Origin may open a socket. The rules,
// in order:
//
//   - no Origin header: a non-browser client (curl, a worker). Allowed here; it
//     still needs a token when auth is on.
//   - matches a configured ALLOWED_ORIGINS pattern.
//   - matches this server's own scheme+host: same-origin, served by PicoFarm
//     itself. Without this, reaching the app by LAN IP or hostname would break
//     /ws for every operator who never set ALLOWED_ORIGINS — the allowlist would
//     be loopback-only by default and the Origin would not be loopback.
func (h *Hub) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, pattern := range h.originPatterns() {
		if matchOriginPattern(pattern, origin) {
			return true
		}
	}
	return origin == ownOrigin(r)
}

func (h *Hub) originPatterns() []string {
	if len(h.AllowedOrigins) > 0 {
		return h.AllowedOrigins
	}
	return defaultAllowedOriginPatterns()
}

// nhooyrOriginPatterns converts the configured origins to the host-only form the
// library's own matcher expects. Keeping the library check on top of
// originAllowed means a future change to one of the two is caught by the other.
func (h *Hub) nhooyrOriginPatterns() []string {
	patterns := h.originPatterns()
	converted := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if _, host, ok := strings.Cut(pattern, "://"); ok {
			converted = append(converted, host)
			continue
		}
		converted = append(converted, pattern)
	}
	return converted
}

// ownOrigin builds the origin the server itself is reachable at, from the
// request. It is what a same-origin browser page sends.
func ownOrigin(r *http.Request) string {
	if r.TLS != nil {
		return "https://" + r.Host
	}
	return "http://" + r.Host
}

// matchOriginPattern compares an Origin against an nhooyr pattern, where only
// the port may be a "*". It is a small local copy so the decision can be made
// before the upgrade, where the library's own check is not reachable.
func matchOriginPattern(pattern, origin string) bool {
	scheme, rest, ok := strings.Cut(pattern, "://")
	if !ok {
		return false
	}
	oScheme, oRest, ok := strings.Cut(origin, "://")
	if !ok || oScheme != scheme {
		return false
	}
	host, port, ok := strings.Cut(rest, ":")
	if !ok {
		// A pattern without a port only matches an origin without one.
		return rest == oRest
	}
	oHost, oPort, ok := strings.Cut(oRest, ":")
	if !ok {
		return false
	}
	if oHost != host {
		return false
	}
	return port == "*" || port == oPort
}

// websocketToken reads the token from the query string or the request headers.
func websocketToken(r *http.Request) string {
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}
	if token := r.Header.Get("X-API-Token"); token != "" {
		return token
	}
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return ""
}

// rejectUpgrade closes an already-upgraded socket with a policy violation status.
// The handshake is complete by then, so an HTTP error can no longer be sent.
func rejectUpgrade(conn *websocket.Conn, reason string) {
	conn.Close(websocket.StatusPolicyViolation, reason) //nolint:errcheck // best-effort rejection
}

// defaultAllowedOriginPatterns mirrors the router's CORS defaults. Keys are in
// nhooyr pattern syntax, where the port is a wildcard.
func defaultAllowedOriginPatterns() []string {
	return []string{
		"http://localhost:*",
		"http://127.0.0.1:*",
		"http://[::1]:*",
	}
}

// writePump sends events to the WebSocket connection.
func (h *Hub) writePump(client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close(websocket.StatusNormalClosure, "") //nolint:errcheck // best-effort cleanup
	}()

	for {
		select {
		case event, ok := <-client.send:
			if !ok {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := wsjson.Write(ctx, client.conn, event)
			cancel()

			if err != nil {
				slog.Error("websocket write error", "error", err)
				return
			}

		case <-ticker.C:
			// Send ping
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := client.conn.Ping(ctx)
			cancel()

			if err != nil {
				return
			}
		}
	}
}

// readPump reads messages from the WebSocket connection.
func (h *Hub) readPump(client *Client) {
	defer func() {
		h.unregister <- client
		client.conn.Close(websocket.StatusNormalClosure, "") //nolint:errcheck // best-effort cleanup
	}()

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, message, err := client.conn.Read(ctx)
		cancel()

		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				slog.Error("websocket read error", "error", err)
			}
			return
		}

		// Handle incoming messages (e.g., subscription requests)
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			slog.Debug("received websocket message", "message", msg)
		}
	}
}

// Stop gracefully shuts down the hub, closing all client connections.
func (h *Hub) Stop() {
	h.mu.RLock()
	if h.stopped {
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	close(h.stop)
}
