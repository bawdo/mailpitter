VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse HEAD)
TAG     := $(shell git describe --exact-match --tags HEAD 2>/dev/null)
DIRTY   := $(shell test -n "$$(git status --porcelain)" && echo true || echo false)
LDFLAGS := -X github.com/bawdo/mailpitter/internal/version.Version=$(VERSION) \
           -X github.com/bawdo/mailpitter/internal/version.Commit=$(COMMIT) \
           -X github.com/bawdo/mailpitter/internal/version.Tag=$(TAG) \
           -X github.com/bawdo/mailpitter/internal/version.Dirty=$(DIRTY)

.DEFAULT_GOAL := help

help: ## List available targets and their descriptions
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Build the mailpitter binary into bin/
	go build -ldflags "$(LDFLAGS)" -o bin/mailpitter .

install: ## Install mailpitter into the Go bin directory
	go install -ldflags "$(LDFLAGS)" .

test: ## Run the unit tests
	go test ./...

lint: ## Run golangci-lint
	golangci-lint run

integration: ## Run the integration tests
	go test -tags integration -v ./test/integration/...

pre-ci: ## Run the pre-CI checks
	./scripts/pre-ci-check.sh

pre-ci-fix: ## Run the pre-CI checks and auto-fix gofmt
	./scripts/pre-ci-check.sh --fix gofmt

.PHONY: help build install test integration lint pre-ci pre-ci-fix
