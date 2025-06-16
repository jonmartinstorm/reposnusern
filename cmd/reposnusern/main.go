package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/fetcher"
	"github.com/jonmartinstorm/reposnusern/internal/logger"
	"github.com/jonmartinstorm/reposnusern/internal/runner"
	_ "github.com/lib/pq"
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

	// TODO her, lag en NewApp greie, og løs det med det. Tenk også DI her.
	deps := runner.AppDeps{
		GitHub: &fetcher.GitHubAPIClient{},
	}

	if err := runner.RunApp(ctx, cfg, deps); err != nil {
		slog.Error("Applikasjonen feilet", "error", err)
		os.Exit(1)
	}

}
