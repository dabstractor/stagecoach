# Research — P2.M3.T1.S1: Correct the agy quick-reference table row

Captured at HEAD on 2026-07-09. All four sources of truth cross-checked and
**mutually consistent**. This subtask edits exactly ONE line (`docs/providers.md:85`)
and exactly TWO cells on it (audit items D1 + D2). Everything else is read-only.

## 1. The edit target — current vs desired (line 85)

**CURRENT (drifted) — `docs/providers.md:85`:**
```
| `agy` | stdin | `-p` | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--approval-mode default`) | — no |
```

**DESIRED (corrected):**
```
| `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |
```

Table header (`docs/providers.md:79-80`) defines 8 columns in this order:
`Provider | Delivery | Print flag | Model flag | Default model | System prompt flag | Tool-disable approach | Stager?`

So the two drifted cells are:
- **Column 3 — Print flag**: `` `-p` `` → `(none)`  (audit item **D1**)
- **Column 7 — Tool-disable approach**: `Read-only constraint (`--approval-mode default`)` → `Read-only constraint (`--mode plan`)`  (audit item **D2**)

The other six agy cells (delivery `stdin`, model flag `--model`, default model
`Gemini 3.5 Flash (Low)`, system prompt `(prepended)`, stager `— no`, and the
provider name `` `agy` ``) are ALREADY CORRECT and must NOT be touched.

## 2. Source-of-truth cross-check (all four agree)

### 2a. Compiled manifest — `internal/provider/builtin.go` `builtinAgy()` (lines ~199–217)
```go
PromptDelivery: strPtr("stdin"),
PrintFlag:      strPtr(""),                // NON-NIL empty — bare -p is value-taking and breaks delivery
ModelFlag:      strPtr("--model"),         // `-m` is REJECTED by agy
DefaultModel:   strPtr("Gemini 3.5 Flash (Low)"),
SystemPromptFlag: strPtr(""),              // NON-NIL empty — sys prepended to payload (§12.2)
ProviderFlag:   strPtr(""),
BareFlags: []string{
    "--mode", "plan",                      // read-only, never-ask. NO --approval-mode.
},
```
→ `PrintFlag=""` ⇒ Print flag cell = **(none)**. `BareFlags=["--mode","plan"]` ⇒ Tool-disable cell =
**Read-only constraint (`--mode plan`)**. ✅ matches desired.

### 2b. Reference manifest — `providers/agy.toml`
```toml
print_flag = ""              # NON-NIL empty: agy's -p is VALUE-TAKING in v1.1.0 — bare -p fails
model_flag = "--model"       # `-m` is REJECTED
bare_flags = [               # appended VERBATIM to enforce read-only, never-ask
  "--mode", "plan",          # NO --approval-mode; plan = read-only (verified 2026-07-08)
]
```
→ identical to 2a. ✅

### 2c. PRD §12.5.1 (h3.58, PRD.md:951–972) — the spec
```toml
print_flag = ""              # NON-NIL empty (agy v1.1.0 -p is value-taking)
model_flag = "--model"       # -m rejected
bare_flags = ["--mode", "plan"]
```
→ identical. ✅ The PRD is the human-owned spec; the code mirrors it; the docs must mirror both.

### 2d. Audit — `plan/013_b8a415cc6e79/architecture/docs_drift_audit.md` §1a
Explicitly tabulates D1 (print flag `-p`→`(none)`) and D2 (tool-disable
`--approval-mode default`→`--mode plan`) with the exact current and proposed line 85
literal (quoted verbatim in §1 above). ✅

## 3. Sibling rows / convention precedent

The **opencode** and **codex** rows already use `(none)` for their Print flag cell
(stdin / no-print-flag providers). agy joining them is consistent — it is the SAME
`(none)` token, not a new invention:
```
| `opencode` | positional | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | — no |
| `codex`    | stdin      | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`--sandbox read-only --ephemeral`) | — no |
```
(`docs/providers.md:82-83`.) The qwen-code row (`docs/providers.md:86`) stays at `-p` /
`--approval-mode default` — qwen-code is a Gemini-CLI fork and DID NOT diverge; it is
OUT OF SCOPE. Do not touch it.

## 4. Scope boundary — what is NOT this subtask

| Audit item | Fix | Owner subtask | Status |
|------------|-----|---------------|--------|
| D1 | agy print flag `-p`→`(none)` | **P2.M3.T1.S1 (THIS)** | here |
| D2 | agy tool-disable `--approval-mode default`→`--mode plan` | **P2.M3.T1.S1 (THIS)** | here |
| D3 | #76 date `2026-07-03`→`2026-07-08` (`docs/providers.md:88`) | P2.M3.T1.S2 | separate |
| D4–D7 | provider count `8`→`7` (`docs/providers.md:3,7,74,92`) | P2.M3.T1.S2 | separate |
| D8–D9 | `docs/README.md` count `8`→`7` + field `21`→`22` | P2.M3.T2 | separate |

`PRD.md`, `tasks.json`, `prd_snapshot.md`, `.gitignore`, and all source files are
READ-ONLY / out of scope. The ONLY file this subtask edits is `docs/providers.md`,
and the ONLY line is **85**.

## 5. Validation tooling discovered

- `.markdownlint.json` exists: `{ default: true, MD013: false, MD033: false, MD060: false }`.
  Relevant rule for a table-cell edit: **MD056 (table-column-count)** — every row must have the
  same number of columns. The edit preserves the column count (8 → 8), so MD056 stays green.
- markdownlint is **NOT** run in `.github/workflows/ci.yml` (CI only runs `go test` + golangci-lint).
  So the deterministic validation gate for THIS edit is the **pinpoint + negative grep** below,
  with an OPTIONAL markdownlint MD056 column-count check (run locally via `npx` if available).
- No Go build/test is affected (docs-only change) — but `go build ./...` and `go test ./...`
  remain green-by-invariant (no `.go` touched).

## 6. Deterministic verification commands (the validation gate)

```bash
cd /home/dustin/projects/stagecoach
# (a) Pinpoint the corrected line 85 — must equal the desired literal EXACTLY:
sed -n '85p' docs/providers.md
# EXPECT:
# | `agy` | stdin | (none) | `--model` | `Gemini 3.5 Flash (Low)` | (prepended) | Read-only constraint (`--mode plan`) | — no |

# (b) Negative — the OLD drift tokens must be GONE from the agy row (line 85) only:
sed -n '85p' docs/providers.md | grep -F -- '`-p`'              && echo "FAIL: stale -p"      || echo "ok: no -p"
sed -n '85p' docs/providers.md | grep -F -- '--approval-mode'  && echo "FAIL: stale approval-mode" || echo "ok: no approval-mode"

# (c) The NEW tokens must be PRESENT on line 85:
sed -n '85p' docs/providers.md | grep -F -- '(none)'           && echo "ok: (none) present"   || echo "FAIL: missing (none)"
sed -n '85p' docs/providers.md | grep -F -- '--mode plan'      && echo "ok: --mode plan present" || echo "FAIL: missing --mode plan"

# (d) The SIX untouched cells are unchanged (delivery/model/default/sys/stager still correct):
sed -n '85p' docs/providers.md | grep -F -- '| stdin |'        | grep -F -- '`--model`' | grep -F -- 'Gemini 3.5 Flash (Low)' | grep -F -- '(prepended)' | grep -F -- '— no' >/dev/null && echo "ok: 6 cells intact" || echo "FAIL: collateral change"

# (e) Column count preserved (MD056) — count pipes; agy row == header row:
hdr=$(sed -n '79p' docs/providers.md | tr -cd '|' | wc -c); row=$(sed -n '85p' docs/providers.md | tr -cd '|' | wc -c); [ "$hdr" = "$row" ] && echo "ok: cols match ($row pipes)" || echo "FAIL: col mismatch hdr=$hdr row=$row"

# (f) Sibling rows NOT regressed (qwen-code keeps -p + --approval-mode default; opencode/codex keep (none)):
sed -n '86p' docs/providers.md | grep -F -- '`-p`' >/dev/null && grep -F -- '--approval-mode default' <(sed -n '86p' docs/providers.md) >/dev/null && echo "ok: qwen-code untouched" || echo "FAIL: qwen-code regressed"
sed -n '82p;83p' docs/providers.md | grep -c '(none)'        # EXPECT: 2 (opencode + codex unchanged)

# (g) OPTIONAL markdownlint MD056 (column-count) if the tool is available locally:
npx --yes markdownlint-cli2 'docs/providers.md' 2>/dev/null | grep -i MD056 && echo "note: MD056 issue" || echo "ok: no MD056 issues (or tool unavailable)"

# (h) Docs-only invariant — no Go source changed, build/test stay green:
git diff --name-only | grep -E '\.go$' && echo "FAIL: Go file touched" || echo "ok: no .go changed"
go build ./... 2>&1 | tail -3   # EXPECT: clean (no error output)
```
