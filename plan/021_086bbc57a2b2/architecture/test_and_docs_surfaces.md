# Architecture: Test Patterns + Docs Surfaces

## Test patterns (verified)

### `internal/config/load_test.go` — `TestValidateFormat` (L1355)

Table-driven with subtests via `t.Run`. Two groups:
- **valid modes**: `[]string{"auto", "conventional", "gitmoji", "plain"}` — asserts `err == nil`
- **invalid modes**: `[]string{"", "emoji", "Conventional", "AUTO", "gitmojii", " auto"}` — asserts:
  1. `err != nil`
  2. `strings.Contains(err.Error(), mode)` — error names the offending value
  3. `strings.Contains(err.Error(), "auto, conventional, gitmoji, plain")` — error names the valid set

**Pattern to follow:** Add `+body` valid cases to the valid slice; add invalid
`+body` cases to the invalid slice. Update the valid-set assertion substring to
match the new grammar error message.

### `internal/prompt/format_test.go`

- `TestFormatScaffoldBody` (L11): table-driven, exact `==` comparison per case.
  Cases: `auto→""`, `plain→""`, `conventional→conventionalScaffold`,
  `gitmoji→instruction+table`, `bogus-unknown-mode→""`, `""→""`.

- `TestBuildFormatSystemPrompt` (L91): subtests via `t.Run`. Mix of exact `==`
  comparison (canonical topology) and `strings.Contains` checks (rule selection,
  subjectTarget interpolation).

**Pattern to follow:** Add a `TestSplitFormat` table test (new). Add
`buildFormatSystemPrompt("<base>+body", …)` subtests asserting `bodyForceDirective`
present and neither `multilineRuleAllow` nor `multilineRuleSingle` present.

### `internal/prompt/system_test.go`

- `TestBuildSystemPrompt_CanonicalExact` (L13): exact string comparison against
  hand-assembled constants for `auto` format (byte-identity baseline).
- `TestBuildSystemPrompt_FormatModes_CanonicalExact` (L331): table-driven, exact
  `==` comparison for `conventional`/`gitmoji`/`plain` through `BuildSystemPrompt`.
- `TestBuildSystemPrompt_FormatModes_Properties` (L383): structural invariant
  checks via `strings.Contains` / absence checks.
- `TestBuildFallbackPrompt_CanonicalExact` (L244): exact comparison for `auto`.

**Pattern to follow:** Add `auto+body` case to `BuildSystemPrompt` tests —
examples block present AND `bodyForceDirective` present AND `multilineRule*` absent.
Add `conventional+body` end-to-end case through `BuildSystemPrompt`.

**Important:** The existing `auto` byte-identity tests (`CanonicalExact`) must
remain GREEN unchanged — this is the FR-F1 guarantee.

## Docs surfaces (verified current text + line numbers)

### `docs/cli.md`

| Line | Current text |
| --- | --- |
| 38 | `\| --format <mode> \| string \| auto \| STAGECOACH_FORMAT \| stagecoach.format \| Message format: auto (style learning) \| conventional \| gitmoji \| plain. An unknown mode is a hard error (exit 1). Also [generation].format. \|` |

### `docs/configuration.md`

| Line | Current text |
| --- | --- |
| 116 | `# format = "auto" # auto\|conventional\|gitmoji\|plain; unknown = hard error (exit 1)` |
| 216 | `\| STAGECOACH_FORMAT \| --format \| Message format (auto\|conventional\|gitmoji\|plain; unknown = hard error) \| STAGECOACH_FORMAT=conventional stagecoach \|` |
| 247 | `\| stagecoach.format \| string \| git config --get stagecoach.format \| Message format: auto \| conventional \| gitmoji \| plain. Unknown = hard error (exit 1). \|` |

### `README.md`

| Line | Current text |
| --- | --- |
| 63 | `\| Message shaping \| --format (auto, conventional, gitmoji, plain), --locale, --context, --template (docs). \|` |
| 196 | `stagecoach --format conventional # force conventional-commit style` |

### Other doc surfaces to check (Mode B sweep)

- `docs/how-it-works.md` — referenced from README:63 as the format-modes docs link. Check for format lists.
- Any other `docs/*.md` with `auto|conventional|gitmoji|plain` pattern.

## Acceptance grep

```bash
# Should return no row that lists the 4 bases without mentioning +body:
rg -n "auto\|conventional\|gitmoji\|plain" docs/cli.md docs/configuration.md
```

## No external dependencies

This delta introduces no new external dependencies. The feature is pure Go string
manipulation (parsing + concatenation) within the existing `prompt` and `config`
packages. No new imports needed beyond what exists.