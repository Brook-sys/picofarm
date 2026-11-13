package service

import (
	"context"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/google/uuid"
)

// Automatic reconnect tuning.
//
// A farm contains printers that are switched off, unplugged or on a flaky
// Wi-Fi, so every attempt costs a full client timeout (Bambu waits up to 10s on
// MQTT, the HTTP clients 10s each). Retrying those every few seconds burns CPU,
// fills the log and hammers a dead address for no gain. The poll therefore stays
// cheap and constant, while the per-printer retry rate decays exponentially to
// a five minute ceiling, with jitter so a whole rack of printers that went dark
// together does not come back in one synchronized burst.
const (
	// autoReconnectPollInterval is the cadence of the bookkeeping pass. It does
	// no network I/O beyond one repository list.
	autoReconnectPollInterval = 5 * time.Second
	// autoReconnectBaseDelay is the wait after the first failed attempt.
	autoReconnectBaseDelay = 5 * time.Second
	// autoReconnectMaxDelay caps the exponential growth.
	autoReconnectMaxDelay = 5 * time.Minute
	// autoReconnectJitter spreads retries by ±20% of the computed delay.
	autoReconnectJitter = 0.2
)

// reconnectBackoff is the reconnect state of a single printer.
type reconnectBackoff struct {
	attempts    int
	nextAttempt time.Time
}

// autoReconnectLoop ticks until ctx is cancelled.
func (s *PrinterService) autoReconnectLoop(ctx context.Context) {
	ticker := time.NewTicker(autoReconnectPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Debug("printer auto reconnect stopped", "reason", ctx.Err())
			return
		case <-ticker.C:
			s.ReconnectOfflinePrinters(ctx)
		}
	}
}

// needsReconnect reports whether a printer state is worth another connection
// attempt. A missing state counts: the manager only records a state after a
// connection attempt, so "no state" means "never tried since startup". Any
// status that suggests the printer is alive (printing, paused, idle) is left
// alone — interrupting a running print to reconnect would be worse than the
// outage.
func needsReconnect(state *model.PrinterState) bool {
	if state == nil {
		return true
	}
	return state.Status == model.PrinterStatusOffline || state.Status == model.PrinterStatusError
}

// printerNeedsAutoReconnect reports whether a printer is a candidate for a
// reconnect attempt right now. It reads the live state so a printer that is
// actually printing is never touched.
func (s *PrinterService) printerNeedsAutoReconnect(p *model.Printer, now time.Time) bool {
	if p.ConnectionType == model.ConnectionTypeManual || p.MaintenanceMode {
		return false
	}
	// chitu (and anything else the manager has no client for) would fail on
	// every single attempt forever. Skipping it keeps the log readable and
	// leaves the gap visible where it belongs: the missing client.
	if !printer.SupportsConnectionType(p.ConnectionType) {
		return false
	}
	state, err := s.manager.GetState(p.ID)
	if err == nil && !needsReconnect(state) {
		return false
	}

	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	if _, inFlight := s.reconnecting[p.ID]; inFlight {
		// A manual reconnect from the UI is already running for this printer.
		return false
	}
	if backoff, ok := s.reconnectBackoff[p.ID]; ok && now.Before(backoff.nextAttempt) {
		return false
	}
	return true
}

// startAutoReconnect reserves a slot for the printer and performs the reconnect
// in the background. Reserving under the lock is what prevents the next pass
// from starting a second attempt for the same printer.
func (s *PrinterService) startAutoReconnect(ctx context.Context, p *model.Printer) {
	s.reconnectMu.Lock()
	if s.reconnecting == nil {
		s.reconnecting = make(map[uuid.UUID]time.Time)
	}
	if s.reconnectBackoff == nil {
		s.reconnectBackoff = make(map[uuid.UUID]reconnectBackoff)
	}
	s.reconnecting[p.ID] = time.Now()
	s.reconnectMu.Unlock()

	printerCopy := *p
	go func() {
		defer s.finishAutoReconnect(printerCopy.ID)
		// Reconnect reloads the printer from the repository, so credentials
		// edited in the meantime are picked up.
		if err := s.Reconnect(ctx, printerCopy.ID); err != nil {
			s.recordReconnectFailure(printerCopy.ID, err)
			return
		}
		s.recordReconnectSuccess(printerCopy.ID)
	}()
}

// finishAutoReconnect clears the in-flight marker.
func (s *PrinterService) finishAutoReconnect(id uuid.UUID) {
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	delete(s.reconnecting, id)
}

// recordReconnectFailure applies the exponential backoff for the next attempt.
func (s *PrinterService) recordReconnectFailure(id uuid.UUID, err error) {
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	if s.reconnectBackoff == nil {
		s.reconnectBackoff = make(map[uuid.UUID]reconnectBackoff)
	}
	backoff := s.reconnectBackoff[id]
	backoff.attempts++
	delay := reconnectDelay(backoff.attempts)
	backoff.nextAttempt = time.Now().Add(delay)
	s.reconnectBackoff[id] = backoff

	slog.Warn("printer auto reconnect failed",
		"printer_id", id,
		"attempt", backoff.attempts,
		"retry_in", delay.Round(time.Second),
		"error", err,
	)
}

// recordReconnectSuccess clears the backoff so the next failure starts from the
// short delay again.
func (s *PrinterService) recordReconnectSuccess(id uuid.UUID) {
	s.reconnectMu.Lock()
	attempts := 0
	if backoff, ok := s.reconnectBackoff[id]; ok {
		attempts = backoff.attempts
		delete(s.reconnectBackoff, id)
	}
	s.reconnectMu.Unlock()

	if attempts > 0 {
		slog.Info("printer reconnected after failures", "printer_id", id, "failed_attempts", attempts)
	}
}

// reconnectDelay returns how long to wait before attempt number n, capped and
// jittered. Attempt 1 waits the base delay.
func reconnectDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := float64(autoReconnectBaseDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(autoReconnectMaxDelay) {
		delay = float64(autoReconnectMaxDelay)
	}
	// Jitter both directions: without it, printers that went offline together
	// retry together forever.
	jitter := 1 + (rand.Float64()*2-1)*autoReconnectJitter
	return time.Duration(delay * jitter)
}
