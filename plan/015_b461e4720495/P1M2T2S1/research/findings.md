# P1.M2.T2.S1 Research Findings — Global default 480s→120s + fix pinning tests

Source: codebase-wide `grep -rn '480'`, reading each hit's context, `architecture/critical_findings.md`
Finding 5, and the (already-landed) P1.M2.T1.S1 PRP + roles.go.

## 0. Dependency state: P1.M2.T1.S1 ALREADY LANDED

`internal/config/roles.go` already contains the planner built-in + `ResolveRoleTimeout`:
- `var defaultRoleTimeouts = map[string]time.Duration{"planner": 480 * time.Second}` (roles.go:12).
- `ResolveRoleTimeout(role, cfg)` applies per-role > built-in(planner 480s) > global(cfg.Timeout).

**Therefore changing the global `Defaults().Timeout` from 480s→120s does NOT affect the planner** — the planner
takes its 480s built-in (which BEATS the global) unless a per-role override exists. This is the whole reason the
tasks were split. Verify: `ResolveRoleTimeout("planner", Defaults-with-Timeout=120s)` still returns 480s.

## 1. THE core change (1 line) + its 4 direct mirrors

| File:line | Current | Change to |
|---|---|---|
| `internal/config/config.go:200` | `Timeout: 480 * time.Second,` | `Timeout: 120 * time.Second,` ← THE default |
| `internal/config/config.go:71` | comment `// generation timeout; Defaults: 480s` | `120s` |
| `internal/config/config.go:183` | comment `// Defaults returns ... timeout 480s ...` | `120s` |
| `internal/config/bootstrap.go:161` | `# timeout        = "480s"` (generated template comment) | `"120s"` |
| `internal/cmd/root.go:169` | help text `...; default 480s)` | `default 120s)` |

## 2. The 5 DEFAULT-PINNING tests (MUST flip to 120s)

These assert the resolved global default equals 480s. After the change they FAIL until flipped:
- `internal/config/config_test.go:20-21` — `if c.Timeout != 480*time.Second { ... want 480s }` (TestDefaults)
- `internal/config/file_test.go:898` — `if dst.Timeout != 480*time.Second` (TestOverlayNilSrc; dst=Defaults())
- `internal/config/load_test.go:693-694` — `if cfg.Timeout != 480*time.Second { ... want 480s (default) }`
- `internal/cmd/root_test.go:246-247` — `if cfg.Timeout != 480*time.Second { ... want 480s (default) }` ← NOT in
  the original contract list (line drift); caught by the `grep -rn '480'` sweep. IN SCOPE.
- `internal/config/file_test.go:113` — comment `// Layer-1 baseline (..., Timeout=480s, …)` (describes Defaults()) → `120s`

## 3. Source COMMENTS that describe the default (accuracy — update to 120s)

- `internal/provider/executor.go:28` — `The Config.Timeout default is 480s (PRD FR25).` → `120s`
- `internal/cmd/models.go:144` — `timeout = cfg.Timeout // bound (FR25 knob; default 480s)` → `120s`
- `pkg/stagecoach/stagecoach.go:40` — `Timeout time.Duration // ...; 0 → config default (480s)` → `120s`
- `internal/generate/realagent_test.go:62` — `// Timeout/MaxDuplicateRetries inherit config.Defaults() (480s/3).` → `(120s/3)`
- `internal/generate/realagent_test.go:136` — `// RUN: ... Timeout per attempt = cfg.Timeout (480s).` → `(120s)`
  (realagent_test.go is behind `//go:build integration_real` so it never runs in `make test`, but update for accuracy.)
- `internal/config/roles.go:75` — godoc line `> [defaults].timeout (cfg.Timeout — the global; 480s today, 120s after P1.M2.T2)`
  → now describes the LANDED state: `(cfg.Timeout — the global; 120s)`. COMMENT ONLY — the planner built-in VALUE on
  roles.go:12 stays 480s (do NOT touch it).

## 4. DOCS (the contract: docs/configuration.md rides WITH the work)

- `docs/cli.md:27` — table row `--timeout ... "480s"` → `"120s"`
- `docs/configuration.md:86` — generated-example comment `# timeout = "480s"` → `"120s"`
- `docs/configuration.md:133` — default-value table `| timeout | 480s | config.Defaults() |` → `120s`
- (No README.md reference to 480 — verified empty.)

## 5. LEAVE — the planner 480s built-in (roles.go) — DO NOT TOUCH THE VALUE

- `internal/config/roles.go:6, 12, 74, 81, 84, 85` — the planner 480s built-in + its godoc. STAYS 480s. (Only :75 is a
  global-default comment → update per §3.) Changing these would undo P1.M2.T1.S1 and break the planner's headroom.

## 6. LEAVE — parseTimeout doc examples (the string "480s" / "480" as PARSER examples, not the default)

These document that `parseTimeout` accepts a duration string; 480 is an arbitrary example value, NOT the default:
- `internal/config/load.go:318` — `// accepts "480s" and bare "480"`
- `internal/config/file.go:28` — `Timeout string ... e.g. "480s"; parsed in materialize (parseTimeout accepts "480s" OR bare "480")`
- `internal/config/file.go:324, 333` — parseTimeout call-site comments
Leave them — they illustrate the parser, and "480s" is a valid example regardless of the default.

## 7. LEAVE — arbitrary-value tests (480 used as a NON-default value in parser/role/merge tests)

These use 480 as a deliberate distinct value to prove parsing/merge, NOT to pin the global default:
- `internal/config/file_test.go:577-585` — `TestLoadTOMLRoleTimeoutValid`: fixture `[role.planner] timeout = "480s"` asserts the
  ROLE parses to 480s (a per-role value, not the global). LEAVE.
- `internal/config/file_test.go:699-707` — overlay field-merge test: `src` planner `Timeout: 480*time.Second` vs dst `300s`; proves
  higher-layer-wins (arbitrary distinct durations). LEAVE.
- `internal/config/file_test.go:1278-1342` — `parseTimeout` table tests: `{"480s" → 480s}`, `{"480" → 480s}`. Tests the PARSER.
  LEAVE (480 is a valid test input; changing is churn unrelated to the default).
- `internal/config/load_test.go:327-335, 370-378, 1129` — planner per-role loading tests (`setRoleTimeout("planner", 480s)`,
  `STAGECOACH_PLANNER_TIMEOUT=480s`). Per-role values, not the global. LEAVE.
- `internal/config/multiturn_test.go:80,108,113,114` — `MultiTurnChunkTokens: 48000` (a different number entirely). LEAVE.

## 8. Why this is safe (no behavioral surprise)

- `Defaults().Timeout` flows into `cfg.Timeout` (Layer-1 baseline) → consumed by `provider.Execute` (single-commit) and by
  `ResolveRoleTimeout` for the non-planner roles (stager/message/arbiter). Lowering 480s→120s tightens THOSE roles only.
- The planner is unaffected: `ResolveRoleTimeout("planner", cfg)` returns the 480s built-in whenever no per-role override exists,
  regardless of `cfg.Timeout`. (roles_test.go:213 `TestResolveRoleTimeout_PlannerBuiltinBeatsGlobal` pins this with cfg.Timeout=120s.)
- No config-version bump / migration: a default change is not a schema change (old files with no `timeout` key just resolve to 120s
  now; a file with `timeout = "300s"` is untouched — file layer beats Layer-1 default).
- `bootstrap.go:161` is a COMMENTED line in the generated template; `bootstrap_test.go` has ZERO references to 480/timeout (verified),
  so the change is safe (no byte-exact test).

## 9. Validation (verified Makefile commands)

- `make test` (= `go test -race ./...`) — the 5 default-pinning tests are the canary: they FAIL if config.go:200 changed but a
  test didn't, and they FAIL (compile/time) if a test changed but config.go didn't. They must all move together.
- `make lint` (golangci-lint) — comments-only changes don't affect it.
- `make coverage-gate` — internal/config is in the gate set; the change is neutral-to-positive on coverage.
- `gofmt -l` — no struct changes; only literal/comment edits → stays empty.
- Grep guards: exactly ONE `Timeout: 120 * time.Second` in Defaults(); ZERO `480` left in the default-pinning test lines; the
  planner built-in (`roles.go:12`) still `480 * time.Second`.

## 10. The single subtlety: 480 appears in ~3 unrelated roles

The trap in this task is over-reaching or under-reaching on the grep sweep. The 480 token appears as: (a) the global default + its
mirrors/tests [CHANGE], (b) the planner built-in [KEEP], (c) parseTimeout examples + arbitrary merge/parser test values [KEEP]. The
PRP's "LEAVE" lists (§5/§6/§7) are as important as the "CHANGE" list — they prevent both false positives (breaking the planner) and
false negatives (missing root_test.go, which the contract's line-number list omitted due to drift).
