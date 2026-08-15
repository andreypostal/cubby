package core

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blugelabs/bluge"
)

// IndexerFolder is the folder, relative to the data folder, holding every index
const IndexerFolder = "idx"

// indexerPathPattern defines the pattern where the data will be saved
// 1. BasePath from config, 2. WorkspaceID, 3. Index Identifier
const indexerPathPattern = "%s/" + IndexerFolder + "/%d/%s"

type workspaceIndexer struct {
	id      uint
	current Closer
}

func (widx *workspaceIndexer) close() {
	if err := widx.current.Close(); err != nil {
		slog.Warn("[INDEXER] Failed to close a workspace indexer",
			slog.Uint64("workspace", uint64(widx.id)),
			slog.String("error", err.Error()),
		)
	}
}

type workspaceIdxPool struct {
	mu sync.Mutex

	cap int

	handlers map[uint]*list.Element
	list     *list.List

	removalMu   sync.Mutex
	removalTime time.Duration
	removals    map[uint]*time.Timer

	wg map[uint]*WaitGroup

	new func(uint) (*workspaceIndexer, error)

	closed atomic.Bool
}

func newIndexerPool(cap int, removalTime time.Duration, newFunc func(uint) (*workspaceIndexer, error)) *workspaceIdxPool {
	if cap < 1 {
		cap = 4
	}

	return &workspaceIdxPool{
		cap:         cap,
		new:         newFunc,
		handlers:    make(map[uint]*list.Element),
		removals:    make(map[uint]*time.Timer),
		removalTime: removalTime,
		list:        list.New(),
		wg:          make(map[uint]*WaitGroup),
	}
}

func (w *workspaceIdxPool) addInUse(workspaceID uint) DoneFunc {
	if _, ok := w.wg[workspaceID]; !ok {
		w.wg[workspaceID] = NewWaitGroup()
	}

	return w.wg[workspaceID].Add()
}

func (w *workspaceIdxPool) wait(ctx context.Context, workspaceID uint) {
	if wg, ok := w.wg[workspaceID]; ok {
		_ = wg.Wait(ctx)
	}
}

func (w *workspaceIdxPool) get(ctx context.Context, workspaceID uint) (DoneFunc, *workspaceIndexer, error) {
	if w.closed.Load() {
		return nil, nil, errors.New("indexer pool closed")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// schedule this item to be removed after 5 minutes, reset it everytime it is used
	w.scheduleRemoval(ctx, workspaceID)

	if el, ok := w.handlers[workspaceID]; ok {
		// Recently used move to front
		w.list.MoveToFront(el)
		return w.addInUse(workspaceID), el.Value.(*workspaceIndexer), nil
	}

	wIdx, err := w.new(workspaceID)
	if err != nil {
		return nil, nil, err
	}

	w.addNewItem(ctx, workspaceID, wIdx)

	return w.addInUse(workspaceID), wIdx, nil
}

func (w *workspaceIdxPool) inject(ctx context.Context, workspaceID uint, handler Closer) {
	if w.closed.Load() {
		_ = handler.Close()
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.scheduleRemoval(ctx, workspaceID)

	idxer := &workspaceIndexer{
		id:      workspaceID,
		current: handler,
	}

	el, ok := w.handlers[workspaceID]
	if !ok {
		w.addNewItem(ctx, workspaceID, idxer)
		return
	}

	// Ensure it is not closed with inflight actions
	w.wait(ctx, workspaceID)

	// Close the old indexer
	el.Value.(*workspaceIndexer).close()

	// Replace the indexer and update our list
	el.Value = idxer
	w.list.MoveToFront(el)
}

// addNewItem do not acquire mutex any caller must ensure the chain holds a mutex lock
func (w *workspaceIdxPool) addNewItem(ctx context.Context, workspaceID uint, handler any) {
	w.ensureCap(ctx)
	w.handlers[workspaceID] = w.list.PushFront(handler)
}

func (w *workspaceIdxPool) scheduleRemoval(ctx context.Context, workspaceID uint) {
	w.removalMu.Lock()
	defer w.removalMu.Unlock()

	if s, ok := w.removals[workspaceID]; ok {
		s.Reset(w.removalTime)
		return
	}

	timer := time.NewTimer(w.removalTime)
	w.removals[workspaceID] = timer
	go func() {
		defer func() {
			timer.Stop()

			w.removalMu.Lock()
			delete(w.removals, workspaceID)
			w.removalMu.Unlock()
		}()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			slog.Info("[INDEXER] Removing idle indexer from pool", slog.Uint64("workspace", uint64(workspaceID)))

			w.mu.Lock()
			defer w.mu.Unlock()

			el, ok := w.handlers[workspaceID]
			if !ok {
				return
			}

			w.removeItem(ctx, el)
		}
	}()
}

// removeItem do not acquire mutex any caller must ensure the chain holds a mutex lock
func (w *workspaceIdxPool) removeItem(ctx context.Context, el *list.Element) {
	wIdx := el.Value.(*workspaceIndexer)

	w.wait(ctx, wIdx.id)

	wIdx.close()
	w.list.Remove(el)

	delete(w.handlers, wIdx.id)
	delete(w.wg, wIdx.id)
}

// ensureCap do not acquire mutex any caller must ensure the chain holds a mutex lock
func (w *workspaceIdxPool) ensureCap(ctx context.Context) {
	if len(w.handlers) >= w.cap {
		w.removeItem(ctx, w.list.Back())
	}
}

func (w *workspaceIdxPool) resume() {
	if !w.closed.Load() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.closed.Store(false)
}

func (w *workspaceIdxPool) stop(ctx context.Context) {
	if w.closed.CompareAndSwap(false, true) {
		w.mu.Lock()
		defer w.mu.Unlock()

		for _, el := range w.handlers {
			w.removeItem(ctx, el)
		}
	}
}

func buildIndexerPath(identifier, basePath string, workspaceID uint) string {
	return fmt.Sprintf(indexerPathPattern, basePath, workspaceID, identifier)
}

type Indexer struct {
	ctx    context.Context
	cancel context.CancelFunc

	readers *workspaceIdxPool
	writers *workspaceIdxPool

	maxResults int

	paused atomic.Bool
}

type IndexerConfig struct {
	WriterPoolSize int
	WriterIdleTime time.Duration

	ReaderPoolSize int
	ReaderIdleTime time.Duration

	BasePath   string
	Identifier string

	MaxResults int
}

func NewIndexer(ctx context.Context, config IndexerConfig) *Indexer {
	ctx, cancel := context.WithCancel(ctx)

	return &Indexer{
		ctx:    ctx,
		cancel: cancel,

		maxResults: config.MaxResults,
		readers: newIndexerPool(config.ReaderPoolSize, config.ReaderIdleTime, func(u uint) (*workspaceIndexer, error) {
			reader, err := bluge.OpenReader(bluge.DefaultConfig(
				buildIndexerPath(config.Identifier, config.BasePath, u),
			))
			if err != nil {
				return nil, err
			}

			return &workspaceIndexer{
				id:      u,
				current: reader,
			}, nil
		}),
		writers: newIndexerPool(config.WriterPoolSize, config.WriterIdleTime, func(u uint) (*workspaceIndexer, error) {
			writer, err := bluge.OpenWriter(bluge.DefaultConfig(
				buildIndexerPath(config.Identifier, config.BasePath, u),
			))
			if err != nil {
				return nil, err
			}

			return &workspaceIndexer{
				id:      u,
				current: writer,
			}, nil
		}),
	}
}

func (i *Indexer) Insert(workspaceID uint, term string) error {
	if i.paused.Load() {
		return errors.New("indexer is not available")
	}

	done, w, err := i.writers.get(i.ctx, workspaceID)
	if err != nil {
		return err
	}
	defer done()

	writer := w.current.(*bluge.Writer)

	doc := bluge.NewDocument(term).AddField(bluge.NewTextField("term", strings.ToLower(term)))
	if err := writer.Update(doc.ID(), doc); err != nil {
		return err
	}

	// Update the reader only after a successful insert, otherwise we get cached data
	reader, err := writer.Reader()
	if err != nil {
		slog.Warn("[INDEXER] Failed to open reader on insert",
			slog.Uint64("workspace", uint64(workspaceID)),
			slog.String("error", err.Error()),
		)
	} else {
		i.readers.inject(i.ctx, workspaceID, reader)
	}

	return nil
}

func (i *Indexer) Query(ctx context.Context, workspaceID uint, term string) ([]string, error) {
	if i.paused.Load() {
		return nil, errors.New("indexer is not available")
	}

	done, r, err := i.readers.get(i.ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	defer done()

	reader := r.current.(*bluge.Reader)

	words := strings.Fields(strings.ToLower(term))
	q := bluge.NewBooleanQuery()
	for _, w := range words {
		q.AddMust(bluge.NewPrefixQuery(w).SetField("term"))
	}

	match, err := reader.Search(ctx, bluge.NewTopNSearch(i.maxResults, q))
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, i.maxResults)
	for {
		m, err := match.Next()
		if m == nil || err != nil {
			break
		}

		_ = m.VisitStoredFields(func(field string, value []byte) bool {
			if field == "_id" {
				out = append(out, string(value))
				return false
			}
			return true
		})
	}

	return out, nil
}

func (i *Indexer) Pause(ctx context.Context) {
	if !i.paused.Swap(true) {
		i.Close(ctx)
	}
}

func (i *Indexer) Resume(ctx context.Context) {
	i.readers.resume()
	i.writers.resume()

	i.ctx, i.cancel = context.WithCancel(ctx)

	i.paused.Store(false)
}

func (i *Indexer) Close(ctx context.Context) {
	i.cancel()

	i.readers.stop(ctx)
	i.writers.stop(ctx)
}
