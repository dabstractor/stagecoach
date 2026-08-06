name: "P1.M1.T1.S2 — Wire preservedDefaultProvider into runConfigInit so --force re-targets the template (BUG-001)"
description: >
  Fix BUG-001 at the call site: in runConfigInit (internal/cmd/config.go), move the `--force` flag read
  UP before the template-generation block and add `if force && providerName == "" { providerName =
  preservedDefaultProvider(path) }` so `config init --force` (without --provider) regenerates the
  template for the PRESERVED `[defaults] provider` instead of auto-detecting pi — eliminating the
  inconsistent `[role.stager]=pi` block that hard-fails decomposition under FR-R5b. Consumes the
  preservedDefaultProvider helper from sibling S1. Plus a doc-comment update (Mode A) and an
  integration regression test. No upstream changes (GenerateBootstrapConfig/MergeActiveSettings are
  correct for a correctly-targeted template).

---

## Goal

**Feature Goal**: Make `config init --force` (without `--provider`) produce an internally-consistent
config: the regenerated `[role.*]` blocks are targeted at the PRESERVED `[defaults] provider` rather
than auto-detected pi, so a user whose default is claude/codex/cursor/agy/qwen-code no longer gets an
injected `[role.stager] provider = "pi"` that breaks multi-commit decomposition with a confusing
FR-R5b error (BUG-001).

**Deliverable**: (1) `runConfigInit` edit (move the `force` read up + add the re-targeting guard before
the validation block); (2) a doc-comment update on `runConfigInit` documenting the BUG-001 re-targeting
behavior (Mode A); (3) an integration regression test `TestConfigInit_Force_ReTargetsToPreservedProvider`
that runs the real command via `Execute(context.Background())` and asserts NO `[role.stager] provider =
"pi"` is injected when the preserved default is claude.

**Success Definition**:
- `config init --force` (no `--provider`) on a config with `[defaults] provider = "claude"` produces a
  config whose `[role.stager]` block is consistent with claude (NOT `provider = "pi"`).
- `config init --force --provider pi` still targets pi explicitly (the `&& providerName == ""` guard).
- `config init` without `--force` (first-run auto-detect) is unchanged.
- The existing `TestConfigInit_Force_PreservesActiveSettings` (which uses `--provider pi`) still passes.
- `go build ./...`, `go test ./internal/cmd/...`, `make test`, `make lint` all pass.

## User Persona (if applicable)

**Target User**: A stagecoach user who runs `config init --force` to refresh their config template after
upgrading, while keeping a single-backend default (claude/codex/cursor/agy/qwen-code).

**Use Case**: User has `provider = "claude"`, `model = "sonnet"`; runs `config init --force`; then runs
`stagecoach` on a dirty un-staged tree (decompose). Today this fails with
`decompose: role "stager": model "sonnet" on pi must be inference/model`. After the fix, decompose works.

**Pain Points Addressed**: The confusing error names a provider (pi) the user never configured, on the
very common `config init --force` refresh workflow, breaking the headline decompose feature.

## Why

- **BUG-001 (Major)**: `config init --force` calls `GenerateBootstrapConfig("")` (auto-detect → pi) and
  then `MergeActiveSettings` preserves the user's `[defaults] provider = "claude"` but the freshly-
  generated `[role.stager] provider = "pi"` survives (the user's file has no `[role.stager]` to reconcile
  it). Result: default=claude, stager=pi → decompose's stager resolves provider=pi + model=""→"sonnet"
  (global fallback) → FR-R5b hard error. PRD §9.17 FR-B2 (force "regenerates the template structure ...
  preserves every active setting"), FR-B8, §9.15 FR-R5b, §13.6.
- **Fix at the right layer**: the inconsistency is introduced upstream (template generated for pi while
  the preserved default is claude); `MergeActiveSettings` cannot infer reconciliation by design. The fix
  re-targets the template to the preserved provider BEFORE generation — keeping the merge non-destructive
  AND the result consistent. PRD §h2.5 Recommendation (Option A).
- **Minimal, surgical**: 3 edits in one function + a doc comment + a test. No upstream function changes.

## What

**User-visible behavior**: `config init --force` without `--provider` now regenerates role blocks for the
preserved default provider (e.g. claude), not pi. With `--provider X`, X wins (unchanged). First-run
bootstrap (no existing file) auto-detects (unchanged).

**Technical change (one function edit + doc comment + test):**
1. Move `force, _ := cmd.Flags().GetBool("force")` from after `GenerateBootstrapConfig` up to before the
   `if tmpl { ... } else { ... }` block.
2. In the `else` branch, after `GetString("provider")` and before the validation block, add the
   re-targeting guard `if force && providerName == "" { providerName = preservedDefaultProvider(path) }`.
3. Update the `runConfigInit` doc comment (config.go:456-458) with the BUG-001 re-targeting sentence.

### Success Criteria
- [ ] `force` flag read moved up (before the template-generation block)
- [ ] `if force && providerName == "" { providerName = preservedDefaultProvider(path) }` added in the else branch
- [ ] `--force` (no `--provider`) on a claude-default config → NO `[role.stager] provider = "pi"` (regression test)
- [ ] `--force --provider pi` → still targets pi (existing test unchanged)
- [ ] no `--force` → auto-detect unchanged
- [ ] runConfigInit doc comment documents the BUG-001 re-targeting (Mode A)
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current runConfigInit structure (with verified line numbers), the 3-edit fix, the per-scenario
truth table proving each path stays correct, the test harness to mirror (with the explicit `--provider pi`
existing test that S2 must NOT break), and the scope fences are all enumerated below.

### Documentation & References

```yaml
- file: internal/cmd/config.go
  why: "THE change site. runConfigInit (line 459). Current structure: path=config.ResolveConfigPath(flagConfig)
        (:465); tmpl read (:466); if tmpl/else block (:468-480); providerName=GetString(\"provider\") (:471);
        validation block if providerName != \"\" { reg.Get } (:472-478); GenerateBootstrapConfig(providerName)
        (:479); force=GetBool(\"force\") (:482 — TOO LATE, must move up); if force { mergeExistingActiveSettings }
        (:490); writeBootstrapFile (:493)."
  pattern: "The else branch reads providerName then validates then generates. Insert the re-target guard
            BETWEEN the GetString and the validation block."
  critical: "LINE NUMBERS DRIFTED ~27 lines since this PRP was drafted (runConfigInit now ~:486; was :459).
             ANCHOR ON CONSTRUCTS, not line numbers. `path` is resolved at the top of runConfigInit
             (config.ResolveConfigPath(flagConfig)) and is in scope for the else branch —
             preservedDefaultProvider(path) compiles. `force` must move up (before the if tmpl/else block)
             so it is in scope inside else; its later uses (the `if force { mergeExistingActiveSettings }`
             block and the writeBootstrapFile call) still resolve. Locate anchors by grep:
             `grep -n 'GetBool(\"force\")\|GenerateBootstrapConfig\|GetString(\"provider\")\|if providerName != \"\"' internal/cmd/config.go`."

- file: internal/cmd/config.go (mergeExistingActiveSettings, line 446)
  why: "The sibling file-reading helper S1's preservedDefaultProvider is co-located with. Confirms the
        read-on-error-fallback idiom and that os/strings/config/provider are already imported (no new import
        for S2 either — S2 only calls the helper + GetBool/GetString, all already used)."

- file: internal/cmd/config_test.go
  why: "Test home (package cmd — internal test). TestConfigInit_Force_PreservesActiveSettings (line 581) is
        the EXACT harness to mirror for S2's regression test. It uses --provider pi EXPLICITLY, so S2's
        `&& providerName == \"\"` guard means it is UNAFFECTED (still passes)."
  pattern: "saveRootState/restoreRootState/resetFlags; setupNoRepo(t) → globalDir; writeConfigFile(t, globalDir,
            \"config.toml\", body); rootCmd.SetArgs([]string{\"config\",\"init\",\"--force\",...});
            Execute(context.Background()); read config.GlobalConfigPath(); assert content substrings."
  critical: "S2's new test must DROP `--provider pi` from SetArgs to exercise the BUG-001 path (providerName==\"\")."

- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/architecture/bug001_analysis.md
  why: "Full root-cause trace (why MergeActiveSettings can't fix it) + the Option-A fix design + the exact
        fixed-code shape + the edge-case table + the test strategy. THE design doc for this fix."
  section: "Fix Design (Option A); Test Strategy"

- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T1S1/PRP.md
  why: "S1 is the CONTRACT — it produces preservedDefaultProvider(path string) string. S2 consumes it.
        S1's Anti-Patterns forbid editing runConfigInit in S1 (S2 owns the wiring)."

- docfile: plan/018_29b859efcd56/bugfix/001_4be1bd5c953f/P1M1T1S2/research/findings.md
  why: "Verified runConfigInit line numbers, the 3-edit fix, the per-scenario truth table, the test design,
        scope fences."
```

### Current Codebase tree (relevant slice)

```bash
internal/cmd/
  config.go        # runConfigInit (459) — THE change site (move force up + add guard + doc comment)
  config_test.go   # +TestConfigInit_Force_ReTargetsToPreservedProvider (mirror line 581, drop --provider)
internal/config/
  bootstrap.go     # GenerateBootstrapConfig / buildBootstrapConfig / StagerFallback — UNCHANGED (correct for a targeted template)
  merge.go         # MergeActiveSettings — UNCHANGED (cannot infer reconciliation by design)
  inert.go         # ActiveSettings — UNCHANGED (consumed by S1's helper)
```

### Desired Codebase tree with files to be added

```bash
internal/cmd/config.go        # MODIFY: runConfigInit (move force read + add re-target guard + doc comment)
internal/cmd/config_test.go   # MODIFY (additive): +TestConfigInit_Force_ReTargetsToPreservedProvider
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (move force UP, not just add the guard): the guard `if force && providerName == ""` needs `force`
//   in scope inside the else branch. Today `force` is declared at :482 (AFTER GenerateBootstrapConfig at
//   :479). It MUST move up to before the `if tmpl` block. After the move, confirm force's later uses
//   (:490 merge, :493 writeBootstrapFile) still compile (they will — force is now declared earlier).

// CRITICAL (--provider must still win): the guard is `if force && providerName == ""`. The `&& providerName
//   == ""` is load-bearing — without it, --force would clobber an explicit --provider. TestConfigInit_Force_
//   _PreservesActiveSettings relies on --provider pi winning (it passes --provider pi + --force).

// S1 LANDED (confirmed): preservedDefaultProvider(path) exists in internal/cmd/config.go. The call
//   compiles. (If for any reason it's absent at implementation time, S1 must land first — S2 is
//   sequenced after S1.)

// GOTCHA (validation block stays as defense-in-depth): the existing `if providerName != "" { reg.Get(...) }`
//   block STAYS unchanged. preservedDefaultProvider already validates against the same registry, so the block
//   always passes for a preserved name — but keep it (it validates explicit --provider too, and is the
//   source of the "unknown provider" error message). Do NOT remove or restructure it.

// GOTCHA (path is already resolved): `path := config.ResolveConfigPath(flagConfig)` at :465 is in scope for
//   the whole function, including the else branch. Do NOT re-resolve it.

// GOTCHA (--template / --interactive unaffected): --template branches at :468 (content = exampleConfigTemplate,
//   no provider logic). --interactive branches at the very top. Neither touches providerName; the moved `force`
//   read does not change their behavior. The `if force { mergeExistingActiveSettings }` at :490 still runs for
//   --template (existing behavior, unchanged).

// SCOPE: S2 is runConfigInit + its doc comment + its test ONLY. Do NOT touch preservedDefaultProvider (S1),
//   GenerateBootstrapConfig/buildBootstrapConfig/StagerFallback, MergeActiveSettings/ActiveSettings,
//   writeBootstrapFile/backup, or docs/cli.md (P1.M1.T3.S1).
```

## Implementation Blueprint

### Data models and structure
None. Pure control-flow edit in one function + a doc comment + a test. No struct/type/schema changes.

### Implementation Tasks (ordered by dependencies)

> **Prerequisite — S1 has LANDED** (confirmed during research: `grep -c 'func preservedDefaultProvider'
> internal/cmd/config.go` == 1). The helper exists and is callable. NOTE: line numbers in this PRP were
> captured mid-research and the file has since drifted ~27 lines (runConfigInit is now at ~:486, was :459).
> ANCHOR ON CODE CONSTRUCTS (the grep/awk patterns below), not absolute line numbers. The construct
> anchors — "after `GetString(\"provider\")`", "before the `if providerName != \"\"` validation block",
> "after `GenerateBootstrapConfig`" — are drift-proof.

```yaml
Task 1: MODIFY internal/cmd/config.go — move the `force` flag read UP
  - CUT the line `force, _ := cmd.Flags().GetBool("force")` from its current position (config.go:482,
    after GenerateBootstrapConfig / after the if tmpl-else block).
  - PASTE it just BEFORE the `if tmpl {` block — i.e. right after `tmpl, _ := cmd.Flags().GetBool("template")`
    (config.go:466), before `var content string` (or before `if tmpl`). Group the two flag reads.
  - SANITY: `force` is now in scope for the else branch (Task 2) AND its existing later uses (the merge at
    ~:490 and writeBootstrapFile at ~:493). Confirm `go build ./internal/cmd/...` compiles.
  - DEPENDENCIES: none.

Task 2: MODIFY internal/cmd/config.go — add the re-targeting guard in the else branch
  - In the else branch, AFTER `providerName, _ := cmd.Flags().GetString("provider")` (config.go:471) and
    BEFORE the `if providerName != "" {` validation block (config.go:472), INSERT:
        // BUG-001 (FR-B2/FR-B8): --force re-targets the regenerated template to the PRESERVED [defaults]
        // provider instead of auto-detecting pi, so the [role.*] blocks are structurally consistent with
        // the preserved default. (Otherwise an inconsistent [role.stager]=pi + preserved default=claude
        // hard-fails decompose under FR-R5b: the stager resolves pi + the global model = a bare model on a
        // provider_flag provider.) --provider still wins (the && providerName == "" guard). Unknown/custom
        // preserved providers fall back to "" (auto-detect) via preservedDefaultProvider — never passed to
        // GenerateBootstrapConfig, so custom providers are not broken.
        if force && providerName == "" {
            providerName = preservedDefaultProvider(path)
        }
  - DO NOT change the validation block that follows — it stays as defense-in-depth (and validates --provider).
  - DEPENDENCIES: Task 1 (force in scope) + S1 (preservedDefaultProvider exists).

Task 3: MODIFY internal/cmd/config.go — update the runConfigInit doc comment (Mode A docs)
  - LOCATE the doc comment block immediately BEFORE `func runConfigInit` (grep `grep -n 'func runConfigInit'`;
    the comment is the lines directly above it — was :456-458, now drifted). After the existing
    "...delegated to config.GenerateBootstrapConfig (P1.M4.T4.S1)." line, APPEND:
        // BUG-001 (FR-B2/FR-B8): with --force and no --provider, the template is re-targeted to the PRESERVED
        // [defaults] provider (read from the existing file) instead of auto-detecting pi, so the generated
        // [role.*] blocks are structurally consistent with the preserved default (otherwise an injected
        // [role.stager]=pi breaks decompose under FR-R5b). --provider still wins; no --force → auto-detect
        // unchanged. See preservedDefaultProvider.
  - NOTE: this is the ONLY doc change in S2 (docs/cli.md / docs/configuration.md are P1.M1.T3.S1).
  - DEPENDENCIES: Task 2.

Task 4: ADD TestConfigInit_Force_ReTargetsToPreservedProvider in internal/cmd/config_test.go
  - PLACE: right after TestConfigInit_Force_PreservesActiveSettings (config_test.go:581) — same concern area.
  - MIRROR that test's harness EXACTLY, but (a) DROP `--provider pi` from SetArgs (to exercise providerName==""),
    and (b) change the assertions to the BUG-001 contract. Body:
        func TestConfigInit_Force_ReTargetsToPreservedProvider(t *testing.T) {
            _, origOut, origErr, origRunE := saveRootState(t)
            defer func() { restoreRootState(t, nil, origOut, origErr, origRunE); resetFlags(configInitCmd.Flags()) }()

            _, _, globalDir := setupNoRepo(t)
            // Single-backend, stager-capable default — the BUG-001 repro (providerName="" + --force).
            preExisting := `config_version = 3
[defaults]
provider = "claude"
model = "sonnet"
`
            writeConfigFile(t, globalDir, "config.toml", preExisting)

            rootCmd.SetOut(io.Discard)
            rootCmd.SetErr(io.Discard)
            rootCmd.SetArgs([]string{"config", "init", "--force"}) // NO --provider ⇒ providerName==""

            if err := Execute(context.Background()); err != nil {
                t.Fatalf("Execute err=%v, want nil", err)
            }

            data, err := os.ReadFile(config.GlobalConfigPath())
            if err != nil { t.Fatalf("cannot read config: %v", err) }
            content := string(data)

            // BUG-001 core: the stager must NOT be routed to pi (the auto-detected default). Pre-fix,
            // GenerateBootstrapConfig("") wrote [role.stager] provider = "pi" and MergeActiveSettings
            // could not reconcile it → decompose failed under FR-R5b.
            if strings.Contains(content, `provider = "pi"`) {
                t.Errorf("BUG-001 regression: config routes a role to pi despite preserved default=claude;\nthe [role.stager] block must target claude (or inherit it), never pi.\n%s", content)
            }
            // FR-B8 still holds: the preserved default is carried verbatim.
            if !strings.Contains(content, `provider = "claude"`) {
                t.Errorf("[defaults] provider=claude was not preserved (FR-B8 violation)\n%s", content)
            }
        }
  - NOTE on the assertion strength: `provider = "pi"` must not appear ANYWHERE in the re-targeted config
    (claude is the target; pi should not be referenced). This is the strongest, simplest content assertion
    and directly mirrors how the existing test asserts substrings. (Optional stronger check: load the config
    via config.Load + ResolveRoleModel("stager", cfg) and assert provider != "pi" — but the content
    assertion is sufficient and matches the existing test style.)
  - DEPENDENCIES: Tasks 1-3.

Task 5: VERIFY — build, the force tests, lint
  - go build ./...
  - go vet ./internal/cmd/...
  - gofmt -l internal/cmd/
  - go test ./internal/cmd/ -run 'ConfigInit_Force' -v   # new test + the unaffected --provider pi test
  - go test ./internal/cmd/... -v
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the moved flag read + re-target guard (runConfigInit else branch)
path := config.ResolveConfigPath(flagConfig)
tmpl, _ := cmd.Flags().GetBool("template")
force, _ := cmd.Flags().GetBool("force")          // ← MOVED UP (was after GenerateBootstrapConfig)
var content string
if tmpl {
	content = exampleConfigTemplate
} else {
	providerName, _ := cmd.Flags().GetString("provider")
	if force && providerName == "" {              // ← BUG-001 re-target to preserved provider
		providerName = preservedDefaultProvider(path)
	}
	if providerName != "" {                        // validation block UNCHANGED (defense-in-depth)
		reg := provider.NewRegistry(nil)
		if _, ok := reg.Get(providerName); !ok {
			return exitcode.New(exitcode.Error, fmt.Errorf("unknown provider %q ...", ...))
		}
	}
	content = config.GenerateBootstrapConfig(providerName) // now the CORRECT provider
}
if force {                                         // FR-B8 merge — now reconciles cleanly
	content = mergeExistingActiveSettings(path, content)
}

// PATTERN: the regression test mirrors TestConfigInit_Force_PreservesActiveSettings but DROPS --provider
//   (to hit providerName=="") and asserts NO `provider = "pi"` survives — the BUG-001 symptom.
```

### Integration Points

```yaml
NO struct / schema / upstream-function / public-API changes. One call-site edit + doc comment + test.

CODE:
  - internal/cmd/config.go::runConfigInit — move force read up + add re-target guard + doc comment
TESTS:
  - internal/cmd/config_test.go — +TestConfigInit_Force_ReTargetsToPreservedProvider

CONSUMED (from S1, must land first):
  - preservedDefaultProvider(path string) string (internal/cmd/config.go)

DOWNSTREAM / UNCHANGED (do NOT touch):
  - GenerateBootstrapConfig / buildBootstrapConfig / StagerFallback (correct for a targeted template)
  - MergeActiveSettings / ActiveSettings (cannot infer reconciliation by design — the fix is upstream of it)
  - writeBootstrapFile / WriteTimestampedBackup (FR-B8 — unrelated)
  - docs/cli.md, docs/configuration.md (P1.M1.T3.S1)
  - configVersionNotice / BUG-002 (P1.M1.T2.S1)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                # the moved force + the helper call must compile (force in scope; S1 landed)
go vet ./internal/cmd/...
gofmt -l internal/cmd/        # Expected: empty.
make lint                     # Expected: zero errors.
```

### Level 2: Unit/Integration Tests (Component Validation)

```bash
# The BUG-001 regression test + the unaffected --provider pi test
go test ./internal/cmd/ -run 'ConfigInit_Force' -v
# Expected: TestConfigInit_Force_ReTargetsToPreservedProvider PASSES (no `provider = "pi"`);
#           TestConfigInit_Force_PreservesActiveSettings still PASSES (--provider pi wins).

# Full cmd package
go test ./internal/cmd/... -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# Build the binary
make build

# The PRD reproduction (BUG-001), now fixed:
TMPDIR=$(mktemp -d)
mkdir -p "$TMPDIR"
cat > "$TMPDIR/repro.toml" <<'EOF'
config_version = 3
[defaults]
provider = "claude"
model = "sonnet"
EOF
HOME="$TMPDIR" XDG_CONFIG_HOME="$TMPDIR" ./bin/stagecoach config init --force --config "$TMPDIR/repro.toml"
echo "--- [role.stager] block after --force (no --provider) ---"
grep -A3 '\[role.stager\]' "$TMPDIR/repro.toml" || echo "(no [role.stager] block — inherits claude)"
# Expected: NO `provider = "pi"` in the stager block (pre-fix it WAS pi). Either provider = "claude" or
#           the block inherits the claude default.
grep -q 'provider = "pi"' "$TMPDIR/repro.toml" && echo "BUG-001 STILL PRESENT (pi routed)" || echo "BUG-001 fixed: no pi routing"
rm -rf "$TMPDIR"
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the re-target guard is present in runConfigInit
grep -n 'force && providerName == ""' internal/cmd/config.go
# Expected: 1 match (the guard).

# Grep guard: force is read BEFORE GenerateBootstrapConfig (moved up)
awk '/func runConfigInit/,/^}/' internal/cmd/config.go | grep -n 'GetBool("force")\|GenerateBootstrapConfig'
# Expected: GetBool("force") line number < GenerateBootstrapConfig line number.

# Grep guard: the BUG-001 doc comment is on runConfigInit
grep -n 'BUG-001' internal/cmd/config.go
# Expected: ≥2 (the doc comment + the inline guard comment).

# Scope-boundary guard: NO upstream function changed
git diff --stat -- internal/config/ internal/provider/
# Expected: empty (S2 is internal/cmd/ only).

# Scope-boundary guard: docs/cli.md NOT touched (P1.M1.T3.S1)
git diff --stat -- docs/
# Expected: empty.

# Regression guard: the existing --provider pi test still passes (it does → --provider wins)
go test ./internal/cmd/ -run 'TestConfigInit_Force_PreservesActiveSettings' -v
# Expected: PASS.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (force in scope; preservedDefaultProvider call compiles — S1 landed)
- [ ] `go vet ./internal/cmd/...` clean
- [ ] `gofmt -l internal/cmd/` empty
- [ ] `make lint` zero errors
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `force` flag read moved up (before the if tmpl/else block)
- [ ] `if force && providerName == "" { providerName = preservedDefaultProvider(path) }` added in the else branch
- [ ] `--force` (no `--provider`) on claude default → NO `provider = "pi"` (regression test)
- [ ] `[defaults] provider = "claude"` still preserved (FR-B8)
- [ ] `--force --provider pi` → still targets pi (existing test unchanged)
- [ ] no `--force` → auto-detect unchanged
- [ ] runConfigInit doc comment documents BUG-001 re-targeting (Mode A)

### Scope-Boundary Validation
- [ ] preservedDefaultProvider NOT modified (S1 owns it — consume only)
- [ ] GenerateBootstrapConfig/buildBootstrapConfig/StagerFallback UNCHANGED
- [ ] MergeActiveSettings/ActiveSettings UNCHANGED
- [ ] writeBootstrapFile/backup UNCHANGED (FR-B8)
- [ ] docs/cli.md / docs/configuration.md UNCHANGED (P1.M1.T3.S1)
- [ ] configVersionNotice / BUG-002 UNCHANGED (P1.M1.T2.S1)

### Code Quality
- [ ] The `&& providerName == ""` guard is present (--provider wins; no accidental clobber)
- [ ] The validation block stays as defense-in-depth (not removed/restructured)
- [ ] Doc comment cites BUG-001 + FR-B2/FR-B8 rationale
- [ ] Regression test mirrors the existing harness idiom and drops `--provider` to hit the bug path

---

## Anti-Patterns to Avoid

- ❌ Don't add the guard WITHOUT moving `force` up — `force` is declared after GenerateBootstrapConfig today; the guard (in the else branch, before generation) would not compile. Move the read up first.
- ❌ Don't drop the `&& providerName == ""` condition — without it, `--force` would clobber an explicit `--provider` (the existing `TestConfigInit_Force_PreservesActiveSettings` passes `--provider pi` and relies on pi winning).
- ❌ Don't remove/restructure the validation block (`if providerName != "" { reg.Get }`) — it stays as defense-in-depth and is the source of the "unknown provider" error for explicit `--provider`. preservedDefaultProvider already validates, so it always passes for a preserved name; leave it.
- ❌ Don't touch GenerateBootstrapConfig / StagerFallback / MergeActiveSettings — they are correct for a correctly-targeted template; the bug is that the template was targeted at the WRONG provider. The fix is entirely at the call site.
- ❌ Don't modify preservedDefaultProvider (S1 owns it). Confirm it landed (`grep -n 'func preservedDefaultProvider' internal/cmd/config.go`) before wiring; S2 is sequenced after S1.
- ❌ Don't re-resolve `path` — it is already `config.ResolveConfigPath(flagConfig)` at the top of runConfigInit and is in scope.
- ❌ Don't break the `--template` / `--interactive` paths — they branch before/around the provider logic; the moved `force` read does not affect them.
- ❌ Don't edit docs/cli.md or docs/configuration.md here — S2's "DOCS" is the runConfigInit Go comment only (Mode A); changeset-level docs are P1.M1.T3.S1.
- ❌ Don't forget the regression test — a bug fix without a guard regresses. Mirror `TestConfigInit_Force_PreservesActiveSettings` but DROP `--provider` to actually exercise providerName=="".

---

## Confidence Score: 9/10

One-pass success is very high: the fix is 3 surgical edits in one function with a verified current-state
structure and line numbers, the per-scenario truth table proves each path stays correct, the existing test
(`--provider pi`) is provably unaffected by the `&& providerName == ""` guard, and the regression test mirrors
a working harness verbatim. The -1 is for the hard dependency on S1: `preservedDefaultProvider` must land
before S2 compiles, and the implementer must confirm it (`grep`) rather than assume — plus the moved `force`
declaration must keep its later uses (:490 merge, :493 writeBootstrapFile) resolving, which is mechanical but
is the one compile risk. Both are mitigated by the Level-1 `go build` gate and the explicit prerequisite check.