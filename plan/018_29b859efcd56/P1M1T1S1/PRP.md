name: "P1.M1.T1.S1 — FR-39a commit-identity transparency: behavioral test + structural guard + doc tag"
description: >
  Assert the commit-identity transparency invariant (PRD §9.9 FR-39a / §20.2) the production code
  ALREADY upholds: stagecoach never sets/overrides git author/committer identity, so every commit it
  creates is indistinguishable in metadata from a hand-made commit. This subtask writes (A) a
  behavioral success-path test in internal/generate/invariants_test.go, (B) a self-source-scan
  structural guard in internal/git/git_test.go that fails if any production code introduces an
  identity write, and (C) two FR-39a doc-tag edits in internal/git/git.go. NO production behavior
  change — runWithInput already deliberately never sets cmd.Env.

---

## Goal

**Feature Goal**: Convert FR-39a from a prose guarantee into an executable, self-defending regression
net — a behavioral test proving a stagecoach commit carries exactly git's resolved identity (no
stagecoach-branded author/email, no Co-Authored-By/Generated-by trailer), and a structural guard that
fails the build if any future production code writes a git identity (the six `GIT_AUTHOR_*`/`GIT_COMMITTER_*`
env keys, or a `user.name`/`user.email` config write).

**Deliverable**: (A) `TestCommitIdentityTransparency` in `internal/generate/invariants_test.go` (two
`t.Run` sub-cases: baseline identity + transparency-without-authorship); (B) `TestNoIdentityWritesInProduction`
in `internal/git/git_test.go` (walks `internal/`+`cmd/`, skips `_test.go` + `//` comments, fails on
forbidden quoted-literal identity tokens, asserts >0 files visited); (C) two FR-39a doc-tag edits in
`internal/git/git.go` (the runWithInput comment at 518-521 + the CommitTree doc at 672-679).

**Success Definition**:
- `go test ./internal/generate/ ./internal/git/ ./internal/config/` is green.
- The behavioral test proves a stagecoach commit's `%an <%ae> | %cn <%ce>` exactly equals git's
  resolved identity, and `%B` exactly equals the stub's canned message (no trailer).
- The structural guard reports ZERO forbidden hits on the current tree (verified), AND is proven to
  FIRE by a temporary injection then reverted (manual verification step).
- The two doc tags are present and do NOT self-trip the guard (they are on `//` lines).

## Why

- **FR-39a (P0, defensive/permanent)**: A stagecoach commit must be indistinguishable in metadata
  from a hand-made commit — no `"stagecoach agent"` author, no branded email, no co-author/machine-made
  trailer. The guarantee is defensive: it exists precisely so no future "helpful" change (an identity
  fallback, a branding trailer, a default identity in `config init`) can ever make a stagecoach commit
  announce itself. The production code already upholds it (runWithInput never sets `cmd.Env`); this
  subtask makes that fact *executable and self-defending* so the invariant cannot silently regress.
- The field note in FR-39a (a coding agent once wrote `user.name = "stagecoach agent"` into a repo's
  local config, and stagecoach faithfully inherited it) is exactly the failure mode the structural
  guard exists to catch at the *source* — stagecoach itself must never write such a key.

## What

**User-visible behavior**: None (tests + comments only). Commit behavior is unchanged.

**Technical change (three deliverables, no production logic change):**
- **A (behavioral):** new `TestCommitIdentityTransparency` (success path) in invariants_test.go.
- **B (structural):** new `TestNoIdentityWritesInProduction` self-source-scan guard in git_test.go.
- **C (doc tag):** append ` (FR-39a — commit-identity transparency)` to the git.go:518-521 comment;
  add one FR-39a sentence to the CommitTree doc comment (git.go:672-679).

### Success Criteria
- [ ] `TestCommitIdentityTransparency` passes (both sub-cases)
- [ ] `TestNoIdentityWritesInProduction` passes (zero forbidden hits; >0 files visited)
- [ ] Guard proven to FIRE on a temporary injection (then reverted) — manual verification
- [ ] Behavioral test proven to FIRE on a temporary divergent identity (then reverted) — manual verification
- [ ] Two FR-39a doc tags present in internal/git/git.go
- [ ] `go test ./internal/generate/ ./internal/git/ ./internal/config/` green; `make test`/`make lint` pass

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact success-path precedent to mirror, the shared fixture helpers, the structural-guard precedent, the self-trip-avoidance reasoning, the doc-tag line numbers, and the manual fire-verification steps are all enumerated below (verified by reading source).

### Documentation & References

```yaml
- file: internal/generate/generate_test.go
  why: "The success-path precedent + shared fixture helpers. TestCommitStaged_Success (84) is the exact invocation template. initRepo (26) ALREADY sets user.name Test/user.email test@example.com. writeFile (34)/stageFile (43)/headSHA (49)/commitRaw (55)/gitOut (61)/runGit (66) are the package-wide helpers."
  pattern: "repo:=t.TempDir(); initRepo(t,repo); commitRaw(t,repo,\"initial\"); writeFile+stageFile; bin:=stubtest.Build(t); m:=stubtest.Manifest(bin,stubtest.Options{Out:...}); cfg:=config.Defaults(); res,err:=CommitStaged(ctx,Deps{Git:git.New(repo),Manifest:m},cfg). Success ⇒ err==nil, res.CommitSHA is the landed SHA."
  gotcha: "initRepo already seeds identity — sub-case (a) resolves expected from the fixture directly. Sub-case (b) OVERRIDES with stagecoach-branded values via the setIdentityConfig-style loop."

- file: internal/generate/invariants_test.go
  why: "The §20.2 home. TestInvariants (159) is table-driven FAILURE-path-only (every row expects err!=nil). Add TestCommitIdentityTransparency as a SEPARATE function (success path), NOT a table row."
  gotcha: "Do NOT add a row to TestInvariants — that table's assertInvariants is failure-path (index/HEAD/tree immutability). The identity assertion is a different, success-path check."

- file: internal/git/committree_test.go
  why: "setIdentityConfig (17) — the repo-local identity helper to mirror for sub-case (b). Writes user.name/user.email via a strings.Fields loop over 'user.name X'/'user.email Y'."
  pattern: "for _, kv := range []string{\"user.name stagecoach agent\",\"user.email agent@stagecoach.local\"} { parts:=strings.Fields(kv); exec.Command(\"git\",\"-C\",dir,\"config\",parts[0],parts[1]).Run() }"

- file: internal/hook/exec_test.go
  why: "TestRun_NoPlumbing (340) — the self-source-scan precedent. Reads a .go file, splits lines, skips blank + //-prefixed, fails on strings.Contains of forbidden tokens."
  pattern: "trimmed:=strings.TrimSpace(line); if trimmed==\"\"||strings.HasPrefix(trimmed,\"//\") { continue }; for f:=range forbidden { if strings.Contains(trimmed,f) { t.Errorf(...) } }"
  gotcha: "Generalize from reading one file to filepath.WalkDir over internal/+cmd/. Match on QUOTED string literals (\"GIT_AUTHOR_NAME\", \"user.name\") so prose in comments can't false-positive."

- file: internal/git/git.go
  why: "The doc-tag targets + the structural fact. runWithInput (522) never sets cmd.Env (the invariant). Comment to tag at 518-521. CommitTree doc at 672-679, CommitTree body at 680."
  gotcha: "The doc-tag edits land on // lines ⇒ skipped by the guard's comment-skip (no self-trip). The CommitTree FR-39a sentence mentions GIT_AUTHOR_*/user.name as PROSE (unquoted) ⇒ does not match the quoted-literal forbidden tokens."

- file: internal/stubtest
  why: "The stub provider IS the mock for the LLM agent. stubtest.Build(t) compiles the stub binary; stubtest.Manifest(bin, stubtest.Options{Out: \"feat: add widget\"}) returns a provider.Manifest whose stub emits Out once and exits 0. The canned Out is the ONLY content allowed in the commit message (no trailer stagecoach could add)."

- docfile: plan/018_29b859efcd56/architecture/fr39a_findings.md
  why: "The precise code locations + the grep proof the invariant holds today + the test-scaffolding allow-list (all identity writes are in _test.go)."
- docfile: plan/018_29b859efcd56/architecture/test_patterns.md
  why: "The exact patterns to mirror (TestCommitStaged_Success invocation, setIdentityConfig loop, TestRun_NoPlumbing scan, the §20.2 identity query)."
- docfile: plan/018_29b859efcd56/P1M1T1S1/research/verification_deltas.md
  why: "The verified anchors, the self-trip-avoidance layers, the forbidden-token shape, the repo-root resolution, and the manual fire-verification steps. READ THIS before editing."
```

### Current Codebase tree (relevant slice)

```bash
internal/generate/
  generate.go           # CommitStaged (188); Result.CommitSHA (72) — UNCHANGED
  generate_test.go      # SHARED helpers: initRepo(26)/writeFile(34)/stageFile(43)/headSHA(49)/commitRaw(55)/gitOut(61); TestCommitStaged_Success(84) precedent
  invariants_test.go    # TestInvariants(159, failure-path table) + NEW TestCommitIdentityTransparency (success path)
internal/git/
  git.go                # runWithInput(522, never sets cmd.Env); CommitTree(680); comment@518-521 + doc@672-679 (doc-tag targets)
  git_test.go           # NEW TestNoIdentityWritesInProduction (self-source-scan guard)
  committree_test.go    # setIdentityConfig(17) — the transparency sub-case helper to mirror
internal/stubtest/      # stubtest.Build / stubtest.Manifest(bin, Options{Out:...}) — the LLM-agent mock
internal/hook/exec_test.go  # TestRun_NoPlumbing(340) — the self-source-scan precedent
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (no production change): this subtask writes TESTS + doc comments ONLY. runWithInput
//   (git.go:522) already deliberately never sets cmd.Env — that IS the invariant. Do NOT alter
//   runWithInput/CommitTree/commit behavior. Do NOT write any production identity-setting code.

// CRITICAL (self-trip avoidance — 3 layers keep the tree green):
//   1. The guard lives in internal/git/git_test.go ⇒ _test.go suffix ⇒ SKIPPED by the walk.
//   2. The guard's own forbidden-token list is in that _test.go file ⇒ skipped by layer 1.
//   3. The doc-tag edits in git.go are on // lines ⇒ skipped by the comment-skip. AND matching is on
//      QUOTED string literals ("GIT_AUTHOR_NAME","user.name") so prose mentions in comments never
//      false-positive even if a skip were absent. VALIDATE zero hits on the current tree before done.

// GOTCHA (invariants table is failure-only): TestInvariants (invariants_test.go:159) is a FAILURE-path
//   table (every row expects err!=nil). FR-39a is a SUCCESS path (a commit lands) ⇒ a SEPARATE
//   TestCommitIdentityTransparency function, NOT a table row.

// GOTCHA (initRepo already seeds identity): initRepo sets user.name "Test"/user.email "test@example.com".
//   Sub-case (a) resolves expected from the fixture directly. Sub-case (b) OVERRIDES via setIdentityConfig-style loop.

// GOTCHA (repo-root resolution): Go test CWD = package dir (internal/git/). Walk UP from os.Getwd()
//   until go.mod is found ⇒ repo root. Then filepath.WalkDir <root>/internal and <root>/cmd. Assert
//   visited >0 production .go files (a path bug scanning nothing would pass vacuously).

// GOTCHA (forbidden-token shape): match the six env keys as QUOTED literals "GIT_AUTHOR_NAME",
//   "GIT_AUTHOR_EMAIL","GIT_AUTHOR_DATE","GIT_COMMITTER_NAME","GIT_COMMITTER_EMAIL","GIT_COMMITTER_DATE"
//   (catches os.Setenv(...) / cmd.Env=[...] call sites), AND "user.name"/"user.email" quoted literals
//   in non-comment lines (catches exec.Command("git","config","user.name",...)). The ONLY existing
//   production user.name/user.email occurrence is the // comment at git.go:520 (skipped).
```

## Implementation Blueprint

### Data models and structure

No struct/type/production-code changes. Two new test functions + two doc-comment edits. The
`Deps`/`Result`/`CommitStaged` API and `runWithInput`/`CommitTree` bodies are unchanged.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: CREATE TestCommitIdentityTransparency in internal/generate/invariants_test.go (DELIVERABLE A)
  - ADD a new function alongside TestInvariants (NOT a table row). Two t.Run sub-cases:
  - SUB-CASE (a) "baseline identity":
      repo:=t.TempDir(); initRepo(t,repo); commitRaw(t,repo,"initial")
      writeFile(t,repo,"staged.txt","x"); stageFile(t,repo,"staged.txt")
      // resolve expected identity from the fixture (initRepo set Test/test@example.com)
      wantName:=gitOut(t,repo,"config","user.name"); wantEmail:=gitOut(t,repo,"config","user.email")
      bin:=stubtest.Build(t); m:=stubtest.Manifest(bin,stubtest.Options{Out:"feat: add widget"})
      cfg:=config.Defaults()
      res,err:=CommitStaged(context.Background(),Deps{Git:git.New(repo),Manifest:m},cfg)
      if err!=nil { t.Fatalf("CommitStaged: %v",err) }; if res.CommitSHA=="" { t.Fatal("empty CommitSHA") }
      got:=gitOut(t,repo,"log","--format=%an <%ae> | %cn <%ce>","-n1",res.CommitSHA)
      want:=fmt.Sprintf("%s <%s> | %s <%s>",wantName,wantEmail,wantName,wantEmail)
      if got!=want { t.Errorf("identity mismatch:\n got %q\nwant %q",got,want) }
      // message is EXACTLY the canned text — no Co-Authored-By/Generated-by trailer
      if msg:=strings.TrimSpace(gitOut(t,repo,"log","--format=%B","-n1",res.CommitSHA)); msg!="feat: add widget" {
          t.Errorf("commit message = %q; want exactly %q (no trailer appended)",msg,"feat: add widget") }
  - SUB-CASE (b) "transparency-without-authorship":
      repo:=t.TempDir(); initRepo(t,repo); commitRaw(t,repo,"initial")
      writeFile+stageFile "staged.txt"
      // override with a stagecoach-LOOKING identity (mirror setIdentityConfig, committree_test.go:17)
      for _,kv := range []string{"user.name stagecoach agent","user.email agent@stagecoach.local"} {
          parts:=strings.Fields(kv); exec.Command("git","-C",repo,"config",parts[0],parts[1]).Run() }
      bin:=stubtest.Build(t); m:=stubtest.Manifest(bin,stubtest.Options{Out:"feat: pass-through check"})
      cfg:=config.Defaults()
      res,err:=CommitStaged(...); if err!=nil { t.Fatalf(...) }
      got:=gitOut(t,repo,"log","--format=%an <%ae> | %cn <%ce>","-n1",res.CommitSHA)
      want:="stagecoach agent <agent@stagecoach.local> | stagecoach agent <agent@stagecoach.local>"
      if got!=want { t.Errorf("stagecoach-looking identity was NOT passed through:\n got %q\nwant %q",got,want) }
  - IMPORTS: context, fmt, os/exec, strings (most already imported in the package's _test.go files;
    check goimports/gofmt). Mirror the existing TestCommitStaged_Success import set.
  - DEPENDENCIES: none (pure test; CommitStaged already exists).

Task 2: CREATE TestNoIdentityWritesInProduction in internal/git/git_test.go (DELIVERABLE B)
  - ADD the guard. Algorithm:
      // 1. resolve repo root by walking UP from os.Getwd() to the dir containing go.mod
      root:=findRepoRoot(t)  // helper: for d:=getwd; ; d=filepath.Dir(d) { if fileExists(d/go.mod) { return d } }
      forbidden := []string{
        `"GIT_AUTHOR_NAME"`,`"GIT_AUTHOR_EMAIL"`,`"GIT_AUTHOR_DATE"`,
        `"GIT_COMMITTER_NAME"`,`"GIT_COMMITTER_EMAIL"`,`"GIT_COMMITTER_DATE"`,
        `"user.name"`,`"user.email"`,
      }
      visited:=0
      for _, sub := range []string{"internal","cmd"} {
        filepath.WalkDir(filepath.Join(root,sub), func(p string, d fs.DirEntry, err error) error {
          if err!=nil || d.IsDir() { return err }
          if !strings.HasSuffix(p, ".go") { return nil }
          if strings.HasSuffix(p, "_test.go") { return nil }          // legit scaffolding
          if filepath.Base(p) == "git_test.go" { return nil }          // belt-and-suspenders: skip self
          data, rerr := os.ReadFile(p); if rerr!=nil { return rerr }
          visited++
          for i, line := range strings.Split(string(data), "\n") {
            trimmed := strings.TrimSpace(line)
            if trimmed=="" || strings.HasPrefix(trimmed, "//") { continue }   // mirror TestRun_NoPlumbing
            for _, f := range forbidden { if strings.Contains(trimmed, f) {
              t.Errorf("%s:%d: forbidden identity-write token %s in: %s", p, i+1, f, trimmed) } }
          }
          return nil
        })
      }
      if visited==0 { t.Fatal("walk visited 0 production .go files — path-resolution bug (guard passes vacuously)") }
  - Use fs.DirEntry/fs.WalkDir (Go 1.16+; the module is go 1.22). Check os/exec is NOT needed here.
  - NOTE: the forbidden tokens are QUOTED literals (include the " in the Go raw/interpreted string) so
    they match os.Setenv("GIT_AUTHOR_NAME",...) call sites, not prose comments.
  - DEPENDENCIES: none.

Task 3: EDIT internal/git/git.go — the two FR-39a doc tags (DELIVERABLE C, Mode A)
  - runWithInput comment (lines 518-521): append a clause/tag. On the block's last line (520 or 521)
    add ` (FR-39a — commit-identity transparency)` so the guarantee is grep-discoverable. Keep it a // line.
  - CommitTree doc comment (lines 672-679): add one sentence, e.g.
    "Per FR-39a, no GIT_AUTHOR_*/GIT_COMMITTER_* env is set and no user.name/user.email config is
     written, so the commit's author/committer are exactly git's resolved identity — stagecoach is
     invisible in commit metadata."
  - DO NOT change any code — only the two // comment blocks.
  - DEPENDENCIES: none (but verify AFTER Task 2 that the tags do not self-trip the guard — they are //
    lines so the comment-skip handles them; the quoted-literal matching also protects prose mentions).

Task 4: VERIFY (manual fire-tests, then revert — do NOT commit the injections)
  - PROVE THE GUARD FIRES: temporarily append e.g. `_ = os.Setenv("GIT_COMMITTER_NAME", "x")` OR a
    `exec.Command("git","config","user.name",...)` line to a PRODUCTION file (e.g. git.go, NON-comment),
    run `go test ./internal/git/ -run TestNoIdentityWritesInProduction`, confirm it FAILLS reporting
    the token, then REVERT the injection. (This proves the guard is not vacuous.)
  - PROVE THE BEHAVIORAL TEST FIRES: temporarily seed a divergent identity mid-run (or temporarily set
    cmd.Env with GIT_COMMITTER_NAME in runWithInput), run `go test ./internal/generate/ -run TestCommitIdentityTransparency`,
    confirm the identity-mismatch assertion FAILS, then REVERT. (This proves the assertion is real.)
  - DEPENDENCIES: Tasks 1-3.
```

### Implementation Patterns & Key Details

```go
// PATTERN: the success-path invocation (mirror TestCommitStaged_Success, generate_test.go:84)
bin := stubtest.Build(t)
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add widget"})
cfg := config.Defaults()
res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)

// PATTERN: the §20.2 identity query (PRD verbatim)
got := gitOut(t, repo, "log", "--format=%an <%ae> | %cn <%ce>", "-n1", res.CommitSHA)
want := fmt.Sprintf("%s <%s> | %s <%s>", wantName, wantEmail, wantName, wantEmail)

// PATTERN: the self-source-scan guard (mirror TestRun_NoPlumbing, generalized to a walk)
trimmed := strings.TrimSpace(line)
if trimmed == "" || strings.HasPrefix(trimmed, "//") { continue }   // skip blank + comments
for _, f := range forbidden { if strings.Contains(trimmed, f) { t.Errorf(...) } }
// forbidden tokens are QUOTED literals: `"GIT_AUTHOR_NAME"`, `"user.name"`, … (match call sites, not prose)
```

### Integration Points

```yaml
NO production code / API / config / build changes. Tests + two // comment blocks.

TESTS:
  - internal/generate/invariants_test.go — +TestCommitIdentityTransparency (2 sub-cases)
  - internal/git/git_test.go — +TestNoIdentityWritesInProduction (walk guard)

DOC COMMENTS (Mode A — the doc edit rides with the work):
  - internal/git/git.go:518-521 — append "(FR-39a — commit-identity transparency)"
  - internal/git/git.go:672-679 — add one FR-39a sentence to the CommitTree doc

UNCHANGED: runWithInput (still never sets cmd.Env); CommitTree; CommitStaged; Result; Deps; config.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
go build ./...
go vet ./...
gofmt -l internal/generate/ internal/git/
# Expected: empty.
make lint
# Expected: zero errors.
```

### Level 2: Unit Tests (Component Validation)

```bash
# The three packages the task names
go test ./internal/generate/ ./internal/git/ ./internal/config/ -v
# Expected: ALL pass, incl. new TestCommitIdentityTransparency (both sub-cases) and
#           TestNoIdentityWritesInProduction (zero forbidden hits, >0 files visited).

# Targeted runs
go test ./internal/generate/ -run TestCommitIdentityTransparency -v
go test ./internal/git/      -run TestNoIdentityWritesInProduction -v

# Whole suite (race)
make test
# Expected: ALL pass.
```

### Level 3: Integration Testing (System Validation)

```bash
# (The behavioral test IS the integration test — real git, real commit-tree, stub provider.)
# No service to start / curl. The manual fire-verification (Level 4) is the additional proof.
```

### Level 4: Creative & Domain-Specific Validation

```bash
# PROVE THE GUARD FIRES (then revert — do NOT commit):
#   temporarily add to internal/git/git.go (a NON-comment line):  _ = os.Setenv("GIT_COMMITTER_NAME","x")
cp internal/git/git.go /tmp/git.go.bak
echo '_ = os.Setenv("GIT_COMMITTER_NAME","x")' >> internal/git/git.go   # TEMP injection
go test ./internal/git/ -run TestNoIdentityWritesInProduction 2>&1 | grep -q 'GIT_COMMITTER_NAME' && echo "GUARD FIRES (good)" || echo "FAIL: guard did not fire"
cp /tmp/git.go.bak internal/git/git.go && rm /tmp/git.go.bak            # REVERT
go test ./internal/git/ -run TestNoIdentityWritesInProduction           # back to PASS (zero hits)

# PROVE THE BEHAVIORAL TEST FIRES (then revert):
#   temporarily seed a divergent identity in runWithInput's cmd.Env (or git config mid-run) so the
#   committed author differs from the resolved expected; run TestCommitIdentityTransparency and confirm
#   the identity-mismatch assertion FAILS; then revert. (Exact injection left to the implementer; the
#   point is to prove the assertion is non-vacuous.)

# Grep guard: the doc tags are present and grep-discoverable
grep -n "FR-39a — commit-identity transparency" internal/git/git.go
grep -n "Per FR-39a" internal/git/git.go
# Expected: 2 hits (the runWithInput comment tag + the CommitTree doc sentence).

# Grep guard: ZERO production identity writes remain (the invariant the guard asserts)
grep -rn "GIT_AUTHOR\|GIT_COMMITTER" --include="*.go" internal/ cmd/ pkg/ | grep -v "_test.go" || echo "OK: zero production env-identity hits"
grep -rn '"user\.name"\|"user\.email"' --include="*.go" internal/ cmd/ pkg/ | grep -v "_test.go" || echo "OK: zero production config-identity-write hits"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean
- [ ] `go vet ./...` clean
- [ ] `gofmt -l internal/generate/ internal/git/` empty
- [ ] `make lint` zero errors
- [ ] `go test ./internal/generate/ ./internal/git/ ./internal/config/` green
- [ ] `make test` (race) all pass

### Feature Validation
- [ ] `TestCommitIdentityTransparency` (a) baseline: commit identity == git's resolved; message == canned exactly
- [ ] `TestCommitIdentityTransparency` (b) transparency: stagecoach-looking identity passed through verbatim
- [ ] `TestNoIdentityWritesInProduction`: zero forbidden hits; >0 files visited
- [ ] Guard proven to FIRE on a temporary injection (then reverted)
- [ ] Behavioral test proven to FIRE on a temporary divergent identity (then reverted)

### Scope-Boundary Validation
- [ ] NO production behavior change (runWithInput still never sets cmd.Env)
- [ ] NO row added to TestInvariants (separate function, success path)
- [ ] NO future Co-authored-by trailer added (deferred)
- [ ] Doc edits limited to the two // comment blocks in internal/git/git.go (README/docs sweep is P1.M1.T2.S1)
- [ ] Doc tags do NOT self-trip the guard (// lines; quoted-literal matching)

### Code Quality
- [ ] Tests follow the package's helper/t.Helper() conventions
- [ ] Guard matches quoted-literal call-site tokens (no false positives on prose comments)
- [ ] Guard asserts >0 files visited (no vacuous pass on a path bug)

---

## Anti-Patterns to Avoid

- ❌ Don't change `runWithInput`/`CommitTree`/commit behavior — the invariant ALREADY holds; this subtask only tests + documents it. Any production identity-setting code is precisely what the guard forbids.
- ❌ Don't add a row to `TestInvariants` — that table is failure-path-only (every row expects err!=nil). FR-39a is a success path; use a separate `TestCommitIdentityTransparency`.
- ❌ Don't match bare `user.name`/`GIT_AUTHOR` substrings in the guard — match QUOTED string literals (`"GIT_AUTHOR_NAME"`, `"user.name"`) so the explanatory comment at git.go:520 (and the new FR-39a doc sentence) don't false-positive. Skip `//` lines too (belt-and-suspenders).
- ❌ Don't forget the `_test.go` skip in the guard — every legitimate identity write (setIdentityConfig, fixture initRepo, decompose/e2e harnesses) lives in `_test.go`. The guard must catch PRODUCTION regressions only.
- ❌ Don't let the guard pass vacuously — assert it visited >0 production `.go` files, and manually prove it FIRES via a temporary injection (then revert).
- ❌ Don't add the future opt-in `Co-authored-by: stagecoach` trailer — explicitly deferred by FR-39a.
- ❌ Don't edit README.md/docs/*.md here — the changeset-level docs sweep is P1.M1.T2.S1. S1's doc edits are ONLY the two code comments in git.go.
- ❌ Don't re-seed identity in sub-case (a) — `initRepo` already sets Test/test@example.com; resolve expected from the fixture directly. Only sub-case (b) overrides.
- ❌ Don't commit the temporary fire-test injections — they are verification only; always revert.

---

## Confidence Score: 9/10

One-pass success is very high: the success-path precedent (`TestCommitStaged_Success`), the
self-source-scan precedent (`TestRun_NoPlumbing`), the shared fixture helpers, and the exact doc-tag
line numbers are all verified, and the self-trip-avoidance is worked out (3 layers: `_test.go` skip +
`//` comment-skip + quoted-literal matching). The -1 is for the repo-root resolution in the guard
(Go test CWD = package dir; a fragile walk-up-to-go.mod could mis-resolve on an unusual layout) —
mitigated by the `visited > 0` assertion that fails the test loudly rather than passing vacuously, and
by the manual fire-injection verification that proves the guard is non-vacuous.