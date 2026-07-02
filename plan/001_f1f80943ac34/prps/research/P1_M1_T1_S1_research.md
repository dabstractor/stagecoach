# Research Notes — P1.M1.T1.S1 (go.mod, cobra stub, Makefile, .gitignore)

## Environment (verified 2026-07-01 on host)
- Go: `go1.26.4` (PRD §22.3 requires ≥1.22 ✅)
- git: `2.54.0` (PRD requires ≥2.20 ✅)
- `git describe --tags --always --dirty` in this repo → `a830674` (no tags yet; falls back to commit short-SHA). VERSION var will be a SHA until first tag.
- `golangci-lint`: **NOT installed** → Makefile `lint` target must EXIST (contract) but validation gates must NOT execute it.
- `goreleaser`: not installed (M8 only).

## Dependency resolution (verified via probe in /tmp)
- `go get github.com/spf13/cobra@v1.10.2` → adds cobra + pflag v1.0.9 + mousetrap v1.1.0. ✅
- `go get github.com/pelletier/go-toml/v2@v2.4.2` → adds go-toml/v2. ✅
- cobra v1.10.2 declares `go 1.15`; go-toml/v2 v2.4.2 declares `go 1.21.0`. ⇒ both build under a `go 1.22` directive (PRD minimum).

## CRITICAL GOTCHA — go mod tidy strips go-toml
Probe showed: after `go build` (main.go imports ONLY cobra), go.mod has go-toml as `// indirect`.
`go mod tidy` then **REMOVES go-toml/v2 entirely** because no source file imports it (first import is M5/config/file.go).
⇒ DO NOT run `go mod tidy` in this task. Both deps remain in go.mod (cobra direct, go-toml indirect). M5.T2 promotes go-toml to direct on import.

## cobra --version behavior (verified empirically)
Using root `*cobra.Command{Use:"stagehand", Version: version}` with `var version="dev"` and NO Run:
- `./bin/stagehand --version` → prints `stagehand version <version>` to **STDOUT**, exit 0. ✅ (matches contract "prints the version and exits 0")
- bare `./bin/stagehand` (no args, no Run) → prints help (Short/Long + flags), exit 0. ✅ (matches "print help")
- cobra ALSO auto-registers `-v` shorthand for the version flag (bonus, harmless).
- ldflags `-X main.version=v0.1.0-1-gabc-dirty` correctly overrides the `dev` default. ✅

### Reconciling the contract's "persistent --version flag bound to var version string"
A literal `PersistentFlags().StringVar(&version,"version",...)` would make `--version` REQUIRE an argument
(`flag needs an argument`) → breaks `stagehand --version` (no value). The ONLY clean cobra way to get
`--version` (no arg) to print+exit-0 with NO Run is the built-in `rootCmd.Version` field. That flag is
root-LOCAL (not persistent), which is sufficient for `stagehand --version`. If `--version` must propagate to
subcommands in M7, revisit then — out of scope for M1 (no subcommands exist yet). Decision: use `Version` field.

## go directive decision
`go mod init` records the toolchain version (1.26.4) by default, which would force `go install` users to need
Go ≥1.26. Instruct `go mod edit -go=1.22` to pin the PRD §22.3 minimum → maximizes install compatibility
(PRD §21.3 `go install ...@latest`). No dep requires >1.21.

## gofmt gate behavior (verified)
`gofmt -l .` RECURSES into subdirs but ALWAYS exits 0 (it lists unformatted files). ⇒ robust gate is
`test -z "$(gofmt -l .)"` (fails when any file is listed). `gofmt -l .` recurses ✅ (listed cmd/stagehand/bad.go).

## Makefile gotcha
Recipes MUST start with a TAB, not spaces. `make` errors `*** missing separator` otherwise.

## Scope boundaries (do NOT do in this task)
- No `internal/` packages (M1.T2 onward).
- No Run/RunE on root cmd (M7.T2).
- No README / docs (Mode B, M8.T4). DOCS impact = none.
- No `.goreleaser.yaml` (M8).
- No `providers/*.toml` (M2/M8).
