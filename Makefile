## Stagehand — build, test, lint, coverage, and install targets. (PRD §21.1)
##
## Usage:  make <target>
##
## Targets:
##   build      Compile the stagehand binary to ./bin/stagehand
##   install    Install stagehand into $GOPATH/bin
##   test       Run all tests with the race detector enabled
##   coverage   Run tests and print per-function coverage
##   lint       Run golangci-lint
##   clean      Remove bin/, coverage.out, and dist/
##   help       Print this help

.DEFAULT_GOAL := build

# --- Version (PRD §21.1, §21.4) -------------------------------------------------
# Injected into main.version via -ldflags at build time. Defaults to "dev";
# override for releases:  make build VERSION=v1.2.3   (goreleaser sets it via env).
# NOTE: -X main.version=... is a silent no-op until main.go declares `var version string`
# (a later subtask adds it). VERIFIED: build exits 0 either way.
VERSION ?= dev

# --- Paths & flags --------------------------------------------------------------
BIN_DIR  := bin
BIN      := $(BIN_DIR)/stagehand
MAIN_PKG := ./cmd/stagehand
LDFLAGS  := -X main.version=$(VERSION)

.PHONY: build install test coverage lint clean help

build: ## Compile the stagehand binary to ./bin/stagehand
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(MAIN_PKG)

install: ## Install stagehand into $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(MAIN_PKG)

test: ## Run all tests with the race detector enabled
	go test -race ./...

coverage: ## Run tests and print per-function coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint: ## Run golangci-lint
	golangci-lint run

clean: ## Remove bin/, coverage.out, and dist/
	rm -rf $(BIN_DIR) coverage.out dist/

help: ## Print this help
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'
