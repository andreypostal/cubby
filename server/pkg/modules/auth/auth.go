package auth

import (
	"fmt"
	"net/http"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/handler"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type Auth struct {
	wsHandler   *handler.WorkspaceHandler
	authHandler *handler.AuthHandler
	userHandler *handler.UserHandler

	db *core.DB
}

func (a *Auth) Id() string {
	return "auth-api"
}

func (a *Auth) New() any {
	return new(Auth)
}

func (a *Auth) Initialize(ctx *core.Context) error {
	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	a.db = db.(*core.DB)

	valid, err := ctx.Module(new(validator.Validate))
	if err != nil {
		return fmt.Errorf("load validator: %w", err)
	}

	hasher, err := ctx.Module(new(core.PasswordHasher))
	if err != nil {
		return fmt.Errorf("load hasher: %w", err)
	}

	jwt, err := ctx.Module(new(core.JwtProvider))
	if err != nil {
		return fmt.Errorf("load jwt provider: %w", err)
	}

	userRepo := repository.NewUserRepository(a.db)
	sessionRepo := repository.NewSessionRepository(a.db)

	wsRepo := repository.NewWorkspaceRepository(a.db)
	wsService := service.NewWorkspaceService(wsRepo, userRepo)

	authService := service.NewAuthService(
		userRepo,
		sessionRepo,
		hasher.(core.PasswordHasher),
		jwt.(core.JwtProvider),
	)

	userService := service.NewUserService(
		userRepo,
		wsRepo,
		sessionRepo,
		hasher.(core.PasswordHasher),
	)

	a.wsHandler = handler.NewWorkspaceHandler(wsService, valid.(*validator.Validate))
	a.authHandler = handler.NewAuthHandler(authService, wsService, valid.(*validator.Validate))
	a.userHandler = handler.NewUserHandler(userService, valid.(*validator.Validate))

	return nil
}

func (a *Auth) Routes() ([]api.Route, error) {
	var routes []api.Route

	// Public routes
	routes = append(routes, api.Route{
		Method:    http.MethodPost,
		Path:      "/auth/login",
		Handlers:  []gin.HandlerFunc{a.authHandler.Login},
		AuthScope: api.PublicRouteType,
	})

	// User authenticated routes
	routes = append(routes, api.Route{
		Method:    http.MethodDelete,
		Path:      "/auth/logout",
		Handlers:  []gin.HandlerFunc{a.authHandler.Logout},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/auth/me/workspace",
		Handlers:  []gin.HandlerFunc{a.authHandler.SwitchWorkspace},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/auth/me/workspace",
		Handlers:  []gin.HandlerFunc{a.authHandler.ResetWorkspace},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/auth/me",
		Handlers:  []gin.HandlerFunc{a.authHandler.Me},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/auth/workspaces",
		Handlers:  []gin.HandlerFunc{a.wsHandler.List},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/auth/workspaces",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Create},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/auth/workspaces/:workspaceID",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Get},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/auth/workspaces/:workspaceID",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Update},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/auth/workspaces/:workspaceID",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Delete},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodGet,
		Path:      "/auth/workspaces/:workspaceID/members",
		Handlers:  []gin.HandlerFunc{a.wsHandler.ListMembers},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/auth/workspaces/:workspaceID/members",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Share},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/auth/workspaces/:workspaceID/members/:userID",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Revoke},
		AuthScope: api.UserRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/auth/workspaces/:workspaceID/leave",
		Handlers:  []gin.HandlerFunc{a.wsHandler.Leave},
		AuthScope: api.UserRouteType,
	})

	// System administration of the user list
	routes = append(routes, api.Route{
		Method:    http.MethodGet,
		Path:      "/users",
		Handlers:  []gin.HandlerFunc{a.userHandler.List},
		AuthScope: api.SystemAdminRouteType,
	}, api.Route{
		Method:    http.MethodPost,
		Path:      "/users",
		Handlers:  []gin.HandlerFunc{a.userHandler.Create},
		AuthScope: api.SystemAdminRouteType,
	}, api.Route{
		Method:    http.MethodPatch,
		Path:      "/users/:userID",
		Handlers:  []gin.HandlerFunc{a.userHandler.Update},
		AuthScope: api.SystemAdminRouteType,
	}, api.Route{
		Method:    http.MethodPut,
		Path:      "/users/:userID/password",
		Handlers:  []gin.HandlerFunc{a.userHandler.ChangePassword},
		AuthScope: api.SystemAdminRouteType,
	}, api.Route{
		Method:    http.MethodDelete,
		Path:      "/users/:userID",
		Handlers:  []gin.HandlerFunc{a.userHandler.Delete},
		AuthScope: api.SystemAdminRouteType,
	})

	return routes, nil
}

func (a *Auth) Migrate() error {
	if err := a.db.AutoMigrate(
		&models.User{},
		&models.Session{},
		&models.Workspace{},
	); err != nil {
		return fmt.Errorf("failed to migrate auth: %w", err)
	}
	return nil
}
