name: "P1.M1.T3.S1 — Make LatestAdmittingPrereleases selection semver-safe + add a regression test (BUG-003)"
description: >
  Fix BUG-003 (Minor): internal/upgrade/releases.go::LatestAdmittingPrereleases selects the highest
  non-draft release via `Compare(candidate, best) > 0`, but upgrade.Compare deliberately returns 0 for
  an unparseable operand (dev-build defense for --check). So a leading non-semver tag (e.g. "nightly")
  sets `best` first and every later valid tag fails `Compare(valid, garbage) > 0` (returns 0) → `best`
  stays stuck on the garbage tag. Fix at the SELECTION site (do NOT change version.go/Compare): gate
  each non-draft candidate through same-package ParseAndClean so a parseable tag ALWAYS beats an
  unparseable one, regardless of array order; among parseable tags, keep strict Compare precedence.
  When all non-draft tags are unparseable, return the first non-draft (graceful — no ErrNoReleases).
  Empty/all-drafts → ErrNoReleases (unchanged). Add TestClient_Prerelease_NonSemverDoesNotWin (two
  orderings). Update the method's doc comment (Mode A). Only releases.go + releases_test.go touched.

---

## Goal

**Feature Goal**: Make the `--prerelease` channel resolver pick the highest-precedence PARSEABLE
non-draft release — never a non-semver ("moving") tag — so a maintainer publishing a `nightly`/`latest`
tag alongside semver releases cannot cause `stagecoach upgrade --prerelease` to select the moving tag.

**Deliverable**: (1) a rewrite of the selection loop in
`internal/upgrade/releases.go::LatestAdmittingPrereleases` (lines 226–232) that gates each candidate
through `ParseAndClean` and tracks the best's parseability; (2) a Mode-A update of the method's doc
comment (lines 210–216 — the current "garbage tags simply tie" sentence becomes false post-fix and must
be rewritten); (3) one new regression test `TestClient_Prerelease_NonSemverDoesNotWin` (two sub-cases:
non-semver-first and non-semver-last) in `internal/upgrade/releases_test.go`, mirroring the existing
`newFakeClient`/`statusServer` pattern. **No change to version.go, detect.go, stage.go, or any
CLI/config/API surface.**

**Success Definition**:
- `[nightly, v1.5.0]` (nightly first) → `LatestAdmittingPrereleases` returns `Release{Tag:"v1.5.0"}`
  (the BUG-003 fix — was returning "nightly").
- `[v1.5.0, nightly]` (order-independence) → still returns `Release{Tag:"v1.5.0"}`.
- `[v3.0.0-DRAFT, v1.9.0, v2.0.0-rc1]` → still returns `v2.0.0-rc1` (existing
  `TestClient_Prerelease_PicksHighest` passes unchanged).
- All-unparseable non-drafts (e.g. `[nightly, latest]`) → returns the first non-draft, NO error
  (graceful; consistent with the prior all-garbage behavior).
- Empty array / all-drafts → `errors.Is(err, ErrNoReleases)` (contract unchanged).
- `go build ./...`, `go test ./internal/upgrade/ -count=1`, `make test`, `make lint` all pass.
- No change to `version.go` (Compare's 0-for-unparseable dev-build defense for `--check` is preserved).

## User Persona (if applicable)

**Target User**: A developer tracking pre-release builds via `stagecoach upgrade --prerelease` against a
repo that also publishes a moving `nightly`/`latest` tag.

**Use Case**: `stagecoach upgrade --prerelease` resolves the latest admitting-prereleases release to
target. After this fix, a moving tag alongside semver releases no longer hijacks the selection.

**Pain Points Addressed**: BUG-003 — the `--prerelease` tag selection could pick a leading non-semver
tag over a valid release. (Downstream impact was contained — SelectAsset then derived an asset name
from the garbage tag that matched no real asset, failing safely with `ErrNoMatchingAsset` — but the
selection logic was incorrect.)

## Why

- **BUG-003 (Minor) / PRD bug_analysis §h2.5 recommendation**: "in LatestAdmittingPrereleases, treat
  Compare==0 against the current best (or an unparseable best) as non-winning … prefer a parseable tag
  over an unparseable one." This task implements that recommendation at the call site.
- **Do NOT touch Compare**: `upgrade.Compare` returns 0 for an unparseable operand *deliberately* — it
  is a documented dev-build defense so `--check` (which calls `Compare(latestTag, currentTag)`) never
  falsely signals an update for a `dev` build (version.go). The bug is that the prerelease SELECTION
  loop reused Compare as a strict-greater test, which the 0-for-unparseable contract breaks. Fixing it
  at the selection site (gate through ParseAndClean) preserves `--check`'s defense.
- **Bounded, surgical**: one loop in one method + one doc comment + one test. Reuses the same-package
  `ParseAndClean`/`Compare` primitives and the existing httptest test harness. Sibling BUG-001 (stage.go)
  and BUG-002 (detect.go) are different files.

## What

**User-visible behavior**: `stagecoach upgrade --prerelease` resolves to the highest semver-precedence
non-draft release, ignoring non-semver ("moving") tags regardless of their position in the releases
list. No new flags/config/API.

**Technical change**:
1. `releases.go` LatestAdmittingPrereleases selection loop (226–232): track `okBest bool` alongside
   `best int`; for each non-draft candidate compute `_, okCandidate := ParseAndClean(r.TagName)`;
   advance when `best < 0 || (okCandidate && !okBest) || (okCandidate && okBest && Compare(r.TagName,
   rs[best].TagName) > 0)`.
2. `releases.go` doc comment (210–216): replace the now-false "garbage tags simply tie" sentence with a
   statement that non-semver tags are deprioritized (a parseable tag always beats an unparseable one).
3. `releases_test.go`: add `TestClient_Prerelease_NonSemverDoesNotWin` with two sub-cases.

### Success Criteria
- [ ] `[nightly, v1.5.0]` → `Release{Tag:"v1.5.0"}`
- [ ] `[v1.5.0, nightly]` → `Release{Tag:"v1.5.0"}` (order-independence)
- [ ] Existing `TestClient_Prerelease_PicksHighest` (→ `v2.0.0-rc1`) still passes
- [ ] All-unparseable non-drafts → first non-draft, no error (graceful)
- [ ] Empty / all-drafts → `errors.Is(err, ErrNoReleases)` (unchanged)
- [ ] `version.go` byte-unchanged (Compare's dev-build defense preserved)
- [ ] `go build ./...`, `go test ./internal/upgrade/ -count=1`, `make test`, `make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact current buggy loop (verbatim, with line numbers), the exact same-package primitives
to reuse (`ParseAndClean(raw) (string, bool)` @version.go:52, `Compare(a, b) int` @version.go:112), the
exact replacement loop, a verified case analysis, the exact test to mirror (`TestClient_Prerelease_PicksHighest`
@line 119 via `newFakeClient`/`statusServer`), the verbatim new test, the doc-comment sentence that
becomes false, and the scope fences against the sibling bugs (different files) are all enumerated below.

### Documentation & References

```yaml
# MUST READ — the file under the fix
- file: internal/upgrade/releases.go
  why: "THE change site. LatestAdmittingPrereleases doc comment @210-216; function @217; selection
        loop @226-232; buggy line @231 (`if best < 0 || Compare(r.TagName, rs[best].TagName) > 0`)."
  pattern: "the loop: `best := -1; for i, r := range rs { if r.Draft { continue }; if best < 0 ||
            Compare(r.TagName, rs[best].TagName) > 0 { best = i } }`"
  critical: "the doc comment's LAST TWO LINES ('Compare returns 0 for unparseable operands (dev-build
             defense), so garbage tags simply tie and never cause an error here.') describe the CURRENT
             buggy behavior and become FALSE after the fix — rewrite them (Mode A). Keep the rest of the
             comment (drafts-excluded, prerelease-aware Compare, ErrNoReleases for empty/all-drafts)."

# MUST READ — the test file to extend (mirror its patterns verbatim)
- file: internal/upgrade/releases_test.go
  why: "newFakeClient(t, handler) @15; statusServer(status, body) @53; TestClient_Prerelease_PicksHighest
        @119 (the EXACT shape to mirror — happy-path tag assertion, no errors.Is);
        TestClient_Prerelease_AllDrafts_NoReleases @139 (inline backtick body literal pattern)."
  pattern: >
    c := newFakeClient(t, statusServer(http.StatusOK, body))
    rel, err := c.LatestAdmittingPrereleases(context.Background())
    if err != nil { t.Fatalf(...) }
    if rel.Tag != "v1.5.0" { t.Errorf(...) }
  critical: "imports (context, errors, net/http, httptest, strings, sync, testing) are ALL already
             present — the new test adds NO import. (No filepath/strconv gotcha here.)"

# MUST READ — the same-package primitives to reuse (DO NOT EDIT this file)
- file: internal/upgrade/version.go
  why: "ParseAndClean(raw string) (string, bool) @52 — returns ok=false for 'nightly'/'latest'/'dev'/
        malformed; the bool is what the fix keys on. Compare(a, b string) int @112 — semver precedence,
        returns 0 if EITHER operand is unparseable (the dev-build defense for --check)."
  critical: "DO NOT change Compare's 0-for-unparseable behavior — it is a deliberate --check defense
             (version.go doc). BUG-003 is fixed at the SELECTION site (releases.go), not in Compare."

# DECISION PROVENANCE
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/architecture/bug_analysis.md
  why: "§h2.3/h3.2 BUG-003 is the authoritative analysis (location releases.go:231, root cause =
        Compare 0-for-unparseable, the [nightly,v1.5.0] probe). §h2.5 gives the fix recommendation:
        'prefer a parseable tag over an unparseable one.' This task implements it."
  section: "BUG-003 (MINOR) — --prerelease tag selection picks a leading non-semver tag"

# SIBLING CONTRACTS (different files — confirm non-overlap)
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T2S1/PRP.md
  why: "BUG-002 edits internal/upgrade/detect.go (detectPath go-install block) — DIFFERENT file, no
        conflict. Read to confirm this task's releases.go edit cannot collide with it."
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T1S1/PRP.md
  why: "BUG-001 edits internal/upgrade/stage.go (sanityCheck) — DIFFERENT file, Complete. No conflict."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go       # THE fix: selection loop @226-232 + doc comment @210-216 (Mode A)
  releases_test.go  # ADD: TestClient_Prerelease_NonSemverDoesNotWin (two sub-cases)
  version.go        # REUSE ONLY (ParseAndClean @52, Compare @112) — DO NOT EDIT (dev-build defense)
  detect.go         # BUG-002 sibling (T2.S1) — NOT touched here
  stage.go          # BUG-001 sibling (T1.S1, Complete) — NOT touched here
```

### Desired Codebase tree with files to be added

```bash
internal/upgrade/releases.go       # MODIFY: selection loop (ParseAndClean gate + okBest) + doc comment (Mode A)
internal/upgrade/releases_test.go  # MODIFY: +TestClient_Prerelease_NonSemverDoesNotWin (two t.Run sub-cases)
# (no new files; no CLI/config/API/version.go/detect.go/stage.go change)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (Go || short-circuits — keep `best < 0` FIRST): the new condition is
//   `best < 0 || (okCandidate && !okBest) || (okCandidate && okBest && Compare(r.TagName, rs[best].TagName) > 0)`.
//   Go evaluates || left→right and short-circuits, so when best<0 the later clauses (which reference
//   rs[best] and call Compare) are NOT evaluated → no rs[-1] out-of-bounds panic. When best>=0,
//   rs[best] is always valid. If you reorder the clauses (e.g. put the Compare check before `best<0`),
//   you WILL panic on the first non-draft iteration. Keep `best < 0` as the first disjunct.

// CRITICAL (track okBest on EVERY advance): you must add `okBest bool` and set `okBest = okCandidate`
//   inside the same `if` that sets `best = i`. Forgetting to update okBest leaves it stale (false) and
//   re-introduces the bug (a parseable best would be treated as unparseable). okBest is the parseability
//   of rs[best], recomputed each time best changes.

// CRITICAL (do NOT change version.go): Compare's 0-for-unparseable is a deliberate dev-build defense
//   for --check (version.go doc). BUG-003 is fixed by gating selection through ParseAndClean, NOT by
//   making Compare stricter. Editing version.go would break the --check dev-build invariant and is
//   out of scope (and out of the forbidden list — no source edits outside the two named files).

// GOTCHA (the doc comment's last sentence becomes FALSE): the current comment ends "Compare returns 0
//   for unparseable operands (dev-build defense), so garbage tags simply tie and never cause an error
//   here." After the fix, garbage tags no longer "simply tie" — they are DEPRIORITIZED. Rewrite that
//   sentence; leaving it creates a comment that lies about the code (the exact failure Mode A exists
//   to prevent). Keep the accurate parts (drafts excluded; prerelease-aware Compare; ErrNoReleases for
//   empty/all-drafts).

// GOTCHA (graceful all-unparseable is intentional): when every non-draft tag is unparseable, best must
//   remain the FIRST non-draft (not -1, not ErrNoReleases). This matches the prior all-garbage behavior
//   and the contract ("no ErrNoReleases since entries exist"). Do NOT add an "skip unparseable entirely"
//   branch that could drive best to -1 on an all-garbage array — that would change ErrNoReleases's
//   meaning (ErrNoReleases is reserved for empty/all-DRAFTS, not all-garbage).

// SCOPE: edit ONLY releases.go (loop + doc comment) + releases_test.go (one test). Do NOT touch
//   version.go, detect.go, stage.go. Do NOT add a CLI flag / config key / env var. Mode A = the inline
//   doc comment only (no user-facing docs file). Do NOT "also fix" SelectAsset or asset-name derivation
//   (that's a different concern; the selection fix is the whole deliverable).
```

## Implementation Blueprint

### Data models and structure
None. No struct/type change. One control-flow loop rewrite in `LatestAdmittingPrereleases` reusing the
same-package `ParseAndClean`/`Compare` primitives.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: MODIFY internal/upgrade/releases.go — rewrite the selection loop (226-232)
  - LOCATE the loop (best := -1 @226; buggy line @231).
  - REPLACE the loop with the parseability-gated version:
        best := -1
        okBest := false // parseability of rs[best]; advanced in lockstep with best.
        for i, r := range rs {
            if r.Draft {
                continue // drafts always excluded (require auth; not real releases).
            }
            _, okCandidate := ParseAndClean(r.TagName) // same-package (version.go:52)
            // Advance best when: (a) first non-draft; OR (b) a parseable candidate beats an unparseable
            // best; OR (c) both parseable and the candidate has strict semver precedence. This
            // deprioritizes non-semver tags (e.g. "nightly", "latest") so a moving tag can never win
            // over a valid release regardless of array order (BUG-003). The `best < 0` disjunct MUST stay
            // first — Go's || short-circuits, so rs[best]/Compare are only reached when best>=0.
            if best < 0 || (okCandidate && !okBest) || (okCandidate && okBest && Compare(r.TagName, rs[best].TagName) > 0) {
                best = i
                okBest = okCandidate
            }
        }
  - KEEP UNCHANGED: the surrounding decode (json.Unmarshal), the `if best < 0 { return ... ErrNoReleases }`
    tail, and the `return rs[best].toRelease(), nil`.
  - VERIFY (by reasoning) the case analysis in the research notes: PicksHighest→rc1, [nightly,v1.5.0]→
    v1.5.0, [v1.5.0,nightly]→v1.5.0, all-garbage→first, empty/all-drafts→ErrNoReleases.
  - DEPENDENCIES: none (ParseAndClean/Compare already in package).

Task 2: MODIFY internal/upgrade/releases.go — update the doc comment (210-216, Mode A)
  - REWRITE the last two lines ("Compare returns 0 for unparseable operands (dev-build defense), so
    garbage tags simply tie and never cause an error here.") to reflect the fix, e.g.:
        // Non-semver tags (e.g. "nightly", "latest", a moving tag) are DEPRIORITIZED: a parseable tag
        // always wins over an unparseable one, regardless of array order. upgrade.Compare returns 0 for
        // unparseable operands (a deliberate dev-build defense for --check), so the selection loop gates
        // each candidate through ParseAndClean rather than relying on Compare alone — this keeps a leading
        // garbage tag from winning over a valid release (BUG-003). When every non-draft tag is
        // unparseable, the first non-draft is returned (graceful — no ErrNoReleases since entries exist).
  - KEEP the accurate earlier lines (drafts excluded; prerelease-aware Compare; v2.0.0-rc1 outranks
    v1.9.0; empty/all-drafts ⇒ ErrNoReleases).
  - DEPENDENCIES: Task 1 (the comment must match the code).

Task 3: MODIFY internal/upgrade/releases_test.go — add TestClient_Prerelease_NonSemverDoesNotWin
  - PLACE near TestClient_Prerelease_PicksHighest (line 119) / the other Prerelease_* tests.
  - ADD (mirror the existing happy-path shape; two t.Run sub-cases):
        func TestClient_Prerelease_NonSemverDoesNotWin(t *testing.T) {
            // BUG-003 regression: a non-semver tag must NOT win over a valid semver tag. Compare returns
            // 0 for unparseable operands, so the loop gates each candidate through ParseAndClean — a
            // parseable tag always beats an unparseable one.
            // Case A: non-semver PRECEDES the valid tag (the original bug — garbage was selected first).
            t.Run("nonsemver_first", func(t *testing.T) {
                body := `[
                    {"tag_name": "nightly", "prerelease": true, "draft": false, "assets": []},
                    {"tag_name": "v1.5.0", "prerelease": true, "draft": false,
                     "assets": [{"name": "x", "browser_download_url": "u", "size": 1}]}
                ]`
                c := newFakeClient(t, statusServer(http.StatusOK, body))
                rel, err := c.LatestAdmittingPrereleases(context.Background())
                if err != nil {
                    t.Fatalf("LatestAdmittingPrereleases: unexpected error: %v", err)
                }
                if rel.Tag != "v1.5.0" {
                    t.Errorf("Tag = %q, want v1.5.0 (parseable beats unparseable — BUG-003)", rel.Tag)
                }
            })
            // Case B: valid tag PRECEDES the non-semver tag (order-independence — the fix holds both ways).
            t.Run("nonsemver_last", func(t *testing.T) {
                body := `[
                    {"tag_name": "v1.5.0", "prerelease": true, "draft": false,
                     "assets": [{"name": "x", "browser_download_url": "u", "size": 1}]},
                    {"tag_name": "nightly", "prerelease": true, "draft": false, "assets": []}
                ]`
                c := newFakeClient(t, statusServer(http.StatusOK, body))
                rel, err := c.LatestAdmittingPrereleases(context.Background())
                if err != nil {
                    t.Fatalf("LatestAdmittingPrereleases: unexpected error: %v", err)
                }
                if rel.Tag != "v1.5.0" {
                    t.Errorf("Tag = %q, want v1.5.0 (order-independence)", rel.Tag)
                }
            })
        }
  - NO new import (context/net/http/httptest already present).
  - DEPENDENCIES: Tasks 1-2.

Task 4: VERIFY build + vet + format + targeted tests + full upgrade suite + no-regression
  - go build ./...
  - go vet ./internal/upgrade/...
  - gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go   # must list nothing
  - go test ./internal/upgrade/ -run 'TestClient_Prerelease' -v                 # new + existing prerelease tests
  - go test ./internal/upgrade/ -count=1                                       # the item's exact command
  - make test && make lint
  - git diff --stat -- internal/upgrade/version.go internal/upgrade/detect.go internal/upgrade/stage.go   # EMPTY
  - REGRESSION-CHECK (by reasoning): pre-fix, TestClient_Prerelease_NonSemverDoesNotWin/nonsemver_first
    would FAIL (returned "nightly"); post-fix it PASSES. (Optional empirical: temporarily restore the
    buggy `if best < 0 || Compare(...) > 0` line, re-run, observe FAIL, then restore the fix.)
```

### Implementation Patterns & Key Details

```go
// PATTERN — the fixed selection loop (gate through ParseAndClean; track okBest; keep `best<0` FIRST)
best := -1
okBest := false
for i, r := range rs {
	if r.Draft {
		continue
	}
	_, okCandidate := ParseAndClean(r.TagName) // version.go:52 — ok=false for "nightly"/"latest"/"dev"
	// `best < 0` MUST be first: Go short-circuits ||, so rs[best]/Compare are reached only when best>=0.
	if best < 0 || (okCandidate && !okBest) || (okCandidate && okBest && Compare(r.TagName, rs[best].TagName) > 0) {
		best = i
		okBest = okCandidate // advanced in lockstep with best
	}
}

// PATTERN — the regression test (mirror TestClient_Prerelease_PicksHighest; two orderings)
c := newFakeClient(t, statusServer(http.StatusOK, body))
rel, err := c.LatestAdmittingPrereleases(context.Background())
if err != nil { t.Fatalf(...) }
if rel.Tag != "v1.5.0" { t.Errorf("Tag = %q, want v1.5.0", rel.Tag) }
```

### Integration Points

```yaml
NO CLI / config / API / struct / docs-surface change. One control-flow loop + a doc comment + one test.

CODE:
  - internal/upgrade/releases.go LatestAdmittingPrereleases loop (226-232) — ParseAndClean gate + okBest
  - internal/upgrade/releases.go doc comment (210-216) — rewrite the false "garbage tags simply tie" line (Mode A)
TEST:
  - internal/upgrade/releases_test.go — +TestClient_Prerelease_NonSemverDoesNotWin (2 sub-cases)

CONSUMED (read-only, reused):
  - upgrade.ParseAndClean (version.go:52) — the parseability gate
  - upgrade.Compare (version.go:112) — strict precedence among parseable tags (0-for-unparseable UNCHANGED)

UNCHANGED (do NOT touch): version.go (Compare dev-build defense); detect.go (BUG-002); stage.go
  (BUG-001); LatestStable / ReleaseByTag; the do() error contract; any CLI/config/API surface.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./internal/upgrade/...
gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go   # Expected: nothing listed
make lint                                                                  # Expected: zero errors
# If gofmt -l lists a file, run `gofmt -w` on it. If vet/lint errors, read + fix.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The prerelease tests: the new regression + the existing PicksHighest/Empty/AllDrafts (all must pass)
go test ./internal/upgrade/ -run 'TestClient_Prerelease' -v
# Expected:
#   TestClient_Prerelease_NonSemverDoesNotWin/nonsemver_first  PASS  (BUG-003 fix)
#   TestClient_Prerelease_NonSemverDoesNotWin/nonsemver_last   PASS  (order-independence)
#   TestClient_Prerelease_PicksHighest                         PASS  (v2.0.0-rc1, unchanged)
#   TestClient_Prerelease_Empty_NoReleases                    PASS  (unchanged)
#   TestClient_Prerelease_AllDrafts_NoReleases                PASS  (unchanged)

# The item's EXACT command — the whole upgrade package
go test ./internal/upgrade/ -count=1
# Expected: ALL pass.

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# LatestAdmittingPrereleases is a pure selection over a decoded array; the httptest unit test IS the
# authoritative gate. (The end-to-end --prerelease path is exercised by the P1.M4.T3 e2e harness, not
# this subtask.) Optional: confirm the fix at the binary level with a one-off fake server:
go build -o /tmp/sc ./cmd/stagecoach
# (Simulating a moving-tag repo requires a live fake; the unit test covers the selection deterministically.)
```

### Level 4: Creative & Domain-Specific Validation

```bash
# Grep guard: the fix is present (ParseAndClean gate + okBest in the loop)
grep -n 'okBest\|ParseAndClean(r.TagName)' internal/upgrade/releases.go
# Expected: the loop references both (okBest declared + assigned; ParseAndClean called per candidate).

# Grep guard: the buggy raw-Compare condition is GONE from the selection loop
! grep -n 'best < 0 || Compare(r.TagName' internal/upgrade/releases.go && echo "OK: buggy line replaced"
# Expected: the bare `if best < 0 || Compare(r.TagName, rs[best].TagName) > 0` line is gone.

# Grep guard: the doc comment no longer claims "garbage tags simply tie"
! grep -n 'garbage tags simply tie' internal/upgrade/releases.go && echo "OK: stale comment rewritten"
# Expected: the now-false sentence is gone (replaced by the deprioritization note).

# Grep guard: version.go byte-unchanged (Compare dev-build defense preserved)
git diff --stat -- internal/upgrade/version.go internal/upgrade/detect.go internal/upgrade/stage.go
# Expected: empty (no sibling-file edits).

# Grep guard: the new test exists with both sub-cases
grep -n 'TestClient_Prerelease_NonSemverDoesNotWin\|nonsemver_first\|nonsemver_last' internal/upgrade/releases_test.go
# Expected: the func + both t.Run names present.

# Scope guard: only releases.go + releases_test.go changed by this subtask
git diff --stat -- internal/upgrade/
# Expected: exactly releases.go + releases_test.go (no version.go/detect.go/stage.go).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/ -count=1` passes (item's exact command); `make test` passes

### Feature Validation
- [ ] `[nightly, v1.5.0]` → `Release{Tag:"v1.5.0"}` (BUG-003 fix)
- [ ] `[v1.5.0, nightly]` → `Release{Tag:"v1.5.0"}` (order-independence)
- [ ] `TestClient_Prerelease_PicksHighest` still passes (`v2.0.0-rc1`, unchanged)
- [ ] All-unparseable non-drafts → first non-draft, no error (graceful)
- [ ] Empty / all-drafts → `errors.Is(err, ErrNoReleases)` (unchanged)

### Scope-Boundary Validation
- [ ] `version.go` byte-unchanged (Compare's dev-build defense preserved)
- [ ] NO change to detect.go (BUG-002) or stage.go (BUG-001)
- [ ] NO CLI/config/API/user-facing-docs change (Mode A = inline doc comment only)
- [ ] Only releases.go (loop + doc comment) + releases_test.go (one test) touched

### Code Quality
- [ ] The `best < 0` disjunct stays FIRST (Go `||` short-circuit → no `rs[-1]` panic)
- [ ] `okBest` advanced in lockstep with `best` on every advance
- [ ] Doc comment rewritten (the false "garbage tags simply tie" sentence removed; deprioritization noted)
- [ ] Test mirrors the existing `newFakeClient`/`statusServer` happy-path shape; both orderings covered

---

## Anti-Patterns to Avoid

- ❌ **Don't change `version.go` / `Compare`.** Its 0-for-unparseable return is a deliberate dev-build defense for `--check` (version.go doc). BUG-003 is fixed at the SELECTION site (gate through `ParseAndClean`), not by making the comparator stricter. Editing Compare would break the `--check` dev-build invariant.
- ❌ **Don't reorder the loop condition.** `best < 0` MUST be the first disjunct — Go's `||` short-circuits, so `rs[best]`/`Compare` are only evaluated when `best >= 0`. Putting a `Compare(r.TagName, rs[best].TagName)` check before `best < 0` panics on the first non-draft iteration (`rs[-1]`).
- ❌ **Don't forget to update `okBest` on every advance.** `okBest = okCandidate` must live inside the same `if` that does `best = i`. A stale `okBest` re-introduces the bug (a parseable best would be treated as unparseable and could be displaced by another parseable tag only via Compare — or worse, an unparseable best would never be displaced).
- ❌ **Don't leave the doc comment's "garbage tags simply tie" sentence.** It describes the buggy behavior and is FALSE after the fix. A comment that lies about the code is the exact failure Mode A exists to prevent — rewrite it to state the deprioritization.
- ❌ **Don't add an "skip unparseable tags entirely" branch.** The contract requires all-unparseable input to return the FIRST non-draft gracefully (no `ErrNoReleases`). Skipping unparseables could drive `best` to -1 on an all-garbage array, changing `ErrNoReleases`'s meaning (reserved for empty/all-DRAFTS).
- ❌ **Don't assert on `errors.Is` in the new happy-path test.** The `[nightly, v1.5.0]` case is a success path (returns `v1.5.0`, no error) — mirror `TestClient_Prerelease_PicksHighest` (tag assertion), not the error cases.
- ❌ **Don't touch `detect.go`/`stage.go`** — BUG-002/BUG-001 are sibling subtasks in different files.
- ❌ **Don't write a test that passes on the pre-fix code.** `TestClient_Prerelease_NonSemverDoesNotWin/nonsemver_first` MUST fail without the fix (returned "nightly"). Verify by reasoning (or a temporary revert) that it is a real regression guard.
- ❌ **Don't add a CLI flag / config key / env var** — Mode A is the inline doc comment; no user-facing surface changes.

---

## Confidence Score

**10/10** — one-pass success likelihood. This is a single control-flow loop rewrite in one method, with
the exact current buggy code quoted (line 231), the exact replacement loop specified, the same-package
primitives named (`ParseAndClean` @52, `Compare` @112), a verified case analysis covering every input
class (existing tests + both new orderings + all-garbage graceful + empty/all-drafts ErrNoReleases), the
exact test to mirror (`TestClient_Prerelease_PicksHighest` @119), the verbatim new test, the doc-comment
sentence identified as becoming-false, and explicit scope fences (no version.go/detect.go/stage.go; no
CLI/config/API). The only conceivable failure modes — reordering the condition (panic), forgetting to
update okBest, leaving the stale comment, or touching Compare — are each guarded by a CRITICAL gotcha +
a Level-4 grep check.