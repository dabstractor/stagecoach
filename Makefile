# stagehand Makefile
#
# Canonical build/test/coverage/lint/vet/fmt/clean targets. The build target
# injects the version via -ldflags "-X main.version=$(VERSION)" where VERSION
# is derived from `git describe`. All recipes use TAB indentation.

VERSION := $(shell git describe --tags --always --dirty)

.PHONY: build test coverage lint vet fmt clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/stagehand ./cmd/stagehand

test:
	go test ./...

coverage:
	go test -coverprofile=coverage.out ./...

lint:
	golangci-lint run

vet:
	go vet ./...

fmt:
	gofmt -s -w .

clean:
	rm -rf bin coverage.out dist
