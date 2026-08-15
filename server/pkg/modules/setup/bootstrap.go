package setup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const setupTokenBytes = 24

// Bootstrap is the first-run phase of a deployment. Uses the setup token
// to create the first system user as admin.
type Bootstrap struct {
	userRepo repository.UserRepository
	authSvc  service.AuthService
	validate *validator.Validate

	// mu guards the token against two callers completing the setup at once
	mu sync.Mutex
	// token is minted at Start and cleared once the first admin exists. It is
	// never written to disk, so restarting the server issues a new one.
	token string
}

func (b *Bootstrap) Id() string {
	return "setup-api"
}

func (b *Bootstrap) New() any {
	return new(Bootstrap)
}

func (b *Bootstrap) Initialize(ctx *core.Context) error {
	db, err := core.GetModule[*core.DB](ctx, new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	hasher, err := core.GetModule[core.PasswordHasher](ctx, new(core.PasswordHasher))
	if err != nil {
		return fmt.Errorf("load hasher: %w", err)
	}

	jwt, err := core.GetModule[core.JwtProvider](ctx, new(core.JwtProvider))
	if err != nil {
		return fmt.Errorf("load jwt provider: %w", err)
	}

	valid, err := core.GetModule[*validator.Validate](ctx, new(validator.Validate))
	if err != nil {
		return fmt.Errorf("load validator: %w", err)
	}
	b.validate = valid

	b.userRepo = repository.NewUserRepository(db)
	b.authSvc = service.NewAuthService(
		b.userRepo,
		repository.NewSessionRepository(db),
		hasher,
		jwt,
	)

	return nil
}

// Start mints and announces the setup token in logs
func (b *Bootstrap) Start() error {
	required, err := b.setupRequired()
	if err != nil {
		return fmt.Errorf("checking whether the setup is required: %w", err)
	}

	if !required {
		return nil
	}

	buf := make([]byte, setupTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("generating the setup token: %w", err)
	}

	b.mu.Lock()
	b.token = hex.EncodeToString(buf)
	token := b.token
	b.mu.Unlock()

	slog.Warn("Setup required: this server has no users yet",
		slog.String("setup_token", token),
		slog.String("hint", "open the web UI and enter this token to create the first administrator"),
	)

	return nil
}

func (b *Bootstrap) Stop(context.Context) error {
	return nil
}

func (b *Bootstrap) Routes() ([]api.Route, error) {
	return []api.Route{
		{
			Method:    http.MethodGet,
			Path:      "/setup/status",
			Handlers:  []gin.HandlerFunc{b.handleStatus},
			AuthScope: api.PublicRouteType,
		},
		{
			Method:    http.MethodPost,
			Path:      "/setup",
			Handlers:  []gin.HandlerFunc{b.handleComplete},
			AuthScope: api.PublicRouteType,
		},
	}, nil
}

// setupRequired checks if any user exists in the database
func (b *Bootstrap) setupRequired() (bool, error) {
	users, err := b.userRepo.Count()
	if err != nil {
		return false, err
	}

	return users == 0, nil
}
