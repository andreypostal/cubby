package cron

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
)

// fakeSessions stands in for the session repository, the job only sweeps
type fakeSessions struct {
	removed int64
	err     error
	panics  bool
	calls   int
}

func (f *fakeSessions) Create(*models.Session) error               { return nil }
func (f *fakeSessions) FindByCode(string) (*models.Session, error) { return nil, nil }
func (f *fakeSessions) Update(*models.Session) error               { return nil }
func (f *fakeSessions) Delete(string) error                        { return nil }
func (f *fakeSessions) DeleteForUser(uint) (int64, error)          { return 0, nil }

func (f *fakeSessions) DeleteExpired(time.Time) (int64, error) {
	f.calls++
	if f.panics {
		panic("the driver blew up")
	}
	return f.removed, f.err
}

func newModule(repo *fakeSessions) *CleanupSessions {
	return &CleanupSessions{
		sessionRepo: repo,
		freezer:     new(core.Freezer).New().(*core.Freezer),
	}
}

// A server that has not swept yet reports no runs rather than failing
func TestReportWithoutRuns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	res := report(t, newModule(&fakeSessions{}))

	if res.Job != "cleanup-sessions-cron" {
		t.Fatalf("unexpected job name %q", res.Job)
	}

	if res.IntervalSeconds != int(cleanupInterval.Seconds()) {
		t.Fatalf("unexpected interval %d", res.IntervalSeconds)
	}

	if len(res.Runs) != 0 {
		t.Fatalf("a module that never ran reports %d runs", len(res.Runs))
	}
}

// Only the last three runs are kept, newest first
func TestReportKeepsTheLastRuns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &fakeSessions{}
	c := newModule(repo)

	for i := 1; i <= runHistorySize+2; i++ {
		repo.removed = int64(i)
		c.cleanup()
	}

	res := report(t, c)

	if len(res.Runs) != runHistorySize {
		t.Fatalf("the report holds %d runs, expected %d", len(res.Runs), runHistorySize)
	}

	// The 5th run swept 5 sessions, the 4th swept 4, and so on
	for i, run := range res.Runs {
		want := int64(runHistorySize + 2 - i)
		if run.Removed != want {
			t.Fatalf("run %d removed %d, expected %d", i, run.Removed, want)
		}

		if run.Status != statusOk {
			t.Fatalf("run %d is reported as %q", i, run.Status)
		}
	}
}

// A run that could not sweep says so, and says why
func TestReportRecordsAFailedRun(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c := newModule(&fakeSessions{err: errors.New("database is locked")})
	c.cleanup()

	res := report(t, c)

	if len(res.Runs) != 1 {
		t.Fatalf("the report holds %d runs, expected 1", len(res.Runs))
	}

	run := res.Runs[0]
	if run.Status != statusFailed {
		t.Fatalf("a failed run is reported as %q", run.Status)
	}

	if run.Error != "database is locked" {
		t.Fatalf("unexpected error %q", run.Error)
	}

	if run.Removed != 0 {
		t.Fatalf("a failed run reports %d removed sessions", run.Removed)
	}
}

// A panicking sweep must not take the server down with it, and it is reported
// like any other failure
func TestReportRecordsAPanickingRun(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c := newModule(&fakeSessions{panics: true})
	c.cleanup()

	res := report(t, c)

	if len(res.Runs) != 1 {
		t.Fatalf("the report holds %d runs, expected 1", len(res.Runs))
	}

	run := res.Runs[0]
	if run.Status != statusFailed {
		t.Fatalf("a panicking run is reported as %q", run.Status)
	}

	if run.Error != "panic: the driver blew up" {
		t.Fatalf("unexpected error %q", run.Error)
	}
}

// The freezer must be released even when the sweep panics while holding it,
// otherwise no backup could ever freeze the system again
func TestAPanickingRunReleasesTheFreezer(t *testing.T) {
	c := newModule(&fakeSessions{panics: true})
	c.cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	unfreeze, err := c.freezer.Freeze(ctx, "backup")
	if err != nil {
		t.Fatalf("the freezer is still held by the panicking run: %v", err)
	}
	unfreeze()
}

// A backup holding the database postpones the sweep instead of failing it
func TestReportRecordsASkippedRun(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &fakeSessions{}
	c := newModule(repo)

	unfreeze, err := c.freezer.Freeze(context.Background(), "backup")
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	defer unfreeze()

	c.cleanup()

	if repo.calls != 0 {
		t.Fatal("the sweep touched the database while it was frozen")
	}

	res := report(t, c)
	if len(res.Runs) != 1 || res.Runs[0].Status != statusSkipped {
		t.Fatalf("a frozen system was not reported as skipped: %+v", res.Runs)
	}
}

// The report is for system admins, and it only reports
func TestReportRouteIsSystemAdminOnly(t *testing.T) {
	routes, err := newModule(&fakeSessions{}).Routes()
	if err != nil {
		t.Fatalf("routes: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("the module exposes %d routes", len(routes))
	}

	if routes[0].AuthScope != api.SystemAdminRouteType {
		t.Fatalf("the report is exposed to %q", routes[0].AuthScope)
	}

	if routes[0].Method != http.MethodGet {
		t.Fatalf("the module exposes %s %s", routes[0].Method, routes[0].Path)
	}
}

func report(t *testing.T, c *CleanupSessions) reportResponse {
	t.Helper()

	engine := gin.New()
	routes, err := c.Routes()
	if err != nil {
		t.Fatalf("routes: %v", err)
	}

	for _, route := range routes {
		engine.Handle(route.Method, route.Path, route.Handlers...)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cron/sessions-cleanup", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("the report answered %d: %s", rec.Code, rec.Body)
	}

	var res reportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("reading the response: %v", err)
	}

	return res
}
