package service

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/dto"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/models"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The workspaces the tests keep their routines in. A second one is what proves
// the scoping holds.
const (
	workspaceID = uint(1)
	otherWSID   = uint(2)
)

func newService(t *testing.T) RoutineService {
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

	// The same tables the module migrates on startup
	require.NoError(t, db.AutoMigrate(&models.Routine{}, &models.RoutineDay{}))

	// A routine belongs to a workspace and the foreign key is enforced
	owner := &authModels.User{Username: "owner", Password: "x", Name: "Owner"}
	require.NoError(t, db.Create(owner).Error)

	for _, id := range []uint{workspaceID, otherWSID} {
		require.NoError(t, db.Create(&authModels.Workspace{
			Model:     gorm.Model{ID: id},
			Name:      "Workspace",
			CreatorID: owner.ID,
		}).Error)
	}

	return NewRoutineService(
		repository.NewRoutineRepository(db),
		repository.NewRoutineDayRepository(db),
	)
}

// everyDayRoutine is scheduled all week, so any day the tests pick is markable
func everyDayRoutine(t *testing.T, s RoutineService) uint {
	t.Helper()

	created, err := s.Create(workspaceID, &dto.CreateRoutineReq{
		Name:        "Workout",
		OnMonday:    true,
		OnTuesday:   true,
		OnWednesday: true,
		OnThursday:  true,
		OnFriday:    true,
		OnSaturday:  true,
		OnSunday:    true,
	})
	require.NoError(t, err)

	return created.ID
}

func today() time.Time {
	return core.NormalizeDate(time.Now())
}

func TestANewRoutineStartsActiveAndAtZero(t *testing.T) {
	s := newService(t)

	created, err := s.Create(workspaceID, &dto.CreateRoutineReq{
		Name:       "Read",
		Pinned:     true,
		OnMonday:   true,
		OnFriday:   true,
		OnSaturday: true,
	})
	require.NoError(t, err)

	assert.Equal(t, "Read", created.Name)
	assert.Zero(t, created.Points)
	assert.True(t, created.Active, "a routine is only turned off through an update")
	assert.True(t, created.Pinned)
	assert.True(t, created.OnMonday)
	assert.False(t, created.OnTuesday)
}

func TestRoutinesAreScopedToTheirWorkspace(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)

	mine, err := s.ListForWorkspace(workspaceID)
	require.NoError(t, err)
	require.Len(t, mine.Routines, 1)

	theirs, err := s.ListForWorkspace(otherWSID)
	require.NoError(t, err)
	assert.Empty(t, theirs.Routines)

	// Every entry point refuses the other workspace, including the day actions
	_, err = s.Get(routineID, otherWSID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = s.Update(routineID, otherWSID, &dto.UpdateRoutineReq{Name: "Theirs"})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = s.Month(routineID, otherWSID, 2026, time.August)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = s.MarkDay(routineID, otherWSID, today(), true)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = s.UnmarkDay(routineID, otherWSID, today())
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, s.Delete(routineID, otherWSID), core.ErrNotAllowed)
}

// A routine that does not exist is refused the same way one owned by somebody
// else is, so the ids of other workspaces stay unguessable
func TestAMissingRoutineIsRefusedLikeSomebodyElses(t *testing.T) {
	s := newService(t)

	_, err := s.Get(404, workspaceID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

func TestUpdatingReplacesTheWholeSchedule(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)

	updated, err := s.Update(routineID, workspaceID, &dto.UpdateRoutineReq{
		Name:        "Workout",
		Description: "at the gym",
		Active:      false,
		OnMonday:    true,
		OnWednesday: true,
	})
	require.NoError(t, err)

	assert.Equal(t, "at the gym", updated.Description)
	assert.False(t, updated.Active)
	assert.True(t, updated.OnMonday)
	// The days left out of the payload are turned off
	assert.False(t, updated.OnTuesday)
	assert.False(t, updated.OnSunday)
}

func TestDeletingARoutine(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	require.NoError(t, s.Delete(routineID, workspaceID))

	_, err := s.Get(routineID, workspaceID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

// One point for a day achieved, one point lost for a day failed
func TestMarkingADayMovesThePointsByOne(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)

	marked, err := s.MarkDay(routineID, workspaceID, today(), true)
	require.NoError(t, err)
	assert.Equal(t, 1, marked.Routine.Points)
	assert.Equal(t, StatusAchieved, marked.Day.Status)
	assert.Equal(t, core.FormatDate(today()), marked.Day.Day)

	failed, err := s.MarkDay(routineID, workspaceID, core.AddDays(today(), -1), false)
	require.NoError(t, err)
	assert.Equal(t, 0, failed.Routine.Points)
	assert.Equal(t, StatusFailed, failed.Day.Status)
}

// Changing your mind about a day undoes the old point and applies the new one,
// which is a swing of two rather than one
func TestFlippingADayMovesThePointsByTwo(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	day := today()

	_, err := s.MarkDay(routineID, workspaceID, day, true)
	require.NoError(t, err)

	flipped, err := s.MarkDay(routineID, workspaceID, day, false)
	require.NoError(t, err)
	assert.Equal(t, -1, flipped.Routine.Points)

	back, err := s.MarkDay(routineID, workspaceID, day, true)
	require.NoError(t, err)
	assert.Equal(t, 1, back.Routine.Points)
}

// Marking a day the way it already stands is a no-op the client should not send
func TestMarkingADayTwiceTheSameWayIsRefused(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	day := today()

	_, err := s.MarkDay(routineID, workspaceID, day, true)
	require.NoError(t, err)

	_, err = s.MarkDay(routineID, workspaceID, day, true)
	assert.Error(t, err)

	// And the refusal did not move the score
	routine, err := s.Get(routineID, workspaceID)
	require.NoError(t, err)
	assert.Equal(t, 1, routine.Points)
}

// The score is a history, not a prediction: days that have not happened yet
// cannot be marked
func TestAFutureDayCannotBeMarked(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	tomorrow := core.AddDays(today(), 1)

	_, err := s.MarkDay(routineID, workspaceID, tomorrow, true)
	assert.Error(t, err)

	_, err = s.UnmarkDay(routineID, workspaceID, tomorrow)
	assert.Error(t, err)
}

// A day the routine does not run on is not part of the score either
func TestADayOutsideTheScheduleCannotBeMarked(t *testing.T) {
	s := newService(t)

	// Scheduled only on the weekday of today, so yesterday is off schedule
	// unless the two share a weekday, which they never do
	created, err := s.Create(workspaceID, &dto.CreateRoutineReq{
		Name:        "Workout",
		OnMonday:    today().Weekday() == time.Monday,
		OnTuesday:   today().Weekday() == time.Tuesday,
		OnWednesday: today().Weekday() == time.Wednesday,
		OnThursday:  today().Weekday() == time.Thursday,
		OnFriday:    today().Weekday() == time.Friday,
		OnSaturday:  today().Weekday() == time.Saturday,
		OnSunday:    today().Weekday() == time.Sunday,
	})
	require.NoError(t, err)

	_, err = s.MarkDay(created.ID, workspaceID, today(), true)
	require.NoError(t, err)

	_, err = s.MarkDay(created.ID, workspaceID, core.AddDays(today(), -1), true)
	assert.Error(t, err)
}

// Unmarking gives back exactly the point the mark took or gave
func TestUnmarkingGivesThePointBack(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	achieved, failedDay := today(), core.AddDays(today(), -1)

	_, err := s.MarkDay(routineID, workspaceID, achieved, true)
	require.NoError(t, err)
	_, err = s.MarkDay(routineID, workspaceID, failedDay, false)
	require.NoError(t, err)

	after, err := s.UnmarkDay(routineID, workspaceID, achieved)
	require.NoError(t, err)
	assert.Equal(t, -1, after.Points)

	after, err = s.UnmarkDay(routineID, workspaceID, failedDay)
	require.NoError(t, err)
	assert.Equal(t, 0, after.Points, "the score is back where it started")
}

// A day never marked has no point to give back
func TestUnmarkingADayThatWasNeverMarked(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)

	_, err := s.UnmarkDay(routineID, workspaceID, today())
	assert.Error(t, err)
}

// The row is soft deleted, so marking the same day again has to restore it
// rather than collide with the unique index it still holds
func TestADayCanBeMarkedAgainAfterBeingUnmarked(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	day := today()

	_, err := s.MarkDay(routineID, workspaceID, day, true)
	require.NoError(t, err)

	_, err = s.UnmarkDay(routineID, workspaceID, day)
	require.NoError(t, err)

	remarked, err := s.MarkDay(routineID, workspaceID, day, false)
	require.NoError(t, err)
	assert.Equal(t, -1, remarked.Routine.Points)
	assert.Equal(t, StatusFailed, remarked.Day.Status)
}

// The month view always returns every day of the month, marked or not
func TestTheMonthViewCoversTheWholeMonth(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)

	month, err := s.Month(routineID, workspaceID, 2026, time.February)
	require.NoError(t, err)

	assert.Equal(t, routineID, month.RoutineID)
	assert.Equal(t, 2026, month.Year)
	assert.Equal(t, int(time.February), month.Month)
	require.Len(t, month.Days, 28)
	assert.Equal(t, "2026-02-01", month.Days[0].Day)
	assert.Equal(t, "2026-02-28", month.Days[27].Day)

	leap, err := s.Month(routineID, workspaceID, 2028, time.February)
	require.NoError(t, err)
	assert.Len(t, leap.Days, 29)
}

func TestTheMonthViewReportsEachDaysStatus(t *testing.T) {
	s := newService(t)

	// Scheduled on weekdays only, so the weekend is disabled
	created, err := s.Create(workspaceID, &dto.CreateRoutineReq{
		Name:        "Workout",
		OnMonday:    true,
		OnTuesday:   true,
		OnWednesday: true,
		OnThursday:  true,
		OnFriday:    true,
	})
	require.NoError(t, err)

	now := today()

	_, err = s.MarkDay(created.ID, workspaceID, lastScheduledDayBefore(now, 0), true)
	require.NoError(t, err)

	month, err := s.Month(created.ID, workspaceID, now.Year(), now.Month())
	require.NoError(t, err)

	byDay := make(map[string]string, len(month.Days))
	for _, d := range month.Days {
		byDay[d.Day] = d.Status
	}

	assert.Equal(t, StatusAchieved, byDay[core.FormatDate(lastScheduledDayBefore(now, 0))])

	// A weekend day of the same month is off schedule
	for d := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC); d.Month() == now.Month(); d = core.AddDays(d, 1) {
		if d.Weekday() != time.Saturday {
			continue
		}
		assert.Equal(t, StatusDisabled, byDay[core.FormatDate(d)], "%s is a saturday", core.FormatDate(d))
		break
	}

	// A scheduled day still to come is not a failure yet
	future, err := s.Month(created.ID, workspaceID, now.Year()+1, time.June)
	require.NoError(t, err)
	for _, d := range future.Days {
		assert.Contains(t, []string{StatusFuture, StatusDisabled}, d.Status)
	}
}

// A scheduled day already past with nothing registered is still waiting
func TestAPastScheduledDayWithoutAMarkIsWaiting(t *testing.T) {
	s := newService(t)

	routineID := everyDayRoutine(t, s)
	now := today()

	month, err := s.Month(routineID, workspaceID, now.Year(), now.Month())
	require.NoError(t, err)

	assert.Equal(t, StatusWaiting, month.Days[0].Status, "the first of this month is in the past")
}

// lastScheduledDayBefore walks back from d until it lands on a weekday
func lastScheduledDayBefore(d time.Time, back int) time.Time {
	d = core.AddDays(d, -back)
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = core.AddDays(d, -1)
	}
	return d
}
