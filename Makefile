# Makefile for local testing (mimics GH Actions tests.yml)

.PHONY: all static-checks build unit-test e2e-test

all: static-checks build unit-test e2e-test

static-checks:
	go mod tidy
	go mod verify
	go vet ./...

build:
	go build ./...

# List all non-e2e test packages dynamically
UNIT_TEST_PKGS := $(shell go list ./... | grep -v ./tfexec/internal/e2etest)

unit-test:
	go test -cover -race $(UNIT_TEST_PKGS)

e2e-test:
	TFEXEC_E2ETEST_VERSIONS=1.5.7 go test -race -timeout=30m -v ./tfexec/internal/e2etest
