# Architecture: Format Subsystem (`+body` modifier)

## Scope

This delta (v3.4) adds an optional `+body` suffix to every `--format` base that
forces a subject-plus-body message regardless of repo history shape. It is
**message-role prompt surface only** — no commit/CAS/rescue/lock/provider changes.

## Current state (verified against code as of `592e55f`)

### `internal/prompt/format.go`

The format scaffold + multi-line rule assembly. Key entities:

| Entity | Type | Line | Notes |
| --- | --- | --- | --- |
| `conventionalScaffold` | const string | ~L15 | Conventional Commits format contract; no trailing newline |
| `gitmojiScaffoldInstruction` | const string | ~L19 | Gitmoji instruction; no trailing newline |
| `formatScaffoldBody(format string) string` | func | ~L28 | Switches on `"conventional"`, `"gitmoji"`, default→`""` (auto/plain/unknown) |
| `withLocale(s, locale string) string` | func | ~L40 | Appends FR-F6 locale line; no-op when locale empty |
| `buildFormatSystemPrompt(format string, hasMultiline bool, subjectTarget int) string` | func | ~L55 | Writes `promptPreamble` + scaffold body + multi-line rule + subject-target line |

**No format-parsing logic exists.** `formatScaffoldBody` receives the raw format
string and switches on exact matches. `buildFormatSystemPrompt` passes the raw
format to `formatScaffoldBody`.

### `internal/prompt/system.go`

Constants + the two top-level builders:

| Entity | Line | Notes |
| --- | --- | --- |
| `promptPreamble` | ~L19 | Shared role + raw-output contract + essence; no trailing newline |
| `examplesIntro` | ~L29 | Auto-only "Match the tone…" line |
| `maturePromptHeader` | ~L35 | `promptPreamble + "\n\n" + examplesIntro` (compile-time const) |
| `antiReuseProhibition` | ~L42 | Verbatim anti-reuse block; has one em-dash U+2014 |
| `multilineRuleAllow` | ~L55 | "Only add a body… if history shows multi-line…" |
| `multilineRuleSingle` | ~L57 | "Only output a single-line subject (no body)." |
| `subjectTargetLine(int) string` | ~L61 | `"Target ~%d characters for the subject line."` |
| `BuildSystemPrompt(examples, hasMultiline, subjectTarget int, format, locale string) string` | ~L102 | Auto-examples topology or delegates to `buildFormatSystemPrompt` |
| `BuildFallbackPrompt(subjectTarget int, format, locale string) string` | ~L168 | §17.2 fallback or delegates to `buildFormatSystemPrompt` |

**Dispatch is on raw `format == "auto"` / `format != "auto"`.** Both builders
call `buildFormatSystemPrompt(format, ...)` and `formatScaffoldBody(format)`
with the raw format string (no parsing).

### `internal/prompt/planner.go`

`BuildPlannerSystemPrompt(examples []string, format, locale string, forcedCount, maxCommits int) string`
at L128. At L142:

```go
if format == "auto" {
    // examples loop (--- markers)
} else {
    b.WriteString(formatScaffoldBody(format))
}
```

**Dispatch and scaffold call use the RAW format string.** See DISCREPANCY below.

### `internal/config/load.go`

- `validFormats = []string{"auto", "conventional", "gitmoji", "plain"}` at L571
- `validateFormat(format string) error` at L577 — loops over `validFormats`, returns
  `fmt.Errorf("invalid format %q (valid: %s)", format, strings.Join(validFormats, ", "))`
  on mismatch. Pure (no I/O). Called once at tail of `Load()`.

### Format flow (cfg.Format → prompt builders)

`cfg.Format` (a plain string) flows **verbatim** to the prompt builders from 4 call sites:

| Caller | File:Line | Function called |
| --- | --- | --- |
| Single-commit path | `internal/generate/generate.go:637,644,650` | `BuildFallbackPrompt` / `BuildSystemPrompt` |
| Multi-commit message role | `internal/decompose/message.go:345,352,358` | `BuildFallbackPrompt` / `BuildSystemPrompt` |
| Multi-commit planner role | `internal/decompose/planner.go:77` | `BuildPlannerSystemPrompt` |
| Hook exec path | `internal/hook/exec.go:66,73,79` | `BuildFallbackPrompt` / `BuildSystemPrompt` |

**Design Decision 5 confirmed:** No new config field needed. `+body` is carried in
the `Format` string value and parsed inside the prompt package. All 4 call sites
gain `+body` support for free since they pass `cfg.Format` verbatim.

## DISCREPANCY: planner.go needs base-parsing (PRD says "no change needed")

The PRD §1 implementation table claims `BuildPlannerSystemPrompt` needs no change:
> "no planner-prompt change needed for the partitioning prompt"

This is **partially incorrect**. The planner does NOT need body-forcing (no
`bodyForceDirective`), which is what the PRD means. BUT it DOES need to parse the
base, because:

1. `if format == "auto"` at L142 uses the **raw** format string. With `auto+body`,
   this check fails (`"auto+body" != "auto"`) and the planner falls through to
   `formatScaffoldBody("auto+body")` → returns `""`. The planner would emit NEITHER
   examples NOR a scaffold — broken.

2. `formatScaffoldBody(format)` at L149 receives `"conventional+body"`, which hits
   the `default` case and returns `""`. The conventional scaffold would be lost.

**Required fix:** Add `base, _ := splitFormat(format)` before the L142 dispatch,
change the check to `base == "auto"`, and call `formatScaffoldBody(base)`.

This is a one-line-each change (3 lines total). It should be folded into the
subtask that creates `splitFormat` (S1), since the planner is a consumer of the
same parser.

## `+body` threading map

After implementation, `splitFormat(format)` is the single parsing seam:

```
splitFormat(format) → (base, forceBody)
  ├── validateFormat(format): validates base against validFormats (S3)
  ├── buildFormatSystemPrompt(format, ...): splits internally; forceBody swaps rule (S1)
  ├── BuildSystemPrompt(..., format, ...): splits; auto+body keeps examples, swaps rule (S2)
  ├── BuildFallbackPrompt(..., format, ...): splits; auto+body keeps fallback body, adds directive (S2)
  └── BuildPlannerSystemPrompt(..., format, ...): splits; keys examples/scaffold on base (S1, discrepancy fix)
```

## Byte-identity guarantee (FR-F1)

`auto` (no suffix) + empty locale must be byte-identical to pre-delta output.
Confirmed achievable: `splitFormat("auto")` returns `("auto", false)`; `forceBody`
is false; the assembly paths are unchanged. The ONLY behavioral change occurs when
`+body` is present in the format string.

## User-facing config/flag surfaces (verified line numbers)

| File | Line | Current text |
| --- | --- | --- |
| `internal/config/config.go` | 105–109 | Format field doc: lists 4 modes |
| `internal/config/bootstrap.go` | 376 | `# format = "auto" # auto\|conventional\|gitmoji\|plain; unknown = hard error (exit 1)` |
| `internal/cmd/root.go` | 201–203 | `"Message format: auto\|conventional\|gitmoji\|plain ..."` flag help |