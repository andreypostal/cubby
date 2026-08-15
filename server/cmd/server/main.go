package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"andreypostal.com/cubby/server/pkg/core"
	"andreypostal.com/cubby/server/pkg/modules/auth"
	"andreypostal.com/cubby/server/pkg/modules/backup"
	"andreypostal.com/cubby/server/pkg/modules/cron"
	"andreypostal.com/cubby/server/pkg/modules/recipes"
	"andreypostal.com/cubby/server/pkg/modules/routine_goals"
	api "andreypostal.com/cubby/server/pkg/modules/server"
	"andreypostal.com/cubby/server/pkg/modules/server/middleware"
	"andreypostal.com/cubby/server/pkg/modules/settings"
	"andreypostal.com/cubby/server/pkg/modules/setup"
	"andreypostal.com/cubby/server/pkg/modules/shop"
	"andreypostal.com/cubby/server/pkg/modules/tasks"
	"andreypostal.com/cubby/server/pkg/modules/web"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/urfave/cli/v3"
)

// Overwritten at link time by the release build, which passes them through
// -ldflags -X. A `go build` without those flags keeps the placeholders, which is
// how a development binary identifies itself.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("cubby %s (commit %s, built %s)", version, commit, date)
}

func main() {
	configFile, wantVersion := parseGlobalArgs(os.Args[1:])

	// Answered before the config is read, because reporting the version is the
	// one thing that has to work on a bare extracted archive or inside a
	// container that has no config file yet. urfave/cli only gets to run once
	// the modules are registered and migrated, which is far too late for it.
	if wantVersion {
		fmt.Println(versionString())
		return
	}

	config, err := getConfig(configFile)
	if err != nil {
		slog.Error("Failed to load the config file",
			slog.String("file", configFile),
			slog.String("error", err.Error()),
		)
		os.Exit(1)
	}

	if err := core.SetupLogging(config); err != nil {
		slog.Error("Failed to set up logging", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx := registerModules(config)

	if err := migrateModules(ctx); err != nil {
		slog.Error("Failed migrating modules", slog.String("error", err.Error()))
		os.Exit(1)
	}

	commanders, err := ctx.Modules(new(core.Commander))
	if err != nil {
		slog.Error("Failed while getting commander modules", slog.String("error", err.Error()))
		os.Exit(1)
	}

	commands := []*cli.Command{startCommand(ctx)}
	for _, c := range commanders {
		commands = append(commands, c.(core.Commander).Command())
	}

	cmd := &cli.Command{
		Name:     "Cubby CLI",
		Usage:    "Administer a Cubby server",
		Version:  versionString(),
		Commands: commands,
		Flags: []cli.Flag{
			// Declared so it shows up in the help output. The value is read by
			// parseGlobalArgs, since the config has to be loaded before this
			// command tree can be built. Local turns off the persistence a v3
			// flag has by default, so that a config flag placed after the
			// command name is rejected instead of being quietly ignored, which
			// matches how far parseGlobalArgs reads.
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "path to config file",
				Value:   defaultConfigFile,
				Local:   true,
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

const defaultConfigFile = "config.yaml"

// parseGlobalArgs reads the two things that have to be known before urfave/cli
// gets to parse anything: the config file, because the command tree is built out
// of modules that need it, and whether the version was asked for, because that
// has to answer on a bare extracted archive with no config file yet.
//
// The flag set stops at the command name, which is why the cli config flag is
// declared Local: a global flag further along the line is urfave/cli's to
// reject. Errors and usage output are discarded so that -h, --help and every
// unknown flag reach it untouched.
func parseGlobalArgs(args []string) (configFile string, wantVersion bool) {
	fs := flag.NewFlagSet("cubby", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&configFile, "config", defaultConfigFile, "")
	fs.StringVar(&configFile, "c", defaultConfigFile, "")
	fs.BoolVar(&wantVersion, "version", false, "")
	fs.BoolVar(&wantVersion, "v", false, "")

	_ = fs.Parse(args)

	// `cubby version` is what the README tells a fresh install to run
	return configFile, wantVersion || fs.Arg(0) == "version"
}

func registerModules(config core.Config) *core.Context {
	registry := core.NewRegistry()

	registry.Register(new(core.DB))
	registry.Register(new(core.JWT))
	registry.Register(new(core.BCryptHasher))
	registry.Register(new(core.ValidatorProvider))
	registry.Register(new(core.Freezer))

	registry.Register(new(backup.SimpleBackup))
	registry.Register(new(cron.CleanupSessions))

	registry.Register(new(middleware.AuthMiddleware))

	registry.Register(new(api.Api))
	registry.Register(new(settings.Settings))
	registry.Register(new(web.Web))
	registry.Register(new(auth.Auth))
	registry.Register(new(shop.Shop))
	registry.Register(new(routine_goals.RoutineGoals))
	registry.Register(new(tasks.Tasks))
	registry.Register(new(recipes.Recipes))

	registry.Register(new(setup.Setup))
	registry.Register(new(setup.Bootstrap))

	return core.NewContext(registry, config)
}

func closeModules(ctx *core.Context) error {
	closers, err := ctx.Modules(new(core.Closer))
	if err != nil {
		return err
	}

	for _, c := range closers {
		id := c.(core.Module).Id()

		if err := c.(core.Closer).Close(); err != nil {
			slog.Error("Module failed to close",
				slog.String("module", id),
				slog.String("error", err.Error()),
			)
			continue
		}

		slog.Info("Module closed", slog.String("module", id))
	}

	return nil
}

func startApps(ctx *core.Context) (func() error, error) {
	apps, err := ctx.Modules(new(core.App))
	if err != nil {
		return nil, err
	}

	for _, a := range apps {
		go func(app core.App) {
			id := app.(core.Module).Id()
			err = app.Start()
			if err != nil {
				slog.Error("Failed to start application, exiting",
					slog.String("module", id),
					slog.String("error", err.Error()),
				)
				os.Exit(1)
			}

			slog.Info("Application started", slog.String("module", id))
		}(a.(core.App))
	}

	return func() error {
		for _, a := range apps {
			id := a.(core.Module).Id()

			if err := a.(core.App).Stop(context.TODO()); err != nil {
				slog.Error("Application failed to stop",
					slog.String("module", id),
					slog.String("error", err.Error()),
				)
				continue
			}

			slog.Info("Application stopped", slog.String("module", id))
		}
		return nil
	}, nil
}

func migrateModules(ctx *core.Context) error {
	migrators, err := ctx.Modules(new(core.Migrator))
	if err != nil {
		return err
	}

	for _, m := range migrators {
		if err := m.(core.Migrator).Migrate(); err != nil {
			return fmt.Errorf("migrating %s: %w", m.(core.Module).Id(), err)
		}
	}

	slog.Info("Modules migrated", slog.Int("count", len(migrators)))

	return nil
}

func getConfig(configFile string) (core.Config, error) {
	var config core.Config

	_, err := os.Stat(configFile)
	switch {
	case err == nil:
		if err := cleanenv.ReadConfig(configFile, &config); err != nil {
			return core.Config{}, fmt.Errorf("loading config file %q: %w", configFile, err)
		}
	default:
		// A defined config file must exist
		return core.Config{}, fmt.Errorf("reading config file %q: %w", configFile, err)
	}

	if err := cleanenv.ReadEnv(&config); err != nil {
		return core.Config{}, fmt.Errorf("loading config from environment: %w", err)
	}

	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return core.Config{}, fmt.Errorf("ensuring data folder %q: %w", configFile, err)
	}

	if err := os.MkdirAll(config.BackupDir, 0o755); err != nil {
		return core.Config{}, fmt.Errorf("ensuring data folder %q: %w", configFile, err)
	}

	return config, nil
}
