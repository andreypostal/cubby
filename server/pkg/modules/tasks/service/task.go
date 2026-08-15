// Package service holds the business rules of the tasks module
package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/tasks/dto"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
	"andreypostal.com/cubby/server/pkg/modules/tasks/repository"
)

const (
	// >> Actual status (persisted)
	StatusCompleted = "completed"
	StatusSkipped   = "skipped"
	StatusFailed    = "failed"
	// >> Inflight status - only for reporting in API response
	// StatusPending is a day that has occurred but was never marked
	StatusPending = "pending"
	// StatusFuture is a day that has not happened yet
	StatusFuture = "future"
)

// MaxRangeDays caps GET tasks by range, the way that it works now, 1 month should be enough
const MaxRangeDays = 31

// Filters accepted by List, describing which slice of the board to return
const (
	FilterActive  = "active"
	FilterStopped = "stopped"
	FilterAll     = "all"
)

// NextOccurrenceLookahead caps the day-by-day walk that resolves a task's next
// occurrence. A monthly rule can skip at most a month, so a full year past the
// start means there really is nothing left to find.
const NextOccurrenceLookahead = 366

// IsValidListFilter reports whether a filter is one List understands.
func IsValidListFilter(filter string) bool {
	switch filter {
	case FilterActive, FilterStopped, FilterAll:
		return true
	default:
		return false
	}
}

type TaskService interface {
	GetBoard(workspaceID uint) (*dto.BoardPayload, error)
	UpdateBoard(workspaceID uint, req *dto.UpdateBoardReq) (*dto.BoardPayload, error)

	List(workspaceID uint, filter string) (*dto.TaskListResp, error)
	Create(workspaceID uint, req *dto.CreateTaskReq) (*dto.TaskPayload, error)
	Get(taskID, workspaceID uint) (*dto.TaskDetailResp, error)
	Update(taskID, workspaceID uint, req *dto.UpdateTaskReq) (*dto.TaskPayload, error)
	Delete(taskID, workspaceID uint) error

	Stop(taskID, workspaceID uint) (*dto.TaskPayload, error)
	Resume(taskID, workspaceID uint, req *dto.ResumeTaskReq) (*dto.TaskPayload, error)

	Day(workspaceID uint, rawDate string) (*dto.DayViewResp, error)
	Range(workspaceID uint, rawFrom, rawTo string) (*dto.RangeViewResp, error)

	MarkDay(taskID, workspaceID uint, day time.Time, req *dto.MarkDayReq) (*dto.OccurrencePayload, error)
	ClearDay(taskID, workspaceID uint, day time.Time) (*dto.OccurrencePayload, error)
}

type ts struct {
	boardRepo repository.BoardRepository
	taskRepo  repository.TaskRepository
	entryRepo repository.TaskEntryRepository
}

func NewTaskService(
	boardRepo repository.BoardRepository,
	taskRepo repository.TaskRepository,
	entryRepo repository.TaskEntryRepository,
) TaskService {
	return &ts{boardRepo: boardRepo, taskRepo: taskRepo, entryRepo: entryRepo}
}

// ---------------------------------------------------------------- board

func (s *ts) GetBoard(workspaceID uint) (*dto.BoardPayload, error) {
	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}
	return toBoardPayload(board), nil
}

func (s *ts) UpdateBoard(workspaceID uint, req *dto.UpdateBoardReq) (*dto.BoardPayload, error) {
	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	if _, err := time.LoadLocation(req.Timezone); err != nil {
		return nil, fmt.Errorf("unknown timezone %q", req.Timezone)
	}

	board.Timezone = req.Timezone
	if err := s.boardRepo.Update(board); err != nil {
		return nil, err
	}

	return toBoardPayload(board), nil
}

// ---------------------------------------------------------------- tasks

// List is the workspace overview: every task definition the board holds, sliced
// by whether it is still running. The counts it reports always cover the whole
// board so a client can label both slices from a single call.
func (s *ts) List(workspaceID uint, filter string) (*dto.TaskListResp, error) {
	if !IsValidListFilter(filter) {
		return nil, fmt.Errorf("status must be one of active, stopped or all")
	}

	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	tasks, err := s.taskRepo.ListForBoard(board.ID)
	if err != nil {
		return nil, err
	}

	today := core.TodayIn(loc)
	latest := latestRunByChain(tasks)

	resp := &dto.TaskListResp{Tasks: []dto.TaskSummaryPayload{}}

	for _, t := range tasks {
		next := nextOccurrence(t, today)
		expired := isExpiredOnetime(t, next)

		if t.IsStopped() {
			resp.StoppedCount++
		} else if !expired {
			resp.ActiveCount++
		}

		if !matchesFilter(t, filter, expired) {
			continue
		}

		item := dto.TaskSummaryPayload{
			TaskPayload: *toTaskPayload(t),
			Resumable:   t.IsStopped() && t.RecurrenceType != models.RecurrenceOnce && latest[t.RootID()] == t.ID,
		}

		if next != nil {
			day := core.FormatDate(*next)
			item.NextOccurrence = &day
		}

		resp.Tasks = append(resp.Tasks, item)
	}

	return resp, nil
}

// isExpiredOnetime reports whether a task is a onetime whose day is already in the past
func isExpiredOnetime(task *models.Task, next *time.Time) bool {
	return task.RecurrenceType == models.RecurrenceOnce && next == nil
}

func matchesFilter(task *models.Task, filter string, expired bool) bool {
	switch filter {
	case FilterActive:
		return !task.IsStopped() && !expired
	case FilterStopped:
		return task.IsStopped()
	default:
		return true
	}
}

// latestRunByChain maps each chain root to the id of its most recent run. Only
// that one is worth resuming — an older stopped run always overlaps the clone
// that continued it, which the API refuses.
func latestRunByChain(tasks []*models.Task) map[uint]uint {
	latest := make(map[uint]uint, len(tasks))
	starts := make(map[uint]time.Time, len(tasks))

	for _, t := range tasks {
		root := t.RootID()
		start := core.NormalizeDate(t.StartsOn)

		current, seen := starts[root]
		// Two runs of a chain cannot share a start day, but ids break the tie
		// anyway so the winner never depends on row order.
		if !seen || start.After(current) || (start.Equal(current) && t.ID > latest[root]) {
			latest[root] = t.ID
			starts[root] = start
		}
	}

	return latest
}

// nextOccurrence walks forward from today until the recurrence fires, which is
// the one thing SQL cannot answer about a task in isolation.
func nextOccurrence(task *models.Task, today time.Time) *time.Time {
	day := core.NormalizeDate(today)
	if start := core.NormalizeDate(task.StartsOn); start.After(day) {
		day = start
	}

	for i := 0; i <= NextOccurrenceLookahead; i++ {
		if task.EndsOn != nil && day.After(core.NormalizeDate(*task.EndsOn)) {
			return nil
		}
		if Occurs(task, day) {
			found := day
			return &found
		}
		day = core.AddDays(day, 1)
	}

	return nil
}

func (s *ts) Create(workspaceID uint, req *dto.CreateTaskReq) (*dto.TaskPayload, error) {
	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	startsOn := core.TodayIn(loc)
	if req.StartsOn != "" {
		startsOn, err = core.ParseDate(req.StartsOn)
		if err != nil {
			return nil, fmt.Errorf("invalid starts_on, expected YYYY-MM-DD")
		}
	}

	task := &models.Task{
		BoardID:      board.ID,
		Name:         req.Name,
		Icon:         req.Icon,
		Description:  req.Description,
		Instructions: req.Instructions,
		Pinned:       req.Pinned,

		RecurrenceType:      models.RecurrenceType(req.RecurrenceType),
		BusinessDayStrategy: models.BusinessDayPrevious,

		StartsOn: startsOn,
	}

	if req.BusinessDayStrategy != "" {
		task.BusinessDayStrategy = models.BusinessDayStrategy(req.BusinessDayStrategy)
	}

	// Ensure correctness of each type of recurrence
	switch task.RecurrenceType {
	case models.RecurrenceOnce:
		end := startsOn
		task.EndsOn = &end

	case models.RecurrenceDaily:
		// Nothing, it just.. happens all the time

	case models.RecurrenceWeekly:
		if err := validateWeekdays(req.Weekdays); err != nil {
			return nil, err
		}
		task.Weekdays = WeekdaysToMask(req.Weekdays)

	case models.RecurrenceMonthly:
		if req.DayOfMonth < 1 || req.DayOfMonth > 31 {
			return nil, fmt.Errorf("day_of_month must be between 1 and 31 for a monthly task")
		}
		task.DayOfMonth = req.DayOfMonth

	case models.RecurrenceAnnual:
		if req.MonthOfYear < 1 || req.MonthOfYear > 12 {
			return nil, fmt.Errorf("month_of_year must be between 1 and 12 for an annual task")
		}
		if req.DayOfMonth < 1 || req.DayOfMonth > 31 {
			return nil, fmt.Errorf("day_of_month must be between 1 and 31 for an annual task")
		}
		task.MonthOfYear = req.MonthOfYear
		task.DayOfMonth = req.DayOfMonth

	default:
		return nil, fmt.Errorf("unknown recurrence_type %q", req.RecurrenceType)
	}

	// Ensure we don't have a dirt object (with data from other recurrence types)
	if task.RecurrenceType != models.RecurrenceWeekly && len(req.Weekdays) > 0 {
		return nil, fmt.Errorf("weekdays is only valid for a weekly task")
	}
	if task.RecurrenceType != models.RecurrenceMonthly && task.RecurrenceType != models.RecurrenceAnnual && req.DayOfMonth != 0 {
		return nil, fmt.Errorf("day_of_month is only valid for a monthly or annual task")
	}
	if task.RecurrenceType != models.RecurrenceAnnual && req.MonthOfYear != 0 {
		return nil, fmt.Errorf("month_of_year is only valid for an annual task")
	}

	if err := s.taskRepo.Create(task); err != nil {
		return nil, err
	}

	return toTaskPayload(task), nil
}

func (s *ts) Get(taskID, workspaceID uint) (*dto.TaskDetailResp, error) {
	task, _, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return nil, err
	}

	chain, err := s.taskRepo.ListChain(task.RootID())
	if err != nil {
		return nil, err
	}

	out := make([]dto.TaskPayload, len(chain))
	for i, t := range chain {
		out[i] = *toTaskPayload(t)
	}

	return &dto.TaskDetailResp{Task: *toTaskPayload(task), Chain: out}, nil
}

func (s *ts) Update(taskID, workspaceID uint, req *dto.UpdateTaskReq) (*dto.TaskPayload, error) {
	task, _, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return nil, err
	}

	task.Name = req.Name
	task.Icon = req.Icon
	task.Description = req.Description
	task.Instructions = req.Instructions
	task.Pinned = req.Pinned

	if err := s.taskRepo.Update(task); err != nil {
		return nil, err
	}

	return toTaskPayload(task), nil
}

func (s *ts) Delete(taskID, workspaceID uint) error {
	task, _, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return err
	}
	return s.taskRepo.Delete(task.ID)
}

func (s *ts) Stop(taskID, workspaceID uint) (*dto.TaskPayload, error) {
	task, board, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return nil, err
	}

	if task.RecurrenceType == models.RecurrenceOnce {
		return nil, fmt.Errorf("onetime task cannot be stopped")
	}
	if task.IsStopped() {
		return nil, fmt.Errorf("task already stopped")
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	today := core.TodayIn(loc)
	now := time.Now().UTC()

	task.EndsOn = &today
	task.StoppedAt = &now

	if err := s.taskRepo.Update(task); err != nil {
		return nil, err
	}

	return toTaskPayload(task), nil
}

// Resume clones the stopped recurrence
func (s *ts) Resume(taskID, workspaceID uint, req *dto.ResumeTaskReq) (*dto.TaskPayload, error) {
	task, board, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return nil, err
	}

	if task.RecurrenceType == models.RecurrenceOnce {
		return nil, fmt.Errorf("onetime task cannot be resumed")
	}
	if !task.IsStopped() {
		return nil, fmt.Errorf("task is not stopped")
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	from := core.TodayIn(loc)
	if req != nil && req.From != "" {
		from, err = core.ParseDate(req.From)
		if err != nil {
			return nil, fmt.Errorf("invalid from, expected YYYY-MM-DD")
		}
	}

	rootID := task.RootID()

	chain, err := s.taskRepo.ListChain(rootID)
	if err != nil {
		return nil, err
	}

	// Ensure we have a continuous chain of tasks - maybe we can add a duplicate functionality
	for _, sibling := range chain {
		if sibling.EndsOn == nil || !core.NormalizeDate(*sibling.EndsOn).Before(from) {
			return nil, fmt.Errorf(
				"cannot resume from %s: it overlaps task %d, which runs until %s",
				core.FormatDate(from), sibling.ID, endsOnLabel(sibling),
			)
		}
	}

	clone := &models.Task{
		BoardID:      task.BoardID,
		Name:         task.Name,
		Icon:         task.Icon,
		Description:  task.Description,
		Instructions: task.Instructions,
		Pinned:       task.Pinned,

		RecurrenceType:      task.RecurrenceType,
		Weekdays:            task.Weekdays,
		DayOfMonth:          task.DayOfMonth,
		MonthOfYear:         task.MonthOfYear,
		BusinessDayStrategy: task.BusinessDayStrategy,

		StartsOn: from,
		EndsOn:   nil,

		OriginTaskID: &rootID,
	}

	if err := s.taskRepo.Create(clone); err != nil {
		return nil, err
	}

	return toTaskPayload(clone), nil
}

// ---------------------------------------------------------------- occurrences

func (s *ts) Day(workspaceID uint, rawDate string) (*dto.DayViewResp, error) {
	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	day := core.TodayIn(loc)
	if rawDate != "" {
		day, err = core.ParseDate(rawDate)
		if err != nil {
			return nil, fmt.Errorf("invalid date, expected YYYY-MM-DD")
		}
	}

	days, err := s.buildRange(board.ID, day, day, core.TodayIn(loc))
	if err != nil {
		return nil, err
	}

	return &days[0], nil
}

func (s *ts) Range(workspaceID uint, rawFrom, rawTo string) (*dto.RangeViewResp, error) {
	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, err
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	from, err := core.ParseDate(rawFrom)
	if err != nil {
		return nil, fmt.Errorf("invalid from, expected YYYY-MM-DD")
	}

	to, err := core.ParseDate(rawTo)
	if err != nil {
		return nil, fmt.Errorf("invalid to, expected YYYY-MM-DD")
	}

	if to.Before(from) {
		return nil, fmt.Errorf("to must not be before from")
	}

	span := int(to.Sub(from).Hours()/24) + 1
	if span > MaxRangeDays {
		return nil, fmt.Errorf("range spans %d days, the maximum is %d", span, MaxRangeDays)
	}

	days, err := s.buildRange(board.ID, from, to, core.TodayIn(loc))
	if err != nil {
		return nil, err
	}

	return &dto.RangeViewResp{
		From: core.FormatDate(from),
		To:   core.FormatDate(to),
		Days: days,
	}, nil
}

func (s *ts) buildRange(boardID uint, from, to, today time.Time) ([]dto.DayViewResp, error) {
	// List every task we should touch
	tasks, err := s.taskRepo.ListActiveInRange(boardID, from, to)
	if err != nil {
		return nil, err
	}

	taskIDs := make([]uint, len(tasks))
	for i, t := range tasks {
		taskIDs[i] = t.ID
	}

	// Gather our knowledge for this interval
	entries, err := s.entryRepo.ListForTasksInRange(taskIDs, from, to)
	if err != nil {
		return nil, err
	}

	// Add our knowledge for a task in a given day
	byTaskDay := make(map[string]*models.TaskEntry, len(entries))
	for _, e := range entries {
		byTaskDay[entryKey(e.TaskID, e.Day)] = e
	}

	payloads := make(map[uint]*dto.TaskPayload, len(tasks))
	for _, t := range tasks {
		payloads[t.ID] = toTaskPayload(t)
	}

	var days []dto.DayViewResp

	// Mount the response for each day in the interval
	for d := from; !d.After(to); d = core.AddDays(d, 1) {
		view := dto.DayViewResp{Day: core.FormatDate(d), Occurrences: []dto.OccurrencePayload{}}

		for _, t := range tasks {
			if !Occurs(t, d) {
				continue
			}

			entry := byTaskDay[entryKey(t.ID, d)]
			view.Occurrences = append(view.Occurrences, dto.OccurrencePayload{
				Task:   *payloads[t.ID],
				Day:    core.FormatDate(d),
				Status: dayStatus(d, today, entry),
				Note:   entryNote(entry),
			})
		}

		days = append(days, view)
	}

	return days, nil
}

func (s *ts) MarkDay(taskID, workspaceID uint, day time.Time, req *dto.MarkDayReq) (*dto.OccurrencePayload, error) {
	task, board, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return nil, err
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	day = core.NormalizeDate(day)
	today := core.TodayIn(loc)

	if err := s.assertMarkable(task, day, today); err != nil {
		return nil, err
	}

	status := models.EntryStatus(req.Status)
	if !models.IsValidEntryStatus(status) {
		return nil, fmt.Errorf("status must be one of completed, skipped or failed")
	}

	// Idempotent by design: re-sending the same mark is a no-op update, not an
	// error, so a client can retry a request without special-casing.
	entry := &models.TaskEntry{
		TaskID: task.ID,
		Day:    day,
		Status: status,
		Note:   req.Note,
	}
	if err := s.entryRepo.Upsert(entry); err != nil {
		return nil, err
	}

	return &dto.OccurrencePayload{
		Task:   *toTaskPayload(task),
		Day:    core.FormatDate(day),
		Status: string(entry.Status),
		Note:   entry.Note,
	}, nil
}

// ClearDay removes a recorded outcome, sending the day back to pending. It is
// idempotent: clearing an already-unmarked day succeeds and reports pending.
func (s *ts) ClearDay(taskID, workspaceID uint, day time.Time) (*dto.OccurrencePayload, error) {
	task, board, err := s.loadOwnedTask(taskID, workspaceID)
	if err != nil {
		return nil, err
	}

	loc, err := boardLocation(board)
	if err != nil {
		return nil, err
	}

	day = core.NormalizeDate(day)
	today := core.TodayIn(loc)

	if err := s.assertMarkable(task, day, today); err != nil {
		return nil, err
	}

	existing, err := s.entryRepo.FindByTaskAndDay(task.ID, day)
	if err != nil && !errors.Is(err, core.ErrResourceNotFound) {
		return nil, err
	}

	if existing != nil {
		if err := s.entryRepo.Delete(existing.ID); err != nil {
			return nil, err
		}
	}

	return &dto.OccurrencePayload{
		Task:   *toTaskPayload(task),
		Day:    core.FormatDate(day),
		Status: dayStatus(day, today, nil),
	}, nil
}

// assertMarkable enforces the three rules an entry must satisfy. Together they
// are what guarantees an entry can never dangle: a row only exists for a day
// the task really occurs on, inside its window, and in the past.
func (s *ts) assertMarkable(task *models.Task, day, today time.Time) error {
	if day.After(today) {
		return fmt.Errorf("cannot record an outcome for a future day")
	}
	// Occurs already checks the window, so this one test covers both "outside
	// the task's window" and "not a day this recurrence falls on".
	if !Occurs(task, day) {
		return fmt.Errorf("%s is not an occurrence of this task", core.FormatDate(day))
	}
	return nil
}

func (s *ts) loadOwnedTask(taskID, workspaceID uint) (*models.Task, *models.TaskBoard, error) {
	board, err := s.boardRepo.GetOrCreateForWorkspace(workspaceID)
	if err != nil {
		return nil, nil, err
	}

	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return nil, nil, normalizeError(err)
	}

	if task.BoardID != board.ID {
		return nil, nil, core.ErrNotAllowed
	}

	return task, board, nil
}

// boardLocation resolves the board timezone, defaults to UTC
func boardLocation(board *models.TaskBoard) (*time.Location, error) {
	if board.Timezone == "" {
		return time.UTC, nil
	}

	loc, err := time.LoadLocation(board.Timezone)
	if err != nil {
		slog.Warn("Falling back to UTC for the board timezone",
			slog.Uint64("board_id", uint64(board.ID)),
			slog.String("timezone", board.Timezone),
			slog.String("error", err.Error()),
		)
		return time.UTC, nil //nolint:nilerr // the fallback is the point, the caller has nothing to decide
	}

	return loc, nil
}

func dayStatus(day, today time.Time, entry *models.TaskEntry) string {
	if entry != nil {
		return string(entry.Status)
	}
	if day.After(today) {
		return StatusFuture
	}
	return StatusPending
}

func entryNote(entry *models.TaskEntry) string {
	if entry == nil {
		return ""
	}
	return entry.Note
}

func entryKey(taskID uint, day time.Time) string {
	return fmt.Sprintf("%d|%s", taskID, core.FormatDate(day))
}

func endsOnLabel(task *models.Task) string {
	if task.EndsOn == nil {
		return "forever"
	}
	return core.FormatDate(*task.EndsOn)
}

func normalizeError(err error) error {
	if errors.Is(err, core.ErrEntityNotFound) || errors.Is(err, core.ErrResourceNotFound) {
		return core.ErrNotAllowed
	}
	return err
}

func toBoardPayload(board *models.TaskBoard) *dto.BoardPayload {
	loc, _ := boardLocation(board)

	return &dto.BoardPayload{
		ID:        board.ID,
		Timezone:  board.Timezone,
		Today:     core.FormatDate(core.TodayIn(loc)),
		CreatedAt: board.CreatedAt,
	}
}

func toTaskPayload(task *models.Task) *dto.TaskPayload {
	payload := &dto.TaskPayload{
		ID:           task.ID,
		Name:         task.Name,
		Icon:         task.Icon,
		Description:  task.Description,
		Instructions: task.Instructions,
		Pinned:       task.Pinned,

		RecurrenceType:      string(task.RecurrenceType),
		Weekdays:            MaskToWeekdays(task.Weekdays),
		DayOfMonth:          task.DayOfMonth,
		MonthOfYear:         task.MonthOfYear,
		BusinessDayStrategy: string(task.BusinessDayStrategy),

		StartsOn: core.FormatDate(task.StartsOn),

		Stopped:      task.IsStopped(),
		StoppedAt:    task.StoppedAt,
		OriginTaskID: task.OriginTaskID,

		CreatedAt: task.CreatedAt,
	}

	if task.EndsOn != nil {
		endsOn := core.FormatDate(*task.EndsOn)
		payload.EndsOn = &endsOn
	}

	return payload
}

func validateWeekdays(days []int) error {
	if len(days) == 0 {
		return fmt.Errorf("a weekly task needs at least one weekday")
	}

	seen := make(map[int]bool, len(days))
	for _, d := range days {
		if d < 0 || d > 6 {
			return fmt.Errorf("weekday %d is out of range, expected 0 (Sunday) to 6 (Saturday)", d)
		}
		if seen[d] {
			return fmt.Errorf("weekday %d appears more than once", d)
		}
		seen[d] = true
	}

	return nil
}
