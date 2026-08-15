package recipes

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/recipes/handler"
	"andreypostal.com/cubby/server/pkg/modules/recipes/models"
	"andreypostal.com/cubby/server/pkg/modules/recipes/repository"
	"andreypostal.com/cubby/server/pkg/modules/recipes/service"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const ingredientsIndexer = "ingredients"

type Recipes struct {
	ctx    context.Context
	cancel context.CancelFunc

	recipeHandler      *handler.RecipeHandler
	categoryHandler    *handler.CategoryHandler
	ingredientsIndexer *core.Indexer

	db *core.DB
}

func (r *Recipes) Id() string {
	return "recipes-api"
}

func (r *Recipes) New() any {
	return new(Recipes)
}

func (r *Recipes) Initialize(ctx *core.Context) error {
	r.ctx, r.cancel = context.WithCancel(context.Background())

	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	r.db = db.(*core.DB)

	valid, err := ctx.Module(new(validator.Validate))
	if err != nil {
		return fmt.Errorf("load validator: %w", err)
	}

	categoryRepo := repository.NewCategoryRepository(r.db)
	recipeRepo := repository.NewRecipeRepository(r.db)

	r.ingredientsIndexer = core.NewIndexer(r.ctx, core.IndexerConfig{
		Identifier: ingredientsIndexer,
		BasePath:   ctx.Config.DataDir,

		WriterPoolSize: ctx.Config.Indexer.WriterPoolSize,
		ReaderPoolSize: ctx.Config.Indexer.ReaderPoolSize,

		WriterIdleTime: ctx.Config.Indexer.WriterIdleTime,
		ReaderIdleTime: ctx.Config.Indexer.ReaderIdleTime,

		MaxResults: ctx.Config.Indexer.SuggestionLimit,
	})

	categoryService := service.NewCategoryService(categoryRepo)
	recipeService := service.NewRecipeService(
		recipeRepo,
		categoryRepo,
		r.ingredientsIndexer,
		ctx.Config.Indexer.SuggestTimeout,
	)

	r.categoryHandler = handler.NewCategoryHandler(categoryService, valid.(*validator.Validate))
	r.recipeHandler = handler.NewRecipeHandler(
		recipeService,
		valid.(*validator.Validate),
	)

	return nil
}

func (r *Recipes) Routes() ([]api.Route, error) {
	var routes []api.Route

	routes = append(routes, api.Route{
		Method:    http.MethodGet,
		Path:      "/recipes/categories",
		Handlers:  []gin.HandlerFunc{r.categoryHandler.List},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/recipes/categories",
		Handlers:  []gin.HandlerFunc{r.categoryHandler.Create},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/recipes/categories/:categoryID",
		Handlers:  []gin.HandlerFunc{r.categoryHandler.Update},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/recipes/categories/:categoryID",
		Handlers:  []gin.HandlerFunc{r.categoryHandler.Delete},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/recipes",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.List},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/recipes",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.Create},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/recipes/:recipeID",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.Get},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/recipes/:recipeID",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.Update},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/recipes/:recipeID",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.Delete},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/recipes/:recipeID/favorite",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.SetFavorite},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/recipes/:recipeID/archive",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.Archive},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/recipes/:recipeID/unarchive",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.Unarchive},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/recipes/:recipeID/steps/reorder",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.ReorderSteps},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/recipes/ingredients/suggestion",
		Handlers:  []gin.HandlerFunc{r.recipeHandler.SuggestIngredientName},
		AuthScope: api.WorkspaceRouteType,
	})

	return routes, nil
}

func (r *Recipes) Pause() error {
	r.ingredientsIndexer.Pause(r.ctx)
	return nil
}

func (r *Recipes) Resume() error {
	r.ingredientsIndexer.Resume(r.ctx)
	return nil
}

func (r *Recipes) Migrate() error {
	if err := r.db.AutoMigrate(
		&models.Category{},
		&models.Recipe{},
		&models.Ingredient{},
		&models.Step{},
	); err != nil {
		return fmt.Errorf("failed to migrate recipes: %w", err)
	}
	return nil
}

func (r *Recipes) Close() error {
	closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r.cancel()
	r.ingredientsIndexer.Close(closeCtx)

	return nil
}
