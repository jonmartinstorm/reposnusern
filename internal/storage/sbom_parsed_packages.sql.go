// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: sbom_parsed_packages.sql

package storage

import (
	"context"
	"database/sql"
)

const insertParsedSBOM = `-- name: InsertParsedSBOM :exec
INSERT INTO sbom_parsed_packages (
  repo_id, name, pkg_group, version, type, path
) VALUES ($1, $2, $3, $4, $5, $6)
`

type InsertParsedSBOMParams struct {
	RepoID   int64
	Name     string
	PkgGroup sql.NullString
	Version  sql.NullString
	Type     string
	Path     string
}

func (q *Queries) InsertParsedSBOM(ctx context.Context, arg InsertParsedSBOMParams) error {
	_, err := q.db.ExecContext(ctx, insertParsedSBOM,
		arg.RepoID,
		arg.Name,
		arg.PkgGroup,
		arg.Version,
		arg.Type,
		arg.Path,
	)
	return err
}
