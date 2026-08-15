package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	"github.com/gin-gonic/gin"
)

type AuthMiddleware struct {
	authService service.AuthService
	wsRepo      repository.WorkspaceRepository
	jwtProvider core.JwtProvider
}

func (a *AuthMiddleware) Id() string {
	return "auth-middleware"
}

func (a *AuthMiddleware) New() any {
	return new(AuthMiddleware)
}

func (a *AuthMiddleware) Initialize(ctx *core.Context) error {
	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	hasher, err := ctx.Module(new(core.PasswordHasher))
	if err != nil {
		return fmt.Errorf("load hasher: %w", err)
	}

	jwt, err := ctx.Module(new(core.JwtProvider))
	if err != nil {
		return fmt.Errorf("load jwt provider: %w", err)
	}
	a.jwtProvider = jwt.(core.JwtProvider)

	userRepo := repository.NewUserRepository(db.(*core.DB))
	sessionRepo := repository.NewSessionRepository(db.(*core.DB))
	a.wsRepo = repository.NewWorkspaceRepository(db.(*core.DB))

	a.authService = service.NewAuthService(
		userRepo,
		sessionRepo,
		hasher.(core.PasswordHasher),
		a.jwtProvider,
	)

	return nil
}

func (a *AuthMiddleware) RequireAuth(withWorkspace, asSystemAdmin bool) func(c *gin.Context) {
	return func(c *gin.Context) {
		if _, ok := core.GetProfile(c); ok {
			c.Next()
			return
		}

		logger := GetLogger(c)

		tokenString, err := extractBearerToken(c)
		if err != nil {
			logger.Debug("authorization header not found")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, err := a.jwtProvider.VerifyToken(tokenString)
		if err != nil {
			logger.Error("authorization token is invalid", slog.String("error", err.Error()))
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		sessionId, sessionFound := claims["sub"]
		userId, userFound := claims["uid"]
		username, usernameFound := claims["username"]
		if !usernameFound || !sessionFound || !userFound {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Load session
		session, err := a.authService.GetSession(sessionId.(string))
		if err != nil {
			logger.Error("Failed to load session", slog.String("reason", err.Error()))
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if session.User.ID != uint(userId.(float64)) || session.User.Username != username.(string) {
			logger.Error(
				"authorization user doesn't match with session user",
				slog.String("expected-user", userId.(string)),
				slog.Any("got-user", userId),
				slog.String("expected-username", username.(string)),
				slog.String("got-email", session.User.Username),
			)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		profile := core.Profile{
			SessionCode: session.Code,
			User: core.ProfileUser{
				ID:            session.User.ID,
				Username:      session.User.Username,
				Name:          session.User.Name,
				IsSystemAdmin: session.User.IsSystemAdmin,
			},
		}

		if session.Workspace != nil {
			// If user is the creator, we don't need to revalidate access
			if session.Workspace.CreatorID != session.User.ID {
				allowed, err := a.wsRepo.HasAccess(session.Workspace.ID, session.User.ID)
				if err != nil {
					logger.Error("Failed to verify workspace access", slog.String("error", err.Error()))
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}

				if !allowed {
					c.AbortWithStatus(http.StatusForbidden)
					return
				}
			}

			profile.Workspace = &core.ProfileWorkspace{
				ID:        session.Workspace.ID,
				Name:      session.Workspace.Name,
				IsCreator: session.Workspace.CreatorID == session.User.ID,
			}
		}

		if withWorkspace && profile.Workspace == nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		if asSystemAdmin && !session.User.IsSystemAdmin {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		core.SetProfile(c, profile)

		c.Next()
	}
}

// extractBearerToken pulls the token string from "Authorization: Bearer <token>"
func extractBearerToken(c *gin.Context) (string, error) {
	header := c.GetHeader("Authorization")
	if header == "" {
		return "", errors.New("missing authorization header")
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("malformed authorization header")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty token")
	}

	return token, nil
}
