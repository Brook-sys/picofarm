package service

import (
	"context"
	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newAutoReconnectService(t *testing.T) *PrinterService {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s := &PrinterService{repo: repository.NewRepositories(db).Printers, manager: printer.NewManager()}
	t.Cleanup(func() { s.StopReconnect(); s.manager.DisconnectAll(); _ = db.Close() })
	return s
}
func TestNeedsReconnectLeavesLivePrintersAlone(t *testing.T) {
	for _, status := range []model.PrinterStatus{model.PrinterStatusError, model.PrinterStatusPrinting, model.PrinterStatusPaused, model.PrinterStatusIdle} {
		if needsReconnect(&model.PrinterState{Status: status}) {
			t.Fatalf("must not reconnect %s", status)
		}
	}
	if !needsReconnect(nil) || !needsReconnect(&model.PrinterState{Status: model.PrinterStatusOffline}) {
		t.Fatal("must retry offline")
	}
}
func TestReconnectConfig(t *testing.T) {
	for _, tc := range []struct {
		interval, max string
		valid         bool
	}{{"10s", "4", true}, {"2s", "1", true}, {"1.99s", "4", false}, {"no", "4", false}, {"10s", "0", false}, {"10s", "-1", false}, {"10s", "1.5", false}, {"", "4", false}} {
		t.Run(tc.interval+tc.max, func(t *testing.T) {
			t.Setenv("PRINTER_RECONNECT_INTERVAL", tc.interval)
			t.Setenv("PRINTER_RECONNECT_MAX_CONCURRENT", tc.max)
			_, err := PrinterReconnectConfigFromEnv()
			if (err == nil) != tc.valid {
				t.Fatal(err)
			}
		})
	}
}
func TestReconnectFairFIFOAndStartSpacing(t *testing.T) {
	now := time.Unix(100, 0)
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	q := &reconnectQueue{config: DefaultPrinterReconnectConfig(), lastStart: map[uuid.UUID]time.Time{a: now}, queue: []*reconnectRequest{{id: a}, {id: b}, {id: c}}}
	if r := takeReconnect(q, now); r.id != b {
		t.Fatal("cooling head blocked FIFO")
	}
	if r := takeReconnect(q, now); r.id != c {
		t.Fatal("starved third printer")
	}
	if r := takeReconnect(q, now.Add(9*time.Second)); r != nil {
		t.Fatal("started early")
	}
	if r := takeReconnect(q, now.Add(10*time.Second)); r == nil || r.id != a {
		t.Fatal("deadline not eligible")
	}
}
func TestReconnectBoundedAndManualAutoDeduplicated(t *testing.T) {
	s := newAutoReconnectService(t)
	if err := s.ConfigureReconnect(PrinterReconnectConfig{2 * time.Second, 2}); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{}, 10)
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
		}
		entered <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()
	var first uuid.UUID
	for i := 0; i < 6; i++ {
		p := &model.Printer{Name: "offline", ConnectionType: model.ConnectionTypeMoonraker, ConnectionURI: server.URL}
		if err := s.repo.Create(context.Background(), p); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = p.ID
		}
	}
	s.ReconnectOfflinePrinters(context.Background())
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("worker not started")
		}
	}
	s.reconnectMu.Lock()
	before := s.reconnectQueue.pending[first]
	s.reconnectMu.Unlock()
	r, err := s.enqueueReconnect(first, false)
	if err != nil || r != before {
		t.Fatal("manual duplicated auto")
	}
	s.StopReconnect()
	if peak.Load() > 2 {
		t.Fatalf("concurrency %d", peak.Load())
	}
}
func TestReconnectMaintenanceCheckedAtExecution(t *testing.T) {
	s := newAutoReconnectService(t)
	p := &model.Printer{Name: "maintenance", ConnectionType: model.ConnectionTypeMoonraker, MaintenanceMode: true, ConnectionURI: "http://127.0.0.1:1"}
	if err := s.repo.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := s.performReconnect(context.Background(), p.ID, false); err == nil {
		t.Fatal("maintenance connected")
	}
}

func TestAutoReconnectPowerCycleWithoutManualRequest(t *testing.T) {
	s := newAutoReconnectService(t)
	if err := s.ConfigureReconnect(PrinterReconnectConfig{2 * time.Second, 1}); err != nil {
		t.Fatal(err)
	}
	var online atomic.Bool
	var probes atomic.Int32
	var actions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			actions.Add(1)
		}
		if r.URL.Path == "/api/version" {
			probes.Add(1)
		}
		if !online.Load() {
			http.Error(w, "off", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"state":{"flags":{"operational":true}}}`))
	}))
	defer server.Close()
	p := &model.Printer{Name: "powercycle", ConnectionType: model.ConnectionTypeOctoPrint, ConnectionURI: server.URL}
	if err := s.repo.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.StartAutoReconnect(ctx)
	deadline := time.After(4 * time.Second)
	for probes.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("automatic initial attempt missing")
		case <-time.After(10 * time.Millisecond):
		}
	}
	online.Store(true)
	deadline = time.After(5 * time.Second)
	for {
		state, err := s.manager.GetState(p.ID)
		if err == nil && state.Status != model.PrinterStatusOffline {
			break
		}
		select {
		case <-deadline:
			t.Fatal("power-on did not recover automatically")
		case <-time.After(10 * time.Millisecond):
		}
	}
	if probes.Load() < 2 {
		t.Fatal("recovery had no retry")
	}
	// Now exercise a real power cycle of the already-connected polling client.
	online.Store(false)
	deadline = time.After(7 * time.Second)
	for {
		state, err := s.manager.GetState(p.ID)
		if err == nil && state.Status == model.PrinterStatusOffline {
			break
		}
		select {
		case <-deadline:
			t.Fatal("poll did not detect power-off")
		case <-time.After(10 * time.Millisecond):
		}
	}
	online.Store(true)
	deadline = time.After(6 * time.Second)
	for {
		state, err := s.manager.GetState(p.ID)
		if err == nil && state.Status != model.PrinterStatusOffline {
			break
		}
		select {
		case <-deadline:
			t.Fatal("connected printer did not recover after power cycle")
		case <-time.After(10 * time.Millisecond):
		}
	}

	if actions.Load() != 0 {
		t.Fatal("reconnect sent an action")
	}
}

func TestAutomaticReconnectSkipRules(t *testing.T) {
	s := newAutoReconnectService(t)
	for _, p := range []*model.Printer{{ConnectionType: model.ConnectionTypeManual}, {ConnectionType: model.ConnectionTypeChiTu}, {ConnectionType: "unknown"}, {ConnectionType: model.ConnectionTypeMoonraker, MaintenanceMode: true}} {
		if s.printerNeedsAutoReconnect(p, time.Now()) {
			t.Fatalf("invalid candidate: %+v", p)
		}
	}
	if !s.printerNeedsAutoReconnect(&model.Printer{ID: uuid.New(), ConnectionType: model.ConnectionTypeMoonraker}, time.Now()) {
		t.Fatal("missing state must retry")
	}
}
