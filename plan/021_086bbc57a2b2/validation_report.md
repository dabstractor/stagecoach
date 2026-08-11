# Stagecoach — Validation Report

**Date:** 2026-08-11  
**Validator:** automated + manual workflow simulation  
**Codebase:** ~83,400 LOC Go (20 internal packages), v3.4 spec (`+body` format variants)  
**Commit:** `a96f06a` (main, working tree clean)

---

## Summary

Stagecoach is a **mature, production-ready** Go CLI. The full validation suite — build, static
analysis, race-enabled unit/integration tests, the ≥85 % coverage gate, the build-tagged e2e
throwaway-repo harness, and a 25-scenario stub-driven end-to-end workflow battery mirroring real
user journeys — **passes with zero failures**. The two flagship safety invariants (snapshot-based
atomic commits / stage-while-generating, and commit-identity transparency FR-39a) hold under direct
adversarial testing. The historical bug sites (config-upgrade clobber FR-B8/B9, the v2.2 arbiter
freeze loophole FR-M1d, multi-backend model-prefix routing FR-R5b) are all verified fixed.

**Two minor polish items** were found (both non-blocking, neither affects correctness or safety).
They are listed below. No critical or major issues were found.

---

## Validation coverage (what was run)

### Phase 1 — Build & cross-compile (PRD §21.2 / G9)
`go build ./...` plus all six goreleaser targets (`linux/darwin/windows × amd64/arm64`,
`CGO_ENABLED=0`). **All compile clean.**

### Phase 2 — Static analysis
`go vet ./...` **clean**. `golangci-lint` (CI pins v1.61) ran clean when installed locally.
`govulncheck` is run by CI (not installed locally — skipped with a note). No `TODO`/`FIXME`/`panic()`
in shipped (non-test, non-stub) source.

### Phase 3 — Unit + integration tests (PRD §20.1/§20.4)
`go test -race -count=1 ./...` — **all 20 packages pass** with the race detector (~95 s). Covers
parsing, command rendering, prompt construction, dedupe, config precedence, git plumbing, the
generate/decompose orchestrators, lock, signal, hooks, integrate, and upgrade.

### Phase 4 — Coverage gate (PRD §20.3, ≥85 %)
| package | coverage |
|---|---|
| `internal/git` | 85.9 % |
| `internal/provider` | 90.0 % |
| `internal/generate` | 89.3 % |
| `internal/config` | 87.1 % |

**All above the 85 % floor.**

### Phase 5 — E2E throwaway-repo suite (PRD §20.5)
`go test -tags=e2e ./internal/e2e/...` — **passes** (32 s). Exercises real `stagecoach`
subprocesses against fresh `git init` temp repos with a stub provider, covering decompose, the
freeze boundary (concurrent-file exclusion across the arbiter gate FR-M1d), lock contention,
orphan reclamation (FR-K1/K3/K4), and hook scenarios.

### Phase 6 — End-to-end user-workflow battery (25 scenarios)
Stub-driven, each in an isolated fresh repo, run **sequentially** (the per-repo run lock serializes
anyway). Every scenario mirrors a documented user journey (README, `docs/cli.md`):

| # | Workflow | Result |
|---|---|---|
| 6.1 | Single-commit path (staged changes) | ✓ |
| 6.2 | `--dry-run` does not move HEAD | ✓ |
| 6.3 | Clean tree → exit 2 | ✓ |
| 6.4 | Unknown provider → exit 1 | ✓ |
| 6.5 | **FR-39a** commit-identity transparency (no branding) | ✓ |
| 6.6 | Rescue on empty generation → exit 3, HEAD unchanged | ✓ |
| 6.7 | Duplicate rejection → exit 3, candidate preserved | ✓ |
| 6.8 | Multi-commit decompose (file-disjoint fast path) → 2 commits | ✓ |
| 6.9 | `--single` escape hatch (one commit from dirty tree) | ✓ |
| 6.10 | Binary filtering FR3a (`[binary]` placeholder, no diff body) | ✓ |
| 6.11 | Payload exclusion FR-X (payload-only, commit faithful) | ✓ |
| 6.12 | `+body` format variants (auto/conventional/gitmoji/plain) force a body | ✓ |
| 6.13 | Invalid format grammar (`bad+body+body`) → exit 1 | ✓ |
| 6.14 | `--template '$msg (#205)'` wraps message | ✓ |
| 6.15 | Merge conflict in index → exit 1 (FR8) | ✓ |
| 6.16 | Root repo (no parent) → root commit (§13.5) | ✓ |
| 6.17 | `config init` populated bootstrap (FR-B1) | ✓ |
| 6.18 | `config init --force` preserves active settings (FR-B8) | ✓ |
| 6.19 | `config upgrade` v2→v3 folds `default_provider` into model prefix (FR-B7) | ✓ |
| 6.20 | `hook install/status/uninstall` cycle | ✓ |
| 6.21 | `hook exec`: plain `git commit` gets generated message (FR-H4) | ✓ |
| 6.22 | `models` lists without HTTP (FR-L1) | ✓ |
| 6.23 | `lock status` read-only (FR-K4) | ✓ |
| 6.24 | `integrate list` (FR-I1) | ✓ |
| 6.25 | `--verbose` shows payload SIZE only, never contents (FR50) | ✓ |

**Manually verified beyond the script:** one-file planner bypass (FR-M2b), `--edit` user-authored
message (FR-E3), `git commit -m` pass-through in hook mode, `config upgrade` no-op on inert files
(FR-B9), `upgrade --check` exit semantics, `STAGECOACH_FORMAT=""` env accepted as `auto`, npm wrapper
`node --check`, `shellcheck install.sh` clean, `go mod verify` + tidy.

---

## Invariants verified (the safety-critical core)

- **Atomic / snapshot-based commits (§13).** Failed generation leaves HEAD and the index
  byte-for-byte unchanged; rescue prints the frozen tree SHA + exact recovery command (exit 3).
- **Stage-while-generating freeze (FR-M1b/M1c/M1d).** A file added to the working tree after a
  decompose run begins appears in **no** commit and remains in the working tree afterward.
- **Commit-identity transparency (FR-39a).** Commits are authored/committer-stamped as the user's
  own `user.name`/`user.email`; no `stagecoach` identity, no `Co-Authored-By:`/`Generated-by:`
  trailer. Verified directly.
- **CAS `update-ref`.** Two-arg form used; never force-updates.
- **Per-repo run lock (FR52).** Contention exits 5 with a clear message; never force-breaks.
- **No network on the commit path (§19).** `models`/`providers` are CLI- or manifest-sourced;
  `upgrade` is the sole, walled-off exception.

---

## Issues found

### Issue 1 — `--format ""` (explicit empty CLI value) rejected, while empty env/config accepted
**Severity: Minor (input-handling consistency)**  
**Spec ref:** §9.19 FR-F1 (`format` default `auto`)  
**Component:** `internal/config` (format validation)

An explicit empty string passed via the `--format` CLI flag is a hard error:
```
$ stagecoach --format "" …
stagecoach: config: format: invalid format "" (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)
exit 1
```
But the **same empty value** is accepted (and treated as the `auto` default) when it arrives from:
- the env var: `STAGECOACH_FORMAT=""` → works (auto)
- the config file: `[generation] format = ""` → works (auto)
- no value at all (flag omitted) → works (auto)

So the empty value is inconsistent across input sources: env/config treat `""` as "use default",
while the CLI flag treats it as malformed. A user who scripts `stagecoach --format "$FMT"` with
`FMT` accidentally unset/empty gets a confusing hard error where the env/config analog silently
does the right thing.

**Repro:** `validate.sh` does not assert this (it is the one deliberate negative case); confirmed
manually against the built binary. Likely a one-line fix — treat an empty CLI `--format` as unset
(consistent with env/config) before running `validateFormat`.

**Risk:** Very low. Does not affect correctness, safety, or the default path. Only bites the narrow
case of an explicitly-empty CLI value.

---

### Issue 2 — `--work-description` failure on a non-session provider does not surface the cause
**Severity: Minor (UX / error reporting)**  
**Spec ref:** §9.26 FR-W4 (work-description requires `session_mode="append"`); §9.24 FR-T8  
**Component:** `internal/generate` (workdesc → rescue path)

`--work-description` mode reuses the §9.24 multi-turn session machinery and therefore requires a
provider whose manifest declares `session_mode = "append"`. Only **pi** ships that today; the other
four built-ins (**claude, codex, cursor, agy**) ship `session_mode = ""`. When a user runs
`--work-description` against one of those four, stagecoach correctly routes to the rescue path
(exit 3, HEAD unchanged) — but it reports only the **generic** rescue recipe and never names the
cause:

```
$ stagecoach --work-description "..." --provider claude
↳ Generating with stub…
❌ Commit generation failed.
… (generic recovery recipe; no mention of session_mode / provider capability) …
exit 3
```

Even `--verbose` does not surface the reason ("provider `claude` has no `session_mode`; work-
description mode needs a session-append-capable provider"). The cause is known internally — the
source comment at `internal/generate/generate.go:316` documents that the session-mode gate yields
the failure cause — but it is not propagated to the user-facing message.

**Repro:** confirmed manually with a non-`append` stub provider (`STAGECOACH_STUB_OUT="…" stagecoach
--work-description "x"` → generic rescue, no cause, even with `--verbose`).

**Risk:** Low. The default provider (pi) is session-capable, so the common path is unaffected. Only
users who (a) adopt `--work-description` and (b) have switched to a non-pi provider hit it. The
failure is safe (HEAD/index untouched); it is merely unhelpful. Surfacing the cause in the rescue
message (or at least under `--verbose`) would let the user self-diagnose.

---

## Conclusion

The codebase passes a comprehensive, multi-layer validation (build × 6 targets, race tests,
coverage gate, e2e harness, and a 25-scenario workflow battery). Every P0/P1 functional requirement
exercised behaves per spec, and every safety invariant holds under direct testing. The two findings
are minor polish items (an input-handling inconsistency and an unsurfaced error cause); neither
blocks production use.