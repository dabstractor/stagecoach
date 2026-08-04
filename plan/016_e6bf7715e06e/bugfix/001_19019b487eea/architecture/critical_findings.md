# Critical Findings — Bugfix 001: Documentation Drift Remediation

## Finding 1: Both defects are confirmed by direct binary verification

The shipped binary was rebuilt (`go build -o /tmp/stagecoach_verify ./cmd/stagecoach`)
and all 7 providers' `prompt_delivery` values were extracted via
`stagecoach providers show <name>`. opencode is the **only** provider whose
binary value (`stdin`) disagrees with the docs table (`positional`). The other
6 providers all match. This means Defect 1 is a single-cell fix.

## Finding 2: The fix is a one-word change in each file

- **Defect 1:** In `docs/providers.md` line 82, the opencode row's Delivery cell
  changes from `positional` to `stdin`. This is a literal string replacement
  within the markdown table row — no other columns or rows change.

- **Defect 2:** In `docs/how-it-works.md` line 197, the parenthetical
  `(codex, cursor)` expands to enumerate all 5 read-only-constrained providers.
  Two acceptable phrasings (PRD offers both):
  - **Exhaustive:** `read-only constraint flags (codex, cursor, agy, qwen-code; opencode's \`run\` is read-only by design)`
  - **Exemplary:** `read-only constraint flags (e.g. codex, cursor)`

  The exhaustive form is more precise and matches the universal "Every" claim
  in the same sentence. The exemplary form is smaller but slightly weakens the
  sentence's precision.

## Finding 3: The byte-faithfulness contract is real and enforceable

`docs/README.md` line 8 states: *"the `docs/` directory tracks the shipped
binary. If anything here disagrees with `stagecoach --help`, the binary is
authoritative."* Each `providers/*.toml` header states it mirrors
`builtinOpenCode()` etc. "BYTE-FOR-BYTE (modulo comments)." This contract makes
the opencode Delivery drift a genuine defect, not a cosmetic preference.

## Finding 4: No test changes needed — existing tests already pin the correct values

The existing test suite already asserts the **correct** `prompt_delivery` values
in `builtin_test.go` (decode-parity TOML: `prompt_delivery = "stdin"` for
opencode at line 87). All tests pass. The defects are in **markdown files** that
no Go test reads. After the doc fixes, no test file needs updating — the tests
already hold the binary side correct.

## Finding 5: Docs are the only artifacts changing — Mode A applies to each fix

Per SOW §5:
- **Mode A (doc-with-work):** Each subtask changes a specific user-facing doc
  cell/line. The fix IS the documentation update. No separate docs subtask
  needed for either fix.
- **Mode B (changeset-level):** A final task verifies cross-doc consistency
  after both fixes. Since both fixes are to docs files (not to code that has
  doc implications), the Mode B sweep is a verification step, not a writing step.

## Finding 6: The fix preserves the table's internal consistency

After fixing Defect 1, the docs/providers.md table will show:
- opencode Delivery = `stdin` (matching binary, builtin.go, providers/opencode.toml)
- The "Rendered command" guidance in the same file already shows stdin piping:
  `opencode run -m <model> < "<sys>\n\n<user payload>"`
- The `providers/opencode.toml` "RENDERED COMMAND" header already shows:
  `opencode run -m anthropic/claude-sonnet-4   < "<sys>\n\n<user payload>"`

So fixing the table cell aligns it with the file's own surrounding prose and
the TOML reference — no cascade of edits needed.
