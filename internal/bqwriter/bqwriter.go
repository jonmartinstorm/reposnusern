package bqwriter

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/models"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type BigQueryWriter struct {
	Client  *bigquery.Client
	Dataset string
}

func NewBigQueryWriter(ctx context.Context, cfg *config.Config) (*BigQueryWriter, error) {
	var client *bigquery.Client
	var err error

	if cfg.BQCredentials != "" {
		client, err = bigquery.NewClient(ctx, cfg.BQProjectID, option.WithCredentialsFile(cfg.BQCredentials))
	} else {
		client, err = bigquery.NewClient(ctx, cfg.BQProjectID)
	}

	if err != nil {
		return nil, fmt.Errorf("kan ikke opprette BigQuery-klient: %w", err)
	}

	writer := &BigQueryWriter{
		Client:  client,
		Dataset: cfg.BQDataset,
	}

	// Sørg for at hver tabell finnes
	tables := map[string]any{
		"repos":         BGRepoEntry{},
		"languages":     BGRepoLanguage{},
		"files":         BGFile{},
		"ci_config":     BGCIConfig{},
		"sbom_packages": FlatBGSBOM{},
	}

	for tableName, schemaExample := range tables {
		if err := ensureTableExists(ctx, client, cfg.BQDataset, tableName, schemaExample); err != nil {
			return nil, fmt.Errorf("kunne ikke sikre tabell %s: %w", tableName, err)
		}
	}

	return writer, nil
}

func (w *BigQueryWriter) ImportRepo(ctx context.Context, entry models.RepoEntry, index int, snapshot time.Time) error {
	repo := ConvertToBG(entry, snapshot)
	langs := ConvertLanguages(entry, snapshot)
	files := ConvertFiles(entry, snapshot)
	ciconfig := ConvertCI(entry, snapshot)
	sbom := extractBGSBOMFlat(entry, snapshot)

	if err := insert(ctx, w.Client, w.Dataset, "repos", []BGRepoEntry{repo}); err != nil {
		return fmt.Errorf("repos insert failed: %w", err)
	}
	if err := insert(ctx, w.Client, w.Dataset, "languages", langs); err != nil {
		return fmt.Errorf("languages insert failed: %w", err)
	}
	if err := insert(ctx, w.Client, w.Dataset, "files", files); err != nil {
		return fmt.Errorf("files insert failed: %w", err)
	}
	if err := insert(ctx, w.Client, w.Dataset, "ci_config", ciconfig); err != nil {
		return fmt.Errorf("ci_config insert failed: %w", err)
	}
	if err := insert(ctx, w.Client, w.Dataset, "sbom_packages", sbom); err != nil {
		return fmt.Errorf("sbom insert failed: %w", err)
	}

	return nil
}

func insert[T any](ctx context.Context, client *bigquery.Client, dataset, table string, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	inserter := client.Dataset(dataset).Table(table).Inserter()
	return inserter.Put(ctx, rows)
}

// ==== Data-strukturer ====

type BGRepoEntry struct {
	RepoID       int64     `bigquery:"repo_id"`
	FullName     string    `bigquery:"full_name"`
	Topics       []string  `bigquery:"topics"`
	Stars        int64     `bigquery:"stars"`
	License      string    `bigquery:"license"`
	SnapshotDate time.Time `bigquery:"snapshot_date"`
	UpdatedAt    time.Time `bigquery:"updated_at"`
	HasSBOM      bool      `bigquery:"has_sbom"`
}

type BGRepoLanguage struct {
	RepoID       int64     `bigquery:"repo_id"`
	Language     string    `bigquery:"language"`
	Bytes        int64     `bigquery:"bytes"`
	SnapshotDate time.Time `bigquery:"snapshot_date"`
}

type BGFile struct {
	RepoID       int64     `bigquery:"repo_id"`
	FileType     string    `bigquery:"file_type"`
	Path         string    `bigquery:"path"`
	Content      string    `bigquery:"content"`
	SnapshotDate time.Time `bigquery:"snapshot_date"`
}

type BGCIConfig struct {
	RepoID       int64     `bigquery:"repo_id"`
	Path         string    `bigquery:"path"`
	Content      string    `bigquery:"content"`
	SnapshotDate time.Time `bigquery:"snapshot_date"`
}

type FlatBGSBOM struct {
	RepoID       int64     `bigquery:"repo_id"`
	Name         string    `bigquery:"name"`
	Version      string    `bigquery:"version"`
	License      string    `bigquery:"license"`
	PURL         string    `bigquery:"purl"`
	SnapshotDate time.Time `bigquery:"snapshot_date"`
}

// ==== Mapping-funksjoner ====

func ConvertToBG(entry models.RepoEntry, snapshot time.Time) BGRepoEntry {
	return BGRepoEntry{
		RepoID:       entry.Repo.ID,
		FullName:     entry.Repo.FullName,
		Topics:       entry.Repo.Topics,
		Stars:        entry.Repo.Stars,
		License:      safeLicense(entry.Repo.License),
		SnapshotDate: snapshot,
		UpdatedAt:    parseTime(entry.Repo.UpdatedAt),
		HasSBOM:      len(entry.SBOM) > 0,
	}
}

func ConvertLanguages(entry models.RepoEntry, snapshot time.Time) []BGRepoLanguage {
	var result []BGRepoLanguage
	for lang, size := range entry.Languages {
		result = append(result, BGRepoLanguage{
			RepoID:       entry.Repo.ID,
			Language:     lang,
			Bytes:        int64(size),
			SnapshotDate: snapshot,
		})
	}
	return result
}

func ConvertFiles(entry models.RepoEntry, snapshot time.Time) []BGFile {
	var result []BGFile
	for typ, list := range entry.Files {
		for _, f := range list {
			result = append(result, BGFile{
				RepoID:       entry.Repo.ID,
				FileType:     typ,
				Path:         f.Path,
				Content:      f.Content,
				SnapshotDate: snapshot,
			})
		}
	}
	return result
}

func ConvertCI(entry models.RepoEntry, snapshot time.Time) []BGCIConfig {
	var result []BGCIConfig
	for _, f := range entry.CIConfig {
		result = append(result, BGCIConfig{
			RepoID:       entry.Repo.ID,
			Path:         f.Path,
			Content:      f.Content,
			SnapshotDate: snapshot,
		})
	}
	return result
}

func extractBGSBOMFlat(entry models.RepoEntry, snapshot time.Time) []FlatBGSBOM {
	raw := entry.SBOM
	var result []FlatBGSBOM
	sbomInner, ok := raw["sbom"].(map[string]interface{})
	if !ok {
		return result
	}
	pkgs, ok := sbomInner["packages"].([]interface{})
	if !ok {
		return result
	}

	for _, p := range pkgs {
		pkg, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, FlatBGSBOM{
			RepoID:       entry.Repo.ID,
			Name:         safeString(pkg["name"]),
			Version:      safeString(pkg["versionInfo"]),
			License:      safeString(pkg["licenseConcluded"]),
			PURL:         extractPURL(pkg),
			SnapshotDate: snapshot,
		})
	}
	return result
}

// ==== Hjelpefunksjoner ====

func safeLicense(lic *models.License) string {
	if lic == nil {
		return ""
	}
	return lic.SpdxID
}

func safeString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func extractPURL(pkg map[string]interface{}) string {
	refs, ok := pkg["externalRefs"].([]interface{})
	if !ok {
		return ""
	}
	for _, ref := range refs {
		refMap, ok := ref.(map[string]interface{})
		if ok && refMap["referenceType"] == "purl" {
			return safeString(refMap["referenceLocator"])
		}
	}
	return ""
}

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func ensureTableExists(ctx context.Context, client *bigquery.Client, dataset, table string, exampleStruct any) error {
	tbl := client.Dataset(dataset).Table(table)
	_, err := tbl.Metadata(ctx)
	if err == nil {
		return nil // tabellen finnes
	}

	if gErr, ok := err.(*googleapi.Error); !ok || gErr.Code != 404 {
		return fmt.Errorf("feil ved henting av tabell-metadata: %w", err)
	}

	schema, err := bigquery.InferSchema(exampleStruct)
	if err != nil {
		return fmt.Errorf("klarte ikke å generere schema for %s: %w", table, err)
	}

	if err := tbl.Create(ctx, &bigquery.TableMetadata{Schema: schema}); err != nil {
		return fmt.Errorf("klarte ikke å opprette tabell %s: %w", table, err)
	}

	return nil
}
