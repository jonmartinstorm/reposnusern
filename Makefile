ARTIFACT_NAME := reposnusern

build:
	@go build -o bin/${ARTIFACT_NAME}/${ARTIFACT_NAME} cmd/${ARTIFACT_NAME}/main.go 

run:
	@go run cmd/${ARTIFACT_NAME}/main.go 

ci: tidy vet lint test

# -------------------------------
# Test targets
# -------------------------------

unit:
	@go test -v $(shell go list ./... | grep -v /test/)

integration:
	@go test -v -tags=integration $(shell go list ./... | grep -v /test/)

e2e:
	@go test -v -tags=e2e ./tests/e2e/...

test: unit integration

# -------------------------------
# Coverage
# -------------------------------

COVER_OUT = cover.out
COVER_FILTERED = cover.filtered.out

EXCLUDE_FILES = \
    cmd/$(ARTIFACT_NAME)/main.go \
    internal/storage/ \
    internal/models/

EXCLUDE_GREP := $(foreach f,$(EXCLUDE_FILES),| grep -v $(f))

go-test-with-cover:
	@go test -coverprofile=$(COVER_OUT) -v $(shell go list ./... | grep -v /test/)
	@cat $(COVER_OUT) $(EXCLUDE_GREP) > $(COVER_FILTERED)
	@go tool cover -html=$(COVER_FILTERED) -o cover.html
	@open cover.html || xdg-open cover.html || echo "Åpne cover.html manuelt"

# -------------------------------
# Lint og hygiene
# -------------------------------

vet:
	@go vet ./...

tidy:
	@go mod tidy

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run


check-secrets:
	@! grep -r "ghp_" . || (echo "🚨 GitHub token funnet i kode!"; exit 1)

# -------------------------------
# Mock-generering
# -------------------------------

generate-mocks:
	@command -v mockery >/dev/null 2>&1 && mockery
