# Git Plumbing Reference

**Purpose:** canonical semantics for the porcelain-free plumbing commands an atomic, snapshot-based commit tool in Go must drive. Each section gives exact CLI syntax, exit codes, stdout/stderr behavior, and a Go `os/exec` invocation pattern. Reuse these as the contract for the commit tool's git adapter.

> **Verification convention.** Exact stderr byte strings in git are stable in spirit but occasionally drift across versions (notably 2.x → 2.4x message rewording). Anywhere this doc says "verify locally", capture the literal bytes once with `2>err.txt` against the minimum git version the tool targets, and store that string as a test fixture. Never substring-match a message you haven't captured from the actual environment.

---

## 1. `git write-tree` — materialize the index into a tree object

**Semantics**
- Reads the **current index** (`.git/index`) and writes a full tree object to the object store.
- Prints the resulting **40-char tree SHA-1** (or 64-char SHA-256 on `extensions.objectFormat=sha256`) to stdout, terminated by `\n`.
- Does **NOT** modify the index, does **NOT** touch `HEAD`, does **NOT** create a commit. It is purely "index → tree object, give me the hash." This is the atomic-snapshot primitive: stage exactly what you want, then call `write-tree` once and treat the returned SHA as the immutable filesystem snapshot.
- Cheap to call repeatedly; safe to call before deciding whether to commit.

**Conflict failure mode**
- If the index contains **unmerged (stage 1/2/3) entries** (i.e., an in-progress merge/rebase/cherry-pick left conflicts), `write-tree` **refuses to write**.
- Exit code: **128**.
- Stderr (rewording across versions — verify locally; representative text):
  ```
  error: cannot write tree object
  fatal: <path>: needs merge
  ```
  Older/newer git may phrase it as `error: cannot write a tree with unresolved merge conflicts`. The stable signal is **exit code ≠ 0** plus the word "merge" or "needs merge" on stderr. Do **not** rely on a single exact phrase.

**Correct handling in the commit tool**
- Treat `write-tree` failure as a hard precondition failure: the snapshot is not well-formed. Either abort the commit with a diagnostic ("unresolved merge conflicts in index; resolve or abort, then retry"), or surface `git diff --name-only --diff-filter=U` to list the offending paths.
- Parse stdout as a single token; trim whitespace. Anything longer than one line is a bug in your invocation (probably stderr leaked to stdout — see Windows note below).

**Go pattern**
```go
func writeTree(ctx context.Context, repo string) (string, error) {
    cmd := exec.CommandContext(ctx, "git", "-C", repo, "write-tree")
    var out, errb bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &errb
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("git write-tree: %w (stderr: %s)", err, strings.TrimSpace(errb.String()))
    }
    return strings.TrimSpace(out.String()), nil // e.g. "3a3b8d7..."
}
```

---

## 2. `git commit-tree <tree> [-p <parent>]... [-m <msg>]... [-F <file>] [-]` — create a commit object

**Semantics**
- Creates a **commit object** wrapping the given tree SHA plus parent(s), author/committer identity (from `user.name`/`user.email` and dates from env: `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL`, `GIT_AUTHOR_DATE`, and the `GIT_COMMITTER_*` twins), and a commit message.
- Prints the **commit SHA** to stdout, `\n`-terminated. Like `write-tree`, this is a pure object write — it does **NOT** move any ref. You must follow it with `update-ref` (see §3) to make the commit reachable from a branch/HEAD.
- `GIT_AUTHOR_DATE` / `GIT_COMMITTER_DATE` accept ISO-8601 (`2026-06-29T10:00:00`), RFC-2822, or unix epoch with timezone (`1719645600 +0000`). For reproducible/testable commits, pin both.

**Flags**
- `-p <parent>` — add a parent commit. **Repeatable**: `-p A -p B` makes a merge commit. **Root commit** = omit `-p` entirely. The tool must branch on "is this the first commit?" before deciding to add `-p`.
- `-m <message>` — commit message paragraph. **Repeatable**: each `-m` becomes a **separate paragraph** separated by a blank line (`\n\n`), NOT a separate line. So `-m "subject" -m "body p1" -m "body p2"` yields `subject\n\nbody p1\n\nbody p2`. This matches `git commit -m` behavior.
- `-F <file>` — read the full message from a file (use `-` to read from **stdin**). Use this when the message contains special characters, newlines, leading `-` (which would otherwise look like a flag), quotes, or is large. `-F` reads the file verbatim and trims a single trailing newline if present. **Prefer `-F` over `-m` for any message sourced from user input or template output** to avoid arg-quoting hell across shells.
- Reading from stdin: `echo "$msg" | git commit-tree <tree> -p <parent> -F -` (note the `-` argument). In Go, write to the command's `Stdin` pipe.

**Root commit**
- A commit with **no `-p`** is a root commit. It is valid. After writing it, `update-ref HEAD <sha>` with the expected-old value being the unborn state (see §3/§4) moves HEAD onto it. There is no special flag for "root"; absence of `-p` is the signal.

**Output**
- Single line: the new commit SHA. Same parsing rule as `write-tree`.

**Go pattern (stdin message, with optional parent)**
```go
func commitTree(ctx context.Context, repo, tree string, parents []string, msg string) (string, error) {
    args := []string{"-C", repo, "commit-tree", tree}
    for _, p := range parents {
        args = append(args, "-p", p)
    }
    args = append(args, "-F", "-") // message via stdin → no quoting pitfalls
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Stdin = strings.NewReader(msg)
    cmd.Env = append(os.Environ(),
        "GIT_AUTHOR_NAME="+authorName,
        "GIT_AUTHOR_EMAIL="+authorEmail,
        "GIT_AUTHOR_DATE="+isoDate,
        "GIT_COMMITTER_NAME="+committerName,
        "GIT_COMMITTER_EMAIL="+committerEmail,
        "GIT_COMMITTER_DATE="+isoDate,
    )
    var out, errb bytes.Buffer
    cmd.Stdout, cmd.Stderr = &out, &errb
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("git commit-tree: %w (stderr: %s)", err, errb.String())
    }
    return strings.TrimSpace(out.String()), nil
}
```

**Important:** never `fmt.Sprintf` a message into `-m "<msg>"` and shell out through a shell. Always pass args as a Go `[]string` to `exec.Command` (no shell), and prefer `-F -` + stdin.

---

## 3. `git update-ref <ref> <new> [<expected-old>]` — atomic ref move (CAS)

**Semantics**
- Moves the ref `<ref>` (e.g. `HEAD`, `refs/heads/main`) to point at `<new>`.
- **2-arg form** `git update-ref HEAD <new>`: **unconditional** overwrite. Whoever calls this wins; no concurrency guard. Dangerous for an "atomic snapshot commit" tool because another process (editor, IDE, cron, a concurrent commit-tool invocation) could have moved HEAD between your `rev-parse HEAD` and your `update-ref`, and you'd clobber their work silently.
- **3-arg form** `git update-ref HEAD <new> <expected-old>`: **compare-and-swap**. Git takes the ref lock, reads the current value, and only writes `<new>` if the current value equals `<expected-old>` (SHA string comparison). The lock + compare happen **inside one git process** under a `.git/<ref>.lock` file → atomic w.r.t. other `git` writers.
  - **Why this is the safe primitive for the commit tool:** it lets you compute `expectedOld` once (from `rev-parse HEAD`, handling the unborn case), do all your read/verify work, then say "advance HEAD to my new commit **only if nobody moved it**." If CAS fails, you know the world changed and you re-read and retry (or abort). No lost update.
- Exit code on **CAS mismatch** (current value ≠ `<expected-old>`): **non-zero (1)**. Stderr (verify locally; representative): `fatal: cannot lock ref 'HEAD': is at <actual> but expected <expected-old>` (git ≥ 2.x may say `fatal: update_ref failed for ref 'HEAD'` on older versions). The stable signal is **exit code ≠ 0**; do not parse the message.
- Special expected-old value: the **all-zeros SHA** (`0000000000000000000000000000000000000000` for sha-1, or 64 zeros for sha-256) represents "the ref does not exist yet" (unborn). So for a **root commit**, use `git update-ref HEAD <newCommit> 0000...0` — it succeeds only if HEAD is currently unborn, i.e. the repo truly has zero commits. This composes with §4's root detection.

**Atomicity guarantee**
- git's ref update uses a per-ref lockfile (`.git/<ref>.lock`); the read-compare-write is serialized against other `git update-ref`, `git commit`, `git push`, etc. that touch the same ref. It is the correct linearization point for the commit tool's "publish" step.

**Go pattern**
```go
// zeroHash returns the all-zeros object id for the repo's hash algo.
func updateRefCAS(ctx context.Context, repo, ref, newSHA, expectedOld string) error {
    cmd := exec.CommandContext(ctx, "git", "-C", repo,
        "update-ref", ref, newSHA, expectedOld)
    var errb bytes.Buffer
    cmd.Stderr = &errb
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("ref CAS failed (expected %s → %s for %s): %w; stderr: %s",
            expectedOld, newSHA, ref, err, errb.String())
    }
    return nil
}
```
Retry policy on CAS failure: re-read current HEAD (`rev-parse HEAD`), re-derive parent + recompute tree if the working set changed, then retry the CAS with the new expected-old. Bound the retry count to avoid livelock.

---

## 4. `git rev-parse HEAD` — current commit SHA, and the unborn/empty-repo trap

**Semantics**
- On a repo **with** at least one commit: prints the 40-char SHA HEAD points at, exit **0**.
- On a repo **with zero commits** (freshly `git init`'d, or all branches deleted): exit **128**, and the literal string `HEAD\n` is printed to **stdout** (yes — the string "HEAD", not empty, not an error placeholder), with a diagnostic on **stderr**:
  ```
  fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree.
  Use '--' to separate paths from revisions, like this:
  'git <command> [<revision>...] -- [<path>...]]'
  ```
  The stdout being non-empty ("HEAD") is the trap: a naive `strings.TrimSpace(out)` + "if empty then unborn" check is **wrong** and will treat "HEAD" as a commit SHA, then blow up later in `commit-tree -p HEAD`.

**Correct root detection in Go — check the exit code, NOT string emptiness**
```go
func currentHead(ctx context.Context, repo string) (sha string, isUnborn bool, err error) {
    cmd := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "HEAD")
    var out, errb bytes.Buffer
    cmd.Stdout, cmd.Stderr = &out, &errb
    runErr := cmd.Run()
    if runErr != nil {
        // exit 128 + "ambiguous argument 'HEAD'" → unborn, not a hard error.
        if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
            return "", true, nil
        }
        return "", false, fmt.Errorf("git rev-parse HEAD: %w; stderr: %s", runErr, errb.String())
    }
    return strings.TrimSpace(out.String()), false, nil
}
```
Then: `if isUnborn { parent = ""; expectedOld = zeroHash } else { parent = sha; expectedOld = sha }`.

**More robust alternative** (recommended — avoids parsing the 128 case at all): use `git rev-parse --verify -q HEAD` which exits 128 silently and prints nothing on unborn, or use `git symbolic-ref HEAD` to confirm HEAD points at a branch then check the branch ref exists. But for an atomic-snapshot tool the exit-code check above is sufficient and explicit.

---

## 5. `git diff --cached` — inspect staged content

**Semantics**
- `git diff --cached` (alias: `--staged`) shows diff between `HEAD` and the **index** (what `write-tree` will materialize). On an unborn repo it diffs against the empty tree (everything staged shows as added) — fine.
- Pathspec exclusions use the magic-prefix syntax:
  - `:!pattern` — exclude paths matching glob `pattern`. Example: `git diff --cached -- ':!*.lock' ':!vendor/*'`.
  - The `:` magic prefix must be passed as a **separate argv element** like `:!*.lock` — in Go pass it literally as one string arg; do not let a shell mangle the `!` (history expansion). Since Go's `exec.Command` skips the shell, this is safe; just don't `sh -c`.
  - Combine includes + excludes: `git diff --cached -- 'src/**' ':!src/generated/*'`.
  - Pathspecs are git-globs, not full regex; `**` matches across directories only when `core.glob` / the pathspec `:(glob)` magic is in effect. For portability prefer explicit `:(glob)` or list patterns explicitly.

**Capping output by bytes**
- `git diff` has **no** built-in `--max-bytes`. To cap output, capture into a `bytes.Buffer`/`io.LimitedReader` yourself:
```go
cmd := exec.CommandContext(ctx, "git", "-C", repo, "diff", "--cached", "--", ":!*.lock")
pr, pw := io.Pipe()
cmd.Stdout = pw
cmd.Stderr = io.Discard
go func() { _ = cmd.Run(); pw.Close() }()
io.CopyN(destWriter, io.LimitReader(pr, maxBytes)) // then drop a truncation sentinel
```
- Alternatively limit scope with `--stat` (compact, bounded per-file) or `--numstat` for machine-readable added/deleted line counts, then only fetch full diffs for the top-N files. `--shortstat` gives a one-line summary.

**`--quiet` → "nothing staged?" signal**
- `git diff --cached --quiet` produces **no diff output**; the **exit code** encodes the answer:
  - **0** → no staged differences (index == HEAD).
  - **1** → there ARE staged differences.
  - **>1** → real error.
- This is the idiomatic, cheap "is anything staged?" check — no output to parse. Combine with pathspec to answer "is anything staged outside of `.lock`/`vendor/`?": `git diff --cached --quiet -- ':!*.lock' ':!vendor/*'`.
```go
func hasStagedChanges(ctx context.Context, repo string, exclude []string) (bool, error) {
    args := append([]string{"-C", repo, "diff", "--cached", "--quiet", "--"}, exclude...)
    cmd := exec.CommandContext(ctx, "git", args...)
    cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
    err := cmd.Run()
    if err == nil { return false, nil }                       // exit 0 → nothing staged
    if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
        return true, nil                                       // exit 1 → staged changes present
    }
    return false, fmt.Errorf("git diff --cached --quiet: %w", err)
}
```

---

## 6. `git log --format=...` — message shape extraction for style learning

**Format specifiers**
- `%s` — **subject** (first line of the commit message, after cleanup).
- `%b` — **body** (everything after the subject line and its trailing blank line).
- `%B` — **raw body / full message** (subject + body, verbatim — the whole message block as stored).
- `%H` — full commit SHA; `%h` — abbreviated; `%an`/`%ae`/`%ad` — author name/email/date; `%cn`/`%ce`/`%cd` — committer equivalents; `%n` — literal newline.

**Multi-record output for style learning**
- The pattern `git log --format='---%n%B'` emits, for each commit, a line containing exactly `---`, a blank line, then the full message, repeated across history. The leading `---` is a delimiter you control and can split on reliably.
- Even safer (avoids a `---` accidentally appearing inside a real commit body): use a delimiter that cannot occur in a commit message by construction, e.g. `--format='%x00%B'` (`%x00` = NUL byte) and split on `\x00`. NUL-delimited is the robust choice for piping to a parser.
- Bound the sample size with `-N` (e.g. `-50`) and/or `--since`/`--until` to keep style-learning input bounded.
```go
// NUL-delimited recent messages for style learning:
cmd := exec.CommandContext(ctx, "git", "-C", repo, "log", "-50", "--format=%x00%B")
// read all, split on \x00, drop empty leading element.
```
- Avoid `--format=---%n%B` if you ever store commit messages that may contain `---` (Markdown horizontal rules are common); prefer the NUL form.

---

## 7. `git diff-tree --no-commit-id --name-status -r <sha>` — "what landed"

**Semantics**
- `git diff-tree` compares a commit against its (first) parent and prints the file-level change set.
- Flags:
  - `-r` — **recurse** into subtrees (without it you get top-level tree entries only, not individual files). Always use `-r`.
  - `--no-commit-id` — suppress the leading commit-SHA line that `diff-tree` prints by default. You usually want this so the output is purely the file list.
  - `--name-status` — for each changed path, print `<status>\t<path>` (and for renames/copies, `<status>\t<source>\t<dest>`).
- **Status codes:** `A` added, `M` modified, `D` deleted, `R` renamed, `C` copied, `T` type-changed (e.g. file↔symlink), `U` unmerged (shouldn't appear post-commit). Renames/copies carry a similarity score: `R90`, `C75`.
- This is the canonical "show me what this commit changed" UX — it's exactly what `git show --name-status` uses internally, but porcelain-free and trivially parseable.

**Parse format (stable, tab-separated)**
```
A\tpath/to/new/file
M\tpath/to/modified
D\tpath/to/deleted
R100\told/name\tnew/name
```
Split each line on `\t`. Two fields = `status, path`; three fields = `status, src, dst` (rename/copy).

**On a root commit** (no parent), `git diff-tree -r --root <sha>` compares against the empty tree and shows every file as `A`. Without `--root`, a root commit produces **no output** (no parent to diff against) — pass `--root` when the commit might be a root.

**Go pattern**
```go
type FileChange struct{ Status, Path, SrcPath string }

func whatLanded(ctx context.Context, repo, sha string, isRoot bool) ([]FileChange, error) {
    args := []string{"-C", repo, "diff-tree", "--no-commit-id", "--name-status", "-r"}
    if isRoot { args = append(args, "--root") }
    args = append(args, sha)
    out, err := exec.CommandContext(ctx, "git", args...).Output()
    if err != nil { return nil, err }
    var changes []FileChange
    for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
        if line == "" { continue }
        f := strings.Split(line, "\t")
        var fc FileChange
        switch len(f) {
        case 2: fc = FileChange{Status: f[0], Path: f[1]}
        case 3: fc = FileChange{Status: f[0], SrcPath: f[1], Path: f[2]}
        default: continue
        }
        changes = append(changes, fc)
    }
    return changes, nil
}
```

---

## Cross-cutting: Go `os/exec` conventions & Windows gotchas

**Always**
- Use `exec.CommandContext` (cancellable; lets you enforce timeouts on `git diff` over huge repos).
- Pass `git -C <repo>` instead of `os.Chdir` — keeps goroutines safe and avoids touching the parent process's CWD.
- Capture **both** stdout and stderr to separate buffers; include a trimmed snippet of stderr in error messages (invaluable for diagnosing `write-tree`/`update-ref` failures).
- Check `*exec.ExitError.ExitCode()` for the documented exit-code signals (128 for unborn HEAD; 1 for diff-quiet / CAS-mismatch). Never use "stdout is empty" as a semantic signal.
- Trim trailing `\n` from single-line outputs (`write-tree`, `commit-tree`, `rev-parse`).
- Pass args as `[]string` — never shell out (`sh -c`, `cmd /c`) for plumbing calls. This eliminates quoting, globbing, history-expansion (`!`), and `;` injection issues entirely.

**Windows-specific gotchas**
1. **`git.exe` lookup.** `exec.LookPath("git")` works if git is on `PATH`. On Windows, prefer the bundled git or `where git` discovery; do not hardcode `C:\Program Files\Git\...`. Some users have only Git Bash installed — invoke the `git` binary, never `bash -c git ...`.
2. **No shell = no `~`, no env expansion, no `&&`.** Compute absolute repo paths in Go (`filepath.Abs`). `-C` must be a real path, not `~/repo`.
3. **Newlines.** git emits `\n`-terminated output even on Windows (it normalizes internally). Do **not** `strings.Replace(out, "\r\n", "\n")` defensively — git's plumbing stdout is already LF. (Only user-edited files via `core.autocrlf` get CRLF; plumbing object output does not.)
4. **Console code page / UTF-8.** `cmd.Stderr`/`cmd.Stdout` as a `bytes.Buffer` reads raw bytes — safe. If you instead let git inherit the parent's Windows console handle, non-ASCII in commit messages may be transcoded; always capture to a buffer and decode as UTF-8.
5. **`--pathspec-from-file` / `-F -`** (stdin) work identically on Windows; the stdin pipe is binary-safe.
6. **File handles / locks.** On Windows, a file held open by another process blocks `write-tree`/index writes in edge cases (the `.git/index.lock` pattern still works, but a tree walk that opens user files can fail with sharing violations). If the commit tool reads working-tree files itself, open them read-only and close promptly.
7. **Symlinks.** Default Windows git has `core.symlinks=false`; `diff-tree` may report `T` (typechange) differently. For a cross-platform tool, treat symlink handling as best-effort and document the Windows caveat.
8. **Long paths.** Repos exceeding `MAX_PATH` (260) need `core.longpaths=true` on Windows; the commit tool should not assume short paths.

**Sequencing recap for an atomic snapshot commit**
1. (optional) `git diff --cached --quiet -- ':!*.lock'` → decide whether to commit.
2. `git rev-parse HEAD` → get current SHA + detect unborn (exit 128 ⇒ root commit, expected-old = zero hash).
3. `git write-tree` → snapshot SHA (fails on conflicts; abort cleanly).
4. `git commit-tree <tree> [-p <parent>] -F -` (stdin message) → new commit SHA.
5. `git update-ref HEAD <newCommit> <expectedOld>` → atomic CAS publish; on failure, re-read & retry or abort.
6. `git diff-tree --no-commit-id --name-status -r [--root] <newCommit>` → report "what landed" to the user.

Steps 3–5 are the atomic core; 1, 2, 6 are read-only and freely retryable.
