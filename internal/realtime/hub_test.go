package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// startHubServer serves /ws from a hub whose Run loop is active, which is what
// a real handshake needs (the register channel is consumed there).
func startHubServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	go hub.Run()
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	t.Cleanup(srv.Close)
	return srv
}

// dial opens a handshake with a caller-controlled Origin. The library copies
// HTTPHeader verbatim and only adds the protocol headers, so this is the one way
// to reach the origin check with an arbitrary origin.
func dial(t *testing.T, srv *httptest.Server, host, origin string) (*websocket.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	header := http.Header{}
	if origin != "" {
		header.Set("Origin", origin)
	}
	opts := &websocket.DialOptions{HTTPHeader: header}
	if host != "" {
		opts.Host = host
	}
	conn, resp, err := websocket.Dial(ctx, srv.URL, opts)
	// Dial owns the upgraded body (nil on success); only close a diagnostic body.
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	return conn, err
}

func TestHubOriginAllowlist(t *testing.T) {
	cases := []struct {
		name      string
		allowed   []string
		host      string
		origin    string
		wantAllow bool
	}{
		{
			name:      "loopback origin is allowed by default",
			allowed:   nil,
			origin:    "http://localhost:8084",
			wantAllow: true,
		},
		{
			// The failure this rule prevents: an operator reaching the app by
			// LAN IP, with ALLOWED_ORIGINS never set, would otherwise lose /ws.
			name:      "same origin as the request host is allowed",
			allowed:   nil,
			host:      "picofarm.lan:8084",
			origin:    "http://picofarm.lan:8084",
			wantAllow: true,
		},
		{
			name:      "a foreign origin is rejected",
			allowed:   nil,
			origin:    "https://evil.example.com",
			wantAllow: false,
		},
		{
			name:      "a configured origin is allowed",
			allowed:   []string{"https://farm.example.test"},
			origin:    "https://farm.example.test",
			wantAllow: true,
		},
		{
			name:      "a configured origin does not cover another host",
			allowed:   []string{"https://farm.example.test"},
			origin:    "https://other.example.test",
			wantAllow: false,
		},
		{
			name:      "a non-browser client without Origin is allowed",
			allowed:   nil,
			origin:    "",
			wantAllow: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hub := NewHub()
			hub.AllowedOrigins = tc.allowed
			srv := startHubServer(t, hub)

			conn, err := dial(t, srv, tc.host, tc.origin)
			if tc.wantAllow {
				if err != nil {
					t.Fatalf("origin %q rejected: %v", tc.origin, err)
				}
				conn.CloseNow()
				return
			}
			if err == nil {
				conn.CloseNow()
				t.Fatalf("origin %q was accepted, want rejection", tc.origin)
			}
		})
	}
}

// With auth on, a handshake without a token must be closed with a policy
// violation rather than silently accepted.
func TestHubRequiresTokenWhenAuthEnabled(t *testing.T) {
	hub := NewHub()
	hub.AuthToken = "senha-global"
	srv := startHubServer(t, hub)

	conn, err := dial(t, srv, "", "http://localhost:8084")
	if err != nil {
		// A handshake-level rejection is also acceptable.
		return
	}
	defer conn.CloseNow()

	// The hub closes the upgraded socket; the next read must fail.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, _, err := conn.Read(ctx); err == nil {
		t.Fatal("connection without a token stayed open with auth enabled")
	}
}

func TestHubAcceptsSharedTokenFromQuery(t *testing.T) {
	hub := NewHub()
	hub.AuthToken = "senha-global"
	srv := startHubServer(t, hub)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	header := http.Header{}
	header.Set("Origin", "http://localhost:8084")
	conn, resp, err := websocket.Dial(ctx, srv.URL+"?token=senha-global", &websocket.DialOptions{HTTPHeader: header})
	// Dial owns the upgraded body (nil on success); only close a diagnostic body.
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial with shared token: %v", err)
	}
	defer conn.CloseNow()

	// The connection must be usable, which is why the read is allowed to block:
	// any immediate closure would surface here as an error.
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("status = %d, want 101", resp.StatusCode)
	}
}

// A named token that is valid connects, and one with a scope that does not cover
// /ws is closed instead.
func TestHubAcceptsScopedNamedToken(t *testing.T) {
	hub := NewHub()
	hub.AuthToken = "senha-global"
	hub.ValidateToken = func(ctx context.Context, token string) ([]string, bool) {
		switch token {
		case "slicer":
			return []string{"/api/slicer"}, true
		case "amplo":
			return []string{"*"}, true
		default:
			return nil, false
		}
	}
	hub.SubjectAllows = func(scopes []string, subject string) bool {
		for _, s := range scopes {
			if s == "*" || s == subject {
				return true
			}
		}
		return false
	}
	srv := startHubServer(t, hub)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	header := http.Header{}
	header.Set("Origin", "http://localhost:8084")

	allowed, resp, err := websocket.Dial(ctx, srv.URL+"?token=amplo", &websocket.DialOptions{HTTPHeader: header})
	// Dial owns the upgraded body (nil on success); only close a diagnostic body.
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("full-scope named token: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		allowed.CloseNow()
		t.Errorf("status = %d, want 101", resp.StatusCode)
	}
	allowed.CloseNow()

	// The handshake itself succeeds; the socket is closed right after the
	// upgrade, which is how a scope rejection surfaces from an already-upgraded
	// connection. So the assertion is on the closed connection, not the dial.
	denied, resp, err := websocket.Dial(ctx, srv.URL+"?token=slicer", &websocket.DialOptions{HTTPHeader: header})
	// Dial owns the upgraded body (nil on success); only close a diagnostic body.
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	if err != nil {
		return // rejected at the handshake: also correct
	}
	defer denied.CloseNow()
	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()
	if _, _, err := denied.Read(readCtx); err == nil {
		t.Fatal("a token whose scope does not cover /ws was accepted")
	}
}
