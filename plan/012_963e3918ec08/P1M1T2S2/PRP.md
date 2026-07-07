---
name: "P1.M1.T2.S2 — Rename cobra template function and verify full compilation"
description: |
  Rename the cobra usage-template function registration string `"stagehandFlagUsages"` →
  `"stagecoachFlagUsages"` in `internal/cmd/root.go`, in lockstep across all 5 occurrences (the
  AddTemplateFunc registration at :246, the two `strings.NewReplacer` template-token targets at :248-249,
  and the two comments at :240 and :253). The Go function `flagUsagesWrapped` (the registered VALUE) is
  UNCHANGED — only the string-literal name under which cobra registers it. One precise sed
  (`s/stagehandFlagUsages/stagecoachFlagUsages/g`) handles all 5; the token cannot match cobra's default
  `.FlagUsages ` substring (the replacer's search side, which must stay) or `flagUsagesWrapped`. The token
  appears NOWHERE else (root.go-only). Validation broadens the contract's `-run TestRoot` filter to
  `-run 'TestRoot|TestHelp'` because `TestHelp_FlagsWrappedWithinWidth` (root_test.go:428, runs
  `Execute(["--help"])`) is the only test that renders the usage template and would catch a
  registration/template-token mismatch. S1 (P1.M1.T2.S1) explicitly defers this token to S2. No docs.
---

## Goal

**Feature Goal**: Complete the cobra-template-function piece of the stagehand→stagecoach rename: the
usage-template helper is registered and referenced as `stagecoachFlagUsages` everywhere, so `--help` /
usage rendering resolves the template func under its new name and produces identical wrapped output.

**Deliverable**: ONE file modified — `internal/cmd/root.go` — via a single precise sed renaming all 5
occurrences of the literal token `stagehandFlagUsages` → `stagecoachFlagUsages` (1 registration string +
2 replacer template-token targets + 2 comments). No other file touched. No Go-identifier rename (the
function `flagUsagesWrapped` keeps its name).

**Success Definition**: `go build ./...` + `go vet ./...` clean; `go test ./internal/cmd/ -run
'TestRoot|TestHelp' -count=1` green (notably `TestHelp_FlagsWrappedWithinWidth`, which renders `--help`
end-to-end and proves the template func resolves under the new name); grep for `stagehandFlagUsages` in
production .go files returns zero; `--help` output is byte-identical to before (only the internal
template-func name changed).

## User Persona

**Target User**: The project rename effort (PRD §h2.30: "All references to 'stagehand' must be replaced
with 'stagecoach'") — this closes the one cobra-template token S1 deferred. Also any user running
`stagecoach --help` (the rename must not break help rendering).

**Use Case**: A user runs `stagecoach --help`. cobra renders the usage template; the template calls
`{{stagecoachFlagUsages .LocalFlags}}`; cobra resolves it via the `AddTemplateFunc("stagecoachFlagUsages",
flagUsagesWrapped)` registration; the wrapped flag-usage block renders. Pre-rename the token was
`stagehandFlagUsages`; this task makes registration + template token consistently `stagecoachFlagUsages`.

**User Journey**: `stagecoach --help` → cobra `UsageTemplate()` (the swapped template containing
`stagecoachFlagUsages .LocalFlags`) → resolves the `stagecoachFlagUsages` func → `flagUsagesWrapped(fs)`
→ `pflag.FlagUsagesWrapped(helpWrapWidth())` → wrapped help to stdout.

**Pain Points Addressed**: Removes the last cobra-plumbing "stagehand" residue S1 left (gotcha G5),
preventing a half-renamed state where the binary is `stagecoach` but its help template internally still
references a `stagehand*` func.

## Why

- **PRD §h2.30 mandates the rename.** "All references to 'stagehand' must be replaced with 'stagecoach'."
  The cobra template-func registration string is one such reference.
- **S1 explicitly deferred this token to S2.** S1 (P1.M1.T2.S1, parallel) renamed 8 Go
  identifiers/const-values containing "stagehand" but deliberately EXCLUDED `stagehandFlagUsages`
  (critical_findings.md F4 lists it; S1 gotcha G5 / anti-pattern: "Don't rename stagehandFlagUsages —
  it's S2's"). This task is that deferred piece.
- **A string-literal rename, not an identifier rename.** The token is the NAME under which cobra
  registers the template func (a `map[string]interface{}` key in cobra's `templateFuncs`). It is not a Go
  identifier — so S1's identifier-rename scope didn't cover it. Renaming just the string (and the
  template references that must match it) is the complete, correct change.
- **Mechanical + lockstep-critical.** The registration name and the two template-token targets emitted by
  `strings.NewReplacer` MUST agree, or cobra's `text/template` fails at render time ("function not
  defined"). A single sed over the file guarantees that lockstep.

## What

Rename the literal token `stagehandFlagUsages` → `stagecoachFlagUsages` at all 5 sites in
`internal/cmd/root.go`. No signature change, no identifier change, no behavioral change (the rendered
help is identical — only the internal template-func namespace name differs). No docs, no tests added.

### Success Criteria

- [ ] `internal/cmd/root.go` contains ZERO occurrences of `stagehandFlagUsages`.
- [ ] `internal/cmd/root.go` registers `cobra.AddTemplateFunc("stagecoachFlagUsages", flagUsagesWrapped)`.
- [ ] The two `strings.NewReplacer` targets emit `stagecoachFlagUsages .LocalFlags ` and
      `stagecoachFlagUsages .InheritedFlags ` (the replacer's SEARCH side `.LocalFlags.FlagUsages ` /
      `.InheritedFlags.FlagUsages ` is UNCHANGED).
- [ ] The Go function `flagUsagesWrapped` is UNCHANGED (still the registered value; its name has no "stagehand").
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l .` clean.
- [ ] `go test ./internal/cmd/ -run 'TestRoot|TestHelp' -count=1` green — in particular
      `TestHelp_FlagsWrappedWithinWidth` (renders `--help`, proving the template func resolves).
- [ ] No file other than `internal/cmd/root.go` is modified.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?_ **Yes.** This PRP quotes the verbatim current code at all 5 occurrence sites (with line
numbers), gives the single sed command, proves the token boundary is safe (cannot match cobra's default
`.FlagUsages ` substring or the `flagUsagesWrapped` function), confirms the token is root.go-only (no
second site), identifies the exact test (`TestHelp_FlagsWrappedWithinWidth`) that functionally proves the
rename, and flags the one contract correction (broaden `-run TestRoot` → `-run 'TestRoot|TestHelp'`).
S1's deferral is confirmed. No inference.

### Documentation & References

```yaml
# MUST READ — the rename mandate + the finding that flags this token
- file: PRD.md
  why: "§h2.30: 'this project was originally named \"stagehand\" and has been renamed. All references to
        \"stagehand\" must be replaced with \"stagecoach\".' §15.1 Synopsis: the binary is `stagecoach`."
  critical: "§h2.30 IS the authorization. The cobra template-func registration string is a 'reference to
             stagehand' that must be replaced."

- docfile: plan/012_963e3918ec08/architecture/critical_findings.md
  why: "F4 lists `stagehandFlagUsages` (cobra template func, internal/cmd/root.go) as a token containing
        'stagehand' that is NOT the env prefix — a false-positive caution for the env-var sed. It is this
        task's exact target."
  critical: "F4 is the source of truth that this token exists and is distinct from STAGEHAND_ env vars."

- docfile: plan/012_963e3918ec08/P1M1T2S1/PRP.md
  why: "The S1 CONTRACT (parallel). S1 renamed 8 Go identifiers/const-values but EXCLUDED
        stagehandFlagUsages (gotcha G5: 'do NOT rename stagehandFlagUsages — that cobra template func is
        P1.M1.T2.S2's territory'). Confirms the S1/S2 boundary: S1 = Go identifiers; S2 (THIS) = the
        cobra template-func string token. No overlap."
  critical: "Treat S1 as landed first (it does not touch this token). This task is the deferred cobra
             token only. Do NOT re-do any of S1's identifier renames."

- docfile: plan/012_963e3918ec08/P1M1T2S2/research/cobra_templatefunc_rename_notes.md
  why: "THIS task's research: the verbatim 5-occurrence inventory (line numbers), the single safe sed,
        the token-boundary safety proof, the 'root.go-only' confirmation, the TestHelp functional gate,
        and the contract's `-run TestRoot` correction (D2). READ THIS FIRST."
  critical: "§1 (the 5 sites) + §4 (the TestHelp gate + the -run filter correction) are the spec.
             §2 (the sed) is copy-paste-ready. §5 is the do-NOT-do scope fence."

- file: internal/cmd/root.go
  why: "THE edit target. The 5 occurrences: comment :240, AddTemplateFunc registration :246, two
        NewReplacer targets :248-249, comment :253. The function flagUsagesWrapped (:258) is UNCHANGED.
        The replacer's search side (.LocalFlags.FlagUsages / .InheritedFlags.FlagUsages) is UNCHANGED."
  pattern: "cobra.AddTemplateFunc(name, fn) registers a template func under the STRING `name`; the
            usage template references it as `{{name .Arg}}`. The strings.NewReplacer swaps cobra's
            default `.FlagUsages ` token → `<name> .FlagUsages ` so the rendered template calls our func."
  gotcha: "The token `stagehandFlagUsages` is the func's template-namespace name. The registration and
           the two replacer targets MUST agree or cobra errors at render time. One sed keeps them lockstep."

# Read-only refs (do NOT edit)
- file: internal/cmd/root_test.go
  why: "READ-ONLY. TestHelp_FlagsWrappedWithinWidth (:428) runs Execute([\"--help\"]) which renders the
        usage template — the functional proof. TestRoot_* (:221/:258/:303/:332/:359) exercise Execute but
        may not render usage; that's why the gate must include TestHelp."

# External references
- url: https://pkg.go.dev/github.com/spf13/cobra#Command.AddTemplateFunc
  why: "Confirms AddTemplateFunc(name string, func interface{}) registers a template func under the STRING
        `name`, resolved by text/template at render time. A name referenced in the template but not
        registered (or vice versa) is a render-time error. This is WHY registration + replacer targets
        must rename in lockstep."
- url: https://pkg.go.dev/strings#NewReplacer
  why: "Confirms NewReplacer(old,new) replaces EVERY occurrence of `old` with `new` in the input string.
        The replacer's `old` (.LocalFlags.FlagUsages ) is cobra's default token — UNCHANGED; only `new`
        (which contains the template-func name) changes."
```

### Current Codebase Tree (relevant slice — module/dir rename DONE in T1; S1 parallel)

```bash
stagecoach/                      # module github.com/dustin/stagehand... → already renamed (go.mod: stagecoach)
├── cmd/stagecoach/main.go       # dir renamed (T1.S2)
├── go.mod                       # module github.com/dustin/stagecoach (T1.S1)
└── internal/cmd/
    └── root.go                  # EDIT TARGET — 5 occurrences of stagehandFlagUsages (the ONLY file)
# (STAGEHAND_ env vars + stagehand.* git-config keys in root.go flag-help text = P1.M2.T1, NOT this task)
```

### Desired Codebase Tree After This Subtask

```bash
stagecoach/
└── (only one existing file modified — no new files)
    internal/cmd/root.go         # stagehandFlagUsages → stagecoachFlagUsages (5 sites)
```

| Path | Action | Responsibility |
|---|---|---|
| `internal/cmd/root.go` | MODIFY | Rename the cobra template-func string token `stagehandFlagUsages` → `stagecoachFlagUsages` at all 5 sites (1 registration + 2 replacer targets + 2 comments) via one sed. |

**Explicitly NOT touched**: the Go function `flagUsagesWrapped` (no "stagehand" in its name), the
replacer's search side (`.LocalFlags.FlagUsages ` / `.InheritedFlags.FlagUsages ` — cobra's default
token), `STAGEHAND_*` env-var literals + `stagehand.*` git-config keys in root.go's flag-help text
(P1.M2.T1), user-facing strings / `.stagehandignore` raw literals (P1.M2.T2/T3), any other file (the
token is root.go-only), any tests (none reference the token by name), any docs, `PRD.md`, `tasks.json`,
`prd_snapshot.md`, `plan/*`.

### Known Gotchas of our Codebase & toolchain

```go
// CRITICAL (G1 — registration + replacer targets MUST rename in lockstep): cobra resolves the template-func
// NAME at render time via the AddTemplateFunc map. If the registration becomes "stagecoachFlagUsages" but a
// replacer target still emits "stagehandFlagUsages .LocalFlags ", the rendered template references an
// UNDEFINED function → text/template errors at --help time. One sed over the file guarantees all three code
// sites (registration + 2 targets) + the 2 comments change together. Do NOT hand-edit one site at a time.

// CRITICAL (G2 — do NOT change the replacer's SEARCH side): strings.NewReplacer(".LocalFlags.FlagUsages ",
// "stagecoachFlagUsages .LocalFlags ", ...). The FIRST argument (.LocalFlags.FlagUsages ) is cobra's DEFAULT
// template substring the replacer searches FOR — it must stay so the swap still finds cobra's default. Only
// the SECOND argument (the replacement, which contains the func name) changes. The sed pattern
// 'stagehandFlagUsages' does not match '.FlagUsages ' (no 'stagehand' prefix), so the search side is safe.

// GOTCHA (G3 — the token is a STRING, not a Go identifier): AddTemplateFunc("stagehandFlagUsages", fn)
// registers under a string key; the template references {{stagehandFlagUsages .Arg}}. This is why S1 (Go
// identifier rename) didn't cover it — it's a string literal. Renaming the string + the template references
// is the complete change. The Go function `flagUsagesWrapped` (the VALUE) is UNCHANGED.

// GOTCHA (G4 — the token is root.go-ONLY): grep -rn 'stagehandFlagUsages' --include='*.go' . (excl. plan/)
// returns 5 hits, all in internal/cmd/root.go. No second registration, no test references it by name, no
// docs. Editing any other file is scope creep.

// GOTCHA (G5 — the contract's `-run TestRoot` gate is too narrow): TestRoot_* tests exercise Execute but
// do NOT render the usage template. TestHelp_FlagsWrappedWithinWidth (root_test.go:428, runs
// Execute(["--help"])) is the ONLY test that renders the template and would catch a registration/token
// mismatch. Broaden the gate to `-run 'TestRoot|TestHelp'` (or run the whole ./internal/cmd/ suite), else
// a broken template passes the narrow gate and fails only at real --help time.

// GOTCHA (G6 — do NOT rename STAGEHAND_ env vars or stagehand.* git-config keys here): root.go's flag-help
// text is full of "env STAGEHAND_..." and "git stagehand.*" literals. Those are P1.M2.T1's territory. The
// sed 'stagehandFlagUsages' will NOT touch them (different token). Leave them for M2.T1.

// GOTCHA (G7 — gofmt is a no-op here): the rename is same-length-ish token substitution within existing
// string literals/comments; gofmt -l should report nothing. Run it anyway as the gate.
```

## Implementation Blueprint

### Data models and structure

None. Pure string-literal rename of a cobra template-func namespace name. No type, signature, or
identifier change.

### The single edit (exact)

**The sed** (run from repo root):
```bash
sed -i 's/stagehandFlagUsages/stagecoachFlagUsages/g' internal/cmd/root.go
```

**Effect on each of the 5 sites** (current → target):

| Line | Current | Target |
|---|---|---|
| 240 (comment) | `...the stagehandFlagUsages template func (registered` | `...the stagecoachFlagUsages template func (registered` |
| 246 (registration) | `cobra.AddTemplateFunc("stagehandFlagUsages", flagUsagesWrapped)` | `cobra.AddTemplateFunc("stagecoachFlagUsages", flagUsagesWrapped)` |
| 248 (replacer target) | `".LocalFlags.FlagUsages ", "stagehandFlagUsages .LocalFlags ",` | `".LocalFlags.FlagUsages ", "stagecoachFlagUsages .LocalFlags ",` |
| 249 (replacer target) | `".InheritedFlags.FlagUsages ", "stagehandFlagUsages .InheritedFlags ",` | `".InheritedFlags.FlagUsages ", "stagecoachFlagUsages .InheritedFlags ",` |
| 253 (comment) | `// flagUsagesWrapped is the stagehandFlagUsages template func: ...` | `// flagUsagesWrapped is the stagecoachFlagUsages template func: ...` |

**Unchanged (verified):** the replacer's search side (`.LocalFlags.FlagUsages ` / `.InheritedFlags.FlagUsages `),
the Go function `flagUsagesWrapped` (line 258), and `pflag.FlagUsagesWrapped(...)`.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: RENAME the token in internal/cmd/root.go
  - FILE: internal/cmd/root.go
  - RUN: sed -i 's/stagehandFlagUsages/stagecoachFlagUsages/g' internal/cmd/root.go
  - VERIFY (token gone): grep -n 'stagehandFlagUsages' internal/cmd/root.go → NO output.
  - VERIFY (new token present, 5 sites): grep -c 'stagecoachFlagUsages' internal/cmd/root.go → 5.
  - VERIFY (search side intact): grep -n '.LocalFlags.FlagUsages \|\.InheritedFlags.FlagUsages '
        internal/cmd/root.go → the two replacer old-sides still present (the swap still works).
  - VERIFY (Go func intact): grep -n 'func flagUsagesWrapped' internal/cmd/root.go → 1 match, unchanged.
  - DO NOT: edit any other file; rename flagUsagesWrapped; touch the STAGEHAND_ env / stagehand.* config literals.

Task 2: VALIDATE (build + vet + the help-render test)
  - RUN: gofmt -l internal/cmd/root.go        # Expected: empty (token substitution; no formatting shift)
  - RUN: go build ./...                        # Expected: exit 0
  - RUN: go vet ./...                          # Expected: exit 0
  - RUN: go test ./internal/cmd/ -run 'TestRoot|TestHelp' -count=1 -v
        # Expected: ALL green. TestHelp_FlagsWrappedWithinWidth renders --help → proves the template func
        # resolves under the new name (a registration/token mismatch would fail Execute here).
  - RUN (broader regression): go test ./internal/cmd/ -count=1   # the whole cmd suite (optional but cheap)
  - RUN (full repo, optional): go test ./...                     # nothing else depends on the token
  - FIX-FORWARD: a TestHelp failure with "function stagecoachFlagUsages not defined" = a missed site
        (impossible after the sed, but re-grep if it occurs); a compile failure = a typo.
```

### Implementation Patterns & Key Details

```go
// === Why the registration name and the template token must agree ===
// cobra.Command.AddTemplateFunc(name, fn) stores fn in an internal templateFuncs map under the STRING name.
// At render time, text/template resolves {{name .Arg}} by looking up that map. If the template emits
// {{stagecoachFlagUsages .LocalFlags}} but the registration is still "stagehandFlagUsages", the lookup
// fails → template render error → Execute(--help) returns err. The strings.NewReplacer that swaps cobra's
// default ".FlagUsages " token → "<name> .FlagUsages " is what makes the rendered template call our func,
// so its NEW-value side MUST use the same name as the registration. One sed keeps them lockstep.

// === Why the sed pattern is safe (token boundary) ===
// The pattern 'stagehandFlagUsages' is a fully-qualified token. It does NOT match:
//   - ".LocalFlags.FlagUsages " / ".InheritedFlags.FlagUsages " (cobra's default — the replacer SEARCH side;
//     no "stagehand" prefix → preserved, so the swap still finds cobra's default template substring).
//   - "flagUsagesWrapped" (the Go function VALUE passed to AddTemplateFunc; no "stagehand" prefix).
//   - "pflag.FlagUsagesWrapped(...)" (the pflag method).
// So the sed touches exactly the 5 intended tokens.

// === Why TestHelp_FlagsWrappedWithinWidth is the functional gate ===
// It calls Execute([]string{"--help"}), which triggers cobra's usage-template render, which calls the
// stagecoachFlagUsages func via the replacer-swapped template. If the rename is inconsistent, the render
// errors and Execute returns a non-nil err → the test's `t.Fatalf("Execute(--help) err=%v", err)` fires.
// The TestRoot_* tests exercise Execute on real subcommands but typically don't render the usage template,
// so they cannot catch this — hence broaden -run to include TestHelp.
```

### Integration Points

```yaml
PRODUCTION (1 file):
  - internal/cmd/root.go: stagehandFlagUsages → stagecoachFlagUsages (5 sites: 1 registration + 2 replacer targets + 2 comments)

UNCHANGED (explicitly):
  - the Go function flagUsagesWrapped (the registered VALUE; no "stagehand" in its name)
  - the replacer's search side (.LocalFlags.FlagUsages / .InheritedFlags.FlagUsages — cobra's default token)

NO-TOUCH (explicitly — owned by sibling subtasks):
  - STAGEHAND_* env-var literals + stagehand.* git-config keys in root.go flag-help text  # P1.M2.T1
  - user-facing strings / .stagehandignore raw literals in root.go/verbose.go             # P1.M2.T2/T3
  - any other file (the token is root.go-only)
  - tests (none reference the token by name), docs, Makefile/.goreleaser/CI                # M3/M4
  - PRD.md, tasks.json, prd_snapshot.md, plan/*

GATE: go build ./... → OK; go vet ./... clean; go test ./internal/cmd/ -run 'TestRoot|TestHelp' green
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagecoach   # (or the repo root; module is github.com/dustin/stagecoach)

gofmt -l internal/cmd/root.go   # Expected: empty (token substitution within strings/comments)
go vet ./internal/cmd/...        # Expected: exit 0
go build ./...                   # Expected: exit 0 (a string-literal rename compiles trivially)

# Expected: Zero errors.
```

### Level 2: The Functional Gate (help rendering proves the rename)

```bash
cd /home/dustin/projects/stagecoach

# THE proof: TestHelp_FlagsWrappedWithinWidth runs Execute(["--help"]) → renders the usage template →
# resolves the stagecoachFlagUsages func. A registration/token mismatch fails here.
go test ./internal/cmd/ -run 'TestHelp' -count=1 -v
# Expected: PASS (TestHelp_FlagsWrappedWithinWidth + any other TestHelp_*).

# The contract's TestRoot gate (unchanged behavior on the run paths):
go test ./internal/cmd/ -run 'TestRoot' -count=1 -v
# Expected: ALL TestRoot_* pass.

# Combined (the recommended single gate):
go test ./internal/cmd/ -run 'TestRoot|TestHelp' -count=1
# Expected: green. NOTE: the contract said `-run TestRoot` only; broaden to include TestHelp (gotcha G5).
```

### Level 3: Scope Discipline (only root.go changed; token gone)

```bash
cd /home/dustin/projects/stagecoach

# The old token is GONE from all production .go files.
grep -rn 'stagehandFlagUsages' --include='*.go' . | grep -v './plan/'
# Expected: NO output.

# The new token is present exactly 5× in root.go and NOWHERE else.
grep -rn 'stagecoachFlagUsages' --include='*.go' . | grep -v './plan/'
# Expected: 5 matches, all in internal/cmd/root.go.

# Only root.go changed.
git diff --stat -- internal/ pkg/ cmd/ docs/
# Expected: internal/cmd/root.go only.

# The replacer search side + the Go func are intact (the swap still works).
grep -n '\.LocalFlags\.FlagUsages \|\.InheritedFlags\.FlagUsages ' internal/cmd/root.go   # → 2 (the old sides)
grep -n 'func flagUsagesWrapped' internal/cmd/root.go                                       # → 1 (unchanged)
```

### Level 4: Help-Render Smoke (end-to-end, beyond the unit test)

```bash
cd /home/dustin/projects/stagecoach

# Build the binary and render --help directly — the most direct proof that cobra resolves the renamed func.
go build -o /tmp/stagecoach-help-check ./cmd/stagecoach
/tmp/stagecoach-help-check --help > /tmp/help.txt 2>&1; echo "exit=$?"
# Expected: exit=0; /tmp/help.txt contains the wrapped flag-usage block (Global Flags / Flags), no
# "function ... not defined" template error. (This is exactly what TestHelp_FlagsWrappedWithinWidth asserts;
# the smoke run is belt-and-suspenders.)
rm -f /tmp/stagecoach-help-check /tmp/help.txt

# Cross-check: the contract's headline — "cobra help rendering works correctly with the new template
# function name" — is satisfied by exit=0 + non-empty, well-formed help output above.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` exits 0.
- [ ] `go vet ./...` exits 0.
- [ ] `gofmt -l .` reports nothing.
- [ ] `go test ./internal/cmd/ -run 'TestRoot|TestHelp' -count=1` green (incl. `TestHelp_FlagsWrappedWithinWidth`).

### Feature Validation
- [ ] `internal/cmd/root.go` has ZERO `stagehandFlagUsages` and exactly 5 `stagecoachFlagUsages`.
- [ ] `cobra.AddTemplateFunc("stagecoachFlagUsages", flagUsagesWrapped)` (registration name renamed).
- [ ] The two replacer targets emit `stagecoachFlagUsages .LocalFlags ` / `stagecoachFlagUsages .InheritedFlags `.
- [ ] The replacer search side (`.LocalFlags.FlagUsages ` / `.InheritedFlags.FlagUsages `) is UNCHANGED.
- [ ] The Go function `flagUsagesWrapped` is UNCHANGED.
- [ ] `--help` renders without a template error (Level 4 smoke / TestHelp).

### Scope Discipline Validation
- [ ] ONLY `internal/cmd/root.go` modified (`git diff --stat` confirms).
- [ ] Did NOT rename `flagUsagesWrapped` (the function value) or the replacer search side.
- [ ] Did NOT rename `STAGEHAND_*` env vars / `stagehand.*` git-config keys (P1.M2.T1) or user-facing strings (P1.M2.T3).
- [ ] Did NOT modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/` (except this PRP + research note).

### Code Quality Validation
- [ ] The rename is lockstep (registration + 2 targets + 2 comments all `stagecoachFlagUsages`).
- [ ] The sed pattern is token-precise (no collateral matches).
- [ ] Help output is byte-identical to pre-rename (only the internal template-func name changed).

---

## Anti-Patterns to Avoid

- ❌ Don't hand-edit one site at a time — the registration and the two replacer targets MUST change together
  or cobra errors at `--help` time. Use the single sed (gotcha G1).
- ❌ Don't change the replacer's SEARCH side (`.LocalFlags.FlagUsages ` / `.InheritedFlags.FlagUsages `) —
  that's cobra's default template substring the swap hunts for; renaming it breaks the swap. Only the
  replacement (new-value) side changes. The sed won't touch it (gotcha G2).
- ❌ Don't rename the Go function `flagUsagesWrapped` — it's the VALUE passed to AddTemplateFunc, not the
  NAME; it has no "stagehand" in it. Leave it (gotcha G3).
- ❌ Don't use the contract's `-run TestRoot` gate alone — it doesn't render the usage template.
  `TestHelp_FlagsWrappedWithinWidth` (runs `--help`) is the only test that catches a mismatch. Broaden to
  `-run 'TestRoot|TestHelp'` (gotcha G5).
- ❌ Don't edit any file other than `internal/cmd/root.go` — the token is root.go-only (gotcha G4).
- ❌ Don't rename `STAGEHAND_*` env-var literals or `stagehand.*` git-config keys in root.go's flag-help
  text — those are P1.M2.T1's. The sed token won't touch them (gotcha G6).
- ❌ Don't modify `PRD.md`, `tasks.json`, `prd_snapshot.md`, or anything under `plan/`.

---

## Confidence Score

**9.5/10** for one-pass implementation success.

Rationale: This is a single-file, single-token rename of a cobra template-func string literal — the sed
pattern `stagehandFlagUsages` is fully-qualified and proven not to match cobra's default `.FlagUsages `
substring (the replacer's search side) or the `flagUsagesWrapped` function. All 5 occurrences are
grep-confirmed to be the exact token and to live in `internal/cmd/root.go` alone (no second site, no test
references the token by name, no docs). S1 explicitly defers this token to S2 (gotcha G5 of S1's PRP), so
there is no conflict with the parallel identifier rename. The functional gate is unusually strong:
`TestHelp_FlagsWrappedWithinWidth` renders `--help` end-to-end, so a registration/template-token mismatch
fails the test deterministically (and the Level 4 smoke runs the binary's `--help` directly). The one
substantive correction — broaden the contract's `-run TestRoot` filter to include `TestHelp` — is front-
loaded as gotcha G5 and the Level 2 gate. The residual 0.5 uncertainty is purely whether `go test ./...`
picks up any test elsewhere that (very indirectly) renders help (the e2e lock_scenarios_test references
help/usage patterns) — running the full `./internal/cmd/` suite plus `--help` smoke covers it. The M2/M3/M4
boundaries (env vars, config paths, user strings, build/docs) are cleanly fenced.
