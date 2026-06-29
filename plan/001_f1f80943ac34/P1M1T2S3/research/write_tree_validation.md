# Research: `WriteTree` Implementation & Test Validation

> **Purpose:** Pin the exact implementation body and test cases for `(*gitRunner).WriteTree`
> (P1.M1.T2.S3), which replaces the panic-stub landed by P1.M1.T2.S1. Built directly on the
> empirically-verified `run()` helper (S1) and the exit-code table; the only NEW work here is
> confirming the write-tree success/conflict behavior on this exact box and designing the
> merge-conflict test fixture.
>
> Verification environment: git 2.54.0, go1.26.4-X:nodwarf5 linux/amd64, 2026-06-29.

---

## 1. Inputs (the CONTRACT this subtask consumes — S1 landed, S2 landed on disk)

Verified on-disk state of `internal/git/git.go`:
- `gitRunner` struct `{ workDir string }`, `New(workDir) Git`, and the fully-implemented
  `run(ctx, repo, args...) (stdout, stderr, exitCode, err)` helper (LookPath → `-C repo` →
  separate buffers → `errors.As(*exec.ExitError)` → **`err=nil` for non-zero exits**,
  `exitCode=-1` only for LookPath/context/start failures).
- `RevParseHEAD` is **already real** (S2 landed) — it is the canonical `run()`-delegation pattern
  to mirror: `err`-first guard, then `code` branches, then trimmed-stdout return.
- `WriteTree` is still a **panic-stub**: `panic("gitRunner.WriteTree: not yet implemented — see P1.M1.T2.S3")`.
- **Imports already include `bytes, context, errors, fmt, os/exec, strings`** — S2 added `strings`.
  **Therefore WriteTree needs ZERO import changes** (it only uses `fmt` + `strings`, both present).

Reusable test helpers (all `package git`, must NOT be redeclared):
- `initRepo(t, dir)` — `git_test.go` — produces an **unborn** repo (zero commits), sets identity env.
- `minGitEnv()` — `revparse_test.go` — returns `[]string{"PATH=…", "HOME=…"}`.
- `makeEmptyCommit(t, dir, msg)` — `revparse_test.go` — `git commit --allow-empty -m msg` with identity env.

`git_test.go`'s `TestStubsPanic` currently asserts a panic for these 10 methods (RevParseHEAD was
removed by S2): `WriteTree, CommitTree, UpdateRefCAS, DiffTree, StagedDiff, HasStagedChanges,
RecentMessages, RecentSubjects, CommitCount, AddAll`. **`WriteTree` is still in that list** — making
it real will FAIL `TestStubsPanic` unless the `WriteTree` line is removed (same one-line edit S2 did
for RevParseHEAD; see §5).

## 2. Empirically confirmed `git write-tree` behavior (git 2.54.0, pinned on this box)

```
# CASE A — empty index (unborn repo, nothing staged):
$ git init -q && git write-tree; echo "EXIT=$?"
4b825dc642cb6eb9a060e54bf8d69288fbee4904       ← the canonical empty-tree object id (sha-1)
EXIT=0

# CASE B — staged file (happy path):
$ echo hello > a.txt && git add a.txt && git write-tree; echo "EXIT=$?"
2e81171448eb9f2ee3821e3d447aa6b2fe3ddba1        ← 40-hex tree SHA
EXIT=0

# CASE C — unresolved merge conflict in the index:
$ <create divergent branches, merge → CONFLICT>
$ git write-tree; echo "EXIT=$?"
c.txt: unmerged (626799f0f85326a8c1fc522db584e86cdfccd51f)
c.txt: unmerged (ba2906d0666cf726c7eaadd2cd3db515dedfdf3a)
c.txt: unmerged (e6bfff5c1d0f0ecd501552b43a1e13d8008abc31)
fatal: git-write-tree: error building trees
EXIT=128

# CASE D — after resolving (git add the resolved file):
$ git write-tree; echo "EXIT=$?"
0851b2bf49b2338a6e7b295ad4e8d474726e4fdc
EXIT=0
```

### 2.1 CRITICAL — conflict stderr phrasing differs from the architecture doc

`git_plumbing_reference.md` §1 documented the *representative* conflict stderr as:
```
error: cannot write tree object
fatal: <path>: needs merge
```
**On git 2.54.0 the ACTUAL bytes are different:**
```
<path>: unmerged (<sha>)          ← one line per stage (1/2/3)
fatal: git-write-tree: error building trees
```
- Exit code is **128** (matches the exit-code table).
- The word **"merge"** IS present — inside **"unmerged"** (`un-merge-ed` contains `merge`),
  appearing 3× (once per stage). So the contract's heuristic *"detect 'merge' or 'needs merge' in
  stderr"* STILL MATCHES via the `"merge"` substring (because `strings.Contains(stderr, "merge")`
  hits `"unmerged"`).
- "needs merge" and "cannot write tree object" do **NOT** appear on 2.54.0 — those are older/different
  phrasings. This is exactly why the architecture doc warns: *"stderr byte strings … occasionally drift
  across versions … Never substring-match a message you haven't captured from the actual environment."*

**Design consequence:** do NOT match an exact phrase. The robust signal is **exit code ≠ 0**
(`git_plumbing_reference.md` §1: *"The stable signal is exit code ≠ 0 plus the word 'merge' … on
stderr. Do not rely on a single exact phrase."*). `WriteTree` branches on `code != 0` and returns an
error that names "unresolved merge conflicts" (per the work-item contract) while including the
trimmed stderr (so the real `unmerged`/`error building trees` text is visible for debugging). The
test asserts on the *produced* message containing "unresolved merge conflicts", not on git's stderr
bytes — so it is version-robust.

## 3. The verified implementation body (delegates to `run()`, mirrors RevParseHEAD)

```go
// WriteTree materializes the current index into a tree object and returns its SHA. It is a
// read-only-with-respect-to-refs operation: it writes a tree object to the object store but does
// NOT modify the index or HEAD (PRD §13.2). It is the immutable-snapshot primitive consumed by
// CommitTree (P1.M1.T2.S4) and the rescue protocol (P1.M3.T3).
//
// write-tree fails (non-zero exit, 128 on git 2.x) when the index has unresolved merge conflicts
// (unmerged stage 1/2/3 entries). That is surfaced here as run()'s exitCode != 0 (err stays nil per
// run()'s invariant); the error names "unresolved merge conflicts" and includes the trimmed stderr,
// whose text contains "unmerged"/"error building trees" on a real conflict (git_plumbing_reference
// §1: the stable signal is exit ≠ 0; do NOT match a single exact stderr phrase).
func (g *gitRunner) WriteTree(ctx context.Context) (sha string, err error) {
    stdout, stderr, code, err := g.run(ctx, g.workDir, "write-tree")
    if err != nil {
        return "", err // git binary missing / context cancelled / start failure (run sets code=-1)
    }
    if code != 0 {
        return "", fmt.Errorf("git write-tree: unresolved merge conflicts in index (exit %d): %s", code, strings.TrimSpace(stderr))
    }
    return strings.TrimSpace(stdout), nil
}
```

**Why `code != 0` (not `code == 128`):**
- The work-item contract says verbatim: *"On exit != 0, return a descriptive error mentioning
  'unresolved merge conflicts'."* — `code != 0` matches that language directly.
- `git_plumbing_reference.md` §1 states the stable signal is **exit ≠ 0** (the doc explicitly warns
  the stderr phrasing drifts; the code is the reliable axis).
- `write-tree` has no documented non-zero exit OTHER than the conflict case, so treating any
  non-zero exit as a conflict precondition failure is correct and slightly more future-proof than
  pinning exactly 128. (RevParseHEAD pinned `== 128` because unborn has a distinct 128-only meaning
  and a different success path; write-tree has no such ambiguity.)

**Branch order (err first, then code):** `run()` guarantees `err != nil ⟹ code == -1` and `err == nil`
for every real git exit. So the `err != nil` guard catches LookPath/context/start failures
authoritatively; only then does `code != 0` decide conflict-vs-success. Mirrors RevParseHEAD exactly.

**No import changes:** `fmt` and `strings` are already imported (S2 added `strings`). The body adds
nothing new. (Contrast with S2, whose single non-obvious edit was adding `strings`; here there is none.)

## 4. Test strategy — `internal/git/writetree_test.go` (package git, NEW file)

A separate file (not appending to `git_test.go`) avoids edit conflicts and matches the per-method
test-file convention S1/S2 established. Same `package git` → reuses `initRepo`, `minGitEnv`,
`makeEmptyCommit`, and reaches `gitRunner`/`New`/`run`.

### 4.1 New helper — `makeMergeConflict(t, dir)`

Creates unmerged (stage 1/2/3) index entries so `write-tree` fails. Branch-name-agnostic via
`git checkout -` (previous-branch), which is robust to `init.defaultBranch` being `main` or `master`.

```go
func makeMergeConflict(t *testing.T, dir string) {
    t.Helper()
    idEnv := []string{
        "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
        "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
    }
    runGit := func(args ...string) {
        cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
        cmd.Env = append(minGitEnv(), idEnv...) // reuse revparse_test.go's minGitEnv (no redeclare)
        if out, err := cmd.CombinedOutput(); err != nil {
            t.Fatalf("makeMergeConflict %v: %v\n%s", args, err, out)
        }
    }
    writeFile := func(name, body string) {
        if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
            t.Fatalf("write %s: %v", name, err)
        }
    }
    // base commit on the default branch with one tracked file
    writeFile("conflict.txt", "base\n"); runGit("add", "conflict.txt"); runGit("commit", "-m", "base")
    // divergent branch edits the same file differently
    runGit("checkout", "-b", "side"); writeFile("conflict.txt", "side\n"); runGit("commit", "-am", "side-change")
    // return to the ORIGINAL branch (branch-name-agnostic: "checkout -" = previous branch)
    runGit("checkout", "-"); writeFile("conflict.txt", "main\n"); runGit("commit", "-am", "main-change")
    // merge side → content conflict (merge exits non-zero; that is the EXPECTED outcome, not a failure)
    merge := exec.Command("git", "-C", dir, "merge", "side"); merge.Env = append(minGitEnv(), idEnv...)
    if err := merge.Run(); err == nil {
        t.Fatalf("makeMergeConflict: expected merge to conflict, but it succeeded cleanly")
    }
}
```

Verified: after this, `git ls-files -u` reports 3 unmerged entries and `git write-tree` exits 128.

### 4.2 Test cases (all tied to verified behavior)

| Test | Fixture | Call | Assertions | What it proves |
|---|---|---|---|---|
| `TestWriteTree_StagedFiles` | `initRepo` + write+`git add` a file | `WriteTree(ctx)` | `err==nil`; `sha` matches `^[0-9a-f]{40,64}$` | Happy path: index → 40-hex TREE_SHA, trimmed |
| `TestWriteTree_EmptyIndex` | `initRepo` only (no commits, nothing staged) | `WriteTree(ctx)` | `err==nil`; `sha == "4b825dc642cb6eb9a060e54bf8d69288fbee4904"` | Edge: works on fresh/unborn repo; empty index → canonical empty-tree SHA |
| `TestWriteTree_MergeConflict` | `initRepo` + `makeMergeConflict` | `WriteTree(ctx)` | `err != nil`; `err.Error()` contains `"unresolved merge conflicts"` | **The core failure mode**: conflict → descriptive error, exit ≠ 0 |
| `TestWriteTree_GitBinaryMissing` | `t.Setenv("PATH","")`; `New(t.TempDir())` | `WriteTree(ctx)` | `err != nil`; contains `"git binary not found"` | `run()`'s err path propagated (NOT misread as conflict/success) |
| `TestWriteTree_ContextCancelled` | `cancel()` before call | `WriteTree(ctx)` | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced (not exit 0/128) |

**The most important assertion** is in `TestWriteTree_MergeConflict`: the error message contains
"unresolved merge conflicts". The empty-tree constant in `TestWriteTree_EmptyIndex`
(`4b825dc642cb6eb9a060e54bf8d69288fbee4904`) is the universally-known sha-1 empty-tree object id;
the test repos on this box are sha-1 by default (`git init` → `extensions.objectFormat` unset).

## 5. The `TestStubsPanic` edit (the ONE non-conflicting touch to `git_test.go`)

`git_test.go`'s `TestStubsPanic` still includes the line:
```go
assertPanics(t, "WriteTree", func() { _, _ = g.WriteTree(ctx) })
```
Once `WriteTree` is real (no panic), `assertPanics` fails (`expected panic, but did not panic`).
**Resolution (same as S2 did for RevParseHEAD):** remove that single line from `TestStubsPanic`.
This is an allowed exception to "don't touch git_test.go" because it is the direct, required
consequence of implementing WriteTree, and the alternative (a permanently-failing suite) is worse.
Document the edit in the commit message. After the edit, `TestStubsPanic` covers the remaining 9
stubs (CommitTree, UpdateRefCAS, DiffTree, StagedDiff, HasStagedChanges, RecentMessages,
RecentSubjects, CommitCount, AddAll).

## 6. Scope boundaries (do NOT do)

- Do NOT touch any of the other 9 interface methods (they stay panic-stubs until their subtasks).
- Do NOT change `run()`, `New`, `gitRunner`, the `Git` interface, `FileChange`, or `StagedDiffOptions`.
- Do NOT add any import (`fmt`/`strings` already present).
- Do NOT add tree-SHA format validation to production code (the contract says return trimmed stdout on
  exit 0); the hex regex is TEST-only.
- Do NOT substring-match a specific git stderr phrase in production code (version-fragile); rely on
  `exitCode != 0` and just name "unresolved merge conflicts" in the message.
- Do NOT add deps; everything is stdlib. `go.mod`/`go.sum` unchanged.

## 7. Decisions log

| # | Point | Decision | Why |
|---|---|---|---|
| D1 | Branch on `code == 128` or `code != 0`? | `code != 0` | Matches contract verbatim ("On exit != 0 …"); reference doc §1 says exit ≠ 0 is the stable signal; write-tree has no other non-zero exit. |
| D2 | Match a specific stderr phrase? | No — message names "unresolved merge conflicts", includes trimmed stderr | git 2.54.0 says "unmerged"/"error building trees", NOT "needs merge"/"cannot write tree object" (doc §1 warns phrasing drifts). Stable axis is the exit code. |
| D3 | New test file vs append to git_test.go? | NEW `writetree_test.go` | Avoids edit conflicts; matches S1/S2 per-method file convention. |
| D4 | Reuse helpers? | Yes — `initRepo`, `minGitEnv`, `makeEmptyCommit` | All `package git`; reusing avoids redeclaration compile errors. New helper `makeMergeConflict` has a distinct name. |
| D5 | How to make the conflict fixture branch-name-agnostic? | `git checkout -` (previous branch) | Robust to `init.defaultBranch` = main vs master; verified working. |
| D6 | Include the empty-index test? | Yes | Strengthens the suite; proves write-tree works on a fresh repo (root-commit flow) and documents the empty-tree constant. Not contract-mandated but cheap and valuable. |
| D7 | Hardcode the empty-tree SHA? | Yes, with a comment | `4b825dc642cb6eb9a060e54bf8d69288fbee4904` is the universally-known sha-1 empty-tree id; test repos are sha-1 here. Comment explains how to update for sha-256. |
| D8 | Edit git_test.go TestStubsPanic? | Yes — remove the WriteTree line | Required consequence of making WriteTree real (mirrors S2's RevParseHEAD removal); one-line, non-conflicting. |
