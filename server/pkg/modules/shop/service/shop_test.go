package service

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	authModels "andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/shop/dto"
	"andreypostal.com/cubby/server/pkg/modules/shop/models"
	"andreypostal.com/cubby/server/pkg/modules/shop/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The workspaces the tests keep their lists in. Everything a service answers is
// scoped to one of them, so a second one is what proves the scoping holds.
const (
	workspaceID = uint(1)
	otherWSID   = uint(2)
)

// testEnv holds the two services over a database and a search index of their own
type testEnv struct {
	lists ListService
	items ListItemService
}

func newTestEnv(t *testing.T) *testEnv {
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

	// The same tables the shop module migrates on startup
	require.NoError(t, db.AutoMigrate(&models.List{}, &models.ListItem{}))

	// A list belongs to a workspace, and the foreign key is enforced, so the two
	// workspaces the tests use have to be real rows
	owner := &authModels.User{Username: "owner", Password: "x", Name: "Owner"}
	require.NoError(t, db.Create(owner).Error)

	for _, id := range []uint{workspaceID, otherWSID} {
		workspace := &authModels.Workspace{
			Model:     gorm.Model{ID: id},
			Name:      "Workspace",
			CreatorID: owner.ID,
		}
		require.NoError(t, db.Create(workspace).Error)
	}

	indexer := core.NewIndexer(context.Background(), core.IndexerConfig{
		Identifier: "items",
		BasePath:   dataDir,

		WriterPoolSize: 1,
		ReaderPoolSize: 1,
		WriterIdleTime: time.Minute,
		ReaderIdleTime: time.Minute,

		MaxResults: 10,
	})
	t.Cleanup(func() { indexer.Close(context.Background()) })

	listRepo := repository.NewListRepository(db)
	itemRepo := repository.NewListItemRepository(db)

	return &testEnv{
		lists: NewListService(listRepo, itemRepo),
		items: NewListItemService(itemRepo, indexer, 5*time.Second),
	}
}

// list creates a list in the main workspace and hands back its id
func (e *testEnv) list(t *testing.T, name string) uint {
	t.Helper()

	created, err := e.lists.Create(workspaceID, &dto.CreateListReq{Name: name, Icon: "cart"})
	require.NoError(t, err)

	return created.ID
}

// item puts a product in a list and hands back its id
func (e *testEnv) item(t *testing.T, listID uint, name string) uint {
	t.Helper()

	created, err := e.items.Create(workspaceID, listID, &dto.CreateListItemReq{Name: name})
	require.NoError(t, err)

	return created.ID
}

func TestListsAreScopedToTheirWorkspace(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")

	mine, err := env.lists.ListForWorkspace(workspaceID)
	require.NoError(t, err)
	require.Len(t, mine.Lists, 1)
	assert.Equal(t, "Groceries", mine.Lists[0].Name)

	theirs, err := env.lists.ListForWorkspace(otherWSID)
	require.NoError(t, err)
	assert.Empty(t, theirs.Lists)

	// Reading, writing and deleting are all refused from the other workspace
	_, err = env.lists.Get(listID, otherWSID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.lists.Update(listID, otherWSID, &dto.UpdateListReq{Name: "Theirs", Icon: "cart"})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.lists.Delete(listID, otherWSID), core.ErrNotAllowed)
}

func TestDeletingAList(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	require.NoError(t, env.lists.Delete(listID, workspaceID))

	_, err := env.lists.Get(listID, workspaceID)
	assert.ErrorIs(t, err, core.ErrResourceNotFound)
}

// A new item carries nothing but its name, the note comes later
func TestCreatingAnItem(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")

	created, err := env.items.Create(workspaceID, listID, &dto.CreateListItemReq{Name: "Milk"})
	require.NoError(t, err)

	assert.Equal(t, "Milk", created.Name)
	assert.Empty(t, created.Notes)
	assert.False(t, created.Bought)
	assert.True(t, created.BoughtAt.IsZero())

	// And it shows up when the list is read back
	list, err := env.lists.Get(listID, workspaceID)
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, created.ID, list.Items[0].ID)
}

func TestNotesAreWrittenAndCleared(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	itemID := env.item(t, listID, "Rice")

	noted, err := env.items.Update(itemID, listID, &dto.UpdateListItemReq{Notes: notes("500g")})
	require.NoError(t, err)
	assert.Equal(t, "500g", noted.Notes)
	// Updating only the note leaves the name alone
	assert.Equal(t, "Rice", noted.Name)

	// An empty note is a request to clear it, which is why the field is a pointer
	cleared, err := env.items.Update(itemID, listID, &dto.UpdateListItemReq{Notes: notes("")})
	require.NoError(t, err)
	assert.Empty(t, cleared.Notes)

	// An absent note is not a request to clear anything
	renamed, err := env.items.Update(itemID, listID, &dto.UpdateListItemReq{Name: "Brown rice"})
	require.NoError(t, err)
	assert.Equal(t, "Brown rice", renamed.Name)

	_, err = env.items.Update(itemID, listID, &dto.UpdateListItemReq{Notes: notes("1kg")})
	require.NoError(t, err)

	stored, err := env.items.Get(itemID, listID)
	require.NoError(t, err)
	assert.Equal(t, "1kg", stored.Notes)
	assert.Equal(t, "Brown rice", stored.Name)
}

// An update that carries nothing is refused rather than writing the row again
func TestUpdatingAnItemWithNothingToChange(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	itemID := env.item(t, listID, "Rice")

	_, err := env.items.Update(itemID, listID, &dto.UpdateListItemReq{})
	assert.Error(t, err)

	// The same note it already carries is not a change either
	_, err = env.items.Update(itemID, listID, &dto.UpdateListItemReq{Name: "Rice"})
	assert.Error(t, err)
}

func TestTogglingAnItemBothWays(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	itemID := env.item(t, listID, "Milk")

	bought, err := env.items.ToggleBought(itemID, listID)
	require.NoError(t, err)
	assert.True(t, bought.Bought)
	assert.False(t, bought.BoughtAt.IsZero())

	back, err := env.items.ToggleBought(itemID, listID)
	require.NoError(t, err)
	assert.False(t, back.Bought)
	assert.True(t, back.BoughtAt.IsZero(), "the bought date outlived the toggle")
}

func TestClearingBoughtItems(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	milk := env.item(t, listID, "Milk")
	rice := env.item(t, listID, "Rice")

	_, err := env.items.ToggleBought(milk, listID)
	require.NoError(t, err)

	remaining, err := env.items.ClearBought(listID)
	require.NoError(t, err)

	// Clearing keeps every item, it only puts them back on the to buy side
	require.Len(t, remaining, 2)
	for _, item := range remaining {
		assert.False(t, item.Bought)
		assert.True(t, item.BoughtAt.IsZero())
	}

	// The one that was never bought is untouched
	untouched, err := env.items.Get(rice, listID)
	require.NoError(t, err)
	assert.False(t, untouched.Bought)
}

// The list id in the route has to be the one holding the item, otherwise the
// items of any list would be reachable through a list the caller does own
func TestItemsAreScopedToTheirList(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	otherID := env.list(t, "Hardware")
	itemID := env.item(t, listID, "Milk")

	_, err := env.items.Get(itemID, otherID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.items.Update(itemID, otherID, &dto.UpdateListItemReq{Notes: notes("1L")})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.items.ToggleBought(itemID, otherID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.items.Delete(itemID, otherID), core.ErrNotAllowed)

	// None of the refusals touched it
	item, err := env.items.Get(itemID, listID)
	require.NoError(t, err)
	assert.Empty(t, item.Notes)
	assert.False(t, item.Bought)
}

func TestDeletingAnItem(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	itemID := env.item(t, listID, "Milk")

	require.NoError(t, env.items.Delete(itemID, listID))

	_, err := env.items.Get(itemID, listID)
	assert.ErrorIs(t, err, core.ErrResourceNotFound)

	list, err := env.lists.Get(listID, workspaceID)
	require.NoError(t, err)
	assert.Empty(t, list.Items)
}

// Creating an item feeds the workspace index, which is what the add product
// field reads back for its suggestions
func TestItemNamesAreSuggestedBackToTheWorkspace(t *testing.T) {
	env := newTestEnv(t)

	listID := env.list(t, "Groceries")
	env.item(t, listID, "Milk")

	found, err := env.items.Suggest(workspaceID, "mil")
	require.NoError(t, err)
	assert.Contains(t, found.Terms, "Milk")

	// The index belongs to one workspace, another one never sees those terms
	_, err = env.items.Suggest(otherWSID, "mil")
	assert.Error(t, err)
}

func notes(value string) *string {
	return &value
}
