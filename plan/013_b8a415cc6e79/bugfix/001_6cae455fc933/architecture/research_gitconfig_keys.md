# Research: git-config key-naming (validate PRD Issue 2)

**Verdict: PRD Issue 2 CONFIRMED.** Implementation reads **camelCase** exclusively.
Docs show snake_case in exactly two places. Git rejects underscores in final config-key segment.

## 1. Complete git-config keys read by `internal/config/git.go`

All multi-word keys are **camelCase**. Comment at git.go:102-105:
> "KEY NAMES ARE CAMELCASE: git config rejects underscores ('invalid key')."

| # | Exact key | Line | Type |
|---|---|---|---|
| 1 | `stagecoach.provider` | 114 | string |
| 2 | `stagecoach.model` | 119 | string |
| 3 | `stagecoach.output` | 124 | string |
| 4 | `stagecoach.format` | 130 | string |
| 5 | `stagecoach.locale` | 135 | string |
| 6 | `stagecoach.template` | 140 | string |
| 7 | `stagecoach.timeout` | 147 | string |
| 8 | **`stagecoach.autoStageAll`** | 158 | bool |
| 9 | `stagecoach.verbose` | 163 | bool |
| 10 | **`stagecoach.stripCodeFence`** | 168 | bool |
| 11 | `stagecoach.push` | 174 | bool |
| 12 | **`stagecoach.noVerify`** | 181 | bool |
| 13 | **`stagecoach.maxDiffBytes`** | 188 | int |
| 14 | **`stagecoach.maxMdLines`** | 195 | int |
| 15 | **`stagecoach.tokenLimit`** | 203 | int |
| 16 | **`stagecoach.diffContext`** | 214 | int |
| 17 | **`stagecoach.maxDuplicateRetries`** | 223 | int |
| 18 | **`stagecoach.subjectTargetChars`** | 230 | int |

## 2. Git-config keys documented in `docs/configuration.md`

### Discrepancies (SNAKE_CASE — WRONG)
| Line | Current | Required |
|---|---|---|
| 210 | `auto_stage_all = true` (INI example) | `autoStageAll = true` |
| 218 | `\| stagecoach.auto_stage_all \| bool \| git config --get --bool stagecoach.auto_stage_all \|` | `stagecoach.autoStageAll` in all 3 occurrences |

### NOT discrepancies (TOML config-file keys — legitimately snake_case, do NOT change)
- Line 87: `# auto_stage_all = true` (TOML `[defaults]` block)
- Lines 106-118: TOML `[generation]` block
- Lines 133-151: "Built-in defaults" table (option names, not git keys)
- Line 166: prose referencing TOML `auto_stage_all` key

**Rule:** bare option names or under `[generation]`/`[defaults]` = TOML (snake_case ✅).
Keys prefixed `stagecoach.` or in `[stagecoach]` INI block = git-config (must be camelCase).

## 3. Secondary finding (out of strict PRD scope)

Git-config table (lines 215-226) is missing 6 keys git.go reads:
`stagecoach.verbose`, `stagecoach.noVerify`, `stagecoach.maxDiffBytes`, `stagecoach.maxMdLines`,
`stagecoach.maxDuplicateRetries`, `stagecoach.subjectTargetChars`.
