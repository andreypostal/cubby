package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"andreypostal.com/cubby/server/pkg/core"
	"github.com/urfave/cli/v3"
)

func startCommand(ctx *core.Context) *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "Run the API Server",
		Action: func(_ context.Context, c *cli.Command) error {
			stopApps, err := startApps(ctx)
			if err != nil {
				return err
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			sig := <-quit

			slog.Info("Shutting down", slog.String("signal", sig.String()))

			// Stop apps initiated
			if err := stopApps(); err != nil {
				slog.Error("Failed while stopping apps", slog.String("error", err.Error()))
			}

			if err := closeModules(ctx); err != nil {
				return err
			}

			slog.Info("Shutdown complete")

			return nil
		},
	}
}
