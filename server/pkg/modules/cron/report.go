package cron

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// runHistorySize is how many of the latest runs the report keeps
const runHistorySize = 3

// possible run status
const (
	statusOk      = "ok"
	statusSkipped = "skipped"
	statusFailed  = "failed"
)

// runRecord describes a single execution of the cleanup job
type runRecord struct {
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
	// Removed is how many sessions the run erased, zero unless it succeeded
	Removed int64  `json:"removed"`
	Status  string `json:"status"`
	// Error tells why a skipped or a failed run cleaned nothing
	Error string `json:"error,omitempty"`
}

// runLog holds the last runHistorySize runs
type runLog struct {
	// mu holds the lock since we can have race between the scheduler write and the endpoint read
	mu   sync.Mutex
	runs []runRecord
}

// record appends a run, dropping the oldest once the log is full
func (l *runLog) record(r runRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.runs = append(l.runs, r)
	if len(l.runs) > runHistorySize {
		l.runs = l.runs[len(l.runs)-runHistorySize:]
	}
}

// latest returns the recorded runs, newest first
func (l *runLog) latest() []runRecord {
	l.mu.Lock()
	defer l.mu.Unlock()

	runs := make([]runRecord, 0, len(l.runs))
	for i := len(l.runs) - 1; i >= 0; i-- {
		runs = append(runs, l.runs[i])
	}

	return runs
}

// reportResponse is what the job has been up to lately
type reportResponse struct {
	Job             string `json:"job"`
	IntervalSeconds int    `json:"interval_seconds"`
	// NextRun is nil while the scheduler is not running
	NextRun *time.Time  `json:"next_run,omitempty"`
	Runs    []runRecord `json:"runs"`
}

func (c *CleanupSessions) handleReport(ctx *gin.Context) {
	res := reportResponse{
		Job:             c.Id(),
		IntervalSeconds: int(cleanupInterval.Seconds()),
		Runs:            c.runs.latest(),
	}

	if c.job != nil {
		if next, err := c.job.NextRun(); err == nil && !next.IsZero() {
			res.NextRun = &next
		}
	}

	ctx.JSON(http.StatusOK, res)
}
