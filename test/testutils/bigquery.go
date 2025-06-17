package testutils

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	bqtc "github.com/testcontainers/testcontainers-go/modules/gcloud/bigquery"
	"google.golang.org/api/option"
	"google.golang.org/api/option/internaloption"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BQTestEnv struct {
	Container *bqtc.Container
	Client    *bigquery.Client
	ProjectID string
}

func MustParseTime(str string) time.Time {
	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		panic(err)
	}
	return t
}

func StartTestBigQueryContainer(t *testing.T) *BQTestEnv {
	t.Helper()
	ctx := context.Background()

	// Start emulator container
	container, err := bqtc.Run(
		ctx,
		"ghcr.io/goccy/bigquery-emulator:0.6.1",
		bqtc.WithProjectID("test"),
	)
	require.NoError(t, err, "failed to start BigQuery emulator container")

	projectID := container.ProjectID()

	opts := []option.ClientOption{
		option.WithEndpoint(container.URI()),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithoutAuthentication(),
		internaloption.SkipDialSettingsValidation(),
	}

	client, err := bigquery.NewClient(ctx, projectID, opts...)
	require.NoError(t, err, "failed to create BigQuery client")

	return &BQTestEnv{
		Container: container,
		Client:    client,
		ProjectID: projectID,
	}
}

func (env *BQTestEnv) Cleanup(t *testing.T) {
	t.Helper()

	require.NoError(t, env.Client.Close(), "failed to close BigQuery client")
	require.NoError(t, testcontainers.TerminateContainer(env.Container), "failed to terminate BigQuery container")
}

func (env *BQTestEnv) CleanupWithoutT() {
	_ = env.Client.Close()
	_ = env.Container.Terminate(context.Background())
}
