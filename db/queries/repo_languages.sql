-- name: InsertRepoLanguage :exec
INSERT INTO repo_languages (
  repo_id, language, bytes
) VALUES ($1, $2, $3);