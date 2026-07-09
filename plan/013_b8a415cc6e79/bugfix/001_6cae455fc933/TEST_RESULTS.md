# Bug Fix Requirements

## Overview

End-to-end PRD validation of the Stagecoach implementation was performed by building the binary,
driving it against real git repositories with both a stub provider (`cmd/stubagent`) and a real
agent (`pi`/`zai/glm-5-turbo`), and exercising every major feature surface: the single-commit
plumbing path, dry-run, duplicate rejection, rescue/timeout, multi-turn fallback, work-description
mode, decomposition trigger routing, binary filtering, payload exclusions, the token-limit gate,
format modes (conventional/gitmoji/plain), templates, locale, `--edit` (including the empty-message
abort), `--no-verify` and the full hook lifecycle (pre-commit mutation re-treeing, the FR-V3
sweep freeze backstop, prepare-commit-msg recursion skip, post-commit), the per-repo run lock
(no-op fast path + Busy exit), provider manifests, `config init`/`upgrade`/`path`, `models`,
`providers list/show`, `hook install/uninstall/status`, and `integrate git-alias`.

Overall the implementation is high quality: the atomic-commit core, the freeze invariants
(FR-M1b/M1c/M1d), the token-limit closed-loop gate (FR3j), the hook execution path (FR-V1–V8),
and the rescue/CAS error handling all behave correctly under direct testing. The e2e test suite
(`//go:build e2e`) passes.

The issues below are concentrated in the **configuration-precedence layer for default-`true`
boolean fields** and a **documentation/implementation mismatch on the git-config keys**. The most
consequential one means a core, documented option (`auto_stage_all`) cannot be persistently
disabled through any correctly-documented method, and the failure is **silent** (the user's
explicit `false` is ignored, and stagecoach does the opposite of what was configured).

## Critical Issues (Must Fix)

_None found._ The atomic-commit core, repo-integrity invariants, and rescue protocol are sound.

## Major Issues (Should Fix)

### Issue 1: `auto_stage_all = false` in a TOML config file is silently ignored
**Severity**: Major
**PRD Reference**: §9.8 FR34 (precedence — config file is a layer that overrides built-in defaults); §9.4 FR16 ("if `auto_stage_all` is enabled (default: true)"); §16.2 (the config-file example lists `auto_stage_all`).
**Expected Behavior**: Per FR34, a config-file value (`~/.config/stagecoach/config.toml` or
`.stagecoach.toml`) overrides the built-in default. A user who writes `auto_stage_all = false`
expects stagecoach to *not* auto-stage when nothing is staged, and to instead exit with
"Nothing to commit." The config-file key is advertised as configurable in the bootstrap output
and `docs/configuration.md`.
**Actual Behavior**: `auto_stage_all = false` in any TOML config file is **silently ignored** —
stagecoach behaves as if `auto_stage_all = true` (the default). The config layer uses an
"only-true-propagates" overlay (`internal/config/file.go` `materialize`/`overlay`:
`if d.AutoStageAll { c.AutoStageAll = true }`), so a `false` value never reaches the resolved
`Config`. There is no warning.

Reproduction (isolated):
1. `git init`, seed a commit, leave a dirty working tree (e.g. `echo b > b.txt`, un-staged).
2. Create a config file with `[defaults]` `auto_stage_all = false` and a stub provider.
3. Run `stagecoach --config <file>`.
4. **Expected**: exit 2 "Nothing to commit." **Actual**: stagecoach auto-stages `b.txt` and commits
   it (exit 0), producing a commit the user explicitly tried to prevent.

This compounds with the next two issues: the *only* working persistent escape is an undocumented
camelCase git-config key, while every documented persistent method (TOML file, snake_case git
config, env var) either silently fails or errors.

**Suggested Fix**: Give `auto_stage_all` (and the other default-`true` boolean, `multi_turn_fallback`)
a real "explicitly set" signal so a file can set them `false`. The cleanest approach mirrors the
`*int` pattern already used for `diff_context`: decode the file-layer booleans as `*bool` (nil =
inherit lower layer; non-nil = override, including `*false`), and have `overlay`/`materialize`
copy the pointed-to value instead of the `if src.X { dst.X = true }` one-way form. At minimum,
emit a `--verbose`/stderr warning when a file sets `auto_stage_all = false` so the silent failure
becomes visible.

### Issue 2: Documented git-config key `stagecoach.auto_stage_all` is invalid; only the undocumented camelCase form works
**Severity**: Major
**PRD Reference**: §9.8 FR36 ("`stagecoach.auto_stage_all`"); §16.3 (git-config ini example uses `autoStageAll`); `docs/configuration.md` lines 210 & 218 (table row `stagecoach.auto_stage_all` with `git config --get --bool stagecoach.auto_stage_all`).
**Expected Behavior**: The git-config key documented in `docs/configuration.md` (and listed in PRD
FR36) should be settable, so users can persistently disable auto-stage-all via
`git config stagecoach.auto_stage_all false`.
**Actual Behavior**: Git rejects underscores in the final config-key component, so
`git config stagecoach.auto_stage_all false` fails with `error: invalid key:
stagecoach.auto_stage_all` and sets nothing. The implementation reads the camelCase key
`stagecoach.autoStageAll` (`internal/config/git.go`), which works but is **not** the key named in
FR36 or in the docs configuration table. Verified directly:
- `git config stagecoach.auto_stage_all false` → `error: invalid key` (exit 1)
- `git config stagecoach.autoStageAll false` → succeeds (exit 0) and correctly disables auto-stage.

So the *only* working persistent method contradicts both the PRD FR36 list and the
`docs/configuration.md` table; the method the docs tell users to use is un-settable.

**Suggested Fix**: Reconcile docs and code. Since git forbids underscores in the name component,
update `docs/configuration.md` (lines 210 and 218) and the PRD FR36 list to use the camelCase
keys the implementation actually reads (`stagecoach.autoStageAll`). (The same snake-vs-camel
divergence exists for the PRD's `stagecoach.auto_stage_all`; the implementation is internally
consistent in camelCase for all multi-word git keys — `stripCodeFence`, `tokenLimit`,
`diffContext`, `noVerify`, etc. — so the docs are the side that needs fixing.)

### Issue 3: No environment-variable source for `auto_stage_all`
**Severity**: Major (in combination with Issues 1 & 2)
**PRD Reference**: §9.8 FR35 (env vars use the `stagecoach_` prefix; "Others: …" implies the set is not closed); §15.2 (the precedence model treats env as a layer).
**Expected Behavior**: A `STAGECOACH_AUTO_STAGE_ALL=false` env var (mirroring
`STAGECOACH_PUSH`, `STAGECOACH_VERBOSE`, `STAGECOACH_NO_VERIFY`) should override the default, per
the `stagecoach_<SETTING>` convention in FR35.
**Actual Behavior**: No such env var is handled. `internal/config/load.go` `loadEnv` has no
`STAGECOACH_AUTO_STAGE_ALL` case (confirmed by grep). So the only ways to disable auto-stage-all
are: the per-invocation `--no-auto-stage` flag (not persistent), and the undocumented camelCase
git-config key from Issue 2. With the TOML path silently broken (Issue 1) and the documented
git-config path invalid (Issue 2), there is **no working, correctly-documented persistent way**
to disable `auto_stage_all`.

**Suggested Fix**: Add `STAGECOACH_AUTO_STAGE_ALL` (bool, DIRECT set so `false` works) to
`loadEnv`, mirroring `STAGECOACH_PUSH`/`STAGECOACH_NO_VERIFY`. Combined with fixing Issue 1, this
restores the full FR34 precedence ladder for this option.

## Minor Issues (Nice to Fix)

### Issue 4: `STAGECOACH_VERBOSE=2` is rejected (PRD §19 documents it for stdin-contents logging)
**Severity**: Minor
**PRD Reference**: §19 ("never the stdin contents unless `stagecoach_VERBOSE=2`").
**Expected Behavior**: `STAGECOACH_VERBOSE=2` enables logging of the stdin payload *contents*
(the diff), as documented in §19. At minimum it should not be a hard error.
**Actual Behavior**: `cfg.Verbose` is a `bool`, and `loadEnv` parses `STAGECOACH_VERBOSE` with
`strconv.ParseBool`, so `STAGECOACH_VERBOSE=2` fails load with:
`stagecoach: config: env config: STAGECOACH_VERBOSE: strconv.ParseBool: parsing "2": invalid
syntax` (exit 1). The code comments in `internal/ui/verbose.go` acknowledge VERBOSE=2 is
"deferred/out of scope," but the PRD documents it as a feature, so a user following the PRD hits
an error.

**Suggested Fix**: Either implement VERBOSE=2 (promote `Config.Verbose` to an int / add a
separate verbose-level), or explicitly reject `2` with a clearer message ("VERBOSE=2 is not yet
supported") and update the PRD/docs to mark it unimplemented. Today the failure is an opaque
parse error.

### Issue 5: `--verbose` payload-size line is missing for positional/flag-delivery providers
**Severity**: Minor
**PRD Reference**: §9.13 FR50 ("the payload size being delivered (byte count + a chars/4 token
estimate — the size only)"); the line is meant to "expose whether the token-limit gate actually ran."
**Expected Behavior**: `--verbose` prints the payload size for every provider invocation.
**Actual Behavior**: `internal/provider/executor.go` logs `vb.VerbosePayload(len(spec.Stdin))`.
For `stdin`-delivery providers (pi, claude, agy) `spec.Stdin` carries the payload and the size
prints correctly. For `positional`/`flag`-delivery providers (opencode, codex, cursor) the
payload is appended to `spec.Args` and `spec.Stdin == ""`, so `VerbosePayload(0)` is a no-op
(`bytes <= 0` guard) and **no payload-size line is emitted**. FR50's stated purpose (exposing
whether the token-limit gate ran) is therefore defeated for those three providers.

**Suggested Fix**: Compute the delivered payload size from the delivery mode —
`len(spec.Stdin)` for stdin, or the length of the trailing positional/`prompt_flag` argument for
positional/flag — before calling `VerbosePayload`.

### Issue 6: `--model` (global) does not override a per-role `[role.message]` model — surprising UX (by-spec)
**Severity**: Minor (UX note, not a correctness bug)
**PRD Reference**: §9.15 FR-R3 (per-role config beats the global `[defaults]`).
**Expected Behavior** (per spec): `--model X` sets the *global* default; a `[role.message]` model
in the config still wins for the message role, and the user must use `--message-model X` to
override it.
**Actual Behavior**: This is correctly implemented, but is a real footgun. With a populated global
config (e.g. `[role.message] model = "zai/glm-5-turbo"`), running
`stagecoach --model glm-5.2` silently keeps the per-role model and the bare `glm-5.2` is never
validated (FR-R5b never fires) — the rendered command uses the config's model. A user expecting
`--model` to "just use this model for this run" sees no error and the wrong model is used.

**Suggested Fix**: Consider a one-line `--verbose` hint when an explicit `--model`/`--provider` is
shadowed by a per-role override (e.g. "note: --model shadowed by [role.message].model; use
--message-model to override"), or document the precedence gotcha prominently in the CLI help text
for `--model`. No behavioral change required.

## Testing Summary
- Total tests performed: ~45 manual end-to-end scenarios across 12 throwaway repos (stub + real
  `pi` agent), plus the full `go test ./...` suite (all passing) and the `//go:build e2e` suite
  (passing).
- Passing (verified correct): single-commit happy path (stub + real pi); dry-run; duplicate
  rejection + rescue; timeout (exit 124, no commit); CAS-freeze; binary filtering + placeholders;
  payload exclusions (payload-only, commit stays faithful); `.stagecoachignore`; token-limit
  water-fill + truncation sentinel + closed-loop fit; format modes (conventional/gitmoji/plain) +
  unknown-mode hard error; template (`$msg` substitution + missing-`$msg` error); locale; `--edit`
  (message edit + empty-message abort → exit 1, no rescue); `--no-verify` (skips pre-commit/
  commit-msg, keeps prepare/post-commit); full hook lifecycle (pre-commit mutation re-tree +
  ReconcileIndex leaves `git status` clean; FR-V3 sweep → hard error; prepare-commit-msg recursion
  skip; post-commit); per-repo run lock (no-op fast path exit 0 + Busy exit 5); FR-R5b bare-model
  hard error on multi-backend providers; work-description mode (description-first payload, append
  session gate); multi-turn fallback (triggers only on `session_mode="append"`); `config init`
  bootstrap (populated, per-role); `config upgrade` v2→v3 (`default_provider` fold); `models`
  (live + curated fallback + `--all`); `providers list/show`; `hook install/uninstall/status`
  (foreign-hook refusal); `integrate git-alias` (install/remove); decompose routing + one-file
  planner bypass label; validation errors (`diff_context` range, `--commits` negative,
  unknown format/template).
- Failing: Issues 1–6 above (4 functional/config, 2 minor).
- Areas with good coverage: the snapshot/atomic-commit core, freeze invariants, hook execution,
  rescue/CAS/timeout error mapping, provider manifest rendering (FR-R5b), prompt construction,
  token-limit gate, parse pipeline.
- Areas needing more attention: the **config-precedence layer for default-`true` booleans**
  (`auto_stage_all`, `multi_turn_fallback`) — the "only-true-propagates" file overlay silently
  breaks the documented ability to set them `false` via file; and the **git-config key naming**
  (docs/PRD say snake_case which git rejects; code reads camelCase).
