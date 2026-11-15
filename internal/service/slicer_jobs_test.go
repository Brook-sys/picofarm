package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

type jobMemorySettings struct {
	mu    sync.Mutex
	value string
	fail  bool
}

func (m *jobMemorySettings) Get(context.Context, string) (*repository.Setting, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &repository.Setting{Value: m.value}, nil
}
func (m *jobMemorySettings) Set(_ context.Context, _, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("disk failure")
	}
	m.value = value
	return nil
}
func jobRequest() SlicerSliceRequest {
	return SlicerSliceRequest{STLFileID: uuid.New(), Printer: "printer", Preset: "preset", Filament: "filament"}
}
func waitJob(t *testing.T, s *SlicerJobsService, id, status string) SlicerJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := s.Get(id)
		if err == nil && job.Status == status {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	job, _ := s.Get(id)
	t.Fatalf("expected %s, got %+v", status, job)
	return job
}
func TestSlicerJobsQueueCancellationAndIdempotency(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls atomic.Int32
	s, err := newSlicerJobsService(context.Background(), &jobMemorySettings{}, func(ctx context.Context, _ SlicerSliceRequest) (*SlicerSliceResult, error) {
		calls.Add(1)
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &SlicerSliceResult{GCode: &model.GCodeLibraryFile{}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	req := jobRequest()
	first, err := s.Submit(req, "first")
	if err != nil {
		t.Fatal(err)
	}
	<-started
	duplicate, err := s.Submit(req, "first")
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("duplicate: %+v %v", duplicate, err)
	}
	changed := req
	changed.Printer = "different"
	if _, err := s.Submit(changed, "first"); !errors.Is(err, ErrSlicerJobInvalid) {
		t.Fatal(err)
	}
	if _, err := s.Cancel(first.ID); !errors.Is(err, ErrSlicerJobConflict) {
		t.Fatal(err)
	}
	queued, err := s.Submit(jobRequest(), "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cancel(queued.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Cancel(queued.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitJob(t, s, first.ID, "succeeded")
	if calls.Load() != 1 {
		t.Fatalf("calls=%d", calls.Load())
	}
}
func TestSlicerJobsConcurrentSubmissionsBounded(t *testing.T) {
	var active, maximum atomic.Int32
	s, err := newSlicerJobsService(context.Background(), &jobMemorySettings{}, func(ctx context.Context, _ SlicerSliceRequest) (*SlicerSliceResult, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			old := maximum.Load()
			if old >= n || maximum.CompareAndSwap(old, n) {
				break
			}
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Submit(jobRequest(), uuid.NewString())
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrSlicerJobsFull) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() > SlicerJobsQueueLimit+1 {
		t.Fatal("queue exceeded capacity")
	}
	if maximum.Load() > 1 {
		t.Fatal("concurrent slicing")
	}
}
func TestSlicerJobsRestartNeverReplays(t *testing.T) {
	req := jobRequest()
	data, _ := json.Marshal([]SlicerJob{{ID: "one", Status: "queued", Request: req}, {ID: "two", Status: "running", Request: req}, {ID: "three", Status: "succeeded", Request: req, Result: &SlicerSliceResult{GCode: &model.GCodeLibraryFile{}}}})
	var calls atomic.Int32
	s, err := newSlicerJobsService(context.Background(), &jobMemorySettings{value: string(data)}, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) { calls.Add(1); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	waitJob(t, s, "one", "interrupted")
	waitJob(t, s, "two", "interrupted")
	waitJob(t, s, "three", "succeeded")
	if calls.Load() != 0 {
		t.Fatal("replayed job")
	}
}
func TestSlicerJobsFailureAndPanicRecover(t *testing.T) {
	var calls atomic.Int32
	s, err := newSlicerJobsService(context.Background(), &jobMemorySettings{}, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) {
		if calls.Add(1) == 1 {
			panic("secret upstream URL")
		}
		return nil, errors.New("secret upstream URL")
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	one, _ := s.Submit(jobRequest(), "one")
	two, _ := s.Submit(jobRequest(), "two")
	for _, id := range []string{one.ID, two.ID} {
		job := waitJob(t, s, id, "failed")
		if job.Error == "secret upstream URL" {
			t.Fatal("leaked upstream error")
		}
	}
}
func TestSlicerJobsPersistenceFailureBlocksWork(t *testing.T) {
	settings := &jobMemorySettings{}
	var calls atomic.Int32
	s, err := newSlicerJobsService(context.Background(), settings, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) { calls.Add(1); return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	settings.mu.Lock()
	settings.fail = true
	settings.mu.Unlock()
	if _, err := s.Submit(jobRequest(), "one"); !errors.Is(err, ErrSlicerJobsUnavailable) {
		t.Fatal(err)
	}
	if len(s.List()) != 0 || calls.Load() != 0 {
		t.Fatal("unpersisted work executed")
	}
}
func TestSlicerJobsIndependentOfRequestAndSnapshots(t *testing.T) {
	s, err := newSlicerJobsService(context.Background(), &jobMemorySettings{}, func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error) {
		return &SlicerSliceResult{GCode: &model.GCodeLibraryFile{}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	req := jobRequest()
	req.Overrides = map[string]map[string]any{"a": {"b": "original"}}
	job, err := s.Submit(req, "one")
	if err != nil {
		t.Fatal(err)
	}
	req.Overrides["a"]["b"] = "mutated"
	job.Request.Overrides["a"]["b"] = "mutated again"
	saved := waitJob(t, s, job.ID, "succeeded")
	if saved.Request.Overrides["a"]["b"] != "original" {
		t.Fatal("caller mutated stored request")
	}
}
