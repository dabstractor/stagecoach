# FR-39a — Precise code locations & the proof the invariant holds

All line numbers verified against the working tree this session. (PRD-cited line numbers are ~2 off
in places; content matches exactly.)

## Production code (no changes except a doc tag)

### `internal/git/git.go`

- **`runWithInput` — line 522** (`func (g *gitRunner) runWithInput(ctx, repo, stdin, args...)`).
  Body builds `exec.Cmd` and **never assigns `cmd.Env`** → child inherits parent environment. This is
  the single structural fact that makes FR-39a hold.
- **The comment to tag — lines 518-521** (current text):
  ```go
  // Identity: cmd.Env is NOT set here, so the child inherits the parent environment. Production
  // callers commit AS the configured user (git resolves user.name/user.email from config/env);
  // tests set repo-local user.name/user.email via `git config` (see committree_test.go).
  ```
  → append ` (FR-39a — commit-identity transparency)` so the guarantee is grep-discoverable.
- **`CommitTree` — line 680** (`func (g *gitRunner) CommitTree(ctx, tree, parents, msg) (sha, err)`).
  Builds `["commit-tree", tree, "-p"..., "-F", "-"]`, calls `runWithInput`, returns trimmed stdout.
  Doc comment spans **lines 672-679** → cite `FR-39a` there.
- **`run` — the sibling helper**: also does NOT set `cmd.Env` (same deliberate choice). Not on the
  commit path but worth noting the invariant is package-wide.

### `internal/config/bootstrap.go`
- Writes only stagecoach config keys. **No** `git config user.*` calls. (Verified.)

### `internal/provider/parse.go` + `internal/generate/finalize.go`
- Emit/clean provider output verbatim; stagecoach appends **no** trailer/footer. (Verified.)

## Grep evidence (production tree, excluding `_test.go`)

```
$ grep -rn "GIT_AUTHOR\|GIT_COMMITTER" internal/ cmd/ pkg/ --include="*.go" | grep -v "_test.go"
   → NONE FOUND   ✅

$ grep -rn "user.name\|user.email" internal/ cmd/ pkg/ --include="*.go" | grep -v "_test.go"
   → internal/git/git.go:520:// callers commit AS the configured user (git resolves user.name/user.email …)
   (only a COMMENT)   ✅
```

## Test-scaffolding identity writes that MUST be allow-listed (NOT violations)

These are legitimate test setup, found only in `_test.go`:

- `internal/git/committree_test.go:17` — `setIdentityConfig(t, dir)` writes `user.name Test` /
  `user.email test@example.com` (the canonical repo-local identity helper to mirror).
- `internal/git/git_test.go:28-36`, `internal/config/git_test.go:32-40`,
  `internal/cmd/{hookexec,root}_test.go`, `internal/signal/signal_integration_test.go`,
  `internal/e2e/harness_test.go:84-85`, and all `internal/decompose/*_test.go` — every one is
  `git config user.name "Test"` / `user.email "test@example.com"` for test fixture identity.
- `internal/generate/{hooks_freeze,repro_freeze,generate}_test.go` — same pattern.

**The structural guard must skip any file ending in `_test.go`** (e.g.
`strings.HasSuffix(path, "_test.go")`) so it catches only production regressions.

## The §20.2 invariant the FR adds (PRD.md line 2148)

> Commit-identity transparency (FR-39a): every commit created by a stagecoach run has `author` and
> `committer` identical to what the user's `git config user.name`/`user.email` (and any
> `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env) resolve — never a stagecoach-branded identity. Asserted by
> resolving the expected identity before the run and comparing against
> `git log -1 --format='%an <%ae> | %cn <%ce>'` of the resulting commit; also assert the commit
> message contains no `Co-Authored-By:`/`Generated-by:` trailer that stagecoach did not put there. A
> test that pre-seeds a repo-local `user.name = "stagecoach agent"` confirms stagecoach inherits it
> (transparency) but that no code path ever writes such a key.

## The exact verification command

```
go test ./internal/generate/ ./internal/git/ ./internal/config/
```