# Research: STAGEHAND_ → STAGECOACH_ env var rename (P1.M2.T1.S1)

Verified against the live codebase (`grep -rn 'STAGEHAND_' --include='*.go' . | wc -l` = **404**).
Source of truth for the bulk uppercase-prefix rename in Go files.

## The surface (404 occurrences across ~22 .go files)

Top files by count: load_test.go (83), load.go (46), root.go (24), default_action_test.go (23),
stagecoach_test.go (22), lock_scenarios_test.go (22), stubtest.go (17), config.go (16), scenarios_test.go (14),
bootstrap.go (14), cmd/stubagent/main.go (13), harness_test.go (10), config_test.go (9), realagent_test.go (8),
hook/script_test.go (7), hook_scenarios_test.go (7), generate_multiturn_failure_test.go (6), decompose_test.go (6),
hook_test.go (6), ui/output_test.go (5), + a handful more (1-4 each in ~8 other files).

Categories of `STAGEHAND_` occurrences (ALL in .go files):
1. **String literals** — `os.LookupEnv("STAGEHAND_PROVIDER")`, `os.Getenv("STAGEHAND_STUB_OUT")`,
   `t.Setenv("STAGEHAND_MODEL", ...)`, error messages (`fmt.Errorf("STAGEHAND_TIMEOUT: %w", err)`), etc.
2. **The per-role prefix literal** — load.go:263 `prefix := "STAGEHAND_" + strings.ToUpper(role)`.
3. **Comments/doc text** — load.go function docs, config.go Long descriptions, bootstrap.go template comments.
4. **Test function NAMES** — `TestLoad_STAGEHAND_CONFIG_EnvPath` / `_MissingFileFails` (load_test.go:928/979).
   These ARE Go identifiers; the sed renames them to `TestLoad_STAGECOACH_CONFIG_EnvPath` — harmless (Go's
   test runner discovers by `Test` prefix; no explicit calls to rename).
5. **Bootstrap template** — bootstrap.go:243-262 is a Go string literal listing env vars as comments for the
   written config; sed renames within the string → the generated config comments say `STAGECOACH_*`.

## NO Go identifiers (const/var/type/func) use STAGEHAND_ as a NAME

Verified: the grep for `STAGEHAND_` excluding string-literal (`"`) and comment (`//`) context found ONLY
comments and the two test function names. There is no `STAGEHAND_PREFIX` constant, no `STAGEHAND_EnvVar`
type. Every `STAGEHAND_` is a literal value, a comment, or a test-name. ⇒ The sed `s/STAGEHAND_/STAGECOACH_/g`
is SAFE: it cannot break any identifier resolution.

## The stub-binary coupling (CRITICAL — must rename consistently)

`internal/stubtest/stubtest.go` SETS the stub env vars (`optsEnvMap`: `"STAGEHAND_STUB_OUT"`,
`"STAGEHAND_STUB_EXIT"`, `"STAGEHAND_STUB_SLEEP_MS"`, `"STAGEHAND_STUB_STDERR"`, `"STAGEHAND_STUB_SCRIPT"`,
`"STAGEHAND_STUB_COUNTER"`, `"STAGEHAND_STUB_ARGSFILE"`). `cmd/stubagent/main.go` READS them via
`os.Getenv("STAGEHAND_STUB_OUT")` etc. (also `STAGEHAND_STUB_STDINFILE`, `STAGEHAND_STUB_MARKER`).
The sed renames BOTH files → setter writes `STAGECOACH_STUB_*`, reader reads `STAGECOACH_STUB_*` → they match.
ALSO: some tests set stub env vars DIRECTLY (e.g., `STAGEHAND_STUB_MARKER`, `STAGEHAND_STUB_STDINFILE` in
lock_scenarios_test.go / realagent_test.go). The sed catches those too. All consistent. ✓

## The per-role prefix

load.go:263: `prefix := "STAGEHAND_" + strings.ToUpper(role)` — this is the runtime-constructed prefix for
`STAGEHAND_PLANNER_PROVIDER`, `STAGEHAND_MESSAGE_MODEL`, etc. The sed renames the literal `"STAGEHAND_"` →
`"STAGECOACH_"`, so per-role env vars become `STAGECOACH_<ROLE>_*`. ✓

## The sed command (contract-specified) + portability

```bash
grep -rl 'STAGEHAND_' --include='*.go' . | grep -v '.git/' | xargs sed -i 's/STAGEHAND_/STAGECOACH_/g'
```

⚠️ **BSD/macOS sed portability**: macOS `sed -i` requires a backup-suffix arg (`sed -i '' 's/.../.../g'`);
GNU `sed -i 's/.../.../g'` works directly. The implementing agent should detect the platform OR use the
portable form. (CI runs on ubuntu-latest = GNU sed, so the contract's form works in CI. Dev on macOS needs
the `''` suffix.) A safe cross-platform alternative: `sed -i.bak 's/STAGEHAND_/STAGECOACH_/g'` then
`find . -name '*.bak' -delete` — works on both.

⚠️ **`xargs` with empty input**: if grep found nothing, `xargs sed` would read stdin (hang). In practice
404 hits guarantees non-empty input. Belt-and-suspenders: `xargs -r` (GNU) or the agent verifies grep
output before piping.

## Scope boundary (what this task does NOT touch)

- **Lowercase `stagehand_`** (git config keys `stagehand.provider`, error message prefixes `stagehand:`,
  comments) — those are P1.M2.T1.S2 (git config keys) and P1.M2.T3 (user-facing strings). The sed here is
  UPPERCASE `STAGEHAND_` only (case-sensitive).
- **Non-.go files** (docs/cli.md, docs/configuration.md, providers/*.toml, .goreleaser.yaml, Makefile, CI) —
  P1.M3 (build/CI) and P1.M4 (docs). The sed is `--include='*.go'`.
- **Import paths / module path** (`github.com/dustin/stagehand`) — already renamed by P1.M1.T1 (Complete).
  Those are lowercase anyway; the uppercase sed doesn't touch them.
- **Go identifiers** (`Stagehand`→`Stagecoach`, `stagehand`→`stagecoach` in var/func names) — P1.M1.T2.S1
  (Implementing). This task is ONLY the `STAGEHAND_` uppercase literal prefix.

## INPUT

"The compiled project from M1 with all identifiers renamed." When this task runs, Go identifiers are already
`Stagecoach`/`stagecoach` (P1.M1.T2.S1 done). The `STAGEHAND_` string literals survived the identifier
rename (they're values, not names). This task renames ONLY those literals.

## DOCS: Mode A — docs/cli.md and docs/configuration.md env var references are updated in P1.M4 (separate
docs task). This task is Go files only.
