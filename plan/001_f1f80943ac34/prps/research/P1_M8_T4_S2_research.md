# Research note — P1.M8.T4.S2: docs/ cross-cutting reference consolidation

## What this task is (Mode B consolidation)

P1.M8.T4.S2 is the **Mode B consolidation** task for the `docs/` reference set.
The Mode A per-file slices already landed in earlier milestones:

| Slice          | Owner task     | File                  | State                          |
| -------------- | -------------- | --------------------- | ------------------------------ |
| config ref     | P1.M5.T3.S1    | `docs/CONFIGURATION.md` | EXISTS, accurate               |
| CLI flags/env + exit codes | P1.M7.T2.S1 | `docs/CONFIGURATION.md` (§8/§9) | EXISTS, accurate   |
| providers list/show | P1.M7.T3.S1 | `docs/PROVIDERS.md`   | EXISTS, accurate               |
| providers list/show + built-ins (M2) | P1.M2.T3.S1 | (conceptual, folded into M7.T3.S1 docs) | n/a |
| install docstring | (distribution tasks) | `install.sh` head comment | EXISTS |

So both target files **already exist** and are accurate against the implementation.
The consolidation job is: (a) fill the one confirmed GAP, (b) de-dup / keep the
two files mutually consistent, (c) confirm README links both, (d) final proofread
against PRD §15 / §16 / §12.

## Verification: docs vs implemented behavior (all CONFIRMED accurate)

- `internal/config/defaults.go` `Default*` consts ↔ `docs/CONFIGURATION.md §1`
  defaults table: timeout 120s, auto_stage_all true, max_diff_bytes 300000,
  max_md_lines 100, max_duplicate_retries 3, subject_target_chars 50, output raw,
  strip_code_fence true. **MATCH.**
- `internal/provider/builtin.go` six manifests ↔ `docs/PROVIDERS.md` built-in table
  + `docs/CONFIGURATION.md` cross-refs: pi(stdin, glm-5-turbo), claude(stdin, sonnet),
  gemini(positional, gemini-2.5-pro), opencode(positional, unset), codex(stdin, unset),
  cursor(positional/command `agent`, unset). **MATCH.** (Verified by running
  `./stagehand providers list` and `./stagehand providers show pi` — output matches the
  documented format exactly.)
- FR34 precedence (7 levels), the 6 `STAGEHAND_*` env vars (note underscore in
  `STAGEHAND_NO_COLOR`), `stagehand.*` camelCase git-config keys + `--bool`,
  `.stagehand.toml` `[defaults]/[generation]/[provider.<name>]` tables ↔ PRD §16. **MATCH.**
- PRD §15.2 flag table (10 flags incl. `-a`, `-v`, no manual `--version`/`--help`) and
  §15.4 exit codes (0/1/2/3; 124 reserved) ↔ `docs/CONFIGURATION.md §8/§9`. **MATCH.**
- `internal/config/load.go` trust-notice string `"stagehand: repo-local config
  changed provider to <name>"` (§19) ↔ `docs/CONFIGURATION.md §6`. **MATCH.**

## CONFIRMED GAP (the one real piece of work)

`docs/PROVIDERS.md` is **missing** the **§12.7.1 "tools-disable asymmetry"** section
that the task contract explicitly requires ("the tools-disable asymmetry §12.7.1").
`grep -ci 'asymmetr\|read-only' docs/PROVIDERS.md` → **0**. The canonical PRD §12.7.1
text (read at PRD.md:716) defines the honest architectural split:

- **Explicit tool-disable flags:** pi (`--no-tools`), claude (`--tools ""`).
- **Read-only constraint instead (no global tool-off switch):** codex
  (`--sandbox read-only`), cursor (`--mode ask`), gemini (`--approval-mode default`).

Consequences (per PRD §12.7.1): safety preserved either way; latency varies; output is
still just the message. This is honest documentation of why the six `bare_flags` differ
in idiom. The current PROVIDERS.md lists the six built-ins in a one-line table but does
NOT explain this split — the consolidation must ADD it.

## Cross-doc duplication analysis (intentional, keep consistent)

The provider **field-merge rule** appears in BOTH files:
- `docs/CONFIGURATION.md §5` "Provider-override field-merge" (config-syntax + precedence lens).
- `docs/PROVIDERS.md` "Field-merge: overriding a provider (FR48)" (show-output lens).

These are **not** contradictory duplication — they serve different audiences (the
configurer vs the debugger) and currently agree verbatim ("field-by-field; only set
fields override; slices/maps replaced wholesale; unknown name = brand-new provider").
The consolidation must verify they STAY in agreement; it should NOT delete one (each
is the right doc for its reader) but SHOULD make the cross-link explicit.

The §12.8 user-provider TOML example is IDENTICAL across README ↔ CONFIGURATION.md §4
golden ↔ PRD §12.8/§16.2 (`[provider.myagent] ... ["--no-mcp", "--ephemeral"]`).
PROVIDERS.md references §12.8 only in one sentence in the field-merge section → add a
clearer pointer.

## README linkage (CONFIRMED present — owned by sibling P1.M8.T4.S1)

`README.md` already links both docs at lines 117 (Configure), 146–147 (CLI + config
reference). README is owned by sibling task P1.M8.T4.S1; THIS task verifies the links
resolve and the cross-referenced blurbs stay accurate (it does not rewrite README).

## Validation tooling available

- `make build` / `make test` / `make vet` / `gofmt -s -w .` — Go toolchain (confirms no
  regression; docs edits must not break code).
- **No markdown linter** configured (no markdownlint/mdl/.prettierrc). `.editorconfig`
  has no `max_line_length` rule. So docs gates are: file-existence, `grep` content
  checks, and binary smoke tests (`./stagehand providers list|show pi`) proving the
  documented output matches reality.
- `go build ./...` currently **green** (verified).

## Source-of-truth anchors to quote in the PRP

- `PRD.md §12.7.1` (716) — canonical tools-disable asymmetry text to adapt.
- `PRD.md §12.1` (412) — manifest schema (PROVIDERS.md TOML-key table already mirrors it).
- `PRD.md §12.8` (735) — user-defined provider example (consistency target).
- `PRD.md §15.2`/`§15.4` (935/958) — flags + exit codes (proofread target).
- `PRD.md §16.1`–`§16.3` (1002/1014/1051) — precedence + golden + git-config keys.
- `internal/provider/builtin.go` — the six real manifests (the truth behind every table).
- `internal/config/{defaults,load}.go` — the truth behind §1 defaults + §5 merge + §6 notice.
- `plan/001_f1f80943ac34/architecture/external_deps.md §B/§C` — verified manifest provenance.
