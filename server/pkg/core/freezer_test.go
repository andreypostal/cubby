package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFreezer() *Freezer {
	return new(Freezer).New().(*Freezer)
}

func TestAnIdleSystemFreezesImmediately(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.Freeze(context.Background(), "backup")
	require.NoError(t, err)

	// Writes are turned away while frozen, and the reason travels with the error
	_, err = f.Enter()
	assert.ErrorIs(t, err, ErrFrozen)
	assert.Contains(t, err.Error(), "backup")

	unfreeze()

	done, err := f.Enter()
	require.NoError(t, err)
	done()
}

// A plain freeze is for backups: writes stop, but the UI can still be read
func TestAPlainFreezeStillLetsReadsThrough(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.Freeze(context.Background(), "backup")
	require.NoError(t, err)
	defer unfreeze()

	done, err := f.EnterRead()
	require.NoError(t, err)
	done()
}

// A restore rewrites the database underneath, so readers are turned away too
func TestFreezeAllTurnsReadsAwayAsWell(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.FreezeAll(context.Background(), "restore")
	require.NoError(t, err)

	_, err = f.EnterRead()
	assert.ErrorIs(t, err, ErrFrozen)

	_, err = f.Enter()
	assert.ErrorIs(t, err, ErrFrozen)

	unfreeze()

	done, err := f.EnterRead()
	require.NoError(t, err)
	done()
}

// The freeze is what makes a backup consistent, so it may not start until the
// requests already inside have finished
func TestFreezeWaitsForTheRequestsAlreadyInside(t *testing.T) {
	f := newFreezer()

	done, err := f.Enter()
	require.NoError(t, err)

	frozen := make(chan error, 1)
	go func() {
		_, err := f.Freeze(context.Background(), "backup")
		frozen <- err
	}()

	select {
	case <-frozen:
		t.Fatal("froze while a request was still inflight")
	case <-time.After(50 * time.Millisecond):
	}

	done()

	select {
	case err := <-frozen:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("freeze never completed after the request left")
	}
}

// FreezeAll waits on the readers too, and only after the writers are out
func TestFreezeAllWaitsForReadersAsWell(t *testing.T) {
	f := newFreezer()

	doneRead, err := f.EnterRead()
	require.NoError(t, err)

	frozen := make(chan error, 1)
	go func() {
		_, err := f.FreezeAll(context.Background(), "restore")
		frozen <- err
	}()

	select {
	case <-frozen:
		t.Fatal("froze while a read was still inflight")
	case <-time.After(50 * time.Millisecond):
	}

	doneRead()

	select {
	case err := <-frozen:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("freeze never completed after the read left")
	}
}

// A request that never finishes must not leave the system frozen forever
func TestAFreezeThatTimesOutReleasesTheSystem(t *testing.T) {
	f := newFreezer()

	done, err := f.Enter()
	require.NoError(t, err)
	defer done()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	unfreeze, err := f.Freeze(ctx, "backup")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, unfreeze)

	// Having given up, the system is usable again
	second, err := f.Enter()
	require.NoError(t, err)
	second()
}

func TestOnlyOneFreezeAtATime(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.Freeze(context.Background(), "backup")
	require.NoError(t, err)

	_, err = f.Freeze(context.Background(), "restore")
	assert.ErrorIs(t, err, ErrAlreadyFrozen)
	assert.Contains(t, err.Error(), "backup", "the error should name the freeze already in place")

	unfreeze()

	unfreeze, err = f.Freeze(context.Background(), "restore")
	require.NoError(t, err)
	unfreeze()
}

// The handler releases its slot early and the middleware releases it again on
// the way out, so releasing twice has to be harmless
func TestReleasingTheSameSlotTwiceIsHarmless(t *testing.T) {
	f := newFreezer()

	done, err := f.Enter()
	require.NoError(t, err)

	done()
	done()

	// The counter did not go negative: a freeze still completes right away
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	unfreeze, err := f.Freeze(ctx, "backup")
	require.NoError(t, err)
	unfreeze()
}

func TestUnfreezingTwiceIsHarmless(t *testing.T) {
	f := newFreezer()

	unfreeze, err := f.Freeze(context.Background(), "backup")
	require.NoError(t, err)

	unfreeze()
	unfreeze()

	done, err := f.Enter()
	require.NoError(t, err)
	done()
}

// The wait group is reusable: a second round of requests can be waited on again
func TestWaitGroupIsReusable(t *testing.T) {
	wg := NewWaitGroup()

	require.NoError(t, wg.Wait(context.Background()))

	for range 2 {
		done := wg.Add()

		waited := make(chan error, 1)
		go func() { waited <- wg.Wait(context.Background()) }()

		select {
		case <-waited:
			t.Fatal("wait returned while a task was still inflight")
		case <-time.After(20 * time.Millisecond):
		}

		done()

		select {
		case err := <-waited:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("wait never returned")
		}
	}
}

func TestWaitGroupWaitsForEveryTask(t *testing.T) {
	wg := NewWaitGroup()

	first, second := wg.Add(), wg.Add()

	waited := make(chan error, 1)
	go func() { waited <- wg.Wait(context.Background()) }()

	first()

	select {
	case <-waited:
		t.Fatal("wait returned with one task still inflight")
	case <-time.After(20 * time.Millisecond):
	}

	second()
	require.NoError(t, <-waited)
}

func TestWaitGroupGivesUpWithTheContext(t *testing.T) {
	wg := NewWaitGroup()

	done := wg.Add()
	defer done()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.ErrorIs(t, wg.Wait(ctx), context.Canceled)
}
