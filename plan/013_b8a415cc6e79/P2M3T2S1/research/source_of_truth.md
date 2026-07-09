# P2.M3.T2.S1 — Source-of-truth cross-check (docs/README.md D8/D9)

Scope: ONE line edit in `docs/README.md` (line 35, the "Provider manifests" index row) — two
token swaps: provider count `8 → 7` (audit D8) and field count `21 → 22` (audit D9).

## 1. The single drifted line (verified verbatim)

```
$ sed -n '35p' docs/README.md
| [Provider manifests](providers.md) | 21-field manifest schema, command rendering, the 8 built-in providers (incl. agy and qwen-code), and adding a new agent. |
```

Corrected target:
```
| [Provider manifests](providers.md) | 22-field manifest schema, command rendering, the 7 built-in providers (incl. agy and qwen-code), and adding a new agent. |
```

Only TWO characters of substance change: `21`→`22` and `8`→`7`. Everything else on the line
(byte-for-byte) stays identical.

## 2. Line 35 is the ONLY drifted line in docs/README.md

```
$ grep -nE '2[012]-field|built-in provider' docs/README.md
35:| [Provider manifests](providers.md) | 21-field manifest schema, command rendering, the 8 built-in providers (incl. agy and qwen-code), and adding a new agent. |
```
→ exactly one match. No other "21-field", "8 built-in", etc. in the file. So the edit is
surgically one line.

NOTE: the TOP-LEVEL `README.md` (repo root) is a DIFFERENT file and is CLEAN — it already says
"Seven built-ins" (audit §3 confirms). Do NOT touch README.md; this task edits ONLY `docs/README.md`.

## 3. Source of truth #1 — provider count = 7

`internal/provider/builtin.go`:
- Line 17 comment: `// ... Seven providers: pi, claude, opencode, codex, cursor, agy, qwen-code.`
- `func BuiltinManifests() map[string]Manifest` returns EXACTLY 7 entries:
  pi, claude, opencode, codex, cursor, agy, qwen-code.
- `grep -c builtinGemini internal/provider/builtin.go` == 0 (gemini removed — superseded by agy).

→ docs/README.md "8 built-in providers" is stale (predates the gemini→agy succession). Correct = 7.

## 4. Source of truth #2 — field count = 22

`internal/provider/manifest.go`:
- `grep -c 'toml:' internal/provider/manifest.go` == **22**.
- The 22 toml tags: name, detect, command, list_models_command, subcommand, prompt_delivery,
  prompt_flag, print_flag, model_flag, default_model, system_prompt_flag, provider_flag,
  session_mode, bare_flags, tooled_flags, **experimental**, output, json_field, strip_code_fence,
  retry_instruction, env, reasoning_levels.

WHY docs/README.md says 21 today (the gotcha): the PRD §12.1 schema *EXAMPLE* block enumerates
21 fields and OMITS `experimental`. But the COMPILED `Manifest` struct in manifest.go adds a 22nd
toml tag — `experimental` (manifest.go:81 `Experimental *bool toml:"experimental"`). The contract
chooses the COMPILED struct (22) as the source of truth, which is also what `docs/providers.md`
(the page this index row points at) already states. So the correction is 21 → 22.

Corroboration — `docs/providers.md` line 3 already reads "the 22-field schema". docs/README.md is
the lone outlier at 21. The fix makes the index row agree with the page it indexes.

## 5. Audit references (the drift was named here)

`plan/013_b8a415cc6e79/architecture/docs_drift_audit.md`:
- §2d (line 64): "`docs/README.md` — TWO drifted items on line 35".
- Line 76 table row (D9): "`manifest.go` has 22 `toml:` tags; `providers.md` ... says '22-field
  schema' / '22 fields'. `docs/README.md` is the lone outlier at 21."
- Line 105 (D8): `docs/README.md` line 35 `the 8 built-in providers` → `the 7 built-in providers`.
- Line 106 (D9): `docs/README.md` line 35 `21-field manifest schema` → `22-field manifest schema`.
- Line 114: "D9 (21→22 field count) is adjacent drift ... lives on the same `docs/README.md` line
  being edited for D8." → confirms D8+D9 are intentionally batched into one one-line edit.

## 6. Parallel-work boundary (no collision)

- This task (P2.M3.T2.S1) edits **`docs/README.md` line 35** only.
- Parallel sibling P2.M3.T1.S1 / P2.M3.T1.S2 edit **`docs/providers.md`** (line 85 agy row; lines
  3/7/74/88/92) — a DIFFERENT FILE. → zero file-level collision. Both can land independently.
- P2.M3.T1.S2's PRP explicitly asserts "docs/README.md unchanged (D8/D9 = sibling P2.M3.T2)",
  confirming the disjoint ownership in both directions.

## 7. Deterministic verification command set

```bash
cd /home/dustin/projects/stagecoach
# (a) NEGATIVE — stale tokens gone (zero matches required):
grep -nE '21-field|the 8 built-in providers' docs/README.md && echo "FAIL" || echo "ok: no stale tokens"
# (b) POSITIVE — corrected line reads both target literals:
sed -n '35p' docs/README.md | grep -F '22-field manifest schema'        >/dev/null && echo "ok: field=22" || echo "FAIL: field"
sed -n '35p' docs/README.md | grep -F 'the 7 built-in providers'        >/dev/null && echo "ok: count=7" || echo "FAIL: count"
# (c) Line 35 is the ONLY changed line; only docs/README.md changed:
git diff --stat docs/README.md        # expect exactly 1 insertion + 1 deletion on line 35
git diff --name-only | grep -vE '^docs/README.md$' | grep . && echo "FAIL: other files touched" || echo "ok: only docs/README.md"
# (d) Docs-only invariant — no .go / PRD.md / tasks.json / prd_snapshot.md / .gitignore:
git diff --name-only | grep -E '\.go$|PRD\.md|tasks\.json|prd_snapshot\.md|\.gitignore' && echo "FAIL: forbidden" || echo "ok: no forbidden files"
# (e) Top-level README.md untouched (it is a different, already-clean file):
git diff --name-only | grep -E '^README\.md$' && echo "FAIL: README.md touched" || echo "ok: README.md untouched"
# (f) Parity: docs/README.md now agrees with providers.md (the page it indexes) and the compiled struct:
grep -F '22-field schema' docs/providers.md   # providers.md line 3 = the authority the index now mirrors
```

## 8. Confidence

One-pass success likelihood: **10/10**. One line, two digit swaps, fully specified by a verbatim
current→desired literal, cross-checked against the compiled manifest (7 built-ins, 22 toml tags)
and the indexed page (providers.md already says "22-field"). No code, no schema, no behavior.
The only failure mode is editing the wrong file (README.md vs docs/README.md) or straying beyond
line 35 — both caught by the grep/diff gate.
