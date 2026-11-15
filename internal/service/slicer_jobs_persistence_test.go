package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestSlicerJobsFinalPersistenceFailureDoesNotReimport(t *testing.T) {
	settings := &jobMemorySettings{}
	imported := make(chan struct{})
	finish := make(chan struct{})
	s, err := newSlicerJobsService(context.Background(), settings, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) {
		close(imported)
		<-finish
		return &SlicerSliceResult{GCode: &model.GCodeLibraryFile{}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Submit(jobRequest(), "one")
	if err != nil {
		t.Fatal(err)
	}
	<-imported
	settings.mu.Lock()
	settings.fail = true
	settings.mu.Unlock()
	close(finish)
	job := waitJob(t, s, first.ID, "succeeded")
	if !job.PersistenceError {
		t.Fatal("missing final persistence warning")
	}
	if _, err := s.Submit(jobRequest(), "two"); !errors.Is(err, ErrSlicerJobsUnavailable) {
		t.Fatal(err)
	}
	s.Close()
	settings.mu.Lock()
	settings.fail = false
	settings.mu.Unlock()
	restarted, err := newSlicerJobsService(context.Background(), settings, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) {
		t.Error("reimported after ambiguous persistence failure")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	waitJob(t, restarted, first.ID, "interrupted")
}
