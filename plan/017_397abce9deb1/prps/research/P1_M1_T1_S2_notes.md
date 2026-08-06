# Research notes — P1.M1.T1.S2 (Comparable semver of the running binary)

## 1. Codebase facts (verified by reading the files)

### cmd/stagecoach/main.go (the source of the version value)
- `var version = "dev"` — ldflags-injected via `-X main.version=...` (Makefile + goreleaser).
- `resolveVersion(v string) string` — produces a DISPLAY string only:
  - release value → returned as-is.
  - "dev" → enriched with VCS info to `"dev (<shortsha>[-dirty])"` via `debug.ReadBuildInfo()` →
    `vcs.revision` + `vcs.modified`.
  - Returns `"dev"` if no VCS info.
- This display string is NOT comparable. main.go wires it once:
  `cmd.Version = resolveVersion(version)` (cobra --version prints it).
- **resolveVersion() MUST stay byte-for-byte unchanged** (it feeds --version). This task only ADDS
  a one-line push of the *raw* ldflags `version` into the upgrade package.

### ldflags value shape (CRITICAL for ParseAndClean — must handle BOTH)
- Makefile: `VERSION ?= dev`; `LDFLAGS := -X main.version=$(VERSION)`.
  - `make build VERSION=v1.2.3` → injects **WITH** leading v ("v1.2.3").
  - `make build` (no VERSION) → injects "dev".
- goreleaser (.goreleaser.yaml line 34): `-X main.version={{.Version}}` where the comment states
  `{{.Version}}` = tag WITHOUT leading "v" (e.g. "1.0.0").
  - So a goreleaser release injects **WITHOUT** leading v ("1.0.0").
- => CurrentSemver/ParseAndClean MUST accept BOTH "1.0.0" and "v1.0.0" as valid release inputs.
- Release tag shape from GitHub Releases API (`tag_name`) and goreleaser `.Tag` = "vX.Y.Z" (WITH v).
  => Compare(a,b) MUST normalize leading-v asymmetry between the two operands.

### debug.ReadBuildInfo().Main.Version is NOT the source (seam decision rationale)
- For a binary built from a checkout (goreleaser, `make build`, bare `go build`), Main.Version is
  `"(devel)"` — NOT the release tag. Reading it would always yield ok=false ("development build"),
  so `--check` on a real goreleaser release would falsely report "development build — cannot compare".
- The ldflags `main.version` is the ONLY authoritative source of the release version.
- cmd/stagecoach cannot be imported by internal/upgrade (import cycle + it's package main).
- => DECIDED SEAM: main.go calls `upgrade.SetCurrentVersion(version)` ONCE at startup
  (right next to the existing `cmd.Version = resolveVersion(version)` line). The upgrade package
  stores it in a package var; `CurrentSemver()` reads it. Tests inject via the same setter and
  restore it via t.Cleanup. This matches the contract's sanctioned suggestion and the milestone's
  "injectable/mockable for tests" requirement, and the P1.M4.T3.S1 consumer's "injected CurrentSemver".

### Dependency P1.M1.T1.S1 is COMPLETE
- internal/exitcode/exitcode.go already has `UpdateAvailable = 6`, `ErrUpdateAvailable` sentinel,
  and the For() branch. This task does NOT touch exitcode; it's a sibling primitive consumed later
  by P1.M4.T2.S1 (--check returns exitcode.New(exitcode.UpdateAvailable, nil) when behind).

### Package placement decision
- Contract offers `internal/upgrade/version.go` OR `internal/version`.
- system_context.md: "New files: internal/upgrade/* (the package)".
- The whole upgrade subsystem (HTTP client P1.M1.T3, detect P1.M2, swap P1.M3, config P1.M1.T2)
  lives under internal/upgrade/. SetCurrentVersion lives on the upgrade package per the contract.
  => DECIDED: package `internal/upgrade`, file `internal/upgrade/version.go`, package name `upgrade`.
  (This is the FIRST file in that package — it will be created fresh.)

### Test style to mirror (repo convention)
- internal/exitcode/exitcode_test.go: table-driven `tests := []struct{name; ...; want}` with
  `for _, tc := range tests { t.Run(tc.name, func(t *testing.T){...}) }`; arrow chars `→`/`<` used in
  names; small standalone TestXxxCodeValue funcs for const/value assertions; same package (white-box).
- I'll mirror this exactly for version_test.go: TestCompare table + TestParseAndClean table +
  TestCurrentSemver table (injecting via SetCurrentVersion + t.Cleanup restore).

### go toolchain
- go1.26.5. `go vet`, `go test`, `gofmt -l` are the gates. gofmt-clean baseline confirmed (empty).

## 2. semver 2.0.0 precedence rules (canonical, used for Compare)
Spec: https://semver.org/ (Semantic Versioning 2.0.0). Relevant items:
- §2: normal form `MAJOR.MINOR.PATCH`, all non-negative integers, no leading zeros.
- §9: prerelease appended with `-`: `1.0.0-alpha.1` → prerelease=`alpha.1` (dot-separated identifiers).
- §10: build metadata appended with `+`: `1.0.0+001`; IGNORED in precedence (`1.0.0+build == 1.0.0`).
- §11 precedence (the core of Compare):
  1. Precedence = compare major, then minor, then patch, each NUMERICALLY (lexically wrong for "10">"9").
  2. A version WITHOUT prerelease > the SAME core WITH prerelease (`1.0.0 > 1.0.0-rc1`). <- contract test
  3. Two prereleases: compare dot-separated identifiers left to right until they differ:
     - both all-digits → compare numerically (so 2 > 10, not "2" > "10").
     - both non-numeric → compare lexically (ASCII).
     - one numeric + one non-numeric → numeric is LOWER.
     - if all preceding equal, the set with MORE fields is GREATER (`1.0.0-alpha < 1.0.0-alpha.1`).
- Leading "v" is NOT part of the spec — it's a git/GitHub tag convention. We strip it before parsing.

## 3. Reference implementation we deliberately do NOT add (golang.org/x/mod/semver)
- https://pkg.go.dev/golang.org/x/mod/semver — `Compare`, `IsValid`, `Major`, `MajorMinor`.
- It REQUIRES a leading "v" (v-prefixed canonical). Our ParseAndClean must accept BOTH with/without v,
  so we can't use it as-is without normalizing; and the contract decision is "stdlib-only, no new dep"
  (matches the repo's minimal-deps go.mod). We implement a small correct subset instead.
- Behavior to replicate: build metadata ignored; prerelease precedence per §11; missing-v tolerated
  on input but we normalize OUTPUT to v-prefixed canonical (matches goreleaser .Tag + API tag_name).

## 4. Output-normalization decision (ParseAndClean / CurrentSemver return shape)
- Decide: normalize to **v-prefixed canonical** `vMAJOR.MINOR.PATCH[-prerelease]`
  (build metadata stripped). Rationale: the GitHub Releases `tag_name` and goreleaser `.Tag` are
  v-prefixed ("v1.2.3"), so current and latest end up in the SAME canonical form for the consumer's
  symmetric compare/display. Consumers may strip the leading "v" for display if desired (P1.M4.T2.S1).
- ParseAndClean("1.0.0")→("v1.0.0",true); ("v1.0.0")→("v1.0.0",true);
  ("1.0.0-rc1")→("v1.0.0-rc1",true); ("dev")→("",false); ("1.0")→("",false); ("")→("",false).
- CurrentSemver() = ParseAndClean(currentVersion). For release "1.0.0"→("v1.0.0",true); "dev"→("",false).

## 5. Compare result convention
- Return int: -1 if a<b, 0 if a==b, +1 if a>b (mirrors golang.org/x/mod/semver.Compare and
  strings/builtin cmp conventions). Invalid operand → treat as not-comparable? Decision: Compare is
  defined ONLY for parseable operands; if either fails to parse, return 0 (equal) so a bad/unparseable
  current never falsely claims an update is available (defense-in-depth for FR-U6's dev-build rule:
  unparseable current → no update claim). Document this clearly. (The --check path additionally gates
  on CurrentSemver's ok flag before even calling Compare, so this is belt-and-suspenders.)

## 6. Contract test table (must pass — from context_scope OUTPUT)
- Compare("1.0.0","1.0.1") < 0   (1.0.0 < 1.0.1)
- Compare("1.0.0","v1.0.0") == 0 (missing-v tolerated, equal)
- Compare("1.0.0","1.0.0-rc1") > 0 (release > prerelease of same core)
- Compare("2.0.0","1.9.9") > 0   (numeric major/minor, not lexical)
- CurrentSemver: ok=true for a release ("0.1.0"/"v0.1.0"), ok=false for "dev".
- Additional prerelease tests to add (full-spec correctness): rc1<rc2, alpha<beta, alpha.1<alpha.2,
  alpha<beta.1, "1.2.3"==v1.2.3, build-metadata ignored "1.0.0+build"==v1.0.0, "1.0.0-1"<"1.0.0-2".

## 7. Mode A DOCS impact
- Package doc comment on internal/upgrade (version.go top) explaining: DISPLAY version
  (main.resolveVersion → "dev (sha-dirty)", feeds --version, NOT comparable) vs COMPARABLE version
  (this package's CurrentSemver/Compare, for --check/upgrade decisions). Cite FR-U5 step1 / FR-U6.
