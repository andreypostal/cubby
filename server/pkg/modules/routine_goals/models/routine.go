package models

import (
	"time"

	"gorm.io/gorm"

	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
)

type Routine struct {
	gorm.Model

	Pinned      bool
	Name        string `gorm:"varchar(100);not null"`
	Description string `gorm:"varchar(250)"`
	Active      bool   `gorm:"not null;default:true"`

	OnMonday    bool
	OnTuesday   bool
	OnWednesday bool
	OnThursday  bool
	OnFriday    bool
	OnSaturday  bool
	OnSunday    bool

	Points int `gorm:"not null;default:0"`

	WorkspaceID uint                 `gorm:"index;not null"`
	Workspace   authModels.Workspace `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// IsScheduledFor tells whether the routine is set to run on the given day's weekday.
func (r *Routine) IsScheduledFor(day time.Time) bool {
	switch day.Weekday() {
	case time.Monday:
		return r.OnMonday
	case time.Tuesday:
		return r.OnTuesday
	case time.Wednesday:
		return r.OnWednesday
	case time.Thursday:
		return r.OnThursday
	case time.Friday:
		return r.OnFriday
	case time.Saturday:
		return r.OnSaturday
	case time.Sunday:
		return r.OnSunday
	default:
		return false
	}
}

// RoutineDay is the registered result of a routine for a single day.
type RoutineDay struct {
	gorm.Model

	RoutineID uint    `gorm:"uniqueIndex:idx_routine_day;not null"`
	Routine   Routine `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	Day    time.Time `gorm:"uniqueIndex:idx_routine_day;not null"`
	Result bool
}
