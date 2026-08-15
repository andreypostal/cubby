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
	"andreypostal.com/cubby/server/pkg/modules/recipes/dto"
	"andreypostal.com/cubby/server/pkg/modules/recipes/models"
	"andreypostal.com/cubby/server/pkg/modules/recipes/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The workspaces the tests keep their recipes in. A second one is what proves
// the scoping holds.
const (
	workspaceID = uint(1)
	otherWSID   = uint(2)
)

// testEnv holds the two services over a database and a search index of their own
type testEnv struct {
	recipes    RecipeService
	categories CategoryService
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

	// The same tables the recipes module migrates on startup
	require.NoError(t, db.AutoMigrate(
		&models.Category{}, &models.Recipe{}, &models.Ingredient{}, &models.Step{},
	))

	// A recipe belongs to a workspace and the foreign key is enforced
	owner := &authModels.User{Username: "owner", Password: "x", Name: "Owner"}
	require.NoError(t, db.Create(owner).Error)

	for _, id := range []uint{workspaceID, otherWSID} {
		require.NoError(t, db.Create(&authModels.Workspace{
			Model:     gorm.Model{ID: id},
			Name:      "Workspace",
			CreatorID: owner.ID,
		}).Error)
	}

	indexer := core.NewIndexer(context.Background(), core.IndexerConfig{
		Identifier: "ingredients",
		BasePath:   dataDir,

		WriterPoolSize: 1,
		ReaderPoolSize: 1,
		WriterIdleTime: time.Minute,
		ReaderIdleTime: time.Minute,

		MaxResults: 10,
	})
	t.Cleanup(func() { indexer.Close(context.Background()) })

	recipeRepo := repository.NewRecipeRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)

	return &testEnv{
		recipes:    NewRecipeService(recipeRepo, categoryRepo, indexer, 5*time.Second),
		categories: NewCategoryService(categoryRepo),
	}
}

// recipe creates a recipe in the main workspace and hands back its id
func (e *testEnv) recipe(t *testing.T, name string) uint {
	t.Helper()

	created, err := e.recipes.Create(workspaceID, &dto.CreateRecipeReq{Name: name})
	require.NoError(t, err)

	return created.ID
}

// archived creates a recipe already out of the default listing
func (e *testEnv) archived(t *testing.T, name string) uint {
	t.Helper()

	id := e.recipe(t, name)
	_, err := e.recipes.Archive(id, workspaceID)
	require.NoError(t, err)

	return id
}

func (e *testEnv) category(t *testing.T, workspace uint, name string) uint {
	t.Helper()

	created, err := e.categories.Create(workspace, &dto.CreateCategoryReq{Name: name})
	require.NoError(t, err)

	return created.ID
}

func TestCreatingARecipeWithItsContent(t *testing.T) {
	env := newTestEnv(t)

	created, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:             "  Pancakes  ",
		Description:      "  fluffy  ",
		EstimatedMinutes: 25,
		Ingredients: []dto.IngredientReq{
			{Name: " Flour ", Quantity: " 200g "},
			{Name: "Milk", Quantity: "300ml"},
		},
		Steps: []dto.StepReq{
			{Title: "Mix"},
			{Title: "Fry"},
		},
	})
	require.NoError(t, err)

	// The form is free to send padded text, the stored recipe is not
	assert.Equal(t, "Pancakes", created.Name)
	assert.Equal(t, "fluffy", created.Description)
	assert.Equal(t, 25, created.EstimatedMinutes)
	assert.False(t, created.Archived)

	require.Len(t, created.Ingredients, 2)
	assert.Equal(t, "Flour", created.Ingredients[0].Name)
	assert.Equal(t, "200g", created.Ingredients[0].Quantity)

	// Rank comes from the order they arrived in
	assert.Equal(t, 0, created.Ingredients[0].Position)
	assert.Equal(t, 1, created.Ingredients[1].Position)

	require.Len(t, created.Steps, 2)
	assert.Equal(t, "Mix", created.Steps[0].Title)
	assert.Equal(t, 0, created.Steps[0].Position)
	assert.Equal(t, 1, created.Steps[1].Position)

	assert.Equal(t, 2, created.IngredientCount)
	assert.Equal(t, 2, created.StepCount)
}

// The form keeps an empty row at the bottom, and it must not be saved
func TestBlankIngredientsAndStepsAreDropped(t *testing.T) {
	env := newTestEnv(t)

	created, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name: "Toast",
		Ingredients: []dto.IngredientReq{
			{Name: "Bread"},
			{Name: "   "},
			{Name: ""},
			{Name: "Butter"},
		},
		Steps: []dto.StepReq{
			{Title: "Toast it"},
			{Title: "  "},
		},
	})
	require.NoError(t, err)

	require.Len(t, created.Ingredients, 2)
	assert.Equal(t, "Bread", created.Ingredients[0].Name)
	assert.Equal(t, "Butter", created.Ingredients[1].Name)
	// Positions close the gap the dropped rows left
	assert.Equal(t, 1, created.Ingredients[1].Position)

	require.Len(t, created.Steps, 1)
}

func TestARecipeNeedsAName(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{Name: "   "})
	assert.ErrorIs(t, err, core.ErrBadRequest)

	id := env.recipe(t, "Pancakes")
	_, err = env.recipes.Update(id, workspaceID, &dto.UpdateRecipeReq{Name: " "})
	assert.ErrorIs(t, err, core.ErrBadRequest)
}

func TestRecipesAreScopedToTheirWorkspace(t *testing.T) {
	env := newTestEnv(t)

	id := env.recipe(t, "Pancakes")

	mine, err := env.recipes.List(workspaceID, ListQuery{})
	require.NoError(t, err)
	require.Len(t, mine.Recipes, 1)

	theirs, err := env.recipes.List(otherWSID, ListQuery{})
	require.NoError(t, err)
	assert.Empty(t, theirs.Recipes)

	_, err = env.recipes.Get(id, otherWSID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.recipes.Update(id, otherWSID, &dto.UpdateRecipeReq{Name: "Theirs"})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.recipes.SetFavorite(id, otherWSID, true)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.recipes.Archive(id, otherWSID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	_, err = env.recipes.ReorderSteps(id, otherWSID, &dto.ReorderStepsReq{StepIDs: []uint{1}})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.recipes.Delete(id, otherWSID), core.ErrNotAllowed)
}

// An update replaces the recipe whole: what is left out of the payload is gone
func TestUpdatingReplacesIngredientsAndSteps(t *testing.T) {
	env := newTestEnv(t)

	created, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:        "Pancakes",
		Ingredients: []dto.IngredientReq{{Name: "Flour"}, {Name: "Milk"}},
		Steps:       []dto.StepReq{{Title: "Mix"}, {Title: "Fry"}},
	})
	require.NoError(t, err)

	updated, err := env.recipes.Update(created.ID, workspaceID, &dto.UpdateRecipeReq{
		Name:        "Crepes",
		Ingredients: []dto.IngredientReq{{Name: "Flour"}},
		Steps:       []dto.StepReq{{Title: "Mix"}, {Title: "Rest"}, {Title: "Fry"}},
	})
	require.NoError(t, err)

	assert.Equal(t, "Crepes", updated.Name)
	require.Len(t, updated.Ingredients, 1)
	require.Len(t, updated.Steps, 3)
	assert.Equal(t, "Rest", updated.Steps[1].Title)
}

func TestFavoriteIsToggledOnAndOff(t *testing.T) {
	env := newTestEnv(t)

	id := env.recipe(t, "Pancakes")

	favorite, err := env.recipes.SetFavorite(id, workspaceID, true)
	require.NoError(t, err)
	assert.True(t, favorite.Favorite)

	plain, err := env.recipes.SetFavorite(id, workspaceID, false)
	require.NoError(t, err)
	assert.False(t, plain.Favorite)
}

func TestArchivingTakesARecipeOutOfTheDefaultListing(t *testing.T) {
	env := newTestEnv(t)

	env.recipe(t, "Pancakes")
	id := env.archived(t, "Old stew")

	archived, err := env.recipes.Get(id, workspaceID)
	require.NoError(t, err)
	assert.True(t, archived.Archived)
	assert.NotNil(t, archived.ArchivedAt)

	byDefault, err := env.recipes.List(workspaceID, ListQuery{})
	require.NoError(t, err)
	require.Len(t, byDefault.Recipes, 1)
	assert.Equal(t, "Pancakes", byDefault.Recipes[0].Name)

	only, err := env.recipes.List(workspaceID, ListQuery{Archived: "only"})
	require.NoError(t, err)
	require.Len(t, only.Recipes, 1)
	assert.Equal(t, "Old stew", only.Recipes[0].Name)

	all, err := env.recipes.List(workspaceID, ListQuery{Archived: "include"})
	require.NoError(t, err)
	assert.Len(t, all.Recipes, 2)
}

func TestARecipeIsArchivedAndUnarchivedOnlyOnce(t *testing.T) {
	env := newTestEnv(t)

	id := env.recipe(t, "Pancakes")

	// It is not archived yet, so there is nothing to bring back
	_, err := env.recipes.Unarchive(id, workspaceID)
	assert.ErrorIs(t, err, core.ErrBadRequest)

	_, err = env.recipes.Archive(id, workspaceID)
	require.NoError(t, err)

	_, err = env.recipes.Archive(id, workspaceID)
	assert.ErrorIs(t, err, core.ErrBadRequest)

	back, err := env.recipes.Unarchive(id, workspaceID)
	require.NoError(t, err)
	assert.False(t, back.Archived)
	assert.Nil(t, back.ArchivedAt)
}

// Deleting is permanent, so it takes two deliberate steps
func TestOnlyAnArchivedRecipeCanBeDeleted(t *testing.T) {
	env := newTestEnv(t)

	id := env.recipe(t, "Pancakes")

	assert.ErrorIs(t, env.recipes.Delete(id, workspaceID), core.ErrBadRequest)

	_, err := env.recipes.Archive(id, workspaceID)
	require.NoError(t, err)

	require.NoError(t, env.recipes.Delete(id, workspaceID))

	_, err = env.recipes.Get(id, workspaceID)
	assert.ErrorIs(t, err, core.ErrNotAllowed)
}

func TestReorderingStepsRewritesTheirRank(t *testing.T) {
	env := newTestEnv(t)

	created, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:  "Pancakes",
		Steps: []dto.StepReq{{Title: "Mix"}, {Title: "Rest"}, {Title: "Fry"}},
	})
	require.NoError(t, err)

	ids := []uint{created.Steps[2].ID, created.Steps[0].ID, created.Steps[1].ID}

	reordered, err := env.recipes.ReorderSteps(created.ID, workspaceID, &dto.ReorderStepsReq{StepIDs: ids})
	require.NoError(t, err)

	require.Len(t, reordered.Steps, 3)
	assert.Equal(t, "Fry", reordered.Steps[0].Title)
	assert.Equal(t, "Mix", reordered.Steps[1].Title)
	assert.Equal(t, "Rest", reordered.Steps[2].Title)
	assert.Equal(t, 0, reordered.Steps[0].Position)
	assert.Equal(t, 2, reordered.Steps[2].Position)
}

// A stale client that sends a partial list would silently drop a step's rank,
// so the whole list is required and duplicates are refused
func TestReorderingRefusesAnythingButTheCompleteList(t *testing.T) {
	env := newTestEnv(t)

	created, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:  "Pancakes",
		Steps: []dto.StepReq{{Title: "Mix"}, {Title: "Rest"}, {Title: "Fry"}},
	})
	require.NoError(t, err)

	first, second := created.Steps[0].ID, created.Steps[1].ID

	_, err = env.recipes.ReorderSteps(created.ID, workspaceID, &dto.ReorderStepsReq{
		StepIDs: []uint{first, second},
	})
	assert.ErrorIs(t, err, core.ErrBadRequest, "a partial list should be refused")

	_, err = env.recipes.ReorderSteps(created.ID, workspaceID, &dto.ReorderStepsReq{
		StepIDs: []uint{first, second, second},
	})
	assert.ErrorIs(t, err, core.ErrBadRequest, "a repeated id should be refused")

	// The order the recipe already had survived both refusals
	unchanged, err := env.recipes.Get(created.ID, workspaceID)
	require.NoError(t, err)
	assert.Equal(t, "Mix", unchanged.Steps[0].Title)
}

func TestCategoriesAreAttachedToARecipe(t *testing.T) {
	env := newTestEnv(t)

	dessert := env.category(t, workspaceID, "Dessert")
	quick := env.category(t, workspaceID, "Quick")

	created, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name: "Pancakes",
		// The same id twice is not an error, it is deduplicated
		CategoryIDs: []uint{dessert, quick, dessert},
	})
	require.NoError(t, err)
	assert.Len(t, created.Categories, 2)

	filtered, err := env.recipes.List(workspaceID, ListQuery{CategoryIDs: []uint{dessert}})
	require.NoError(t, err)
	require.Len(t, filtered.Recipes, 1)

	// An update with no categories detaches them all
	updated, err := env.recipes.Update(created.ID, workspaceID, &dto.UpdateRecipeReq{Name: "Pancakes"})
	require.NoError(t, err)
	assert.Empty(t, updated.Categories)
}

// A category of another workspace is refused rather than quietly skipped
func TestARecipeCannotBorrowAnotherWorkspacesCategory(t *testing.T) {
	env := newTestEnv(t)

	theirs := env.category(t, otherWSID, "Dessert")

	_, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:        "Pancakes",
		CategoryIDs: []uint{theirs},
	})
	assert.ErrorIs(t, err, core.ErrBadRequest)

	// And an id that exists nowhere is refused the same way
	_, err = env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:        "Pancakes",
		CategoryIDs: []uint{404},
	})
	assert.ErrorIs(t, err, core.ErrBadRequest)
}

func TestListingPagesAndClampsThePageSize(t *testing.T) {
	env := newTestEnv(t)

	for _, name := range []string{"One", "Two", "Three"} {
		env.recipe(t, name)
	}

	// A page size of zero means the client said nothing
	byDefault, err := env.recipes.List(workspaceID, ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 1, byDefault.Pagination.Page)
	assert.Equal(t, DefaultPerPage, byDefault.Pagination.PerPage)
	assert.Equal(t, int64(3), byDefault.Pagination.Total)
	assert.Equal(t, 1, byDefault.Pagination.TotalPages)

	// A single request may not pull the whole workspace
	capped, err := env.recipes.List(workspaceID, ListQuery{PerPage: 5000})
	require.NoError(t, err)
	assert.Equal(t, MaxPerPage, capped.Pagination.PerPage)

	// Pages below the first are the first
	negative, err := env.recipes.List(workspaceID, ListQuery{Page: -3})
	require.NoError(t, err)
	assert.Equal(t, 1, negative.Pagination.Page)

	paged, err := env.recipes.List(workspaceID, ListQuery{PerPage: 2})
	require.NoError(t, err)
	assert.Len(t, paged.Recipes, 2)
	assert.Equal(t, 2, paged.Pagination.TotalPages)

	second, err := env.recipes.List(workspaceID, ListQuery{PerPage: 2, Page: 2})
	require.NoError(t, err)
	assert.Len(t, second.Recipes, 1)

	// Past the end is empty rather than an error
	past, err := env.recipes.List(workspaceID, ListQuery{PerPage: 2, Page: 9})
	require.NoError(t, err)
	assert.Empty(t, past.Recipes)
	assert.Equal(t, int64(3), past.Pagination.Total)
}

func TestListingRejectsAnUnknownArchivedFilter(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.recipes.List(workspaceID, ListQuery{Archived: "sometimes"})
	assert.ErrorIs(t, err, core.ErrBadRequest)
}

func TestListingFindsRecipesByNameAndFavorite(t *testing.T) {
	env := newTestEnv(t)

	pancakes := env.recipe(t, "Pancakes")
	env.recipe(t, "Beef stew")

	found, err := env.recipes.List(workspaceID, ListQuery{Search: "cake"})
	require.NoError(t, err)
	require.Len(t, found.Recipes, 1, "the search should match any part of the name")
	assert.Equal(t, "Pancakes", found.Recipes[0].Name)

	_, err = env.recipes.SetFavorite(pancakes, workspaceID, true)
	require.NoError(t, err)

	favorites, err := env.recipes.List(workspaceID, ListQuery{FavoritesOnly: true})
	require.NoError(t, err)
	require.Len(t, favorites.Recipes, 1)
	assert.Equal(t, "Pancakes", favorites.Recipes[0].Name)
}

// The listing carries the counts so a card never has to load the content
func TestListingCarriesTheContentCounts(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:        "Pancakes",
		Ingredients: []dto.IngredientReq{{Name: "Flour"}, {Name: "Milk"}},
		Steps:       []dto.StepReq{{Title: "Mix"}},
	})
	require.NoError(t, err)

	listed, err := env.recipes.List(workspaceID, ListQuery{})
	require.NoError(t, err)
	require.Len(t, listed.Recipes, 1)
	assert.Equal(t, 2, listed.Recipes[0].IngredientCount)
	assert.Equal(t, 1, listed.Recipes[0].StepCount)
}

// Saving a recipe feeds the workspace index, which is what the ingredient field
// reads back for its suggestions
func TestIngredientsAreSuggestedBackToTheWorkspace(t *testing.T) {
	env := newTestEnv(t)

	_, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
		Name:        "Pancakes",
		Ingredients: []dto.IngredientReq{{Name: "Flour"}},
	})
	require.NoError(t, err)

	found, err := env.recipes.Suggest(workspaceID, "flo")
	require.NoError(t, err)
	assert.Contains(t, found.Terms, "Flour")

	// The index belongs to one workspace, another one never sees those terms
	_, err = env.recipes.Suggest(otherWSID, "flo")
	assert.Error(t, err)
}

func TestCategoriesAreScopedAndNamedOnlyOnce(t *testing.T) {
	env := newTestEnv(t)

	id := env.category(t, workspaceID, "Dessert")

	// The same name is free in another workspace, but taken in this one
	env.category(t, otherWSID, "Dessert")

	_, err := env.categories.Create(workspaceID, &dto.CreateCategoryReq{Name: "Dessert"})
	assert.Error(t, err)

	mine, err := env.categories.List(workspaceID)
	require.NoError(t, err)
	require.Len(t, mine.Categories, 1)
	assert.Equal(t, "Dessert", mine.Categories[0].Name)

	_, err = env.categories.Update(id, otherWSID, &dto.UpdateCategoryReq{Name: "Theirs"})
	assert.ErrorIs(t, err, core.ErrNotAllowed)

	assert.ErrorIs(t, env.categories.Delete(id, otherWSID), core.ErrNotAllowed)
}

// The count on a category is what the filter bar shows, so it only counts the
// recipes a click would actually bring back
func TestACategoryCountsOnlyItsActiveRecipes(t *testing.T) {
	env := newTestEnv(t)

	dessert := env.category(t, workspaceID, "Dessert")

	for _, name := range []string{"Pancakes", "Brownies"} {
		_, err := env.recipes.Create(workspaceID, &dto.CreateRecipeReq{
			Name:        name,
			CategoryIDs: []uint{dessert},
		})
		require.NoError(t, err)
	}

	listed, err := env.categories.List(workspaceID)
	require.NoError(t, err)
	require.Len(t, listed.Categories, 1)
	assert.Equal(t, 2, listed.Categories[0].RecipeCount)

	brownies, err := env.recipes.List(workspaceID, ListQuery{Search: "Brownies"})
	require.NoError(t, err)
	require.Len(t, brownies.Recipes, 1)

	_, err = env.recipes.Archive(brownies.Recipes[0].ID, workspaceID)
	require.NoError(t, err)

	listed, err = env.categories.List(workspaceID)
	require.NoError(t, err)
	assert.Equal(t, 1, listed.Categories[0].RecipeCount)
}
