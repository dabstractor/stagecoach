# Research — P1.M3.T4.S1: internal/git/log.go — CommitCount / RecentMessages / RecentSubjects

## Task scope (from work item CONTRACT DEFINITION)

- INPUT: `git.Git` (P1.M3.T1.S1) — the shipped, unexported `(g *Git) run(args ...string) (string, error)` exec seam.
- LOGIC:
  - `CommitCount() (int, error)` — `git rev-list --count HEAD`; return **0 on a rootless/unborn repo (NO error)**.
  - `RecentMessages(n int) (string, error)` — `git log --format='---%n%B' -<n>` **RAW** (the prompt layer trims blanks + caps lines; **do NOT trim here**).
  - `RecentSubjects(n int) ([]string, error)` — `git log --format=%s -<n>` split into lines, **trimmed**, newest-first.
- OUTPUT: history for prompt examples (P1.M4.T1.S1) + multi-line detect + the dedupe set (P1.M6.T1.S2).

## Verified git behaviors (git 2.54.0, REAL binary, temp repos)

### CommitCount: `git rev-list --count HEAD`
- Repo WITH history (3 commits) → stdout `"3\n"`, exit 0. Trim + strconv.Atoi.
- UNBORN repo (git init, no commits) → **exit 128**, stderr:
  `fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree.`
  → detect via `ee.Code == 128 && strings.Contains(ee.Stderr, "unknown revision")` → return `(0, nil)`.
  This is the IDENTICAL detection pattern plumbing.go's `RevParseHEAD` uses for the unborn case (verified in internal/git/plumbing.go).
- Not-a-repo → exit 128 with different stderr (`"not a git repository"`) → surface the typed `*ExitError` as-is (genuine failure).

### RecentMessages: `git log --format='---%n%B' -<n>` (RAW, no trim)
- Repo with commits `[feat: first\n\nFirst body line.\nSecond body line., fix: second, chore: third]`, `-2`:
  ```
  ---
  chore: third

  ---
  fix: second commit

  ```
  i.e. `---` separator, `%n` (newline), `%B` (raw body incl. embedded blank lines), one block per commit, **newest-first**, trailing newline included. Return this **RAW** (the prompt layer does `sed '/^$/d' | head -100`).
- UNBORN repo → **exit 128**, stderr: `fatal: your current branch 'main' does not have any commits yet`
  → detect via `ee.Code == 128 && strings.Contains(ee.Stderr, "does not have any commits yet")` → return `("", nil)`.
  NOTE: match on `"does not have any commits yet"` (NOT the branch name — it varies main/master/etc.).
- **Arg form:** passing `--format=---%n%B` as a literal arg (NO shell quotes) yields BYTE-IDENTICAL output to the shell `'---%n%B'` form. Verified. The `%` is not special in a Go double-quoted string, so `g.run("log", "--format=---%n%B", fmt.Sprintf("-%d", n))` is correct.
- `n <= 0`: `git log -0` → empty stdout, exit 0 (naturally empty; no special guard needed). Negative `n` would produce `--N` (git rejects, exit 128) but no caller passes negative n (prompt layer uses 20/50). Pass n verbatim as `-<n>` via `fmt.Sprintf("-%d", n)`.

### RecentSubjects: `git log --format=%s -<n>` (split into lines, trimmed, newest-first)
- Repo with commits above, `-2` → stdout:
  ```
  chore: third
  fix: second commit
  ```
  → split on `"\n"`, TrimSpace each, skip empties (the trailing newline yields a final empty element) → `["chore: third", "fix: second commit"]`, **newest-first**, in order.
- UNBORN repo → **exit 128**, same `"does not have any commits yet"` stderr → return `(nil, nil)` (empty slice, no error).
- `n` larger than commit count (e.g. `-50` on a 3-commit repo) → returns all 3 subjects, exit 0 (no error).

### seedCommits ordering (harness helper in gittestutil_test.go)
- `seedCommits(msgs)` commits `msgs[0], msgs[1], ..., msgs[k]` in order. The LAST committed is the newest (HEAD). `git log` returns newest-first, so:
  - `RecentSubjects(n)` returns `[msgs[k], msgs[k-1], ..., msgs[0]]` (truncated to n).
  - `CommitCount()` returns `len(msgs)`.
  This matches the contract's "subjects returned newest-first in order; counts match".

## Detection-pattern consistency (CRITICAL)

`internal/git/plumbing.go` `RevParseHEAD` already establishes the package's "unborn repo is not an error" pattern:
```go
var ee *ExitError
if errors.As(runErr, &ee) && ee.Code == 128 && strings.Contains(ee.Stderr, "unknown revision") {
    return "", false, nil   // unborn — NOT an error
}
```
- `CommitCount` mirrors this EXACTLY (`rev-list --count HEAD` produces the identical "unknown revision" stderr on an unborn repo).
- `RecentMessages` / `RecentSubjects` use the sibling detection (`"does not have any commits yet"`) because `git log` on an unborn repo emits a different message than `rev-list`/`rev-parse`. Verified empirically above.

## Test design (MOCKING scenarios from the contract, white-box package git + REAL git via S2 harness)

1. `TestCommitCount_MatchesSeededCount` — seed N known commits, assert `CommitCount() == N`.
2. `TestCommitCount_UnbornReturnsZeroNoError` — unborn repo, assert `(0, nil)`.
3. `TestRecentSubjects_NewestFirstInOrder` — seed known subjects `[s0,s1,s2]`, assert `RecentSubjects(n) == [s2,s1,s0]` (newest-first), each trimmed, in order; and a `n` smaller than history truncates to the n newest.
4. `TestRecentSubjects_UnbornReturnsEmpty` — unborn repo, assert empty slice + nil error.
5. `TestRecentMessages_RawFormat` — seed a commit with a MULTI-LINE body, assert `RecentMessages(n)` returns the RAW `---`-separated `%B` output with the body blank lines INTACT (no trimming) — proves the "raw, do not trim here" contract.
6. `TestRecentMessages_UnbornReturnsEmpty` — unborn repo, assert `("", nil)`.

## Dependency / scope boundaries

- DEPENDS ON (shipped, DO NOT modify): `internal/git/git.go` (`Git`, `New`, unexported `run`, `ExitError`), `internal/git/gittestutil_test.go` (`newTempRepo`, `writeFileStage`, `seedCommits`, `mustRun`). The S2 harness is the test seam (NOT reinvented).
- Sibling precedent: `internal/git/plumbing.go` (method file: plain `package git` line + leading file-level comment; builds `[]string` args; calls `g.run`; surfaces typed `*ExitError`), `internal/git/diff.go` (same posture). `internal/git/plumbing_test.go` + `internal/git/diff_test.go` = white-box test precedent (stdlib `testing` only, composes S2 harness, drives REAL binary, one behavior per `Test*`).
- This task ADDS `internal/git/log.go` + `internal/git/log_test.go`. The sibling `stage.go` (T4.S2: HasStagedChanges/AddAll) is a SEPARATE work item — do NOT implement it here.
- stdlib-only imports: `errors`, `fmt`, `strconv`, `strings`. NO go-git, NO testify, NO go.mod/go.sum change, NO `go mod tidy`.
- git.go OWNS the `// Package git` doc — log.go uses a PLAIN `package git` line + a leading file-level comment (mirror plumbing.go/diff.go).

## Consumers (downstream, NOT implemented here)

- `internal/prompt/examples.go` (M4.T1.S1): reads `CommitCount()` (>1 ⇒ build examples) and `RecentMessages(20)` (the raw `---%n%B` stream the prompt layer trims via `sed '/^$/d' | head -100` and scans for multi-line via the awk heuristic, reference_impl.md §6).
- `internal/generate/dedupe.go` (M6.T1.S2): the dedupe set = `RecentSubjects(50)` (reference §1/§D: `git log --format=%s -50`).

## References
- reference_impl.md §1 (pipeline: `commit_count = git rev-list --count HEAD || 0`; `examples = git log --format='---%n%B' -20 | sed '/^$/d' | head -100`; dedupe `subject in git log --format=%s -50`) and §6 (multi-line awk heuristic over the `---`-separated examples).
- external_deps.md §D (verified git 2.54 commands: commit count `|| 0`; last 20 msgs `git log --format='---%n%B' -20` trim blanks + head -100; last 50 subjects `git log --format=%s -50` dedupe set).
- decisions.md §2 (the Git method surface incl. CommitCount/RecentMessages/RecentSubjects).
