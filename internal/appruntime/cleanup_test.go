package appruntime

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestCleanupResourcesOrderAndCloseError(t *testing.T) {
	closeErr := errors.New("database close failed")
	var order []string
	err := cleanupResources(func() error {
		order = append(order, "database")
		return closeErr
	}, func() { order = append(order, "worker one") }, func() { order = append(order, "worker two") })
	if !errors.Is(err, closeErr) {
		t.Fatalf("cleanup error = %v, want %v", err, closeErr)
	}
	if want := []string{"worker one", "worker two", "database"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("cleanup order = %v, want %v", order, want)
	}
}

func TestShutdownTimeoutDoesNotAbandonCleanup(t *testing.T) {
	app := &Runtime{stop: make(chan struct{}), done: make(chan struct{})}
	started := make(chan struct{})
	release := make(chan struct{})
	nextStage := make(chan struct{})
	databaseClosed := make(chan struct{})
	closeErr := errors.New("database close failed")
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	// Even a failed assertion releases and joins every test-owned goroutine.
	t.Cleanup(func() {
		unblock()
		awaitCleanupSignal(t, app.Done())
	})
	go func() {
		defer close(app.done)
		<-app.stop
		app.err = cleanupResources(func() error {
			close(databaseClosed)
			return closeErr
		}, func() {
			close(started)
			<-release
		}, func() { close(nextStage) })
	}()

	// An already-expired deadline avoids timing-dependent sleeps while testing
	// the same caller-bound return as a deadline expiring during cleanup.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := app.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v, want deadline exceeded", err)
	}
	awaitCleanupSignal(t, started)
	waitResult := make(chan error, 1)
	go func() { waitResult <- app.Wait() }()
	for name, signal := range map[string]<-chan struct{}{
		"next stage": nextStage, "database close": databaseClosed, "Done": app.Done(),
	} {
		select {
		case <-signal:
			t.Fatalf("%s completed while the first cleanup stage was blocked", name)
		default:
		}
	}
	select {
	case err := <-waitResult:
		t.Fatalf("Wait returned before actual cleanup: %v", err)
	default:
	}

	unblock()
	awaitCleanupSignal(t, app.Done())
	awaitCleanupSignal(t, nextStage)
	awaitCleanupSignal(t, databaseClosed)
	select {
	case err := <-waitResult:
		if !errors.Is(err, closeErr) {
			t.Fatalf("Wait = %v, want database close error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return after cleanup completed")
	}
	if err := app.Shutdown(context.Background()); !errors.Is(err, closeErr) {
		t.Fatalf("repeated Shutdown = %v, want database close error", err)
	}
}

func awaitCleanupSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup signal did not arrive")
	}
}
