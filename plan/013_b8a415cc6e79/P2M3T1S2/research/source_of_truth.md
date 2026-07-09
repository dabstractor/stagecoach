# P2.M3.T1.S2 — Source of Truth Cross-Check

**Task**: Fix the #76 verification date (D3) and the provider count "8"→"7" (D4–D7) in `docs/providers.md`.
**Scope**: `docs/providers.md` ONLY. Lines 3, 7, 74, 88, 92. Five edits (one optional reframe).
**Out of scope**: line 85 (agy table row = sibling P2.M3.T1.S1); `docs/README.md` (D8/D9 = sibling P2.M3.T2).

---

## 1. The "7 built-ins" claim — verified against 3 code sources

| Source | Evidence | Count |
|--------|----------|-------|
| `internal/provider/builtin.go:17` | comment: `// qwen-code (experimental). Seven providers: pi, claude, opencode, codex, cursor, agy, qwen-code.` | **7** |
| `internal/provider/builtin.go` | `func builtin*()`: builtinPi, builtinClaude, builtinAgy, builtinQwenCode, builtinOpenCode, builtinCodex, builtinCursor | **7 funcs** |
| `internal/provider/registry.go:16` | `preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}` | **7** |
| (negative) `grep -c builtinGemini internal/provider/builtin.go` | **0** — gemini built-in removed (superseded by agy 2026-06-18) | 0 |

**Conclusion**: The built-in count is **7**, unambiguously. `docs/providers.md` prose says "8"/"eight" in 4 places (D4–D7), contradicting the 7-row table at lines 80–85. All 4 → "7"/"seven".

## 2. The "2026-07-08" verification date — verified against PRD §12.5.1.1

PRD §12.5.1.1 (h4.0) heading: *"#### 12.5.1.1 Status (agy) — verified **2026-07-08** against agy v1.1.0"*.
- Item 1: #76 non-TTY stdout drop — *"**no longer reproduces on v1.1.0**"*.
- Items 1–3 = **RESOLVED**.
- *"Items 1–3 are cleared; agy ships `experimental = true` (§12.7.2) solely pending item 4."*

`docs/providers.md:88` currently says *"no longer reproduces as of **2026-07-03**"* — predates the v1.1.0 re-verification. Fix → **2026-07-08** (audit item D3).

## 3. Optional accuracy reframe (line 88 prose)

Current line 88 phrasing is *slightly* inaccurate beyond just the date: it says agy is experimental
"pending the remaining §12.5.1.1 checklist items (the non-TTY stdout drop, issue #76, no longer
reproduces…)" — implying #76 is still a *pending* blocker. Per PRD §12.5.1.1, items 1–3 (incl. #76)
are **RESOLVED**; agy is experimental **solely** pending **item 4** (the tooled/stager flag combo).

The contract marks this reframe **OPTIONAL**. Two acceptable outcomes for line 88:
- **Minimal (REQUIRED)**: date-only swap — `as of **2026-07-03**` → `as of **2026-07-08**`.
- **Reframe (OPTIONAL, recommended)**: also align the "pending" clause with PRD §12.5.1.1 so #76 is
  shown as resolved and the only open item is item 4. Proposed literal in PRP Task 5.

## 4. Verbatim current → desired literals (the 5 edits)

| # | Line | Current token | Desired token |
|---|------|---------------|---------------|
| D4 | 3 | `the 8 built-in providers` | `the 7 built-in providers` |
| D5 | 7 | `Eight providers are compiled in as built-ins` | `Seven providers are compiled in as built-ins` |
| D6 | 74 | `## The 8 built-in providers` | `## The 7 built-in providers` |
| D3 | 88 | `no longer reproduces as of **2026-07-03**` | `no longer reproduces as of **2026-07-08**` |
| D7 | 92 | `The eight built-in providers achieve tool-safety` | `The seven built-in providers achieve tool-safety` |

## 5. Non-overlap with parallel sibling P2.M3.T1.S1

S1 edits **line 85** (agy table row: D1 print flag, D2 tool-disable cell).
S2 edits **lines 3, 7, 74, 88, 92** (D3–D7).
**No line overlap.** Both edit `docs/providers.md` in place (no add/remove of lines), so line numbers
stay stable across the two parallel edits. S2's gate must assert line 85 is NOT modified by S2 (leave
it to S1), and that `docs/README.md` is NOT touched (leave it to P2.M3.T2).

## 6. Verification approach

No markdownlint in CI (`.github/workflows/ci.yml` runs only `go test` + golangci-lint). Authority gate
= deterministic grep: after edits, `grep -nE '8 built|eight built|Eight providers|2026-07-03' docs/providers.md`
must return **zero** matches; `grep -nE '7 built|Seven providers|seven built|2026-07-08'` must hit
exactly the 5 expected lines. Docs-only → `go build ./...` / `go test ./...` stay green by construction.
