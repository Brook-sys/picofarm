package appruntime

import (
	"context"
	"testing"
	"time"
)

func TestRunBoundedStageFast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	runBoundedStage("fast", ctx.Done(), func() { close(done) })
	select {
	case <-done:
	default:
		t.Fatal("fast stage never ran")
	}
}

func TestRunBoundedStageStuck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	ran := make(chan struct{})
	// The stage must be abandoned, not waited on: this is what keeps a stuck
	// slicer job or device disconnect from hanging shutdown forever.
	runBoundedStage("stuck", ctx.Done(), func() {
		close(ran)
		<-make(chan struct{})
	})
	select {
	case <-ran:
	default:
		t.Fatal("stuck stage never started")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("bounded stage blocked shutdown for %v", elapsed)
	}
}
