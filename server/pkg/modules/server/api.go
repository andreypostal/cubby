package api

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"github.com/gin-gonic/gin"
)

var (
	PublicRouteType      = "public"
	UserRouteType        = "user"
	WorkspaceRouteType   = "workspace"
	SystemAdminRouteType = "system-admin"
)

const (
	apiPrefix = "/api"
	apiV1Path = apiPrefix + "/v1"
)

type Api struct {
	env  string
	port int
	host string

	server *http.Server

	routes []Route

	// fallback handles requests matching no route
	fallback gin.HandlerFunc

	allowedOrigins []string
	allowedMethods []string
	allowedHeaders []string
	exposedHeaders []string
	maxAge         int

	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration

	authMiddlewares  *middleware.AuthMiddleware
	freezeMiddleware gin.HandlerFunc

	// tls hands over the certificate to serve with, when there is one
	tls TLSProvider
}

func (a *Api) Id() string {
	return "rest-api"
}

func (a *Api) New() any {
	return new(Api)
}

func (a *Api) Initialize(ctx *core.Context) error {
	a.port = ctx.Config.API.Port
	a.host = ctx.Config.API.Host
	a.env = ctx.Config.Env

	// Load CORS Config
	a.allowedHeaders = ctx.Config.API.CORS.AllowedHeaders
	a.allowedMethods = ctx.Config.API.CORS.AllowedMethods
	a.allowedOrigins = ctx.Config.API.CORS.AllowedOrigins
	a.exposedHeaders = ctx.Config.API.CORS.ExposedHeaders
	a.maxAge = ctx.Config.API.CORS.MaxAge

	a.readTimeout = ctx.Config.API.ReadTimeout
	a.writeTimeout = ctx.Config.API.WriteTimeout
	a.idleTimeout = ctx.Config.API.IdleTimeout

	authMiddlewares, err := ctx.Module(new(middleware.AuthMiddleware))
	if err != nil {
		return fmt.Errorf("get auth middleware: %w", err)
	}
	a.authMiddlewares = authMiddlewares.(*middleware.AuthMiddleware)

	freezer, err := core.GetModule[*core.Freezer](ctx, new(core.Freezer))
	if err != nil {
		return fmt.Errorf("get system freezer: %w", err)
	}
	a.freezeMiddleware = middleware.FreezeGate(freezer)

	tlsProvider, err := core.GetModule[TLSProvider](ctx, new(TLSProvider))
	if err != nil {
		return fmt.Errorf("get tls provider: %w", err)
	}
	a.tls = tlsProvider

	// Load routes from modules
	mods, err := ctx.Modules(new(Router))
	if err != nil {
		return fmt.Errorf("get routers: %w", err)
	}

	for _, router := range mods {
		routes, err := router.(Router).Routes()
		if err != nil {
			return fmt.Errorf("get routes: %w", err)
		}

		a.routes = append(a.routes, routes...)
	}

	fallback, err := core.GetModule[Fallback](ctx, new(Fallback))
	if err != nil {
		return fmt.Errorf("get fallback: %w", err)
	}

	if fallback != nil {
		a.fallback = fallback.Fallback()
	}

	return nil
}

func (a *Api) Start() error {
	if a.env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	engine.Use(middleware.RequestID())
	engine.Use(middleware.InjectLogger(slog.Default()))
	engine.Use(middleware.Logger())
	engine.Use(middleware.Recover())
	engine.Use(a.freezeMiddleware)
	engine.Use(middleware.CORS(core.CORSConfig{
		AllowedOrigins: a.allowedOrigins,
		AllowedMethods: a.allowedMethods,
		AllowedHeaders: a.allowedHeaders,
		ExposedHeaders: a.exposedHeaders,
		MaxAge:         a.maxAge,
	}))

	v1 := engine.Group(apiV1Path)

	public := v1.Group("")

	// Add user auth middleware
	user := v1.Group("")
	user.Use(a.authMiddlewares.RequireAuth(false, false))

	// Add workspace auth middleware
	workspace := v1.Group("")
	workspace.Use(a.authMiddlewares.RequireAuth(true, false))

	// System admin auth middleware
	systemAdmin := v1.Group("")
	systemAdmin.Use(a.authMiddlewares.RequireAuth(false, true))

	addRoute := func(to *gin.RouterGroup, r Route) {
		to.Handle(r.Method, r.Path, r.Handlers...)
	}

	for _, route := range a.routes {
		switch route.AuthScope {
		case PublicRouteType:
			addRoute(public, route)
		case UserRouteType:
			addRoute(user, route)
		case WorkspaceRouteType:
			addRoute(workspace, route)
		case SystemAdminRouteType:
			addRoute(systemAdmin, route)
		}
	}

	// Static UI and SPA history fallback. A root catch-all route cannot coexist
	// with the /api/v1 tree, so NoRoute is the only mount point. Its handlers run
	// the full global middleware chain, logger and recover included.
	engine.NoRoute(a.handlePossibleUI())

	addr := net.JoinHostPort(a.host, strconv.Itoa(a.port))
	a.server = &http.Server{
		Addr:         addr,
		Handler:      engine,
		ReadTimeout:  a.readTimeout,
		WriteTimeout: a.writeTimeout,
		IdleTimeout:  a.idleTimeout,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", addr, err)
	}

	cert := a.tls.Certificate()
	if cert != nil {
		ln = tls.NewListener(ln, tlsConfig(cert))
	}

	go func() {
		slog.Info("Starting server",
			slog.String("addr", addr),
			slog.Bool("tls", cert != nil),
		)

		if err := a.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Error during server listen", slog.String("error", err.Error()))
		}
	}()

	return nil
}

type TLSProvider interface {
	Certificate() *tls.Certificate
}

func tlsConfig(cert *tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{*cert},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		NextProtos: []string{"http/1.1"},
	}
}

// handlePossibleUI builds gin's NoRoute handler, keeps API responses stable
// redirect everything else to our fallback UI handler
func (a *Api) handlePossibleUI() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		isApi := p == apiPrefix || strings.HasPrefix(p, apiPrefix+"/")

		if a.fallback == nil || isApi {
			core.RespondError(c, core.NewError(http.StatusNotFound, "not found", nil))
			return
		}

		a.fallback(c)
	}
}

func (a *Api) Stop(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

// Router is a module interface that adds routes to the [Api].
type Router interface {
	Routes() ([]Route, error)
}

type Fallback interface {
	Fallback() gin.HandlerFunc
}

type Handler func(*gin.Context)

type Route struct {
	Method string
	// Path represents the Route relative path
	Path     string
	Handlers []gin.HandlerFunc

	// AuthScope represents the auth requirements for this route
	// public | user | workspace
	AuthScope string
}
