package runner_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/mocks"
	"github.com/jonmartinstorm/reposnusern/internal/runner"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRunApp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RunApp Suite")
}

var _ = Describe("RunAppSafe", func() {
	var (
		ctx   context.Context
		cfg   config.Config
		deps  *mocks.MockRunnerDeps
		db    *sql.DB
		smock sqlmock.Sqlmock
	)

	BeforeEach(func() {
		var err error
		ctx = context.Background()
		cfg = config.Config{
			Org:         "test",
			Token:       "123",
			PostgresDSN: "mockdb",
		}
		db, smock, err = sqlmock.New()
		Expect(err).To(BeNil())

		deps = mocks.NewMockRunnerDeps(GinkgoT())
	})

	AfterEach(func() {
		if db != nil {
			err := smock.ExpectationsWereMet()
			Expect(err).To(BeNil())
		}
	})

	It("returnerer feil når GetRepoPage feiler", func() {
		deps.EXPECT().
			OpenDB(cfg.PostgresDSN).
			Return(db, nil)

		deps.EXPECT().
			GetRepoPage(cfg, 1).
			Return(nil, errors.New("API fail"))

		err := runner.RunAppSafe(ctx, cfg, deps)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("API fail"))
	})

	It("returnerer feil når OpenDB feiler", func() {
		deps.EXPECT().
			OpenDB(cfg.PostgresDSN).
			Return(nil, errors.New("DB nede"))

		err := runner.RunAppSafe(ctx, cfg, deps)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("DB nede"))
	})
})
