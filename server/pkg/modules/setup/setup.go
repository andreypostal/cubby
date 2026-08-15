package setup

import (
	"context"
	"fmt"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth/models"
	"andreypostal.com/cubby/server/pkg/modules/auth/repository"
	"andreypostal.com/cubby/server/pkg/modules/auth/service"
	"github.com/urfave/cli/v3"
)

type Setup struct {
	db *core.DB

	authService service.AuthService
}

func (r *Setup) Id() string {
	return "setup-cmd"
}

func (r *Setup) New() any {
	return new(Setup)
}

func (s *Setup) Initialize(ctx *core.Context) error {
	db, err := ctx.Module(new(core.DB))
	if err != nil {
		return fmt.Errorf("load db: %w", err)
	}

	s.db = db.(*core.DB)

	hasher, err := ctx.Module(new(core.PasswordHasher))
	if err != nil {
		return fmt.Errorf("load hasher: %w", err)
	}

	jwt, err := ctx.Module(new(core.JwtProvider))
	if err != nil {
		return fmt.Errorf("load jwt provider: %w", err)
	}

	userRepo := repository.NewUserRepository(s.db)
	sessionRepo := repository.NewSessionRepository(s.db)

	s.authService = service.NewAuthService(
		userRepo,
		sessionRepo,
		hasher.(core.PasswordHasher),
		jwt.(core.JwtProvider),
	)

	return nil
}

func (s *Setup) Command() *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "Server setup utils",
		Commands: []*cli.Command{
			s.newUserCommand(),
			s.promoteUserCommand(),
			s.demoteUserCommand(),
		},
	}
}

func (s *Setup) newUserCommand() *cli.Command {
	return &cli.Command{
		Name:  "new-user",
		Usage: "Create a new user in the system",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "username",
				Aliases:  []string{"u"},
				Usage:    "The login username",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "password",
				Aliases:  []string{"p"},
				Usage:    "The raw password used for login",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "The friendly name for this user",
				Required: true,
			},
			&cli.BoolFlag{
				Name:     "admin",
				Aliases:  []string{"a"},
				Usage:    "Defines if a user is created as a system admin access level",
				Required: false,
				Value:    false,
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			username := c.String("username")
			password := c.String("password")
			name := c.String("name")
			isSystemAdmin := c.Bool("admin")

			if err := s.authService.CreateUser(name, username, password, isSystemAdmin); err != nil {
				return err
			}

			fmt.Printf("[SETUP] user %q created\n", username)

			return nil
		},
	}
}

func (s *Setup) changeUserSystemAdminFlag(isAdmin bool) func(context.Context, *cli.Command) error {
	return func(_ context.Context, c *cli.Command) error {
		username := c.String("username")

		if err := s.authService.ChangeUserSystemAdminAccess(username, isAdmin); err != nil {
			return err
		}

		action := "demoted"
		if isAdmin {
			action = "promoted"
		}

		fmt.Printf("[SETUP] user %q %s\n", username, action)

		return nil
	}
}

func (s *Setup) promoteUserCommand() *cli.Command {
	return &cli.Command{
		Name:  "promote-user",
		Usage: "Promote a user to system admin access level",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "username",
				Aliases:  []string{"u"},
				Usage:    "The username for the user that will be promoted",
				Required: true,
			},
		},
		Action: s.changeUserSystemAdminFlag(true),
	}
}

func (s *Setup) demoteUserCommand() *cli.Command {
	return &cli.Command{
		Name:  "demote-user",
		Usage: "Demote a user from system admin access level",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "username",
				Aliases:  []string{"u"},
				Usage:    "The username for the user that will be demoted",
				Required: true,
			},
		},
		Action: s.changeUserSystemAdminFlag(false),
	}
}

func (s *Setup) Migrate() error {
	if err := s.db.AutoMigrate(
		&models.User{},
	); err != nil {
		return fmt.Errorf("failed to migrate auth: %w", err)
	}
	return nil
}
