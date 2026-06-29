# P1.M1.T3.S4 Research — RecentSubjects (duplicate detection)

## §1. Contract recap (from work item + landed interface)

The work item specifies ONE method, owned by P1.M1.T3.S4:

- **`RecentSubjects(ctx, n int) ([]string, error)`** — `git log --format=%s -<n>`; parse output into a
  `[]string` (one subject per line); **return empty slice on unborn repo**. Output feeds the dedupe
  check (P1.M3.T2), which builds a set/map for O(1) exact-match lookup against a freshly-generated
  subject.

### Interface signature — ALREADY landed, NO conflict

Unlike T3.S3 (which had to reconcile a `maxLines` conflict), `RecentSubjects`'s signature in the
landed `Git` interface (`internal/git/git.go`, declared by P1.M1.T2.S1) EXACTLY matches the work-item
description — **no reconciliation needed**:

```go
// RecentSubjects returns up to n most-recent commit subjects (first line) for duplicate
// detection. Callers must short-circuit when isUnborn.
RecentSubjects(ctx context.Context, n int) (subjects []string, err error)
```

The panic-stub on disk is:

```go
func (g *gitRunner) RecentSubjects(ctx context.Context, n int) ([]string, error) {
	panic("gitRunner.RecentSubjects: not yet implemented — see P1.M1.T3.S4")
}
```

**Resolution:** replace the stub body with the real implementation. Keep the signature byte-for-byte.

## §2. THE key design distinction — why `\n` split (NOT `%x00` NUL) is correct here

This is the single most important finding for this subtask, and it is the reason `RecentSubjects` is
substantially SIMPLER than its sibling `RecentMessages` (T3.S3):

| Aspect | `RecentMessages` (T3.S3) | `RecentSubjects` (T3.S4) |
|---|---|---|
| Placeholder | `%B` (full body) | `%s` (subject = first line) |
| Records contain newlines? | **YES** (bodies span multiple lines) | **NO** (`%s` is single-line by git's definition) |
| Delimiter-collision risk? | **YES** — a body may contain a markdown horizontal rule `---` (FINDING 9) | **NO** — a subject is exactly one line; `---` inside a subject stays on its own line |
| Split strategy | `strings.Split(out, "\x00")` (NUL, cannot occur in object content) | `strings.Split(out, "\n")` (newline — the natural record separator) |
| Line cap needed? | **YES** (`maxRecentMessageLines=100`) — multi-line bodies can blow the prompt budget | **NO** — each subject is one line; caller bounds `n` (FR31: 50); at most 50 short lines |
| Purpose | Style examples (prompt) | Dedupe set membership |

### Empirical proof (git 2.54.0 on this box)

```
$ git commit --allow-empty -m "fix: handle --- edge" && git log --format=%s -1 | cat -A
fix: handle --- edge$                                  ← ONE line; the "---" does NOT start a new record

$ git commit --allow-empty -m $'subj with --- \n body line' && git log --format=%s -1 | cat -A
subj with ---  body line$                              ← %s flattens to a single line (no embedded \n)
$ git log --format=%B -1 | cat -A                       ← contrast: %B (body) DOES span lines
subj with ---$
 body line$
```

**Conclusion:** `git log --format=%s -<n>` emits exactly `<subject>\n` per commit, with a single
trailing newline. Splitting on `"\n"`, trimming each element, and dropping empties (incl. the final
trailing-empty from the terminal newline) yields a clean `[]string`. **FINDING 9's NUL delimiter is
NOT needed and MUST NOT be imported here** — it exists solely to disambiguate `%B` bodies. Using
`%x00` here would be cargo-culting T3.S3 without understanding WHY T3.S3 needed it.

### Consequence: NO new constant, NO new import

- RecentMessages (T3.S3) added `maxRecentMessageLines = 100` and the `strconv` import. RecentSubjects
  needs NEITHER:
  - No line cap → no constant.
  - Uses only `strings` (Split/TrimSpace — already imported) and `fmt` (`Sprintf("-%d", n)`,
    `Errorf` — already imported). **Zero new imports.**

## §3. Empirical findings (verified against git 2.54.0)

All run via `git -C <repo> ...` (mirrors `run()`'s `-C repo` flag).

| Scenario | stdout | exit | → return |
|---|---|---|---|
| unborn (init only, 0 commits) | (none; "fatal: your current branch ... does not have any commits yet") | **128** | `(nil, nil)` |
| 3 commits, ask for 50 | `docs: third\nfix: second one\nfeat: first commit\n` | 0 | 3 subjects, newest-first |
| 3 commits, ask for 2 | `docs: third\nfix: second one\n` | 0 | 2 subjects (git honors `-<n>`) |
| non-repo dir | (none; "fatal: not a git repository") | **128** | `(nil, nil)` — indistinguishable from unborn (inherited from RevParseHEAD S2) |

**Branch semantics (identical to T3.S3's RecentMessages):**
- `run()` returns `err != nil` → `(nil, err)` (git missing / ctx cancelled / start failure; code=-1).
- `code == 128` → `(nil, nil)` — unborn repo (NOT an error). Mirrors RevParseHEAD S2.
- `code != 0` (non-128) → wrapped error.
- `code == 0` → split on `"\n"`, TrimSpace, drop empties, return.

## §4. Design decisions

- **D1 (split strategy): `\n`, not `\x00`.** Justified in §2. The `%s` placeholder guarantees
  one-line-per-commit, making `\n` the natural and safe delimiter. NUL is overkill and would
  obscure the simpler correct model.

- **D2 (no line cap).** Subjects are single-line and the caller bounds `n` (FR31=50). There is no
  unbounded-growth risk that would justify a cap (unlike multi-line `%B` bodies). Adding a cap would
  be dead code defending against an impossible input.

- **D3 (n <= 0 defensive guard).** Mirror RecentMessages (T3.S3 D7): `if n <= 0 { return nil, nil }`
  before the git call, avoiding undefined `git log -0` behavior. The caller passes 50 (FR31); the
  guard is cheap defensive coding.

- **D4 (TrimSpace each line).** git's `%s` is already clean, but the trailing terminal newline
  produces a trailing empty element after Split. TrimSpace + `if s == "" { continue }` handles it and
  also any genuinely-empty subject (`git commit --allow-empty-message` edge case).

- **D5 (mirror T3.S3's branch structure).** err → code 128 → code != 0 → parse. Identical to
  RecentMessages/RevParseHEAD for reviewer consistency.

## §5. Parallel-execution / scope boundaries

- **With T3.S3 (running concurrently):** T3.S3 edits git.go's import block (adds `strconv`), adds
  `maxRecentMessageLines`, replaces `RecentMessages`+`CommitCount` stubs, and removes the
  `RecentMessages`+`CommitCount` lines from `git_test.go`'s `TestStubsPanic`. THIS subtask edits
  git.go's `RecentSubjects` stub region (a DIFFERENT method, far from imports and from the other two
  bodies) and removes the `RecentSubjects` line from `TestStubsPanic` (a DIFFERENT assertPanics
  line). **Non-overlapping.** This subtask adds NO imports — T3.S3 is the only one touching the
  import block. After both land: `TestStubsPanic` covers only `AddAll` (T3.S5's remaining stub).

- **Out of scope (do NOT implement):** `AddAll` (T3.S5); the dedupe set-build (P1.M3.T2 — consumes
  the `[]string` this returns); the prompt builder (P1.M3.T1); the orchestrator (P1.M3.T4); the CLI.
  The `Git` interface, `gitRunner`, `run()`, `runWithInput`, `New`, and all other landed methods are
  byte-identical to their landed forms.

## §6. Test reuse — NO new helpers

- `initRepo(t, dir)` — git_test.go (S1). Creates a minimal repo.
- `makeEmptyCommit(t, dir, msg)` — revparse_test.go (S2). Creates an empty commit with `msg`
  (preserves embedded newlines via `-m`, so a subject-with-`---` or subject+body fixture needs no
  new helper).
- Both are `package git`, in scope for the new test file. **Do NOT redeclare** (compile error).

## §7. Test matrix (9 functions, `recentsubjects_test.go`, package git)

| Test | Fixture | Key assertion | Proves |
|---|---|---|---|
| `TestRecentSubjects_UnbornRepo` | `initRepo` (0 commits) | `err==nil && len==0` | exit 128 ⇒ empty (D-branch) |
| `TestRecentSubjects_ReturnsSubjects` | 3 commits | `len==3`; newest-first order; each has NO `"\n"` | single-line guarantee of `%s` |
| `TestRecentSubjects_NExceedsCommits` | 2 commits, call `(ctx, 50)` | `len==2 && err==nil` | git returns only what exists; FR31's n=50 works |
| `TestRecentSubjects_SubjectOnlyExcludesBody` | commit `feat: x\n\nbody text here` | returned subject `== "feat: x"` (no body) | `%s` vs `%B` distinction |
| `TestRecentSubjects_MarkdownHRInSubject` | commit `fix: handle --- edge` | `len==1`; subject contains `---` intact (NOT split) | `\n` split is safe for `%s` (contrast FINDING 9) |
| `TestRecentSubjects_ZeroOrNegativeN` | born repo; call `(ctx, 0)` and `(ctx, -5)` | `(nil, nil)`; no error | n<=0 defensive guard (D3) |
| `TestRecentSubjects_NotARepo` | plain dir (no `initRepo`) | `err==nil && len==0` | exit 128 ⇒ empty (inherited indistinguishability) |
| `TestRecentSubjects_GitBinaryMissing` | `t.Setenv("PATH","")` | `err` contains "git binary not found" | `run()` err path propagated |
| `TestRecentSubjects_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` | ctx.Err() surfaced (not exit code) |
