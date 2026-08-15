package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A timestamp keeps its calendar day, and everything below the day is dropped
func TestNormalizeDateKeepsTheDayAndDropsTheTime(t *testing.T) {
	d := NormalizeDate(time.Date(2026, time.March, 14, 23, 45, 12, 999, time.UTC))

	assert.Equal(t, time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC), d)
}

// A zone east of UTC can already be on the next day, and that is the day kept,
// because the value is converted before it is truncated
func TestNormalizeDateConvertsToUTCBeforeTruncating(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*3600)

	// 2026-03-14 22:00 in Tokyo is still 13:00 UTC of the same day
	sameDay := NormalizeDate(time.Date(2026, time.March, 14, 22, 0, 0, 0, tokyo))
	assert.Equal(t, time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC), sameDay)

	// 2026-03-15 06:00 in Tokyo is still the 14th in UTC
	dayBefore := NormalizeDate(time.Date(2026, time.March, 15, 6, 0, 0, 0, tokyo))
	assert.Equal(t, time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC), dayBefore)
}

func TestParseDateAndFormatDateRoundTrip(t *testing.T) {
	d, err := ParseDate("2026-08-13")
	require.NoError(t, err)
	assert.Equal(t, "2026-08-13", FormatDate(d))
	assert.Equal(t, time.UTC, d.Location())

	// A leap day only parses in a year that has one
	_, err = ParseDate("2026-02-29")
	assert.Error(t, err)

	d, err = ParseDate("2028-02-29")
	require.NoError(t, err)
	assert.Equal(t, "2028-02-29", FormatDate(d))
}

func TestParseDateRejectsAnythingThatIsNotADate(t *testing.T) {
	for _, in := range []string{"", "13/08/2026", "2026-8-13", "2026-08-13T00:00:00Z", "not-a-date"} {
		_, err := ParseDate(in)
		assert.Error(t, err, "parsing %q should have failed", in)
	}
}

// FormatDate reads the day in UTC, matching what NormalizeDate wrote
func TestFormatDateReportsTheUTCDay(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*3600)

	assert.Equal(t, "2026-03-14", FormatDate(time.Date(2026, time.March, 15, 6, 0, 0, 0, tokyo)))
}

// TodayIn answers with the day in loc, but expressed as a UTC date-only value,
// which is what makes it comparable with everything stored
func TestTodayInUsesTheGivenLocation(t *testing.T) {
	now := time.Now().UTC()

	utcToday := TodayIn(time.UTC)
	assert.Equal(t, NormalizeDate(now), utcToday)

	// A nil location is not an error, it falls back to UTC
	assert.Equal(t, utcToday, TodayIn(nil))

	// Far enough east that the day is never behind UTC's, and the answer is
	// still a whole date-only value rather than a shifted timestamp
	ahead := TodayIn(time.FixedZone("far-east", 14*3600))
	assert.Equal(t, time.UTC, ahead.Location())
	assert.Zero(t, ahead.Sub(utcToday)%(24*time.Hour))
	assert.False(t, ahead.Before(utcToday))
}

func TestDaysInMonth(t *testing.T) {
	assert.Equal(t, 31, DaysInMonth(2026, time.January))
	assert.Equal(t, 28, DaysInMonth(2026, time.February))
	assert.Equal(t, 30, DaysInMonth(2026, time.April))
	assert.Equal(t, 31, DaysInMonth(2026, time.December))

	// Leap years are the reason this helper exists
	assert.Equal(t, 29, DaysInMonth(2028, time.February))
	assert.Equal(t, 29, DaysInMonth(2000, time.February))
	assert.Equal(t, 28, DaysInMonth(1900, time.February))
}

func TestAddDaysCrossesMonthAndYearBoundaries(t *testing.T) {
	assert.Equal(t,
		time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		AddDays(time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC), 1),
	)

	assert.Equal(t,
		time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC),
		AddDays(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), -1),
	)

	d := time.Date(2026, time.June, 10, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, d, AddDays(d, 0))
}
