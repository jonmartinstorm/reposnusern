package bqwriter

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jonmartinstorm/reposnusern/internal/models"
)

type BigQueryWriter struct {
	client  *bigquery.Client
	dataset string
	table   string
}

func NewBigQueryWriter(ctx context.Context, projectID, dataset, table string) (*BigQueryWriter, error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("kan ikke opprette BigQuery-klient: %w", err)
	}
	return &BigQueryWriter{client: client, dataset: dataset, table: table}, nil
}

func (w *BigQueryWriter) ImportRepo(ctx context.Context, entry models.RepoEntry, index int, snapshot time.Time) error {
	bgEntry := ConvertToBG(entry, snapshot)

	inserter := w.client.Dataset(w.dataset).Table(w.table).Inserter()
	if err := inserter.Put(ctx, bgEntry); err != nil {
		return fmt.Errorf("kunne ikke skrive til BigQuery: %w", err)
	}
	return nil
}

type BGRepoEntry struct {
	RepoID       int64     `bigquery:"repo_id"`
	FullName     string    `bigquery:"full_name"`
	Topics       []string  `bigquery:"topics"`
	Stars        int64     `bigquery:"stars"`
	License      string    `bigquery:"license"`
	SnapshotDate time.Time `bigquery:"snapshot_date"`
	UpdatedAt    time.Time `bigquery:"updated_at"`
	HasSBOM      bool      `bigquery:"has_sbom"`
	SBOM         []BGSBOM  `bigquery:"sbom_packages"`
}

type BGSBOM struct {
	Name    string `bigquery:"name"`
	Version string `bigquery:"version"`
	License string `bigquery:"license"`
	PURL    string `bigquery:"purl"`
}

func ConvertToBG(entry models.RepoEntry, snapshot time.Time) BGRepoEntry {
	sbomPackages := extractBGSBOM(entry.SBOM)
	return BGRepoEntry{
		RepoID:       entry.Repo.ID,
		FullName:     entry.Repo.FullName,
		Topics:       entry.Repo.Topics,
		Stars:        entry.Repo.Stars,
		License:      safeLicense(entry.Repo.License),
		SnapshotDate: snapshot,
		UpdatedAt:    parseTime(entry.Repo.UpdatedAt),
		HasSBOM:      len(sbomPackages) > 0,
		SBOM:         sbomPackages,
	}
}

func extractBGSBOM(raw map[string]interface{}) []BGSBOM {
	var result []BGSBOM
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
		result = append(result, BGSBOM{
			Name:    safeString(pkg["name"]),
			Version: safeString(pkg["versionInfo"]),
			License: safeString(pkg["licenseConcluded"]),
			PURL:    extractPURL(pkg),
		})
	}
	return result
}

func extractPURL(pkg map[string]interface{}) string {
	refs, ok := pkg["externalRefs"].([]interface{})
	if !ok {
		return ""
	}
	for _, ref := range refs {
		refMap, ok := ref.(map[string]interface{})
		if !ok {
			continue
		}
		if refMap["referenceType"] == "purl" {
			return safeString(refMap["referenceLocator"])
		}
	}
	return ""
}

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

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, value)
	return t
}
