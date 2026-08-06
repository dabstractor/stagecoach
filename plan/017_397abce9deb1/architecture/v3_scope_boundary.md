# v3.0 scope boundary — what is walled off (must NOT change)

PRD §9.29 intro + §10.5 are explicit: self-update is **walled off from the commit-generation core**.
This file is the negative-space checklist for plan 017. If any subtask seems to require touching the
items below, the design is wrong — stop and re-scope.

## `stagecoach upgrade` MUST NOT

- Acquire the per-repo run lock (`internal/lock`, FR52/§18.5).
- Read or mutate any repository, index, or ref (`internal/git`, `internal/generate`,
  `internal/decompose`).
- Invoke any provider / agent CLI (`internal/provider`, `internal/prompt`).
- Trigger `config.Load`'s first-run bootstrap write (FR-B3) — upgrade is repo-independent and must
  run outside a git repo. Its command `PersistentPreRunE` is a no-op (mirror `internal/cmd/lock.go`).
  The `[upgrade]` config table is read from the GLOBAL config file only; a per-repo `[upgrade]` is
  ignored with a `--verbose` note (FR-U10).
- Auto-elevate privileges (`sudo`/UAC). A printed command is fine; auto-`sudo` is forbidden (FR-U4).
- Overwrite a binary the detection believes a package manager owns, except via `--force` with a
  warning (FR-U1).
- Use `--dangerously-*` / bypass-permissions flags (n/a here, but hold the line).

## What stays byte-for-byte unchanged

- The commit path: `internal/git`, `internal/generate`, `internal/decompose`, `internal/provider`,
  `internal/prompt`, `internal/hook(s)`, `internal/integrate`, `internal/exclude`, `internal/signal`,
  `internal/watchdog`, `internal/ui`, `pkg/stagecoach`.
- The rescue protocol, CAS commit core, snapshot/freeze invariants (§13, §18).
- The provider manifests (`internal/provider/builtin.go`, `providers/*.toml`).
- Exit codes 0/1/2/3/5/124 semantics (only ADD `6` — do not remap existing ones).
- The existing `config upgrade` command (config-schema migration) — do not rename/repurpose it.
  Binary self-update is a NEW command `stagecoach upgrade`; config-schema update stays
  `stagecoach config upgrade`. Two different things; the help text must disambiguate.

## §19 "no network calls" — now scoped (amendment, not a relaxation)

- The commit path (§9.1–§9.28) makes NO network calls — **unchanged**. Verified: `net/http` is used
  nowhere in the repo today.
- `stagecoach upgrade` is the **explicit, named exception**: it fetches ONLY the project's own GitHub
  release artifacts + checksums. Never provider credentials, never a diff, never repo data.
- The security narrative in README/docs must reflect this scoping (Mode B doc task): "no network
  calls on the commit path; `stagecoach upgrade` is the one exception and it touches only release
  artifacts."

## Exit-code discipline (FR-U12)

| code | meaning (v3.0) |
|------|----------------|
| 0 | up-to-date / upgraded / printed-the-command (upgrade path) — same code as commit success |
| 1 | upgrade failure (general error) — same code as commit general error |
| 6 | **NEW** update-available (`--check`) — distinct from commit-path codes |
| 2/3/5/124 | commit-path only — never produced by `upgrade` |

`6` must be added to `internal/exitcode/exitcode.go` as `UpdateAvailable` and wired so a
`--check` that finds a newer version returns `exitcode.New(exitcode.UpdateAvailable, nil)` (or a
sentinel the `For()` map recognizes). It is NOT a commit-path code and must never be returned by the
commit path.

## Additive config (no migration)

The `[upgrade]` table (`channel`, `source_repo`) is OPTIONAL and ADDITIVE. Per FR-B4, additive
config changes MUST NOT bump `CurrentConfigVersion` (stays 3) and MUST NOT emit a load-time upgrade
advisory. An older config file without `[upgrade]` is fully valid; `Defaults()` seeds
`channel="stable"`, `source_repo="dabstractor/stagecoach"`.

## Distribution plumbing is build/release-only

npm wrapper / flake.nix / winget manifests / mise-asdf scripts add **no Go code to the binary's
commit path** and **no new FR surface** (PRD §10.5). They are CI/release/JS/Nix artifacts. Their
only interaction with the binary is the npm wrapper setting `STAGECOACH_INSTALL_METHOD=npm`, which
Phase 1's detection (FR-U2) reads.
