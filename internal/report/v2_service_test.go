package report

import (
	"context"
	"testing"
	"time"
)

type v2MemoryRepository struct {
	statistics map[string]Statistics
	jobs       map[string]AIJob
}

func (r *v2MemoryRepository) LoadAggregatedStatistics(_ context.Context, start, _ time.Time) (Statistics, error) {
	return r.statistics[start.Format("2006-01-02")], nil
}

func (r *v2MemoryRepository) CreateAIJob(_ context.Context, job AIJob) (AIJob, bool, error) {
	if existing, found := r.jobs[job.InputHash]; found {
		return existing, true, nil
	}
	job.CreatedAt = time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	r.jobs[job.InputHash] = job
	return job, false, nil
}

func (r *v2MemoryRepository) FindAIJob(_ context.Context, id string) (AIJob, bool, error) {
	for _, job := range r.jobs {
		if job.ID == id {
			return job, true, nil
		}
	}
	return AIJob{}, false, nil
}

func TestV2StatisticsUsesAnyHistoricalRangeAndDataSnapshot(t *testing.T) {
	repository := &v2MemoryRepository{statistics: map[string]Statistics{
		"2026-09-05": {Summary: Summary{Expense: 10}, ExpenseByCategory: []CategoryTotal{}, TopMerchants: []MerchantTotal{}},
		"2026-09-06": {Summary: Summary{Expense: 20}, ExpenseByCategory: []CategoryTotal{}, TopMerchants: []MerchantTotal{}},
	}, jobs: map[string]AIJob{}}
	service := NewV2Service(repository, &fakeGenerator{})
	service.now = func() time.Time { return time.Date(2026, 9, 6, 18, 0, 0, 0, jakartaLocation) }
	first, err := service.Statistics(context.Background(), StatisticsRequest{StartDate: "2026-09-06", EndDate: "2026-09-06", Comparison: ComparisonInput{Mode: ComparisonPreviousEquivalent}})
	if err != nil || first.Range.StartDate != "2026-09-06" || first.ComparisonRange == nil || first.ComparisonRange.StartDate != "2026-09-05" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	firstHash := first.SnapshotHash
	repository.statistics["2026-09-06"] = Statistics{Summary: Summary{Expense: 45}, ExpenseByCategory: []CategoryTotal{}, TopMerchants: []MerchantTotal{}}
	second, err := service.Statistics(context.Background(), StatisticsRequest{StartDate: "2026-09-06", EndDate: "2026-09-06"})
	if err != nil || second.SnapshotHash == firstHash || second.Summary.Expense != 45 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestV2CreateJobRejectsStaleSnapshotAndDeduplicates(t *testing.T) {
	repository := &v2MemoryRepository{statistics: map[string]Statistics{
		"2026-09-05": {ExpenseByCategory: []CategoryTotal{}, TopMerchants: []MerchantTotal{}},
	}, jobs: map[string]AIJob{}}
	service := NewV2Service(repository, &fakeGenerator{})
	service.now = func() time.Time { return time.Date(2026, 9, 6, 18, 0, 0, 0, jakartaLocation) }
	if _, _, err := service.CreateAIJob(context.Background(), CreateAIJobRequest{StartDate: "2026-09-05", EndDate: "2026-09-05", SnapshotHash: "stale"}); err != ErrSnapshotStale {
		t.Fatalf("err=%v", err)
	}
	statistics, err := service.Statistics(context.Background(), StatisticsRequest{StartDate: "2026-09-05", EndDate: "2026-09-05"})
	if err != nil {
		t.Fatal(err)
	}
	first, existed, err := service.CreateAIJob(context.Background(), CreateAIJobRequest{StartDate: "2026-09-05", EndDate: "2026-09-05", SnapshotHash: statistics.SnapshotHash})
	if err != nil || existed {
		t.Fatalf("first=%+v existed=%t err=%v", first, existed, err)
	}
	second, existed, err := service.CreateAIJob(context.Background(), CreateAIJobRequest{StartDate: "2026-09-05", EndDate: "2026-09-05", SnapshotHash: statistics.SnapshotHash})
	if err != nil || !existed || second.ID != first.ID {
		t.Fatalf("second=%+v existed=%t err=%v", second, existed, err)
	}
}
