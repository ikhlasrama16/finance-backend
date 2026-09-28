package report

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type v2Repository interface {
	LoadAggregatedStatistics(context.Context, time.Time, time.Time) (Statistics, error)
	CreateAIJob(context.Context, AIJob) (AIJob, bool, error)
	FindAIJob(context.Context, string) (AIJob, bool, error)
}

type V2Service struct {
	repository v2Repository
	generator  Generator
	now        func() time.Time
}

func NewV2Service(repository v2Repository, generator Generator) *V2Service {
	return &V2Service{repository: repository, generator: generator, now: time.Now}
}

func (s *V2Service) Statistics(ctx context.Context, input StatisticsRequest) (StatisticsResponse, error) {
	resolved, err := resolveRequest(input, s.now())
	if err != nil {
		return StatisticsResponse{}, err
	}
	current, err := s.repository.LoadAggregatedStatistics(ctx, resolved.start, resolved.endExclusive)
	if err != nil {
		return StatisticsResponse{}, err
	}
	response := StatisticsResponse{
		Range: resolved.dateRange, ComparisonMode: resolved.comparisonMode, Summary: current.Summary,
		ExpenseByCategory: current.ExpenseByCategory, IncomeByCategory: current.IncomeByCategory,
		TopMerchants: current.TopMerchants, TopIncomeSources: current.TopIncomeSources,
	}
	if resolved.comparison != nil {
		previous, err := s.repository.LoadAggregatedStatistics(ctx, resolved.previousStart, resolved.previousEnd)
		if err != nil {
			return StatisticsResponse{}, err
		}
		comparison := BuildComparison(current, previous)
		response.ComparisonRange = resolved.comparison
		response.Comparison = &comparison
	}
	hash, err := snapshotHash(response)
	if err != nil {
		return StatisticsResponse{}, err
	}
	response.SnapshotHash = hash
	return response, nil
}

func (s *V2Service) CreateAIJob(ctx context.Context, input CreateAIJobRequest) (AIJob, bool, error) {
	statistics, err := s.Statistics(ctx, StatisticsRequest{StartDate: input.StartDate, EndDate: input.EndDate, Comparison: input.Comparison})
	if err != nil {
		return AIJob{}, false, err
	}
	if strings.TrimSpace(input.SnapshotHash) != "" && input.SnapshotHash != statistics.SnapshotHash {
		return AIJob{}, false, ErrSnapshotStale
	}
	snapshot, err := json.Marshal(statistics)
	if err != nil {
		return AIJob{}, false, fmt.Errorf("encode AI report snapshot: %w", err)
	}
	prompt := "Berikut snapshot statistik keuangan yang harus Anda jelaskan. Gunakan hanya data ini:\n" + string(snapshot)
	model := s.model()
	inputHash := hashStrings(promptVersion, model, statistics.SnapshotHash)
	var comparisonStart, comparisonEnd *string
	if statistics.ComparisonRange != nil {
		comparisonStart, comparisonEnd = &statistics.ComparisonRange.StartDate, &statistics.ComparisonRange.EndDate
	}
	job, existed, err := s.repository.CreateAIJob(ctx, AIJob{
		ID: newJobID(), Status: "queued", SnapshotHash: statistics.SnapshotHash, Model: model,
		InputHash: inputHash, Prompt: prompt, Snapshot: string(snapshot), Attempts: 0, MaxAttempts: 3,
		RangeStart: statistics.Range.StartDate, RangeEnd: statistics.Range.EndDate, ComparisonMode: statistics.ComparisonMode,
		ComparisonStart: comparisonStart, ComparisonEnd: comparisonEnd,
	})
	if err != nil {
		return AIJob{}, false, err
	}
	return job, existed, nil
}

func (s *V2Service) GetAIJob(ctx context.Context, id string) (AIJob, bool, error) {
	return s.repository.FindAIJob(ctx, id)
}

func (s *V2Service) model() string {
	if s.generator == nil || s.generator.Model() == "" {
		return defaultModel
	}
	return s.generator.Model()
}

func snapshotHash(value StatisticsResponse) (string, error) {
	value.SnapshotHash = ""
	encoded, err := json.Marshal(struct {
		Version string             `json:"version"`
		Value   StatisticsResponse `json:"value"`
	}{Version: "statistics-v2", Value: value})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func hashStrings(values ...string) string {
	digest := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return hex.EncodeToString(digest[:])
}

func newJobID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic("secure random source unavailable")
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func resolveRequest(input StatisticsRequest, now time.Time) (resolvedRequest, error) {
	if strings.TrimSpace(input.StartDate) == "" || strings.TrimSpace(input.EndDate) == "" {
		return resolvedRequest{}, ErrRangeDates
	}
	start, err := time.ParseInLocation("2006-01-02", input.StartDate, jakartaLocation)
	if err != nil {
		return resolvedRequest{}, ErrInvalidDate
	}
	end, err := time.ParseInLocation("2006-01-02", input.EndDate, jakartaLocation)
	if err != nil {
		return resolvedRequest{}, ErrInvalidDate
	}
	if end.Before(start) {
		return resolvedRequest{}, ErrInvalidDateRange
	}
	today := now.In(jakartaLocation)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, jakartaLocation)
	if end.After(today) {
		return resolvedRequest{}, ErrFutureDate
	}
	resolved := resolvedRequest{dateRange: DateRange{StartDate: start.Format("2006-01-02"), EndDate: end.Format("2006-01-02")}, start: start, endExclusive: end.AddDate(0, 0, 1)}
	mode := strings.TrimSpace(input.Comparison.Mode)
	if mode == "" {
		mode = ComparisonPreviousEquivalent
	}
	resolved.comparisonMode = mode
	switch mode {
	case ComparisonNone:
		return resolved, nil
	case ComparisonPreviousEquivalent:
		days := int(resolved.endExclusive.Sub(start).Hours() / 24)
		resolved.previousStart, resolved.previousEnd = start.AddDate(0, 0, -days), start
	case ComparisonPreviousCalendarWeek:
		if start.Weekday() != time.Monday || end.Sub(start).Hours() != 6*24 {
			return resolvedRequest{}, ErrComparisonMode
		}
		resolved.previousStart, resolved.previousEnd = start.AddDate(0, 0, -7), start
	case ComparisonPreviousCalendarMonth:
		if start.Day() != 1 || end.AddDate(0, 0, 1).Day() != 1 {
			return resolvedRequest{}, ErrComparisonMode
		}
		resolved.previousStart = start.AddDate(0, -1, 0)
		resolved.previousEnd = start
	case ComparisonCustom:
		if strings.TrimSpace(input.Comparison.StartDate) == "" || strings.TrimSpace(input.Comparison.EndDate) == "" {
			return resolvedRequest{}, ErrComparisonDates
		}
		previousStart, err := time.ParseInLocation("2006-01-02", input.Comparison.StartDate, jakartaLocation)
		if err != nil {
			return resolvedRequest{}, ErrInvalidDate
		}
		previousEnd, err := time.ParseInLocation("2006-01-02", input.Comparison.EndDate, jakartaLocation)
		if err != nil {
			return resolvedRequest{}, ErrInvalidDate
		}
		if previousEnd.Before(previousStart) {
			return resolvedRequest{}, ErrInvalidDateRange
		}
		resolved.previousStart, resolved.previousEnd = previousStart, previousEnd.AddDate(0, 0, 1)
	default:
		return resolvedRequest{}, ErrComparisonMode
	}
	comparison := DateRange{StartDate: resolved.previousStart.Format("2006-01-02"), EndDate: resolved.previousEnd.AddDate(0, 0, -1).Format("2006-01-02")}
	resolved.comparison = &comparison
	return resolved, nil
}
