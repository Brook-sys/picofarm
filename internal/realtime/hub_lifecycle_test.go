package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

const lifecycleTimeout = 2 * time.Second

// lifecycleServer observes the actual HTTP handler, not a process-wide goroutine
// count (whose baseline already includes the very pumps we want to see exit).
func lifecycleServer(t *testing.T, hub *Hub, clients int) (*httptest.Server, <-chan struct{}, <-chan struct{}) {
	t.Helper()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		hub.Run()
	}()
	handlerDone := make(chan struct{}, clients)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { handlerDone <- struct{}{} }()
		hub.HandleWebSocket(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(hub.Stop)
	return srv, runDone, handlerDone
}

func awaitLifecycle(t *testing.T, ctx context.Context, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for %s: %v", what, ctx.Err())
	}
}

// A real client reads the shutdown result so a graceful implementation can
// complete its close handshake too. The required signal is handler completion:
// a readPump stuck sending unregister after Stop cannot produce it.
func TestHubStopDoesNotParkReadPump(t *testing.T) {
	hub := NewHub()
	srv, runDone, handlerDone := lifecycleServer(t, hub, 1)
	conn, err := dial(t, srv, "", "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()
	waitForClients(t, hub, 1)

	ctx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()
	readDone := make(chan error, 1)
	go func() {
		_, _, err := conn.Read(ctx)
		readDone <- err
	}()
	hub.Stop()
	awaitLifecycle(t, ctx, runDone, "hub Run to return")
	awaitLifecycle(t, ctx, handlerDone, "WebSocket handler to return")
	select {
	case err := <-readDone:
		// Stop intentionally uses CloseNow: EOF rather than a close frame is
		// allowed, but a context timeout is not evidence of server shutdown.
		if err == nil || ctx.Err() != nil {
			t.Fatalf("client did not observe server shutdown: %v (context: %v)", err, ctx.Err())
		}
	case <-ctx.Done():
		t.Fatal("client did not observe server shutdown before deadline")
	}
}

// Peers deliberately do not read, so a sequential Close handshake would take
// five seconds per client. All handlers and Run must return within one shared
// deadline, without depending on client cleanup to release them.
func TestHubStopMultipleActiveClients(t *testing.T) {
	const clients = 8
	hub := NewHub()
	srv, runDone, handlerDone := lifecycleServer(t, hub, clients)
	for i := 0; i < clients; i++ {
		conn, err := dial(t, srv, "", "")
		if err != nil {
			t.Fatalf("dial client %d: %v", i, err)
		}
		defer conn.CloseNow()
	}
	waitForClients(t, hub, clients)

	ctx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()
	hub.Stop()
	awaitLifecycle(t, ctx, runDone, "hub Run to return")
	for i := 0; i < clients; i++ {
		awaitLifecycle(t, ctx, handlerDone, "all WebSocket handlers to return")
	}
	hub.mu.RLock()
	remaining := len(hub.clients)
	hub.mu.RUnlock()
	if remaining != 0 {
		t.Fatalf("%d clients remain registered after shutdown", remaining)
	}
}

func TestHubHandleWebSocketAfterStop(t *testing.T) {
	hub := NewHub()
	srv, runDone, handlerDone := lifecycleServer(t, hub, 1)
	hub.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()
	awaitLifecycle(t, ctx, runDone, "hub Run to return before dial")

	conn, err := dial(t, srv, "", "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()
	awaitLifecycle(t, ctx, handlerDone, "post-stop handler to return")
	if _, _, err := conn.Read(ctx); err == nil || ctx.Err() != nil {
		t.Fatalf("post-stop socket was not closed promptly: %v (context: %v)", err, ctx.Err())
	}
}

// Wait for all simultaneous callers: returning from the test early would hide
// panics or blocked calls in goroutines that had not yet run.
func TestHubStopIsIdempotent(t *testing.T) {
	hub := NewHub()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		hub.Run()
	}()
	var callers sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			<-start
			hub.Stop()
		}()
	}
	callersDone := make(chan struct{})
	go func() {
		callers.Wait()
		close(callersDone)
	}()
	close(start)
	ctx, cancel := context.WithTimeout(context.Background(), lifecycleTimeout)
	defer cancel()
	awaitLifecycle(t, ctx, callersDone, "concurrent Stop callers")
	awaitLifecycle(t, ctx, runDone, "hub Run to return")
	hub.Stop()
}

func waitForClients(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(lifecycleTimeout)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		got := len(hub.clients)
		hub.mu.RUnlock()
		if got == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("hub never reached %d clients", want)
}
