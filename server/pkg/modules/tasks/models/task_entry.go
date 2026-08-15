package models

import (
	"time"

	"gorm.io/gorm"
)

type EntryStatus string

const (
	// EntryCompleted means the user did the task that day
	EntryCompleted EntryStatus = "completed"
	// EntrySkipped means the user consciously chose not to do it
	EntrySkipped EntryStatus = "skipped"
	// EntryFailed means the day passed without the task being done or that
	// the user was not able to achieve it (conscious action as well)
	EntryFailed EntryStatus = "failed"
)

func IsValidEntryStatus(s EntryStatus) bool {
	switch s {
	case EntryCompleted, EntrySkipped, EntryFailed:
		return true
	default:
		return false
	}
}

type TaskEntry struct {
	gorm.Model

	TaskID uint `gorm:"uniqueIndex:idx_task_day;not null"`
	Task   Task `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Day time.Time `gorm:"uniqueIndex:idx_task_day;not null"`

	Status EntryStatus `gorm:"varchar(16);not null"`
	Note   string      `gorm:"varchar(250)"`
}
