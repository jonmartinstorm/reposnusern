package runner

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/dbwriter"
	"github.com/jonmartinstorm/reposnusern/internal/fetcher"
	"github.com/jonmartinstorm/reposnusern/internal/models"
)

type RunnerDeps interface {
	OpenDB(dsn string) (*sql.DB, error)
	GetRepoPage(cfg config.Config, page int) ([]models.RepoMeta, error)
	ImportRepo(ctx context.Context, db *sql.DB, entry models.RepoEntry, index int, snapshotDate time.Time) error
	Fetcher() fetcher.GraphQLFetcher
}

type AppDeps struct {
	GitHub fetcher.GitHubAPI
}

func (AppDeps) OpenDB(dsn string) (*sql.DB, error) {
	return sql.Open("postgres", dsn)
}

func (a AppDeps) GetRepoPage(cfg config.Config, page int) ([]models.RepoMeta, error) {
	return a.GitHub.GetRepoPage(cfg, page)
}

func (a AppDeps) Fetcher() fetcher.GraphQLFetcher {
	return a.GitHub
}

func (AppDeps) ImportRepo(ctx context.Context, db *sql.DB, entry models.RepoEntry, index int, snapshotDate time.Time) error {
	return dbwriter.ImportRepo(ctx, db, entry, index, snapshotDate)
}

var OpenSQL = sql.Open

func RunApp(ctx context.Context, cfg config.Config, deps RunnerDeps) error {
	return RunAppSafe(ctx, cfg, deps)
}

func RunAppSafe(ctx context.Context, cfg config.Config, deps RunnerDeps) error {
	start := time.Now()

	err := Run(ctx, cfg, deps)
	if err != nil {
		slog.Debug("Runner feilet", "error", err)
		return err
	}

	LogMemoryStats()
	slog.Info("Ferdig!", "varighet", time.Since(start).String())
	return nil
}

func LogMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	slog.Debug("Minnebruk",
		"alloc", ByteSize(m.Alloc),
		"totalAlloc", ByteSize(m.TotalAlloc),
		"sys", ByteSize(m.Sys),
		"numGC", m.NumGC)
}

func ByteSize(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func CheckDatabaseConnection(ctx context.Context, dsn string) error {
	db, err := OpenSQL("postgres", dsn)
	if err != nil {
		slog.Debug("Klarte ikke å åpne databaseforbindelse", "dsn", dsn, "error", err)

		return fmt.Errorf("DB open-feil: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		// Lukker eksplisitt på feil, og returnerer
		if cerr := db.Close(); cerr != nil {
			slog.Warn("Klarte ikke å lukke testDB", "error", cerr)
		}
		slog.Debug("Ping mot database feilet", "dsn", dsn, "error", err)

		return fmt.Errorf("DB ping-feil: %w", err)
	}

	// Normal defer for clean exit
	defer func() {
		if cerr := db.Close(); cerr != nil {
			slog.Warn("Klarte ikke å lukke testDB", "error", cerr)
		}
	}()

	slog.Info("DB-tilkobling OK")
	return nil
}

const MaxDebugRepos = 10

func Run(ctx context.Context, cfg config.Config, deps RunnerDeps) error {
	snapshotDate := time.Now().Truncate(24 * time.Hour)
	slog.Info("Starter snapshot", "dato", snapshotDate.Format("2006-01-02"))

	db, err := deps.OpenDB(cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("DB-feil: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Warn("Klarte ikke å lukke databaseforbindelsen", "error", err)
		}
	}()

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(10 * time.Minute)

	page := 1
	repoIndex := 0

	for {
		repos, err := deps.GetRepoPage(cfg, page)
		if err != nil {
			return fmt.Errorf("klarte ikke hente repo-side: %w", err)
		}
		if len(repos) == 0 {
			break
		}

		for _, repo := range repos {
			if cfg.SkipArchived && repo.Archived {
				if cfg.Debug {
					slog.Info("Skipper arkivert repo", "repo", repo.FullName)
				}
				continue
			}

			if cfg.Debug && repoIndex >= MaxDebugRepos {
				slog.Info("Debug-modus: stopper etter 10 repoer")
				return nil
			}

			slog.Info("Henter detaljer via GraphQL", "repo", repo.FullName)
			entry := deps.Fetcher().Fetch(cfg.Org, repo.Name, cfg.Token, repo)
			if entry == nil {
				slog.Warn("Hopper over tomt repo", "repo", repo.FullName)
				continue
			}

			repoIndex++
			slog.Info("Behandler repo", "nummer", repoIndex, "navn", repo.FullName)

			if err := deps.ImportRepo(ctx, db, *entry, repoIndex, snapshotDate); err != nil {
				return fmt.Errorf("import repo: %w", err)
			}

			if repoIndex%25 == 0 {
				runtime.GC()
			}
		}

		page++
	}

	slog.Info("Ferdig med alle repos!")
	return nil
}
