package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/printer"
	"github.com/google/uuid"
)

// PrinterReconnectConfig controls both automatic and operator-requested attempts.
type PrinterReconnectConfig struct {
	Interval      time.Duration
	MaxConcurrent int
}

func DefaultPrinterReconnectConfig() PrinterReconnectConfig {
	return PrinterReconnectConfig{10 * time.Second, 4}
}
func (c PrinterReconnectConfig) Validate() error {
	if c.Interval < 2*time.Second {
		return fmt.Errorf("PRINTER_RECONNECT_INTERVAL must be a duration of at least 2s")
	}
	if c.MaxConcurrent < 1 || c.MaxConcurrent > 256 {
		return fmt.Errorf("PRINTER_RECONNECT_MAX_CONCURRENT must be an integer between 1 and 256")
	}
	return nil
}
func PrinterReconnectConfigFromEnv() (PrinterReconnectConfig, error) {
	c := DefaultPrinterReconnectConfig()
	var err error
	if v, ok := os.LookupEnv("PRINTER_RECONNECT_INTERVAL"); ok {
		c.Interval, err = time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("invalid PRINTER_RECONNECT_INTERVAL: %w", err)
		}
	}
	if v, ok := os.LookupEnv("PRINTER_RECONNECT_MAX_CONCURRENT"); ok {
		c.MaxConcurrent, err = strconv.Atoi(v)
		if err != nil {
			return c, fmt.Errorf("invalid PRINTER_RECONNECT_MAX_CONCURRENT: %w", err)
		}
	}
	return c, c.Validate()
}
func (s *PrinterService) ConfigureReconnect(c PrinterReconnectConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	if s.reconnectQueue != nil {
		return fmt.Errorf("reconnect scheduler already started")
	}
	s.reconnectConfig = c
	return nil
}

type reconnectRequest struct {
	id        uuid.UUID
	automatic bool
	running   bool
	done      chan struct{}
	err       error
}
type reconnectQueue struct {
	ctx       context.Context
	cancel    context.CancelFunc
	config    PrinterReconnectConfig
	pending   map[uuid.UUID]*reconnectRequest
	queue     []*reconnectRequest
	lastStart map[uuid.UUID]time.Time
	wake      chan struct{}
	workers   sync.WaitGroup
}

// All queue operations, including dequeue/start accounting, hold reconnectMu.
// A single pending entry covers queued AND running attempts; there is no expiry
// that could admit a duplicate while a slow transport is still in progress.
func (s *PrinterService) schedulerLocked() *reconnectQueue {
	if s.reconnectQueue != nil {
		return s.reconnectQueue
	}
	c := s.reconnectConfig
	if c.Interval == 0 {
		c = DefaultPrinterReconnectConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	q := &reconnectQueue{ctx: ctx, cancel: cancel, config: c, pending: map[uuid.UUID]*reconnectRequest{}, lastStart: map[uuid.UUID]time.Time{}, wake: make(chan struct{}, 1)}
	s.reconnectQueue = q
	for i := 0; i < c.MaxConcurrent; i++ {
		q.workers.Add(1)
		go s.reconnectWorker(q)
	}
	return q
}
func (s *PrinterService) enqueueReconnect(id uuid.UUID, automatic bool) (*reconnectRequest, error) {
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	q := s.schedulerLocked()
	if err := q.ctx.Err(); err != nil {
		return nil, err
	}
	if r := q.pending[id]; r != nil {
		if !automatic && !r.running {
			r.automatic = false
		}
		return r, nil
	}
	r := &reconnectRequest{id: id, automatic: automatic, done: make(chan struct{})}
	q.pending[id] = r
	q.queue = append(q.queue, r)
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return r, nil
}

// takeReconnect scans FIFO order, skipping cooling printers without blocking
// unrelated printers. Removing before starting and appending retries at the tail
// prevents a repository's first few offline printers from starving the rest.
func takeReconnect(q *reconnectQueue, now time.Time) *reconnectRequest {
	for i, r := range q.queue {
		if last := q.lastStart[r.id]; !last.IsZero() && now.Sub(last) < q.config.Interval {
			continue
		}
		q.queue = append(q.queue[:i], q.queue[i+1:]...)
		q.lastStart[r.id] = now
		r.running = true
		return r
	}
	return nil
}
func (s *PrinterService) reconnectWorker(q *reconnectQueue) {
	defer q.workers.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if q.ctx.Err() != nil {
			return
		}
		s.reconnectMu.Lock()
		r := takeReconnect(q, time.Now())
		s.reconnectMu.Unlock()
		if r == nil {
			select {
			case <-q.ctx.Done():
				return
			case <-q.wake:
			case <-ticker.C:
			}
			continue
		}
		ctx, cancel := context.WithTimeout(q.ctx, q.config.Interval-time.Second)
		err := s.performReconnect(ctx, r.id, r.automatic)
		cancel()
		s.reconnectMu.Lock()
		r.err = err
		delete(q.pending, r.id)
		close(r.done)
		s.reconnectMu.Unlock()
		if err != nil && q.ctx.Err() == nil {
			slog.Debug("printer reconnect failed", "printer_id", r.id, "error", err)
		}
	}
}

// StopReconnect cancels actual I/O and waits for workers; it must precede manager shutdown.
func (s *PrinterService) StopReconnect() {
	s.reconnectMu.Lock()
	q := s.reconnectQueue
	if q != nil {
		q.cancel()
	}
	s.reconnectMu.Unlock()
	if q == nil {
		return
	}
	q.workers.Wait()
	s.reconnectMu.Lock()
	defer s.reconnectMu.Unlock()
	for _, r := range q.queue {
		r.err = context.Canceled
		close(r.done)
		delete(q.pending, r.id)
	}
	q.queue = nil
}
func needsReconnect(state *model.PrinterState) bool {
	return state == nil || state.Status == model.PrinterStatusOffline
}
func (s *PrinterService) printerNeedsAutoReconnect(p *model.Printer, _ time.Time) bool {
	if p.MaintenanceMode || !printer.SupportsConnectionType(p.ConnectionType) {
		return false
	}
	state, err := s.manager.GetState(p.ID)
	return err != nil || needsReconnect(state)
}
func (s *PrinterService) autoReconnectLoop(ctx context.Context) {
	s.reconnectMu.Lock()
	q := s.schedulerLocked()
	s.reconnectMu.Unlock()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ctx.Done():
			s.StopReconnect()
			return
		case <-ticker.C:
			s.ReconnectOfflinePrinters(ctx)
		}
	}
}
