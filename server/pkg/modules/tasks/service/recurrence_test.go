package service

import (
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
	"github.com/stretchr/testify/require"
)

func annualTask(month, day int, strategy models.BusinessDayStrategy) *models.Task {
	return &models.Task{
		RecurrenceType:      models.RecurrenceAnnual,
		MonthOfYear:         month,
		DayOfMonth:          day,
		BusinessDayStrategy: strategy,
		StartsOn:            date(2026, time.January, 1),
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestAnnualOccursOnItsDayEveryYear(t *testing.T) {
	task := annualTask(12, 25, models.BusinessDayNone)

	require.True(t, Occurs(task, date(2026, time.December, 25)))
	require.True(t, Occurs(task, date(2027, time.December, 25)))
	require.True(t, Occurs(task, date(2031, time.December, 25)))
}

func TestAnnualIgnoresTheSameDayInOtherMonths(t *testing.T) {
	task := annualTask(12, 25, models.BusinessDayNone)

	require.False(t, Occurs(task, date(2026, time.November, 25)))
	require.False(t, Occurs(task, date(2026, time.December, 24)))
	require.False(t, Occurs(task, date(2026, time.December, 26)))
}

// The weekend shift is the monthly engine's, so an annual task lands on the
// shifted day and not on the nominal one.
func TestAnnualShiftsOffWeekends(t *testing.T) {
	// 2026-08-15 is a Saturday.
	task := annualTask(8, 15, models.BusinessDayPrevious)

	require.False(t, Occurs(task, date(2026, time.August, 15)))
	require.True(t, Occurs(task, date(2026, time.August, 14)))
}

// Day 31 means "last day of the month", so a February annual clamps rather than
// spilling into March.
func TestAnnualClampsToTheMonthLength(t *testing.T) {
	task := annualTask(2, 31, models.BusinessDayNone)

	require.True(t, Occurs(task, date(2026, time.February, 28)))
	// 2028 is a leap year.
	require.True(t, Occurs(task, date(2028, time.February, 29)))
	require.False(t, Occurs(task, date(2028, time.February, 28)))
}

func TestAnnualRespectsItsWindow(t *testing.T) {
	task := annualTask(12, 25, models.BusinessDayNone)
	task.StartsOn = date(2027, time.January, 1)
	ends := date(2028, time.June, 1)
	task.EndsOn = &ends

	require.False(t, Occurs(task, date(2026, time.December, 25)))
	require.True(t, Occurs(task, date(2027, time.December, 25)))
	require.False(t, Occurs(task, date(2028, time.December, 25)))
}

// The lookahead has to be long enough for a rule that fires once a year, which
// is the widest gap any recurrence can have.
func TestNextOccurrenceFindsTheFollowingYear(t *testing.T) {
	task := annualTask(1, 10, models.BusinessDayNone)
	today := date(2026, time.January, 11)

	next := nextOccurrence(task, today)

	require.NotNil(t, next)
	require.Equal(t, core.FormatDate(date(2027, time.January, 10)), core.FormatDate(*next))
}

// A onetime task whose day has passed is what the active listing drops.
func TestExpiredOnetime(t *testing.T) {
	task := &models.Task{RecurrenceType: models.RecurrenceOnce, StartsOn: date(2026, time.August, 1)}
	ends := task.StartsOn
	task.EndsOn = &ends

	today := date(2026, time.August, 13)
	require.True(t, isExpiredOnetime(task, nextOccurrence(task, today)))

	// Its own day still counts as ahead of us: it is due, not gone.
	today = date(2026, time.August, 1)
	require.False(t, isExpiredOnetime(task, nextOccurrence(task, today)))

	// A recurring task never expires this way, even with nothing left to fire.
	daily := &models.Task{RecurrenceType: models.RecurrenceDaily, StartsOn: date(2026, time.August, 1)}
	require.False(t, isExpiredOnetime(daily, nil))
}
