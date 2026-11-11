package service

import (
	"sort"
	"strings"

	"github.com/Brook-sys/picofarm/internal/model"
	"github.com/Brook-sys/picofarm/internal/repository"
)

type preparationPhaseAccumulator struct {
	total int
	count int
}

func computePreparationAnalytics(records []repository.PrinterPreparationRecord, totalJobs int) *model.PrinterPreparationAnalytics {
	result := &model.PrinterPreparationAnalytics{
		TotalJobs: totalJobs, Phases: []model.PrinterPreparationPhase{}, RecentJobs: []model.PrinterPreparationJob{},
	}
	byJob := make(map[string][]repository.PrinterPreparationRecord)
	for _, record := range records {
		byJob[record.JobID.String()] = append(byJob[record.JobID.String()], record)
	}
	phaseTotals := make(map[string]*preparationPhaseAccumulator)
	for _, jobRecords := range byJob {
		sort.Slice(jobRecords, func(i, j int) bool { return jobRecords[i].OccurredAt.Before(jobRecords[j].OccurredAt) })
		printingIndex := -1
		for i, record := range jobRecords {
			if strings.EqualFold(strings.TrimSpace(record.Phase), "printing") {
				printingIndex = i
				break
			}
		}
		if printingIndex <= 0 {
			continue
		}
		start, printingStarted, ended := jobRecords[0].OccurredAt, jobRecords[printingIndex].OccurredAt, jobRecords[printingIndex].JobEndedAt
		if !printingStarted.After(start) || !ended.After(start) {
			continue
		}
		preparationSeconds, activeSeconds := int(printingStarted.Sub(start).Seconds()), int(ended.Sub(start).Seconds())
		if preparationSeconds <= 0 || activeSeconds <= 0 || preparationSeconds > activeSeconds {
			continue
		}
		for i := 0; i < printingIndex; i++ {
			phaseStart, phaseEnd := jobRecords[i].OccurredAt, printingStarted
			if i+1 < len(jobRecords) {
				phaseEnd = jobRecords[i+1].OccurredAt
			}
			seconds, phase := int(phaseEnd.Sub(phaseStart).Seconds()), strings.TrimSpace(jobRecords[i].Phase)
			if seconds <= 0 || phase == "" {
				continue
			}
			acc := phaseTotals[phase]
			if acc == nil {
				acc = &preparationPhaseAccumulator{}
				phaseTotals[phase] = acc
			}
			acc.total += seconds
			acc.count++
		}
		result.MeasuredJobs++
		result.TotalPreparationSeconds += preparationSeconds
		result.TotalActiveSeconds += activeSeconds
		result.RecentJobs = append(result.RecentJobs, model.PrinterPreparationJob{
			JobID: jobRecords[0].JobID, JobName: jobRecords[0].JobName, StartedAt: start,
			PreparationSeconds: preparationSeconds, ActiveSeconds: activeSeconds,
			PreparationPercent: float64(preparationSeconds) / float64(activeSeconds) * 100,
		})
	}
	if result.MeasuredJobs > 0 {
		result.AvgPreparationSeconds = result.TotalPreparationSeconds / result.MeasuredJobs
	}
	result.TotalPrintingSeconds = result.TotalActiveSeconds - result.TotalPreparationSeconds
	if result.TotalActiveSeconds > 0 {
		result.PreparationPercent = float64(result.TotalPreparationSeconds) / float64(result.TotalActiveSeconds) * 100
	}
	if result.TotalJobs > 0 {
		result.DataCoveragePercent = float64(result.MeasuredJobs) / float64(result.TotalJobs) * 100
	}
	for phase, acc := range phaseTotals {
		percent := 0.0
		if result.TotalPreparationSeconds > 0 {
			percent = float64(acc.total) / float64(result.TotalPreparationSeconds) * 100
		}
		result.Phases = append(result.Phases, model.PrinterPreparationPhase{Phase: phase, TotalSeconds: acc.total, AverageSeconds: acc.total / acc.count, Occurrences: acc.count, Percent: percent})
	}
	sort.Slice(result.Phases, func(i, j int) bool { return result.Phases[i].TotalSeconds > result.Phases[j].TotalSeconds })
	sort.Slice(result.RecentJobs, func(i, j int) bool { return result.RecentJobs[i].StartedAt.After(result.RecentJobs[j].StartedAt) })
	if len(result.RecentJobs) > 10 {
		result.RecentJobs = result.RecentJobs[:10]
	}
	return result
}
