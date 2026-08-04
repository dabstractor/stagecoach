# Research Notes — P1.M1.T1.S1 (opencode Delivery cell: positional → stdin)

Verification against the CURRENT working tree (2026-07-13). The task description and the
`architecture/system_context.md §Defect 1` cross-check are accurate. These notes record the exact
verified state for a one-pass one-cell edit.

## VERIFIED — the defect (docs/providers.md:82)

The "## The 7 built-in providers" table is at docs/providers.md lines 77-86. The opencode row is
**line 82**:
```
| `opencode` | positional | (none) | `-m` | (user must set) | (prepended) | Read-only constraint (`run` subcommand) | no per-surface switch; read-only by design — documented limitation | — no |
```
The Delivery column is the **2nd column** (Provider, **Delivery**, Print flag, Model flag, …). Its
cell reads the literal word `positional`. It must become the literal word `stdin`.

## VERIFIED — the authoritative value is `stdin` (4 independent sources)
1. **Source** — `internal/provider/builtin.go` builtinOpenCode(): its doc comment records
   `REVISION (delivery): PromptDelivery="stdin" (§12.6 said "positional")` and the struct field is
   `PromptDelivery: strPtr("stdin")`. (Verified 2026-07-08, opencode 1.1.23.)
2. **Reference TOML** — `providers/opencode.toml:45` `prompt_delivery = "stdin"  # REVISED from §12.6 "positional"`.
3. **Test TOML** — `internal/provider/builtin_test.go` decode-parity block carries the opencode
   delivery as `stdin`.
4. **Binary** — `stagecoach providers show opencode | grep prompt_delivery` prints
   `prompt_delivery = 'stdin'` (per the task's reproduced check).

## VERIFIED — all 7 Delivery values (post-fix target)

| Line | Provider | Current docs Delivery | Binary prompt_delivery | Status |
|------|----------|-----------------------|------------------------|--------|
| 80 | pi | stdin | stdin | ✓ |
| 81 | claude | stdin | stdin | ✓ |
| 82 | opencode | **positional** | **stdin** | ✗ THE FIX |
| 83 | codex | stdin | stdin | ✓ |
| 84 | cursor | positional | positional | ✓ (cursor IS legitimately positional — builtin.go:418) |
| 85 | agy | stdin | stdin | ✓ |
| 86 | qwen-code | stdin | stdin | ✓ |

NOTE: cursor (line 84) is the ONLY provider that is genuinely `positional` (builtin.go:418
`PromptDelivery: strPtr("positional")`). Do NOT "fix" cursor — its docs value is correct. Only opencode
(line 82) drifts.

## VERIFIED — no cascade edits needed
- grep `opencode...positional` across docs/ + README.md returns ONLY docs/providers.md:82. No other
  doc file references opencode's delivery as positional.
- docs/providers.md's own opencode "Rendered command" guidance and providers/opencode.toml's
  RENDERED COMMAND header already correctly show stdin piping (`< "<sys>\n\n<user payload>"`). So only
  the table cell is stale.

## ROOT CAUSE (for the implementer's understanding, no action needed)
git-blame shows commit `b8f081d` ("Add Chrome-disable column to providers.md table", 2026-07-13) rewrote
line 82 to add the Chrome-disable column and carried the stale `positional` delivery forward. The
underlying opencode→stdin revision landed earlier (commit `010ecee`); the chrome-disable column task
re-touched the row without refreshing the delivery cell.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T2.S1** (Issue 2): expand the docs/how-it-works.md "### Safety invariant" parenthetical to
  cover all 5 read-only-constrained providers (currently names only "codex, cursor"). DIFFERENT file,
  DIFFERENT issue. Do NOT touch how-it-works.md in this subtask.
- **P1.M1.T3.S1**: verify all 7 provider rows match binary values + cross-reference how-it-works prose
  consistency. That's a verification subtask; this subtask (S1) is the single-cell fix it will verify.
- DO NOT alter: any other cell in the opencode row, any other table row, the table header, any prose in
  providers.md, or any other file. The diff is exactly one word on one line.

## VALIDATION APPROACH
There is no code/test/build change — this is a markdown documentation edit. Validation is:
- a grep confirming line 82 now reads `stdin` in the Delivery cell;
- a grep confirming cursor (line 84) is UNCHANGED (still `positional` — the correct value);
- a markdown-lint / table-consistency check (the project ships `.markdownlint.json`);
- optionally re-running the binary cross-check (`stagecoach providers show opencode`).
`go build`/`go test` are unaffected but can be run as a no-regression sanity check.
