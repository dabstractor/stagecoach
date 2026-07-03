# stagehand Makefile
#
# Canonical build/test/coverage/lint/vet/fmt/clean targets plus the release
# flow (PRD §21.1, §21.2; G9). The build target injects the version via
# -ldflags "-X main.version=$(VERSION)" where VERSION is derived from
# `git describe` — the same `main.version` symbol goreleaser injects via
# {{.Version}} (see .goreleaser.yaml).
#
#   Contributor loop:    make build test lint
#   Cross-compile check: make cross-build
#   Local release test:  make release-snapshot   (tag-free; no publish)
#   Real release:        make release            (needs a git tag + publishing tokens; maintainer only)
#
# All recipes use TAB indentation.

VERSION := $(shell git describe --tags --always --dirty)

.PHONY: build test coverage lint vet fmt clean release release-snapshot cross-build

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

# Full release: cross-compiles, archives, and publishes to GitHub Releases +
# Homebrew/Scoop/AUR. Requires a git tag and the *_GITHUB_TOKEN / AUR_KEY env
# vars (see .goreleaser.yaml). Run by a maintainer on a tagged commit.
release:
	goreleaser release --clean

# Local release dry-run: builds every archive + checksum + manifest WITHOUT
# publishing or needing a git tag. This is the MOCKING gate for the release flow.
release-snapshot:
	goreleaser release --snapshot --clean

# Cross-compile sanity check: builds all 6 OS/arch binaries (no archiving or
# publishing). The lightest green-signal that the cross-compile matrix compiles.
cross-build:
	goreleaser build --snapshot --clean
