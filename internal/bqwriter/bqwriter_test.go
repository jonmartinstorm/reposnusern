package bqwriter_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jonmartinstorm/reposnusern/internal/bqwriter"
	"github.com/jonmartinstorm/reposnusern/internal/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBQWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BigQueryWriter Suite")
}

var _ = Describe("BigQueryWriter simple import", func() {
	It("kjører ImportRepo uten crash med dummy client", func() {
		ctx := context.Background()
		snapshot := time.Now()

		// Lag tom BigQuery client (NB: bruker tom projectID og forventer at du ikke har credentials i test)
		client, _ := bigquery.NewClient(ctx, "dummy-project") // ikke bruk klienten egentlig

		writer := &bqwriter.BigQueryWriter{
			Client:  client,
			Dataset: "fake_dataset",
		}

		entry := models.RepoEntry{
			Repo: models.RepoMeta{
				ID:        1,
				FullName:  "testorg/repo",
				Stars:     42,
				Topics:    []string{"go"},
				License:   &models.License{SpdxID: "MIT"},
				UpdatedAt: "2025-06-15T10:00:00Z",
			},
			Languages: map[string]int{"Go": 1000},
			Files: map[string][]models.FileEntry{
				"dockerfile": {{Path: "Dockerfile", Content: "FROM alpine"}},
			},
			CIConfig: []models.FileEntry{
				{Path: ".github/workflows/test.yml", Content: "name: CI"},
			},
			SBOM: map[string]interface{}{
				"sbom": map[string]interface{}{
					"packages": []interface{}{
						map[string]interface{}{
							"name":             "pkg",
							"versionInfo":      "1.0",
							"licenseConcluded": "MIT",
							"externalRefs": []interface{}{
								map[string]interface{}{
									"referenceType":    "purl",
									"referenceLocator": "pkg:golang/pkg@1.0",
								},
							},
						},
					},
				},
			},
		}

		// Dette vil feile på ekte insert, men det vi tester er at mapping og kall ikke panikker
		err := writer.ImportRepo(ctx, entry, 0, snapshot)
		Expect(err).To(HaveOccurred()) // fordi insert prøver å kjøre og feiler
	})
})
