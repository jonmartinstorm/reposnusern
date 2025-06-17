package bigquery_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/jonmartinstorm/reposnusern/internal/bqwriter"
	"github.com/jonmartinstorm/reposnusern/internal/models"
	"github.com/jonmartinstorm/reposnusern/test/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	tcbq "github.com/testcontainers/testcontainers-go/modules/gcloud/bigquery"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestBigQueryWriter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BigQueryWriter-integrasjon")
}

var _ = Describe("BigQueryWriter", Ordered, func() {
	var (
		ctx       context.Context
		container *tcbq.Container
		client    *bigquery.Client
		writer    *bqwriter.BigQueryWriter
	)

	const (
		datasetID = "test_dataset"
		tableID   = "repos"
	)

	BeforeAll(func() {
		ctx = context.Background()

		var err error
		container, err = tcbq.Run(
			ctx,
			"ghcr.io/goccy/bigquery-emulator:0.6.1",
			tcbq.WithProjectID("test"),
		)
		Expect(err).To(BeNil())

		opts := []option.ClientOption{
			option.WithEndpoint(container.URI()),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
			option.WithoutAuthentication(),
			internaloption.SkipDialSettingsValidation(),
		}
		client, err = bigquery.NewClient(ctx, container.ProjectID(), opts...)
		Expect(err).To(BeNil())

		// Lag dataset og tabellen
		err = client.Dataset(datasetID).Create(ctx, &bigquery.DatasetMetadata{Location: "US"})
		Expect(err).To(BeNil())

		schema := bigquery.Schema{
			{Name: "repo_id", Type: bigquery.IntegerFieldType},
			{Name: "full_name", Type: bigquery.StringFieldType},
			{Name: "topics", Type: bigquery.StringFieldType, Repeated: true},
			{Name: "stars", Type: bigquery.IntegerFieldType},
			{Name: "license", Type: bigquery.StringFieldType},
			{Name: "snapshot_date", Type: bigquery.TimestampFieldType},
			{Name: "updated_at", Type: bigquery.TimestampFieldType},
			{Name: "has_sbom", Type: bigquery.BooleanFieldType},
			{Name: "sbom_packages", Type: bigquery.RecordFieldType, Repeated: true, Schema: bigquery.Schema{
				{Name: "name", Type: bigquery.StringFieldType},
				{Name: "version", Type: bigquery.StringFieldType},
				{Name: "license", Type: bigquery.StringFieldType},
				{Name: "purl", Type: bigquery.StringFieldType},
			}},
		}
		err = client.Dataset(datasetID).Table(tableID).Create(ctx, &bigquery.TableMetadata{Schema: schema})
		Expect(err).To(BeNil())

		writer = &bqwriter.BigQueryWriter{
			Client:  client,
			Dataset: datasetID,
		}
	})

	AfterAll(func() {
		if client != nil {
			_ = client.Close()
		}
		if container != nil {
			_ = testcontainers.TerminateContainer(container)
		}
	})

	AfterEach(func() {
		// Sikrer at det ble skrevet minst én rad
		q := client.Query("SELECT COUNT(*) FROM `" + datasetID + "." + tableID + "`")
		it, err := q.Read(ctx)
		Expect(err).To(BeNil())

		var row []bigquery.Value
		hasRow := it.Next(&row)
		Expect(hasRow).To(BeTrue(), "Forventer minst én rad skrevet til BigQuery")
		Expect(row[0]).To(BeNumerically(">", 0), "Ingen rader skrevet til BigQuery")
	})

	It("skriver én repo til BigQuery", func() {
		entry := models.RepoEntry{
			Repo: models.RepoMeta{
				ID:        123,
				Name:      "demo",
				FullName:  "testorg/demo",
				Language:  "Go",
				License:   &models.License{SpdxID: "MIT"},
				Topics:    []string{"oss"},
				Stars:     42,
				UpdatedAt: testutils.MustParseTime("2025-06-17T10:00:00Z").Format(time.RFC3339),
			},
			Languages: map[string]int{"Go": 1234},
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
									"referenceLocator": "pkg:purl",
								},
							},
						},
					},
				},
			},
		}

		err := writer.ImportRepo(ctx, entry, testutils.MustParseTime("2025-06-17T10:05:00Z"))
		Expect(err).To(BeNil())

		q := client.Query("SELECT COUNT(*) FROM `" + datasetID + "." + tableID + "` WHERE full_name = 'testorg/demo'")
		it, err := q.Read(ctx)
		Expect(err).To(BeNil())

		var row []bigquery.Value
		Expect(it.Next(&row)).To(BeTrue())
		Expect(row[0]).To(Equal(int64(1)))
	})
})
