# Critical Findings — Documentation Drift Table

## All drift items to fix (9 total)

| # | File | Line(s) | Drift | Correct Value | Source of Truth |
|---|------|---------|-------|---------------|-----------------|
| D1 | `docs/providers.md` | 85 | agy Print flag cell = `` `-p` `` | ``(none)`` | `builtin.go:205` PrintFlag=strPtr("") |
| D2 | `docs/providers.md` | 85 | agy Tool-disable cell = `` `--approval-mode default` `` | `` `--mode plan` `` | `builtin.go:210-212` BareFlags=["--mode","plan"] |
| D3 | `docs/providers.md` | 88 | #76 verified `as of 2026-07-03` | `as of 2026-07-08` | PRD §12.5.1 verified 2026-07-08 (agy v1.1.0) |
| D4 | `docs/providers.md` | 3 | `the 8 built-in providers` | `the 7 built-in providers` | 7 built-ins in `builtin.go` |
| D5 | `docs/providers.md` | 7 | `Eight providers are compiled in` | `Seven providers are compiled in` | same |
| D6 | `docs/providers.md` | 74 | `## The 8 built-in providers` | `## The 7 built-in providers` | same |
| D7 | `docs/providers.md` | 92 | `The eight built-in providers achieve` | `The seven built-in providers achieve` | same |
| D8 | `docs/README.md` | 35 | `the 8 built-in providers` | `the 7 built-in providers` | 7 built-ins |
| D9 | `docs/README.md` | 35 | `21-field manifest schema` | `22-field manifest schema` | `manifest.go` has 22 toml tags |

## Priority

- **D1, D2** — HIGH: document an invocation that no longer works on agy v1.1.0 (`-p` fails,
  `--approval-mode` doesn't exist). These are user-facing correctness errors.
- **D3** — MEDIUM: factual date staleness.
- **D4–D8** — MEDIUM: mechanical count corrections. Internally contradictory (the table at
  lines 80-85 lists exactly 7 providers, so "8" in the heading contradicts the table).
- **D9** — LOW: adjacent drift on the same `docs/README.md` line being edited for D8.

## Files confirmed CLEAN (no changes needed)

- `README.md` — correctly states "Seven built-ins", explicitly documents gemini removal
- `docs/cli.md` — no stale references
- `docs/configuration.md` — no stale references
- `docs/how-it-works.md` — no stale references

## Smoke test for final verification

```bash
make build && ./bin/stagecoach providers list   # must show 7 built-ins, no gemini
./bin/stagecoach providers show agy              # must show print="", model_flag="--model", --mode plan
git grep -in 'gemini' -- docs/ README.md providers/  # only model names or lineage comments
```
