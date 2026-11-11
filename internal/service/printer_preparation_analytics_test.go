package service

import (
	"testing"
	"time"

	"github.com/Brook-sys/picofarm/internal/repository"
	"github.com/google/uuid"
)

func TestComputePreparationAnalyticsAggregatesJobsAndPhases(t *testing.T) {
	job1 := uuid.New()
	job2 := uuid.New()
	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	records := []repository.PrinterPreparationRecord{
		{JobID: job1, JobName: "part-a.gcode", Phase: "Heating Bed", OccurredAt: base, JobEndedAt: base.Add(70 * time.Minute)},
		{JobID: job1, JobName: "part-a.gcode", Phase: "Homing", OccurredAt: base.Add(5 * time.Minute), JobEndedAt: base.Add(70 * time.Minute)},
		{JobID: job1, JobName: "part-a.gcode", Phase: "Printing", OccurredAt: base.Add(10 * time.Minute), JobEndedAt: base.Add(70 * time.Minute)},
		{JobID: job2, JobName: "part-b.gcode", Phase: "Heating Bed", OccurredAt: base.Add(2 * time.Hour), JobEndedAt: base.Add(2*time.Hour + 40*time.Minute)},
		{JobID: job2, JobName: "part-b.gcode", Phase: "Printing", OccurredAt: base.Add(2*time.Hour + 4*time.Minute), JobEndedAt: base.Add(2*time.Hour + 40*time.Minute)},
	}

	got := computePreparationAnalytics(records, 2)
	if got.MeasuredJobs != 2 || got.TotalJobs != 2 {
		t.Fatalf("jobs = measured %d total %d, want 2/2", got.MeasuredJobs, got.TotalJobs)
	}
	if got.TotalPreparationSeconds != 14*60 {
		t.Fatalf("total preparation = %d, want %d", got.TotalPreparationSeconds, 14*60)
	}
	if got.AvgPreparationSeconds != 7*60 {
		t.Fatalf("avg preparation = %d, want %d", got.AvgPreparationSeconds, 7*60)
	}
	if got.TotalActiveSeconds != 110*60 {
		t.Fatalf("total active = %d, want %d", got.TotalActiveSeconds, 110*60)
	}
	wantPercent := float64(14) / 110 * 100
	if got.PreparationPercent < wantPercent-0.001 || got.PreparationPercent > wantPercent+0.001 {
		t.Fatalf("preparation percent = %f, want %f", got.PreparationPercent, wantPercent)
	}
	if len(got.Phases) != 2 || got.Phases[0].Phase != "Heating Bed" || got.Phases[0].TotalSeconds != 9*60 {
		t.Fatalf("unexpected phase breakdown: %#v", got.Phases)
	}
	if len(got.RecentJobs) != 2 || got.RecentJobs[0].JobID != job2 {
		t.Fatalf("recent jobs not newest first: %#v", got.RecentJobs)
	}
}

func TestComputePreparationAnalyticsExcludesIncompleteJobsFromMeasuredSample(t *testing.T) {
	job := uuid.New()
	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	records := []repository.PrinterPreparationRecord{
		{JobID: job, JobName: "incomplete.gcode", Phase: "Heating Bed", OccurredAt: base, JobEndedAt: base.Add(time.Hour)},
		{JobID: job, JobName: "incomplete.gcode", Phase: "Homing", OccurredAt: base.Add(5 * time.Minute), JobEndedAt: base.Add(time.Hour)},
	}

	got := computePreparationAnalytics(records, 3)
	if got.TotalJobs != 3 || got.MeasuredJobs != 0 || got.DataCoveragePercent != 0 {
		t.Fatalf("unexpected coverage: %#v", got)
	}
	if got.TotalPreparationSeconds != 0 || len(got.RecentJobs) != 0 {
		t.Fatalf("incomplete job must not contribute metrics: %#v", got)
	}
}

func TestComputePreparationAnalyticsNormalizesPrintingMarker(t *testing.T) {
	job := uuid.New()
	base := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	records := []repository.PrinterPreparationRecord{
		{JobID: job, Phase: "Bed Mesh Calibration", OccurredAt: base, JobEndedAt: base.Add(30 * time.Minute)},
		{JobID: job, Phase: "  PRINTING  ", OccurredAt: base.Add(3 * time.Minute), JobEndedAt: base.Add(30 * time.Minute)},
	}

	got := computePreparationAnalytics(records, 1)
	if got.MeasuredJobs != 1 || got.TotalPreparationSeconds != 180 {
		t.Fatalf("printing marker was not normalized: %#v", got)
	}
}
