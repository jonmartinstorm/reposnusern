package test

import (
	"context"
	"testing"

	"cloud.google.com/go/bigquery"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("BigQuery emulator integrasjon", Ordered, func() {
	var (
		ctx       context.Context
		container *tcbq.Container
		client    *bigquery.Client
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
	})

	AfterAll(func() {
		if client != nil {
			_ = client.Close()
		}
		if container != nil {
			_ = testcontainers.TerminateContainer(container)
		}
	})

	It("oppretter tabell og henter ut rader", func() {
		datasetID := "test_dataset"
		tableID := "people"

		// Opprett dataset
		err := client.Dataset(datasetID).Create(ctx, &bigquery.DatasetMetadata{
			Location: "US",
		})
		Expect(err).To(BeNil())

		// Opprett tabell
		schema := bigquery.Schema{
			{Name: "id", Type: bigquery.IntegerFieldType},
			{Name: "name", Type: bigquery.StringFieldType},
		}
		table := client.Dataset(datasetID).Table(tableID)
		err = table.Create(ctx, &bigquery.TableMetadata{Schema: schema})
		Expect(err).To(BeNil())

		// Sett inn én rad
		inserter := table.Inserter()
		err = inserter.Put(ctx, []*bigquery.ValuesSaver{
			{
				Schema: schema,
				Row:    []bigquery.Value{1, "Alice"},
			},
		})
		Expect(err).To(BeNil())

		// Spørr og sjekk resultat
		query := client.Query("SELECT name FROM `test.test_dataset.people` WHERE id = 1")
		it, err := query.Read(ctx)
		Expect(err).To(BeNil())

		var row []bigquery.Value
		Expect(it.Next(&row)).To(BeTrue())
		Expect(row[0]).To(Equal("Alice"))
	})
})
