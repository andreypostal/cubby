package shop

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"andreypostal.com/cubby/server/pkg/modules/shop/handler"
	"andreypostal.com/cubby/server/pkg/modules/shop/models"
	"andreypostal.com/cubby/server/pkg/modules/shop/repository"
	"andreypostal.com/cubby/server/pkg/modules/shop/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const itemsIndexer = "items"

type Shop struct {
	ctx    context.Context
	cancel context.CancelFunc

	listHandler *handler.ListHandler

	itemIndexer *core.Indexer

	db *core.DB
}

func (s *Shop) Id() string {
	return "shop-api"
}

func (s *Shop) New() any {
	return new(Shop)
}

func (s *Shop) Initialize(ctx *core.Context) error {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	s.db = db.(*core.DB)

	valid, err := ctx.Module(new(validator.Validate))
	if err != nil {
		return fmt.Errorf("load validator: %w", err)
	}

	s.itemIndexer = core.NewIndexer(s.ctx, core.IndexerConfig{
		Identifier: itemsIndexer,
		BasePath:   ctx.Config.DataDir,

		WriterPoolSize: ctx.Config.Indexer.WriterPoolSize,
		ReaderPoolSize: ctx.Config.Indexer.ReaderPoolSize,

		WriterIdleTime: ctx.Config.Indexer.WriterIdleTime,
		ReaderIdleTime: ctx.Config.Indexer.ReaderIdleTime,

		MaxResults: ctx.Config.Indexer.SuggestionLimit,
	})

	listRepo := repository.NewListRepository(s.db)
	itemRepo := repository.NewListItemRepository(s.db)

	listService := service.NewListService(listRepo, itemRepo)
	itemService := service.NewListItemService(itemRepo, s.itemIndexer, ctx.Config.Indexer.SuggestTimeout)

	s.listHandler = handler.NewListHandler(listService, itemService, valid.(*validator.Validate))

	return nil
}

func (s *Shop) Routes() ([]api.Route, error) {
	var routes []api.Route

	routes = append(routes, api.Route{
		Method:    http.MethodGet,
		Path:      "/shop/lists",
		Handlers:  []gin.HandlerFunc{s.listHandler.List},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/shop/lists",
		Handlers:  []gin.HandlerFunc{s.listHandler.CreateList},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/shop/lists/:listID",
		Handlers:  []gin.HandlerFunc{s.listHandler.GetList},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/shop/lists/:listID",
		Handlers:  []gin.HandlerFunc{s.listHandler.UpdateList},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/shop/lists/:listID",
		Handlers:  []gin.HandlerFunc{s.listHandler.DeleteList},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/shop/lists/:listID/items",
		Handlers:  []gin.HandlerFunc{s.listHandler.CreateItem},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/shop/lists/:listID/items/suggestion",
		Handlers:  []gin.HandlerFunc{s.listHandler.SuggestItemName},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/shop/lists/:listID/items/:itemID",
		Handlers:  []gin.HandlerFunc{s.listHandler.GetItem},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/shop/lists/:listID/items/:itemID",
		Handlers:  []gin.HandlerFunc{s.listHandler.UpdateItem},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/shop/lists/:listID/items/:itemID",
		Handlers:  []gin.HandlerFunc{s.listHandler.DeleteItem},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/shop/lists/:listID/items/:itemID/toggle",
		Handlers:  []gin.HandlerFunc{s.listHandler.ToggleItem},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/shop/lists/:listID/items/clear-bought",
		Handlers:  []gin.HandlerFunc{s.listHandler.ClearBoughtItems},
		AuthScope: api.WorkspaceRouteType,
	})

	return routes, nil
}

func (s *Shop) Pause() error {
	s.itemIndexer.Pause(s.ctx)
	return nil
}

func (s *Shop) Resume() error {
	s.itemIndexer.Resume(s.ctx)
	return nil
}

func (s *Shop) Migrate() error {
	if err := s.db.AutoMigrate(
		&models.List{},
		&models.ListItem{},
	); err != nil {
		return fmt.Errorf("failed to migrate shop: %w", err)
	}
	return nil
}

func (s *Shop) Close() error {
	closeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.cancel()
	s.itemIndexer.Close(closeCtx)

	return nil
}
