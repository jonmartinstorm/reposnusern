// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: repo_languages.sql

package storage

import (
	"context"
)

const insertLanguage = `-- name: InsertLanguage :exec
INSERT INTO repo_languages (
    repo_id, language, bytes
) VALUES (?, ?, ?)
`

type InsertLanguageParams struct {
	RepoID   int64
	Language string
	Bytes    int64
}

func (q *Queries) InsertLanguage(ctx context.Context, arg InsertLanguageParams) error {
	_, err := q.db.ExecContext(ctx, insertLanguage, arg.RepoID, arg.Language, arg.Bytes)
	return err
}
