package dto

import "time"

type CreateRoutineReq struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description" validate:"max=250"`
	Pinned      bool   `json:"pinned"`

	OnMonday    bool `json:"on_monday"`
	OnTuesday   bool `json:"on_tuesday"`
	OnWednesday bool `json:"on_wednesday"`
	OnThursday  bool `json:"on_thursday"`
	OnFriday    bool `json:"on_friday"`
	OnSaturday  bool `json:"on_saturday"`
	OnSunday    bool `json:"on_sunday"`
}

type UpdateRoutineReq struct {
	Name        string `json:"name" validate:"required,min=3,max=100"`
	Description string `json:"description" validate:"max=250"`
	Pinned      bool   `json:"pinned"`
	Active      bool   `json:"active"`

	OnMonday    bool `json:"on_monday"`
	OnTuesday   bool `json:"on_tuesday"`
	OnWednesday bool `json:"on_wednesday"`
	OnThursday  bool `json:"on_thursday"`
	OnFriday    bool `json:"on_friday"`
	OnSaturday  bool `json:"on_saturday"`
	OnSunday    bool `json:"on_sunday"`
}

// RoutinePayload is the public representation of a routine
type RoutinePayload struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Pinned      bool      `json:"pinned"`
	Active      bool      `json:"active"`
	Points      int       `json:"points"`
	CreatedAt   time.Time `json:"created_at"`

	OnMonday    bool `json:"on_monday"`
	OnTuesday   bool `json:"on_tuesday"`
	OnWednesday bool `json:"on_wednesday"`
	OnThursday  bool `json:"on_thursday"`
	OnFriday    bool `json:"on_friday"`
	OnSaturday  bool `json:"on_saturday"`
	OnSunday    bool `json:"on_sunday"`
}

type WorkspaceRoutinesResp struct {
	Routines []RoutinePayload `json:"routines"`
}

// DayStatusPayload is the status of a single day for a routine, used in the month view.
type DayStatusPayload struct {
	Day    string `json:"day"`
	Status string `json:"status"`
}

type RoutineMonthResp struct {
	RoutineID uint               `json:"routine_id"`
	Year      int                `json:"year"`
	Month     int                `json:"month"`
	Days      []DayStatusPayload `json:"days"`
}

type MarkDayReq struct {
	Achieved bool `json:"achieved"`
}

type MarkDayResp struct {
	Routine RoutinePayload   `json:"routine"`
	Day     DayStatusPayload `json:"day"`
}
