# Research Note — Dangling-tree detection for the missing-command regression test

## Question
How to assert "NO new tree object was written" (PRD Issue 3 contract clause d) in a Go test?

## Findings

### `git count-objects -v` count-line delta is the cleanest signal
Verified empirically in a scratch repo:

```
git init; echo a>a; git add a; git commit -m init    # baseline count: 3
echo b>b; git add b                                   # staged NEW file
git write-tree                                        # writes a NEW tree object
  -> count: 3 -> 5  (delta detected)  ✓
```

Contrast: `git write-tree` on an index whose tree already exists (no new content) does **NOT**
increment count (the object already exists). **Implication:** the regression test MUST stage a NEW
file so that, if the pre-flight check were removed, `WriteTree` would write a new object and the
count guard would fire. With nothing staged, `CommitStaged` short-circuits to `ErrNothingToCommit`
BEFORE `WriteTree`, so count stays unchanged even in the buggy build — masking the regression.

### Why not `git fsck` / `git cat-file -t <sha>`?
- `git fsck --lost-found` reports dangling objects, but in a repo with staged-but-uncommitted
  content the staged **blob** is itself dangling → false positives. Not clean here.
- `git cat-file -t <tree>` requires knowing the tree SHA, but the run failed before producing one
  (the whole point) — there is no SHA to probe.

### Helper shape (returns the `count:` line string; needs only `strings`, already imported)
```go
func objectCountLine(t *testing.T, dir string) string {
    t.Helper()
    for _, line := range strings.Split(gitOut(t, dir, "count-objects", "-v"), "\n") {
        if strings.HasPrefix(line, "count:") {
            return line
        }
    }
    t.Fatalf("git count-objects -v: no 'count:' line")
    return ""
}
```
Compare `before := objectCountLine(t, repo)` vs `after` after the failed run → assert equal.

## Error-type flow (why assertions a/b/c hold)

- **Library path (`GenerateCommit` direct):** `buildDeps` returns the raw
  `fmt.Errorf("provider %q: command %q not found. Is the agent installed?", ...)` (plain,
  sentinel-free). `errors.As(err, &re)` (*RescueError) is **false**; `exitcode.For` falls through
  every `errors.Is` branch to `return Error` (1); `err.Error()` contains the literal strings.
- **CLI path (`runDefault` → `GenerateCommit` → `handleGenError`):** the plain error is wrapped by
  `handleGenError`'s generic branch as `exitcode.New(exitcode.Error, err)` → `*ExitError{Code:1}`.
  `exitcode.For` returns the `ExitError.Code` = 1; `errors.As(*ExitError, &re)` traverses Unwrap →
  plain error → still not *RescueError → false. No `❌ Commit generation failed.` / `Tree ID:` on
  stderr (those are only printed by the rescue branch).

## Pre-flight ordering proof (why clause d holds)
`GenerateCommit`: `resolveConfig` → `buildDeps` (pre-flight fires + returns HERE) → (never reaches)
`CommitStaged`/`runPipeline` → (never reaches) `WriteTree`. So zero objects are written. ✓
