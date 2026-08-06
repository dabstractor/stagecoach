# Research Notes — P1.M1.T1.S1 (FR-39a commit-identity transparency: test + guard + doc tag)

Verification against the CURRENT working tree. The task description + `architecture/fr39a_findings.md`
+ `architecture/test_patterns.md` are accurate. These notes record the exact verified anchors.

## VERIFIED — the invariant already holds (this subtask writes TESTS + a doc tag, NOT production logic)
- grep `GIT_AUTHOR|GIT_COMMITTER` across internal/ cmd/ pkg/ (excl `_test.go`) → **ZERO hits** ✅
- grep `user.name|user.email` across internal/ cmd/ pkg/ (excl `_test.go`) → **only the comment at
  internal/git/git.go:520** ✅
- `runWithInput` (git.go:522) builds `exec.Cmd` and never assigns `cmd.Env` → child inherits parent
  environment → git resolves author/committer itself. `CommitTree` (git.go:680) calls runWithInput.
  This is the single structural fact making FR-39a hold. DO NOT change commit behavior.

## VERIFIED — the success-path precedent to mirror (internal/generate/generate_test.go:84)
`TestCommitStaged_Success` is the exact template for the behavioral test's invocation:
```go
repo := t.TempDir(); initRepo(t, repo); commitRaw(t, repo, "initial")
writeFile(t, repo, "staged.txt", "x"); stageFile(t, repo, "staged.txt")
bin := stubtest.Build(t)
m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add widget"})
cfg := config.Defaults()
res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
// success ⇒ err == nil, res.CommitSHA is the landed SHA (generate.go:72)
```
It already checks `gitOut(t, repo, "log", "--format=%B", "-n1", res.CommitSHA)` — the exact message
query FR-39a needs. The identity query is `gitOut(t, repo, "log", "--format=%an <%ae> | %cn <%ce>", "-n1", res.CommitSHA)`.

## VERIFIED — fixture helpers (package generate, generate_test.go)
- `initRepo(t, dir)` (:26) → `git init` + `config user.name "Test"` + `config user.email "test@example.com"`.
  **KEY: initRepo ALREADY sets identity.** So sub-case (a) "baseline" resolves the expected identity
  from the fixture directly (Test / test@example.com) — NO re-seed needed. Sub-case (b) OVERRIDES with
  stagecoach-branded values.
- `writeFile(t,dir,name,body)` (:34), `stageFile(t,dir,name)` (:43), `headSHA(t,dir)` (:49),
  `commitRaw(t,dir,msg)` (:55), `gitOut(t,dir,args...)` (:61), `runGit(t,dir,args...)` (:66).
All are `t.Helper()`-wrapped and package-wide reusable from any internal/generate/*_test.go.

## VERIFIED — invariants_test.go is the §20.2 home, but the table is FAILURE-path-only
`TestInvariants` (invariants_test.go:159) is table-driven; every row drives CommitStaged expecting
`err != nil` then asserts index/HEAD/tree immutability via `assertInvariants`. FR-39a is a SUCCESS
path (a commit lands), so it MUST be a SEPARATE `TestCommitIdentityTransparency` function alongside
`TestInvariants` — NOT a row in the failure table. Same shared helpers; different assertion.

## VERIFIED — the transparency-without-authorship helper to mirror (internal/git/committree_test.go:17)
`setIdentityConfig(t, dir)`:
```go
for _, kv := range []string{"user.name Test", "user.email test@example.com"} {
    parts := strings.Fields(kv)
    exec.Command("git", "-C", dir, "config", parts[0], parts[1]).Run()
}
```
Sub-case (b) mirrors this with `"user.name stagecoach agent"` / `"user.email agent@stagecoach.local"`.

## VERIFIED — the structural-guard precedent (internal/hook/exec_test.go:340 TestRun_NoPlumbing)
```go
src, _ := os.ReadFile("exec.go")
lines := strings.Split(string(src), "\n")
for i, line := range lines {
    trimmed := strings.TrimSpace(line)
    if trimmed == "" || strings.HasPrefix(trimmed, "//") { continue }   // ← skip blank + comments
    for f := range forbidden { if strings.Contains(trimmed, f) { t.Errorf(...) } }
}
```
Generalize: `filepath.WalkDir` over internal/ + cmd/ .go files; skip `_test.go` (legit scaffolding) +
the guard's own file; skip blank + `//`-prefixed lines; FAIL on forbidden **quoted string-literal**
tokens in non-comment lines.

## VERIFIED — the doc-tag targets (internal/git/git.go)
- **Comment to tag — lines 518-521** (exact text confirmed):
  `// Identity: cmd.Env is NOT set here, so the child inherits the parent environment. Production`
  `// callers commit AS the configured user (git resolves user.name/user.email from config/env);`
  `// tests set repo-local user.name/user.email via git config (see committree_test.go).`
  → append ` (FR-39a — commit-identity transparency)` (to the last line of that block, or as a
  trailing clause). It is a `//` line ⇒ skipped by the guard's comment-skip (no self-trip).
- **CommitTree doc comment — lines 672-679** → add one FR-39a sentence. It lives in a `//` block ⇒
  skipped by the guard. The sentence will mention `GIT_AUTHOR_*`/`user.name` as PROSE (unquoted) ⇒
  does NOT match the quoted-literal forbidden tokens even if the skip were absent.

## KEY ONE-PASS CONSIDERATIONS

### A. Self-trip avoidance (the guard must not fire on its own tree)
Three layers keep the current tree green:
1. The guard lives in `internal/git/git_test.go` → `_test.go` suffix ⇒ SKIPPED.
2. The guard's own forbidden-token list (`"GIT_AUTHOR_NAME"`, `"user.name"`, …) is in that _test.go
   file ⇒ skipped by layer 1.
3. The doc-tag edits in git.go are on `//` lines ⇒ skipped by the comment-skip. Match on **quoted
   string literals** (`"GIT_AUTHOR_NAME"`, `"user.name"`) so prose mentions in comments never
   false-positive even without the skip.

### B. Forbidden-token shape (match call sites, not comments)
- The six env keys as quoted literals: `"GIT_AUTHOR_NAME"`, `"GIT_AUTHOR_EMAIL"`, `"GIT_AUTHOR_DATE"`,
  `"GIT_COMMITTER_NAME"`, `"GIT_COMMITTER_EMAIL"`, `"GIT_COMMITTER_DATE"` — catches
  `os.Setenv("GIT_COMMITTER_NAME", …)` / `cmd.Env = []string{"GIT_AUTHOR_NAME=…"}`.
- `"user.name"` / `"user.email"` quoted literals in non-comment lines — catches
  `exec.Command("git","config","user.name",…)`. The ONLY existing production occurrence is the
  `//` comment at git.go:520 (skipped) ⇒ guard reports ZERO hits today. VALIDATE this before done.

### C. Repo-root resolution for the walk
Go test CWD = package dir (internal/git/). Resolve repo root ONCE by walking UP from `os.Getwd()`
until a `go.mod` is found; then `filepath.WalkDir` over `<root>/internal` and `<root>/cmd`. Assert the
walk visited > 0 production `.go` files (guards against a path bug passing vacuously). Skip any path
with `strings.HasSuffix(p, "_test.go")`.

### D. The behavioral test's two sub-cases (t.Run)
- (a) "baseline identity": initRepo sets Test/test@example.com. Resolve expected = those. Run stub
  `Out:"feat: add widget"`. Assert err==nil, CommitSHA!="". Compare
  `git log -1 --format='%an <%ae> | %cn <%ce>' <SHA>` == "Test <test@example.com> | Test <test@example.com>".
  ALSO `git log -1 --format=%B <SHA>` TrimSpace == "feat: add widget" exactly (no trailer appended).
- (b) "transparency-without-authorship": override via the setIdentityConfig-style loop to
  "stagecoach agent"/"agent@stagecoach.local". Run. Assert the commit IS stamped with those EXACT
  values (proving stagecoach passes a stagecoach-LOOKING identity through rather than overriding).

### E. Verification the guard FIRES (manual, not committed)
Temporarily inject e.g. `os.Setenv("GIT_COMMITTER_NAME","x")` or a `git config user.name` call into a
production file (or temporarily into git.go), run the guard, confirm it FAILS, then REVERT. The task
explicitly requires this "prove it fires" step. Likewise prove the behavioral test fires by temporarily
seeding a divergent identity mid-run (or setting cmd.Env) and confirming the assertion fails, then revert.

## SCOPE BOUNDARIES (sibling subtask — do NOT implement here)
- **P1.M1.T2.S1**: README.md + docs/how-it-works.md sweep for any FR-39a documentation surface.
  S1's doc edits are ONLY the two code comments in internal/git/git.go (Mode A).
- Do NOT: change commit behavior / runWithInput / CommitTree logic; add the future opt-in
  Co-authored-by trailer; write any production code that sets identity (that is what the guard forbids).