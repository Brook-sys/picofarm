package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

const slicerJobsSettingKey = "slicer_jobs_v1"
const SlicerJobsQueueLimit = 8
const SlicerJobsHistoryLimit = 100

var (
	ErrSlicerJobsFull        = errors.New("slicer job queue or history is full")
	ErrSlicerJobNotFound     = errors.New("slicer job not found")
	ErrSlicerJobConflict     = errors.New("operation not allowed in current job state; only queued jobs can be cancelled and only terminal jobs can be removed")
	ErrSlicerJobInvalid      = errors.New("invalid slicer job request or idempotency key")
	ErrSlicerJobsUnavailable = errors.New("slicer jobs unavailable; restart required")
)

type SlicerJob struct {
	ID               string             `json:"id"`
	Status           string             `json:"status"`
	Request          SlicerSliceRequest `json:"request"`
	Result           *SlicerSliceResult `json:"result,omitempty"`
	Error            string             `json:"error,omitempty"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
	IdempotencyKey   string             `json:"idempotency_key"`
	PersistenceError bool               `json:"persistence_error,omitempty"`
}

type slicerJobSettings interface {
	Get(context.Context, string) (*repository.Setting, error)
	Set(context.Context, string, string) error
}

// SlicerJobsService is a single-process, bounded convenience queue, not a durable
// task engine. Use exactly one instance per database. No retries or restart replay:
// SliceSTL imports once, and a crash after import may leave an unlinked result.
// The settings snapshot must not be edited through generic settings endpoints.
type SlicerJobsService struct {
	mu          sync.Mutex
	settings    slicerJobSettings
	slice       func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error)
	jobs        []SlicerJob
	ctx         context.Context
	cancel      context.CancelFunc
	wake        chan struct{}
	done        chan struct{}
	unavailable bool
}

func NewSlicerJobsService(ctx context.Context, settings *SettingsService, slicer *SlicerService) (*SlicerJobsService, error) {
	return newSlicerJobsService(ctx, settings, slicer.SliceSTL)
}

func newSlicerJobsService(parent context.Context, settings slicerJobSettings, slice func(context.Context, SlicerSliceRequest) (*SlicerSliceResult, error)) (*SlicerJobsService, error) {
	ctx, cancel := context.WithCancel(parent)
	s := &SlicerJobsService{settings: settings, slice: slice, ctx: ctx, cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{}), jobs: []SlicerJob{}}
	saved, err := settings.Get(ctx, slicerJobsSettingKey)
	if err != nil {
		cancel()
		return nil, err
	}
	if saved != nil && saved.Value != "" {
		if err := json.Unmarshal([]byte(saved.Value), &s.jobs); err != nil {
			cancel()
			return nil, fmt.Errorf("invalid slicer jobs snapshot")
		}
		if len(s.jobs) > SlicerJobsHistoryLimit {
			cancel()
			return nil, ErrSlicerJobsFull
		}
	}
	for i := range s.jobs {
		if s.jobs[i].Status == "queued" || s.jobs[i].Status == "running" {
			s.jobs[i].Status = "interrupted"
			s.jobs[i].Error = "PicoFarm restarted; remote execution and library result may exist. No automatic retry."
			s.jobs[i].UpdatedAt = time.Now().UTC()
		}
	}
	if err := s.persist(); err != nil {
		cancel()
		return nil, err
	}
	go s.run()
	return s, nil
}

// Close cancels local I/O, not necessarily the remote process, and waits for the
// worker to stop. Call before closing the database on graceful shutdown.
func (s *SlicerJobsService) Close() { s.cancel(); <-s.done }

func (s *SlicerJobsService) persist() error {
	data, err := json.Marshal(s.jobs)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.settings.Set(ctx, slicerJobsSettingKey, string(data))
}

func cloneSlicerJob(job SlicerJob) SlicerJob {
	data, _ := json.Marshal(job)
	var copy SlicerJob
	_ = json.Unmarshal(data, &copy)
	return copy
}

func (s *SlicerJobsService) Submit(req SlicerSliceRequest, key string) (SlicerJob, error) {
	if req.STLFileID == uuid.Nil || len(key) < 1 || len(key) > 128 || (req.ExportType != "" && req.ExportType != "gcode") {
		return SlicerJob{}, ErrSlicerJobInvalid
	}
	quantity, err := normalizeSlicerGeometry(req.Quantity, req.Arrange, req.Orient, req.Rotate, req.RotateX, req.RotateY)
	if err != nil {
		return SlicerJob{}, fmt.Errorf("%w: %s", ErrSlicerJobInvalid, err)
	}
	req.Quantity = quantity
	// Bound persisted overrides and detach caller-owned maps and rotation pointers.
	payload, err := json.Marshal(req)
	if err != nil || len(payload) > 64<<10 {
		return SlicerJob{}, ErrSlicerJobInvalid
	}
	req = SlicerSliceRequest{}
	_ = json.Unmarshal(payload, &req)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.IdempotencyKey == key {
			existing, _ := json.Marshal(job.Request)
			if !bytes.Equal(existing, payload) {
				return SlicerJob{}, ErrSlicerJobInvalid
			}
			return cloneSlicerJob(job), nil
		}
	}
	if s.unavailable || s.ctx.Err() != nil {
		return SlicerJob{}, ErrSlicerJobsUnavailable
	}
	queued := 0
	for _, job := range s.jobs {
		if job.Status == "queued" {
			queued++
		}
	}
	if queued >= SlicerJobsQueueLimit || len(s.jobs) >= SlicerJobsHistoryLimit {
		return SlicerJob{}, ErrSlicerJobsFull
	}
	now := time.Now().UTC()
	job := SlicerJob{ID: uuid.NewString(), Status: "queued", Request: req, IdempotencyKey: key, CreatedAt: now, UpdatedAt: now}
	s.jobs = append(s.jobs, job)
	if err := s.persist(); err != nil {
		s.jobs = s.jobs[:len(s.jobs)-1]
		s.unavailable = true
		return SlicerJob{}, ErrSlicerJobsUnavailable
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return cloneSlicerJob(job), nil
}

func (s *SlicerJobsService) List() []SlicerJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]SlicerJob, 0, len(s.jobs))
	for i := len(s.jobs) - 1; i >= 0; i-- {
		jobs = append(jobs, cloneSlicerJob(s.jobs[i]))
	}
	return jobs
}

func (s *SlicerJobsService) Get(id string) (SlicerJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.ID == id {
			return cloneSlicerJob(job), nil
		}
	}
	return SlicerJob{}, ErrSlicerJobNotFound
}

func (s *SlicerJobsService) Cancel(id string) (SlicerJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return SlicerJob{}, ErrSlicerJobsUnavailable
	}
	for i := range s.jobs {
		job := &s.jobs[i]
		if job.ID != id {
			continue
		}
		if job.Status == "cancelled" {
			return cloneSlicerJob(*job), nil
		}
		if job.Status != "queued" {
			return SlicerJob{}, ErrSlicerJobConflict
		}
		previous := *job
		job.Status = "cancelled"
		job.UpdatedAt = time.Now().UTC()
		if err := s.persist(); err != nil {
			*job = previous
			s.unavailable = true
			return SlicerJob{}, ErrSlicerJobsUnavailable
		}
		return cloneSlicerJob(*job), nil
	}
	return SlicerJob{}, ErrSlicerJobNotFound
}

// Delete forgets a terminal job and its deduplication key, never its G-code.
func (s *SlicerJobsService) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable {
		return ErrSlicerJobsUnavailable
	}
	for i, job := range s.jobs {
		if job.ID != id {
			continue
		}
		if job.Status == "queued" || job.Status == "running" {
			return ErrSlicerJobConflict
		}
		previous := append([]SlicerJob(nil), s.jobs...)
		s.jobs = append(s.jobs[:i:i], s.jobs[i+1:]...)
		if err := s.persist(); err != nil {
			s.jobs = previous
			s.unavailable = true
			return ErrSlicerJobsUnavailable
		}
		return nil
	}
	return ErrSlicerJobNotFound
}

func (s *SlicerJobsService) run() {
	defer close(s.done)
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.wake:
		}
		for s.next() {
		}
	}
}

func (s *SlicerJobsService) next() bool {
	s.mu.Lock()
	if s.unavailable || s.ctx.Err() != nil {
		s.mu.Unlock()
		return false
	}
	index := -1
	for i := range s.jobs {
		if s.jobs[i].Status == "queued" {
			index = i
			break
		}
	}
	if index < 0 {
		s.mu.Unlock()
		return false
	}
	s.jobs[index].Status = "running"
	s.jobs[index].UpdatedAt = time.Now().UTC()
	if err := s.persist(); err != nil {
		s.unavailable = true
		s.jobs[index].PersistenceError = true
		s.mu.Unlock()
		return false
	}
	jobID := s.jobs[index].ID
	req := cloneSlicerJob(s.jobs[index]).Request
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(s.ctx, 60*time.Minute)
	result, err := s.execute(ctx, req)
	cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	// Terminal history may be deleted while this job runs; indices can shift.
	for i := range s.jobs {
		if s.jobs[i].ID == jobID {
			index = i
			break
		}
	}
	job := &s.jobs[index]
	job.UpdatedAt = time.Now().UTC()
	switch {
	case s.ctx.Err() != nil:
		job.Status = "interrupted"
		job.Error = "Local shutdown; remote execution and library result may exist."
	case err != nil:
		// Never persist raw upstream errors: they can contain URLs or credentials.
		job.Status = "failed"
		job.Error = "Slicing or result import failed; check slicer connectivity and inputs. No automatic retry."
	case result == nil || result.GCode == nil:
		job.Status = "failed"
		job.Error = "Slicer returned no library result."
	default:
		job.Status = "succeeded"
		job.Result = result
	}
	if err := s.persist(); err != nil {
		job.PersistenceError = true
		s.unavailable = true
	}
	return !s.unavailable
}

func (s *SlicerJobsService) execute(ctx context.Context, req SlicerSliceRequest) (result *SlicerSliceResult, err error) {
	defer func() {
		if recover() != nil {
			result = nil
			err = errors.New("slicer worker panic")
		}
	}()
	return s.slice(ctx, req)
}
