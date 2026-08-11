# Delta PRD — v3.4: `+body` format variants (forced body output)

**Base spec:** `spec/SPEC.md` at v3.4 (revision block committed in `68f023b`)
**Prior session:** `plan/020_6979db625159` — implemented v3.3 (Winget→Chocolatey + model-example cleanup); both phases Complete.
**Scope of this delta:** implement the v3.4 spec change in CODE. The spec text itself is already committed; only the implementation, tests, and user-facing docs lag.

---

## 0. What changed (diff analysis)

The v3.3 → v3.4 PRD delta is a **single, small, cohesive feature**: an optional `+body` suffix on every `--format` base that forces a subject-plus-body message regardless of repo history shape. It is message-role prompt surface only.

**Net spec diff** (commit `68f023b`, 4 files, +15/−8 lines):
- New revision block in `SPEC.md` metadata (v3.4).
- **G16** amended — notes the `+body` variant.
- **US28** added — maintainer wants `--format conventional+body` etc.
- **§9.19 FR-F1** amended — grammar is now `<base>[+body]`; anything outside it is a hard error.
- **§9.19 FR-F9** added (new) — the `+body` modifier semantics (overrides FR12 conditional multi-line rule; subject contract untouched; composes with locale/context/template; applies everywhere a message is produced per FR-F5).
- **§15.2** `--format` flag row amended — shows `<base>[+body]`.
- **§16.2** config `format` comment amended.
- **§17.8** amended — "Three orthogonal deltas" → "Four"; new "Body forcing (`+body`, FR-F9)" paragraph with the verbatim body directive text.

**Explicitly out of scope** (per the v3.4 revision block): no commit/CAS/rescue/lock/provider changes; message-role prompt surface only. The `gitui` removal visible in uncommitted `README.md`/`docs/cli.md` is a **separate** cleanup (matches commit `592e55f`, spec-only) and is **not** part of this delta.

---

## 1. Implementation surface (verified against current code)

The format subsystem already exists and is well-factored. The `+body` modifier threads through exactly these seams:

| File | Location | Current state | Change |
| --- | --- | --- | --- |
| `internal/config/load.go` | `validFormats` (L571) + `validateFormat` (L574–582) | closed set `{auto, conventional, gitmoji, plain}`; rejects everything else | Accept `<base>[+body]` grammar; reject `+body` alone / on unknown base / repeated / other suffixes |
| `internal/prompt/format.go` | `formatScaffoldBody`, `buildFormatSystemPrompt` | switches on base; selects `multilineRuleAllow`/`Single` by `hasMultiline` | Add the body-forcing directive constant; add a `<base>[+body]` parser; thread `forceBody` so the multi-line rule is replaced by the body directive when set |
| `internal/prompt/system.go` | `BuildSystemPrompt` (L190), `BuildFallbackPrompt` (L~178) | take `format` string, dispatch auto vs scaffold | Parse `forceBody` from format; `auto+body` keeps the examples block but swaps the multi-line rule for the body directive |
| `internal/prompt/planner.go` | `BuildPlannerSystemPrompt` (L142) | appends `formatScaffoldBody(format)` for non-auto | Keys scaffold on BASE only (subject contract unchanged); `+body` affects the planner only via its FR-M11 single-call message, which reuses the message-role path — no planner-prompt change needed for the partitioning prompt |
| `internal/config/config.go` | `Format` field doc (L106–109) | lists 4 modes | Document the `<base>[+body]` grammar |
| `internal/config/bootstrap.go` | config template comment (L376) | `# auto\|conventional\|gitmoji\|plain` | Add `+body` note |
| `internal/cmd/root.go` | `--format` flag help (L202) | `auto\|conventional\|gitmoji\|plain` | Add `+body` note |

**Tests to update/add:**
- `internal/config/load_test.go` — `TestValidateFormat` (L1354): add `+body` variants as valid; add invalid cases (`+body` alone, `auto+body+body`, `bogus+body`, `auto+Body`).
- `internal/prompt/format_test.go` — `TestFormatScaffoldBody` (keys on base, unaffected by suffix once parsed) + new `buildFormatSystemPrompt(..., forceBody=true)` cases asserting the body directive replaces the multi-line rule.
- `internal/prompt/system_test.go` — new `auto+body` case: examples block retained AND body directive present (not `multilineRuleAllow`/`Single`).

**Docs (Mode A — ride with work):**
- `docs/cli.md:38` — `--format` table row.
- `docs/configuration.md:116, 216, 247` — config `format` examples.
- `internal/config/bootstrap.go:376` (user-facing config template) + `internal/cmd/root.go:202` (`--help`) — ride with the code task.

---

## 2. Design decisions (locked)

1. **Parse once, at the seam.** A single helper `splitFormat(format string) (base string, forceBody bool)` splits on a trailing `+body` (case-sensitive, exactly once). `validateFormat` calls it and validates the BASE against the existing 4-element set; `buildFormatSystemPrompt` / `BuildSystemPrompt` call it to decide rule selection. The `formatScaffoldBody` dispatch stays keyed on BASE — the subject contract is untouched (FR-F9: "`+body` is a pure, orthogonal body-forcing modifier").
2. **Grammar is strict (FR-F1).** Valid: `auto`, `conventional`, `gitmoji`, `plain`, and each with exactly one trailing `+body`. Invalid (hard error, exit 1): `+body` alone, `<base>+body+body`, `<unknown>+body`, `<base>+Body` (case), `<base>+body ` (trailing space), any other suffix. The error message names the offending value and the valid grammar — mirroring the existing `validateFormat` message shape.
3. **`auto+body` is the one special case.** It keeps the §17.1 style-examples block (auto learns the subject style) but replaces the `<multi-line rule>` block with the body directive. Concretely in `BuildSystemPrompt`: when `forceBody`, emit the examples block as today, then the body directive constant instead of `multilineRuleAllow`/`multilineRuleSingle`, then the subject-target line. `BuildFallbackPrompt` (new-repo, ≤1 commit) gains the same: `auto+body` on a new repo is the conventional fallback subject + forced body.
4. **The body directive is a single verbatim constant** (committed from §17.8, no trailing newline — `format.go`'s package convention):

   > `ALWAYS follow the subject with a body — a blank line, then a wrapped (~72-column) explanation of what this change does and why. Use a short bullet list only when the change has several distinct parts. The subject above still follows its format contract.`

   It replaces the multi-line rule block in both the auto-examples topology (§17.1) and the non-auto scaffold topology (`buildFormatSystemPrompt`). It does NOT alter the subject scaffold (`conventionalScaffold` / `gitmojiScaffoldInstruction` / plain-none) — the subject contract is unchanged by the suffix.
5. **No new config field, no new flag.** `+body` is part of the `Format` string value, resolved through the existing 5-layer precedence and validated at the tail of `Load()` exactly as today. `--format`, `STAGECOACH_FORMAT`, `stagecoach.format`, `[generation].format` all gain `+body` for free.
6. **Composes unchanged.** `--locale` (FR-F6) appends after the body directive; `--template` (FR-F8) wraps the full `$msg` (subject + body); duplicate rejection (§9.7) stays subject-based. No FR-F7 (`--context`) interaction. Per FR-F5 the body-forcing applies everywhere a message is produced (message role, planner FR-M11 shortcut, arbiter commit) — all of which route through `BuildSystemPrompt`/`buildFormatSystemPrompt`, so the single seam covers them.

---

## 3. Backlog

### Phase P1 — Implement the `+body` format modifier (message-role prompt surface)

A single cohesive phase: grammar parsing + validation + prompt assembly, with tests and user-facing docs riding along. No commit/CAS/rescue/lock/provider touch.

#### Milestone P1.M1 — Core: grammar, validation, prompt assembly

**Task P1.M1.T1 — `+body` grammar parsing + prompt assembly (+ inline doc/flag-help updates)**

Implement the `<base>[+body]` grammar end-to-end through the format subsystem, plus the user-facing config-template and `--help` text. The body-forcing directive replaces the multi-line rule; the subject scaffold is unchanged.

- **Subtask P1.M1.T1.S1 — `internal/prompt/format.go`: add `splitFormat` parser + body directive constant + thread `forceBody` through `buildFormatSystemPrompt`**
  - Add `bodyForceDirective` const (verbatim from §17.8 / Design Decision 4 above; no trailing newline).
  - Add `splitFormat(format string) (base string, forceBody bool)`: returns `(format, false)` when no `+body` suffix; `(base, true)` when format == `base + "+body"`; the caller validates `base`. Case-sensitive on the suffix.
  - Change `buildFormatSystemPrompt` signature to `buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int)` → internally `base, forceBody := splitFormat(format)`; when `forceBody`, write `bodyForceDirective` in place of the `multilineRuleAllow`/`multilineRuleSingle` selection. `formatScaffoldBody` continues to receive `base` (subject contract keyed on base). Keep the function pure (no I/O) so it stays unit-testable.
  - **PRD selectors:** §9.19 FR-F1/FR-F9, §17.8.
  - **Docs:** [Mode A] none beyond inline code comments — `format.go` has no user-facing surface.

- **Subtask P1.M1.T1.S2 — `internal/prompt/system.go`: thread `forceBody` through `BuildSystemPrompt` + `BuildFallbackPrompt` (the `auto+body` special case)**
  - `BuildSystemPrompt`: `base, forceBody := splitFormat(format)`. When `forceBody`, the auto-examples topology emits the examples block as today but writes `bodyForceDirective` instead of the `multilineRuleAllow`/`multilineRuleSingle` branch. Non-auto `+body` delegates to `buildFormatSystemPrompt` (S1) which already handles it. Dispatch `formatScaffoldBody(base)` (not the raw `format`) so the scaffold keys on the base.
  - `BuildFallbackPrompt`: parse `forceBody`; when set, the §17.2 fallback gains the body directive after the essence line (the new-repo `auto+body` case). The non-auto new-repo path delegates to `buildFormatSystemPrompt`.
  - Preserve the FR-F1 byte-identity guarantee: `auto` (no suffix) + empty locale is byte-identical to today; only a present `+body` changes bytes.
  - **PRD selectors:** §9.19 FR-F1/FR-F9, §17.1, §17.2, §17.8.
  - **Docs:** [Mode A] none — `system.go` has no user-facing surface.

- **Subtask P1.M1.T1.S3 — `internal/config/load.go`: extend `validateFormat` to the `<base>[+body]` grammar**
  - `validateFormat`: `base, forceBody := splitFormat(format)`; validate `base` against the existing `validFormats` set; reject if `forceBody` and `base` is not a valid base (covers `+body` alone and `bogus+body`). The slice `validFormats` stays the 4 bases (it documents the base set; the suffix is grammar, not a mode). Update the error message to name the grammar (e.g. `invalid format %q (valid: <base>[+body], base ∈ auto, conventional, gitmoji, plain)`).
  - **PRD selectors:** §9.19 FR-F1.
  - **Docs:** [Mode A] none — `validateFormat` output is an error string, not a doc surface.

- **Subtask P1.M1.T1.S4 — User-facing config/flag text: `internal/config/config.go` Format doc, `internal/config/bootstrap.go` template comment, `internal/cmd/root.go` `--format` help**
  - `config.go` `Format` field doc (L106–109): update to the `<base>[+body]` grammar, note `+body` forces a body (FR-F9).
  - `bootstrap.go` template comment (L376): `# <base>[+body]: auto|conventional|gitmoji|plain, each optionally +body; unknown = hard error (exit 1)`.
  - `root.go` `--format` flag help (L202): `Message format: <base>[+body] — auto|conventional|gitmoji|plain, append +body to force a subject+body (...)`.
  - **PRD selectors:** §15.2 (`--format`), §16.2 (config `format`).
  - **Docs:** [Mode A] these ARE the user-facing config/flag surfaces — they ride with this subtask.

#### Milestone P1.M2 — Tests + user-facing docs sync

**Task P1.M2.T1 — Test coverage + docs sync (Mode A)**

Close the test gaps opened by P1.M1 and sync the user-facing docs (`docs/cli.md`, `docs/configuration.md`) off the old 4-mode list to the `<base>[+body]` grammar.

- **Subtask P1.M2.T1.S1 — Tests: `load_test.go` validation, `format_test.go` assembly, `system_test.go` `auto+body`**
  - `internal/config/load_test.go` `TestValidateFormat` (L1354): add valid cases `auto+body`, `conventional+body`, `gitmoji+body`, `plain+body`; add invalid cases `+body`, `auto+body+body`, `bogus+body`, `auto+Body`, `auto+body ` (trailing space); update the asserted error substring to the new grammar message.
  - `internal/prompt/format_test.go`: add `splitFormat` table test; add `buildFormatSystemPrompt("<base>+body", …)` cases asserting `bodyForceDirective` is present and neither `multilineRuleAllow` nor `multilineRuleSingle` is; assert the subject scaffold (`conventionalScaffold` etc.) is still present for `conventional+body`/`gitmoji+body`.
  - `internal/prompt/system_test.go`: add an `auto+body` case — examples block present (auto) AND `bodyForceDirective` present AND `multilineRuleAllow`/`Single` absent; add a `conventional+body` end-to-end case through `BuildSystemPrompt`.
  - **PRD selectors:** §9.19 FR-F1/FR-F9, §17.8.
  - **Docs:** [Mode A] none — test-only.

- **Subtask P1.M2.T1.S2 — `docs/cli.md` + `docs/configuration.md`: `--format`/config `format` references → `<base>[+body]`**
  - `docs/cli.md:38` `--format` table row: `Message format: <base>[+body] — auto (style learning) | conventional | gitmoji | plain; append +body to force a subject+body. Unknown = hard error (exit 1).`
  - `docs/configuration.md:116` (config template comment), `:216` (env example), `:247` (git-config table row): update each to the `<base>[+body]` grammar; keep the "unknown = hard error" note.
  - **Acceptance:** `rg -n "auto\|conventional\|gitmoji\|plain" docs/cli.md docs/configuration.md` returns no row that omits the `+body` option (the grammar is stated wherever the 4 bases are listed).
  - **PRD selectors:** §15.2, §16.2.
  - **Docs:** [Mode A] this IS the docs work.

**Task P1.M2.T2 — Sync changeset-level documentation (Mode B)**

Final coherence sweep depending on all above. Verifies the whole delta ships consistent.

- **Subtask P1.M2.T2.S1 — README + overview coherence sweep across the `+body` delta**
  - `rg -n "conventional.*gitmoji.*plain\|gitmoji.*plain" README.md docs/` — any feature-blurb or format list that should mention `+body` now does; any that lists the 4 bases states the `+body` option.
  - If README has a "message shaping" or `--format` blurb, ensure it names `+body` (mirroring G16/US28). If it has an example invocation, optionally add `stagecoach --format conventional+body`.
  - `go test ./...` green; `go vet ./...` clean.
  - **PRD selectors:** G16, US28, §15.2, §16.2.
  - **Docs:** [Mode B] this IS the changeset-level sync.

---

## 4. Acceptance

- `go test ./internal/config/... ./internal/prompt/... ./internal/cmd/...` green, including new `+body` validation and assembly cases.
- `stagecoach --format conventional+body --dry-run` produces a subject (`type(scope): description`) **plus** a body; `stagecoach --format plain+body --dry-run` produces a plain subject plus a body; `stagecoach --format auto+body --dry-run` keeps learned subject style plus a body — all regardless of repo history shape.
- `stagecoach --format bogus+body` exits 1 with a grammar-naming error; `stagecoach --format auto+body+body` exits 1.
- `auto` (no suffix) output is byte-identical to pre-delta (FR-F1 byte-identity preserved).
- `docs/cli.md`, `docs/configuration.md`, the bootstrap config template, and `--format --help` all state the `<base>[+body]` grammar.
- No commit/CAS/rescue/lock/provider file is touched by this delta.

---

## 5. Reference to completed work

- The format subsystem (`internal/prompt/format.go`, `system.go`, `planner.go`; `internal/config/load.go` `validateFormat`) was implemented in prior sessions and is the baseline. This delta extends it in place — no re-implementation.
- `validFormats` (load.go:571), `formatScaffoldBody`/`buildFormatSystemPrompt` (format.go), `BuildSystemPrompt`/`BuildFallbackPrompt` (system.go), and `BuildPlannerSystemPrompt` (planner.go) are the exact seams; their current structure (preamble + scaffold + multi-line rule + subject-target) is preserved, with the multi-line rule becoming conditional on `!forceBody`.
- Prior architecture research (`plan/020_6979db625159/architecture/`) covers the broader config/prompt subsystems but not this specific feature; no duplication — the surface is small enough to specify fully above.