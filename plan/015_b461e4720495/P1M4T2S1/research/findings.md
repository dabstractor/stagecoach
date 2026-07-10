# Findings — P1.M4.T2.S1 (Docs sync for per-role generation timeouts, FR-R7/§9.15)

Mode B documentation-sync task for the v2.8 per-role-timeout feature (FR-R7) that LANDED in P1.M1–P1.M3.
The Mode A "riding docs" shipped **unevenly**: docs/cli.md got the 4 `--<role>-timeout` flags + the
120s default (P1.M1.T2.S2 ✓); docs/configuration.md got the `stagecoach.role.<role>.timeout` git-config
key (P1.M1.T2.S3 ✓) but is MISSING the per-role **env-var** rows, the `[defaults]` FR-R7 comment, the
`[role.planner]` timeout example, and the per-role prose. README + how-it-works have ZERO timeout docs.
This task closes those gaps. Pure docs — no source.

## 0. The shipped user-facing surface (verified against LANDED code — match these names EXACTLY)

| Layer | Name | Source |
|---|---|---|
| Global default | `120s` (was 480s) | `internal/config/config.go:200` `Defaults().Timeout = 120 * time.Second` |
| Planner built-in role default | `480s` (the ONLY role with a built-in) | `internal/config/roles.go:11` `defaultRoleTimeouts = {"planner": 480s}` |
| Resolution | per-role override > built-in (planner 480s) > global 120s | `roles.go:96` `ResolveRoleTimeout(role, cfg)` |
| Flag | `--planner-timeout` / `--stager-timeout` / `--message-timeout` / `--arbiter-timeout` (string, zero default) | `internal/cmd/root.go:258-265` |
| Env | `STAGECOACH_PLANNER_TIMEOUT` / `_STAGER_` / `_MESSAGE_` / `_ARBITER_` (parseTimeout: "600s" or bare 600) | `internal/config/load.go:319` (`prefix+"_TIMEOUT"`) |
| Config file | `[role.<role>].timeout` (string, parsed) | `internal/config/file.go` `fileRoleConfig.Timeout` |
| **Git config** | **`stagecoach.role.<role>.timeout`** (string, parseTimeout) | `internal/config/git.go:167-175` |
| Global flag help | `--timeout` → "default 120s" | `root.go:169` |

**CRITICAL — the git-config layer WAS implemented** (`stagecoach.role.<role>.timeout`, git.go:167) even
though the delta_prd's "Out of scope" line said it would NOT be. The docs MUST document it. docs/configuration.md
already does (line 228 + the NOTE at 241 — P1.M1.T2.S3 Mode A ✓); **docs/cli.md's flag map does NOT**
(lines 453-456 show git config as "—" — STALE, must fix). The per-role git-config layer is **timeout-only**:
provider/model/reasoning have NO git-config layer (those stay "—" — that is correct).

**CRITICAL — the planner 480s is a per-role BUILT-IN, not a default-override target.** `--timeout` (global)
does NOT change the planner — it governs stager/message/arbiter + is the fallback. To change the planner
use `--planner-timeout` / `[role.planner].timeout` / `STAGECOACH_PLANNER_TIMEOUT` /
`stagecoach.role.planner.timeout`. State this asymmetry clearly (it is the #1 user confusion point).

**CRITICAL — single-commit path = message role = 120s.** The `message` role is the ONLY active role on the
single-commit path; it has NO built-in ⇒ inherits the global 120s (was 480s pre-v2.8 — a deliberate default
change; back-compat note: users who relied on 480s set `--timeout 480s` or `[role.message].timeout`).

## 1. PRD wording to match (verbatim anchors)
- **§9.15 FR-R7** (delta_prd:35-54): "Each role resolves its OWN generation timeout … Layers (highest wins,
  per role): `--<role>-timeout` flag > `STAGECOACH_<ROLE>_TIMEOUT` env > `[role.<role>].timeout` >
  built-in role default > global `[defaults].timeout` (default 120s). planner = 480s; stager/message/arbiter
  inherit the global 120s." + the back-compat sentence (delta_prd:54).
- **§16.1** (selected): "timeout 120s (global fallback for every role; **planner role default 480s** — FR-R7)".
- **§16.2 [defaults]** (selected): `timeout = "120s"   # global fallback for every role (FR-R7); planner defaults to 480s`.
- **§16.4** (selected): per-role config (provider/model/reasoning — the TIMEOUT twin mirrors these exactly).
- **§15.2** (selected): global `--timeout` default `120s`; the per-role `--<role>-timeout` rows.
- **FR-T5** (delta_prd:51): multi-turn per-turn timeout = the **message** role's resolved timeout
  (`message-timeout × (N+1)` total budget). This drives the how-it-works.md multi-turn edit.

## 2. Docs insertion points (exact file + heading + line, verified)

### docs/configuration.md — the PRIMARY target (contract a)
- **L86** `# timeout        = "120s"` (in the `[defaults]` file-format example) → ADD the FR-R7 comment
  (contract a3). The PRD §16.2 wording: `# global fallback for every role (FR-R7); planner defaults to 480s`.
- **L90-101** the `[role.*]` file-format block (`[role.planner] model = "opus"` etc.) → ADD a `[role.planner]`
  timeout example (contract a2): a commented `# timeout = "600s"   # per-role (FR-R7); overrides the planner's 480s built-in`.
- **Env-vars table (L169-202)**: has `STAGECOACH_TIMEOUT` (L184) + per-role PROVIDER/MODEL/REASONING
  (L190-202) but **NO per-role TIMEOUT rows** → ADD 4 rows `STAGECOACH_<ROLE>_TIMEOUT` (grouped with the
  per-role rows). Mirror the row format: `| Variable | Mirrors flag | Description | Example |`.
- **L251 per-role prose**: "Every role (including message) exposes `--<role>-provider`/`--<role>-model`/
  `--<role>-reasoning` (FR-R3)." → ADD timeout to that list + a sentence on per-role timeout resolution
  (planner 480s; others inherit 120s; the 4 surfaces) (contract a1).
- **Defaults table L133** `timeout | 120s | config.Defaults()` — the GLOBAL default is correct (120s). Leave
  as-is; the planner-480s detail belongs in the [defaults] comment (L86) + the per-role prose (L251), not
  the global defaults table.
- **Git-config table L228** `stagecoach.role.<role>.timeout` — ALREADY documented ✓ (P1.M1.T2.S3). The NOTE
  at L241 confirms the timeout-only asymmetry ✓. NO edit needed here.

### docs/how-it-works.md (contract b)
- **L132** (cross-ref after the Safety section): "See configuration.md for per-role model configuration and
  cli.md for the decompose and per-role flags." → ADD per-role timeout mention (FR-R7; planner 480s).
- **L303** (multi-turn section): "total wall-clock ≈ `timeout × (N+1)`" → this is the FR-T5 line; it now
  uses the **message role's** resolved timeout. Fix to `message-timeout × (N+1)` + cite FR-R7/FR-T5.
- There is NO dedicated "per-role configuration overview" section; L132 + the multi-turn section are the
  touch points (contract b's "if there's a per-role config overview" qualifier).

### README.md (contract c)
- **Multi-commit decomposition section (~L160-170)**: the per-repo config example shows `[role.planner]
  provider = "claude" model = "opus"`. README has ZERO timeout docs. → ADD a `# timeout = "600s"` line to
  the `[role.planner]` example AND/OR a `> [!NOTE]` that each role resolves its own timeout (`--<role>-timeout`,
  FR-R7) and the planner defaults to 480s.
- **§21.5 README structure**: FAQ is item 10; there is no dedicated timeout surface — the multi-commit
  section + the existing `--reasoning` NOTE are the right neighbor for a `--<role>-timeout` mention.

### docs/cli.md (contract d — VERIFY + fix stale)
- **L27** `--timeout <dur>` default `"120s"` — VERIFIED CORRECT (not 480s) ✓. Contract point (d) PASSES.
- **L60-63** the 4 `--<role>-timeout` flags — present ✓ (P1.M1.T2.S2).
- **Flag↔env↔git-config map L453-456**: `--planner-timeout` / `--stager-timeout` / `--message-timeout` /
  `--arbiter-timeout` rows show the git-config column as `—` (STALE — the layer EXISTS). FIX →
  `stagecoach.role.planner.timeout` / `.stager.` / `.message.` / `.arbiter.` (the ONLY per-role rows with a
  git-config value; provider/model/reasoning correctly stay `—`). This is the Mode B coherence fix.

## 3. The bootstrap.go gap (PRODUCTION file — OUT OF SCOPE, flag only)
`internal/config/bootstrap.go` (the `config init` template — user-facing but a .go PRODUCTION file):
- **L161** `[defaults]` line already says `# timeout = "120s"` ✓ (updated P1.M2.T2.S1) but LACKS the
  "planner defaults to 480s" comment.
- **Env-var block (L248-260)**: lists `STAGECOACH_TIMEOUT` + per-role PROVIDER/MODEL but NOT the per-role
  `STAGECOACH_<ROLE>_TIMEOUT` variants.
The delta_prd Phase 3 #1 wanted both fixed — but bootstrap.go is a SOURCE file (FORBIDDEN) and is NOT in
this task's contract (a/b/c/d are all docs files). DO NOT EDIT IT. The human-readable docs/configuration.md
env table + the [defaults] comment are the authoritative public docs and ARE in scope. Note the residual
in the PRP so the human is aware.

## 4. Validation tooling (verified)
- `.markdownlint.json` → MD013 (line length) OFF, MD033 (inline HTML) OFF, MD060 OFF. Existing docs are
  long-line single-paragraph (NO hard wrapping). `npx` + `node` installed → `npx markdownlint-cli2`.
- No `make docs` target — invoke markdownlint directly. `make build` works (smoke test if needed).
- Grep guards: every docs file mentions per-role timeout; no stale "480s" default claim remains in the
  global-flag context; cli.md flag map git-config column filled.

## 5. Scope fence (touch ONLY these; everything else READ-ONLY)
TOUCH: `README.md`, `docs/how-it-works.md`, `docs/configuration.md`, `docs/cli.md`.
DO NOT TOUCH: `PRD.md`, `plan/**`, `tasks.json`, `prd_snapshot.md`, any `internal/*`/`cmd/*` source
(especially `internal/config/bootstrap.go` — see §3), `providers/*.toml`, `FUTURE_SPEC.md`,
`.markdownlint.json`, `Makefile`, `go.mod`.
