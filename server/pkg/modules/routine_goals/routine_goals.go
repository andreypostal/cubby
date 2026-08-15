package routine_goals

import (
	"context"
	"fmt"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/handler"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/models"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/repository"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals/service"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type RoutineGoals struct {
	ctx    context.Context
	cancel context.CancelFunc

	routineHandler *handler.RoutineHandler

	db *core.DB
}

func (r *RoutineGoals) Id() string {
	return "routine-goals-api"
}

func (r *RoutineGoals) New() any {
	return new(RoutineGoals)
}

func (r *RoutineGoals) Initialize(ctx *core.Context) error {
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

	routineRepo := repository.NewRoutineRepository(r.db)
	dayRepo := repository.NewRoutineDayRepository(r.db)

	routineService := service.NewRoutineService(routineRepo, dayRepo)

	r.routineHandler = handler.NewRoutineHandler(routineService, valid.(*validator.Validate))

	return nil
}

func (r *RoutineGoals) Routes() ([]api.Route, error) {
	var routes []api.Route

	routes = append(routes, api.Route{
		Method:    http.MethodGet,
		Path:      "/routines",
		Handlers:  []gin.HandlerFunc{r.routineHandler.List},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/routines",
		Handlers:  []gin.HandlerFunc{r.routineHandler.CreateRoutine},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/routines/:routineID",
		Handlers:  []gin.HandlerFunc{r.routineHandler.GetRoutine},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/routines/:routineID",
		Handlers:  []gin.HandlerFunc{r.routineHandler.UpdateRoutine},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/routines/:routineID",
		Handlers:  []gin.HandlerFunc{r.routineHandler.DeleteRoutine},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/routines/:routineID/month",
		Handlers:  []gin.HandlerFunc{r.routineHandler.Month},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/routines/:routineID/days/:date",
		Handlers:  []gin.HandlerFunc{r.routineHandler.MarkDay},
		AuthScope: api.WorkspaceRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/routines/:routineID/days/:date",
		Handlers:  []gin.HandlerFunc{r.routineHandler.UnmarkDay},
		AuthScope: api.WorkspaceRouteType,
	})

	return routes, nil
}

func (r *RoutineGoals) Migrate() error {
	if err := r.db.AutoMigrate(
		&models.Routine{},
		&models.RoutineDay{},
	); err != nil {
		return fmt.Errorf("failed to migrate routine goals: %w", err)
	}
	return nil
}

func (r *RoutineGoals) Close() error {
	r.cancel()
	return nil
}
