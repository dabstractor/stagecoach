# P1.M1.T3.S3 Research — CommitCount & RecentMessages

## §1. Contract recap (from work item + landed interface)

The work item specifies two methods, both owned by P1.M1.T3.S3:

- **`CommitCount(ctx) (int, error)`** — `git rev-list --count HEAD`; **returns 0 on unborn**.
- **`RecentMessages(ctx, n int, maxLines int) ([]string, error)`** — `git log --format='%x00%B' -<n>`,
  split on NUL, trim, drop empty, cap total lines at maxLines (100).

### ⚠️ THE interface-signature conflict (decision D1)

The work-item description's `RecentMessages(ctx, n int, maxLines int)` **CONFLICTS** with the
already-landed `Git` interface (declared by P1.M1.T2.S1, present on disk in `internal/git/git.go`):

```go
RecentMessages(ctx context.Context, n int) (messages []string, err error)
```

The interface is **FIXED** — it is the contract every caller and every sibling subtask (S2, T3.S1,
T3.S2) relies on and explicitly protects ("byte-identical to its landed form"). Changing the
signature would break the interface, `New`, the stub, and the `TestStubsPanic` line. **RESOLUTION
(D1): implement the `maxLines=100` cap as an internal package constant** (PRD FR11 fixes it at 100 —
it is NOT caller-configurable in v1):

```go
const maxRecentMessageLines = 100 // PRD §9.3/FR11: ≤100 lines total across style examples
```

The signature stays `RecentMessages(ctx context.Context, n int) ([]string, error)`. This satisfies
the work-item LOGIC (cap at 100 lines) WITHOUT touching the interface. The `n` parameter IS the
count passed to `git log -<n>`.

## §2. Empirical findings (verified against git 2.54.0 on this box)

All run via `git -C <repo> ...` (mirrors run()'s `-C repo` flag, NOT cmd.Dir).

### CommitCount — `git rev-list --count HEAD`

| Scenario | stdout | exit | → return |
|---|---|---|---|
| unborn (init only, 0 commits) | (none; "fatal: ambiguous argument 'HEAD'") | **128** | `(0, nil)` |
| 3 commits | `"3\n"` | 0 | `(3, nil)` |
| non-repo dir | (none; "fatal: not a git repository") | **128** | `(0, nil)` — indistinguishable from unborn (see G4) |

Command: `git rev-list --count HEAD`. Parse stdout with `strconv.Atoi(strings.TrimSpace(stdout))`.

### RecentMessages — `git log --format='%x00%B' -<n>`

| Scenario | exit | → return |
|---|---|---|
| unborn (0 commits) | **128** ("fatal: your current branch 'main' does not have any commits yet") | `(nil, nil)` defensive |
| 3 commits, ask for 20 | 0 | 3 messages (git returns only what exists) |
| non-repo dir | **128** | `(nil, nil)` — indistinguishable from unborn (inherited) |

### The NUL format (`%x00%B`) — FINDING 9 confirmed safe

`git log --format='%x00%B' -3` emits (xxd-verified):
```
00 74 68 69 72 64...  →  \x00 + "third commit subject only\n\n"
00 66 65 61 74...     →  \x00 + "feat: multi-line\n\nThis is a body paragraph\nexplaining the change.\n\n"
00 73 69 6e 67 6c 65  →  \x00 + "single line subject\n\n"
```
- The **first byte is NUL** → splitting on `"\x00"` yields a leading `""` element (dropped after
  TrimSpace).
- Each subsequent element is one commit body (%B = raw body, subject + blank + body) with a trailing
  `\n\n` → `strings.TrimSpace` cleans it to the full message.
- **NUL cannot appear in commit message text** (git forbids NUL in object content). The `---` markdown
  horizontal-rule collision (the commit-pi bug, FINDING 9) is eliminated: verified that a commit body
  containing `---` survives intact inside ONE split element (does NOT create a spurious split).

### The markdown `---` collision proof

A commit with message `docs: update\n\n---\n\nThis is after a horizontal rule`:
- OLD commit-pi format `---%n%B` split on `---` → would BREAK here (3 false fragments).
- NEW `%x00%B` split on `\x00` → the `---` stays inside the single message body. **Confirmed.**

### Multi-line message roundtrip via `makeEmptyCommit`

`makeEmptyCommit(t, dir, "feat: x\n\nBody A.\nBody B.")` (S2's helper, single `-m` arg) preserves the
embedded newlines — verified via `git log --format=%B` retrieval: subject + blank + body intact.
So multi-line fixtures need NO new helper (reuse makeEmptyCommit with a `\n`-bearing string).

### The total-line cap (decision D4 — keep complete messages)

Algorithm: iterate messages newest-first; accumulate `lines = strings.Count(msg, "\n") + 1`; **stop
when the next message would push total > 100** (do NOT include a partial message — partial style
examples mislead the model).

Verified with 30 commits × 4 lines each (120 total > 100):
```
CAP: stopping before msg of 4 lines (total would be 104)
-> kept 25 messages, total 100 lines (<=100 OK=True)
```
The cap keeps the 25 newest complete messages, exactly 100 lines. Older messages are dropped silently
(no sentinel — they are style examples, not a truncated diff; a sentinel would pollute the prompt).

## §3. Design decisions

- **D1** — interface signature FIXED (`n int` only); `maxLines=100` is an internal constant
  `maxRecentMessageLines`. See §1.
- **D2** — NUL-delimited format `%x00%B`; split on `"\x00"` (FINDING 9). The ONLY delimiter that
  cannot collide with commit text.
- **D3** — exit 128 ⇒ defensive empty/zero for BOTH methods (CommitCount `(0,nil)`; RecentMessages
  `(nil,nil)`). Matches the contract ("returns 0 on unborn") and the interface doc ("callers
  short-circuit when isUnborn"). RecentMessages returning empty (not error) on unborn is safe: the
  prompt builder treats empty ⇒ fallback path.
- **D4** — line cap keeps COMPLETE messages only; stop before exceeding 100; newest-first order (git
  log default). No truncation sentinel.
- **D5** — drop empty split elements (the leading `""` before the first NUL, and any blank records).
- **D6** — NEW import `strconv` (CommitCount parses the count via `strconv.Atoi`). This is the ONE
  import change in this subtask (distinct from S2/T3.S2 which add none). gofmt-sorted placement:
  between `os/exec` and `strings` (`strconv` < `strings` alphabetically).
- **D7** — guard `n <= 0` in RecentMessages ⇒ `(nil, nil)` without calling git (avoids undefined
  `git log -0` behavior). Caller passes 20 (PRD FR11); the guard is cheap defensive coding.
- **D8** — two test files (`commitcount_test.go`, `recentmessages_test.go`), matching the one-file-
  per-method convention (revparse_test.go / writetree_test.go / …). Reuse `initRepo`, `makeEmptyCommit`
  — NO new helpers.

## §4. Test matrix

### commitcount_test.go (5 tests)
| Test | Fixture | Asserts |
|---|---|---|
| `TestCommitCount_UnbornRepo` | `initRepo` (0 commits) | `(0, nil)` — exit 128 ⇒ 0 |
| `TestCommitCount_ThreeCommits` | `initRepo` + 3× `makeEmptyCommit` | `(3, nil)` |
| `TestCommitCount_TenCommits` | `initRepo` + 10× `makeEmptyCommit` (loop) | `(10, nil)` |
| `TestCommitCount_GitBinaryMissing` | `t.Setenv("PATH","")` | `err` contains "git binary not found"; `0` |
| `TestCommitCount_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)`; `0` |

NOTE (G4): no separate "NotARepo" test — non-repo ALSO exits 128 ⇒ `(0, nil)`, identical to unborn.
This indistinguishability is INHERITED from S2 (RevParseHEAD treats exit 128 as isUnborn) and is
acceptable (callers gate via RevParseHEAD; a non-repo never reaches CommitCount in the happy path).

### recentmessages_test.go (9 tests)
| Test | Fixture | Asserts |
|---|---|---|
| `TestRecentMessages_UnbornRepo` | `initRepo` (0 commits) | `(nil, nil)` — exit 128 ⇒ empty |
| `TestRecentMessages_SingleLine` | `initRepo` + 2 single-line commits | 2 msgs, each 1 line, exact match |
| `TestRecentMessages_MultiLineBody` | commit with `feat: x\n\nBody A.\nBody B.` | msg includes body, `\n` count ≥ 2 (multi-line detectable) |
| `TestRecentMessages_MarkdownHRCollision` | commit body containing `---` | the `---` stays in ONE message (NUL safety, FINDING 9) |
| `TestRecentMessages_NExceedsCommits` | 2 commits, ask for 20 | 2 msgs (no error; git returns only what exists) |
| `TestRecentMessages_LineCap100` | 30× 4-line commits | `totalLines ≤ 100`; `len < 30` (some dropped) |
| `TestRecentMessages_NotARepo` | `t.TempDir()` w/o initRepo | `(nil, nil)` — exit 128 ⇒ empty (inherited, D3) |
| `TestRecentMessages_GitBinaryMissing` | `t.Setenv("PATH","")` | `err` contains "git binary not found" |
| `TestRecentMessages_ContextCancelled` | `cancel()` before call | `errors.Is(err, context.Canceled)` |

## §5. Non-overlap with parallel subtasks

- **T3.S2 (HasStagedChanges)** lands concurrently. Its git.go edit is the `HasStagedChanges` method
  body (already real on disk); its git_test.go edit removes the `HasStagedChanges` line from
  `TestStubsPanic`. THIS subtask edits the `RecentMessages` + `CommitCount` method bodies and removes
  THEIR two lines from `TestStubsPanic`. **Distinct, non-overlapping regions** (different method
  bodies; different assertPanics lines). The ONE shared region is the import block: T3.S2 adds NOTHING
  (its PRP: "NO import change"); THIS subtask adds `strconv`. Since only one subtask touches imports,
  there is no merge conflict.
- **T3.S1 (StagedDiff)** already landed (real on disk). Untouched here.
- **T3.S4 (RecentSubjects)** is a later subtask; its `RecentSubjects` stub is NOT touched here.

After this subtask + T3.S2, `TestStubsPanic` covers 2 remaining stubs: `RecentSubjects`, `AddAll`.
