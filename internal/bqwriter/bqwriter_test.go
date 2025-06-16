package bqwriter_test

import (
	"context"
	"errors"
	"time"

	"github.com/jonmartinstorm/reposnusern/internal/bqwriter"
	"github.com/jonmartinstorm/reposnusern/internal/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

// Enkel mock for BigQuery-inserter
type MockInserter struct {
	mock.Mock
}

func (m *MockInserter) Put(ctx context.Context, src interface{}) error {
	args := m.Called(ctx, src)
	return args.Error(0)
}

var _ = Describe("BigQueryWriter", func() {
	var (
		ctx      context.Context
		snapshot time.Time
		mockIns  *MockInserter
		repo     models.RepoEntry
	)

	BeforeEach(func() {
		ctx = context.Background()
		snapshot = time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)
		mockIns = new(MockInserter)

		repo = models.RepoEntry{
			Repo: models.RepoMeta{
				ID:        42,
				FullName:  "testorg/demo",
				Stars:     100,
				Topics:    []string{"go", "cloud"},
				License:   &models.License{SpdxID: "MIT"},
				UpdatedAt: "2025-06-15T12:00:00Z",
			},
			SBOM: map[string]interface{}{
				"sbom": map[string]interface{}{
					"packages": []interface{}{
						map[string]interface{}{
							"name":             "loglib",
							"versionInfo":      "1.0.0",
							"licenseConcluded": "Apache-2.0",
							"externalRefs": []interface{}{
								map[string]interface{}{
									"referenceType":    "purl",
									"referenceLocator": "pkg:golang/loglib@1.0.0",
								},
							},
						},
					},
				},
			},
		}
	})

	It("skal konvertere RepoEntry til korrekt BigQuery-struktur", func() {
		bg := bqwriter.ConvertToBG(repo, snapshot)

		Expect(bg.RepoID).To(Equal(int64(42)))
		Expect(bg.FullName).To(Equal("testorg/demo"))
		Expect(bg.License).To(Equal("MIT"))
		Expect(bg.Topics).To(ContainElement("go"))
		Expect(bg.HasSBOM).To(BeTrue())
		Expect(bg.SBOM).To(HaveLen(1))
		Expect(bg.SBOM[0].PURL).To(Equal("pkg:golang/loglib@1.0.0"))
	})

	It("skal kalle Put på inserter uten feil", func() {
		bg := bqwriter.ConvertToBG(repo, snapshot)

		mockIns.On("Put", mock.Anything, bg).Return(nil)

		err := mockIns.Put(ctx, bg)
		Expect(err).NotTo(HaveOccurred())

		mockIns.AssertExpectations(GinkgoT())
	})

	It("skal returnere feil hvis Put feiler", func() {
		bg := bqwriter.ConvertToBG(repo, snapshot)

		mockIns.On("Put", mock.Anything, bg).Return(errors.New("kunne ikke lagre"))

		err := mockIns.Put(ctx, bg)
		Expect(err).To(MatchError(ContainSubstring("kunne ikke lagre")))

		mockIns.AssertExpectations(GinkgoT())
	})
})
