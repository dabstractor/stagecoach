# Research Notes — P1.M7.T3.S2: config init / path subcommands (FR38)

## Work item contract (verbatim essentials)
- `config init`: write the commented example config (PRD §16.2 template, **all
  sections commented**) to the resolved global path; refuse to overwrite an
  existing file unless `--force`; on user confirm, offer to add `./.stagehand.toml`
  to a local `.gitignore` (PRD §16.1). 
- `config path`: print the resolved global config path (XDG-aware).
- MOCKING: init writes valid TOML that **parses back to defaults**; existing-file
  no-clobber; `--force` overwrites; path prints the expected XDG/default location.
- DOCS: Mode A — the **generated example config IS the canonical config-file
  reference** (every key documented inline).
- Dependency: P1.M5.T2.S1 = `config.GlobalConfigPath()` (DONE).

## Dependency surface (already built — consume, do not modify)
- `internal/config/file.go` → `func GlobalConfigPath() (string, error)`:
  honors `XDG_CONFIG_HOME`, falls back to `$HOME/.config`, always appends
  `stagehand/config.toml`. **Exported.** Its doc explicitly states it is "the
  single source of truth for ... the CLI `config path` / `config init`
  subcommands (P1.M7.T3.S2)". Both XDG+HOME empty → hard error.
- `internal/config/file.go` in-package (white-box only): `fileDTO`,
  `defaultsDTO`, `generationDTO` (pointer-per-scalar TOML shape), and
  `parseDuration(s)` (accepts "120s" or bare int seconds).
- `internal/config/defaults.go` → exported `Default*` constants + `Default()`:
  DefaultTimeout=120s, DefaultAutoStageAll=true, DefaultMaxDiffBytes=300000,
  DefaultMaxMdLines=100, DefaultMaxDuplicateRetries=3, DefaultSubjectTargetChars=50,
  DefaultOutput=provider.OutputRaw("raw"), DefaultStripCodeFence=true.
  Default() leaves Provider/Model/Verbose/NoColor at zero (the empty/false defaults).
- `internal/ui/exitcode.go`: ExitSuccess=0, ExitError=1. Cobra RunE returning a
  non-nil error → rootCmd.Execute() returns err → main.go os.Exit(1). No manual
  os.Exit needed for the no-clobber error path.

## CLI pattern (sibling template: cmd/stagehand/providers.go — MUST mirror)
- Package `main`, cobra. Self-register via a package-level `init()` in the new
  sibling file so **main.go stays untouched** (zero conflict with P1.M7.T2.S1,
  which owns rootCmd.Run + persistent flags). main.go only defines rootCmd +
  calls rootCmd.Execute(); on error os.Exit(1).
- Parent command has no Run (cobra prints help on bare `stagehand config`).
- Render/I/O helpers are PURE functions (hermetic test targets): take `io.Writer`
  / `io.Reader` + explicit args; never touch global state except the env-derived
  path (which tests pin via `t.Setenv("XDG_CONFIG_HOME", t.TempDir())`).
- Imports kept minimal: fmt, io, os, bufio, path/filepath; cobra; go-toml/v2;
  internal/config. **Do NOT import internal/git or internal/generate** (keeps the
  command leaf thin; matches providers.go invariant).

## Test conventions (mirrored from cmd/stagehand/providers_test.go + internal/config/*_test.go)
- White-box `package main` / `package config`; stdlib only (testing, bytes,
  strings, reflect, os, path/filepath, toml). **No testify.**
- Hermetic via `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` + `t.TempDir()` for
  the gitignore cwd. Read stdin via `cmd.SetIn(bytes.NewReader(...))` /
  `cmd.SetOut(&buf)`; or drive pure helpers directly.
- `t.Setenv` mutates the process env for the test goroutine; GlobalConfigPath
  reads it at call time — synchronous RunE/helper calls see it. Safe.

## Design decisions for THIS task
1. **Single source of truth for the template**: put the commented §16.2 example
   in `internal/config/example.go` as an exported `ExampleConfig() string`.
   Rationale: (a) it is config-domain content (the canonical file shape); (b)
   the config package already owns fileDTO + Default* + GlobalConfigPath — the
   template is their documentation companion; (c) the "parses back to defaults"
   MOCKING contract needs white-box access to fileDTO/parseDuration, which only
   an in-package test has. The CLI stays thin (writes the string + gitignore UX).
2. **"All sections commented"**: every line of the §16.2 template is prefixed
   with `# ` so the written file is a documented no-op (all defaults). Users
   uncomment a line to change that setting. Inline comments after values are
   valid TOML and ignored by the parser.
3. **"Parses back to defaults" interpretation**: the `[defaults]` + `[generation]`
   scalar blocks use the TRUE default values (provider="" / model="" documented
   as "empty = auto-resolve"; timeout="120s"; auto_stage_all=true;
   max_diff_bytes=300000; max_md_lines=100; max_duplicate_retries=3;
   subject_target_chars=50; output="raw"; strip_code_fence=true). When the test
   strips the leading `# ` and `toml.Unmarshal`s into `fileDTO`, those pointer
   fields must be non-nil and equal the Default* constants. The `[provider.pi]`
   / `[provider.myagent]` blocks are clearly-marked OPTIONAL examples
   (commented); they populate fileDTO.Provider as examples and the test only
   asserts the file parses without error (they are not "defaults").
4. **`config init` flow**: GlobalConfigPath() → if exists && !--force → error
   (exit 1, no write) → else MkdirAll(dir,0o755) + WriteFile(path,
   ExampleConfig(),0o644) → print "Wrote ..." → offerGitignore (only if
   `./.git` present; read one y/N line from stdin; append `.stagehand.toml` to
   `./.gitignore`, creating it if absent).
5. **`config path` flow**: GlobalConfigPath() → print path to stdout (exit 0).
   On the both-empty error, return it (exit 1).
6. **gitignore helper is hermetic**: `offerGitignore(out io.Writer, in io.Reader,
   cwd string) error` — pure-ish (stat `./.git`, read one line from `in`, write
   to `cwd/.gitignore`, print prompts to `out`). Default No on empty/EOF
   (non-interactive). Testable with fakes; NOT part of the 4 MOCKING behaviors
   (secondary coverage).
7. **Docs (Mode A)**: the inline-commented template IS the canonical config-file
   reference (every key documented inline). Additionally append a short, clearly
   additive "## Onboarding: config init / config path" section to
   docs/CONFIGURATION.md (owned task P1.M5.T3.S1 is COMPLETE, so no live merge
   risk). Do NOT duplicate the full key table (CONFIGURATION.md already owns it).

## Validation gates (verified working in this repo)
- `go build ./...` → exit 0 (verified).
- `go vet ./internal/config/ ./cmd/stagehand/` → clean (verified).
- `go test ./internal/config/` → ok (verified).
- New-file gates: `gofmt -l`, `go vet ./internal/config/ ./cmd/stagehand/`,
  `go test ./internal/config/ -run TestExampleConfig -v`,
  `go test ./cmd/stagehand/ -run TestConfig -v`, `go test ./...`.
- Smoke: `go run ./cmd/stagehand config path`; 
  `XDG_CONFIG_HOME=$(mktemp -d) go run ./cmd/stagehand config init --force`.

## External references
- cobra command tree + init() registration: https://pkg.go.dev/github.com/spf13/cobra
- XDG Base Directory Specification (CONFIG_HOME semantics): https://specifications.freedesktop.org/basedir-spec/latest/#variables
- go-toml/v2 Marshal/Unmarshal + inline comments: https://github.com/pelletier/go-toml/v2
