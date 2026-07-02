# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: a header-only consistency fix — add the missing reasoning env-var lines to
> the `bootstrapHeader` constant + a regression test. Issue 4 (minor). No behavior change; the header IS
> the documentation.

## 0. The exact change is 2 lines, inserted in one precise spot

`bootstrapHeader` (internal/config/bootstrap.go, the raw-string const consumed by `buildBootstrapConfig`
via `b.WriteString(bootstrapHeader)`) has an env-var block. The per-role `_PROVIDER / _MODEL` lines are
followed directly by `STAGEHAND_COMMITS`. The fix inserts 2 lines **between**
`#   STAGEHAND_ARBITER_PROVIDER / _MODEL   …` and `#   STAGEHAND_COMMITS   …`:

```
#   STAGEHAND_REASONING                  global reasoning effort: off|low|medium|high (PRD §9.8 FR35, §16.2)
#   STAGEHAND_<ROLE>_REASONING           per-role reasoning override (role = planner|stager|message|arbiter)
```

Use these strings VERBATIM (the task LOGIC gives them exactly). The `<ROLE>` literal is intentional — see §2.

## 1. CRITICAL CORRECTION — do NOT add `STAGEHAND_MAX_COMMITS`

Issue 4's prose ("Suggested Fix: Add the reasoning (and max-commits) env-var lines") mentions max-commits,
but the item_description §1 explicitly corrects this: **`STAGEHAND_MAX_COMMITS` is NOT an env var.**
Verified: `internal/config/load.go:301` reads `max-commits` only via `fs.Changed("max-commits")` +
`fs.GetInt` (a CLI FLAG), with NO `os.LookupEnv`. `docs/cli.md:36,157` show `—` in the env column for
`--max-commits`. So adding a `STAGEHAND_MAX_COMMITS` line would be FALSE documentation. Add ONLY the 2
reasoning lines. (The `--max-commits` FLAG is already documented in the header's CLI-flags section — no
env line is warranted.)

## 2. The `<ROLE>` shorthand is consistent with the CLI-flags section (not a style break)

The env-var block documents per-role provider/model as 4 EXPLICIT lines (PLANNER/STAGER/MESSAGE/ARBITER).
At first glance a single `STAGEHAND_<ROLE>_REASONING` line looks inconsistent — but the header's CLI-flags
section ALREADY uses the `<role>` compact shorthand: `# --<role>-provider / --<role>-model  (role =
planner|stager|message|arbiter)`. So the `<ROLE>` reasoning line matches that established shorthand, and
its "(role = planner|stager|message|arbiter)" enumeration matches verbatim. The task's 2-line choice is
well-founded and faithful to the header's existing patterns. Use it as given.

## 3. The 5 reasoning env vars are REAL (verified against load.go)

- Global: `STAGEHAND_REASONING` → `load.go:181` (`os.LookupEnv("STAGEHAND_REASONING")` → `cfg.Reasoning`).
- Per-role: `STAGEHAND_<ROLE>_REASONING` → `load.go:215` (loop over `roleNames={planner,stager,message,
  arbiter}` → `cfg.setRoleReasoning(role, v)`).
Documented at `docs/cli.md:43-49` + `docs/configuration.md:152-156`. These are exactly what the new header
lines surface. The header was simply never updated when FR-R6 reasoning shipped — pure docs drift.

## 4. No existing test pins exact header content → adding lines is safe

Grepped every test referencing the header env-var block. The only `STAGEHAND_*` mentions in
`internal/config/*_test.go` are in `load_test.go` and are `t.Setenv(...)` calls (exercising loadEnv), NOT
assertions on header text. `bootstrap_test.go` validates `buildBootstrapConfig` output with
`strings.Contains` (specific substrings) + a TOML-validity check (`TestBuildBootstrapConfig_ValidTOML`
unmarshals the output — the header is all comments, so 2 new comment lines don't affect it). No test
exact-matches the header. So the insertion breaks nothing; the new test is the only one that cares.

## 5. The new test — TDD, mirrors bootstrap_test.go's `assertContains` pattern

`bootstrap_test.go` has a helper `assertContains(t, content, substrs...)` and uses `strings.Contains`
throughout. Add `TestBuildBootstrapConfig_HeaderDocumentsReasoningEnvVars`:
`buildBootstrapConfig("pi", nil)` → `assertContains(t, content, "STAGEHAND_REASONING",
"STAGEHAND_<ROLE>_REASONING")`. It FAILS pre-fix (neither string is in the header today) and PASSES
post-fix → a real regression guard. Both assertions are independent (`STAGEHAND_REASONING` is NOT a
substring of `STAGEHAND_<ROLE>_REASONING` because of `<ROLE>_` in between), so both are needed.

## 6. No conflict with the parallel/future work

- Parallel **P1.M3.T1.S1** (running now) edits `internal/decompose/decompose.go` (runSingleShortcut
  index-sync) + `internal/cmd/default_action.go` + `internal/config/roles.go` +
  `internal/generate/generate.go`. It does NOT touch `internal/config/bootstrap.go` or `bootstrap_test.go`.
  No overlap. ✓
- Future **P1.M6.T1.S1** (docs sync) edits `README.md`/`docs/cli.md`/`docs/providers.md` — NOT the
  bootstrap header (a different file). No overlap.
- This subtask touches EXACTLY 2 files: `internal/config/bootstrap.go` (the const) +
  `internal/config/bootstrap_test.go` (the new test). go.mod/go.sum unchanged.

## Sources
- `internal/config/bootstrap.go` — `bootstrapHeader` const (the edit target) + `buildBootstrapConfig`
  (the test's entry point). READ-then-edit.
- `internal/config/bootstrap_test.go` — the `assertContains`/`strings.Contains` test pattern to mirror.
- `internal/config/load.go:181,215` — proves the 5 reasoning env vars are real; `:301` proves max-commits
  is a FLAG only (the §1 correction).
- `docs/cli.md:43-49,164-170` + `docs/configuration.md:152-156` — the documented env-var wording the
  header must match.
- The CLI-flags section of `bootstrapHeader` (`--<role>-provider` shorthand) — the precedent for the
  `<ROLE>` shorthand (§2).
