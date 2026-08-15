package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrFrozen        = errors.New("system is busy")
	ErrAlreadyFrozen = errors.New("system already frozen")
)

type UnfreezeFunc func()

type Freezer struct {
	frozen    atomic.Bool
	exclusive atomic.Bool
	reason    string

	mu     sync.Mutex
	wg     *WaitGroup
	readWg *WaitGroup
}

func (f *Freezer) Id() string {
	return "system-freezer"
}

func (f *Freezer) New() any {
	nf := new(Freezer)
	nf.wg = NewWaitGroup()
	nf.readWg = NewWaitGroup()

	return nf
}

func (f *Freezer) Enter() (DoneFunc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.frozen.Load() {
		return nil, fmt.Errorf("%w: %s", ErrFrozen, f.reason)
	}

	return f.wg.Add(), nil
}

func (f *Freezer) EnterRead() (DoneFunc, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.exclusive.Load() {
		return nil, fmt.Errorf("%w: %s", ErrFrozen, f.reason)
	}

	return f.readWg.Add(), nil
}

// Freeze waits all inflight tasks and returns a idempotent UnfreezeFunc that releases the freeze lock acquired
// or an error when the application is already frozen or context is closed
func (f *Freezer) Freeze(ctx context.Context, reason string) (UnfreezeFunc, error) {
	return f.freeze(ctx, reason, false)
}

// FreezeAll blocks new reads and writes, and wait for inflight actions to end
func (f *Freezer) FreezeAll(ctx context.Context, reason string) (UnfreezeFunc, error) {
	return f.freeze(ctx, reason, true)
}

func (f *Freezer) freeze(ctx context.Context, reason string, exclusive bool) (UnfreezeFunc, error) {
	if !f.frozen.CompareAndSwap(false, true) {
		f.mu.Lock()
		defer f.mu.Unlock()

		return nil, fmt.Errorf("%w: %s", ErrAlreadyFrozen, f.reason)
	}

	f.mu.Lock()
	f.reason = reason
	f.mu.Unlock()

	// Readers are turned away before the wait starts
	f.exclusive.Store(exclusive)

	unfreeze := func() {
		f.mu.Lock()
		f.reason = ""
		f.exclusive.Store(false)
		f.frozen.Store(false)
		f.mu.Unlock()
	}

	requestedAt := time.Now()

	// Wait for all inflight requests
	if err := f.wg.Wait(ctx); err != nil {
		// In this case we are forced to unfreeze
		unfreeze()

		slog.Warn("Gave up waiting on inflight requests to freeze the system",
			slog.String("reason", reason),
			slog.Duration("waited", time.Since(requestedAt)),
			slog.String("error", err.Error()),
		)

		return nil, err
	}

	if exclusive {
		if err := f.readWg.Wait(ctx); err != nil {
			unfreeze()
			slog.Warn("Gave up waiting on inflight reads to freeze the system",
				slog.String("reason", reason),
				slog.Duration("waited", time.Since(requestedAt)),
				slog.String("error", err.Error()),
			)

			return nil, err
		}
	}

	slog.Info("System frozen",
		slog.String("reason", reason),
		slog.Bool("reads_blocked", exclusive),
		slog.Duration("waited", time.Since(requestedAt)),
	)

	frozenAt := time.Now()
	return sync.OnceFunc(func() {
		unfreeze()

		slog.Info("System resumed",
			slog.String("reason", reason),
			slog.Duration("held", time.Since(frozenAt)),
		)
	}), nil
}
