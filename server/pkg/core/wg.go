package core

import (
	"context"
	"sync"
)

type DoneFunc func()

type WaitGroup struct {
	inflight int
	mu       sync.Mutex
	empty    chan struct{}
}

func NewWaitGroup() *WaitGroup {
	return &WaitGroup{
		empty: make(chan struct{}),
	}
}

func (wg *WaitGroup) Add() DoneFunc {
	wg.mu.Lock()
	wg.inflight++
	wg.mu.Unlock()

	return sync.OnceFunc(func() {
		wg.mu.Lock()
		defer wg.mu.Unlock()

		wg.inflight--
		if wg.inflight == 0 {
			close(wg.empty)
			wg.empty = make(chan struct{})
		}
	})
}

func (wg *WaitGroup) Wait(ctx context.Context) error {
	wg.mu.Lock()
	if wg.inflight == 0 {
		wg.mu.Unlock()
		return nil
	}
	emptyCh := wg.empty
	wg.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-emptyCh:
		return nil
	}
}
