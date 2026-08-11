name: "P1.M3.T3.S1 — Validate + segment-escape c.Repo in GitHub releases request paths (BUG-008)"
description: >
  A SMALL, surgical defense-in-depth fix for BUG-008 (PRD §h3.7 "Issue 5"). internal/upgrade/releases.go
  builds GitHub API request paths by interpolating `c.Repo` (the config-sourced "owner/repo" string,
  default "dabstractor/stagecoach") UNESCAPED at 3 sites — LatestStable (~179), ReleaseByTag (~198, where
  the TAG is already url.PathEscape'd but c.Repo is not), and LatestAdmittingPrereleases (~223). A
  malformed c.Repo (a space, `?`, `#`, control char, or extra `/` from a mistyped `source_repo`) would
  build a malformed HTTP request line or silently target the wrong resource. CRITICAL: `url.PathEscape
  (c.Repo)` is the WRONG fix — it escapes `/` → `%2F`, breaking "owner/repo" into one segment (GitHub
  404s). The correct fix is SPLIT-AND-ESCAPE: split c.Repo on `/`, VALIDATE each segment is a non-empty
  run of GitHub's owner/repo charset `[a-zA-Z0-9._-]`, then escape EACH segment individually. Validation
  is the primary gate (a malformed config → a clear error, NO HTTP call); per-segment escaping is
  defense-in-depth (all valid-repo chars are unreserved, so escaping is a no-op for valid input and only
  matters if validation were ever bypassed). DELIVERABLE (2 files, stdlib-only — FR-U12 holds; NO new
  imports since url/strings/errors/fmt are already in releases.go): (1) releases.go — add the
  `ErrMalformedRepo` sentinel + a `(c *Client) repoPath() (string, error)` method (validates + returns
  `"/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(repo)`) + the `isRepoSegment` validator; EDIT the
  3 path-building sites to call `c.repoPath()` and return its error; [Mode A] godoc on repoPath
  documenting the why-not-whole-PathEscape rationale + BUG-008. (2) releases_test.go — add
  `TestClient_MalformedRepo_RejectedBeforeHTTP` (table-driven: "", "noslash", "a/b/c", "a//b", "/b",
  "a/", "a b/c", "a/b#c", … → each of the 3 methods returns errors.Is(err, ErrMalformedRepo) AND the
  httptest handler is NEVER invoked — proves validation is pre-request). The existing happy-path tests
  (Repo:"owner/repo") are UNCHANGED and are the regression proof that valid repos are unaffected
  (PathEscape is a no-op for [a-zA-Z0-9._-]). The diagnostic error string at ~line 248
  (`fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)`) is a message, NOT a request path —
  LEFT AS-IS (no injection vector). NO Client-struct/constructor change (fields stay caller-set; a
  constructor would churn the un-landed command layer P1.M4). NO edit to detect.go (parallel BUG-004
  timeout constant / BUG-007 Linuxbrew pathHeuristic — distinct file + line ranges; the P1.M3.T2.S1 PRP
  explicitly scopes out "BUG-008 (P1.M3.T3.S1, releases.go)"), config.go, go.mod, or any PRD/task file.
  go.mod unchanged. Scope: `git status --porcelain` == releases.go + releases_test.go ONLY.

---

## Goal

**Feature Goal**: Close the BUG-008 path-injection / malformed-request gap in the GitHub Releases client:
validate `c.Repo` is a well-formed "owner/repo" and escape each segment before interpolating it into
`/repos/{owner}/{repo}/...` request paths, so a mistyped `source_repo` config value yields a clear
`ErrMalformedRepo` error (no HTTP call) instead of a malformed request or a wrong-resource 404 — while
leaving valid "owner/repo" values byte-for-byte unaffected.

**Deliverable** (2 files, stdlib-only):
1. `internal/upgrade/releases.go` — add `var ErrMalformedRepo`; add `func (c *Client) repoPath() (string,
   error)` (split → validate each segment via `isRepoSegment` → escape each via `url.PathEscape` → return
   the `/repos/owner/repo` prefix); add `func isRepoSegment(s string) bool`; EDIT the 3 path-building
   sites (LatestStable, ReleaseByTag, LatestAdmittingPrereleases) to call `c.repoPath()` and return its
   error before `c.do(...)`. [Mode A] godoc on `repoPath` (the why-not-whole-PathEscape rationale + BUG-008).
2. `internal/upgrade/releases_test.go` — add `TestClient_MalformedRepo_RejectedBeforeHTTP` (table-driven;
   asserts all 3 methods return `errors.Is(err, ErrMalformedRepo)` AND an httptest "must-not-hit" handler
   is never invoked).

**Success Definition**:
- `c := &Client{Repo: "owner/repo"}` (and the default "dabstractor/stagecoach") → all 3 methods build the
  SAME path as today (`/repos/owner/repo/releases/...`); the existing happy-path test suite passes
  UNCHANGED (PathEscape is a no-op for `[a-zA-Z0-9._-]`).
- `c := &Client{Repo: <malformed>}` (any of: `""`, `"noslash"`, `"a/b/c"`, `"a//b"`, `"/b"`, `"a/"`,
  `"a b/c"`, `"a/b#c"`, `"a/b?c"`, …) → each of LatestStable / ReleaseByTag / LatestAdmittingPrereleases
  returns an error wrapping `ErrMalformedRepo`, and NO HTTP request is made (the httptest handler is
  never invoked).
- The 3 raw `"/repos/"+c.Repo+...` interpolations are GONE; the tag escaping (`url.PathEscape(tag)` in
  ReleaseByTag) is PRESERVED.
- NO new go.mod requires (url/strings/errors/fmt already imported in releases.go). NO `internal/*`
  import (FR-U12 grep guard).
- `go build ./...` clean; `gofmt -l` empty on the 2 files; `go vet ./internal/upgrade/...` clean;
  `go test ./internal/upgrade/ -race` green; `make test` + `make lint` clean.
- `git status --porcelain` == `internal/upgrade/releases.go` + `internal/upgrade/releases_test.go`.

## User Persona (if applicable)

**Target User**: A user who sets `source_repo` in their stagecoach config (to point at a fork or self-host)
—and mistypes it (a stray space, a missing slash, a copy-paste artifact). Also: the maintainer auditing
the upgrade subsystem's network surface for injection / malformed-request vectors.

**Use Case**: `stagecoach upgrade` resolves the GitHub release; the Client builds `/repos/{owner}/{repo}/...`.
A mistyped `source_repo` (e.g. `"dabstractor stagecoach"` or `"dabstractor//stagecoach"`) should fail
FAST with "malformed source repo (want \"owner/repo\")" — not build a garbage URL that 404s or, worse,
targets a wrong path.

**User Journey**: user sets `source_repo` (or keeps the default) → runs `stagecoach upgrade` → Client
validates c.Repo → [valid → proceeds as today] OR [malformed → clear ErrMalformedRepo, exit non-zero,
no confusing GitHub error].

**Pain Points Addressed**: BUG-008 — the asymmetry that the `tag` was url.PathEscape'd but `c.Repo` was
not, leaving a mistyped repo to produce a confusing HTTP failure instead of a clear config error; and
the latent path-injection vector (a `?`/`#`/space in c.Repo reaching the request line).

## Why

- **Defense-in-depth on the sole network surface**: releases.go is the ONLY place stagecoach makes
  network calls (§19 named exception). A request-path bug here is the place injection-style defects
  matter most. Validating + escaping the interpolated segment closes the gap the tag-only escaping left.
- **Clearer failure for mistyped config**: today a bad `source_repo` yields an opaque GitHub 404/HTTP
  error; after the fix it yields `ErrMalformedRepo` ("want \"owner/repo\"") — actionable at the config
  layer. The command layer (P1.M4) can surface it as a config problem, not a network problem.
- **Consistency with the tag escaping**: ReleaseByTag already escapes the tag; the repo was the
  inconsistent sibling. This makes all interpolated path components validated/escaped.
- **Zero-risk to the happy path**: for valid "owner/repo" the charset is `[a-zA-Z0-9._-]`, every char of
  which `url.PathEscape` leaves untouched — so escaping is a no-op and validation always passes. The
  change is invisible to every existing test and every real install.
- **Bounded, no-conflict scope**: 2 files in internal/upgrade. The parallel upgrade tasks (BUG-004
  defaultQueryTimeout, BUG-007 Linuxbrew pathHeuristic) edit `detect.go` at distinct line ranges —
  different file entirely.

## What

**User-visible behavior**: a mistyped `source_repo` now produces a clear "malformed source repo" error
instead of a confusing GitHub failure. Valid repos are unaffected.

**Technical change**: a `repoPath()` validation+escape helper + an `ErrMalformedRepo` sentinel, applied
at the 3 path-building method entries. Stdlib-only; no new imports.

### Success Criteria
- [ ] `internal/upgrade/releases.go` adds `var ErrMalformedRepo = errors.New("upgrade: malformed source
      repo (want \"owner/repo\")")` (alongside ErrNoReleases/ErrRateLimited/ErrHTTP).
- [ ] `internal/upgrade/releases.go` adds `func (c *Client) repoPath() (string, error)` that: splits
      `c.Repo` on `/`; requires exactly 2 non-empty segments each passing `isRepoSegment`; else returns
      `fmt.Errorf("source repo %q: %w", c.Repo, ErrMalformedRepo)`; on success returns
      `"/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])` (no trailing slash).
- [ ] `internal/upgrade/releases.go` adds `func isRepoSegment(s string) bool` (non-empty; every rune in
      `[a-zA-Z0-9._-]`).
- [ ] [Mode A] `repoPath` has a godoc explaining: WHY NOT `url.PathEscape(c.Repo)` (it breaks the `/`);
      validation is the gate, escaping is defense-in-depth; the charset; BUG-008.
- [ ] `LatestStable`, `ReleaseByTag`, `LatestAdmittingPrereleases` each call `c.repoPath()` and return
      its error before building the path. ReleaseByTag KEEPS its empty-tag guard (→ ErrHTTP) and its
      `url.PathEscape(tag)`.
- [ ] The 3 raw `"/repos/"+c.Repo+...` interpolations are GONE.
- [ ] `internal/upgrade/releases_test.go` adds `TestClient_MalformedRepo_RejectedBeforeHTTP` — a
      table over malformed repos, asserting each of the 3 methods returns `errors.Is(err,
      ErrMalformedRepo)` AND a "must-not-hit" httptest handler is never invoked.
- [ ] The existing happy-path tests (Repo:"owner/repo") pass UNCHANGED.
- [ ] `go build ./...` clean; `gofmt -l` empty on the 2 files; `go vet ./internal/upgrade/...` clean;
      `go test ./internal/upgrade/ -race` green; `make test` + `make lint` clean.
- [ ] NO new go.mod requires; NO `internal/*` import in releases.go (FR-U12).
- [ ] `git status --porcelain` == releases.go + releases_test.go ONLY.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim 3 path-building sites (with exact anchors), the verbatim `newReq`/`do`/method
bodies, the CRITICAL "PathEscape(c.Repo) is wrong" insight (with the %2F trace), the exact `repoPath()` +
`isRepoSegment` bodies, the exact 3-method edit pattern, the config default + charset justification, the
test idiom to mirror (`newFakeClient`, `errors.Is`, the `r.URL.Path` assertion, the "must-not-hit"
handler trick), the honest note that escaping is untestable-for-valid-input (validation is the gate),
the scope fences (parallel detect.go edits are a different file), and 8 grep guards.

### Documentation & References

```yaml
# MUST READ — the BUG-008 spec + the why-not-whole-PathEscape reasoning (the contract's key insight).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "## BUG-008 (MINOR): c.Repo unescaped in releases.go URLs"
  why: "Names the 3 sites (LatestStable ~179, ReleaseByTag ~198, LatestAdmittingPrereleases ~223) and
        states the trap: url.PathEscape escapes '/' to '%2F' which BREAKS 'owner/repo' — so the fix is
        split-and-escape each segment (or validate-at-construction). The contract picks split-and-escape
        as 'the most robust defense-in-depth'."
  critical: "Do NOT apply url.PathEscape to the WHOLE c.Repo — it yields 'owner%2Frepo' and GitHub 404s.
             Split on '/', escape each segment. Validation (charset [a-zA-Z0-9._-]) is the primary gate."

# MUST READ — codebase-specific findings for THIS item (verbatim sites, the repoPath/isRepoSegment bodies,
#              the test design, the escaping-is-a-no-op-for-valid-input honesty, scope fences, grep guards).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T3S1/research/findings.md
  why: "§1 the 3 sites + the line-~248 diagnostic message (LEAVE AS-IS); §2 the %2F trap; §3 the verbatim
        repoPath()/isRepoSegment/ErrMalformedRepo design + the 3-method edit pattern; §4 the config source
        + charset justification; §5 the test design (newFakeClient idiom + the must-not-hit handler); §6
        scope fences (parallel detect.go edits); §7 validation cmds; §8 grep guards."
  critical: "repoPath() is a METHOD on *Client (reads c.Repo) — not a free function. The 3 methods call it
             at their ENTRY and return its error BEFORE c.do(). The diagnostic fmt.Errorf at ~248 is a
             MESSAGE not a path — leave it. Escaping is a no-op for valid input — the testable gate is
             validation (bad repo → ErrMalformedRepo, no HTTP)."

# MUST READ — the file under edit (the 3 methods + newReq/do + the existing sentinels + imports).
- file: internal/upgrade/releases.go
  why: "LatestStable (~179: c.do(ctx, \"/repos/\"+c.Repo+\"/releases/latest\")); ReleaseByTag (~195 empty-tag
        guard → ErrHTTP, ~198 path with url.PathEscape(tag)); LatestAdmittingPrereleases (~223: c.do(ctx,
        \"/repos/\"+c.Repo+\"/releases\")). The sentinel block (~24-39) is where ErrMalformedRepo is added.
        imports (~13-22): net/url + strings + errors + fmt are ALL already present → NO new import."
  pattern: "Sentinels are `var ErrXxx = errors.New(\"upgrade: …\")`, wrapped at use sites with
            `fmt.Errorf(\"…: %w\", ErrXxx)` so errors.Is reaches them AND the concrete message survives.
            do() maps HTTP outcomes to the 3 sentinels; repoPath() adds a 4th (pre-request validation)."
  gotcha: "do() itself is UNCHANGED — it takes a pre-built path. Validation belongs at the 3 path-BUILDING
           sites (where the path is constructed), not in do() (which cannot re-derive c.Repo from a path)."

# MUST READ — the test file (the idioms to mirror: newFakeClient, errors.Is, r.URL.Path assertion).
- file: internal/upgrade/releases_test.go
  why: "newFakeClient(t, handler) (line 15) returns &Client{BaseURL: ts.URL, Repo: \"owner/repo\"} — the
        valid-repo pattern the happy-path tests use (they are the regression proof valid repos are
        unaffected). TestClient_ReleaseByTag_OK (line 189) asserts r.URL.Path via the handler.
        TestClient_LatestStable_404_NoReleases (line 81) shows the errors.Is(err, ErrXxx) idiom.
        TestClient_TransportFailure_HTTP (line 289) shows the bare &Client{BaseURL, Repo:...} construction."
  pattern: "Table-driven subtests with t.Run; assert via errors.Is(err, ErrXxx). For the must-not-hit
            proof: httptest.NewServer(handler) where handler does t.Errorf(\"...must not reach HTTP: %s\",
            r.URL.Path) — if repoPath() fails to reject, the handler fires and the test fails."
  gotcha: "The malformed-repo tests do NOT need a real handler response — repoPath() must error BEFORE
           c.do() is reached, so the handler should ONLY signal 'should not be invoked'. Set BaseURL to
           the must-not-hit server (NOT empty/default — an empty BaseURL would hit the REAL GitHub API if
           validation somehow failed, making the test flaky/networked)."

# CONTEXT — the config source (c.Repo ← config.Upgrade.SourceRepo) — confirms the default + that it is config-sourced.
- file: internal/config/config.go
  why: "Line 56: `SourceRepo string` with the comment 'owner/repo'; default 'dabstractor/stagecoach' (set
        for a fork/self-host). Line 259: the Defaults() seed `SourceRepo: \"dabstractor/stagecoach\"`.
        Confirms c.Repo is a CONFIG value (trusted-but-validatable) and that 'dabstractor/stagecoach'
        MUST pass repoPath() (it does — both segments are [a-zA-Z0-9._-])."
  gotcha: "Do NOT edit config.go — SourceRepo is correct as-is. repoPath() validates at USE time (in the
           Client), not at config load. (A constructor-time check would churn the un-landed command layer.)"

# CONTEXT — the parallel PRP (P1.M3.T2.S1, BUG-007) — confirm ZERO conflict (it edits detect.go, not releases.go).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T2S1/PRP.md
  why: "Confirms the sibling edits internal/upgrade/detect.go (pathHeuristics, ~line 370) — a DIFFERENT
        file from releases.go. It explicitly states 'BUG-008 (P1.M3.T3.S1, releases.go)' is OUT of its
        scope. P1.M3.T1.S1 (BUG-004) also edits detect.go (defaultQueryTimeout, ~line 121). → this item's
        2 files (releases.go + releases_test.go) do NOT overlap. No merge conflict."
  critical: "Do NOT edit detect.go or detect_test.go (the siblings own them during their implementation)."
```

### Current Codebase tree (relevant slice)

```bash
internal/upgrade/
  releases.go         # EDIT — +ErrMalformedRepo + repoPath() + isRepoSegment(); EDIT 3 path-building sites. [Mode A] godoc.
  releases_test.go    # EDIT — +TestClient_MalformedRepo_RejectedBeforeHTTP
  detect.go           # READ-ONLY — parallel P1.M3.T1.S1 (timeout const) + P1.M3.T2.S1 (Linuxbrew pathHeuristic); DO NOT TOUCH
  detect_test.go      # READ-ONLY — parallel siblings; DO NOT TOUCH
  download.go / resolve.go / stage.go / swap*.go / version.go  # READ-ONLY — LANDED siblings
internal/config/config.go  # READ-ONLY — SourceRepo default (correct as-is; not edited)
go.mod / Makefile / .golangci.yml   # READ-ONLY — validation (gofmt/vet/test/lint)
```

### Desired Codebase tree with files to be added/modified

```bash
internal/upgrade/releases.go         # EDIT — +sentinel +repoPath() +isRepoSegment(); 3 sites use repoPath(); [Mode A] godoc
internal/upgrade/releases_test.go    # EDIT — +TestClient_MalformedRepo_RejectedBeforeHTTP
# No new files. No detect.go/config.go/go.mod changes. No PRD/task file changes.
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (url.PathEscape(c.Repo) is the WRONG fix): c.Repo is "owner/repo". url.PathEscape escapes
// '/' to '%2F', yielding "owner%2Frepo" — a SINGLE path segment. GitHub 404s on /repos/owner%2Frepo/….
// The fix is SPLIT-AND-ESCAPE: strings.Split(c.Repo, "/") → validate 2 segments → url.PathEscape EACH.
// For valid repos ([a-zA-Z0-9._-]) PathEscape is a no-op (all unreserved), so the path is byte-identical.

// CRITICAL (validation is the gate; escaping is defense-in-depth): for a VALID "owner/repo" the charset
// is [a-zA-Z0-9._-], every char of which url.PathEscape leaves untouched. So escaping CANNOT be observed
// via a path diff for valid input — it only matters if a special char slipped past validation, which
// isRepoSegment prevents first. The TESTABLE guarantee is: malformed → ErrMalformedRepo + NO HTTP call.
// (Do not try to 'prove escaping' with a '.' repo — '.' is unreserved, not escaped. Test the validation.)

// CRITICAL (validate at the 3 path-BUILDING sites, NOT in do()): do() receives a pre-built path string;
// it cannot re-derive c.Repo. The validation belongs in repoPath(), called at the entry of each of
// LatestStable/ReleaseByTag/LatestAdmittingPrereleases — where the path is constructed. do() is UNCHANGED.

// GOTCHA (ErrMalformedRepo is a 4th sentinel, NOT ErrHTTP): the 3 existing sentinels (ErrNoReleases /
// ErrRateLimited / ErrHTTP) describe HTTP OUTCOMES. ErrMalformedRepo is a PRE-REQUEST validation failure
// (no request was made). Do NOT wrap it as ErrHTTP (that overloads 'HTTP request failed'). The command
// layer (P1.M4) maps errors generically via errors.Is; a distinct sentinel gives a clearer config error.

// GOTCHA (NO constructor change): the Client is a plain struct; fields are set directly by the caller
// (the un-landed command layer P1.M4.T1.S1). repoPath() is a USE-TIME check (called per method), not a
// constructor — adding a NewClient would churn that un-landed consumer for no gain.

// GOTCHA (the line-~248 diagnostic is a MESSAGE, not a path): `fmt.Errorf("/repos/%s/releases: %w",
// c.Repo, ErrNoReleases)` in LatestAdmittingPrereleases interpolates c.Repo into an ERROR STRING for
// humans — it is NOT a request path and has no injection vector. LEAVE IT AS-IS (minimal scope). Only
// the 3 c.do(...) / path-building sites change.

// GOTCHA (charset is GitHub's actual restriction, not over-restrictive): GitHub owner/repo names allow
// exactly [a-zA-Z0-9._-] (and hyphens; no spaces, no Unicode, no other punctuation). isRepoSegment
// encodes that contract — a segment with any other char is genuinely malformed config, not a false reject.

// GOTCHA (stdlib-only, FR-U12): releases.go already imports net/url (ReleaseByTag's tag escape),
// strings, errors, fmt. repoPath()/isRepoSegment reuse those — NO new import. NO internal/* (grep guard).
```

## Implementation Blueprint

### Data models and structure

```go
// One new sentinel (alongside ErrNoReleases/ErrRateLimited/ErrHTTP). No new structs/types.
var ErrMalformedRepo = errors.New("upgrade: malformed source repo (want \"owner/repo\")")
// repoPath() + isRepoSegment() are plain methods/funcs. No Client-struct change.
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: ADD ErrMalformedRepo + repoPath() + isRepoSegment() to internal/upgrade/releases.go
  - SENTINEL: add to the existing `var ( … )` sentinel block (after ErrHTTP):
        // ErrMalformedRepo is returned (pre-request) when c.Repo is not a well-formed "owner/repo" —
        // i.e. not exactly two non-empty segments of GitHub's owner/repo charset [a-zA-Z0-9._-]. It is
        // a config-validation error (distinct from the HTTP-outcome sentinels): no request is made.
        // BUG-008: previously c.Repo was interpolated into request paths UNESCAPED.
        ErrMalformedRepo = errors.New("upgrade: malformed source repo (want \"owner/repo\")")
  - HELPER isRepoSegment (place near repoPath, or with the other unexported helpers):
        // isRepoSegment reports whether s is a non-empty run of GitHub owner/repo name characters
        // ([a-zA-Z0-9._-]). It is the validation gate for repoPath (BUG-008): GitHub restricts owner
        // and repo names to this set, so any other character (space, '/', control, Unicode, '%', query
        // or fragment markers) is rejected as malformed config.
        func isRepoSegment(s string) bool {
            if s == "" {
                return false
            }
            for _, r := range s {
                switch {
                case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9',
                    r == '.' || r == '_' || r == '-':
                default:
                    return false
                }
            }
            return true
        }
  - METHOD repoPath (place on *Client, near do()/newReq):
        // repoPath returns the validated, segment-escaped "/repos/{owner}/{repo}" path prefix for c.Repo
        // (BUG-008). c.Repo is config-sourced ("owner/repo", default "dabstractor/stagecoach") and is
        // interpolated into GitHub API paths.
        //
        // url.PathEscape(c.Repo) is the WRONG fix: it escapes the '/' to "%2F", collapsing "owner/repo"
        // into a single segment that GitHub 404s on. Instead this splits on '/', VALIDATES each segment
        // via isRepoSegment (the primary gate — a malformed config value → ErrMalformedRepo, no HTTP
        // call), and percent-escapes each segment individually. For valid repos the charset is
        // [a-zA-Z0-9._-] — all unreserved — so url.PathEscape is a no-op and the path is byte-identical
        // to the prior unescaped form; the per-segment escaping is defense-in-depth (it only matters if
        // a special char ever bypassed validation, which validation prevents first). Returns the prefix
        // WITHOUT a trailing slash (callers append "/releases…").
        func (c *Client) repoPath() (string, error) {
            parts := strings.Split(c.Repo, "/")
            if len(parts) != 2 || !isRepoSegment(parts[0]) || !isRepoSegment(parts[1]) {
                return "", fmt.Errorf("source repo %q: %w", c.Repo, ErrMalformedRepo)
            }
            return "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]), nil
        }
  - NAMING: ErrMalformedRepo (matches the ErrXxx sentinel convention); repoPath (method, reads c.Repo);
    isRepoSegment (validator). All match the package's style.
  - GOTCHA: NO new import (url/strings/errors/fmt already present). NO Client-struct change.

Task 2: EDIT the 3 path-building sites in releases.go to use repoPath()
  - SITE 1 — LatestStable (the `body, err := c.do(ctx, "/repos/"+c.Repo+"/releases/latest")` line):
        prefix, err := c.repoPath()
        if err != nil {
            return Release{}, err
        }
        body, err := c.do(ctx, prefix+"/releases/latest")
        if err != nil {
            return Release{}, err
        }
    (The rest of LatestStable — the json.Unmarshal + toRelease — is UNCHANGED.)
  - SITE 2 — ReleaseByTag (KEEP the empty-tag guard; insert repoPath after it; build path from prefix):
        if strings.TrimSpace(tag) == "" {
            return Release{}, fmt.Errorf("release by tag: empty tag: %w", ErrHTTP)   // UNCHANGED
        }
        prefix, err := c.repoPath()
        if err != nil {
            return Release{}, err
        }
        path := prefix + "/releases/tags/" + url.PathEscape(tag)   // tag STILL escaped
        body, err := c.do(ctx, path)
        ... // UNCHANGED
  - SITE 3 — LatestAdmittingPrereleases (the `body, err := c.do(ctx, "/repos/"+c.Repo+"/releases")` line):
        prefix, err := c.repoPath()
        if err != nil {
            return Release{}, err
        }
        body, err := c.do(ctx, prefix+"/releases")
        ... // UNCHANGED (the selection loop + the line-~248 diagnostic fmt.Errorf is LEFT AS-IS)
  - PRESERVE: the empty-tag guard (ErrHTTP); the url.PathEscape(tag); the do() three-sentinel mapping;
    the selection loop in LatestAdmittingPrereleases; the line-~248 diagnostic message (NOT a path).
  - VERIFY: grep `c.repoPath()` → 3 hits; grep `"/repos/"+c.Repo` → ZERO hits in path-building code.

Task 3: ADD TestClient_MalformedRepo_RejectedBeforeHTTP to internal/upgrade/releases_test.go
  - FILE COMMENT (or inline): note that this proves the VALIDATION gate (bad repo → ErrMalformedRepo +
    no HTTP); escaping is a no-op for valid input and is exercised only indirectly (the happy-path tests
    with Repo:"owner/repo" prove valid repos are unaffected).
  - BODY (table-driven; mirror newFakeClient's spirit but with a must-not-hit handler):
        func TestClient_MalformedRepo_RejectedBeforeHTTP(t *testing.T) {
            // A handler that MUST NOT be invoked — repoPath() rejects before c.do() builds/sends the request.
            ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                t.Errorf("malformed repo must not reach HTTP: %s", r.URL.Path)
            }))
            t.Cleanup(ts.Close)
            bad := []string{
                "",                 // empty
                "noslash",          // no slash → 1 segment
                "a/b/c",            // 3 segments
                "a//b",             // empty middle segment
                "/b",               // empty owner (split → ["","b"])
                "a/",               // empty repo (split → ["a",""])
                "a b/c",            // space in owner
                "a/b c",            // space in repo
                "a/b#c",            // fragment marker
                "a/b?c",            // query marker
                "owner/repo!",      // '!' not in charset
            }
            for _, repo := range bad {
                t.Run(repo, func(t *testing.T) {
                    c := &Client{BaseURL: ts.URL, Repo: repo}
                    if _, err := c.LatestStable(context.Background()); !errors.Is(err, ErrMalformedRepo) {
                        t.Errorf("LatestStable err = %v, want errors.Is ErrMalformedRepo", err)
                    }
                    if _, err := c.ReleaseByTag(context.Background(), "v1.2.3"); !errors.Is(err, ErrMalformedRepo) {
                        t.Errorf("ReleaseByTag err = %v, want errors.Is ErrMalformedRepo", err)
                    }
                    if _, err := c.LatestAdmittingPrereleases(context.Background()); !errors.Is(err, ErrMalformedRepo) {
                        t.Errorf("LatestAdmittingPrereleases err = %v, want errors.Is ErrMalformedRepo", err)
                    }
                })
            }
        }
  - FOLLOW pattern: releases_test.go's `&Client{BaseURL: …, Repo: …}` construction + errors.Is idiom.
  - GOTCHA: BaseURL MUST be the must-not-hit server (NOT ""/default) — so a validation FAILURE (bug)
    hits the fake (t.Errorf) instead of the real GitHub API (flaky/networked). The handler's t.Errorf
    is the proof validation ran pre-request.
  - GOTCHA: the empty-string repo case (`""`) — strings.Split("", "/") returns [""] (1 element) →
    len != 2 → ErrMalformedRepo. Verify this is the observed behavior (it is).

Task 4: VERIFY — build, format, vet, tests (new + regression), lint, grep guards
  - go build ./...
  - gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go   # empty
  - go vet ./internal/upgrade/...
  - go test ./internal/upgrade/ -run 'TestClient_MalformedRepo|TestClient_LatestStable|TestClient_ReleaseByTag|TestClient_Prerelease' -race -v
  - go test ./internal/upgrade/ -race     # full regression — existing happy-path tests prove valid repos unaffected
  - make test && make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN (the validation+escape helper — split, validate each segment, escape each segment):
func (c *Client) repoPath() (string, error) {
	parts := strings.Split(c.Repo, "/")
	if len(parts) != 2 || !isRepoSegment(parts[0]) || !isRepoSegment(parts[1]) {
		return "", fmt.Errorf("source repo %q: %w", c.Repo, ErrMalformedRepo)
	}
	return "/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]), nil
}

// PATTERN (each of the 3 methods validates at its entry, before c.do):
prefix, err := c.repoPath()
if err != nil {
	return Release{}, err
}
body, err := c.do(ctx, prefix+"/releases/latest") // or prefix+"/releases/tags/"+url.PathEscape(tag), or prefix+"/releases")

// PATTERN (the malformed-repo test — must-not-hit handler proves pre-request validation):
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	t.Errorf("malformed repo must not reach HTTP: %s", r.URL.Path) // fires ONLY if repoPath fails to reject
}))
```

### Integration Points

```yaml
PRODUCTION (internal/upgrade/releases.go):
  - +var ErrMalformedRepo (sentinel block)
  - +func (c *Client) repoPath() (string, error)
  - +func isRepoSegment(s string) bool
  - EDIT LatestStable / ReleaseByTag / LatestAdmittingPrereleases to call c.repoPath() at entry.

IMPORTS (all stdlib — FR-U12): NONE new. releases.go already imports context, encoding/json, errors,
  fmt, io, net/http, net/url, strings. repoPath/isRepoSegment reuse url+strings+errors+fmt.

CONSUMERS (the command layer P1.M4.T1.S1, un-landed): constructs &Client{Repo: config.SourceRepo, …} and
  calls LatestStable/ReleaseByTag/LatestAdmittingPrereleases. A malformed SourceRepo now surfaces as
  errors.Is(err, upgrade.ErrMalformedRepo) — the command layer may print a config-focused message. The
  Client struct API is UNCHANGED (no constructor; fields still caller-set).

NO database / migration / routes / new types beyond the sentinel / new flag / config change / exitcode
mapping (the command layer maps ErrMalformedRepo generically) / docs (Mode A: code-comment only; the
docs sync is P1.M3.T4.S1).

SCOPE FENCES:
  - Touches ONLY internal/upgrade/releases.go + internal/upgrade/releases_test.go.
  - Does NOT edit detect.go / detect_test.go (parallel P1.M3.T1.S1 BUG-004 + P1.M3.T2.S1 BUG-007 —
    distinct file, distinct line ranges; the P1.M3.T2.S1 PRP explicitly scopes out BUG-008), config.go
    (SourceRepo default is correct as-is), the command layer, download.go, resolve.go, stage.go,
    swap*.go, version.go, go.mod, or any PRD/task file.
  - Adds NO flag, NO config field, NO third-party dependency, NO new exported TYPE (ErrMalformedRepo is a
    var; repoPath/isRepoSegment are unexported; no struct change).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build (no new import — url/strings/errors/fmt already present).
go build ./...
# Expected: clean.

# Format.
gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go
# Expected: empty. If listed: gofmt -w internal/upgrade/releases.go internal/upgrade/releases_test.go.

# Vet.
go vet ./internal/upgrade/...
# Expected: clean.

# Lint.
make lint
# Expected: zero errors (repoPath/isRepoSegment/ErrMalformedRepo all used; no unused-symbol finding).

# Scope guard: ONLY the 2 files changed.
git status --porcelain
# Expected: internal/upgrade/releases.go, internal/upgrade/releases_test.go. ZERO changes to detect.go /
#           detect_test.go / config.go / go.mod.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The new malformed-repo test + the existing happy-path tests for the 3 methods.
go test ./internal/upgrade/ -run 'TestClient_MalformedRepo|TestClient_LatestStable|TestClient_ReleaseByTag|TestClient_Prerelease' -race -v
# Expected: ALL PASS —
#   TestClient_MalformedRepo_RejectedBeforeHTTP: every bad-repo subtest → errors.Is(err, ErrMalformedRepo)
#     for all 3 methods + the must-not-hit handler NEVER fires (no t.Errorf).
#   TestClient_LatestStable_OK / _404 / _403 / _429 / _500, TestClient_ReleaseByTag_OK / _404 / _EmptyTag,
#     TestClient_Prerelease_*: all UNCHANGED (Repo:"owner/repo" → repoPath() returns the same path →
#     identical behavior). These ARE the regression proof valid repos are unaffected.

# Full upgrade-package regression.
go test ./internal/upgrade/ -race
# Expected: green.

# Full race suite + lint.
make test
# Expected: green.
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A — this is a request-PATH construction fix in a unit-testable client (the httptest fakes in
# releases_test.go ARE the integration proof: they assert the exact r.URL.Path the client builds).
# The end-to-end `stagecoach upgrade` against the REAL GitHub API is the e2e harness's job (P1.M4) —
# it exercises the production default Repo "dabstractor/stagecoach" (which repoPath() accepts, so the
# path is byte-identical to today). No new integration step is owed here.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the 3 raw c.Repo interpolations are GONE.
grep -n '"/repos/" + c.Repo\|"/repos/"+c.Repo\|c.Repo + "/releases\|c.Repo+"/releases' internal/upgrade/releases.go
# Expect: ZERO hits in path-building code. (The line-~248 fmt.Errorf DIAGNOSTIC "/repos/%s/releases" may
#         still match a loose grep — that is a MESSAGE, not a path; it is intentionally left. Disambiguate
#         by checking it is inside an fmt.Errorf, not an c.do(...) arg.)

# Guard 2: repoPath() exists + is called at all 3 method entries.
grep -n 'func (c \*Client) repoPath()' internal/upgrade/releases.go   # 1 hit (the definition)
grep -n 'c.repoPath()' internal/upgrade/releases.go                   # 3 hits (the 3 call sites)

# Guard 3: ErrMalformedRepo + isRepoSegment exist.
grep -n 'ErrMalformedRepo' internal/upgrade/releases.go   # ≥3 hits (decl + repoPath error-wrap + …)
grep -n 'func isRepoSegment' internal/upgrade/releases.go # 1 hit

# Guard 4: the tag escaping is PRESERVED in ReleaseByTag.
grep -n 'url.PathEscape(tag)' internal/upgrade/releases.go  # 1 hit

# Guard 5: the empty-tag guard is PRESERVED (still → ErrHTTP).
grep -n 'release by tag: empty tag' internal/upgrade/releases.go  # 1 hit

# Guard 6: the do() three-sentinel mapping is UNCHANGED (ErrNoReleases/ErrRateLimited/ErrHTTP all present).
grep -n 'ErrNoReleases\|ErrRateLimited\|ErrHTTP' internal/upgrade/releases.go  # each still present

# Guard 7: stdlib-only (FR-U12) — no new internal/* import.
grep -nE 'github.com/dabstractor/stagecoach/internal/' internal/upgrade/releases.go
# Expect: ZERO hits (the package imports NO internal/* — walled off).

# Guard 8: the malformed-repo test exists + uses a must-not-hit handler.
grep -n 'func TestClient_MalformedRepo_RejectedBeforeHTTP' internal/upgrade/releases_test.go  # 1 hit
grep -n 'must not reach HTTP' internal/upgrade/releases_test.go  # 1 hit (the must-not-hit handler)

# Guard 9: scope — only releases.go + releases_test.go.
git status --porcelain
# Expect: the 2 files ONLY.
git diff --name-only | grep -E 'detect\.go|config\.go|go\.mod|swap\.go|stage\.go|resolve\.go' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (no new import)
- [ ] `gofmt -l` empty on releases.go + releases_test.go
- [ ] `go vet ./internal/upgrade/...` clean
- [ ] `make lint` zero errors
- [ ] `go test ./internal/upgrade/ -race` green (new test + full regression)
- [ ] `make test` green

### Feature Validation
- [ ] Malformed c.Repo (all table cases) → each of the 3 methods returns errors.Is(err, ErrMalformedRepo)
      AND the must-not-hit handler is never invoked (Task 3) (grep guards 2,3,8)
- [ ] Valid c.Repo ("owner/repo", "dabstractor/stagecoach") → path byte-identical to today; existing
      happy-path tests pass UNCHANGED (the regression proof)
- [ ] The 3 raw c.Repo interpolations are GONE; the tag escaping + empty-tag guard preserved (grep 1,4,5)
- [ ] do() + its 3 sentinels are UNCHANGED (grep 6)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY internal/upgrade/releases.go + internal/upgrade/releases_test.go (grep 9)
- [ ] NO edit to detect.go / detect_test.go (parallel siblings), config.go, the command layer, go.mod,
      or any PRD/task file
- [ ] NO new exported type, NO new flag, NO new import, NO third-party dependency (grep 7)
- [ ] The line-~248 diagnostic fmt.Errorf is LEFT AS-IS (it is a message, not a path)

### Code Quality & Docs
- [ ] [Mode A] repoPath godoc explains: why NOT whole-PathEscape (%2F breaks owner/repo); validation is
      the gate, escaping is defense-in-depth; the charset; BUG-008
- [ ] isRepoSegment godoc explains the charset is GitHub's actual restriction
- [ ] The test file documents that escaping is a no-op for valid input (validation is the testable gate)
- [ ] Follows the package's errors.New("upgrade: …") sentinel + fmt.Errorf("…: %w", ErrXxx) conventions

---

## Anti-Patterns to Avoid

- ❌ Don't apply `url.PathEscape` to the WHOLE `c.Repo`. It escapes `/` → `%2F`, collapsing "owner/repo"
  into one segment that GitHub 404s on. SPLIT on `/` and escape EACH segment. (The single most important
  constraint — the contract and the architecture doc both flag it.)
- ❌ Don't put the validation in `do()`. do() takes a pre-built path string; it cannot re-derive c.Repo.
  The validation belongs in `repoPath()`, called at the ENTRY of each of the 3 path-BUILDING methods.
- ❌ Don't wrap ErrMalformedRepo as ErrHTTP. The 3 existing sentinels describe HTTP OUTCOMES; a malformed
  repo is a PRE-REQUEST validation failure (no request was made). A distinct sentinel gives the command
  layer a clearer, config-focused error. (Don't overload "HTTP request failed".)
- ❌ Don't try to "prove the escaping" with a valid repo. The valid charset `[a-zA-Z0-9._-]` is entirely
  unreserved, so `url.PathEscape` is a no-op — there is no path diff to assert. Escaping is pure
  defense-in-depth (only matters if validation is bypassed, which cannot happen). The TESTABLE gate is
  VALIDATION: bad repo → ErrMalformedRepo + no HTTP. Test that. (A '.'-containing repo doesn't prove
  escaping — '.' is unreserved, not escaped.)
- ❌ Don't change the Client struct or add a constructor. The fields are caller-set (the un-landed command
  layer P1.M4.T1.S1 constructs `&Client{Repo: ..., ...}`). repoPath() is a USE-TIME check — adding a
  NewClient would churn that consumer for no gain and expand the scope.
- ❌ Don't touch `detect.go` or `detect_test.go`. The parallel P1.M3.T1.S1 (BUG-004 timeout const) and
  P1.M3.T2.S1 (BUG-007 Linuxbrew pathHeuristic) own them. This item is releases.go + releases_test.go
  ONLY. (The P1.M3.T2.S1 PRP explicitly scopes out "BUG-008 (P1.M3.T3.S1, releases.go)".)
- ❌ Don't edit the line-~248 diagnostic `fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)`.
  It is an ERROR MESSAGE for humans, not a request path — there is no injection vector, and the c.Repo
  there is informational. Leave it (minimal scope). Only the 3 c.do(...)/path-building sites change.
- ❌ Don't drop the empty-tag guard or the `url.PathEscape(tag)` in ReleaseByTag. The empty-tag guard
  (→ ErrHTTP) and the tag escaping are correct and MUST be preserved; repoPath() is ADDED alongside them,
  not replacing them.
- ❌ Don't set the malformed-repo test's BaseURL to "" (default GitHub). If validation fails to reject,
  the test would hit the REAL GitHub API (flaky, networked, possibly rate-limited). ALWAYS point it at an
  httptest "must-not-hit" server whose handler does t.Errorf — so a validation bug fails the test
  deterministically instead of leaking to the network.
- ❌ Don't add a charset char outside GitHub's actual set. The owner/repo charset is exactly
  `[a-zA-Z0-9._-]` (GitHub's restriction). Adding e.g. `+` would reject valid names; omitting `.`/`-`/`_`
  would reject valid names too. Encode the real contract.
- ❌ Don't sync docs here. [Mode A]: the repoPath godoc IS the documentation. The README/packaging docs
  sync is P1.M3.T4.S1.