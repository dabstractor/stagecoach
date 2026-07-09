# External Dependencies & Constraints

## 1. Git Config Key Naming Rules (Issue 2)

Git config variable names follow strict rules (git source `config.c`, documented in
`git help config` → "SYNTAX"):

- A config variable name is `<section>.<key>` where `<key>` is the "name" after the last dot.
- The **name** (final dotted segment) may contain ONLY `[a-zA-Z0-9-]` — **letters, digits, and hyphen**.
- **Underscores (`_`) are NOT allowed** in the name component. `git config stagecoach.auto_stage_all`
  fails with `error: invalid key: stagecoach.auto_stage_all` (exit 1).
- The section name (`stagecoach`) is case-insensitive and may contain alphanumerics and `-`.
- The key name within a section is case-insensitive but the **convention** in this codebase is camelCase
  (e.g. `autoStageAll`, `stripCodeFence`), which git preserves on read-back.

**Implication:** the documented snake_case key `stagecoach.auto_stage_all` is not merely ignored —
it is **un-settable**. The implementation's camelCase `stagecoach.autoStageAll` is the only valid form.
Docs must be reconciled to match code.

## 2. `strconv.ParseBool` Behavior (Issues 3, 4)

Go's `strconv.ParseBool` accepts: `1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False`.
**Any other value (including `"2"`) returns an error.** This is why `STAGECOACH_VERBOSE=2`
fails with an opaque parse error.

New env-var cases for booleans (`STAGECOACH_AUTO_STAGE_ALL`, `STAGECOACH_MULTI_TURN_FALLBACK`)
must use `strconv.ParseBool` + wrapped error + DIRECT pointer assignment (`boolPtr(b)`), mirroring
the existing `STAGECOACH_PUSH` (`load.go:301`) and `STAGECOACH_NO_VERIFY` (`load.go:310`) pattern.

## 3. Provider Delivery Modes (Issue 5)

Manifests declare `prompt_delivery` (`internal/provider/manifest.go:46`):
- `"stdin"` (default): payload → `spec.Stdin`; `spec.Args` clean. Providers: pi, claude, agy, qwen, opencode, codex.
- `"positional"`: payload appended as trailing `spec.Args` element; `spec.Stdin == ""`. Provider: cursor.
- `"flag"`: `*r.PromptFlag` + payload appended to `spec.Args`; `spec.Stdin == ""`. No builtin; user-defined only.

`CmdSpec` (`render.go:22-29`) intentionally does NOT carry the delivery mode — `"Stdin=\"\"
disambiguates"`. This is why the executor cannot self-correct the payload-size computation; the
fix must record the size at the `Render` call site where the delivery mode IS known.

## 4. No External Network/Service Dependencies

All six issues are purely internal (config layer, docs, diagnostics). No external API changes,
no schema migrations beyond the v3 config already in place. The `config upgrade` path is unaffected.
