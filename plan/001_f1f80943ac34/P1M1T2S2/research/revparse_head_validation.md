# Research: `RevParseHEAD` Implementation & Test Validation

> **Purpose:** Pin the exact implementation body and test cases for `(*gitRunner).RevParseHEAD`
> (P1.M1.T2.S2), which replaces the panic-stub landed by P1.M1.T2.S1. Built directly on the
> empirically-verified `run()` helper and exit-code table from S1's research; the only NEW work
> here is confirming the delegation logic, the import delta, and the born-vs-unborn test fixtures.
>
> Verification environment: git 2.54.0, go1.26.4-X:nodwarf5 linux/amd64, 2026-06-29.

---

## 1. Inputs (the CONTRACT this subtask consumes — from P1.M1.T2.S1, assume landed as-specified)

The S1 PRP creates `internal/git/git.go` containing, verbatim:

- `gitRunner` struct `{ workDir string }`.
- `func (g *gitRunner) run(ctx context.Context, repo string, args ...string) (stdout string, stderr string, exitCode int, err error)` — fully implemented (LookPath → `-C repo` args → separate buffers → `errors.As(*exec.ExitError)` → **`err=nil` for non-zero exits**, `exitCode=-1` only for LookPath/context/start failures).
- `func (g *gitRunner) RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error)` — currently a **panic-stub** (`panic("… not yet implemented — see P1.M1.T2.S2")`).
- The `Git` interface (with `RevParseHEAD(ctx) (sha string, isUnborn bool, err error)`) and `New(workDir string) Git`.
- `git_test.go` (`package git`) with an `initRepo(t, dir)` helper that runs `git -C dir init` directly via `exec.Command` with a minimal env (PATH, HOME, GIT_*_NAME/EMAIL) — producing an **unborn** repo (zero commits).

**S1 imports:** `bytes, context, errors, fmt, os/exec`. **`strings` is NOT imported yet** — see §3.

## 2. Empirically confirmed git behavior (re-pinned on this exact box)

```
$ git init /tmp/shtest && cd /tmp/shtest
$ git rev-parse HEAD; echo "EXIT=$?"
fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree.
...
HEAD                                    ← stdout = literal "HEAD\n" (NON-EMPTY — the trap)
EXIT=128

$ git -c user.name=t -c user.email=t@t commit --allow-empty -m init
$ git rev-parse HEAD; echo "EXIT=$?"
e3a1a9ffc18cae1155beaa478bca1532afc8b928   ← 40-hex SHA + "\n"
EXIT=0
```

Matches S1's `run_helper_validation.md` §3.1 and `critical_findings.md` FINDING 1 exactly.
**Therefore:** `run(ctx, repo, "rev-parse", "HEAD")` returns, for the unborn case,
`stdout="HEAD\n", stderr=<fatal>, exitCode=128, err=nil` (the S1 G2 invariant: exit 128 is NOT a
Go error). `RevParseHEAD` must key off `exitCode == 128`, **never** `stdout == ""`.

## 3. The verified implementation body (delegates to `run()`)

```go
func (g *gitRunner) RevParseHEAD(ctx context.Context) (sha string, isUnborn bool, err error) {
    stdout, stderr, code, err := g.run(ctx, g.workDir, "rev-parse", "HEAD")
    if err != nil {
        return "", false, err // LookPath miss / context cancel / start failure only (run sets exitCode=-1)
    }
    if code == 128 {
        return "", true, nil // unborn repo — detected via EXIT CODE, not stdout emptiness (FINDING 1)
    }
    if code != 0 {
        return "", false, fmt.Errorf("git rev-parse HEAD: unexpected exit %d: %s", code, strings.TrimSpace(stderr))
    }
    return strings.TrimSpace(stdout), false, nil
}
```

**Why this order of checks matters:** `run()` guarantees `err != nil ⟹ code == -1` (infrastructural
failure) and `err == nil` for every real git exit (0, 128, etc.). So the `err != nil` guard runs
first (cheap, authoritative), then the `code == 128` unborn branch, then the catch-all for any
*other* non-zero exit (shouldn't occur for `rev-parse HEAD` on a healthy repo, but is surfaced
with the trimmed stderr snippet for debuggability — per the architecture's "include stderr in
errors" convention). `code == 0` falls through to the trimmed-SHA return.

### 3.1 The import delta (the ONE non-obvious edit)

`RevParseHEAD` needs `strings.TrimSpace` (born-case stdout has a trailing `"\n"`; the unborn
error path also trims stderr). S1's `git.go` imports `bytes, context, errors, fmt, os/exec` but
**not `strings`**. The implementing agent MUST add `"strings"` to the existing import block. (A
Go compiler error — `undefined: strings` — will flag this immediately, but stating it explicitly
prevents a wasted build-iterate cycle.)

`fmt` is already imported (used by `run`'s LookPath error); no other import changes.

## 4. Test strategy — `internal/git/revparse_test.go` (package git, NEW file)

A separate test file (not appending to S1's `git_test.go`) is chosen deliberately: S1 is landing
**in parallel**, so a distinct file avoids any edit-conflict on `git_test.go`, and per-method test
files are idiomatic Go. Same `package git` → has access to `gitRunner`, `New`, `run`, and the
`initRepo(t, dir)` helper S1 defines in `git_test.go`.

### 4.1 Fixture helper to add (for the born case)

S1's `initRepo` produces an **unborn** repo. For the born case we need a repo with ≥1 commit. Add:

```go
// makeEmptyCommit creates an empty root commit in dir so HEAD becomes "born".
// Uses exec.Command directly (identity via env) — does NOT call the Git interface under test.
func makeEmptyCommit(t *testing.T, dir, msg string) {
    t.Helper()
    cmd := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", msg)
    cmd.Env = append(minGitEnv(),
        "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
        "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
    )
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("makeEmptyCommit(%s): %v\n%s", dir, err, out)
    }
}
```

**`minGitEnv()` note:** S1's `initRepo` builds a minimal env. To avoid depending on S1's private
helper name, `revparse_test.go` defines its own `minGitEnv()` returning `[]string{"PATH=" +
os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}` (the two vars `git init`/`commit` need). If
`initRepo` is instead reused from `git_test.go`, this stays self-contained. The two helpers
(`makeEmptyCommit`, `minGitEnv`) use names that do NOT collide with S1's `initRepo`.

### 4.2 Test cases (all tied to verified behavior)

| Test | Fixture | Call | Assertions |
|---|---|---|---|
| `TestRevParseHEAD_UnbornRepo` | `initRepo` (zero commits) | `RevParseHEAD(ctx)` | `sha==""`, `isUnborn==true`, `err==nil` — **proves exit-128 detection, not string emptiness** |
| `TestRevParseHEAD_BornRepo` | `initRepo` + `makeEmptyCommit` | `RevParseHEAD(ctx)` | `isUnborn==false`, `err==nil`, `sha` matches `^[0-9a-f]{40,64}$` (sha-1=40, sha-256=64) |
| `TestRevParseHEAD_GitBinaryMissing` | any dir; `t.Setenv("PATH","")` | `RevParseHEAD(ctx)` | `err != nil`, message contains `"git binary not found"`, `isUnborn==false`, `sha==""` — proves `run()`'s `err` path is propagated (NOT misread as unborn) |
| `TestRevParseHEAD_ContextCancelled` | any dir; `cancel()` before call | `RevParseHEAD(ctx)` | `err != nil`, `errors.Is(err, context.Canceled)`, `isUnborn==false` — proves ctx.Err() is surfaced (NOT exit 128) |

**The most important assertion** is in `TestRevParseHEAD_UnbornRepo`: `isUnborn == true`. A naive
string-emptiness check would fail here because `stdout == "HEAD\n"` (non-empty). The exit-code
branch (`code == 128`) is the only correct path. The `TestRevParseHEAD_GitBinaryMissing` case is
the guard against regressing `err` into the `isUnborn` branch (LookPath miss → `code == -1`, which
must NOT satisfy `code == 128`).

### 4.3 Deterministic ctx-cancel handling

Cancelling `ctx` **before** calling `RevParseHEAD` makes `cmd.Run()` fail immediately; `run()`
checks `ctx.Err() != nil` (before `errors.As`) and returns `(-1, ctx.Err())`. So `RevParseHEAD`
returns `("", false, context.Canceled)`. Assert via `errors.Is(err, context.Canceled)` (wrapsafe).

## 5. Scope boundaries (do NOT do)

- Do NOT touch any of the other 10 interface methods (they stay panic-stubs until their subtasks).
- Do NOT change `run()`, `New`, `gitRunner`, `Git` interface, or `FileChange`/`StagedDiffOptions`.
- Do NOT add `--verify -q` or `symbolic-ref` (the contract mandates plain `rev-parse HEAD`).
- Do NOT add SHA-format validation to production code (the contract says return trimmed stdout on
  exit 0); the hex regex is a TEST-only sanity check of git's contract.
- Do NOT add deps; `strings` is stdlib. `go.mod`/`go.sum` unchanged.

## 6. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | New test file vs append to `git_test.go` | NEW `revparse_test.go` | Avoids parallel-edit conflict with S1; idiomatic per-method test file. |
| D2 | Reuse S1's `initRepo`? | Yes for the unborn fixture; add `makeEmptyCommit` for born | `initRepo` is the contract; `makeEmptyCommit` is the new born fixture. Names don't collide. |
| D3 | Error message includes stderr? | Yes (trimmed) for the unexpected-exit branch only | Architecture "include stderr snippet" convention; unborn branch returns clean `("", true, nil)`. |
| D4 | SHA validation in code? | No; regex is TEST-only | Contract says return trimmed stdout on exit 0; don't over-engineer. |
| D5 | Cancel-ctx test asserts `errors.Is`? | Yes, not `==` | Wrapsafe across Go versions; `run()` returns `ctx.Err()` unwrapped but `errors.Is` is future-proof. |
