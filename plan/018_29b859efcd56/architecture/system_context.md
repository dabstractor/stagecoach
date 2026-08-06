# System Context — Delta: FR-39a (commit-identity transparency)

## What this delta is

FR-39a ("Commit-identity transparency — stagecoach is invisible in commit metadata", P0 → G3) is a
**defensive / permanent guarantee that codifies behavior the code ALREADY upholds**. It is NOT new
functionality. The deliverable for this delta is:

1. A **behavioral regression test** (success-path) asserting every stagecoach commit's author/committer
   == git-resolved identity, and that no branding trailer is appended.
2. A **structural guard** (source-scan test) asserting no production `.go` path writes identity config
   or sets identity env.
3. An **in-code doc tag** (`FR-39a`) on the existing comment so the guarantee is grep-discoverable.

## Scope boundary (verified)

- No CLI flag, no config key, no provider/manifest change, no output change. (PRD §3.)
- No interaction with v3.0 self-update (`stagecoach upgrade` makes no commits). (PRD §5.)
- The future opt-in `Co-authored-by: stagecoach` trailer flag is **explicitly deferred** — out of scope.

## The commit path (the surface FR-39a constrains)

```
generate.CommitStaged(ctx, Deps{Git, Manifest}, cfg)  (internal/generate/generate.go:188)
  └─ git.Git.CommitTree(ctx, tree, parents, msg)       (internal/git/git.go:680)
        args = ["commit-tree", tree, "-p" <parents>..., "-F", "-"]   // message via stdin (FINDING 4)
        └─ (*gitRunner).runWithInput(ctx, repo, stdin, args...)      (internal/git/git.go:522)
              └─ exec.LookPath("git"); exec.Cmd{Args: full}
              └─ *** cmd.Env is NEVER set *** → child inherits parent environment
```

Because `runWithInput` deliberately leaves `cmd.Env` unset, `git commit-tree` resolves author/committer
from the user's own config/env (local > global > system > `GIT_*` env) — exactly the FR-39a guarantee.
Stagecoach injects nothing.

## Why this is test-only work (the proof the invariant holds)

| FR-39a clause | Production state (verified this session) |
| --- | --- |
| No `GIT_AUTHOR_*` / `GIT_COMMITTER_*` env on any git subprocess | ✅ ZERO production hits (grep excludes `_test.go`). `runWithInput` sets no `cmd.Env`. |
| No `user.name` / `user.email` config writes (any scope) | ✅ Only occurrence is a **comment** (`internal/git/git.go:520`); all writes are in `_test.go` scaffolding. |
| Config bootstrap never writes identity | ✅ `internal/config/bootstrap.go` writes only stagecoach keys (`[defaults]`, `[role.*]`, `config_version`). |
| No branding trailer/footer | ✅ Messages emitted verbatim from provider, parsed/cleaned (`internal/provider/parse.go`, `internal/generate/finalize.go`); stagecoach appends nothing. |

## Documentation disposition (per SOW §5)

- **Mode A (doc-with-work):** the implementing subtask appends `(FR-39a — commit-identity transparency)`
  to the existing `runWithInput` comment (`internal/git/git.go:518-521`) and cites FR-39a in the
  `CommitTree` doc comment (`internal/git/git.go:672-679`). No `docs/*.md` page discusses commit
  identity today (confirmed: zero hits in `docs/`), so no per-file doc update is required.
- **Mode B (changeset-level):** per SOW directive ("do not skip the final docs task"), a lightweight
  final task sweeps `README.md` + `docs/how-it-works.md`. Likely conclusion: no change needed (FR-39a
  is an internal invariant with no user-facing surface and the README makes no AI-branding claim to
  retract) — but the implementing agent makes that call formally.

## Risk

- **No behavior risk** — this delta adds a regression net, not new logic.
- The only way it can "fail" is a false-positive structural guard (catches a legitimate comment or
  test helper). Mitigation: exclude `_test.go` and match on actual call-site patterns.
- Test-scaffolding identity writes (`setIdentityConfig`, `git config user.name Test`) are LEGITIMATE
  test setup and MUST be allow-listed by the structural guard.