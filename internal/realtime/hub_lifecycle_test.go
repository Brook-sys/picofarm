package realtime

import (
	"net/url"
	"runtime"
	"testing"
	"time"
)

// stopWait blocks until the hub's Run loop has returned, so the test observes
// post-stop goroutines deterministically rather than racing them.
func stopWait(hub *Hub) {
	for i := 0; i < 200; i++ {
		hub.Stop()
		select {
		case <-time.After(10 * time.Millisecond):
		}
		hub.mu.RLock()
		stopped := len(hub.clients) == 0
		hub.mu.RUnlock()
		if stopped {
			return
		}
	}
}

// TestHubStopDoesNotParkReadPump guards a real leak: readPump used to send to
// the unbuffered unregister channel after Run stopped receiving, parking one
// goroutine per client. Repeated Start/Stop (desktop host, tests) would then
// accumulate goroutines forever.
func TestHubStopDoesNotParkReadPump(t *testing.T) {
	hub := NewHub()
	srv := startHubServer(t, hub)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + u.Host

	conn, err := dial(t, srv, "", origin)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.CloseNow()

	// Confirm the connection is registered before measuring the baseline.
	waitForClients(t, hub, 1)

	before := runtime.NumGoroutine()
	stopWait(hub)

	// readPump must observe the closed socket and exit through the select with
	// stop; writePump exits when the client's send channel closes.
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before+1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+1 {
		t.Fatalf("goroutines after Stop: before=%d after=%d (readPump parked?)", before, after)
	}
}

// TestHubStopIsIdempotent guards the TOCTOU panic: closing an already-closed
// stop channel crashes the process.
func TestHubStopIsIdempotent(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	for i := 0; i < 5; i++ {
		hub.Stop()
	}
}

func waitForClients(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		got := len(hub.clients)
		hub.mu.RUnlock()
		if got >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("hub never reached %d clients", want)
}
