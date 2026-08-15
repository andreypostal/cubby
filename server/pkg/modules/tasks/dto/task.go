package dto

import "time"

type UpdateBoardReq struct {
	Timezone string `json:"timezone" validate:"required,timezone"`
}

type BoardPayload struct {
	ID        uint      `json:"id"`
	Timezone  string    `json:"timezone"`
	Today     string    `json:"today"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateTaskReq struct {
	Name         string `json:"name" validate:"required,min=3,max=100"`
	Icon         string `json:"icon" validate:"required,max=50"`
	Description  string `json:"description" validate:"max=250"`
	Instructions string `json:"instructions" validate:"max=5000"`
	Pinned       bool   `json:"pinned"`

	RecurrenceType string `json:"recurrence_type" validate:"required,oneof=once daily weekly monthly annual"`

	// Weekdays is the "friendly" form of the stored bitmask
	Weekdays []int `json:"weekdays" validate:"required_if=RecurrenceType weekly,dive,min=0,max=6"`

	DayOfMonth int `json:"day_of_month" validate:"min=0,max=31"`

	// MonthOfYear is 1 (January) to 12 (December), annual tasks only
	MonthOfYear int `json:"month_of_year" validate:"min=0,max=12"`

	// BusinessDayStrategy defaults to "previous" when empty.
	BusinessDayStrategy string `json:"business_day_strategy" validate:"omitempty,oneof=none previous next closest"`

	StartsOn string `json:"starts_on" validate:"omitempty,datetime=2006-01-02"`
}

// UpdateTaskReq only metadata, so we don't mess with recurrences
type UpdateTaskReq struct {
	Name         string `json:"name" validate:"required,min=3,max=100"`
	Icon         string `json:"icon" validate:"required,max=50"`
	Description  string `json:"description" validate:"max=250"`
	Instructions string `json:"instructions" validate:"max=5000"`
	Pinned       bool   `json:"pinned"`
}

type ResumeTaskReq struct {
	From string `json:"from" validate:"omitempty,datetime=2006-01-02"`
}

// TaskPayload is the public representation of a task definition
type TaskPayload struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
	Pinned       bool   `json:"pinned"`

	RecurrenceType      string `json:"recurrence_type"`
	Weekdays            []int  `json:"weekdays"`
	DayOfMonth          int    `json:"day_of_month"`
	MonthOfYear         int    `json:"month_of_year"`
	BusinessDayStrategy string `json:"business_day_strategy"`

	StartsOn string  `json:"starts_on"`
	EndsOn   *string `json:"ends_on"`

	Stopped   bool       `json:"stopped"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`

	OriginTaskID *uint `json:"origin_task_id"`

	CreatedAt time.Time `json:"created_at"`
}

// TaskSummaryPayload is a task definition plus the derived bits the workspace
// overview needs, so a client can show what is coming and what can be acted on
// without asking for a day view per task.
type TaskSummaryPayload struct {
	TaskPayload

	// NextOccurrence is the first day from today onwards this task fires on,
	// or nil when it has none left (stopped, or a onetime already past).
	NextOccurrence *string `json:"next_occurrence"`

	// Resumable reports whether resuming would actually produce a new run: the
	// task is stopped, recurring, and no sibling has taken over from it yet.
	Resumable bool `json:"resumable"`
}

type TaskListResp struct {
	Tasks []TaskSummaryPayload `json:"tasks"`

	// Counts always describe the whole board, never the filtered slice, so the
	// UI can label its filters without a second request.
	ActiveCount  int `json:"active_count"`
	StoppedCount int `json:"stopped_count"`
}

type TaskDetailResp struct {
	Task  TaskPayload   `json:"task"`
	Chain []TaskPayload `json:"chain"`
}

type OccurrencePayload struct {
	Task   TaskPayload `json:"task"`
	Day    string      `json:"day"`
	Status string      `json:"status"`
	Note   string      `json:"note,omitempty"`
}

type DayViewResp struct {
	Day         string              `json:"day"`
	Occurrences []OccurrencePayload `json:"occurrences"`
}

type RangeViewResp struct {
	From string        `json:"from"`
	To   string        `json:"to"`
	Days []DayViewResp `json:"days"`
}

type MarkDayReq struct {
	Status string `json:"status" validate:"required,oneof=completed skipped failed"`
	Note   string `json:"note" validate:"max=250"`
}
