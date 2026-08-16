package report

import (
	"context"
	"time"
)

type repository interface {
	LoadTransactions(context.Context, time.Time, time.Time) ([]TransactionRecord, error)
	FindCache(context.Context, Period, string, string) (CacheEntry, bool, error)
	SaveCache(context.Context, Period, string, string, string) error
}

type Service struct {
	repository repository
	generator  Generator
	now        func() time.Time
}

func NewService(repository repository, generator Generator) *Service {
	return NewServiceWithClock(repository, generator, time.Now)
}

func NewServiceWithClock(repository repository, generator Generator, now func() time.Time) *Service {
	return &Service{repository: repository, generator: generator, now: now}
}

func (s *Service) Generate(ctx context.Context, input Request) (Response, error) {
	period, err := BuildPeriod(input, s.now())
	if err != nil {
		return Response{}, err
	}
	current, err := s.repository.LoadTransactions(ctx, period.Start, period.EndExclusive)
	if err != nil {
		return Response{}, err
	}
	previous, err := s.repository.LoadTransactions(ctx, period.PreviousStart, period.PreviousEnd)
	if err != nil {
		return Response{}, err
	}
	statistics := Calculate(current, period.Start, period.EndExclusive)
	comparison := BuildComparison(statistics, Calculate(previous, period.PreviousStart, period.PreviousEnd))
	response := Response{
		Period: period.Type, StartDate: period.StartDate(), EndDate: period.EndDate(), Summary: statistics.Summary,
		ExpenseByCategory: statistics.ExpenseByCategory, TopMerchants: statistics.TopMerchants, Comparison: comparison,
		AI: AIResult{Status: "unavailable", Model: s.model()},
	}
	summaryHash, err := SummaryHash(response)
	if err != nil {
		return Response{}, err
	}
	if cached, found, cacheErr := s.repository.FindCache(ctx, period, summaryHash, s.model()); cacheErr == nil && found {
		content := cached.Content
		generatedAt := cached.CreatedAt
		response.AI = AIResult{Content: &content, Status: "cached", Model: cached.Model, GeneratedAt: &generatedAt}
		return response, nil
	}
	if s.generator == nil {
		return response, nil
	}
	prompt, err := BuildPrompt(response)
	if err != nil {
		return Response{}, err
	}
	content, err := s.generator.Generate(ctx, prompt)
	if err != nil {
		return response, nil
	}
	generatedAt := s.now().UTC()
	response.AI = AIResult{Content: &content, Status: "generated", Model: s.model(), GeneratedAt: &generatedAt}
	_ = s.repository.SaveCache(ctx, period, summaryHash, content, s.model())
	return response, nil
}

func (s *Service) model() string {
	if s.generator == nil || s.generator.Model() == "" {
		return defaultModel
	}
	return s.generator.Model()
}
