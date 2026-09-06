package report

import (
	"errors"
	"time"
)

const (
	ComparisonNone                  = "none"
	ComparisonPreviousEquivalent    = "previous_equivalent"
	ComparisonPreviousCalendarWeek  = "previous_calendar_week"
	ComparisonPreviousCalendarMonth = "previous_calendar_month"
	ComparisonCustom                = "custom"
)

var (
	ErrRangeDates      = errors.New("start_date and end_date are required")
	ErrFutureDate      = errors.New("end_date must not be in the future")
	ErrComparisonMode  = errors.New("invalid comparison mode")
	ErrComparisonDates = errors.New("custom comparison requires start_date and end_date")
	ErrSnapshotStale   = errors.New("report data has changed; refresh statistics before generating AI analysis")
)

type DateRange struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type ComparisonInput struct {
	Mode      string `json:"mode"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

type StatisticsRequest struct {
	StartDate  string          `json:"start_date"`
	EndDate    string          `json:"end_date"`
	Comparison ComparisonInput `json:"comparison"`
}

type StatisticsResponse struct {
	Range             DateRange       `json:"range"`
	ComparisonRange   *DateRange      `json:"comparison_range,omitempty"`
	ComparisonMode    string          `json:"comparison_mode"`
	Summary           Summary         `json:"summary"`
	ExpenseByCategory []CategoryTotal `json:"expense_by_category"`
	TopMerchants      []MerchantTotal `json:"top_merchants"`
	Comparison        *Comparison     `json:"comparison,omitempty"`
	SnapshotHash      string          `json:"snapshot_hash"`
}

type CreateAIJobRequest struct {
	StartDate    string          `json:"start_date"`
	EndDate      string          `json:"end_date"`
	Comparison   ComparisonInput `json:"comparison"`
	SnapshotHash string          `json:"snapshot_hash"`
}

type AIJob struct {
	ID              string     `json:"id"`
	InputHash       string     `json:"-"`
	Status          string     `json:"status"`
	SnapshotHash    string     `json:"snapshot_hash"`
	Model           string     `json:"model"`
	Content         *string    `json:"content,omitempty"`
	ErrorCode       string     `json:"error_code,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Prompt          string     `json:"-"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"max_attempts"`
	RangeStart      string     `json:"-"`
	RangeEnd        string     `json:"-"`
	ComparisonMode  string     `json:"-"`
	ComparisonStart *string    `json:"-"`
	ComparisonEnd   *string    `json:"-"`
	Snapshot        string     `json:"-"`
	Created         bool       `json:"-"`
}

type resolvedRequest struct {
	dateRange      DateRange
	comparisonMode string
	comparison     *DateRange
	start          time.Time
	endExclusive   time.Time
	previousStart  time.Time
	previousEnd    time.Time
}
