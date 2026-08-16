package report

import (
	"testing"
	"time"
)

func TestBuildPeriodAsiaJakarta(t *testing.T) {
	tests := []struct {
		name     string
		input    Request
		now      time.Time
		start    string
		end      string
		prevFrom string
		prevTo   string
	}{
		{"daily uses Jakarta date", Request{Period: "daily"}, time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC), "2026-08-16", "2026-08-16", "2026-08-15", "2026-08-15"},
		{"weekly is Monday through today", Request{Period: "weekly"}, time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC), "2026-08-10", "2026-08-16", "2026-08-03", "2026-08-09"},
		{"monthly compares equivalent month to date", Request{Period: "monthly"}, time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC), "2026-08-01", "2026-08-16", "2026-07-01", "2026-07-16"},
		{"month end clamps previous month", Request{Period: "monthly"}, time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC), "2026-03-01", "2026-03-31", "2026-02-01", "2026-02-28"},
		{"custom compares immediately previous window", Request{Period: "custom", StartDate: "2026-08-03", EndDate: "2026-08-05"}, time.Now(), "2026-08-03", "2026-08-05", "2026-07-31", "2026-08-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			period, err := BuildPeriod(tt.input, tt.now)
			if err != nil {
				t.Fatal(err)
			}
			if period.StartDate() != tt.start || period.EndDate() != tt.end || period.PreviousStart.Format("2006-01-02") != tt.prevFrom || period.PreviousEnd.AddDate(0, 0, -1).Format("2006-01-02") != tt.prevTo {
				t.Fatalf("period = %s-%s, previous %s-%s", period.StartDate(), period.EndDate(), period.PreviousStart.Format("2006-01-02"), period.PreviousEnd.AddDate(0, 0, -1).Format("2006-01-02"))
			}
		})
	}
}

func TestBuildPeriodValidation(t *testing.T) {
	for _, input := range []Request{{Period: "yearly"}, {Period: "custom"}, {Period: "custom", StartDate: "x", EndDate: "2026-08-01"}, {Period: "custom", StartDate: "2026-08-02", EndDate: "2026-08-01"}} {
		if _, err := BuildPeriod(input, time.Now()); err == nil {
			t.Fatalf("expected error for %+v", input)
		}
	}
}
