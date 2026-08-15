package tasks

import (
	"context"
	"fmt"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"andreypostal.com/cubby/server/pkg/modules/tasks/handler"
	"andreypostal.com/cubby/server/pkg/modules/tasks/models"
	"andreypostal.com/cubby/server/pkg/modules/tasks/repository"
	"andreypostal.com/cubby/server/pkg/modules/tasks/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type Tasks struct {
	ctx    context.Context
	cancel context.CancelFunc

	taskHandler *handler.TaskHandler

	db *core.DB
}

func (t *Tasks) Id() string {
	return "tasks-api"
}

func (t *Tasks) New() any {
	return new(Tasks)
}

func (t *Tasks) Initialize(ctx *core.Context) error {
	t.ctx, t.cancel = context.WithCancel(context.Background())

	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	t.db = db.(*core.DB)

	valid, err := ctx.Module(new(validator.Validate))
	if err != nil {
		return fmt.Errorf("load validator: %w", err)
	}

	boardRepo := repository.NewBoardRepository(t.db)
	taskRepo := repository.NewTaskRepository(t.db)
	entryRepo := repository.NewTaskEntryRepository(t.db)

	taskService := service.NewTaskService(boardRepo, taskRepo, entryRepo)

	t.taskHandler = handler.NewTaskHandler(taskService, valid.(*validator.Validate))

	return nil
}

func (t *Tasks) Routes() ([]api.Route, error) {
	var routes []api.Route

	routes = append(routes, api.Route{
		Method:    http.MethodGet,
		Path:      "/tasks/board",
		Handlers:  []gin.HandlerFunc{t.taskHandler.GetBoard},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/tasks/board",
		Handlers:  []gin.HandlerFunc{t.taskHandler.UpdateBoard},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/tasks/day",
		Handlers:  []gin.HandlerFunc{t.taskHandler.Day},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/tasks/range",
		Handlers:  []gin.HandlerFunc{t.taskHandler.Range},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/tasks",
		Handlers:  []gin.HandlerFunc{t.taskHandler.List},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/tasks",
		Handlers:  []gin.HandlerFunc{t.taskHandler.CreateTask},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/tasks/:taskID",
		Handlers:  []gin.HandlerFunc{t.taskHandler.GetTask},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/tasks/:taskID",
		Handlers:  []gin.HandlerFunc{t.taskHandler.UpdateTask},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/tasks/:taskID",
		Handlers:  []gin.HandlerFunc{t.taskHandler.DeleteTask},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/tasks/:taskID/stop",
		Handlers:  []gin.HandlerFunc{t.taskHandler.StopTask},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/tasks/:taskID/resume",
		Handlers:  []gin.HandlerFunc{t.taskHandler.ResumeTask},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/tasks/:taskID/days/:date",
		Handlers:  []gin.HandlerFunc{t.taskHandler.MarkDay},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/tasks/:taskID/days/:date",
		Handlers:  []gin.HandlerFunc{t.taskHandler.ClearDay},
		AuthScope: api.WorkspaceRouteType,
	})

	return routes, nil
}

func (t *Tasks) Migrate() error {
	if err := t.db.AutoMigrate(
		&models.TaskBoard{},
		&models.Task{},
		&models.TaskEntry{},
	); err != nil {
		return fmt.Errorf("failed to migrate tasks: %w", err)
	}
	return nil
}

func (t *Tasks) Close() error {
	t.cancel()
	return nil
}
