package report

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepository struct {
	records map[string][]TransactionRecord
	cache   map[string]CacheEntry
}

func (r *memoryRepository) LoadTransactions(_ context.Context, start, _ time.Time) ([]TransactionRecord, error) {
	return r.records[start.Format("2006-01-02")], nil
}

func (r *memoryRepository) FindCache(_ context.Context, period Period, hash, model string) (CacheEntry, bool, error) {
	entry, found := r.cache[cacheKey(period, hash, model)]
	return entry, found, nil
}

func (r *memoryRepository) SaveCache(_ context.Context, period Period, hash, content, model string) error {
	r.cache[cacheKey(period, hash, model)] = CacheEntry{Content: content, Model: model, CreatedAt: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)}
	return nil
}

func cacheKey(period Period, hash, model string) string {
	return period.Type + period.StartDate() + period.EndDate() + hash + model
}

type fakeGenerator struct {
	content string
	err     error
	calls   int
}

func (g *fakeGenerator) Generate(_ context.Context, _ string) (string, error) {
	g.calls++
	return g.content, g.err
}
func (g *fakeGenerator) Model() string { return "openrouter/free" }

func TestServiceCacheDependsOnSummary(t *testing.T) {
	repository := &memoryRepository{records: map[string][]TransactionRecord{
		"2026-08-01": {{Type: "expense", Amount: 100, Source: "manual", CategoryName: "Food"}},
		"2026-07-01": {{Type: "expense", Amount: 90, Source: "manual", CategoryName: "Food"}},
	}, cache: make(map[string]CacheEntry)}
	generator := &fakeGenerator{content: "Insight"}
	now := func() time.Time { return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC) }
	service := NewServiceWithClock(repository, generator, now)

	first, err := service.Generate(context.Background(), Request{Period: "monthly"})
	if err != nil || first.AI.Status != "generated" || generator.calls != 1 {
		t.Fatalf("first = %+v, calls=%d, err=%v", first.AI, generator.calls, err)
	}
	second, err := service.Generate(context.Background(), Request{Period: "monthly"})
	if err != nil || second.AI.Status != "cached" || generator.calls != 1 {
		t.Fatalf("second = %+v, calls=%d, err=%v", second.AI, generator.calls, err)
	}
	repository.records["2026-08-01"] = append(repository.records["2026-08-01"], TransactionRecord{Type: "expense", Amount: 10, Source: "manual", CategoryName: "Food"})
	third, err := service.Generate(context.Background(), Request{Period: "monthly"})
	if err != nil || third.AI.Status != "generated" || generator.calls != 2 {
		t.Fatalf("third = %+v, calls=%d, err=%v", third.AI, generator.calls, err)
	}
}

func TestServiceAIUnavailableReturnsStatistics(t *testing.T) {
	repository := &memoryRepository{records: map[string][]TransactionRecord{
		"2026-08-16": {{Type: "income", Amount: 500, Source: "import"}, {Type: "expense", Amount: 200, Source: "manual", CategoryName: "Food"}},
		"2026-08-15": {},
	}, cache: make(map[string]CacheEntry)}
	service := NewServiceWithClock(repository, &fakeGenerator{err: errors.New("429")}, func() time.Time {
		return time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC)
	})
	response, err := service.Generate(context.Background(), Request{Period: "daily"})
	if err != nil || response.AI.Content != nil || response.AI.Status != "unavailable" || response.Summary.Income != 500 || response.Summary.Expense != 200 {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}

func TestSummaryHashDeterministic(t *testing.T) {
	response := Response{Period: "monthly", StartDate: "2026-08-01", EndDate: "2026-08-16", Summary: Summary{Expense: 100}}
	first, err := SummaryHash(response)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SummaryHash(response)
	if err != nil || first != second {
		t.Fatalf("hashes %q %q err=%v", first, second, err)
	}
}
