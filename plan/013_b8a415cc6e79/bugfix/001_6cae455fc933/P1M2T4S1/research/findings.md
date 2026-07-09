# Research Notes — Doc Sweep (stale config-precedence references)

> **Dispatch note**: This research dir + the requested write path are labelled `P1M2T4S1`, but the
> `item_title`/`item_description` (the actual work contract) describe the **"Sweep README.md and
> overview docs for stale config-precedence references"** task, which `tasks.json` (line 190) maps to
> **P1.M2.T7.S1**. `P1.M2.T4.S1` is a *different* task ("Gracefully reject STAGECOACH_VERBOSE=2").
> The PRP honors the detailed item contract (doc sweep) and is written to the requested path.
> The VERBOSE=2 work is a *dependency* referenced inside this PRP (it touches `docs/cli.md`/configuration.md),
> not the thing being built here.

## Current state of each target file (read 2026-07-09)

### docs/configuration.md — ALREADY CONSISTENT (per-feature Mode A updates landed)
- L87: `# auto_stage_all = true` (template, fine)
- L110: `multi_turn_fallback` comment notes "set false to DISABLE (now honored via file/git-config ...)" — mostly fine
- L133: defaults table `auto_stage_all | true` (fine)
- L140: `multi_turn_fallback | true` (fine)
- L157: `no_verify` note — correctly states auto_stage_all/multi_turn_fallback are `*bool` and do NOT have the only-true-propagates limitation. CORRECT.
- L166: `multi_turn_fallback` note — **MISLEADING PHRASING**: "or via `git config stagecoach.autoStageAll`-style `*bool` behavior". There is NO `stagecoach.multiTurnFallback` git key (see git.go below), so this references the wrong field. **CLEANUP TARGET.**
- L179: `STAGECOACH_VERBOSE` row (T4 will finalize VERBOSE=2 wording)
- L200/201: env rows `STAGECOACH_AUTO_STAGE_ALL` / `STAGECOACH_MULTI_TURN_FALLBACK` — present, correct
- L212: INI `autoStageAll = true` (camelCase, fixed in T3)
- L220: git-config table `stagecoach.autoStageAll` (camelCase, fixed in T3)

### docs/cli.md — REAL DRIFT (3 lines)
- L31: `| --no-auto-stage | bool | false | — | — | ...` → Env var + Git config columns are `—` but
  `STAGECOACH_AUTO_STAGE_ALL` (inverse) and `stagecoach.autoStageAll` now exist. **FIX.**
- L63: "The behavioral flags (`--all`, `--no-auto-stage`, `--dry-run`) have no env-var or git-config
  analogs." → STALE: `--no-auto-stage` now has BOTH. Drop it from the list (leave `--all`, `--dry-run`). **FIX.**
- L396: env→flag→gitconfig table `| --no-auto-stage | — | — |` → same drift. **FIX.**
- L25 (`--model`) + help text: cross-check vs T6 (--model shadow hint) once T6 lands.
- L28/L393 (`--verbose`/`STAGECOACH_VERBOSE`): cross-check vs T4 (graceful VERBOSE=2 reject) once T4 lands.

### README.md — CLEAN (verify-only, optional enhancement)
- L264: "**Config precedence** (highest → lowest): CLI flags > STAGECOACH_* env > repo git config
  (stagecoach.*) > repo .stagecoach.toml > global config file > provider defaults > built-in defaults."
  → ACCURATE, lists all 5 layers. No stale "cannot disable" language anywhere in README.
- Features table (L56–67): mentions multi-turn fallback generically; does NOT mention auto_stage_all
  configurability. Optional enhancement: add a one-line note that auto_stage_all/multi_turn_fallback are
  tunable through all layers. (Keep lightweight — README is a high-level overview.)

### docs/how-it-works.md — CLEAN (verify-only)
- L275–276: multi-turn trigger conditions, accurate ("multi_turn_fallback is enabled (default true)").
- L327: "stagecoach auto-stages all (even with `auto_stage_all` disabled)" — this is VALID
  work-description-mode behavior, NOT a stale limitation. Leave as-is.

## Code-level ground truth (the contract each doc must match)

`internal/config/git.go` reads EXACTLY these `stagecoach.*` keys (all camelCase):
provider, model, output, format, locale, template, timeout, **autoStageAll**, verbose,
stripCodeFence, push, noVerify, maxDiffBytes, maxMdLines, tokenLimit, diffContext,
maxDuplicateRetries, subjectTargetChars.

**CRITICAL ACCURACY FACT**: There is **NO** `stagecoach.multiTurnFallback` git-config key, and
**no** CLI flag for multi_turn_fallback. So:
- `auto_stage_all` → configurable via flag (`--no-auto-stage`, inverse) + env (`STAGECOACH_AUTO_STAGE_ALL`)
  + git-config (`stagecoach.autoStageAll`) + TOML. ALL 4 layers. ✓
- `multi_turn_fallback` → configurable via env (`STAGECOACH_MULTI_TURN_FALLBACK`) + TOML ONLY.
  No flag, no git-config key. Disable per-provider alternatively via `session_mode = ""`.

⇒ The doc sweep must NOT let any overview doc claim multi_turn_fallback is flag/git-config settable.
The item contract's blanket "fully configurable through all layers" is literally true only for
auto_stage_all; docs must state multi_turn_fallback's real (narrower) surface.

## Validation (pure docs — no code/test changes)
- `grep -rn -iE "cannot disable|only-true-propagates|only true propagates" docs/ README.md`
  → only the no_verify/push notes (which are correct AND explicitly exempt auto_stage_all/multi_turn).
- `grep -rn "no env-var or git-config analogs" docs/cli.md` → sentence updated to drop `--no-auto-stage`.
- `grep -rn "stagecoach.auto_stage_all" docs/ README.md` → none (snake_case git key fully purged).
- `go build ./...` and `go test ./...` unchanged (no behavioral change) — confirms docs-only.
