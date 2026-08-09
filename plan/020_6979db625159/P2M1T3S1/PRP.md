name: "P2.M1.T3.S1 — Final README.md + overview coherence sweep across the entire delta (Mode B documentation sync)"
description: >
  The Mode B FINAL GATE for the plan-020 delta (Chocolatey+PowerShell installer replace Winget [P1] +
  model-example token refresh zai/glm-5.2 → anthropic/claude-haiku [P2]). It is primarily a
  VERIFICATION task with conditional straggler-fixes — it does NOT redo the token-replacement work the
  implementing subtasks already did. Concretely: (a) run `rg -ni winget README.md docs/` → MUST be zero
  hits [VERIFIED clean]; (b) run `rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob
  "!plan/**"` → MUST be zero hits [PREREQ: P2.M1.T2.S2 — the config-subsystem test rename — must have
  landed, since this grep does NOT exclude internal test files]; (c)-(d) verify README.md distribution +
  install-quickstart sections name Chocolatey + the PowerShell installer (not Winget) [VERIFIED coherent];
  (e) verify docs/packaging.md, docs/cli.md, docs/providers.md, docs/configuration.md are mutually + with-
  code consistent [VERIFIED coherent]; (f) spot-check internal doc cross-reference anchors; fix any stale
  ones (none expected — the delta is token-replacement, not section renames); (g) run `go test ./...` →
  green [VERIFIED baseline: go build OK]. CONDITIONAL STRAGGLER FIXES discovered by the sweep (outside the
  literal greps — grep (a) is README+docs-only; grep (b) matches zai/glm not winget — but inside the
  "entire delta coherence" mandate, contract (e) "consistent with the code"): install.sh L39/L45
  ("Scoop/Winget" user-facing rejection message + comment), plugins/asdf-stagecoach/README.md L88-91
  ("Scoop or Winget"), and plugins/asdf-stagecoach/bin/install L28 ("Scoop/Winget per PRD §21.3" stale
  comment) — all reference the now-DROPPED Winget channel (PRD §21.2) and must point at Scoop/Chocolatey/
  PowerShell instead. DELIBERATELY LEAVE docs/providers.md:127 `zai/gpt-5.4` — it is an intentional
  personal-override illustration (PRD §12.3), grep-compliant, the analog of P2.M1.T2.S2's gpt-5.4
  out-of-scope guard. Validates via the two contract greps (zero) + `go test ./...` (green) + gofmt + a
  scope guard. NO code/docs the siblings already landed are re-edited unless the sweep finds a real defect.

---

## Goal

**Feature Goal**: After the FULL plan-020 delta lands (P1 Winget→Chocolatey+PowerShell installer; P2
model-example cleanup zai/glm-5.2 → anthropic/claude-haiku), the repository's user-facing surfaces are
mutually coherent: zero Winget references in README+docs, zero stale `zai/glm` model-example tokens
outside spec/plan, the distribution docs agree with the code, no broken doc cross-references, and the
whole test suite is green. Any straggler the implementing subtasks missed (the sweep's actual deliverable)
is fixed.

**Deliverable**: (1) The two contract acceptance greps returning zero hits + `go test ./...` green, as
evidence the delta is coherent. (2) CONDITIONAL edits — only if the sweep finds stragglers. The verified
stragglers to fix: `install.sh` (L39 comment + L45 user-facing error), `plugins/asdf-stagecoach/README.md`
(L88-91 prose), `plugins/asdf-stagecoach/bin/install` (L28 comment) — all currently point Windows users at
the dropped Winget channel. No new files.

**Success Definition**:
- `rg -ni winget README.md docs/` → **zero hits** (gate a).
- `rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"` → **zero hits** (gate b; requires P2.M1.T2.S2 landed).
- `go test ./...` → **green** (gate g).
- README distribution/install sections name Chocolatey + the PowerShell installer; the 4 docs files are
  mutually consistent and consistent with the code; no broken internal doc anchors.
- The install.sh + asdf-plugin Winget stragglers are fixed (Scoop/Chocolatey/PowerShell).
- `docs/providers.md:127` `zai/gpt-5.4` is LEFT (intentional illustration, grep-compliant).
- `gofmt -l` clean on any edited .go files (none expected); scope clean (only the straggler files + any
  genuine defect found).

## User Persona (if applicable)

**Target User**: A new user reading the README/docs to install Stagecoach on Windows — and a maintainer
reviewing the delta for release. **Use Case**: the user follows the documented install path; the
distribution docs, the installers' error messages, and the plugins all agree on which Windows channels
exist. **Pain Points Addressed**: contradictory guidance — e.g. README says "Chocolatey / PowerShell
installer" but `install.sh`'s rejection message says "Scoop or Winget" (a channel that no longer ships).

## Why

- **Mode B final gate**: this is the changeset-level documentation sync that runs LAST, after every
  implementing subtask. Its job is to catch cross-cutting coherence defects the per-file subtasks
  structurally cannot see (e.g. an installer script + a plugin the sibling tasks never touched).
- **The Winget→Chocolatey decision is repo-wide**: PRD §21.2 dropped Winget ("Chosen OVER winget (v3.3)")
  for the whole distribution surface. Any remaining Winget reference — in a doc, an installer error, or a
  plugin README — is now a factual lie to the user. The sweep's value is finding the ones the siblings
  didn't own.
- **The model-example token refresh is also repo-wide** (gate b), but that grep is the authoritative
  zero-hit check; once T2.S2 lands it proves code+docs+tests all use the canonical `anthropic/claude-haiku`.

## What

A verification sweep with conditional fixes. Run the two greps (must be zero), run `go test ./...` (must
be green), verify README + the 4 docs files are coherent (they are), spot-check doc cross-references, and
fix the discovered install.sh + asdf-plugin Winget stragglers. Do NOT re-edit any sibling's already-correct
work; do NOT touch `docs/providers.md:127`'s `zai/gpt-5.4` (intentional).

### Success Criteria
- [ ] Gate (a): `rg -ni winget README.md docs/` → zero hits.
- [ ] Gate (b): `rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"` → zero hits.
- [ ] Gate (g): `go test ./...` → green (exit 0).
- [ ] README distribution + install-quickstart sections name Chocolatey (`choco install stagecoach`) and
      the PowerShell installer (`irm … install.ps1 | iex`); no Winget.
- [ ] docs/packaging.md, docs/cli.md, docs/providers.md, docs/configuration.md mutually consistent +
      consistent with the code (Chocolatey/PowerShell; `anthropic/claude-haiku`).
- [ ] install.sh L39 + L45 + plugins/asdf-stagecoach/README.md L88-91 + bin/install L28 → point at
      Scoop/Chocolatey/PowerShell (not Winget).
- [ ] No broken internal doc cross-reference anchors (spot-checked).
- [ ] `docs/providers.md:127` `zai/gpt-5.4` UNTOUCHED (intentional illustration).
- [ ] Scope: only the straggler files (+ any genuine defect found) modified; no sibling's correct work
      re-edited; no spec/PRD/plan/tasks touched.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact contract grep commands + their current verified status, the complete inventory of what
is ALREADY coherent (so the implementer doesn't redundantly re-edit it), the exact straggler locations +
verbatim fix text, the deliberate-leave note for `zai/gpt-5.4`, the hard prerequisite on P2.M1.T2.S2, the
spot-check recipe for cross-references, and the verified validation commands.

### Documentation & References

```yaml
# MUST READ — the complete verified state for THIS sweep (what's clean, the straggler inventory, the gates).
- docfile: plan/020_6979db625159/P2M1T3S1/research/findings.md
  why: "§0 the two contract greps + their CURRENT verified status (a clean; b pending T2.S2); §1 what is
        ALREADY coherent per file (README/packaging/cli/providers/configuration — do NOT re-edit); §2 the
        straggler inventory with verbatim old text + line numbers (install.sh L39/L45, asdf README L88-91,
        asdf bin/install L28); §3 the cross-reference spot-check recipe; §4 the go-test baseline; §5 the
        sibling-contract table (disjoint scopes); §6 the scope fence; §7 validation commands."

# MUST READ — the PREREQUISITE sibling contract (the in-flight task that makes gate (b) reach zero).
- docfile: plan/020_6979db625159/P2M1T2S2/PRP.md
  why: "T2.S2 is the config-subsystem test-fixture token refresh (8 files). Gate (b) does NOT exclude
        internal test files, so it reaches zero ONLY after T2.S2 lands. Treat T2.S2 as a HARD PREREQUISITE:
        run the sweep AFTER it ships. (T2.S2 also has the M3 `default_provider=\"zai\"`→`\"anthropic\"` rule
        the contract grep misses — not this task's concern, but explains why gate (b) can give a false pass
        without T2.S2's correctness grep.)"

# MUST READ — the PRD authority for the install-channel list this sweep enforces.
- url: PRD.md#21.2   # §21.2 goreleaser — "Chosen OVER winget (v3.3)"; Chocolatey pipe + PowerShell installer
  why: "Establishes WHY Winget is dropped (Defender scan hard-blocks the unsigned binary every release) and
        that the §21.3 Windows channels are now Scoop / Chocolatey / PowerShell installer. The straggler
        fixes must point at exactly these three."
- url: PRD.md#21.3   # §21.3 Install paths — the canonical install-path list (choco install; irm | iex)
  why: "The authoritative list this sweep checks the README/docs/installers against."
- url: PRD.md#12.3   # §12.3 Built-in provider: pi — anthropic/claude-haiku is the canonical example; z.ai is a personal override
  why: "Explains why docs/providers.md:127 `zai/gpt-5.4` is an INTENTIONAL personal-override illustration
        (grep-compliant) and must be LEFT — it is NOT a defect."

# MUST READ — the files to FIX (the stragglers). Read exact text before editing; re-anchor with grep.
- file: install.sh
  why: "L39 comment + L45 user-facing error message both say 'Scoop/Winget' / 'Scoop or Winget'. The Unix
        curl|sh installer rejects Windows with this message — it now points at a dropped channel."
  pattern: "The err() + comment block at L38-46 is the platform-detect rejection. Keep the structure; change
            ONLY the channel list wording."
  gotcha: "install.sh is shell — no gofmt. Preserve the err() printf format and the ${owner}/${name}
           interpolation verbatim. Test the script still parses: `bash -n install.sh`."
- file: plugins/asdf-stagecoach/README.md
  why: "L88-91 'Supported platforms' section: 'Windows users should use [Scoop] or [Winget]'. Markdown prose."
  gotcha: "Keep the existing Scoop markdown link; ADD a Chocolatey link + reference the PowerShell installer;
           DROP the Winget link. Match surrounding markdown style."
- file: plugins/asdf-stagecoach/bin/install
  why: "L27-28 comment: 'Windows users use Scoop/Winget per PRD §21.3)'. Shell comment only (the rejection
        at the `*)` case prints a generic message, no channel list)."
  gotcha: "Comment-only edit. `bash -n plugins/asdf-stagecoach/bin/install` to confirm it still parses."

# CONTEXT — the files ALREADY coherent (READ-ONLY — do NOT re-edit unless a real defect appears).
- file: README.md
  why: "Install section L81-134 (Chocolatey L101, PowerShell L104); features blurb L6; model examples
        L307/L316 (anthropic/claude-haiku). P1.M3.T1.S2 + P2.M1.T1.S2 landed these. Gate (a) confirms no winget."
- file: docs/packaging.md
  why: "Chocolatey section L8-33 (P1.M3.T1.S1). Gate (a) confirms no winget."
- file: docs/cli.md
  why: "upgrade channels L404 (P1.M3.T1.S2 + P2.M1.T1.S2). Gate (a)/(b) confirm clean."
- file: docs/providers.md
  why: "L72 anthropic/claude-haiku (P2.M1.T1.S2). L127 zai/gpt-5.4 — LEAVE (intentional, §12.3)."
- file: docs/configuration.md
  why: "L40/L232 anthropic/claude-haiku (P2.M1.T1.S2). Gate (b) confirms no glm."

# CONTEXT — the delta scope docs (provenance for what the sweep covers).
- docfile: plan/020_6979db625159/architecture/system_context.md
  why: "The validation table + Phase-2 scope-expansion note. Confirms the delta = Winget→Chocolatey +
        PowerShell installer (P1) + model-example cleanup (P2). The sweep is the final coherence gate."
- docfile: plan/020_6979db625159/architecture/model_cleanup_scope.md
  why: "Defines the gate-(b) acceptance criterion (zero hits) + the zai/gpt-5.4 personal-override carve-out."
```

### Current Codebase tree (relevant slice)

```bash
# FIX (conditional — the discovered stragglers):
install.sh                                  # EDIT — L39 comment + L45 user-facing error (Scoop/Winget → Scoop/Chocolatey/PowerShell)
plugins/asdf-stagecoach/README.md           # EDIT — L88-91 prose (drop Winget link; add Chocolatey + PowerShell)
plugins/asdf-stagecoach/bin/install         # EDIT — L28 comment (Scoop/Winget → Scoop/Chocolatey/PowerShell)
# (any genuine cross-reference defect found in the spot-check — none expected)

# READ-ONLY (already coherent — do NOT re-edit):
README.md                                   # P1.M3.T1.S2 + P2.M1.T1.S2 — gate (a)/(b) clean
docs/packaging.md                           # P1.M3.T1.S1 — gate (a) clean
docs/cli.md                                 # P1.M3.T1.S2 + P2.M1.T1.S2 — gate (a)/(b) clean
docs/providers.md                           # P2.M1.T1.S2 — L72 claude-haiku; L127 zai/gpt-5.4 LEAVE
docs/configuration.md                       # P2.M1.T1.S2 — gate (b) clean
docs/how-it-works.md, docs/README.md        # cross-reference spot-check targets
# PREREQUISITE (must have landed before running gate b):
internal/{config,cmd}/*_test.go             # P2.M1.T2.S2 (in-flight) — gate (b) reaches zero only after it ships
```

### Desired Codebase tree with files to be added/edited

```bash
install.sh                                  # EDIT — L39/L45 channel wording (conditional straggler fix)
plugins/asdf-stagecoach/README.md           # EDIT — L88-91 channel wording (conditional straggler fix)
plugins/asdf-stagecoach/bin/install         # EDIT — L28 comment wording (conditional straggler fix)
# (no new files)
```

### Known Gotchas of our codebase & Library Quirks

```text
# CRITICAL (gate (b) has a HARD PREREQUISITE): `rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**"
# --glob "!plan/**"` does NOT exclude internal test files. It currently (verified) returns hits ONLY in
# internal/{config,cmd}/*_test.go — that is P2.M1.T2.S2's in-flight scope. Gate (b) reaches ZERO only AFTER
# T2.S2 ships. Run this sweep AFTER T2.S2 lands; before that, gate (b) will fail and it is NOT this task's
# job to fix the test fixtures (T2.S2 owns them — disjoint files).

# CRITICAL (the contract greps do NOT cover the stragglers — that's the sweep's point): grep (a) is scoped
# to `README.md docs/` ONLY, so install.sh + plugins/ winget references do NOT fail it. grep (b) matches
# `zai/glm` not `winget`. The install.sh/asdf stragglers are discovered by the "consistent with the code" /
# "entire delta coherence" mandate — fix them even though the literal greps pass without the fix.

# CRITICAL (LEAVE zai/gpt-5.4 in docs/providers.md:127): it is an intentional personal-override illustration
# of the multi-backend prefix form (PRD §12.3), grep-compliant (the contract grep matches zai/glm, not
# zai/gpt), and the documented analog of P2.M1.T2.S2's zai/gpt-5.4* out-of-scope guard in
# config_init_interactive_test.go L105/107/119/120. Do NOT "fix" it — it is correct.

# GOTCHA (shell edits have no gofmt): install.sh + bin/install are bash. Validate with `bash -n <file>`
# (syntax parse) after editing, not gofmt. Preserve the err() printf format and the ${owner}/${name}
# interpolation in install.sh verbatim.

# GOTCHA (don't re-edit sibling work): README.md, docs/packaging.md, docs/cli.md, docs/providers.md,
# docs/configuration.md are ALREADY coherent (gates a/b clean; verified). Re-editing them is wasted churn
# and risks introducing a defect. Only edit if the sweep finds a REAL defect (none currently known).

# GOTCHA (the model grep can give a FALSE pass): gate (b) does NOT match a bare `default_provider = "zai"`
# (no /glm). T2.S2's correctness grep `rg 'zai' internal/cmd/config_test.go internal/config/migrate_test.go`
# is the real gate for that — but that is T2.S2's concern, not this sweep's. This sweep only asserts gate (b)
# is zero; if you want extra rigor, also confirm T2.S2's correctness grep is zero (proves the config tests
# don't silently emit zai/claude-haiku).

# GOTCHA (don't touch spec/PRD/plan/tasks): spec/** (AGENTS.md rule 1), PRD.md, plan/**/tasks.json,
# prd_snapshot.md, delta_prd.md are human/orchestrator-owned. The sweep reads them for reference only.
```

## Implementation Blueprint

### Data models and structure
None — this is a verification + prose/comment text edits. No structs, no schemas, no behavior change.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: PREREQUISITE CHECK — confirm P2.M1.T2.S2 has landed
  - CHECK the plan status: P2.M1.T2.S2 must be Complete (not "Implementing"). If it is still in-flight,
    STOP — gate (b) cannot reach zero until it ships (it owns the internal test-file tokens gate (b) sees).
  - PROOF T2.S2 landed: `rg -n "zai/glm|glm-5\.2|glm-5-turbo" internal/config/migrate_test.go
    internal/cmd/config_test.go` → zero hits (these are T2.S2's heaviest files). Also `rg -n 'zai'
    internal/cmd/config_test.go internal/config/migrate_test.go` → zero (T2.S2's M3 correctness gate).
  - If both are zero, T2.S2 is in (or effectively in); proceed. If not, gate (b) will fail — flag it.

Task 1: GATE (a) — `rg -ni winget README.md docs/` → expect ZERO
  - RUN: rg -ni winget README.md docs/
  - EXPECT: no output (zero hits). [VERIFIED clean — P1.M3.T1.S1/S2 landed.]
  - IF NON-ZERO: a winget reference survived in README/docs — fix it (the surrounding text should mirror
    docs/packaging.md's Chocolatey section), then re-run. (Not expected.)

Task 2: GATE (b) — `rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"` → expect ZERO
  - RUN the command from the repo root.
  - EXPECT: no output (zero hits) — after Task 0 confirms T2.S2 landed.
  - IF NON-ZERO: the hits are stragglers a sibling missed. Categorize: if in internal/**_test.go → that is
    T2.S1/T2.S2 territory (flag, do not fix unless T2.S2 is confirmed done and a file slipped); if in a
    doc or runtime .go → fix per the zai/glm-5.2 → anthropic/claude-haiku mapping (M1 bare→bare; M2
    zai/glm-5.2 → anthropic/claude-haiku). Re-run until zero.

Task 3: VERIFY README distribution + install-quickstart coherence (contract c+d)
  - READ README.md L81-134 (Install section): confirm it lists `choco install stagecoach` (L101) and
    `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex` (L104), plus the features
    blurb (L6) names Chocolatey + PowerShell installer. [VERIFIED — no edit.]
  - EXPECT: matches PRD §21.3. If a section is stale (it isn't), align it to docs/packaging.md.

Task 4: VERIFY the 4 docs files + cross-references (contract e+f)
  - READ/SCAN docs/packaging.md (Chocolatey L8-33), docs/cli.md (upgrade channels L404),
    docs/providers.md (L72 claude-haiku; L127 zai/gpt-5.4 — LEAVE), docs/configuration.md (L40/L232).
    Confirm mutually consistent + consistent with the code (detect.go/delegate.go channels). [VERIFIED.]
  - CROSS-REFERENCE SPOT-CHECK: run `rg -no '\]\([^)]+\.md#[^)]+\)' README.md docs/*.md` and pick a few
    anchors (e.g. docs/cli.md#upgrade, docs/providers.md#the-schema, docs/how-it-works.md#multi-commit-
    decomposition); confirm the target section heading exists in the target file (grep the slug). The delta
    is token-replacement not renames, so expect zero broken anchors. Fix any genuinely-broken anchor found
    (adjust the slug to match the actual heading). [None expected.]

Task 5: FIX the install.sh + asdf-plugin Winget stragglers (the sweep's real deliverable)
  - EDIT install.sh:
    * L39 comment: `# Windows users use Scoop/Winget (PRD §21.3). Reject anything else with a clear, actionable error.`
      → `# Windows users use Scoop/Chocolatey/PowerShell (PRD §21.3). Reject anything else with a clear, actionable error.`
    * L45 err: `err "Windows users: see https://github.com/${owner}/${name}#install (Scoop or Winget)."`
      → `err "Windows users: see https://github.com/${owner}/${name}#install (Scoop, Chocolatey, or the PowerShell installer)."`
    * VALIDATE: `bash -n install.sh` (parses). Preserve the ${owner}/${name} interpolation.
  - EDIT plugins/asdf-stagecoach/README.md (L88-91 "Supported platforms"):
    * Replace `Windows users should use [Scoop](https://scoop.sh/) or\n[Winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/) —` with a version listing
      Scoop, Chocolatey, and the PowerShell installer (keep the Scoop link; add a Chocolatey link
      https://chocolatey.org/docs/installation and/or point at the repo #install guide for the PowerShell
      one; drop the Winget link). Match surrounding markdown style.
  - EDIT plugins/asdf-stagecoach/bin/install (L27-28 comment):
    * `# Windows users use Scoop/Winget per PRD §21.3).` → `# Windows users use Scoop/Chocolatey/PowerShell per PRD §21.3).`
      (Comment-only; note the existing stray `)` after §21.3 — leave it or tidy it, either parses.)
    * VALIDATE: `bash -n plugins/asdf-stagecoach/bin/install` (parses).
  - After edits, RE-RUN: `rg -ni winget install.sh plugins/asdf-stagecoach/` → expect ZERO (proves the
    stragglers are gone from the distribution scripts/plugins too).

Task 6: GATE (g) — `go test ./...` → expect GREEN
  - RUN: go test ./...
  - EXPECT: exit 0, no FAIL package. [VERIFIED baseline: go build ./... OK; the config tests are internally
    consistent so they pass now and after T2.S2's rename.]
  - IF A PACKAGE FAILS: it is unrelated to the docs/prose edits (Tasks 1-5 touch only shell + markdown, no
    .go). Flag it; do not fix unrelated failures here unless it is a direct regression from the delta.

Task 7: SCOPE GUARD — confirm only intended files changed
  - RUN: git status --porcelain
  - EXPECT: only install.sh + plugins/asdf-stagecoach/README.md + plugins/asdf-stagecoach/bin/install (the
    stragglers) + any genuine cross-ref defect fixed. NO README.md/docs/*.go/spec/PRD/plan/tasks changes
    (those are either already-landed sibling work or human/orchestrator-owned).
```

### Implementation Patterns & Key Details

```bash
# PATTERN (gate a — the README+docs winget gate; verified clean):
rg -ni winget README.md docs/          # → must be empty

# PATTERN (gate b — the repo-wide model-token gate; needs T2.S2):
rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"   # → must be empty

# PATTERN (the straggler fix — shell comment + user-facing error, preserve interpolation):
err "Windows users: see https://github.com/${owner}/${name}#install (Scoop, Chocolatey, or the PowerShell installer)."
#   ↑ keep ${owner}/${name} verbatim; the err() helper does printf 'stagecoach: %s\n'

# PATTERN (post-fix proof — stragglers gone from the scripts/plugins too):
rg -ni winget install.sh plugins/asdf-stagecoach/    # → empty after Task 5
```

### Integration Points

```yaml
# NONE (config/migration/build/behavior). This is a docs/prose + shell-comment coherence sweep.
CONSUMES:
  - P1.M3.T1.S1/S2 (docs/packaging.md, cli.md, README winget→chocolatey) — already coherent.
  - P2.M1.T1.S2 (runtime code + user docs glm→claude-haiku) — already coherent.
  - P2.M1.T2.S1/S2 (provider + config test fixtures glm→claude-haiku) — S2 is the PREREQUISITE for gate (b).
PRODUCES:
  - The two contract greps at zero + go test ./... green (the delta's coherence proof).
  - install.sh + plugins/asdf-stagecoach winget stragglers fixed (the sweep's real delta).
PARALLEL: none — this is the FINAL task; it runs after all siblings.
```

## Validation Loop

> **Go repo + shell/markdown edits, NOT Python/ruff/mypy.** Use the contract greps, go test, `bash -n` for
> the shell scripts, and the scope guard. Run from the repo root.

### Level 1: The contract greps (the PRIMARY gates)

```bash
# Gate (a) — winget gone from README + docs.
rg -ni winget README.md docs/
# Expected: NO output (zero hits). [VERIFIED clean.]

# Gate (b) — stale model tokens gone repo-wide (excl spec/plan). Requires P2.M1.T2.S2 landed.
rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"
# Expected: NO output (zero hits). If non-empty and the hits are internal/**_test.go, T2.S2 has not landed
# (Task 0 prerequisite); if a doc/runtime .go slipped through, fix per the M1/M2 mapping.
```

### Level 2: Shell-script syntax (the install.sh + asdf edits)

```bash
# install.sh and the asdf plugin installer must still parse after the comment/error edits.
bash -n install.sh && echo "install.sh OK"
bash -n plugins/asdf-stagecoach/bin/install && echo "asdf install OK"
# Expected: both print "OK". (Comment + one error-string edit — no structural change.)

# Stragglers gone from the scripts/plugins too.
rg -ni winget install.sh plugins/asdf-stagecoach/
# Expected: NO output.
```

### Level 3: Whole-repo test (gate g)

```bash
# The full delta must leave the whole suite green.
go test ./...
# Expected: exit 0, no FAIL package. (Tasks 1-5 touch only shell + markdown — no .go — so a failure here is
# an unrelated regression to flag, not a docs-edit artifact.)
```

### Level 4: Scope guard + cross-reference sanity

```bash
# Only the intended files changed.
git status --porcelain
# Expected: install.sh + plugins/asdf-stagecoach/README.md + plugins/asdf-stagecoach/bin/install
#           (+ any genuine cross-ref defect). NOT README.md/docs/*.go/spec/PRD/plan/tasks.

# Guard: no out-of-scope / forbidden files touched.
git status --porcelain | grep -E 'spec/|PRD\.md|plan/.*tasks\.json|prd_snapshot|delta_prd' \
  && echo "FAIL: forbidden file touched" || echo "OK: scope clean"
# Expected: "OK: scope clean".

# Cross-reference spot-check (contract f): a sample of internal doc anchors resolve.
for anchor in "docs/cli.md#upgrade" "docs/providers.md#the-schema" "docs/how-it-works.md#multi-commit-decomposition"; \
  do f="${anchor%%#*}"; s="${anchor##*#}"; grep -qiE "^#+ .*$(echo $s | tr '-' ' ')" "$f" && echo "$anchor OK" || echo "$anchor MISSING"; done
# Expected: all "OK" (the delta renamed no sections). If "MISSING", fix the anchor's slug to the real heading.
```

## Final Validation Checklist

### Technical Validation
- [ ] Gate (a) `rg -ni winget README.md docs/` → zero hits
- [ ] Gate (b) `rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"` → zero hits
- [ ] Gate (g) `go test ./...` → green (exit 0)
- [ ] `bash -n install.sh` + `bash -n plugins/asdf-stagecoach/bin/install` → parse OK
- [ ] `rg -ni winget install.sh plugins/asdf-stagecoach/` → zero hits (stragglers fixed)

### Feature Validation
- [ ] README distribution + install-quickstart name Chocolatey (`choco install stagecoach`) + PowerShell
      installer (`irm … install.ps1 | iex`); no Winget
- [ ] docs/packaging.md, docs/cli.md, docs/providers.md, docs/configuration.md mutually consistent +
      consistent with the code
- [ ] install.sh L39/L45 + asdf README L88-91 + asdf bin/install L28 → Scoop/Chocolatey/PowerShell (no Winget)
- [ ] No broken internal doc cross-reference anchors (Level 4 spot-check all "OK")

### Scope-Boundary Validation
- [ ] `docs/providers.md:127` `zai/gpt-5.4` UNTOUCHED (intentional personal-override illustration, §12.3)
- [ ] NO already-coherent sibling file re-edited (README/packaging/cli/providers/configuration) unless a real
      defect was found (none expected)
- [ ] NO spec/**, PRD.md, plan/, tasks.json, prd_snapshot.md, delta_prd.md touched
- [ ] `git status --porcelain` shows only the straggler files (+ any genuine defect fixed)

### Documentation & Deployment
- [ ] Mode B: this IS the changeset-level documentation sync — the final sweep proving the whole delta is
      coherent. The two greps at zero + go test green ARE the documentation-coherence deliverable.

---

## Anti-Patterns to Avoid

- ❌ Don't run gate (b) before P2.M1.T2.S2 lands — it will fail on the in-flight config test fixtures, and
  those are NOT this task's to fix (T2.S2 owns them on disjoint files). Confirm T2.S2 shipped first (Task 0).
- ❌ Don't re-edit README.md / docs/packaging.md / docs/cli.md / docs/providers.md / docs/configuration.md —
  they are already coherent (gates a/b clean; verified). Re-editing is churn that risks introducing a
  defect. Only touch them if the sweep finds a REAL defect (none currently known).
- ❌ Don't "fix" `docs/providers.md:127` `zai/gpt-5.4`. It is an intentional personal-override illustration
  (PRD §12.3), grep-compliant (gate b matches zai/glm, not zai/gpt), and the documented analog of T2.S2's
  zai/gpt-5.4* out-of-scope guard. Leaving it is correct.
- ❌ Don't conflate the stragglers with the contract greps. install.sh + the asdf plugin winget references do
  NOT fail gate (a) [scoped to README+docs] or gate (b) [matches zai/glm]. Fix them because the task is the
  "coherence sweep across the ENTIRE DELTA" and they reference a dropped channel — not because a grep fails.
- ❌ Don't run ruff/mypy/pytest — this is a Go repo with shell + markdown edits. The gates are the two
  contract greps, go test, `bash -n`, and the scope guard.
- ❌ Don't touch the runtime .go files (detect.go/delegate.go/goreleaser/install.ps1) — P1.M1/M2 landed them
  correctly; the sweep verifies, it doesn't re-implement.
- ❌ Don't edit spec/**, PRD.md, plan/, or tasks.json — human/orchestrator-owned (AGENTS.md rules 1-2).
- ❌ Don't skip the `bash -n` parse check on install.sh + bin/install after editing them — the edits are
  shell comments + one error string, but confirm they still parse.
- ❌ Don't homogenize the asdf README markdown beyond the channel list — keep its existing structure/links;
  only swap the Winget reference for Chocolatey + PowerShell.

---

## Confidence Score

**One-pass success likelihood: 9/10.** This is a verification-dominated sweep: the two contract greps are
already verified clean (a) / pending one prerequisite (b = T2.S2), the README + 4 docs files are verified
coherent, go build is OK, and the actual implementation is a small, verbatim set of text edits to two shell
files + one markdown file (the install.sh/asdf winget stragglers) plus a cross-reference spot-check. The
1-point deduction is for (a) the hard prerequisite on P2.M1.T2.S2 (gate b can't be asserted until it lands)
and (b) the judgment call that the install.sh/asdf stragglers ARE in scope — they are outside the literal
greps but inside the "entire delta coherence" mandate, and the PRP defends the call explicitly. No code,
schema, or API change; pure coherence verification + prose/comment fixes.