package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/fetcher"
	"github.com/jonmartinstorm/reposnusern/internal/logger"
	"github.com/jonmartinstorm/reposnusern/internal/runner"
)

func main() {
	ctx := context.Background()

	logger.SetupLogger()

	cfg, err := config.NewConfig()
	if err != nil {
		slog.Error("Ugyldig konfigurasjon:", "error", err)
		os.Exit(1)
	}

	logger.SetDebug(cfg.Debug)

	if !cfg.SkipArchived {
		slog.Info("Inkluderer arkiverte repositories")
	}

	app := runner.NewApp(cfg, &runner.AppDeps{
		GitHub: &fetcher.GitHubAPIClient{},
	})

	if err := app.RunApp(ctx); err != nil {
		slog.Error("Applikasjonen feilet", "error", err)
		os.Exit(1)
	}

}
