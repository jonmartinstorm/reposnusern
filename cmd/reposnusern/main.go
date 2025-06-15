package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/fetcher"
	"github.com/jonmartinstorm/reposnusern/internal/runner"
	_ "github.com/lib/pq"
)

func main() {
	// Context for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	go func() {
		<-ctx.Done()
		slog.Info("SIGTERM mottatt – rydder opp...")
		// Her kan vi legge til ekstra rydding om vi trenger det
		// TODO sende context til dbcall og skriving av filer.
	}()

	cfg := config.LoadAndValidateConfig()
	runner.SetupLogger(cfg.Debug)

	if err := runner.CheckDatabaseConnection(ctx, cfg.PostgresDSN); err != nil {
		slog.Error("Klarte ikke å nå databasen", "error", err)
		os.Exit(1)
	}

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
