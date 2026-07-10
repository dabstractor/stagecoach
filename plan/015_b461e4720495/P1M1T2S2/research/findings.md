# Research Findings — P1.M1.T2.S2 (per-role --<role>-timeout CLI flags + flag-loop parsing)

## 1. Prerequisite state: P1.M1.T2.S1 has LANDED (in working tree)

`git status` shows `internal/config/load.go` modified (M) by the in-flight S1 work. Both S1
deliverables already exist in the working tree (verified by reading the file):

- **`setRoleTimeout` helper** — load.go:66-78 (mirrors `setRoleReasoning`, takes `time.Duration`):
  ```go
  func (c *Config) setRoleTimeout(role string, d time.Duration) {
      if c.Roles == nil { c.Roles = make(map[string]RoleConfig) }
      rc := c.Roles[role]
      rc.Timeout = d
      c.Roles[role] = rc
  }
  ```
- **env `_TIMEOUT` branch** — load.go:315-325 (inside `loadEnv`'s per-role loop, after `_REASONING`):
  ```go
  if v, ok := os.LookupEnv(prefix + "_TIMEOUT"); ok && v != "" {
      d, err := parseTimeout(v)
      if err != nil { return fmt.Errorf("%s_TIMEOUT: %w", prefix, err) }
      cfg.setRoleTimeout(role, d)
  }
  ```

**CONSEQUENCE for S2**: the dependency (`setRoleTimeout`) is satisfied. S2 CONSUMES it; does NOT
re-add it. S2's flag-loop branch calls `cfg.setRoleTimeout(role, d)`.

## 2. The CRITICAL env-vs-flag error-handling asymmetry

This is the single most important implementation detail:

| Layer | Function signature | Bad-value behavior | Reason |
|-------|-------------------|--------------------|--------|
| ENV (S1) | `func loadEnv(cfg *Config) error` | **RETURNS wrapped error** | loadEnv returns error |
| FLAG (S2, this task) | `func loadFlags(cfg *Config, fs *pflag.FlagSet)` (NO error return) | **SILENTLY ignores** | loadFlags cannot return error |

The contract specifies the flag pattern explicitly:
```go
if fs.Changed(role + "-timeout") {
    if v, err := fs.GetString(role + "-timeout"); err == nil {
        if d, perr := parseTimeout(v); perr == nil {   // perr==nil guard = SILENT ignore on bad value
            cfg.setRoleTimeout(role, d)
        }
    }
}
```
This mirrors the **global `--timeout` flag** at load.go:433-438 EXACTLY (which also silently ignores
a malformed `--timeout abc`). It is NOT the env pattern (which returns an error). If the implementer
copy-pastes the env branch's `return fmt.Errorf(...)` it WILL NOT COMPILE (loadFlags has no error
return) — self-enforcing, but the intent is "silent ignore" per the global-flag precedent.

## 3. root.go structure (internal/cmd/root.go)

- **Per-role flag VARS** — single `var (...)` block, lines 66-81, ending with `flagArbiterReasoning string`.
  → ADD 4 vars (`flagPlannerTimeout`, `flagStagerTimeout`, `flagMessageTimeout`, `flagArbiterTimeout`)
    after `flagArbiterReasoning string`, before the block's closing `)`.
- **Flag REGISTRATIONS** — `init()`, the per-role reasoning block ends at line 251-253:
  ```go
  pf.StringVar(&flagArbiterReasoning, "arbiter-reasoning", "", "...")
  // --version is auto-added by cobra ...
  ```
  → ADD 4 `pf.StringVar(...)` registrations after `flagArbiterReasoning`, before the `// --version` comment.
- **Global `--timeout` registration** is at root.go:165 (STRING flag, zero default `""`).
  The contract's per-role help text mirrors this string-flag discipline.

## 4. load.go loadFlags per-role loop (load.go:452-470)

```go
// Per-role provider/model overrides (PRD §9.15 FR-R3, §15.2).
for _, role := range roleNames {
    if fs.Changed(role + "-provider") { ... cfg.setRoleProvider(role, v) ... }
    if fs.Changed(role + "-model")    { ... cfg.setRoleModel(role, v) ... }
    if fs.Changed(role + "-reasoning"){ ... cfg.setRoleReasoning(role, v) ... }
}
```
→ ADD `-timeout` branch AFTER the `-reasoning` branch, BEFORE the loop's closing `}`.

`roleNames` (load.go:19) = `["planner","stager","message","arbiter"]` — the loop is general; the
single branch handles all 4 roles.

## 5. newFlagSet test helper MUST be extended (load_test.go:53-77)

`newFlagSet` registers per-role `-provider`/`-model` but NOT `-timeout` (and not `-reasoning` — a
pre-existing gap, OUT OF SCOPE here). To `fs.Set("planner-timeout", ...)` + `fs.Changed(...)==true`
in a test, the flag MUST be registered. Without it `fs.Set` returns an error and `Changed` is false.

```go
for _, role := range roleNames {
    fs.String(role+"-provider", "", "")
    fs.String(role+"-model", "", "")
    fs.String(role+"-timeout", "", "")   // ← ADD (S2)
}
```
Adding a registration is SAFE for all existing tests (un-set flags: `Changed==false`, no behavior change).

## 6. docs/cli.md — TWO tables to update

git.go does NOT read any `stagecoach.role.*` keys (grep-confirmed: only scalar `stagecoach.*` keys).
Per-role git-config reading for timeout is **P1.M1.T2.S3** (planned, separate). So docs/cli.md uses
`—` in the git-config column for ALL per-role rows (existing provider/model/reasoning rows do too).

- **Main flags table** (~lines 47-59): add 4 rows after `--arbiter-reasoning` (line 59), before
  `--version`. Columns: `| flag | type | default | env | git-config | description |`.
- **Env/git mapping table** (~lines 433-443): add 4 rows after `--arbiter-reasoning` (line 443).
  Columns: `| flag | env | git-config |`.

Docs scope is docs/cli.md ONLY (contract "DOCS [Mode A]"). README/how-it-works/configuration are
P1.M4.T2.S1 (fenced out).

## 7. Flag help text follows the contract verbatim

The contract specifies the EXACT help text (mentions env + the forward-looking git key, mirroring how
the existing provider/model flag help texts mention `git stagecoach.role.planner` even though git.go
doesn't read per-role keys yet — a pre-existing advisory pattern):
```
"Per-role generation timeout for the planner (env STAGECOACH_PLANNER_TIMEOUT; git stagecoach.role.planner.timeout)"
```
Repeat for stager/message/arbiter.

## 8. Coordination with S1 (parallel, both touch load.go — DIFFERENT functions)

S1 edits `loadEnv` (adds setRoleTimeout helper + env _TIMEOUT branch). S2 edits `loadFlags` (adds
flag-loop _timeout branch). DIFFERENT functions, non-overlapping line regions → clean merge. S2's
load.go edit anchors on the `-reasoning` branch TEXT inside `loadFlags`, so it is robust to S1's
line drift. S1 does NOT touch root.go or docs/cli.md → no conflict there.

## 9. Validation commands (verified)

- `go build ./...` — compile (setRoleTimeout consumed)
- `go vet ./internal/cmd/... ./internal/config/...` — vet changed packages
- `gofmt -l internal/cmd/root.go internal/config/load.go internal/config/load_test.go` — format check
- `make lint` = `golangci-lint run`
- `make test` = `go test -race ./...`
- Targeted: `go test ./internal/config/... -run 'PerRoleTimeout|Timeout' -v`
