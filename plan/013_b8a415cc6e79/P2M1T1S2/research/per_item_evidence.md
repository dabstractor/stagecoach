# P2.M1.T1.S2 — Per-item PRD evidence (collected at HEAD = fd99358)

Read-only verification of `PRD.md` for gemini removal cleanliness. The PRD was purged in
commit `cdbccf5` ("Purge gemini-cli from PRD after built-in removal"); this confirms that
committed state. Every item below was checked by direct grep/read against `PRD.md`.

## Result: all 8 items PASS.

## Item-by-item evidence (PRD.md line numbers at HEAD)

| Item | Section | PRD.md line | Observed | Verdict |
|------|---------|-------------|----------|---------|
| (a) | §12.5 (h3.57) REMOVED stub | 943 | `### 12.5 ~~Built-in provider: Gemini CLI~~ — REMOVED (superseded by agy, §12.5.1)` — exact match | PASS |
| (b) | §12 terminology list / fixed-backend examples | 692–696 | provider examples row = `pi, opencode, claude, codex, cursor, agy, qwen-code`; fixed-backend = `(claude, codex, cursor, agy, qwen-code)` — no gemini (only `gemini-3.1-pro` as a model example, which is legitimate) | PASS |
| (c) | §6.1 G2 (h3.10) | 174 | `**pi, Claude Code, agy, opencode**` — lists agy, not gemini | PASS |
| (d) | §9.16 FR-D1 (h3.32) cascade | 407 | `**pi, opencode, cursor, agy, qwen-code, codex, claude.**` — no gemini | PASS |
| (e) | §9.16 FR-D4 default-model table | 415–422 | rows: pi, opencode, cursor, agy, qwen-code, codex, claude — no gemini row | PASS |
| (f) | §14 package layout | 1390–1396 | providers/ tree: pi, claude, agy, opencode, codex, cursor .toml — no gemini.toml (only reference to `providers/gemini.toml` is the REMOVED stub prose at :945 stating it was deleted) | PASS |
| (g) | Appendix D quick reference | 2277–2285 | rows: pi, claude, agy, opencode, codex, cursor — no gemini row | PASS |
| (h) | Appendix B.5 rescue example | 2245 | `↳ Generating with Gemini 3.5 Flash (Low) in agy…` — uses agy (provider); "Gemini 3.5 Flash" is the model name, legitimate | PASS |

## Negative-space sweep

- `grep -nE '^\| (gemini|\*\*gemini\*\*)' PRD.md` → NO table row names gemini as a provider.
- `grep -n 'gemini.toml' PRD.md` → exactly ONE hit at :945 (the REMOVED stub prose that says it
  was deleted). None in the §14 tree or anywhere as a shipped file.

## Legitimate "gemini" model-name / lineage hits (NOT drift — per contract NOTE)

The word `gemini` legitimately persists because **agy runs the Gemini model family** and **qwen-code
is a Gemini-CLI fork**. These are model names / lineage prose, not provider enumerations:
- :401 FR-R5 — `gemini-3.1-pro` as a model-string example.
- :417 FR-D4 agy row — `gemini-3.1-pro`, `gemini-3.5-flash`, `gemini-3.1-flash-lite` (agy's models).
- :694 §12 terminology — `gemini-3.1-pro` as a model example.
- :696 §12 fixed-backend — `gemini-3.1-pro` as a bare-model example.
- :949/:986/:987 §12.5.1 agy — "superseded `gemini`", "gemini-cli lineage's `-m`", "diverged from gemini-cli".
- :995/:1003/:1005/:1007/:1009 §12.5.2 qwen-code — "manifest mirrors `gemini` (§12.5)", gemini-lineage flag prose.
- :2245 Appendix B.5 — "Gemini 3.5 Flash" model name.
- :2300 Appendix E.11 — `gemini-3.1-pro` as a model-name verification example.

## Out-of-scope observations (NOT gemini, NOT among items a–h — do NOT act here)

1. **Appendix F decision-log FR-D1 echo (line 2323)** reads "pi → opencode → cursor → agy → codex →
   claude" — it OMITS `qwen-code`, disagreeing with the authoritative FR-D1 at :407. This is a
   pre-existing inconsistency unrelated to gemini and outside items (a)–(h). Reported only.
2. **§14 package-layout tree (lines 1390–1396)** lists only 6 .toml files — it OMITS
   `qwen-code.toml` (the §14 tree predates qwen-code). Again unrelated to gemini and outside items
   (a)–(h); P2.M1.T1.S1's code-side check (item c) is the authoritative providers/ file-set proof.
   Reported only.

## DOCS surface (Mode A)

Per contract §5, the PRD edits themselves ARE this subtask's documentation surface — confirming the
spec-of-record for the removal is the entire task. No source changes; PRD.md is READ-ONLY (human-owned).
