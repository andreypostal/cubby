package service

import (
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
)

// Occurs is the "recurrence engine", it defines if a given task happens on a given day
//
// It will check all our recurrence types (once, daily, weekly or monthly)
// d is normalized so we don't rely on specific timezone
func Occurs(t *models.Task, d time.Time) bool {
	d = core.NormalizeDate(d)

	if t == nil || !inWindow(t, d) {
		return false
	}

	switch t.RecurrenceType {
	case models.RecurrenceOnce:
		return d.Equal(core.NormalizeDate(t.StartsOn))
	case models.RecurrenceDaily:
		return true
	case models.RecurrenceWeekly:
		return t.Weekdays&models.WeekdayBit(d.Weekday()) != 0
	case models.RecurrenceMonthly:
		// Check if this task monthly day (considering weekends and shift strategies)
		// is the same as the one requested
		return MonthlyOccurrenceFor(d.Year(), d.Month(), t.DayOfMonth, t.BusinessDayStrategy).Equal(d)
	case models.RecurrenceAnnual:
		// Same resolution as monthly, but pinned to a single month
		if int(d.Month()) != t.MonthOfYear {
			return false
		}
		return MonthlyOccurrenceFor(d.Year(), d.Month(), t.DayOfMonth, t.BusinessDayStrategy).Equal(d)
	default:
		return false
	}
}

// inWindow reports whether d falls inside the task active window
func inWindow(t *models.Task, d time.Time) bool {
	if d.Before(core.NormalizeDate(t.StartsOn)) {
		return false
	}
	if t.EndsOn != nil && d.After(core.NormalizeDate(*t.EndsOn)) {
		return false
	}
	return true
}

// MonthlyOccurrenceFor resolves the day a monthly task falls on in the
// given month, respecting last day of the month and weekends shift strategies
func MonthlyOccurrenceFor(year int, month time.Month, dayOfMonth int, strategy models.BusinessDayStrategy) time.Time {
	// Ensure we are within month days
	day := dayOfMonth
	if last := core.DaysInMonth(year, month); day > last {
		day = last
	}
	if day < 1 {
		day = 1
	}

	date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	// Shift weekends if necessary
	return shiftToBusinessDay(date, strategy)
}

func shiftToBusinessDay(d time.Time, strategy models.BusinessDayStrategy) time.Time {
	if strategy == models.BusinessDayNone || isBusinessDay(d) {
		return d
	}

	forward := []int{1, -1}
	backward := []int{-1, 1}

	// Define which vector we should use based on the strategy
	var order []int
	switch strategy {
	case models.BusinessDayNext:
		order = forward
	case models.BusinessDayClosest:
		if d.Weekday() == time.Sunday {
			order = forward
		} else {
			order = backward
		}
	default:
		order = backward
	}

	for _, dir := range order {
		if shifted, ok := walkToBusinessDay(d, dir); ok {
			return shifted
		}
	}

	// We should not reach this, but I mean.. just return it
	return d
}

// walkToBusinessDay moves the day based on the direction chosen
// but respecting the month the date belongs to
func walkToBusinessDay(d time.Time, dir int) (time.Time, bool) {
	month := d.Month()
	for !isBusinessDay(d) {
		d = core.AddDays(d, dir)
		if d.Month() != month {
			return time.Time{}, false
		}
	}
	return d, true
}

// isBusinessDay reports whether d is a working day
// this can be used for considering other things in the future like holidays
func isBusinessDay(d time.Time) bool {
	switch d.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	default:
		return true
	}
}

func WeekdaysToMask(days []int) uint8 {
	var mask uint8
	for _, d := range days {
		if d < 0 || d > 6 {
			continue
		}
		mask |= models.WeekdayBit(time.Weekday(d))
	}
	return mask
}

func MaskToWeekdays(mask uint8) []int {
	days := make([]int, 0, 7)
	for d := 0; d <= 6; d++ {
		if mask&models.WeekdayBit(time.Weekday(d)) != 0 {
			days = append(days, d)
		}
	}
	return days
}
