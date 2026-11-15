package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/google/uuid"
)

// Atomic counters allow observation even if a regression launches asynchronous work.
type reconnectQueueSpy struct {
	findCalls  atomic.Int32
	startCalls atomic.Int32
	item       *model.QueueItem
}

func (s *reconnectQueueSpy) FindNextReadyForPrinter(context.Context, uuid.UUID) (*model.QueueItem, error) {
	s.findCalls.Add(1)
	return s.item, nil
}

func (s *reconnectQueueSpy) Start(context.Context, uuid.UUID) error {
	s.startCalls.Add(1)
	return nil
}

func TestDispatcherReconnectDoesNotDispatch(t *testing.T) {
	for _, previous := range []struct {
		name   string
		status *model.PrinterState
	}{
		{name: "offline_to_idle", status: &model.PrinterState{Status: model.PrinterStatusOffline}},
		{name: "nil_to_idle"},
	} {
		t.Run(previous.name, func(t *testing.T) {
			dispatcher, repos, _ := setupDispatcherTestEnv(t)
			ctx := context.Background()
			printerObj := createTestPrinter(t, repos, "Reconnect test printer")
			job := createTestPrintJob(t, repos, model.PrintJobStatusQueued, 0, true)
			spy := &reconnectQueueSpy{item: &model.QueueItem{
				ID: uuid.New(), AssignedPrinterID: &printerObj.ID, FileName: "queued.gcode",
			}}
			dispatcher.queueSvc = spy
			if err := dispatcher.SetGlobalEnabled(ctx, true); err != nil {
				t.Fatalf("enable global dispatch: %v", err)
			}
			if err := dispatcher.UpdateSettings(ctx, &model.AutoDispatchSettings{
				PrinterID: printerObj.ID, Enabled: true, RequireConfirmation: true, TimeoutMinutes: 30,
			}); err != nil {
				t.Fatalf("enable printer dispatch: %v", err)
			}

			assertNoDispatch := func() {
				t.Helper()
				if got := spy.findCalls.Load(); got != 0 {
					t.Fatalf("reconnect must not look up queue work; got %d calls", got)
				}
				if got := spy.startCalls.Load(); got != 0 {
					t.Fatalf("reconnect must not start queue work; got %d calls", got)
				}
				pending, err := repos.Dispatch.ListPending(ctx)
				if err != nil {
					t.Fatalf("list pending dispatches: %v", err)
				}
				if len(pending) != 0 {
					t.Fatalf("reconnect must leave pending dispatches empty; got %d", len(pending))
				}
			}
			assertNoDispatch()
			oldState := previous.status
			if oldState != nil {
				oldState = &model.PrinterState{PrinterID: printerObj.ID, Status: oldState.Status}
			}
			dispatcher.handlePrinterStatusChange(&model.PrinterState{
				PrinterID: printerObj.ID, Status: model.PrinterStatusIdle,
			}, oldState)

			// The callback can launch a goroutine for eligible transitions. Observe a
			// bounded window, not just the synchronous return; this is not proof
			// against arbitrarily delayed asynchronous work.
			deadline := time.Now().Add(200 * time.Millisecond)
			for {
				assertNoDispatch()
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}

			// Positive fixture control: enabled settings and the queued job really
			// can create a dispatch; the negative result is not disabled setup.
			if err := dispatcher.OnPrinterIdle(printerObj.ID); err != nil {
				t.Fatalf("positive control OnPrinterIdle: %v", err)
			}
			pending, err := repos.Dispatch.ListPending(ctx)
			if err != nil {
				t.Fatalf("list positive-control dispatches: %v", err)
			}
			if len(pending) != 1 {
				t.Fatalf("positive control should create one dispatch, got %d", len(pending))
			}
			if pending[0].JobID != job.ID || pending[0].PrinterID != printerObj.ID {
				t.Fatal("positive control dispatch must target the queued job and test printer")
			}
		})
	}
}
