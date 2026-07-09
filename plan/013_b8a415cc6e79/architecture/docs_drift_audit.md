# Documentation Drift Audit — Provider Lineup Correction (gemini → agy)

Read-only audit. **No files were modified.** Source of truth used to flag drift:
- `internal/provider/builtin.go` — `BuiltinManifests()` contains exactly **7** entries (pi, claude, opencode, codex, cursor, agy, qwen-code). Its own comment states: *"Seven providers: pi, claude, opencode, codex, cursor, agy, qwen-code."* No `builtinGemini` exists.
- `internal/provider/registry.go:11` — `preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "qwen-code", "codex", "claude"}` (7 entries).
- `internal/provider/manifest.go:37` — `Manifest` struct has **22** `toml:` tags.
- `providers/agy.toml` + `builtin.go` `builtinAgy()` — the compiled agy manifest: `PrintFlag = ""` (empty, no flag) and `BareFlags = ["--mode", "plan"]`. Issue #76 "NO LONGER REPRODUCES" verified **2026-07-08** (agy v1.1.0).

The lineup correction removed `gemini` (superseded by `agy` on 2026-06-18), dropping the built-in count from 8 → 7. Residual drift is the stale **"8"** count, a stale **agy table row**, and a stale **#76 verification date**.

---

## 1. `docs/providers.md`

### 1a. Quick-reference table — `agy` row (line 85) — TWO drifted cells

**Current (line 85):**
```
| `agy` | stdin | `-p` | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--approval-mode default`) | — no |
```

| Cell | Current (drifted) | Correct (per `builtinAgy()` / `providers/agy.toml`) | Reason |
|------|------------------|-----------------------------------------------------|--------|
| **Print flag** | `-p` | `(none)` | agy v1.1.0's `-p`/`--print`/`--prompt` is **value-taking**; a bare `-p` fails ("flag needs an argument: -p"). `PrintFlag = ""` — agy reads stdin with no flag. (Compare opencode/codex rows which correctly say `(none)`.) |
| **Tool-disable approach** (the "bare essentials" cell) | `Read-only constraint (--approval-mode default)` | `Read-only constraint (--mode plan)` | `--approval-mode` was **removed** in agy v1.1.0. `BareFlags = ["--mode", "plan"]` is the read-only equivalent. |

**Proposed line 85:**
```
| `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |
```

### 1b. #76 non-TTY stdout fix — verification date (line 88) — STALE

**Current (line 88):**
> `agy` is **experimental** (PRD §12.5.1) pending the remaining §12.5.1.1 checklist items (the non-TTY stdout drop, issue #76, no longer reproduces as of **2026-07-03**) and cannot serve as a stager…

**Should be:** `…no longer reproduces as of **2026-07-08**`. The manifest + `builtinAgy()` re-verified #76 on **2026-07-08** against agy v1.1.0 (live stdin runs return stdout correctly). The `2026-07-03` date predates the v1.1.0 re-verification.

### 1c. Provider count — 4 occurrences (lines 3, 7, 74, 92) — STALE "8" → should be "7"

| Line | Current text | Correct text |
|------|--------------|--------------|
| 3 | `…the 8 built-in providers, the tools-disable asymmetry…` | `…the 7 built-in providers, the tools-disable asymmetry…` |
| 7 | `Eight providers are compiled in as built-ins (zero config needed).` | `Seven providers are compiled in as built-ins (zero config needed).` |
| 74 | `## The 8 built-in providers` | `## The 7 built-in providers` |
| 92 | `The eight built-in providers achieve tool-safety via two distinct mechanisms…` | `The seven built-in providers achieve tool-safety via two distinct mechanisms…` |

The table at lines 80–85 lists exactly 7 providers, so the heading/prose "8" contradicts the table itself.

### 1d. Remaining `gemini`-as-built-in references — **NONE (clean)**

The three `gemini`/`Gemini` matches in `providers.md` are all legitimate, not drift:
- Line 85 / line 131: `Gemini 3.5 Flash (Low/High/Medium)` — these are **model display labels** (from `agy models`), not a provider name.
- Line 88: "a **Gemini-CLI fork** for Qwen3-Coder" — describes qwen-code's lineage. Legitimate.

No `gemini` provider name appears as a built-in anywhere in the file.

---

## 2. `docs/cli.md`, `docs/configuration.md`, `docs/how-it-works.md` — **CLEAN (no gemini-as-built-in drift)**

Targeted grep for `gemini`/`Gemini`, `8 built`/`eight built`, and `21-field`/`22-field` returned **no matches** in any of these three files. All three correctly enumerate the 7-provider auto-detection order (`pi, opencode, cursor, agy, qwen-code, codex, claude`). No action required here.

### 2d. `docs/README.md` — TWO drifted items on line 35 (no gemini, but related lineup drift)

`docs/README.md` has no `gemini` reference. However line 35 (the "Provider manifests" index row) carries two count/field drifts from the same correction:

**Current (line 35):**
```
| [Provider manifests](providers.md) | 21-field manifest schema, command rendering, the 8 built-in providers (incl. agy and qwen-code), and adding a new agent. |
```

| Drift | Current | Correct | Reason |
|-------|---------|---------|--------|
| Provider count | `the 8 built-in providers` | `the 7 built-in providers` | Lineup correction dropped gemini (8 → 7). |
| Field count (adjacent) | `21-field manifest schema` | `22-field manifest schema` | `manifest.go` has 22 `toml:` tags; `providers.md` (the referenced page) says "22-field schema" / "22 fields". `docs/README.md` is the lone outlier at 21. |

**Proposed line 35:**
```
| [Provider manifests](providers.md) | 22-field manifest schema, command rendering, the 7 built-in providers (incl. agy and qwen-code), and adding a new agent. |
```

---

## 3. `README.md` (top-level) — **CLEAN (no stale gemini reference)**

The sole `gemini` mention (line 355, FAQ "Which agents are supported?") is **correct and intentional**:
> Seven built-ins are auto-detected: **pi**, **opencode**, **cursor**, **agy** *(experimental)*, **qwen-code** *(experimental)*, **codex**, **claude**. (Google's `gemini` / Gemini CLI is **no longer shipped** — it was superseded by **agy**, the Antigravity CLI, on 2026-06-18.)

It correctly states "Seven built-ins" and explicitly documents the gemini removal. All other provider lists in README.md enumerate 7 providers. **No drift to fix here.**

---

## Summary table of all drift to fix

| # | File | Line(s) | Drift | Fix |
|---|------|---------|-------|-----|
| D1 | `docs/providers.md` | 85 | agy Print flag cell = `-p` | → `(none)` |
| D2 | `docs/providers.md` | 85 | agy Tool-disable cell = `--approval-mode default` | → `--mode plan` |
| D3 | `docs/providers.md` | 88 | #76 verified `as of 2026-07-03` | → `as of 2026-07-08` |
| D4 | `docs/providers.md` | 3 | `the 8 built-in providers` | → `the 7 built-in providers` |
| D5 | `docs/providers.md` | 7 | `Eight providers are compiled in` | → `Seven providers are compiled in` |
| D6 | `docs/providers.md` | 74 | `## The 8 built-in providers` | → `## The 7 built-in providers` |
| D7 | `docs/providers.md` | 92 | `The eight built-in providers achieve tool-safety` | → `The seven built-in providers achieve tool-safety` |
| D8 | `docs/README.md` | 35 | `the 8 built-in providers` | → `the 7 built-in providers` |
| D9 | `docs/README.md` | 35 | `21-field manifest schema` (adjacent drift) | → `22-field manifest schema` |

**Files needing zero changes:** `docs/cli.md`, `docs/configuration.md`, `docs/how-it-works.md`, `README.md` (all clean — no stale gemini-as-built-in references; correct 7-provider enumeration).

**Notes / risks:**
- D1/D2 (agy table row) are the highest-signal drift — they document an invocation (`-p` + `--approval-mode default`) that `builtin.go`/`agy.toml` confirm **no longer works** on agy v1.1.0.
- D3 (date) is a factual staleness; safe to correct.
- D4–D8 are mechanical count corrections (8→7).
- D9 (21→22 field count) is adjacent drift surfaced during this audit; flag for the editor's discretion — it is not strictly the "provider lineup" correction but lives on the same `docs/README.md` line being edited for D8.
- Minor (not flagged for edit): `registry.go:55` allocates `len(userOverrides)+8` with comment "built-ins + overrides headroom" — this is a capacity *hint* with explicit "headroom" wording, not a count claim; leave as-is.
