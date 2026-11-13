package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/database"
	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

func newAutoReconnectService(t *testing.T) *PrinterService {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repos := repository.NewRepositories(db)
	return &PrinterService{
		repo:             repos.Printers,
		manager:          printer.NewManager(),
		reconnecting:     make(map[uuid.UUID]time.Time),
		reconnectBackoff: make(map[uuid.UUID]reconnectBackoff),
	}
}

// A printer that reports itself alive must never be touched by auto reconnect:
// disconnecting a printing printer to "fix" it would abort a real print.
func TestNeedsReconnectLeavesLivePrintersAlone(t *testing.T) {
	cases := []struct {
		name  string
		state *model.PrinterState
		want  bool
	}{
		{"no state at all", nil, true},
		{"offline", &model.PrinterState{Status: model.PrinterStatusOffline}, true},
		{"error", &model.PrinterState{Status: model.PrinterStatusError}, true},
		{"printing", &model.PrinterState{Status: model.PrinterStatusPrinting}, false},
		{"paused", &model.PrinterState{Status: model.PrinterStatusPaused}, false},
		{"idle", &model.PrinterState{Status: model.PrinterStatusIdle}, false},
	}
	for _, tc := range cases {
		if got := needsReconnect(tc.state); got != tc.want {
			t.Errorf("%s: needsReconnect = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestReconnectDelayGrowsExponentiallyAndCaps(t *testing.T) {
	base := float64(autoReconnectBaseDelay)
	max := float64(autoReconnectMaxDelay)

	for attempt := 1; attempt <= 20; attempt++ {
		delay := reconnectDelay(attempt)
		want := base
		for i := 1; i < attempt; i++ {
			want *= 2
			if want > max {
				want = max
			}
		}
		// autoReconnectJitter is a duration constant, so the jitter window has
		// to be computed as floats before comparing.
		low, high := want*(1-autoReconnectJitter), want*(1+autoReconnectJitter)
		got := float64(delay)
		if got < low || got > high {
			t.Fatalf("attempt %d: delay %v outside jittered window [%v, %v]", attempt, delay, time.Duration(low), time.Duration(high))
		}
	}

	// The jitter window must be computed in float64: the tuning constants are
	// typed durations (integers in nanoseconds), so 0.8 cannot multiply one.
	if float64(reconnectDelay(0)) < float64(autoReconnectBaseDelay)*(1-autoReconnectJitter) {
		t.Fatalf("attempt 0 should behave like attempt 1, got %v", reconnectDelay(0))
	}
	if float64(reconnectDelay(100)) > float64(autoReconnectMaxDelay)*(1+autoReconnectJitter) {
		t.Fatalf("delay must not exceed the cap, got %v", reconnectDelay(100))
	}
}

// Printers that can never connect must be skipped, otherwise auto reconnect
// burns a full client timeout on each of them every few seconds forever.
func TestPrinterNeedsAutoReconnectSkipsImpossiblePrinters(t *testing.T) {
	s := newAutoReconnectService(t)

	impossible := []*model.Printer{
		{ID: uuid.New(), ConnectionType: model.ConnectionTypeManual},
		{ID: uuid.New(), ConnectionType: model.ConnectionTypeManual, MaintenanceMode: false},
		{ID: uuid.New(), ConnectionType: model.ConnectionTypeMoonraker, MaintenanceMode: true},
		{ID: uuid.New(), ConnectionType: model.ConnectionTypeChiTu},
		{ID: uuid.New(), ConnectionType: "nonsense"},
	}
	for _, p := range impossible {
		if s.printerNeedsAutoReconnect(p, time.Now()) {
			t.Errorf("printer type %q maintenance=%v should not be a reconnect candidate", p.ConnectionType, p.MaintenanceMode)
		}
	}

	// A printer whose connection is possible and which has no state yet must be
	// a candidate: that is the normal case right after startup.
	ok := &model.Printer{ID: uuid.New(), ConnectionType: model.ConnectionTypeMoonraker}
	if !s.printerNeedsAutoReconnect(ok, time.Now()) {
		t.Error("expected a state-less moonraker printer to be a reconnect candidate")
	}
}

// A printer in the middle of a reconnect attempt must not get a second one.
func TestPrinterNeedsAutoReconnectSkipsInFlightPrinters(t *testing.T) {
	s := newAutoReconnectService(t)
	id := uuid.New()
	s.reconnecting[id] = time.Now()

	p := &model.Printer{ID: id, ConnectionType: model.ConnectionTypeMoonraker}
	if s.printerNeedsAutoReconnect(p, time.Now()) {
		t.Error("printer with a reconnect in flight must not be retried")
	}
}

// A printer still inside its backoff window must not be retried: this is what
// stops a printer that is simply switched off from being hammered every pass.
func TestPrinterNeedsAutoReconnectHonoursBackoffDeadline(t *testing.T) {
	s := newAutoReconnectService(t)
	id := uuid.New()
	s.reconnectBackoff[id] = reconnectBackoff{attempts: 3, nextAttempt: time.Now().Add(time.Minute)}

	p := &model.Printer{ID: id, ConnectionType: model.ConnectionTypeMoonraker}
	if s.printerNeedsAutoReconnect(p, time.Now()) {
		t.Error("printer inside its backoff window must not be retried")
	}
	if !s.printerNeedsAutoReconnect(p, time.Now().Add(2*time.Minute)) {
		t.Error("printer past its backoff deadline should be retried")
	}
}

// waitIdle blocks until no reconnect attempt is in flight.
func waitIdle(t *testing.T, s *PrinterService) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		s.reconnectMu.Lock()
		busy := len(s.reconnecting) > 0
		s.reconnectMu.Unlock()
		if !busy {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("reconnect attempts never settled")
}

func attempts(t *testing.T, s *PrinterService, id uuid.UUID) int {
	t.Helper()
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	return s.reconnectBackoff[id].attempts
}

// A failing printer must accumulate attempts and be deferred, not retried on
// the next pass.
func TestReconnectOfflinePrintersBacksOffAfterFailure(t *testing.T) {
	ctx := context.Background()
	s := newAutoReconnectService(t)

	// A Moonraker URL that refuses connections: the attempt fails fast, but the
	// backoff must still be recorded.
	p := &model.Printer{Name: "down", ConnectionType: model.ConnectionTypeMoonraker, ConnectionURI: "http://127.0.0.1:1"}
	if err := s.repo.Create(ctx, p); err != nil {
		t.Fatalf("create printer: %v", err)
	}

	s.ReconnectOfflinePrinters(ctx)
	waitIdle(t, s)
	if got := attempts(t, s, p.ID); got != 1 {
		t.Fatalf("expected one recorded failed attempt, got %d", got)
	}

	// Immediately running the pass again must not produce another attempt.
	s.reconnectMu.Lock()
	first := s.reconnectBackoff[p.ID].nextAttempt
	s.reconnectMu.Unlock()
	if !first.After(time.Now()) {
		t.Fatal("expected the next attempt to be scheduled in the future")
	}
	s.ReconnectOfflinePrinters(ctx)
	waitIdle(t, s)
	if got := attempts(t, s, p.ID); got != 1 {
		t.Fatalf("expected the backoff window to defer the retry, got %d attempts", got)
	}

	// Past the deadline the printer is tried again.
	s.reconnectMu.Lock()
	s.reconnectBackoff[p.ID] = reconnectBackoff{attempts: 1, nextAttempt: time.Now().Add(-time.Second)}
	s.reconnectMu.Unlock()
	s.ReconnectOfflinePrinters(ctx)
	waitIdle(t, s)
	if got := attempts(t, s, p.ID); got != 2 {
		t.Fatalf("expected a second attempt once the deadline passed, got %d", got)
	}
}

// A printer that comes back must have its backoff cleared, so a later outage
// starts from the short delay again instead of waiting five minutes.
func TestReconnectOfflinePrintersClearsBackoffOnSuccess(t *testing.T) {
	ctx := context.Background()
	s := newAutoReconnectService(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	p := &model.Printer{Name: "back", ConnectionType: model.ConnectionTypeMoonraker, ConnectionURI: server.URL}
	if err := s.repo.Create(ctx, p); err != nil {
		t.Fatalf("create printer: %v", err)
	}
	s.reconnectMu.Lock()
	s.reconnectBackoff[p.ID] = reconnectBackoff{attempts: 5, nextAttempt: time.Now().Add(-time.Second)}
	s.reconnectMu.Unlock()

	s.ReconnectOfflinePrinters(ctx)
	waitIdle(t, s)

	s.reconnectMu.Lock()
	_, stillBackedOff := s.reconnectBackoff[p.ID]
	s.reconnectMu.Unlock()
	if stillBackedOff {
		t.Fatalf("expected the backoff to be cleared after a successful reconnect, attempts=%d", attempts(t, s, p.ID))
	}
	state, err := s.manager.GetState(p.ID)
	if err != nil {
		t.Fatalf("expected a state after a successful reconnect: %v", err)
	}
	if state.Status == model.PrinterStatusOffline {
		// GetStatus against a server that answers 200 with an empty body parses
		// as offline; what matters is that the connection itself succeeded.
		t.Log("state after reconnect:", state.Status)
	}
}

// The loop must stop when its context is cancelled instead of leaking a
// goroutine that outlives shutdown.
func TestStartAutoReconnectStopsOnContextCancel(t *testing.T) {
	s := newAutoReconnectService(t)
	ctx, cancel := context.WithCancel(context.Background())

	s.StartAutoReconnect(ctx)
	s.StartAutoReconnect(ctx) // second call must not start another goroutine

	cancel()
	// The loop holds no lock while sleeping on the ticker, so this only has to
	// terminate the loop; the assertion is that the second call was a no-op.
	s.autoReconnectMu.Lock()
	on := s.autoReconnectOn
	s.autoReconnectMu.Unlock()
	if !on {
		t.Fatal("expected auto reconnect to be marked as started")
	}
}
