# Test Patterns to Mirror — FR-39a regression net

## 1. The §20.2 invariant suite (where the behavioral test goes)

**File:** `internal/generate/invariants_test.go` (package `generate`).

- **`TestInvariants`** (line 159) is a **table-driven FAILURE-path** suite: every `scenario` row drives
  `CommitStaged` expecting `err != nil`, then funnels through `assertInvariants(t, repo, before,
  treeSHA, wantHead)` (line 99) which checks the three index/HEAD/tree immutability invariants.
  - **IMPORTANT:** because every existing row is a failure path, FR-39a's test (a SUCCESS path — a
    commit lands and must carry git's identity) should be a **dedicated `TestCommitIdentityTransparency`
    alongside `TestInvariants`**, NOT a new row in the failure table. The shared fixture helpers are
    reusable; only the assertion differs.

### Shared fixture helpers (package `generate`, defined in `generate_test.go`)
- `initRepo(t, dir)` — `generate_test.go:26` — `git init` + config.
- `commitRaw(t, dir, msg)` — `:55` — a raw plumbing/porcelain commit for fixture seeding.
- `headSHA(t, dir)` — `:49`.
- `gitOut(t, dir, args...)` — `:61` — runs `git -C dir <args>`, returns trimmed stdout.
- `writeFile(t, dir, name, body)` / `stageFile(t, dir, name)` — shared across the package's `_test.go`.

### The stub provider API (`internal/stubtest`)
- `stubtest.Build(t)` → compiles + returns the stub binary path.
- `stubtest.Manifest(bin, stubtest.Options{Out: "feat: x"})` → a `provider.Manifest` whose stub emits
  `Out` once on stdout (exit 0). This is the **canned commit message** the success-path test uses; the
  message is the ONLY content allowed in the resulting commit (no trailer stagecoach could have added).
- `stubtest.NewScript(t, bin, []string{...})` → multi-response manifest.
- `Options{Exit, SleepMS}` simulate failure/slow agents (not needed for the success path).

### The success-path invocation
```go
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add widget"})   // canned message
cfg := config.Defaults()
res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
// on success: err == nil, res.CommitSHA is the landed commit (internal/generate/generate.go:72)
```

### The exact identity-comparison query (PRD §20.2 verbatim)
```
git log -1 --format='%an <%ae> | %cn <%ce>' <res.CommitSHA>
```
Compare against the identity git resolves **before** the run:
```
git -C <repo> config user.name   ;   git -C <repo> config user.email
```
(both set by the fixture; in the transparency case set to `"stagecoach agent"` / `agent@stagecoach.local`).

### Message-trailer assertion
`git log -1 --format=%B <res.CommitSHA>` must equal EXACTLY the stub's canned message (e.g.
`"feat: add widget"`) — no `Co-Authored-By:` / `Generated-by:` / `🤖` appended. Since the stub's
message is known and short, assert `strings.TrimSpace(got) == canned` (stronger than a substring
"does not contain").

## 2. The transparency-without-authorship case (mirror `setIdentityConfig`)

Pre-seed repo-local identity that LOOKS stagecoach-branded, run, and assert the commit IS stamped with
it (stagecoach passes it through faithfully — correct) — proving stagecoach does not override identity.
Mirror `setIdentityConfig` at `internal/git/committree_test.go:17`:
```go
for _, kv := range []string{"user.name stagecoach agent", "user.email agent@stagecoach.local"} {
    parts := strings.Fields(kv)
    exec.Command("git", "-C", dir, "config", parts[0], parts[1]).Run()
}
```

## 3. The structural guard (mirror `TestRun_NoPlumbing`)

**Precedent:** `internal/hook/exec_test.go:340` — `TestRun_NoPlumbing` reads its own source
(`os.ReadFile("exec.go")`), splits into lines, skips blanks/comments, and fails on
`strings.Contains(line, forbiddenSymbol)`. Generalize this:

**Location:** `internal/git/git_test.go` (a new `TestNoIdentityWritesInProduction`) or
`internal/config/config_test.go`. Git-package is the natural home since `CommitTree` is the
constrained surface.

**Algorithm:**
1. `filepath.WalkDir` over `internal/` and `cmd/` (resolve roots relative to the test's working dir;
   Go test CWD = the package dir, so compute repo root via `runtime.Caller` or walk up to find
   `go.mod`, OR hardcode the two subtrees relative to a known anchor). Prefer walking from a
   repo-root resolved once.
2. Skip any path with `strings.HasSuffix(path, "_test.go")` (legitimate scaffolding) AND skip the
   guard's own file to avoid self-match.
3. For each production `.go` line, after trimming, skip blank + `//` comment lines (mirror
   `TestRun_NoPlumbing`).
4. FAIL if any line matches a forbidden identity-write pattern. Match on the actual call-site shape,
   not bare substrings, to avoid false positives on comments:
   - a `git config` invocation writing identity: the substring `config"` ... `user.name` or
     `user.email` appearing as command args — simplest robust signal: a line containing BOTH a config
     verb AND (`"user.name"` or `"user.email"`). Keep it simple: flag `"user.name"` / `"user.email"`
     as bare string literals in NON-comment production lines (the only legit occurrence today is the
     comment at git.go:520, which is a `//` line and thus skipped). Validate this produces zero hits
     on the current tree before declaring done.
   - any of the six env keys as a string literal: `"GIT_AUTHOR_NAME"`, `"GIT_AUTHOR_EMAIL"`,
     `"GIT_AUTHOR_DATE"`, `"GIT_COMMITTER_NAME"`, `"GIT_COMMITTER_EMAIL"`, `"GIT_COMMITTER_DATE"`.
5. Assert the walk visited > 0 production files (guards against a path-resolution bug silently
   scanning nothing and passing vacuously).
6. **Self-test the guard:** inject a temporary `os.Setenv("GIT_COMMITTER_NAME", "x")`-style or
   `exec.Command("git","config","user.name",...)` line into a throwaway production file (or temporarily
   append to git.go), run the guard, confirm it FIRES, then revert. (PRD §4 verification step.)

## 4. The in-code doc tag (Mode A)

- `internal/git/git.go:518-521` comment → append ` (FR-39a — commit-identity transparency)`.
- `internal/git/git.go:672-679` `CommitTree` doc comment → add one sentence citing FR-39a (e.g.
  "Per FR-39a, no GIT_AUTHOR_*/GIT_COMMITTER_* env is set and no user.name/user.email is written, so
  the commit's author/committer are exactly git's resolved identity — stagecoach is invisible.")

## 5. What NOT to do

- Do NOT add the future opt-in `Co-authored-by: stagecoach` trailer (deferred).
- Do NOT write production code that sets identity (that is precisely what the guard forbids).
- Do NOT make the structural guard match bare `user.name` in COMMENT lines — the only existing
  production occurrence is the explanatory comment at git.go:520, which must keep documenting the
  guarantee. Match non-comment lines only (mirror `TestRun_NoPlumbing`'s comment-skip).