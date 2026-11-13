package printer

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// moonrakerServer returns a test server that answers 200 for every path, with
// an optional hook so a test can block a specific request.
func moonrakerServer(t *testing.T, hook func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hook != nil {
			hook(w, r)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

// Regression test for the global lock in Connect. The manager used to hold its
// write mutex for the whole Connect, including the network call, so a printer
// that was unreachable froze every status read and every other printer's
// connection for as long as the client timeout. One slow printer is enough to
// stall a whole farm.
func TestManagerConnectDoesNotHoldManagerLockDuringNetworkIO(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	slow := moonrakerServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/info" {
			return
		}
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
	})
	fast := moonrakerServer(t, nil)

	manager := NewManager()
	t.Cleanup(manager.DisconnectAll)

	slowPrinter := &model.Printer{
		ID:             uuid.New(),
		Name:           "slow",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  slow.URL,
	}
	fastPrinter := &model.Printer{
		ID:             uuid.New(),
		Name:           "fast",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  fast.URL,
	}

	slowDone := make(chan error, 1)
	go func() { slowDone <- manager.Connect(slowPrinter) }()

	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("slow printer never reached network I/O")
	}

	// The slow printer is now blocked inside its HTTP call. Nothing else may
	// wait behind it.
	start := time.Now()
	if err := manager.Connect(fastPrinter); err != nil {
		t.Fatalf("connect fast printer while another is connecting: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("connecting a second printer took %v while the first was blocked in network I/O", elapsed)
	}
	if _, err := manager.GetState(fastPrinter.ID); err != nil {
		t.Fatalf("state read blocked or failed while another printer was connecting: %v", err)
	}
	if states := manager.GetAllStates(); len(states) == 0 {
		t.Fatal("expected at least one state while another printer was connecting")
	}

	close(release)
	select {
	case err := <-slowDone:
		if err != nil {
			t.Fatalf("slow printer connect: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("slow printer connect never finished after being released")
	}
}

// A second Connect for the same printer must replace the previous client
// instead of leaking it, otherwise the old MQTT/WebSocket connection stays
// open forever and keeps reporting status for a printer the UI thinks is gone.
func TestManagerConnectReplacesPreviousClientWithoutLeaking(t *testing.T) {
	server := moonrakerServer(t, nil)
	manager := NewManager()
	t.Cleanup(manager.DisconnectAll)

	p := &model.Printer{
		ID:             uuid.New(),
		Name:           "replaced",
		ConnectionType: model.ConnectionTypeMoonraker,
		ConnectionURI:  server.URL,
	}

	for i := 0; i < 3; i++ {
		if err := manager.Connect(p); err != nil {
			t.Fatalf("connect #%d: %v", i+1, err)
		}
	}

	manager.mu.RLock()
	count := len(manager.clients)
	manager.mu.RUnlock()
	if count != 1 {
		t.Fatalf("expected exactly one client after repeated connects, got %d", count)
	}
}

func TestManagerConnectRejectsUnsupportedConnectionType(t *testing.T) {
	manager := NewManager()

	p := &model.Printer{ID: uuid.New(), Name: "chitu", ConnectionType: model.ConnectionTypeChiTu}
	if err := manager.Connect(p); err == nil {
		t.Fatal("expected an error for a connection type with no client")
	}

	// The failure must not leave a half-open state behind either.
	state, err := manager.GetState(p.ID)
	if err != nil {
		t.Fatalf("expected an offline state to be recorded, got error %v", err)
	}
	if state.Status != model.PrinterStatusOffline {
		t.Fatalf("status = %q, want offline", state.Status)
	}
}

func TestSupportsConnectionType(t *testing.T) {
	supported := []model.ConnectionType{
		model.ConnectionTypeOctoPrint,
		model.ConnectionTypeBambuLAN,
		model.ConnectionTypeBambuCloud,
		model.ConnectionTypeMoonraker,
	}
	for _, ct := range supported {
		if !SupportsConnectionType(ct) {
			t.Errorf("%q should be supported", ct)
		}
	}
	for _, ct := range []model.ConnectionType{model.ConnectionTypeManual, model.ConnectionTypeChiTu, model.ConnectionType("nonsense")} {
		if SupportsConnectionType(ct) {
			t.Errorf("%q should not be supported", ct)
		}
	}
}

func TestManagerConnectSkipsMaintenanceMode(t *testing.T) {
	manager := NewManager()

	p := &model.Printer{
		ID:              uuid.New(),
		ConnectionType:  model.ConnectionTypeMoonraker,
		MaintenanceMode: true,
	}
	if err := manager.Connect(p); err != nil {
		t.Fatalf("maintenance mode should be a no-op, got %v", err)
	}
	state, err := manager.GetState(p.ID)
	if err != nil {
		t.Fatalf("expected an offline state for a printer in maintenance: %v", err)
	}
	if state.Status != model.PrinterStatusOffline {
		t.Fatalf("status = %q, want offline", state.Status)
	}
}

func TestManagerConnectFailsAfterDisconnectAll(t *testing.T) {
	manager := NewManager()
	manager.DisconnectAll()

	p := &model.Printer{ID: uuid.New(), ConnectionType: model.ConnectionTypeMoonraker}
	if err := manager.Connect(p); err == nil {
		t.Fatal("expected Connect to refuse work after DisconnectAll (graceful shutdown)")
	}
}
