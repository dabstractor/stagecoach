# Research — P1.M3.T1.S2: internal/git/gittestutil_test.go — temp-repo harness

## 1. Contract (verbatim from work item)
- Helpers in a `_test.go` (or testutil build-tagged file):
  - `newTempRepo(t)` → temp dir + `git init -q` + deterministic `user.email`/`user.name` config, returns `*Git`.
  - `seedCommits(g, msgs []string)` → creates commits with given FULL messages (for history / multi-line tests).
  - `writeFileStage(g, path, content)` → writes a file + `git add`.
- Self-test asserts the harness bootstraps a repo with ≥2 commits and `rev-parse HEAD` succeeds.
- MOCKING: uses the REAL git binary (that is the point — catches plumbing regressions); only the repo is temporary (`t.Cleanup` removes dir).
- INPUT: `git.Git` (P1.M3.T1.S1 — DONE & shipped: `New(dir)`, `(g *Git) run(args ...)`, `ExitError`).
- OUTPUT: shared harness for all `internal/git/*_test.go` and the generate integration tests (P1.M6.T3).
- RESEARCH NOTE: PRD §20.1 layer 2; decisions.md §8 (shared harness / integration seam); plan_overview §8 (test infra stands as its own unit).

## 2. S1 API surface the harness builds on (verified against shipped internal/git/git.go)
- `func New(dir string) (*Git, error)` — resolves git via `exec.LookPath("git")`; `dir==""` accepted.
- `func (g *Git) run(args ...string) (string, error)` — **UNEXPORTED**; builds `exec.Command(g.git, args...)`, `cmd.Dir=g.dir`, captures stdout+stderr, returns typed `*ExitError` on non-zero exit, RAW stdout on success.
- `type Git struct { dir string; git string }` — BOTH fields UNEXPORTED. `g.dir` = working dir (empty ⇒ inherit cwd).
- `type ExitError struct { Args []string; Code int; Stderr string }` + `Error()`.
- ⇒ The harness file MUST be **white-box `package git`** to call `g.run` (lowercase) and read `g.dir` (lowercase). This is the same posture as the shipped `internal/git/git_test.go`.

## 3. Packaging / visibility facts (CRITICAL — why white-box + why downstream tests must follow)
- A `_test.go` file is compiled ONLY into the test binary of its own package; its symbols (even exported-looking ones) are **not importable** by any other package. So a helper defined in `gittestutil_test.go` is usable ONLY by other `_test.go` files **in the same directory** (`internal/git/`).
- A `package git_test` (external/black-box) `_test.go` file in the same dir CANNOT see unexported helpers or `g.run`/`g.dir`. Therefore ALL `internal/git/*_test.go` that use this harness MUST be **`package git` (white-box)**. (The shipped `git_test.go` is already white-box — consistent.)
- This harness is **not** a build-tagged production helper; it is a plain `_test.go` so it is automatically excluded from `go build`/the shipped binary. No build tag needed (git is always present on the host per system_context.md §2).

## 4. Helper signature decision (deviation flagged + justified)
- Contract wrote `seedCommits(g, msgs []string)` and `writeFileStage(g, path, content)` WITHOUT a `*testing.T`. But a helper that runs `g.run("commit",...)` and hits a git error MUST surface it; ignoring it silently corrupts every downstream test, and `panic` is non-idiomatic.
- ⇒ Canonical signatures take `tb testing.TB` as the FIRST parameter for ALL THREE helpers (`newTempRepo`, `seedCommits`, `writeFileStage`). This lets each call `tb.Helper()` (failures point at the CALLER, not the helper) and `tb.Fatalf`/`tb.Logf` on git/exec errors. This is the dominant Go test-harness idiom and is forward-compatible (downstream T2/T3/T4 PRPs will reference THIS harness's API).
- `newTempRepo` returns ONLY `*Git` (NOT the dir): because the harness is white-box it can read `g.dir` directly when it needs a filesystem path (e.g. `writeFileStage` writes under `g.dir`). Keeping the return type `*Git` matches the contract verbatim and keeps the API minimal.

## 5. Verified git behaviors for the harness (host, git 2.54.0) — all run via `g.run`-equivalent
| command | context | result |
|---|---|---|
| `git init -q` | empty temp dir, `cmd.Dir=dir` | creates `.git`, exit 0, no stdout |
| `git config user.email X` / `git config user.name Y` | fresh repo | sets REPO-LOCAL config (no `--global`); readback returns exactly X/Y |
| `git rev-parse HEAD` | unborn branch (fresh init, 0 commits) | **exit 128**, stderr "fatal: ambiguous argument 'HEAD'" → this is why `RevParseHEAD` returns an `ok bool`; the harness self-test must seed ≥1 commit before asserting rev-parse succeeds |
| `git rev-parse HEAD` | repo with ≥1 commit | stdout = full 40-char SHA + "\n", exit 0 |
| `git add <relpath>` | repo, `cmd.Dir=g.dir` | stages the file at the relative path; exit 0 |
| `git commit -q -m "<full msg>"` | staged content, repo-local identity set | creates commit, advances HEAD, exit 0. A SINGLE `-m` arg containing newlines preserves the FULL multi-line/paragraph message (verified via `git log --format=%B`). No `-F`/stdin needed (and `g.run` has `cmd.Stdin=nil` anyway) |
| `git rev-list --count HEAD` | repo with N commits | stdout = decimal N + "\n", exit 0 → parse with `strconv.Atoi(strings.TrimSpace(out))` |
- ⇒ `seedCommits` uses porcelain `git commit` (NOT plumbing commit-tree): it is TEST SETUP, porcelain updates HEAD automatically, and it pairs with `writeFileStage` so each commit has distinct staged content. Each iteration writes a UNIQUE file (e.g. `file<i>.txt`) so successive commits are non-empty and produce a deterministic linear history for RecentMessages/RecentSubjects/CommitCount consumers.

## 6. Deterministic-config / cross-machine gotchas
- **Identity**: `git commit` FAILS ("Author identity unknown") unless `user.email`/`user.name` are set. The contract's required "deterministic user.email/user.name config" satisfies this. Set REPO-LOCAL (`g.run("config","user.email",...)`) so it never leaks into `--global`.
- **GPG signing (defensive addition)**: a developer host with `git config --global commit.gpgsign true` would make every `git commit` in the harness try to sign (prompt/fail in CI) and flake the whole suite. In a FRESH temp repo `commit.gpgsign` is UNSET (inherits unset→false) on this host, but for a harness whose entire purpose is deterministic cross-machine reproducibility, set `g.run("config","commit.gpgsign","false")` REPO-LOCAL. This is a small, justified extension beyond the literal "user.email/user.name" wording; it has zero effect on production (production repos inherit the user's real config — we never touch gpg there). Flagged as a deliberate, documented deviation.

## 7. Temp-dir lifecycle
- Use `tb.TempDir()` (Go 1.15+, stdlib `testing`): returns a dir that the testing package REMOVES automatically at the end of the test via an internal `t.Cleanup`. So `newTempRepo` needs NO explicit `t.Cleanup(os.RemoveAll(...))` — `tb.TempDir()` already does it. (Contract's "t.Cleanup removes dir" is SATISFIED by `tb.TempDir()`.) Do NOT use `os.MkdirTemp` + manual cleanup (that reinvents `tb.TempDir`).
- Each test gets its OWN isolated temp repo (no shared state, no `t.Parallel` collisions, no `InitRef` leakage). Repos are independent → tests are order-independent.

## 8. Validation gates (verified working on host)
- `go build ./internal/git/` → builds non-test files (git.go); confirms package integrity (does NOT compile `_test.go`).
- `go vet ./internal/git/` → **the gate that COMPILES `_test.go` files** (vet includes tests); this is the real harness-compile check.
- `test -z "$(gofmt -l internal/git/)"` → gofmt-clean (gofmt -l always exits 0 → wrap in `test -z`, proven idiom).
- `go test ./internal/git/` → compiles the harness + runs the self-test (+ the shipped S1 tests still pass). Confirms newTempRepo bootstraps ≥2 commits and `rev-parse HEAD` succeeds.
- `go test ./...` → whole-module integrity (Makefile `test` target green; internal/ui + internal/provider unaffected).
- Baseline before this task: `go test ./...` GREEN (git/provider/ui).

## 9. Dependency boundary / scope discipline (anti-regression)
- S2 depends on S1 ONLY (DONE). It does NOT implement T2/T3/T4 plumbing/diff/log/stage methods — it only provides the SETUP helpers those tests will call.
- DO NOT import any new external dep (go-git, testify, etc.); stdlib `testing` + the imports already used in git.go (bytes/errors/strings/os/exec) + `os`/`path/filepath`/`strconv`/`fmt` for the helpers.
- DO NOT touch main.go, Makefile, go.mod, go.sum, internal/ui, internal/provider, or the shipped git.go/git_test.go. DO NOT run `go mod tidy`.
- DOCS = Mode A: godoc on each helper explaining it drives the REAL git binary over a temp repo (PRD §20.1 layer 2) and citing decisions.md §8. No README/docs/providers created.

## 10. Consumer preview (forward compatibility — what T2/T3/T4 tests will do with this harness)
- T2 (plumbing) tests: `g := newTempRepo(t); seedCommits(t, g, []string{"a\n\nbody","b"}); ... g.RevParseHEAD(); g.WriteTree(); g.CommitTree(...); g.UpdateRefCAS(...)`.
- T3 (diff) tests: `g := newTempRepo(t); writeFileStage(t, g, "dir/f.go", "..."); g.StagedDiff(cfg)`.
- T4 (log) tests: `g := newTempRepo(t); seedCommits(t, g, multiLineMsgs); g.CommitCount(); g.RecentMessages(5); g.RecentSubjects(5)`.
- M6.T3 (generate integration): builds a temp repo via this harness + injects a stub provider.
⇒ The helpers (`newTempRepo`/`seedCommits`/`writeFileStage`) must be general enough for ALL of these: `writeFileStage` must `MkdirAll` the parent dir (T3 nests files under subdirs); `seedCommits` must create distinct content per commit (T4 history); `newTempRepo` must leave an unborn repo (T2 root-commit tests need `RevParseHEAD` ok=false).
