package core

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newIndexer builds an indexer over a data folder of its own. poolSize is what
// decides how many workspaces stay open at once.
func newIndexer(t *testing.T, poolSize int) *Indexer {
	t.Helper()

	// The paths are built by hand, so the drive letter has to come with forward
	// slashes like the rest of the data folder
	dataDir := path.Join(filepath.ToSlash(t.TempDir()), "data")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	idx := NewIndexer(context.Background(), IndexerConfig{
		Identifier: "items",
		BasePath:   dataDir,

		WriterPoolSize: poolSize,
		ReaderPoolSize: poolSize,
		WriterIdleTime: time.Minute,
		ReaderIdleTime: time.Minute,

		MaxResults: 10,
	})
	t.Cleanup(func() { idx.Close(context.Background()) })

	return idx
}

func query(t *testing.T, idx *Indexer, workspaceID uint, term string) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	found, err := idx.Query(ctx, workspaceID, term)
	require.NoError(t, err)

	return found
}

// A term is searchable by any prefix of it, which is what drives the suggestions
// while somebody is still typing
func TestATermIsFoundByItsPrefix(t *testing.T) {
	idx := newIndexer(t, 2)

	require.NoError(t, idx.Insert(1, "Milk"))

	for _, prefix := range []string{"m", "mi", "mil", "milk", "MILK"} {
		assert.Contains(t, query(t, idx, 1, prefix), "Milk", "searching for %q", prefix)
	}

	assert.Empty(t, query(t, idx, 1, "bread"))
}

// The stored term keeps the casing it was written with, only the matching is
// case insensitive
func TestTheSuggestionKeepsItsOriginalCasing(t *testing.T) {
	idx := newIndexer(t, 2)

	require.NoError(t, idx.Insert(1, "Olive Oil"))

	assert.Equal(t, []string{"Olive Oil"}, query(t, idx, 1, "olive"))
}

// Every word of the query has to match, so a second word narrows the results
func TestEveryWordOfTheQueryHasToMatch(t *testing.T) {
	idx := newIndexer(t, 2)

	require.NoError(t, idx.Insert(1, "Olive Oil"))
	require.NoError(t, idx.Insert(1, "Sunflower Oil"))

	assert.Len(t, query(t, idx, 1, "oil"), 2)
	assert.Equal(t, []string{"Olive Oil"}, query(t, idx, 1, "oli oil"))
	assert.Empty(t, query(t, idx, 1, "olive butter"))
}

// The term is the document id, so writing it twice leaves one suggestion
func TestTheSameTermIsStoredOnlyOnce(t *testing.T) {
	idx := newIndexer(t, 2)

	require.NoError(t, idx.Insert(1, "Milk"))
	require.NoError(t, idx.Insert(1, "Milk"))

	assert.Equal(t, []string{"Milk"}, query(t, idx, 1, "milk"))
}

// Each workspace holds its own index on disk and never sees the others' terms
func TestWorkspacesDoNotShareTerms(t *testing.T) {
	idx := newIndexer(t, 4)

	require.NoError(t, idx.Insert(1, "Milk"))
	require.NoError(t, idx.Insert(2, "Bread"))

	assert.Equal(t, []string{"Milk"}, query(t, idx, 1, "m"))
	assert.Equal(t, []string{"Bread"}, query(t, idx, 2, "b"))
	assert.Empty(t, query(t, idx, 1, "bread"))
	assert.Empty(t, query(t, idx, 2, "milk"))
}

// A workspace that never had anything written has no index to open, and that
// has to be an error rather than an empty answer, so the caller can tell them apart
func TestQueryingAWorkspaceWithNoIndexFails(t *testing.T) {
	idx := newIndexer(t, 2)

	_, err := idx.Query(context.Background(), 99, "milk")
	assert.Error(t, err)
}

// The pool only keeps a few workspaces open at a time, and evicting one must not
// cost it its terms
func TestTermsSurviveTheirWorkspaceLeavingThePool(t *testing.T) {
	idx := newIndexer(t, 1)

	require.NoError(t, idx.Insert(1, "Milk"))
	// The second workspace pushes the first one out of a pool that holds one
	require.NoError(t, idx.Insert(2, "Bread"))

	assert.Equal(t, []string{"Bread"}, query(t, idx, 2, "b"))
	assert.Equal(t, []string{"Milk"}, query(t, idx, 1, "m"))
}

// Pausing is what lets a backup copy the index files, so nothing may touch them
// until it resumes
func TestAPausedIndexerRefusesWorkAndPicksUpAfterwards(t *testing.T) {
	idx := newIndexer(t, 2)

	require.NoError(t, idx.Insert(1, "Milk"))

	idx.Pause(context.Background())

	assert.Error(t, idx.Insert(1, "Bread"))

	_, err := idx.Query(context.Background(), 1, "milk")
	assert.Error(t, err)

	// Pausing twice is what happens when two backups overlap
	idx.Pause(context.Background())

	idx.Resume(context.Background())

	require.NoError(t, idx.Insert(1, "Bread"))
	assert.Equal(t, []string{"Milk"}, query(t, idx, 1, "milk"))
	assert.Equal(t, []string{"Bread"}, query(t, idx, 1, "bread"))
}

// A closed indexer is done: it refuses work instead of reopening the files
func TestAClosedIndexerRefusesWork(t *testing.T) {
	idx := newIndexer(t, 2)

	require.NoError(t, idx.Insert(1, "Milk"))

	idx.Close(context.Background())

	assert.Error(t, idx.Insert(1, "Bread"))

	_, err := idx.Query(context.Background(), 1, "milk")
	assert.Error(t, err)

	// Closing twice is harmless, the module shutdown may run after a pause
	assert.NotPanics(t, func() { idx.Close(context.Background()) })
}

// Every index lives under the data folder, in a folder of its own per workspace
func TestIndexPathsAreScopedByWorkspace(t *testing.T) {
	assert.Equal(t, "data/"+IndexerFolder+"/1/items", buildIndexerPath("items", "data", 1))
	assert.Equal(t, "data/"+IndexerFolder+"/42/ingredients", buildIndexerPath("ingredients", "data", 42))

	// Two identifiers of the same workspace never collide
	assert.NotEqual(t, buildIndexerPath("items", "data", 1), buildIndexerPath("ingredients", "data", 1))
}
