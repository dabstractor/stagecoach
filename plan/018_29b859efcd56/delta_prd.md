# Stagecoach — Delta PRD: Commit-identity transparency (FR-39a)

| Field | Value |
| --- | --- |
| **Delta source** | PRD.md commit `347b23f` ("Add FR-39a: commit-identity transparency guarantee") |
| **Delta size** | **Small** — 4 insertions / 1 deletion in `PRD.md` (~440 words, one file). One new P0 requirement (`FR-39a`) plus three cross-references. No code behavior change required. |
| **Builds on** | Session 017 (`plan/017_397abce9deb1/`) — v3.0 self-update + distribution surface; the commit-plumbing (`internal/git`, `internal/generate`, `internal/decompose`) it exercises is unchanged. |
| **Last updated** | 2026-08-06 |

---

## 1. What changed (diff analysis)

The previous session's PRD ended at the **v3.0** self-update + distribution revision (commit `6a8e76d`). The current PRD adds exactly **one commit on top** (`347b23f`), introducing a single new requirement and wiring it through three cross-reference sites. Confirmed via `git show 347b23f -- PRD.md`:

1. **NEW requirement — `FR-39a` (§9.9 Commit creation, P0 → G3).** "Commit-identity transparency — stagecoach is invisible in commit metadata." Codifies that stagecoach NEVER sets, overrides, or injects a git author/committer identity: no `user.name`/`user.email` writes to any config scope (system/global/local, not even transiently), no `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL`/`GIT_COMMITTER_NAME`/`GIT_COMMITTER_EMAIL`/`GIT_AUTHOR_DATE`/`GIT_COMMITTER_DATE` env on the `commit-tree` subprocess (or any other). The author AND committer of every stagecoach commit are exactly what git resolves from the user's own config/env (git's standard precedence). No stagecoach-branded author/email, no `Co-Authored-By:` trailer, no `Generated-by:`/`🤖` footer. If git cannot resolve an identity, stagecoach surfaces git's error verbatim and exits non-zero — it MUST NOT invent a stagecoach-branded fallback. Defensive & permanent. Field note: a coding agent once wrote `user.name = "stagecoach agent"` into a repo's local git config; stagecoach inherited it faithfully (correct behavior), but this FR records that stagecoach must never be the *source* of such an identity. (A single explicit opt-in `Co-authored-by: stagecoach` trailer MAY ship behind its own flag in a future revision, default off; v3 ships with no such flag.)

2. **MODIFIED — §13.2 point 2 (`commit-tree`).** Added one trailing sentence: because stagecoach invokes `commit-tree` without setting `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env and without writing `user.name`/`user.email`, the commit's author/committer are exactly git's resolved identity (FR-39a) — stagecoach is invisible in commit metadata.

3. **NEW bullet — §19 (Security considerations).** "Commit-identity transparency (FR-39a)." Summarizes the guarantee; branding a commit as machine-made would be exactly the unsolicited mutation this tool refuses.

4. **NEW invariant — §20.2 (Property/invariant tests).** "Commit-identity transparency (FR-39a)": assert every commit's `author`/`committer` == git-resolved identity; assert the message has no `Co-Authored-By:`/`Generated-by:` trailer stagecoach didn't add; and a test that pre-seeds repo-local `user.name = "stagecoach agent"` confirms stagecoach inherits it (transparency) but that **no code path ever writes such a key**.

**Nothing else changed.** No other FRs, no config/CLI/provider surface, no commit/CAS/rescue/lock logic. The v3.0 revision block in the PRD header is unchanged — FR-39a is a quiet, defensive patch on top of v3.0, not a new revision.

---

## 2. Implementation status (critical: the invariant ALREADY holds)

FR-39a is a **defensive/permanent guarantee that codifies existing behavior**, not new functionality. The previous session's implementation already satisfies it:

| FR-39a clause | Current state | Location |
| --- | --- | --- |
| No `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env on any git subprocess | ✅ Holds — `CommitTree` calls `runWithInput`, which deliberately does NOT set `cmd.Env` (child inherits parent env). Verified: `grep -rn "GIT_AUTHOR\|GIT_COMMITTER" internal/ cmd/ pkg/` returns **zero** production hits. | `internal/git/git.go:680` (`CommitTree`) + `:510` (`runWithInput`, "cmd.Env is NOT set here") |
| No `user.name`/`user.email` config writes (any scope) | ✅ Holds — no production code path writes identity config. The only `user.name`/`user.email` writes in the repo are (a) a **comment** at `git.go:520` and (b) **test scaffolding** (`setIdentityConfig` in `committree_test.go`; `config user.name Test` in generate/hook/repro tests). | `internal/git/committree_test.go:17`, `internal/generate/*_test.go` |
| Config bootstrap never writes identity | ✅ Holds — `internal/config/bootstrap.go` writes only stagecoach config keys (`[defaults]`, `[role.*]`, `config_version`); no `git config user.*` calls. | `internal/config/bootstrap.go` |
| No branding trailer/footer | ✅ Holds — messages are emitted verbatim from the provider, parsed/cleaned; stagecoach appends nothing. | `internal/provider/parse.go`, `internal/generate/finalize.go` |

**Therefore the implementation work for this delta is a regression test + a defensive structural guard — not a behavior change.** The risk this FR exists to close is a *future* regression (a "helpful" fallback identity, a branding trailer, a `config init` that sets `user.name`), and the §20.2 invariant is what catches it.

---

## 3. Scope delta

### New requirement (in scope)

- **`FR-39a` — Commit-identity transparency (P0, → G3).** As quoted in §1.1 above. Adds one behavioral invariant (commits carry git's resolved identity, never a stagecoach brand) and one structural invariant (no production code path writes identity config or sets identity env).

### Modified requirements (note, no task needed)

- **§13.2 / §19 / §20.2** — cross-reference additions only (one sentence, one bullet, one invariant line). These are documentation of the new FR, not independent work items.

### Removed requirements

- None.

### Documentation impact

- **Mode A (doc-with-work):** the canonical FR text lives in `PRD.md` (already updated by the source commit). The only code-side doc touched is the existing comment at `internal/git/git.go:516-519` ("Identity: cmd.Env is NOT set here…") — the implementing task should add an explicit **`(FR-39a)`** tag to it so the guarantee is grep-discoverable from code. No `docs/` page currently discusses commit identity (it is an internal invariant), so no per-file doc update is required; if the task adds a one-line note to `docs/how-it-works.md`'s plumbing section, that rides with the test task.
- **Mode B (changeset-level docs):** **does not apply.** FR-39a is a defensive invariant the code already upholds; it is not a user-visible feature, adds no CLI flag, no config key, and changes no output. The README makes no AI-branding claim to retract, and the §19 security narrative is PRD-internal. A README/FAQ edit would be noise.

---

## 4. Plan

**One phase, one milestone, one task (with one subtask).** Sized to the actual change.

### Phase P1 — Commit-identity transparency regression net (FR-39a)

**Milestone P1.M1 — Assert the invariant the code already upholds.**

The deliverable is the §20.2 "commit-identity transparency" invariant plus a structural guard that the field note calls out ("no code path ever writes such a key"). Both land in the test tree; the only production change is a one-word `FR-39a` doc tag on the existing `runWithInput` comment.

#### Task P1.M1.T1 — Behavioral + structural assertion of FR-39a

- **Subtask P1.M1.T1.S1 — Commit-identity transparency test + structural guard + doc tag.**
  - **Behavioral invariant** (`internal/generate/invariants_test.go`, extend the `TestInvariants` §20.2 table — or a dedicated `TestCommitIdentityTransparency` alongside it): drive a successful `CommitStaged` against the stub harness (reuse the existing temp-repo + stubtest pattern), then assert on the resulting commit:
    1. Resolve the expected identity *before* the run (`git config user.name` + `user.email`, plus any `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env the test sets) and compare against `git log -1 --format='%an <%ae> | %cn <%ce>'` of the produced commit — they must match exactly.
    2. Assert the commit **message** contains no `Co-Authored-By:` / `Generated-by:` / `🤖` trailer that stagecoach did not place there (the stub's canned message is the only allowed content).
    3. **Transparency-without-authorship** case: pre-seed a repo-local `git config user.name "stagecoach agent"` + `user.email agent@stagecoach.local`, run, and assert the resulting commit IS stamped with those values (stagecoach inherits the override faithfully — correct) — proving stagecoach *passes identity through* rather than overriding it.
  - **Structural guard** (a cheap, fast test — e.g. `TestNoIdentityWritesInProduction` in `internal/git/git_test.go` or `internal/config/config_test.go`): walk the non-test `.go` files under `internal/` and `cmd/` and assert via `go/parser` or a substring scan that **no** production source contains `git config … user.name`, `git config … user.email`, or sets any of `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL`/`GIT_AUTHOR_DATE`/`GIT_COMMITTER_NAME`/`GIT_COMMITTER_EMAIL`/`GIT_COMMITTER_DATE` as an env key. This is the "no code path ever *writes* such a key" half of §20.2 — the regression net for a future "helpful" fallback. Exclude test files (which legitimately set `user.name` for scaffolding). Mirror the existing `committree_test.go` allow-list of test-only identity writes.
  - **Doc tag (Mode A):** append ` (FR-39a — commit-identity transparency)` to the existing "Identity: cmd.Env is NOT set here…" comment at `internal/git/git.go:516-519` so the guarantee is grep-discoverable from the code, and cite FR-39a in the `CommitTree` doc comment (`git.go:670`).
  - **Verification:** `go test ./internal/generate/ ./internal/git/ ./internal/config/` passes; the new test fails if anyone later adds a `GIT_COMMITTER_NAME=…` env to `CommitTree` or a `git config user.name` to bootstrap (inject such a change locally to confirm the guard fires, then revert).
  - **Mode A docs:** the `FR-39a` tag above is the only doc edit; no `docs/*.md` change required (FR-39a is an internal invariant, see §3).
  - **Out of scope:** the future opt-in `Co-authored-by: stagecoach` trailer flag (FR-39a explicitly defers it; v3 ships without it). No CLI flag, no config key, no provider/manifest change.

---

## 5. Risks & notes

- **No behavior risk.** FR-39a describes what the code already does; this delta adds a regression net, not new logic. The only way it can "fail" is if the structural guard is too broad (false-positive on a legitimate comment or test helper) — mitigated by excluding `_test.go` files and matching on actual `exec`/`config` call sites, not bare substrings.
- **Test-scaffolding identity writes stay.** `setIdentityConfig` (committree_test.go) and the `git config user.name Test` lines in generate/hook tests are *legitimate* test setup, not violations. The structural guard must allow-list test files (`!strings.HasSuffix(file, "_test.go")`) so it catches only production regressions.
- **No interaction with the v3.0 self-update work.** `stagecoach upgrade` makes no commits and touches no git identity; FR-39a is scoped to the commit path only.