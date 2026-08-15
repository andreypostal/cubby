package core

import (
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

// NormalizeDate truncates to midnight UTC, turning a timestamp into a date-only value
func NormalizeDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseDate reads a YYYY-MM-DD string into a date-only value
func ParseDate(s string) (time.Time, error) {
	d, err := time.Parse(DateLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing date %q: %w", s, err)
	}
	return NormalizeDate(d), nil
}

// FormatDate renders a date-only value back into a YYYY-MM-DD string.
func FormatDate(t time.Time) string {
	return t.UTC().Format(DateLayout)
}

// TodayIn resolves which day it currently is in loc, returned as a date-only
func TodayIn(loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// DaysInMonth tells how many days the given month holds
func DaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// AddDays shifts by n days
func AddDays(d time.Time, n int) time.Time {
	return d.AddDate(0, 0, n)
}
