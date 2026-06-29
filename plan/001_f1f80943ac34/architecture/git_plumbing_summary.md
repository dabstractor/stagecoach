# Git Plumbing Reference (Condensed)

> Full reference with Go code patterns in `git_plumbing_reference.md` (researcher output).
> This is the implementation-facing summary with exit codes and the atomic-commit sequence.

## Atomic Commit Sequence (the core IP, §13)

```
1. (read-only)  git diff --cached --quiet          → exit 1 = staged; exit 0 = nothing
2. (read-only)  git rev-parse HEAD                 → SHA or exit 128 (unborn → root commit)
3. (snapshot)   git write-tree                     → TREE_SHA (fails exit 128 if conflicts in index)
4. (generate)   <agent CLI>                        → commit message (this is where time is spent)
5. (object)     git commit-tree [-p PARENT] -F - TREE  → NEW_SHA (dangling, no ref moved)
6. (CAS)        git update-ref HEAD NEW PARENT     → atomic publish (fails if HEAD moved)
7. (report)     git diff-tree --no-commit-id --name-status -r [--root] NEW  → "what landed"
```

Steps 3–6 are the atomic core. The index and HEAD are **never touched** between step 3 and step 6.
A failure anywhere before step 6 leaves the repo byte-for-byte unchanged.

## Exit-Code Cheat Sheet

| Command | Exit 0 | Exit 1 | Exit 128 | Notes |
|---|---|---|---|---|
| `git rev-parse HEAD` | has SHA | — | unborn repo | stdout = literal "HEAD" on unborn; check exit code! |
| `git write-tree` | TREE_SHA | — | conflict in index | abort before generation |
| `git commit-tree` | NEW_SHA | — | bad tree/parent | object write only; no ref moved |
| `git update-ref HEAD <new> <old>` | CAS success | CAS mismatch (HEAD moved) | — | **never force-update** |
| `git diff --cached --quiet` | nothing staged | staged changes exist | error | exit 1 = "has staged" |
| `git diff-tree ... <sha>` | (always) | — | bad sha | use `--root` for root commits |

## Key Go Patterns (from researcher output)

**write-tree:**
```go
cmd := exec.CommandContext(ctx, "git", "-C", repo, "write-tree")
// capture stdout (TREE_SHA, trim \n), stderr. On error → abort.
```

**commit-tree (message via stdin with -F -):**
```go
args := []string{"-C", repo, "commit-tree", tree}
for _, p := range parents { args = append(args, "-p", p) }  // root commit: no -p
args = append(args, "-F", "-")  // read message from stdin — avoids all quoting issues
cmd.Stdin = strings.NewReader(msg)
```

**update-ref CAS:**
```go
// 3-arg form = compare-and-swap. Fails if HEAD != expected-old.
cmd := exec.CommandContext(ctx, "git", "-C", repo, "update-ref", "HEAD", newSHA, expectedOld)
// On exit != 0: CAS failed (HEAD moved). Do NOT force. Print rescue message.
// For root commit: expectedOld = all-zeros hash (ref must be unborn).
```

**has-staged-changes:**
```go
cmd := exec.CommandContext(ctx, "git", "-C", repo, "diff", "--cached", "--quiet")
// exit 0 → nothing; exit 1 → staged; >1 → error
```

**diff-tree (what landed):**
```go
args := []string{"-C", repo, "diff-tree", "--no-commit-id", "--name-status", "-r"}
if isRoot { args = append(args, "--root") }  // root commit: diff against empty tree
args = append(args, sha)
// parse: "A\tpath", "M\tpath", "R100\told\tnew" (tab-separated)
```

## Diff Capture (§9.1, matching commit-pi)

```go
// 1. Markdown files: per-file diff, capped at max_md_lines (100) lines each
//    git diff --cached -- '<file>'  → head -n max_md_lines

// 2. Non-markdown: single diff with pathspec exclusions, capped at max_diff_bytes (300000)
//    git diff --cached -- \
//      ':!*.lock' ':!package-lock.json' ':!pnpm-lock.yaml' ':!yarn.lock' \
//      ':!*.snap' ':!*.map' ':!vendor/*' ':!*.md' ':!*.markdown'
//    → cap output at max_diff_bytes, append truncation sentinel

// 3. Concatenate markdown section + other section into one payload.
```

## Log Queries (§9.3, §9.7)

```go
// Commit count (decides mature vs new-repo prompt)
git rev-list --count HEAD   // → integer; "0" or "1" on unborn/fresh repo

// Style examples (last 20 full messages, ≤100 lines)
git log --format='%x00%B' -20   // NUL-delimited (robust); commit-pi used '---%n%B' (markdown-collision risk)

// Recent subjects (last 50, for duplicate check)
git log --format=%s -50   // one subject per line; exact-match check via a set/map
```

## Cross-Platform Notes

- Always use `git -C <repo>` (not `os.Chdir`) for goroutine safety.
- Capture stdout AND stderr to separate buffers; include stderr snippet in error messages.
- Pass args as `[]string` — NEVER shell out (`sh -c`/`cmd /c`) for plumbing. Eliminates quoting,
  globbing, history-expansion, and injection issues.
- Git emits `\n`-terminated output even on Windows (normalizes internally). No need for `\r\n` → `\n`.
- `SysProcAttr.Setpgid` is Unix-only; Windows needs a build-tag abstraction (see `critical_findings.md` #10).
