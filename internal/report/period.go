package report

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidPeriod    = errors.New("period must be daily, weekly, monthly, or custom")
	ErrCustomDates      = errors.New("custom reports require start_date and end_date")
	ErrInvalidDate      = errors.New("dates must use YYYY-MM-DD")
	ErrInvalidDateRange = errors.New("end_date must not be before start_date")
)

var jakartaLocation = mustLocation("Asia/Jakarta")

func BuildPeriod(input Request, now time.Time) (Period, error) {
	periodType := strings.ToLower(strings.TrimSpace(input.Period))
	localNow := now.In(jakartaLocation)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, jakartaLocation)

	switch periodType {
	case PeriodDaily:
		return newPeriod(periodType, today, today.AddDate(0, 0, 1), today.AddDate(0, 0, -1), today), nil
	case PeriodWeekly:
		weekdayOffset := (int(today.Weekday()) + 6) % 7 // Monday is zero.
		start := today.AddDate(0, 0, -weekdayOffset)
		end := today.AddDate(0, 0, 1)
		return newPeriod(periodType, start, end, start.AddDate(0, 0, -7), end.AddDate(0, 0, -7)), nil
	case PeriodMonthly:
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, jakartaLocation)
		end := today.AddDate(0, 0, 1)
		previousStart := start.AddDate(0, -1, 0)
		previousEnd := equivalentPreviousMonthEnd(today)
		return newPeriod(periodType, start, end, previousStart, previousEnd), nil
	case PeriodCustom:
		if strings.TrimSpace(input.StartDate) == "" || strings.TrimSpace(input.EndDate) == "" {
			return Period{}, ErrCustomDates
		}
		start, err := time.ParseInLocation("2006-01-02", input.StartDate, jakartaLocation)
		if err != nil {
			return Period{}, ErrInvalidDate
		}
		end, err := time.ParseInLocation("2006-01-02", input.EndDate, jakartaLocation)
		if err != nil {
			return Period{}, ErrInvalidDate
		}
		if end.Before(start) {
			return Period{}, ErrInvalidDateRange
		}
		endExclusive := end.AddDate(0, 0, 1)
		days := int(endExclusive.Sub(start).Hours() / 24)
		return newPeriod(periodType, start, endExclusive, start.AddDate(0, 0, -days), start), nil
	default:
		return Period{}, ErrInvalidPeriod
	}
}

func newPeriod(periodType string, start, end, previousStart, previousEnd time.Time) Period {
	return Period{Type: periodType, Start: start, EndExclusive: end, PreviousStart: previousStart, PreviousEnd: previousEnd}
}

func equivalentPreviousMonthEnd(today time.Time) time.Time {
	previousMonthStart := time.Date(today.Year(), today.Month()-1, 1, 0, 0, 0, 0, jakartaLocation)
	lastPreviousDay := previousMonthStart.AddDate(0, 1, 0).AddDate(0, 0, -1).Day()
	day := min(today.Day(), lastPreviousDay)
	return time.Date(previousMonthStart.Year(), previousMonthStart.Month(), day, 0, 0, 0, 0, jakartaLocation).AddDate(0, 0, 1)
}

func mustLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return location
}
