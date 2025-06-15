package runner

import (
	"context"
	"database/sql"
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
