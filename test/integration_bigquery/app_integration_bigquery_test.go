package bigquery_test

import (
	"context"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/jonmartinstorm/reposnusern/internal/bqwriter"
	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/models"
	"github.com/jonmartinstorm/reposnusern/internal/runner"
	"github.com/jonmartinstorm/reposnusern/test/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/testcontainers/testcontainers-go"
	tcbq "github.com/testcontainers/testcontainers-go/modules/gcloud/bigquery"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAppBigQueryIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "App-integrasjon BigQuery")
}

var _ = Describe("runner.App BigQuery", Ordered, func() {
	var (
		ctx       context.Context
		container *tcbq.Container
		client    *bigquery.Client
		app       *runner.App
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

		// Lag dataset og tabellen som appen forventer
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
	})

	AfterAll(func() {
		if client != nil {
			_ = client.Close()
		}
		if container != nil {
			_ = testcontainers.TerminateContainer(container)
		}
	})

	It("kjører hele appen og skriver til BigQuery", func() {
		cfg := config.Config{
			Org:         "testorg",
			Token:       "123",
			Debug:       true,
			Parallelism: 2,
			BQProjectID: container.ProjectID(),
			BQDataset:   datasetID,
			BQTable:     tableID,
		}

		writer := &bqwriter.BigQueryWriter{
			Client:  client,
			Dataset: datasetID,
		}

		mockRepos := []models.RepoMeta{
			{ID: 1, Name: "demo", FullName: "testorg/demo"},
			{ID: 2, Name: "lib", FullName: "testorg/lib"},
		}

		fetcher := &testutils.MockFetcher{}
		fetcher.On("GetReposPage", mock.Anything, cfg, 1).Return(mockRepos, nil)
		fetcher.On("GetReposPage", mock.Anything, cfg, 2).Return([]models.RepoMeta{}, nil)

		for i, repo := range mockRepos {
			entry := &models.RepoEntry{
				Repo: models.RepoMeta{
					ID:       repo.ID,
					Name:     repo.Name,
					FullName: repo.FullName,
					Language: "Go",
					License:  &models.License{SpdxID: "MIT"},
					Topics:   []string{"oss"},
				},
				Languages: map[string]int{"Go": 1000 + i},
			}
			fetcher.On("FetchRepoGraphQL", mock.Anything, repo).Return(entry, nil)
		}

		app = runner.NewApp(cfg, writer, fetcher)

		err := app.Run(ctx)
		Expect(err).To(BeNil())

		q := client.Query("SELECT COUNT(*) FROM `" + cfg.BQDataset + "." + cfg.BQTable + "` WHERE full_name LIKE 'testorg/%'")
		it, err := q.Read(ctx)
		Expect(err).To(BeNil())

		var row []bigquery.Value
		Expect(it.Next(&row)).To(BeTrue())
		Expect(row[0]).To(Equal(int64(2)))
	})
})
