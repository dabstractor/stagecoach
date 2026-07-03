# Research notes — P1.M8.T2.S2: Makefile finalize + release target

## Task contract (verbatim, from tasks.json id=P1.M8.T2.S2)
1. RESEARCH NOTE: PRD §21.1; decisions.md §1.
2. INPUT: Makefile skeleton (P1.M1.T1.S1); goreleaser config (P1.M8.T2.S1).
3. LOGIC: Finalize Makefile — `build` injects version via git describe; add
   `release` target (goreleaser release --clean), `release-snapshot` (snapshot),
   `cross-build` sanity check; ensure `make build test lint` is the contributor loop.
   MOCKING: `make build` produces a versioned binary; `make release-snapshot` succeeds.
4. OUTPUT: dev + release build flow.
- DOCS: **none declared in this subtask's contract** → defers to Mode B
  (changeset-level README sync in P1.M8.T4.S1). The Makefile itself carries
  the contributor-loop + release-flow docstring inline (header comment).

## Verified host state (2026-07-03)
- `goreleaser` is on PATH; `goreleaser --version` → **GitVersion v2.9.0**.
  This is the pinned version required by S1 (`brews` Formula hard-deprecated
  >= v2.10). Any newer goreleaser on PATH will FAIL `goreleaser check`/
  release on the `brews:` block.
- `.goreleaser.yaml` already exists (created by S1); `goreleaser check` →
  "1 configuration file(s) validated", zero deprecation errors. ✓
- `make build` → `go build -ldflags "-X main.version=7674f34" -o bin/stagehand ...`; exit 0. ✓
- `./bin/stagehand --version` → `stagehand version 7674f34` (git describe =
  short SHA; repo has NO tags yet → `--always` falls back to commit SHA).
  Proves ldflags injection to `main.version` works. ✓
- `go test ./...` → all 8 packages ok, exit 0. ✓ (part of contributor loop)
- `golangci-lint` is **NOT installed** on this host. The `lint` target text
  exists but the binary is absent → `make lint` fails locally until a
  contributor installs it. The MOCKING gate does NOT include lint; gates
  are `make build` + `make release-snapshot` (+ cross-build sanity).
- `make clean` already does `rm -rf bin coverage.out dist` — dist/ is the
  goreleaser output dir and is gitignored. ✓
- Makefile uses **TAB-indented recipes** (must be preserved: spaces →
  "missing separator. Stop.").
- The current Makefile (skeleton from S1) defines ONLY: build, test,
  coverage, lint, vet, fmt, clean. MISSING (this task's gap): `release`,
  `release-snapshot`, `cross-build`, their `.PHONY` entries, and the
  contributor-loop / release-flow documentation.

## goreleaser subcommand matrix (all run against the real config)
| command | result | use |
|---|---|---|
| `goreleaser check` | valid, 0 deprecations | config validity |
| `goreleaser build --clean` | **FAILS**: "git doesn't contain any tags - either add a tag or use --snapshot" | ❌ wrong form for cross-build |
| `goreleaser build --snapshot --clean` | **SUCCESS** — builds all 6 binaries (linux/darwin/windows × amd64/arm64), "build succeeded", exit 0; ~instant (no archive/checksum/manifest step) | ✅ cross-build sanity target |
| `goreleaser release --snapshot --clean` | **SUCCESS** — dist/ already populated with 6 archives + checksums.txt + homebrew/Formula/stagehand.rb + scoop/stagehand.json + aur/stagehand-bin.pkgbuild (verified via ls dist/) | ✅ release-snapshot (MOCKING gate) |
| `goreleaser release --clean` | (not run) — requires a git tag + TAP_GITHUB_TOKEN/SCOOP_GITHUB_TOKEN/AUR_KEY; PUBLISHES. Correct as the maintainer `release` target. | ✅ release target |

## Key decisions
1. **cross-build = `goreleaser build --snapshot --clean`**, NOT `go build`
   per-target and NOT `goreleaser build --clean` (the latter fails without a
   tag). `goreleaser build` is the idiomatic "source of truth for builds"
   subcommand (per `goreleaser build --help`), it honors the .goreleaser.yaml
   cross-compile matrix + ldflags, and `--snapshot` makes it tag-free. It is
   the lightest green-signal for the 6-target matrix.
2. **`make build test lint` stays a 3-target invocation** (make runs multiple
   goals in order natively — no aggregate target needed). The "ensure" is
   satisfied by (a) keeping build/test/lint targets intact and (b) documenting
   the loop in the Makefile header. Adding a redundant `check`/`all` alias is
   NOT required by the contract and would diverge from the S1 skeleton.
3. **VERSION symbol parity**: Makefile `-X main.version=$(VERSION)` and
   goreleaser `-X main.version={{.Version}}` target the SAME Go symbol
   (`main.version` in cmd/stagehand/main.go, default `"dev"`). For a tagged
   release both inject the version; git describe keeps the `v` prefix
   (`v1.0.0`), goreleaser strips it (`1.0.0`) — harmless dev divergence, the
   binary reports whatever it was built with. No change needed.
4. **DOCS = Mode B deferral** (no per-item doc). The Makefile header comment
   is the inline documentation surface; README install/contributor docs are
   the changeset-level Mode B sync in P1.M8.T4.S1.

## The exact edit (single file: repo-root Makefile)
- UPDATE header comment to document the contributor loop + release targets.
- ADD `release release-snapshot cross-build` to the `.PHONY` line.
- APPEND three targets (TAB-indented):
  - `release:` → `goreleaser release --clean`
  - `release-snapshot:` → `goreleaser release --snapshot --clean`
  - `cross-build:` → `goreleaser build --snapshot --clean`
- KEEP build/test/coverage/lint/vet/fmt/clean + VERSION exactly as-is.

## Gate summary (verified executable, ONE command each)
- `make build` → produces bin/stagehand with git-describe version. (MOCKING gate #1)
- `make release-snapshot` → goreleaser snapshot succeeds, emits dist/. (MOCKING gate #2)
- `make cross-build` → all 6 targets compile. (sanity, optional primary)
- `make test` → go test ./... green. (contributor-loop leg)
