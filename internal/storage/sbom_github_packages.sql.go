// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: sbom_github_packages.sql

package storage

import (
	"context"
	"database/sql"
	"time"
)

const insertOrUpdateGithubSBOM = `-- name: InsertOrUpdateGithubSBOM :exec
INSERT INTO sbom_github_packages (
  repo_id, hentet_dato, name, version, license, purl
) VALUES (
  $1, $2, $3, $4, $5, $6
)
ON CONFLICT (repo_id, hentet_dato, name, version) DO UPDATE SET
  license = EXCLUDED.license,
  purl = EXCLUDED.purl
`

type InsertOrUpdateGithubSBOMParams struct {
	RepoID     int64
	HentetDato time.Time
	Name       string
	Version    sql.NullString
	License    sql.NullString
	Purl       sql.NullString
}

func (q *Queries) InsertOrUpdateGithubSBOM(ctx context.Context, arg InsertOrUpdateGithubSBOMParams) error {
	_, err := q.db.ExecContext(ctx, insertOrUpdateGithubSBOM,
		arg.RepoID,
		arg.HentetDato,
		arg.Name,
		arg.Version,
		arg.License,
		arg.Purl,
	)
	return err
}
