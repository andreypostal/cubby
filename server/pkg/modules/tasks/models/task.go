package models

import (
	"time"

	"gorm.io/gorm"
)

type RecurrenceType string

const (
	RecurrenceOnce    RecurrenceType = "once"
	RecurrenceDaily   RecurrenceType = "daily"
	RecurrenceWeekly  RecurrenceType = "weekly"
	RecurrenceMonthly RecurrenceType = "monthly"
	RecurrenceAnnual  RecurrenceType = "annual"
)

// BusinessDayStrategy decides what a monthly task does when its day lands on a weekend
type BusinessDayStrategy string

const (
	// BusinessDayNone keeps the date even when it is a Saturday or Sunday
	BusinessDayNone BusinessDayStrategy = "none"
	// BusinessDayPrevious moves back to the previous business day (sunday will hit Friday)
	BusinessDayPrevious BusinessDayStrategy = "previous"
	// BusinessDayNext moves forward to the next business day (Saturday will hit Monday)
	BusinessDayNext BusinessDayStrategy = "next"
	// BusinessDayClosest moves Saturday back to Friday and Sunday to Monday
	BusinessDayClosest BusinessDayStrategy = "closest"
)

// AllWeekdaysMask bitmask for a full week
const AllWeekdaysMask uint8 = 0b1111111

// WeekdayBit maps a time.Weekday to bit inside Task.Weekdays
func WeekdayBit(w time.Weekday) uint8 {
	return 1 << uint(w)
}

type Task struct {
	gorm.Model

	BoardID uint      `gorm:"index;not null"`
	Board   TaskBoard `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Name         string `gorm:"varchar(100);not null"`
	Icon         string `gorm:"varchar(50);not null"`
	Description  string `gorm:"varchar(250)"`
	Instructions string `gorm:"type:text"`
	Pinned       bool

	RecurrenceType RecurrenceType `gorm:"varchar(16);not null"`

	Weekdays uint8

	DayOfMonth  int
	MonthOfYear int

	BusinessDayStrategy BusinessDayStrategy `gorm:"varchar(16);not null;default:previous"`

	StartsOn time.Time  `gorm:"index;not null"`
	EndsOn   *time.Time `gorm:"index"`

	StoppedAt *time.Time

	OriginTaskID *uint `gorm:"index"`
}

func (t *Task) IsStopped() bool {
	return t.StoppedAt != nil
}

func (t *Task) RootID() uint {
	if t.OriginTaskID != nil {
		return *t.OriginTaskID
	}
	return t.ID
}
