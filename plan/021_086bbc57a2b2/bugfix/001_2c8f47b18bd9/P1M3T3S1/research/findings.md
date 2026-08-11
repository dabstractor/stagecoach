# P1.M3.T3.S1 — Research Findings (Validate/escape c.Repo segments in releases.go — BUG-008)

Source: direct read of internal/upgrade/releases.go + releases_test.go + the architecture BUG-008
spec + config.go (SourceRepo default). No external research (stdlib net/url only). ~3 tool calls.

## §0 — The bug (architecture/bugfix_subsystems.md §BUG-008; contract)

`internal/upgrade/releases.go` builds GitHub API request paths by interpolating `c.Repo` (the
"owner/repo" config string) UNESCAPED at 3 sites, while the `tag` IS escaped. A malformed `c.Repo`
(containing a space, `?`, `#`, control char, or extra `/`) would build a malformed HTTP request line
or silently target the wrong resource. Fix: validate + segment-escape `c.Repo` so a bad config value
yields a clear error and a normal "owner/repo" is unaffected.

## §1 — The 3 path-building sites (releases.go) + the 1 diagnostic message

- **LatestStable** (~line 179): `c.do(ctx, "/repos/"+c.Repo+"/releases/latest")` — c.Repo UNESCAPED.
- **ReleaseByTag** (~line 198): `path := "/repos/" + c.Repo + "/releases/tags/" + url.PathEscape(tag)` —
  tag escaped, c.Repo NOT. (Also: empty-tag guard at ~195 returns ErrHTTP — keep.)
- **LatestAdmittingPrereleases** (~line 223): `c.do(ctx, "/repos/"+c.Repo+"/releases")` — c.Repo UNESCAPED.
- **Diagnostic message** (~line 248): `fmt.Errorf("/repos/%s/releases: %w", c.Repo, ErrNoReleases)` —
  this is an ERROR STRING (not a request path); no injection vector. LEAVE AS-IS (minimal scope).

## §2 — Why url.PathEscape(c.Repo) is WRONG (the contract's key insight)

`c.Repo` is "owner/repo". `url.PathEscape` escapes `/` → `%2F`, so `url.PathEscape("owner/repo")` ==
`"owner%2Frepo"`, which yields `/repos/owner%2Frepo/releases/latest` — GitHub treats that as a SINGLE
segment and 404s. The GitHub API REQUIRES the literal `owner/repo` (two path segments). So the fix is
NOT "PathEscape the whole c.Repo". It is: **split on `/`, validate each segment, escape each segment
individually**:
```go
"/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
```
For valid GitHub owner/repo names (charset `[a-zA-Z0-9._-]`), `url.PathEscape` is a no-op (all those
chars are unreserved) — so valid repos are byte-identical to today. The escaping is pure
defense-in-depth (it only matters if a special char slipped past validation, which validation prevents
first). The VALIDATION is the primary gate (malformed → clear error, no HTTP call).

## §3 — The design: repoPath() helper + ErrMalformedRepo sentinel (all in releases.go)

- **New sentinel** (alongside ErrNoReleases/ErrRateLimited/ErrHTTP):
  `var ErrMalformedRepo = errors.New("upgrade: malformed source repo (want \"owner/repo\")")`.
  It is a PRE-REQUEST validation error (distinct from the 3 HTTP-outcome sentinels); the command layer
  (P1.M4) maps it generically via errors.Is. NOT ErrHTTP (don't overload "HTTP request failed" — no
  request was made).
- **New method** `(c *Client) repoPath() (string, error)`:
  - `strings.Split(c.Repo, "/")` → must yield exactly 2 non-empty segments, each `isRepoSegment`
    (charset `[a-zA-Z0-9._-]`, non-empty). Else → `fmt.Errorf("source repo %q: %w", c.Repo, ErrMalformedRepo)`.
  - Returns `"/repos/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])` (NO trailing slash).
  - [Mode A] godoc: documents WHY not-whole-PathEscape (breaks /), that validation is the gate + escaping
    is defense-in-depth, the charset, and BUG-008.
- **New validator** `isRepoSegment(s string) bool`: non-empty, every rune in `[a-zA-Z0-9._-]`.
- **Apply at the 3 method entries** (LatestStable / ReleaseByTag / LatestAdmittingPrereleases):
  ```go
  prefix, err := c.repoPath()
  if err != nil { return Release{}, err }
  body, err := c.do(ctx, prefix+"/releases/latest")   // or prefix+"/releases/tags/"+url.PathEscape(tag), or prefix+"/releases"
  ```
  ReleaseByTag keeps its existing empty-tag guard (returns ErrHTTP) BEFORE repoPath (or after — order
  between two pre-request validation errors is immaterial; keep empty-tag first for minimal diff).

NO new imports (`url`, `strings`, `errors`, `fmt` all already imported). NO Client-struct change (the
fields are still set by the caller — a constructor would churn the un-landed command layer P1.M4).
NO change to `do()` (it receives a pre-built path; validation belongs at the path-building sites).

## §4 — The config source + why "owner/repo" is trusted-but-validated

`c.Repo` ← `config.Upgrade.SourceRepo` (config.go:56, default `"dabstractor/stagecoach"` at
config.go:259; overridable for a fork/self-host). It is a CONFIG value (not user input at request
time), so it is "trusted" in the sense that the user controls it — BUT config values can be mistyped
(space, stray char) and the GitHub URL contract must still be upheld. Validation turns a mistyped
`source_repo` into a clear "malformed source repo" error instead of a confusing 404/HTTP failure.
The charset `[a-zA-Z0-9._-]` is GitHub's actual owner/repo name restriction (GitHub does not allow
spaces, Unicode, or punctuation beyond `.`/`_`/`-`), so the validator is correct, not over-restrictive.

## §5 — Test design (mirror releases_test.go's newFakeClient + errors.Is idioms)

Existing tests (releases_test.go) use `Repo: "owner/repo"` (valid) via `newFakeClient(t, handler)`
(line 15: `&Client{BaseURL: ts.URL, Repo: "owner/repo"}`) and assert `errors.Is(err, ErrXxx)`.
`TestClient_ReleaseByTag_OK` (line 189) asserts `r.URL.Path` via the handler. AFTER the fix these all
STILL PASS (valid repo → path byte-identical) — they are the regression proof that valid repos are
unaffected.

NEW test `TestClient_MalformedRepo_RejectedBeforeHTTP` (table-driven):
- Bad-repo cases: `""`, `"noslash"`, `"a/b/c"` (3 segments), `"a//b"` (empty segment), `"/b"` (empty
  owner → split ["","b"]), `"a/"` (empty repo → split ["a",""]), `"a b/c"` (space in owner),
  `"a/b c"` (space in repo), `"a/b#c"` (fragment marker), `"a/b?c"` (query marker).
- For each: `c := &Client{BaseURL: mustNotHit.URL, Repo: bad}` where `mustNotHit` is an httptest server
  whose handler does `t.Errorf("malformed repo must not reach HTTP: %s", r.URL.Path)` — PROVES the
  validation fires before any request. Call all 3 methods (LatestStable, ReleaseByTag("v1"),
  LatestAdmittingPrereleases); assert `errors.Is(err, ErrMalformedRepo)` for each.

NOTE on escaping testability: for valid repos the charset is `[a-zA-Z0-9._-]`, ALL of which
`url.PathEscape` leaves untouched — so escaping is provably a no-op for valid input and cannot be
asserted via a path diff. The escaping is pure defense-in-depth (only matters if validation were
bypassed, which cannot happen). The VALIDATION (bad repo → ErrMalformedRepo, no HTTP) is the testable
gate. Document this honestly in the test file comment.

## §6 — Scope fences (zero overlap with parallel upgrade tasks)

TOUCHES (2 files):
- `internal/upgrade/releases.go` — +ErrMalformedRepo sentinel + repoPath() method + isRepoSegment()
  validator; EDIT the 3 path-building sites to use repoPath(); [Mode A] godoc.
- `internal/upgrade/releases_test.go` — +TestClient_MalformedRepo_RejectedBeforeHTTP.

DOES NOT TOUCH (zero overlap):
- `internal/upgrade/detect.go` — parallel P1.M3.T1.S1 (BUG-004, defaultQueryTimeout constant ~line 121)
  AND P1.M3.T2.S1 (BUG-007, pathHeuristics Linuxbrew entry ~line 370). The P1.M3.T2.S1 PRP explicitly
  states "BUG-008 (P1.M3.T3.S1, releases.go)" is out of its scope. Distinct file, distinct line ranges.
- `internal/config/config.go` / `upgrade.go` — SourceRepo default is correct as-is; not edited.
- The command layer (P1.M4.T1.S1, un-landed) that constructs the Client — not edited (repoPath() is a
  use-time validation, not a constructor change).
- go.mod (NO new imports — url/strings/errors/fmt already in releases.go), detect_test.go, any PRD/task
  file, README/docs (Mode A: code-comment only; the docs sync is P1.M3.T4.S1).

## §7 — Validation commands (verified)

```bash
go build ./...                                       # clean (no new import)
gofmt -l internal/upgrade/releases.go internal/upgrade/releases_test.go   # empty
go vet ./internal/upgrade/...                        # clean
go test ./internal/upgrade/ -run 'TestClient_MalformedRepo|TestClient_LatestStable|TestClient_ReleaseByTag|TestClient_Prerelease' -race -v
go test ./internal/upgrade/ -race                    # full regression — existing tests prove valid repos unaffected
make test && make lint
git status --porcelain                               # == releases.go + releases_test.go ONLY
```

## §8 — Grep guards
- The 3 raw `"/repos/"+c.Repo+` interpolations are GONE (grep `c.Repo+"/releases` / `"/repos/"+c.Repo`
  → ZERO hits in path-building code).
- `repoPath()` exists + is called at all 3 method entries (grep `c.repoPath()` → 3 hits).
- `ErrMalformedRepo` + `isRepoSegment` exist (grep → ≥1 hit each).
- The tag escaping is preserved (grep `url.PathEscape(tag)` → still 1 hit in ReleaseByTag).
- No new go.mod requires; no `internal/*` import in releases.go (FR-U12 grep guard).
- Scope: ONLY releases.go + releases_test.go changed.