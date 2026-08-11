name: "P1.M1.T1.S3 — load.go: extend validateFormat to the <base>[+body] grammar"
description: >
  Extend internal/config/load.go's validateFormat to accept each of the 4 format bases WITH exactly one
  trailing "+body" suffix (FR-F1/FR-F9): strip an optional case-sensitive "+body", validate the resulting
  BASE against the existing validFormats set (same loop, same 4 bases — the suffix is grammar, not a mode),
  and update the error message to surface the grammar. Rejects +body alone, <base>+body+body, <unknown>+body,
  <base>+Body (case mismatch), and any other suffix. forceBody is DISCARDED here (validation only checks
  base — the prompt layer owns body-forcing). CRITICAL: splitFormat is UNEXPORTED in package prompt and
  config does NOT import prompt — INLINE the 2-line split in validateFormat (do NOT export/import). Pure
  function, called once at Load()'s tail. validFormats + the existing TestValidateFormat stay green; S3
  EXTENDS the test with +body cases. No signature/config/user-text changes (S4 owns user-facing text).

---

## Goal

**Feature Goal**: Make `validateFormat` (the load-time gate, FR-F1: "Anything outside the `<base>[+body]`
grammar is a hard configuration error") accept the `+body` suffix on every valid base, so a resolved
`cfg.Format` like `conventional+body` or `auto+body` passes load and reaches the prompt layer (S1/S2),
while every malformed variant is rejected with a grammar-aware error.

**Deliverable**: (1) `validateFormat` in `internal/config/load.go` rewritten to strip an optional
case-sensitive `+body` and validate the BASE against the unchanged `validFormats` set; (2) updated error
message surfacing the `<base>[+body]` grammar; (3) updated doc comment; (4) `TestValidateFormat` in
`load_test.go` EXTENDED with the `+body` valid + invalid cases. `validFormats` unchanged (4 bases).

**Success Definition**:
- `validateFormat` returns nil for `auto`, `conventional`, `gitmoji`, `plain` AND each with exactly one
  `+body` (`auto+body`, `conventional+body`, `gitmoji+body`, `plain+body`).
- It returns an error for: `+body` alone, `bogus+body`, `conventional+body+body`, `conventional+Body`
  (case mismatch), and every previously-invalid value (`""`, `emoji`, `Conventional`, `AUTO`, …).
- The error message names the grammar and still contains the substring `auto, conventional, gitmoji, plain`
  (so the existing `TestValidateFormat` assertion passes UNCHANGED).
- `validFormats` stays the 4 bases; `validateFormat` stays PURE; its signature + call site (`Load()` tail)
  are unchanged.
- Existing `TestValidateFormat` passes unchanged; the extended cases pass.
- `go build ./...`, `go vet ./...`, `gofmt -l internal/config/`, `go test ./internal/config/...`,
  `make test`, `make lint` pass.

## User Persona (if applicable)

**Target User**: Stagecoach users setting `--format` / `stagecoach_FORMAT` / `stagecoach.format` /
`[generation].format` (FR-F1). Internal function (the error string is the only user-facing surface;
S4 owns the doc/flag/help text).

**Use Case**: A user runs `stagecoach --format conventional+body` to force a subject-plus-body message
(FR-F9). Today load rejects it (`"conventional+body"` is not in the 4-base set); after S3 it passes load
and the prompt layer (S1/S2) emits the body-forcing directive.

**Pain Points Addressed**: FR-F1 — the `+body` suffix must be a valid, accepted grammar, not a load error.

## Why

- **FR-F1 (P1)**: the format grammar is `<base>[+body]`. S1 (splitFormat + prompt assembly) and S2
  (system.go threading) make the prompt layer honor `+body`; S3 is the load-time gate that lets a
  `+body` value THROUGH config resolution instead of rejecting it. Without S3, `--format conventional+body`
  fails at `Load()` before any prompt is built — S1/S2 are unreachable for `+body`.
- **Defense at the right layer**: validation runs ONCE on the fully-resolved `cfg.Format` (not per-layer),
  so a typo in any layer (file/env/flag) is caught with a clear, grammar-aware message. Locale is
  deliberately NOT validated (FR-F6 — free-form).
- **Why inline, not import**: `splitFormat` is unexported in package `prompt`; `config` is a foundational
  layer with no `internal/*` imports. Inlining the 2-line split keeps config decoupled (see Known Gotchas).

## What

**User-visible behavior**: `--format <base>+body` is accepted at load (was: hard error). Malformed
variants get a clearer error: `invalid format "x" (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)`.

**Technical change (one function body + doc comment + test extension):**
- `validateFormat`: strip optional `+body` → validate base against `validFormats` → grammar-aware message.
- `TestValidateFormat`: add the 4 `+body` valid cases + 4 new invalid cases.

### Success Criteria
- [ ] `validateFormat` accepts the 4 bases + each with exactly one `+body`
- [ ] `validateFormat` rejects `+body`, `<unknown>+body`, `<base>+body+body`, `<base>+Body`, and the existing invalid set
- [ ] Error message contains `auto, conventional, gitmoji, plain` (existing assertion holds) + names the grammar
- [ ] `validFormats` unchanged (4 bases); `validateFormat` pure; signature + call site unchanged
- [ ] Existing `TestValidateFormat` passes unchanged; extended `+body` cases pass
- [ ] `go build ./...`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current function body (line-verified), the inline-vs-import decision with reasoning,
the full edge-case truth table, the existing test's assertions (and why they still hold), the message
substring that MUST be preserved, and the scope fences are all below.

### Documentation & References

```yaml
- file: internal/config/load.go
  why: "THE file. validFormats(571) + validateFormat(577-584) + the Load() tail call site(208)."
  pattern: "KEEP the validFormats slice (4 bases) and the for-loop membership test; insert a 2-line
            optional-'+body' strip BEFORE the loop; update the error format string. Stay PURE (no I/O)."
  gotcha: "validateFormat is in package config; splitFormat (internal/prompt) is UNEXPORTED and config
           does NOT import prompt. INLINE the strip — do NOT call/import splitFormat."

- file: internal/config/load_test.go
  why: "TestValidateFormat(1355) — the existing validation test. Its assertions: valid modes → nil;
        invalid modes → err whose Error() contains the mode AND the substring 'auto, conventional,
        gitmoji, plain'. S3 EXTENDS it (the +body cases); the existing rows stay green unchanged."
  pattern: "Mirror the existing table-driven shape (validModes / invalidModes sub-tests). Add the 4
            +body valid rows + the 4 new invalid rows. The comprehensive cross-package +body suite is
            P1.M2.T1.S1 — S3 owns only load_test.go."

- file: internal/prompt/format.go (S1 — READ-ONLY reference for the grammar semantics)
  why: "splitFormat(58) defines the canonical grammar split (HasSuffix/TrimSuffix on '+body', case-
        sensitive). S3's inline strip MUST be behavior-identical (same stdlib calls) so validation and
        the prompt layer agree on what '+body' means."
  pattern: "splitFormat('+body')→('',true); splitFormat('conventional+body+body')→('conventional+body',true);
            splitFormat('Conventional+Body')→('Conventional+Body',false). The inline strip reproduces
            these exactly (the base ends up not in validFormats ⇒ error for all three)."

- docfile: plan/021_086bbc57a2b2/architecture/format_subsystem.md
  why: "The subsystem map: validFormats/validateFormat location + purity + the single-call-site tail
        of Load(); confirms the +body threading map (S1 parser → S2 system.go → S3 load.go validation)."
  section: "internal/config/load.go; +body threading map"

- docfile: plan/021_086bbc57a2b2/P1M1T1S3/research/findings.md
  why: "Verified the cross-package constraint (§2), the current function body (§3), the edge-case truth
        table (§5), and the existing test's preserved-substring assertion (§6)."
```

### Current Codebase tree (relevant slice)

```bash
internal/config/
  load.go        # THE file: validateFormat(577) body + doc comment + error message. validFormats(571) UNCHANGED.
  load_test.go   # EXTEND TestValidateFormat(1355) with +body cases
internal/prompt/format.go  # S1 (splitFormat) — READ-ONLY reference for grammar semantics; UNCHANGED in S3
```

### Desired Codebase tree with files to be added

```bash
internal/config/load.go        # MODIFY: validateFormat body (strip +body → validate base) + doc comment + message
internal/config/load_test.go   # MODIFY (additive): extend TestValidateFormat with +body valid + invalid rows
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (cross-package): splitFormat is UNEXPORTED in package prompt (internal/prompt/format.go:58).
//   validateFormat is in package config (internal/config/load.go). config imports ZERO internal/* packages
//   today (only stdlib + pflag). You CANNOT call splitFormat from config. INLINE the 2-line strip:
//     base := format
//     if strings.HasSuffix(format, "+body") { base = strings.TrimSuffix(format, "+body") }
//   This is behavior-identical to splitFormat (same stdlib calls, same case-sensitivity). Do NOT export
//   splitFormat (would churn S1's just-landed function) and do NOT import internal/prompt in config
//   (inverted layering for 2 lines of trivial string logic). forceBody is UNUSED in validation.

// CRITICAL (preserve the error substring): the existing TestValidateFormat asserts the error contains
//   "auto, conventional, gitmoji, plain" verbatim. The new message "valid: <base>[+body], base ∈ %s" with
//   strings.Join(validFormats, ", ") yields "...base ∈ auto, conventional, gitmoji, plain" — the substring
//   IS present. Do NOT reorder/rename the bases or drop the Join. The offending value stays %q=format
//   (so the "contains mode" assertion also holds).

// GOTCHA (<base>+body+body rejects itself for free): HasSuffix strips ONE "+body" ⇒ base becomes
//   "<base>+body" ⇒ not in validFormats ⇒ error. No special-case for doubled suffixes. Case-sensitivity
//   is free (strings.HasSuffix is case-sensitive) ⇒ "+Body"/"+BODY" don't strip ⇒ un-stripped base fails
//   the membership test. "+body" alone ⇒ base="" ⇒ fails.

// GOTCHA (validateFormat stays PURE): no I/O, no git, directly unit-testable. Called ONCE at Load()'s
//   tail (load.go:208) on the FULLY RESOLVED cfg.Format (a low-layer typo overridden higher is NOT an
//   error — only the resolved value is validated). Locale is NOT validated (FR-F6).

// GOTCHA (forceBody is not validation's concern): validateFormat checks the BASE only. Whether +body is
//   present (forceBody) is the prompt layer's concern (S1/S2). Discard it: base, _ := <split>.

// SCOPE: S3 is load.go (validateFormat) + load_test.go (TestValidateFormat) ONLY. Do NOT touch validFormats
//   (stays 4 bases), validateFormat's signature/call-site, system.go (S2), format.go/planner.go (S1),
//   config.go/bootstrap.go/root.go user-facing text (S4). Do NOT add the comprehensive +body suite
//   (P1.M2.T1.S1). Do NOT export splitFormat or add an internal/prompt import.
```

## Implementation Blueprint

### Data models and structure

No struct/type/signature/config changes. One function body rewritten (same signature), one test extended.
No new imports (`strings` already in load.go; load_test.go already imports `strings`).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REWRITE validateFormat in internal/config/load.go (SAME signature, grammar-aware)
  - KEEP: func validateFormat(format string) error { ... } and the validFormats slice (L571, 4 bases).
  - NEW BODY (inline the split — do NOT call splitFormat; see Known Gotchas):
        func validateFormat(format string) error {
            // FR-F1 <base>[+body] grammar: strip an optional case-sensitive "+body" suffix, then validate
            // the BASE. The suffix is grammar, not a mode, so validFormats stays the 4 bases. forceBody is
            // discarded here — the prompt layer (S1/S2) owns body-forcing; validation only gates the base.
            // NOTE: splitFormat (internal/prompt) is the canonical split but is unexported and lives in a
            // package config does not import; this inline strip is behavior-identical (same stdlib calls).
            base := format
            if strings.HasSuffix(format, "+body") {
                base = strings.TrimSuffix(format, "+body")
            }
            for _, m := range validFormats {
                if base == m {
                    return nil
                }
            }
            return fmt.Errorf("invalid format %q (valid: <base>[+body], base ∈ %s)", format, strings.Join(validFormats, ", "))
        }
  - UPDATE the doc comment (L573-area): note the <base>[+body] grammar — valid iff base ∈ validFormats
    with an optional single "+body"; "+body" alone, "<base>+body+body", "<base>+Body", "<unknown>+body"
    are hard errors; forceBody is ignored (prompt-layer concern); PURE; called once at Load() tail.
  - DEPENDENCIES: S1's splitFormat concept (landed — informs the exact strip semantics; not a code dependency).

Task 2: EXTEND TestValidateFormat in internal/config/load_test.go
  - KEEP the existing validModes (4 bare bases) + invalidModes (6 existing) rows UNCHANGED — they still pass.
  - ADD to validModes: "auto+body", "conventional+body", "gitmoji+body", "plain+body" (each → nil).
  - ADD to invalidModes: "+body", "bogus+body", "conventional+body+body", "conventional+Body" (each → error).
    The existing invalid-row assertions (err != nil AND err.Error() contains the mode AND contains
    "auto, conventional, gitmoji, plain") apply to the new rows too — verify they hold:
      - "+body" → err contains `"+body"` ✓ and the valid-set substring ✓.
      - "conventional+Body" → err contains `"conventional+Body"` ✓ and the substring ✓.
  - (Optional) add a focused sub-test asserting the NEW valid +body rows return nil (the table already does
    this if you add them to validModes). The comprehensive cross-package +body suite is P1.M2.T1.S1.
  - DEPENDENCIES: Task 1.

Task 3: VERIFY — gates
  - go build ./...
  - go vet ./...
  - gofmt -l internal/config/
  - go test ./internal/config/ -run TestValidateFormat -v   # existing rows green + new rows green
  - go test ./internal/config/...                            # whole config package
  - make test && make lint
```

### Implementation Patterns & Key Details

```go
// PATTERN: the grammar-aware validateFormat (inline split — behavior-identical to prompt.splitFormat)
func validateFormat(format string) error {
	base := format
	if strings.HasSuffix(format, "+body") {   // case-sensitive; "+Body"/"+BODY" do NOT strip
		base = strings.TrimSuffix(format, "+body")
	}
	for _, m := range validFormats {
		if base == m {
			return nil
		}
	}
	// %q = the ORIGINAL format (so the offending value is named); %s = the 4 bases (preserves the substring
	// "auto, conventional, gitmoji, plain" the existing test asserts).
	return fmt.Errorf("invalid format %q (valid: <base>[+body], base ∈ %s)", format, strings.Join(validFormats, ", "))
}

// PATTERN: the elegant rejections (no special-casing)
//   "conventional+body+body" → strip ONE "+body" → base "conventional+body" → not in set → ERROR
//   "Conventional+Body"      → "+Body" ≠ "+body" → no strip → base "Conventional+Body" → not in set → ERROR
//   "+body"                  → strip → base "" → not in set → ERROR
```

### Integration Points

```yaml
NO struct/signature/config/public-API/user-text changes. One function body + one test extension.

CODE:
  - internal/config/load.go — validateFormat body + doc comment + error message
TESTS:
  - internal/config/load_test.go — extend TestValidateFormat (+body valid + invalid rows)

CONSUMED (concept, not code):
  - splitFormat grammar (internal/prompt/format.go, S1) — informs the exact strip semantics (inlined, not imported)

UNCHANGED:
  - validFormats slice (4 bases); validateFormat signature + purity + Load() call site (load.go:208)
  - system.go (S2), format.go/planner.go (S1), config.go/bootstrap.go/root.go user-facing text (S4)

DOWNSTREAM (do NOT implement in S3):
  - P1.M1.T1.S4: user-facing config/flag/help text (config.go Format doc, bootstrap.go template, root.go --format help)
  - P1.M2.T1.S1: comprehensive +body suite (format_test.go assembly + splitFormat, system_test.go auto+body/conventional+body, load_test.go validation)
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...                       # validateFormat rewrites cleanly; no new imports
go vet ./...
gofmt -l internal/config/            # Expected: empty.
make lint                            # Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The validation gate (existing rows green + new +body rows green)
go test ./internal/config/ -run TestValidateFormat -v
# Expected: PASS — 4 bare bases + 4 +body bases → nil; 6 existing invalid + 4 new invalid → error w/ grammar message.

# Whole config package
go test ./internal/config/... -v
# Expected: ALL pass (the validateFormat change is behavior-preserving for the pre-existing set).

# Whole suite
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# validateFormat is PURE and called once at Load()'s tail — the unit test IS the integration proof.
# (The end-to-end +body path — `--format conventional+body` → load passes → prompt emits bodyForceDirective
#  → subject+body message — is gated by S1 (assembly) + S2 (system.go threading) landing, and is validated
#  by P1.M2.T1.S1's cross-package suite. S3's within-scope proof is TestValidateFormat.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: validateFormat strips +body and validates base (NOT a raw exact-match anymore)
grep -n 'HasSuffix(format, "+body")\|TrimSuffix(format, "+body")\|base == m' internal/config/load.go
# Expected: the strip + the base-membership test present.

# Grep guard: the error message surfaces the grammar AND preserves the valid-set substring
grep -n 'valid: <base>\[+body\]' internal/config/load.go
# Expected: 1 match. (The substring "auto, conventional, gitmoji, plain" comes from strings.Join(validFormats).)

# Grep guard: validFormats UNCHANGED (4 bases — the suffix is grammar, not a mode)
grep -n 'validFormats = ' internal/config/load.go
# Expected: `var validFormats = []string{"auto", "conventional", "gitmoji", "plain"}` (unchanged).

# Grep guard: NO internal/prompt import added (config stays decoupled — inline split, not import)
grep -n 'stagecoach/internal/prompt' internal/config/load.go
# Expected: 0 matches.

# Grep guard: validateFormat signature + purity unchanged (still takes string, returns error, no I/O)
grep -n 'func validateFormat(format string) error' internal/config/load.go
# Expected: 1 match (signature unchanged).

# Scope-boundary guard: only load.go + load_test.go changed
git diff --name-only
# Expected: only internal/config/load.go + internal/config/load_test.go (plus whatever S1/S2 changed in
#   their files — S3's own diff is config-only).

# Behavior-preservation guard: the existing TestValidateFormat rows pass UNCHANGED
go test ./internal/config/ -run 'TestValidateFormat/valid_' -v   # 4 bare bases
go test ./internal/config/ -run 'TestValidateFormat/invalid_' -v # 6 existing invalid + 4 new
# Expected: ALL pass.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/config/` empty
- [ ] `make lint` zero errors
- [ ] `make test` all pass

### Feature Validation
- [ ] `validateFormat` accepts the 4 bases + each with exactly one `+body`
- [ ] `validateFormat` rejects `+body`, `<unknown>+body`, `<base>+body+body`, `<base>+Body`, and the existing invalid set
- [ ] Error message contains `auto, conventional, gitmoji, plain` (existing assertion holds) + names the `<base>[+body]` grammar
- [ ] `forceBody` discarded (validation checks base only)
- [ ] Extended `TestValidateFormat` rows pass

### Scope-Boundary Validation
- [ ] `validFormats` UNCHANGED (4 bases)
- [ ] `validateFormat` signature + purity + Load() call site unchanged
- [ ] NO `internal/prompt` import / NO exported `splitFormat` (inline split)
- [ ] system.go (S2), format.go/planner.go (S1) UNCHANGED
- [ ] config.go/bootstrap.go/root.go user-facing text UNCHANGED (S4)
- [ ] Only load.go + load_test.go changed

### Code Quality
- [ ] Doc comment cites FR-F1 `<base>[+body]` + the rejection cases + purity + the inline-vs-import rationale
- [ ] Inline split is behavior-identical to `prompt.splitFormat` (same stdlib calls, case-sensitive)
- [ ] Error message preserves the valid-set substring (existing test green)

---

## Anti-Patterns to Avoid

- ❌ Don't call `splitFormat` from `validateFormat` — it's UNEXPORTED in package `prompt`, and `config`
  doesn't import `prompt`. Inline the 2-line `HasSuffix`/`TrimSuffix` split. Don't export `splitFormat`
  (churns S1) and don't add a `config → prompt` import (inverted layering for trivial logic).
- ❌ Don't add `+body` variants to `validFormats` — the suffix is grammar, not a mode. The slice stays the
  4 bases; the strip handles the suffix. (Adding 8 entries would also break the "valid set" the error cites.)
- ❌ Don't drop or reorder `auto, conventional, gitmoji, plain` in the error — the existing `TestValidateFormat`
  asserts that substring verbatim. Use `strings.Join(validFormats, ", ")` so it's auto-maintained.
- ❌ Don't lowercase the suffix or use `EqualFold` — `+body` is case-sensitive (FR-F1). `strings.HasSuffix`
  is already case-sensitive; `+Body`/`+BODY` must NOT strip (they reject via the un-stripped base failing
  membership).
- ❌ Don't special-case `<base>+body+body` — stripping ONE `+body` leaves base `"<base>+body"` which fails
  membership naturally. A special-case is dead code.
- ❌ Don't use `forceBody` in validation — it's the prompt layer's concern (S1/S2). Validate the BASE only.
- ❌ Don't change the signature, the `Load()` call site (load.go:208), or the purity (no I/O).
- ❌ Don't touch user-facing text (config.go/bootstrap.go/root.go) — that's S4. The error string is the
  only user-facing surface S3 owns.
- ❌ Don't add the comprehensive cross-package `+body` suite — that's P1.M2.T1.S1. S3 extends only
  `TestValidateFormat` in load_test.go.

---

## Confidence Score: 9/10

One-pass success is very high: a ~8-line rewrite of a pure function (inline split → base membership →
grammar-aware message), an explicit edge-case truth table verified against the inline logic, and a
behavior-preservation proof for the existing test (the `auto, conventional, gitmoji, plain` substring is
preserved; all 6 existing invalid modes still error; the 4 bare bases still pass). The one non-obvious
decision — inline the split rather than call/import `splitFormat` — is spelled out with the layering
rationale (config is foundational, prompt is higher-level; the 2-line split isn't worth an inverted
cross-package dependency or churning S1's just-landed unexported function). The -1 is for the error-message
wording: the exact phrasing (`<base>[+body], base ∈ …`) is a suggestion mirroring the item's — an
implementer might phrase it slightly differently; the HARD constraint (preserve the
`auto, conventional, gitmoji, plain` substring so the existing assertion holds) is called out explicitly,
so any reasonable wording passes. No new imports, no concurrency, no integration step — the unit test is
the proof.