VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse HEAD)
TAG     := $(shell git describe --exact-match --tags HEAD 2>/dev/null)
DIRTY   := $(shell test -n "$$(git status --porcelain)" && echo true || echo false)
LDFLAGS := -X github.com/bawdo/mailpitter/internal/version.Version=$(VERSION) \
           -X github.com/bawdo/mailpitter/internal/version.Commit=$(COMMIT) \
           -X github.com/bawdo/mailpitter/internal/version.Tag=$(TAG) \
           -X github.com/bawdo/mailpitter/internal/version.Dirty=$(DIRTY)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/mailpitter .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

lint:
	golangci-lint run

integration:
	go test -tags integration -v ./test/integration/...

pre-ci:
	./scripts/pre-ci-check.sh

pre-ci-fix:
	./scripts/pre-ci-check.sh --fix gofmt

.PHONY: build install test integration lint pre-ci pre-ci-fix
