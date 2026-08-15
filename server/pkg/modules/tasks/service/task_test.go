package service

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/tasks/dto"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
	"andreypostal.com/cubby/server/pkg/modules/tasks/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// taskTestEnv is the task service over a database of its own, with two
// workspaces so the ownership rules have something to refuse
type taskTestEnv struct {
	service TaskService

	workspaceID uint
	// other belongs to somebody else, and proves the refusals
	other uint

	// today is the board's today, since every window rule is written about it
	today time.Time
}

func newTaskTestEnv(t *testing.T) *taskTestEnv {
	t.Helper()

	// The dsn is a file: URI, so the drive letter has to come with forward slashes
	dataDir := path.Join(filepath.ToSlash(t.TempDir()), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	registry := core.NewRegistry()
	registry.Register(new(core.DB))

	ctx := core.NewContext(registry, core.Config{
		DataDir: dataDir,
		DB: core.DBConfig{
			MaxOpenConns:    1,
			MaxIdleConns:    1,
			ConnMaxLifetime: time.Minute,
		},
	})

	db, err := core.GetModule[*core.DB](ctx, new(core.DB))
	require.NoError(t, err)

	// The database has to let go of the files before TempDir removes them
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.AutoMigrate(
		&authModels.User{},
		&authModels.Workspace{},
		&models.TaskBoard{},
		&models.Task{},
		&models.TaskEntry{},
	))

	owner := &authModels.User{Username: "owner", Password: "x", Name: "owner"}
	require.NoError(t, db.Create(owner).Error)

	env := &taskTestEnv{
		service: NewTaskService(
			repository.NewBoardRepository(db),
			repository.NewTaskRepository(db),
			repository.NewTaskEntryRepository(db),
		),
		// The board defaults to UTC, so that is the day the service works against
		today: core.TodayIn(time.UTC),
	}

	for _, w := range []struct {
		name string
		into *uint
	}{
		{"Kitchen", &env.workspaceID},
		{"Garage", &env.other},
	} {
		workspace := &authModels.Workspace{Name: w.name, CreatorID: owner.ID}
		require.NoError(t, db.Create(workspace).Error)
		*w.into = workspace.ID
	}

	return env
}

// create adds a task to the main workspace and hands back its payload
func (e *taskTestEnv) create(t *testing.T, req *dto.CreateTaskReq) *dto.TaskPayload {
	t.Helper()

	created, err := e.service.Create(e.workspaceID, req)
	require.NoError(t, err)

	return created
}

// daily is the simplest task there is: it happens every day from startsOn on
func (e *taskTestEnv) daily(t *testing.T, name string, startsOn time.Time) *dto.TaskPayload {
	t.Helper()

	return e.create(t, &dto.CreateTaskReq{
		Name:           name,
		Icon:           "check",
		RecurrenceType: string(models.RecurrenceDaily),
		StartsOn:       core.FormatDate(startsOn),
	})
}

// day shifts the board's today by n days
func (e *taskTestEnv) day(n int) time.Time {
	return core.AddDays(e.today, n)
}

// ---------------------------------------------------------------- board

func TestBoardIsCreatedOnDemandInUTC(t *testing.T) {
	env := newTaskTestEnv(t)

	board, err := env.service.GetBoard(env.workspaceID)
	require.NoError(t, err)
	assert.Equal(t, "UTC", board.Timezone)
	assert.Equal(t, core.FormatDate(env.today), board.Today)

	// Asking twice keeps the same board rather than making a second one
	again, err := env.service.GetBoard(env.workspaceID)
	require.NoError(t, err)
	assert.Equal(t, board.ID, again.ID)

	// Every workspace gets its own
	otherBoard, err := env.service.GetBoard(env.other)
	require.NoError(t, err)
	assert.NotEqual(t, board.ID, otherBoard.ID)
}

func TestUpdateBoardOnlyAcceptsKnownTimezones(t *testing.T) {
	env := newTaskTestEnv(t)

	updated, err := env.service.UpdateBoard(env.workspaceID, &dto.UpdateBoardReq{Timezone: "America/Sao_Paulo"})
	require.NoError(t, err)
	assert.Equal(t, "America/Sao_Paulo", updated.Timezone)

	_, err = env.service.UpdateBoard(env.workspaceID, &dto.UpdateBoardReq{Timezone: "Mars/Olympus"})
	require.Error(t, err)

	// The rejected one did not stick
	board, err := env.service.GetBoard(env.workspaceID)
	require.NoError(t, err)
	assert.Equal(t, "America/Sao_Paulo", board.Timezone)
}

// ---------------------------------------------------------------- create

func TestCreateOnetimeClosesItsOwnWindow(t *testing.T) {
	env := newTaskTestEnv(t)

	task := env.create(t, &dto.CreateTaskReq{
		Name:           "Renew the passport",
		Icon:           "id",
		RecurrenceType: string(models.RecurrenceOnce),
		StartsOn:       core.FormatDate(env.day(3)),
	})

	assert.Equal(t, core.FormatDate(env.day(3)), task.StartsOn)
	require.NotNil(t, task.EndsOn, "a onetime task ends on the day it happens")
	assert.Equal(t, core.FormatDate(env.day(3)), *task.EndsOn)
}

func TestCreateDefaultsToTodayAndPreviousBusinessDay(t *testing.T) {
	env := newTaskTestEnv(t)

	task := env.daily(t, "Water the plants", env.today)

	assert.Equal(t, core.FormatDate(env.today), task.StartsOn)
	assert.Nil(t, task.EndsOn, "a daily task runs forever")
	assert.Equal(t, string(models.BusinessDayPrevious), task.BusinessDayStrategy)
	assert.False(t, task.Stopped)
}

func TestCreateStoresWeekdaysAsAMask(t *testing.T) {
	env := newTaskTestEnv(t)

	task := env.create(t, &dto.CreateTaskReq{
		Name:           "Gym",
		Icon:           "dumbbell",
		RecurrenceType: string(models.RecurrenceWeekly),
		Weekdays:       []int{1, 3, 5},
		StartsOn:       core.FormatDate(env.today),
	})

	assert.Equal(t, []int{1, 3, 5}, task.Weekdays, "the mask round-trips back to the friendly form")
}

func TestCreateRejectsIncompleteRecurrences(t *testing.T) {
	env := newTaskTestEnv(t)

	for _, tc := range []struct {
		name string
		req  *dto.CreateTaskReq
	}{
		{"unknown type", &dto.CreateTaskReq{RecurrenceType: "hourly"}},
		{"weekly without weekdays", &dto.CreateTaskReq{RecurrenceType: string(models.RecurrenceWeekly)}},
		{"weekly out of range", &dto.CreateTaskReq{
			RecurrenceType: string(models.RecurrenceWeekly), Weekdays: []int{7},
		}},
		{"weekly repeating a day", &dto.CreateTaskReq{
			RecurrenceType: string(models.RecurrenceWeekly), Weekdays: []int{2, 2},
		}},
		{"monthly without a day", &dto.CreateTaskReq{RecurrenceType: string(models.RecurrenceMonthly)}},
		{"monthly past the 31st", &dto.CreateTaskReq{
			RecurrenceType: string(models.RecurrenceMonthly), DayOfMonth: 32,
		}},
		{"annual without a month", &dto.CreateTaskReq{
			RecurrenceType: string(models.RecurrenceAnnual), DayOfMonth: 25,
		}},
		{"annual without a day", &dto.CreateTaskReq{
			RecurrenceType: string(models.RecurrenceAnnual), MonthOfYear: 12,
		}},
		{"invalid starts_on", &dto.CreateTaskReq{
			RecurrenceType: string(models.RecurrenceDaily), StartsOn: "25/12/2026",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.Name = "Whatever"
			tc.req.Icon = "check"

			_, err := env.service.Create(env.workspaceID, tc.req)
			assert.Error(t, err)
		})
	}
}

// Fields belonging to another recurrence are refused rather than ignored, so a
// stored task never carries settings its type does not read.
func TestCreateRejectsFieldsFromAnotherRecurrence(t *testing.T) {
	env := newTaskTestEnv(t)

	_, err := env.service.Create(env.workspaceID, &dto.CreateTaskReq{
		Name: "Gym", Icon: "check",
		RecurrenceType: string(models.RecurrenceDaily),
		Weekdays:       []int{1},
	})
	assert.Error(t, err, "weekdays belong to a weekly task")

	_, err = env.service.Create(env.workspaceID, &dto.CreateTaskReq{
		Name: "Gym", Icon: "check",
		RecurrenceType: string(models.RecurrenceDaily),
		DayOfMonth:     10,
	})
	assert.Error(t, err, "day_of_month belongs to a monthly or annual task")

	_, err = env.service.Create(env.workspaceID, &dto.CreateTaskReq{
		Name: "Gym", Icon: "check",
		RecurrenceType: string(models.RecurrenceMonthly),
		DayOfMonth:     10,
		MonthOfYear:    12,
	})
	assert.Error(t, err, "month_of_year belongs to an annual task")
}

// ---------------------------------------------------------------- ownership

func TestTasksAreReachableOnlyFromTheirOwnWorkspace(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	_, err := env.service.Get(task.ID, env.other)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.service.Update(task.ID, env.other, &dto.UpdateTaskReq{Name: "Stolen", Icon: "check"})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.service.Delete(task.ID, env.other), core.ErrNotAllowed)

	// A task that never existed is refused the same way, so probing ids tells
	// a caller nothing about another workspace
	_, err = env.service.Get(task.ID+999, env.workspaceID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

func TestUpdateOnlyTouchesMetadata(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.create(t, &dto.CreateTaskReq{
		Name:           "Gym",
		Icon:           "dumbbell",
		RecurrenceType: string(models.RecurrenceWeekly),
		Weekdays:       []int{1, 3},
		StartsOn:       core.FormatDate(env.today),
	})

	updated, err := env.service.Update(task.ID, env.workspaceID, &dto.UpdateTaskReq{
		Name:         "Gym, but early",
		Icon:         "sunrise",
		Description:  "before work",
		Instructions: "bring the towel",
		Pinned:       true,
	})
	require.NoError(t, err)

	assert.Equal(t, "Gym, but early", updated.Name)
	assert.Equal(t, "sunrise", updated.Icon)
	assert.Equal(t, "before work", updated.Description)
	assert.True(t, updated.Pinned)

	// The recurrence is untouched: only stop and resume may move a window
	assert.Equal(t, string(models.RecurrenceWeekly), updated.RecurrenceType)
	assert.Equal(t, []int{1, 3}, updated.Weekdays)
	assert.Equal(t, task.StartsOn, updated.StartsOn)
}

func TestDeleteTakesTheTaskOutOfReach(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	require.NoError(t, env.service.Delete(task.ID, env.workspaceID))

	_, err := env.service.Get(task.ID, env.workspaceID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	list, err := env.service.List(env.workspaceID, FilterAll)
	require.NoError(t, err)
	assert.Empty(t, list.Tasks)
}

// ---------------------------------------------------------------- list

func TestListSplitsActiveFromStoppedAndAlwaysCountsBoth(t *testing.T) {
	env := newTaskTestEnv(t)

	active := env.daily(t, "Water the plants", env.today)
	stopped := env.daily(t, "Old routine", env.today)
	_, err := env.service.Stop(stopped.ID, env.workspaceID)
	require.NoError(t, err)

	// A onetime whose day has passed is neither: it simply expired
	env.create(t, &dto.CreateTaskReq{
		Name: "Renew the passport", Icon: "id",
		RecurrenceType: string(models.RecurrenceOnce),
		StartsOn:       core.FormatDate(env.day(-5)),
	})

	activeList, err := env.service.List(env.workspaceID, FilterActive)
	require.NoError(t, err)
	require.Len(t, activeList.Tasks, 1)
	assert.Equal(t, active.ID, activeList.Tasks[0].ID)

	stoppedList, err := env.service.List(env.workspaceID, FilterStopped)
	require.NoError(t, err)
	require.Len(t, stoppedList.Tasks, 1)
	assert.Equal(t, stopped.ID, stoppedList.Tasks[0].ID)

	all, err := env.service.List(env.workspaceID, FilterAll)
	require.NoError(t, err)
	assert.Len(t, all.Tasks, 3, "the expired onetime is still a task")

	// The counts describe the board, not the slice that was asked for
	for _, resp := range []*dto.TaskListResp{activeList, stoppedList, all} {
		assert.Equal(t, 1, resp.ActiveCount)
		assert.Equal(t, 1, resp.StoppedCount)
	}

	_, err = env.service.List(env.workspaceID, "whatever")
	assert.Error(t, err)
}

func TestListReportsTheNextOccurrence(t *testing.T) {
	env := newTaskTestEnv(t)

	daily := env.daily(t, "Water the plants", env.today)
	future := env.create(t, &dto.CreateTaskReq{
		Name: "Renew the passport", Icon: "id",
		RecurrenceType: string(models.RecurrenceOnce),
		StartsOn:       core.FormatDate(env.day(4)),
	})
	past := env.create(t, &dto.CreateTaskReq{
		Name: "Pay the fine", Icon: "receipt",
		RecurrenceType: string(models.RecurrenceOnce),
		StartsOn:       core.FormatDate(env.day(-4)),
	})

	list, err := env.service.List(env.workspaceID, FilterAll)
	require.NoError(t, err)

	next := make(map[uint]*string, len(list.Tasks))
	for _, task := range list.Tasks {
		next[task.ID] = task.NextOccurrence
	}

	require.NotNil(t, next[daily.ID])
	assert.Equal(t, core.FormatDate(env.today), *next[daily.ID], "a daily task fires today")

	require.NotNil(t, next[future.ID])
	assert.Equal(t, core.FormatDate(env.day(4)), *next[future.ID])

	assert.Nil(t, next[past.ID], "a onetime already past has nothing left")
}

// ---------------------------------------------------------------- stop and resume

func TestStopClosesTheWindowToday(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	stopped, err := env.service.Stop(task.ID, env.workspaceID)
	require.NoError(t, err)

	assert.True(t, stopped.Stopped)
	assert.NotNil(t, stopped.StoppedAt)
	require.NotNil(t, stopped.EndsOn)
	assert.Equal(t, core.FormatDate(env.today), *stopped.EndsOn)

	_, err = env.service.Stop(task.ID, env.workspaceID)
	assert.Error(t, err, "a stopped task cannot be stopped again")
}

func TestOnetimeTasksCannotBeStoppedOrResumed(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.create(t, &dto.CreateTaskReq{
		Name: "Renew the passport", Icon: "id",
		RecurrenceType: string(models.RecurrenceOnce),
		StartsOn:       core.FormatDate(env.today),
	})

	_, err := env.service.Stop(task.ID, env.workspaceID)
	assert.Error(t, err)

	_, err = env.service.Resume(task.ID, env.workspaceID, nil)
	assert.Error(t, err)
}

func TestResumeClonesTheTaskIntoTheSameChain(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	_, err := env.service.Resume(task.ID, env.workspaceID, nil)
	assert.Error(t, err, "a running task has nothing to resume")

	_, err = env.service.Stop(task.ID, env.workspaceID)
	require.NoError(t, err)

	// The stopped run covers today, so resuming from today would overlap it
	_, err = env.service.Resume(task.ID, env.workspaceID, &dto.ResumeTaskReq{
		From: core.FormatDate(env.today),
	})
	assert.Error(t, err)

	clone, err := env.service.Resume(task.ID, env.workspaceID, &dto.ResumeTaskReq{
		From: core.FormatDate(env.day(1)),
	})
	require.NoError(t, err)

	assert.NotEqual(t, task.ID, clone.ID)
	assert.Equal(t, core.FormatDate(env.day(1)), clone.StartsOn)
	assert.Nil(t, clone.EndsOn)
	assert.False(t, clone.Stopped)
	require.NotNil(t, clone.OriginTaskID)
	assert.Equal(t, task.ID, *clone.OriginTaskID, "the clone points back at the run that started the chain")

	detail, err := env.service.Get(clone.ID, env.workspaceID)
	require.NoError(t, err)
	require.Len(t, detail.Chain, 2, "both runs belong to the same chain")
	assert.Equal(t, task.ID, detail.Chain[0].ID, "the chain comes back in start order")
	assert.Equal(t, clone.ID, detail.Chain[1].ID)
}

// Only the newest run of a chain is worth resuming: an older one always
// overlaps the clone that took over from it.
func TestOnlyTheLatestRunOfAChainIsResumable(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	_, err := env.service.Stop(task.ID, env.workspaceID)
	require.NoError(t, err)

	stoppedList, err := env.service.List(env.workspaceID, FilterStopped)
	require.NoError(t, err)
	require.Len(t, stoppedList.Tasks, 1)
	assert.True(t, stoppedList.Tasks[0].Resumable)

	clone, err := env.service.Resume(task.ID, env.workspaceID, &dto.ResumeTaskReq{
		From: core.FormatDate(env.day(1)),
	})
	require.NoError(t, err)

	stoppedList, err = env.service.List(env.workspaceID, FilterStopped)
	require.NoError(t, err)
	require.Len(t, stoppedList.Tasks, 1)
	assert.Equal(t, task.ID, stoppedList.Tasks[0].ID)
	assert.False(t, stoppedList.Tasks[0].Resumable, "the clone already took over")

	// And the clone is the one running now
	activeList, err := env.service.List(env.workspaceID, FilterActive)
	require.NoError(t, err)
	require.Len(t, activeList.Tasks, 1)
	assert.Equal(t, clone.ID, activeList.Tasks[0].ID)
}

// ---------------------------------------------------------------- day and range

func TestDayListsTheOccurrencesOfThatDay(t *testing.T) {
	env := newTaskTestEnv(t)

	daily := env.daily(t, "Water the plants", env.today)
	// A onetime tomorrow does not show up today
	env.create(t, &dto.CreateTaskReq{
		Name: "Renew the passport", Icon: "id",
		RecurrenceType: string(models.RecurrenceOnce),
		StartsOn:       core.FormatDate(env.day(1)),
	})

	view, err := env.service.Day(env.workspaceID, core.FormatDate(env.today))
	require.NoError(t, err)

	assert.Equal(t, core.FormatDate(env.today), view.Day)
	require.Len(t, view.Occurrences, 1)
	assert.Equal(t, daily.ID, view.Occurrences[0].Task.ID)
	assert.Equal(t, StatusPending, view.Occurrences[0].Status)

	// An empty date means today
	today, err := env.service.Day(env.workspaceID, "")
	require.NoError(t, err)
	assert.Equal(t, view.Day, today.Day)

	_, err = env.service.Day(env.workspaceID, "13/08/2026")
	assert.Error(t, err)
}

func TestRangeWalksEveryDayAndSeparatesPendingFromFuture(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.day(-1))

	_, err := env.service.MarkDay(task.ID, env.workspaceID, env.day(-1), &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
		Note:   "done before breakfast",
	})
	require.NoError(t, err)

	resp, err := env.service.Range(env.workspaceID, core.FormatDate(env.day(-1)), core.FormatDate(env.day(1)))
	require.NoError(t, err)

	assert.Equal(t, core.FormatDate(env.day(-1)), resp.From)
	assert.Equal(t, core.FormatDate(env.day(1)), resp.To)
	require.Len(t, resp.Days, 3, "both ends are included")

	for i, expected := range []string{StatusCompleted, StatusPending, StatusFuture} {
		require.Len(t, resp.Days[i].Occurrences, 1, "day %s", resp.Days[i].Day)
		assert.Equal(t, expected, resp.Days[i].Occurrences[0].Status, "day %s", resp.Days[i].Day)
	}

	assert.Equal(t, "done before breakfast", resp.Days[0].Occurrences[0].Note)
	assert.Empty(t, resp.Days[1].Occurrences[0].Note)
}

func TestRangeRefusesBadBounds(t *testing.T) {
	env := newTaskTestEnv(t)

	_, err := env.service.Range(env.workspaceID, "not a date", core.FormatDate(env.today))
	assert.Error(t, err)

	_, err = env.service.Range(env.workspaceID, core.FormatDate(env.today), "not a date")
	assert.Error(t, err)

	_, err = env.service.Range(env.workspaceID, core.FormatDate(env.today), core.FormatDate(env.day(-1)))
	assert.Error(t, err, "to must not come before from")

	_, err = env.service.Range(env.workspaceID, core.FormatDate(env.today), core.FormatDate(env.day(MaxRangeDays)))
	assert.Error(t, err, "one day past the maximum span")

	// The widest range that still fits is fine
	resp, err := env.service.Range(env.workspaceID, core.FormatDate(env.today), core.FormatDate(env.day(MaxRangeDays-1)))
	require.NoError(t, err)
	assert.Len(t, resp.Days, MaxRangeDays)
}

// ---------------------------------------------------------------- marking

func TestMarkDayIsIdempotentAndShowsUpInTheDayView(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	marked, err := env.service.MarkDay(task.ID, env.workspaceID, env.today, &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, marked.Status)
	assert.Equal(t, core.FormatDate(env.today), marked.Day)

	// Re-sending the same mark is a no-op update, so a client can retry freely
	_, err = env.service.MarkDay(task.ID, env.workspaceID, env.today, &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	require.NoError(t, err)

	// And a different outcome overwrites the first one rather than piling up
	_, err = env.service.MarkDay(task.ID, env.workspaceID, env.today, &dto.MarkDayReq{
		Status: string(models.EntrySkipped),
		Note:   "changed my mind",
	})
	require.NoError(t, err)

	view, err := env.service.Day(env.workspaceID, core.FormatDate(env.today))
	require.NoError(t, err)
	require.Len(t, view.Occurrences, 1, "one entry per task and day")
	assert.Equal(t, StatusSkipped, view.Occurrences[0].Status)
	assert.Equal(t, "changed my mind", view.Occurrences[0].Note)
}

func TestMarkDayRefusesWhatCannotBeAnOutcome(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	_, err := env.service.MarkDay(task.ID, env.workspaceID, env.day(1), &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	assert.Error(t, err, "a future day has no outcome yet")

	_, err = env.service.MarkDay(task.ID, env.workspaceID, env.day(-1), &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	assert.Error(t, err, "the day is outside the task's window")

	_, err = env.service.MarkDay(task.ID, env.workspaceID, env.today, &dto.MarkDayReq{Status: "maybe"})
	assert.Error(t, err, "unknown status")

	_, err = env.service.MarkDay(task.ID, env.other, env.today, &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

// A weekly task only accepts the weekdays it actually falls on, even when the
// day sits well inside its window.
func TestMarkDayRefusesADayTheTaskDoesNotFallOn(t *testing.T) {
	env := newTaskTestEnv(t)

	// Yesterday is inside the window, but only today's weekday occurs
	task := env.create(t, &dto.CreateTaskReq{
		Name: "Gym", Icon: "dumbbell",
		RecurrenceType: string(models.RecurrenceWeekly),
		Weekdays:       []int{int(env.today.Weekday())},
		StartsOn:       core.FormatDate(env.day(-7)),
	})

	_, err := env.service.MarkDay(task.ID, env.workspaceID, env.day(-1), &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	assert.Error(t, err)

	_, err = env.service.MarkDay(task.ID, env.workspaceID, env.day(-7), &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
	})
	assert.NoError(t, err, "same weekday, a week earlier")
}

func TestClearDaySendsTheDayBackToPending(t *testing.T) {
	env := newTaskTestEnv(t)
	task := env.daily(t, "Water the plants", env.today)

	_, err := env.service.MarkDay(task.ID, env.workspaceID, env.today, &dto.MarkDayReq{
		Status: string(models.EntryCompleted),
		Note:   "done",
	})
	require.NoError(t, err)

	cleared, err := env.service.ClearDay(task.ID, env.workspaceID, env.today)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, cleared.Status)
	assert.Empty(t, cleared.Note)

	// Clearing an unmarked day succeeds just the same
	cleared, err = env.service.ClearDay(task.ID, env.workspaceID, env.today)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, cleared.Status)

	view, err := env.service.Day(env.workspaceID, core.FormatDate(env.today))
	require.NoError(t, err)
	require.Len(t, view.Occurrences, 1)
	assert.Equal(t, StatusPending, view.Occurrences[0].Status)

	// Marking again after a clear revives the day rather than failing on the
	// unique index the cleared row still holds
	marked, err := env.service.MarkDay(task.ID, env.workspaceID, env.today, &dto.MarkDayReq{
		Status: string(models.EntryFailed),
	})
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, marked.Status)
}
