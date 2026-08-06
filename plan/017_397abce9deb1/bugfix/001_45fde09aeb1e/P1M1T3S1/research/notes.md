# P1.M1.T3.S1 — Research notes (BUG-003: semver-safe prerelease tag selection)

## 0. Task shape

Surgical bug-fix in ONE method of ONE file + ONE regression test + a Mode-A doc-comment update.
`internal/upgrade/releases.go::LatestAdmittingPrereleases` lets a leading non-semver tag (e.g.
"nightly") win over a valid semver tag because `upgrade.Compare` returns 0 for unparseable operands.
Fix: gate each candidate through `ParseAndClean` so a parseable tag always beats an unparseable one.
**Do NOT change version.go** (Compare's 0-for-unparseable is a deliberate dev-build defense for
`--check` — BUG-003 is fixed at the SELECTION site, not in the comparator).

## 1. The buggy code (internal/upgrade/releases.go) — EXACT, current

Doc comment (lines 210–216), function (line 217), selection loop (lines 226–232):
```go
// LatestAdmittingPrereleases resolves the highest-precedence non-draft release from the full
// /releases array (the --prerelease channel, FR-U5 step 2). Drafts are always excluded (they require
// auth and are not real releases). Prereleases are admitted and ordered by the same-package
// upgrade.Compare (version.go:105) — which is full semver precedence, prerelease-aware (§11.4), so a
// v2.0.0-rc1 correctly outranks a stable v1.9.0. An empty array or an all-drafts array yields
// ErrNoReleases. Compare returns 0 for unparseable operands (dev-build defense), so garbage tags
// simply tie and never cause an error here.
func (c *Client) LatestAdmittingPrereleases(ctx context.Context) (Release, error) {
	body, err := c.do(ctx, "/repos/"+c.Repo+"/releases")
	if err != nil {
		return Release{}, err
	}
	var rs []ghRelease
	if err := json.Unmarshal(body, &rs); err != nil {
		return Release{}, fmt.Errorf("decode releases: %v: %w", err, ErrHTTP)
	}
	best := -1
	for i, r := range rs {
		if r.Draft {
			continue // drafts always excluded (require auth; not real releases).
		}
		if best < 0 || Compare(r.TagName, rs[best].TagName) > 0 {   // ← BUG-003: line 231
			best = i
		}
	}
	if best < 0 {
		return Release{}, fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)
	}
	return rs[best].toRelease(), nil
}
```
- **The bug (line 231):** `Compare(valid, garbage)` returns 0 (garbage unparseable) → never `> 0` →
  once `best` is set to a leading garbage tag, no valid tag can displace it. The doc comment's last two
  lines ("garbage tags simply tie and never cause an error here") describe the CURRENT (buggy) behavior
  and become INACCURATE after the fix → they must be rewritten (Mode A).

## 2. Same-package primitives to reuse (internal/upgrade/version.go — DO NOT EDIT)

- `func ParseAndClean(raw string) (string, bool)` (version.go:52) — normalizer; `ok=false` for
  "nightly"/"latest"/"dev"/malformed. The bool is what the fix keys on.
- `func Compare(a, b string) int` (version.go:112) — semver precedence (prerelease-aware, §11.4);
  returns 0 if EITHER operand is unparseable (the dev-build defense for `--check` that CAUSES BUG-003
  in the selection loop). The fix DEPRIORITIZES unparseable tags at the call site rather than touching
  Compare, so `--check`'s defense is preserved.

## 3. The fix — verified against every case

Replace the loop body so each candidate is checked for parseability, and track `okBest` alongside
`best`. Advance when: (a) `best < 0` (first non-draft, even if unparseable — graceful); OR
(b) `okCandidate && !okBest` (parseable beats unparseable best); OR (c) `okCandidate && okBest &&
Compare(candidate, best) > 0` (strict precedence among parseable tags).

**Case analysis (all verified):**
| Array (non-draft, in order) | Trace | Result | OK? |
|---|---|---|---|
| `[v3.0.0-DRAFT, v1.9.0, v2.0.0-rc1]` (existing PicksHighest) | draft skip; best=1(ok); Compare(rc1,1.9)>0→best=2 | v2.0.0-rc1 | ✓ existing test still passes |
| `[nightly, v1.5.0]` (BUG-003 repro, new) | best=0(unok); okC&&!okB→best=1 | v1.5.0 | ✓ FIXED |
| `[v1.5.0, nightly]` (order-independence, new) | best=0(ok); nightly: !okC→no advance | v1.5.0 | ✓ |
| `[nightly, latest]` (all-unparseable) | best=0(unok); latest: !okC→no advance | nightly | ✓ graceful, no ErrNoReleases |
| `[]` / all-drafts | best stays -1 | ErrNoReleases | ✓ contract unchanged |

**Critical correctness detail:** the condition is `best < 0 || (okCandidate && !okBest) || (okCandidate
&& okBest && Compare(...))`. Go's `||` short-circuits LEFT→RIGHT, so when `best < 0` the later clauses
(including `rs[best]` and `Compare`) are NOT evaluated → no out-of-bounds `rs[-1]` access, no panic.
When `best >= 0`, `rs[best]` is always valid. The `best < 0` guard is load-bearing — keep it FIRST.

## 4. Test patterns to mirror (internal/upgrade/releases_test.go)

- `newFakeClient(t, handler)` (line 15) — `httptest.NewServer` + `Client{BaseURL: ts.URL, Repo:"owner/repo"}`, auto-closed via `t.Cleanup`.
- `statusServer(status, body)` (line 53) — handler that writes status + body verbatim.
- `TestClient_Prerelease_PicksHighest` (line 119) — the EXACT shape to mirror: `newFakeClient(t, statusServer(http.StatusOK, body))`; `c.LatestAdmittingPrereleases(context.Background())`; assert `rel.Tag`.
- `TestClient_Prerelease_AllDrafts_NoReleases` (line 139) — inline body literal pattern (multi-line backtick string with the GitHub fields). Mirror for the new test bodies.
- **Imports (lines 3-10):** context, errors, net/http, httptest, strings, sync, testing — ALL already present. The new test needs NO new import (no filepath/strconv needed). No import gotcha here (unlike sibling T2.S1's `path/filepath`).

The new test asserts the contract's exact body: `[{nightly, prerelease:true, draft:false, assets:[]},{v1.5.0, prerelease:true, draft:false, assets:[{name:"x"}]}]` → `Release{Tag:"v1.5.0"}`, plus a reversed-order sub-case (order-independence). No `errors.Is` needed (happy-path tag assertion), but the empty/all-drafts/ErrNoReleases cases already exist (lines 131, 139) and must stay green.

## 5. Scope fences (confirmed vs siblings)

- **T2.S1 (BUG-002)** edits `internal/upgrade/detect.go` — DIFFERENT file; no conflict. (Read its PRP:
  it touches detectPath's go-install block + detect_test.go; never releases.go.)
- **T1.S1 (BUG-001)** edits `internal/upgrade/stage.go` (sanityCheck) — DIFFERENT file; Complete; no conflict.
- **This task** edits ONLY `internal/upgrade/releases.go` (loop + doc comment) + `internal/upgrade/releases_test.go` (one new test func). NOT version.go, NOT detect.go, NOT stage.go, NOT any CLI/config/API surface. Mode A = the inline doc comment only.