package cron

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron/v2"
)

// cleanupInterval is how often expired sessions are swept from the database
const cleanupInterval = time.Hour

type CleanupSessions struct {
	scheduler gocron.Scheduler
	// job is the scheduled cleanup, kept so the report can tell when it runs next
	job gocron.Job

	sessionRepo repository.SessionRepository
	freezer     *core.Freezer

	// runs is what the report is built from
	runs runLog
}

func (c *CleanupSessions) Id() string {
	return "cleanup-sessions-cron"
}

func (c *CleanupSessions) New() any {
	return new(CleanupSessions)
}

func (c *CleanupSessions) Initialize(ctx *core.Context) error {
	db, err := core.GetModule[*core.DB](ctx, new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}
	c.sessionRepo = repository.NewSessionRepository(db)

	freezer, err := core.GetModule[*core.Freezer](ctx, new(core.Freezer))
	if err != nil {
		return fmt.Errorf("getting system freezer: %w", err)
	}
	c.freezer = freezer

	s, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("init gocron scheduler: %w", err)
	}
	c.scheduler = s

	return nil
}

func (c *CleanupSessions) Routes() ([]api.Route, error) {
	return []api.Route{
		{
			Method:    http.MethodGet,
			Path:      "/cron/sessions-cleanup",
			Handlers:  []gin.HandlerFunc{c.handleReport},
			AuthScope: api.SystemAdminRouteType,
		},
	}, nil
}

func (c *CleanupSessions) Start() error {
	job, err := c.scheduler.NewJob(
		gocron.DurationJob(cleanupInterval),
		gocron.NewTask(c.cleanup),
		gocron.WithName(c.Id()),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
		// Sessions expired while the server was down are swept on startup
		gocron.WithStartAt(gocron.WithStartImmediately()),
	)
	if err != nil {
		return fmt.Errorf("scheduling the expired sessions cleanup: %w", err)
	}
	c.job = job

	c.scheduler.Start()
	return nil
}

// cleanup removes the sessions already past their expiration date, recording
// how it went for the report
func (c *CleanupSessions) cleanup() {
	startedAt := time.Now()
	elapsed := func() int64 { return time.Since(startedAt).Milliseconds() }

	// Recover before releasing the freeze to ensure that the freeze gate
	// is unlocked
	defer func() {
		data := recover()
		if data == nil {
			return
		}

		slog.Error("panic recovered",
			"error", data,
			"job", c.Id(),
			"stack", string(debug.Stack()),
		)

		c.runs.record(runRecord{
			StartedAt:  startedAt,
			DurationMs: elapsed(),
			Status:     statusFailed,
			Error:      fmt.Sprintf("panic: %v", data),
		})
	}()

	done, err := c.freezer.Enter()
	if err != nil {
		// A backup or a restore holds the database, the next run picks it up
		slog.Warn("Skipped the expired sessions cleanup", slog.String("error", err.Error()))
		c.runs.record(runRecord{
			StartedAt:  startedAt,
			DurationMs: elapsed(),
			Status:     statusSkipped,
			Error:      err.Error(),
		})
		return
	}
	defer done()

	removed, err := c.sessionRepo.DeleteExpired(time.Now())
	if err != nil {
		slog.Error("Failed to clean up expired sessions", slog.String("error", err.Error()))
		c.runs.record(runRecord{
			StartedAt:  startedAt,
			DurationMs: elapsed(),
			Status:     statusFailed,
			Error:      err.Error(),
		})
		return
	}

	c.runs.record(runRecord{
		StartedAt:  startedAt,
		DurationMs: elapsed(),
		Removed:    removed,
		Status:     statusOk,
	})

	if removed > 0 {
		slog.Info("Removed expired sessions", slog.Int64("count", removed))
	}
}

func (c *CleanupSessions) Stop(context.Context) error {
	return c.scheduler.Shutdown()
}
