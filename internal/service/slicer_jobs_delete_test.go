package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Brook-sys/picofarm/internal/model"
)

func TestSlicerJobsDeleteTerminalAndReuseKey(t *testing.T) {
	s, err := newSlicerJobsService(context.Background(), &jobMemorySettings{}, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) {
		return &SlicerSliceResult{GCode: &model.GCodeLibraryFile{}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	req := jobRequest()
	first, err := s.Submit(req, "key")
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, s, first.ID, "succeeded")
	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(first.ID); !errors.Is(err, ErrSlicerJobNotFound) {
		t.Fatal(err)
	}
	second, err := s.Submit(req, "key")
	if err != nil || second.ID == first.ID {
		t.Fatalf("key retained after explicit delete: %+v %v", second, err)
	}
}
func TestSlicerJobsDeleteDuringExecutionDoesNotShiftResult(t *testing.T) {
	settings := &jobMemorySettings{}
	started := make(chan struct{}, 2)
	finish := make(chan struct{}, 2)
	s, err := newSlicerJobsService(context.Background(), settings, func(ctx context.Context, _ SlicerSliceRequest) (*SlicerSliceResult, error) {
		started <- struct{}{}
		select {
		case <-finish:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &SlicerSliceResult{GCode: &model.GCodeLibraryFile{}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first, _ := s.Submit(jobRequest(), "one")
	<-started
	finish <- struct{}{}
	waitJob(t, s, first.ID, "succeeded")
	second, _ := s.Submit(jobRequest(), "two")
	<-started
	queued, _ := s.Submit(jobRequest(), "three")
	if err := s.Delete(second.ID); !errors.Is(err, ErrSlicerJobConflict) {
		t.Fatal(err)
	}
	if err := s.Delete(queued.ID); !errors.Is(err, ErrSlicerJobConflict) {
		t.Fatal(err)
	}
	if err := s.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	finish <- struct{}{}
	waitJob(t, s, second.ID, "succeeded")
	<-started
	finish <- struct{}{}
	waitJob(t, s, queued.ID, "succeeded")
	settings.mu.Lock()
	settings.fail = true
	settings.mu.Unlock()
	if err := s.Delete(second.ID); !errors.Is(err, ErrSlicerJobsUnavailable) {
		t.Fatal(err)
	}
	if _, err := s.Get(second.ID); err != nil {
		t.Fatal("lost history after failed persistence")
	}
}
