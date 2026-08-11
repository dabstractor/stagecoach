name: "P1.M3.T4.S1 — Review and update docs/packaging.md and README.md for upgrade-detection accuracy (BUG-004/BUG-007/BUG-008)"
description: >
  A DOC-ONLY changeset-level documentation sync. The P1.M3 upgrade fixes are mostly INTERNAL
  behavior changes; this task verifies the shipped docs accurately reflect them and corrects the one
  real gap. VERDICT (research/findings.md): README.md is ALL ACCURATE — L90 already says
  "Homebrew (macOS / Linuxbrew)" and L161 correctly lists Homebrew under DELEGATE / AUR-Nix under PRINT;
  BUG-004 (cmdRunner per-query timeout/WaitDelay) and BUG-008 (releases.go c.Repo segment-escape) are
  pure-internal fixes with ZERO doc surface (no shipped doc references PM-query timeouts or the GitHub
  releases URL/repo) → CONFIRMED NO-OP for those. The ONE actionable gap is docs/packaging.md: it is the
  MAINTAINER doc that documents per-channel "`stagecoach upgrade` behavior" (Chocolatey has a block at
  L33; PowerShell has one at L48), but the brew channel — a first-class DELEGATE channel whose
  Linuxbrew detection BUG-007 specifically fixed — has NO such block (only a passing intro mention as a
  publishing tap). The contract LOGIC explicitly directs "Review docs/packaging.md for Homebrew/Linuxbrew
  coverage." DELIVERABLE: add ONE compact `## Homebrew / Linuxbrew` section to docs/packaging.md
  (immediately after the intro, before `## Chocolatey`) mirroring the Chocolatey block — stating that
  `stagecoach upgrade` detects BOTH macOS Homebrew (`/opt/homebrew/Cellar/`, `/usr/local/Cellar/`) AND
  Linuxbrew (`/home/linuxbrew/.linuxbrew/Cellar/`) and DELEGATES to `brew upgrade stagecoach` (brew owns
  the binary under the Cellar → never self-swapped; run directly, not printed, unlike Chocolatey). Every
  claim is code-verified (pathHeuristics detect.go:372-376; delegate_test.go:60; printCommand
  delegate.go:344). DOC-ONLY: zero code/tests/spec. ZERO overlap with the parallel P1.M3.T3.S1 (edits
  internal/upgrade/releases.go CODE — a different file). Scope: `git status --porcelain` ==
  docs/packaging.md ONLY (README.md + cli.md get zero edits — confirmed accurate / out of scope).

---

## Goal

**Feature Goal**: Make the changeset-level upgrade-detection documentation ACCURATE and COMPLETE so the
shipped docs reflect the BUG-004/BUG-007/BUG-008 improvements. Concretely: (1) confirm README.md is
already accurate (no-op); (2) confirm BUG-004 + BUG-008 have no doc surface (no-op); (3) close the one
real gap — docs/packaging.md has no brew-channel "`stagecoach upgrade` behavior" note, so the BUG-007
Linuxbrew-detection improvement is undocumented in the maintainer doc that exists precisely to document
per-channel upgrade behavior.

**Deliverable** (doc-only, ONE file): insert a compact `## Homebrew / Linuxbrew` section into
`docs/packaging.md` (after the intro, before `## Chocolatey`), mirroring the existing Chocolatey block's
"`stagecoach upgrade` behavior" pattern. No code, no tests, no other docs, no spec. Plus the recorded
review confirming everything else is accurate.

**Success Definition**:
- README.md gets ZERO edits (re-confirmed accurate: L90 "macOS / Linuxbrew"; L161 brew=DELEGATE, AUR/Nix=PRINT).
- docs/packaging.md gains the `## Homebrew / Linuxbrew` section naming all 3 Cellar roots
  (`/opt/homebrew/Cellar/`, `/usr/local/Cellar/`, `/home/linuxbrew/.linuxbrew/Cellar/`) and stating brew
  DELEGATES to `brew upgrade stagecoach` (run directly, not printed; never self-swapped).
- BUG-004 (timeout) + BUG-008 (URL escaping): confirmed no doc surface — ZERO edits anywhere.
- DOC-ONLY: `make test` + `make lint` green and byte-for-byte unaffected for all CODE; the mkdocs site
  still builds (packaging.md is a docs-site page).
- `git status --porcelain` == `docs/packaging.md` ONLY.

## User Persona (if applicable)

**Target User**: A maintainer (or a Linuxbrew user reading the maintainer notes) who wants to know how
`stagecoach upgrade` treats a brew install — on macOS AND on Linux — without reading Go source. The
existing Chocolatey block answers this for Chocolatey; the new section answers it for brew.

**Use Case**: A Linuxbrew user runs `stagecoach upgrade`; the doc tells them it is detected (via the
Linuxbrew Cellar root) and delegates to `brew upgrade stagecoach` rather than self-swapping.

**User Journey**: user installs via `brew` (macOS or Linux) → runs `stagecoach upgrade` → detection
matches the Cellar path → stagecoach runs `brew upgrade stagecoach` → the doc explains why (brew owns
the binary; FR-U1/FR-U4 forbids self-swap).

**Pain Points Addressed**: Before BUG-007, a Linuxbrew install was mis-detected as `direct` and
self-swapped (fighting brew). packaging.md never documented brew upgrade behavior at all. This section
documents the corrected delegate behavior for both brew platforms.

## Why

- **The contract directs attention here**: the item's LOGIC explicitly says "Review docs/packaging.md
  for Homebrew/Linuxbrew coverage." packaging.md is THE maintainer doc for per-channel upgrade behavior,
  and the brew channel's behavior was undocumented despite being a first-class delegate channel that
  BUG-007 improved.
- **Completes the Chocolatey/brew symmetry**: Chocolatey (a PRINT channel) has a detailed
  "`stagecoach upgrade` behavior" block; brew (a DELEGATE channel) had none. The new block is the brew
  twin — same structure, accurate to brew's run-don't-print behavior.
- **Reflects BUG-007 without internal jargon**: the section documents that Linuxbrew is detected
  (alongside macOS Homebrew) — the user-visible improvement — without leaking the internal "BUG-007" id
  into shipped docs (matching the Chocolatey block's style).
- **Cheap, zero-risk**: doc-only. Cannot change behavior, cannot break tests. Validation = markdownlint
  clean + mkdocs builds + grep guards.

## What

**User-visible behavior**: none (doc-only).

**Technical change**: insert ONE markdown section into `docs/packaging.md`. No code, no tests, no other
docs, no spec, no cli.md.

### Success Criteria
- [ ] docs/packaging.md has a new `## Homebrew / Linuxbrew` section placed after the intro paragraph and
      before `## Chocolatey`.
- [ ] The section names all 3 Cellar roots (`/opt/homebrew/Cellar/`, `/usr/local/Cellar/`,
      `/home/linuxbrew/.linuxbrew/Cellar/`) and states `stagecoach upgrade` DELEGATES to
      `brew upgrade stagecoach` (run directly, not printed; never self-swapped, FR-U1/FR-U4).
- [ ] The section mirrors the Chocolatey block's `- **`stagecoach upgrade` behavior**:` bullet pattern.
- [ ] README.md gets ZERO edits (confirmed accurate — L90/L161).
- [ ] BUG-004 + BUG-008: ZERO doc edits anywhere (confirmed no doc surface).
- [ ] DOC-ONLY: no `.go`/spec/tasks.json/cli.md change; `make test` + `make lint` green; mkdocs builds.
- [ ] `git status --porcelain` == `docs/packaging.md` ONLY.

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim placement anchor (intro's closing `)` line + `## Chocolatey` heading), the
verbatim drop-in section text, the per-line README accuracy table, the confirmed no-op verdicts for
BUG-004/BUG-008, the code-verified brew delegation facts (with file:line), the parallel-task no-conflict
confirmation, the validation commands, and the grep guards.

### Documentation & References

```yaml
# MUST READ — the per-surface review + the exact placement anchor + the verbatim drop-in text + the
#              code-verified accuracy cross-check (the PRIMARY research artifact).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T4S1/research/findings.md
  why: "§0 the corrected behavior (BUG-004/007/008 LANDED + the brew delegation facts). §1 README table —
        ALL accurate (no-op). §2 packaging.md sweep — no stale claim, but the brew-channel gap. §3 the
        EXACT placement anchor + the verbatim drop-in `## Homebrew / Linuxbrew` text + the per-claim
        code cross-check. §4 scope fences. §5 validation."
  critical: "README.md needs ZERO edits. The ONLY file to change is docs/packaging.md. BUG-004 + BUG-008
             are no-ops (no doc surface). The new section must name all 3 Cellar roots and state brew
             DELEGATES (runs `brew upgrade stagecoach` directly, NOT printed) — brew is NOT a
             printCommand channel (unlike Chocolatey)."

# MUST READ — the file under edit (the intro placement anchor + the Chocolatey block to mirror).
- file: docs/packaging.md
  why: "L3-7 intro: 'Homebrew (tap `dabstractor/homebrew-stagecoach`), Scoop (…), and AUR (…) are all
        wired into `release.yml` … (AUR publish is currently disabled: …)'. INSERT the new section
        between this intro's closing `)` line and `## Chocolatey` (L9). L33: the Chocolatey
        '- **`stagecoach upgrade` behavior**:' block to MIRROR. L48: the PowerShell 'direct/self-swap'
        note (context). The file is a docs-site page (mkdocs) — keep heading levels consistent (## peer
        to ## Chocolatey)."
  pattern: "Channels are `##` sections (## Chocolatey, ## Nix (flakes), ## Documentation site). Each
            channel's upgrade behavior is a '- **`stagecoach upgrade` behavior**:' bullet. Inline
            `code` for commands/paths. FR refs (FR-U1/FR-U4) are cited inline, matching Chocolatey's
            style. No internal BUG-IDs in the shipped text."
  gotcha: "The intro ALREADY mentions 'Homebrew (tap …)' as a PUBLISHING channel — that is NOT
           duplicative of the new section (the intro is about release.yml pushes; the new section is
           about `stagecoach upgrade` DETECTION/delegation). Chocolatey has the same intro-mention +
           detailed-section pattern."

# MUST READ — the code that PROVES the doc claims (so the section is accurate, not aspirational).
- file: internal/upgrade/detect.go
  section: "pathHeuristics table (L370-377): the 3 Cellar roots → ChannelBrew
            ('/opt/homebrew/Cellar/' Apple Silicon; '/usr/local/Cellar/' Intel macOS;
            '/home/linuxbrew/.linuxbrew/Cellar/' Linuxbrew, BUG-007). ChannelDirect is the ONLY
            self-swap channel (L50). The tier-(b) probe 'brew list stagecoach' (L288, goos darwin+linux)."
  why: "Proves the 3 roots + that brew ≠ direct (so it never self-swaps). READ-ONLY — do NOT edit
        detect.go (parallel P1.M3.T1.S1 BUG-004 + P1.M3.T2.S1 BUG-007 own it)."

# MUST READ — the delegation table (brew RUNS, does not PRINT).
- file: internal/upgrade/delegate.go
  section: "printCommand (L344-392) handles ONLY AUR/Nix/Deb/Rpm/Chocolatey (the PRINT channels).
            ChannelBrew is ABSENT from printCommand → it is a DELEGATE channel (RUN, not print).
            delegate_test.go:60 {'brew', ChannelBrew, [['brew','upgrade','stagecoach']]} proves the
            exact command stagecoach runs."
  why: "Proves 'delegates to brew upgrade stagecoach, run directly, not printed' — the key contrast
        with Chocolatey (which IS printed). READ-ONLY — do NOT edit delegate.go."

# CONTEXT — the PRD recommendation/bug this reflects + the contract framing.
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "## BUG-007 (MINOR): Linuxbrew Cellar root missing from pathHeuristics — names the missing
            '/home/linuxbrew/.linuxbrew/Cellar/' root + the fix. ## BUG-004 / ## BUG-008 confirm those
            are internal (no doc surface)."
  why: "BUG-007 is the user-visible improvement the new doc section reflects. BUG-004/BUG-008 are the
        confirmed no-ops."

# CONTEXT — the parallel PRP (P1.M3.T3.S1) — confirm ZERO conflict (it edits CODE, a different file).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M3T3S1/PRP.md
  why: "Confirms the sibling edits ONLY internal/upgrade/releases.go + releases_test.go (CODE) and is
        [Mode A] (code-comment-only). This item edits a shipped DOC — DIFFERENT file. No merge conflict."
  critical: "Do NOT edit internal/upgrade/releases.go (the sibling owns it). This item is
             docs/packaging.md ONLY."

# CONTEXT — README accuracy (confirm no edit needed).
- file: README.md
  section: "L90 '# Homebrew (macOS / Linuxbrew)' (already acknowledges Linuxbrew); L161 the Updating
            paragraph ('detects how you installed it and delegates … Homebrew … prints … AUR … Nix …
            self-swaps only for a direct install'); L164 'stagecoach upgrade  # detect → delegate
            (or self-swap), confirm, apply'."
  why: "READ-ONLY — README is accurate. Do NOT edit README.md (the no-op half of this task)."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  packaging.md          # EDIT — insert `## Homebrew / Linuxbrew` section after the intro, before `## Chocolatey`. DOC-ONLY.
  cli.md                # READ-ONLY — the `#upgrade` flag reference; out of scope (contract names packaging.md + README.md only).
  how-it-works.md / configuration.md / providers.md / README.md / ci-validation.md / windows-test-support.md  # READ-ONLY — no upgrade-detection stale claim.
README.md               # READ-ONLY — L90/L161 already accurate (no-op).
internal/upgrade/detect.go     # READ-ONLY — pathHeuristics (3 Cellar roots); owned by parallel BUG-004/BUG-007.
internal/upgrade/delegate.go   # READ-ONLY — printCommand (PRINT channels) + brew delegation.
internal/upgrade/releases.go   # READ-ONLY — BUG-008; owned by parallel P1.M3.T3.S1.
spec/                          # READ-ONLY — human-owned; FR-U1/U2/U3/U4 state the correct behavior.
mkdocs.yml / .markdownlint.json / Makefile   # READ-ONLY — validation (mkdocs build, markdownlint, make test/lint).
```

### Desired Codebase tree with files to be added/modified

```bash
docs/packaging.md   # EDIT — +`## Homebrew / Linuxbrew` section (compact, ~10 lines). NOTHING ELSE.
# No new files. No README/cli.md/spec/code/test/go.mod/Makefile/PRD/task changes.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (DOC-ONLY — do not touch code): the edit is markdown text in docs/packaging.md ONLY. Do NOT
     edit any .go file, README.md, cli.md, spec/, tasks.json, or prd_snapshot. If you find yourself
     editing anything but docs/packaging.md, STOP. -->

<!-- CRITICAL (brew DELEGATES, it does not PRINT): ChannelBrew is ABSENT from printCommand
     (delegate.go:344 — that handles only AUR/Nix/Deb/Rpm/Chocolatey). stagecoach RUNS
     `brew upgrade stagecoach` directly (delegate_test.go:60). So the doc must say "delegates to /
     runs brew upgrade stagecoach", NOT "prints brew upgrade stagecoach". Chocolatey is the PRINT
     contrast (it prints because `choco upgrade` needs admin). Mis-stating this re-introduces the
     exact inaccuracy the review is meant to remove. -->

<!-- CRITICAL (name all 3 Cellar roots): BUG-007's whole point is Linuxbrew detection. The section MUST
     list `/home/linuxbrew/.linuxbrew/Cellar/` alongside the two macOS roots, or it fails to reflect
     the fix. All 3 are verified at detect.go:372-376. -->

<!-- GOTCHA (heading level — ## peer to ## Chocolatey): packaging.md uses `##` for channel sections
     (## Chocolatey, ## Nix (flakes), ## Documentation site). Use `## Homebrew / Linuxbrew` (NOT ### —
     ### is the 'PowerShell installer' sub-pattern). mkdocs builds a nav from these headings; a level
     shift would reorder the site nav. -->

<!-- GOTCHA (placement — after intro, before ## Chocolatey): insert between the intro's closing `)`
     (the AUR-disabled note) and `## Chocolatey`. Do NOT insert inside the Chocolatey section or after
     Nix. Anchor on the literal `## Chocolatey` heading (prepend the new section + a blank line). -->

<!-- GOTCHA (no internal BUG-IDs in shipped text): the Chocolatey block cites FR-U1/FR-U4 but no
     'BUG-xxx' tags. Match that — cite FR-U1/FR-U4; do NOT write 'BUG-007' in packaging.md (it is a
     shipped maintainer doc, not an internal tracker). -->

<!-- GOTCHA (mkdocs site build): packaging.md is rendered by mkdocs-material (mkdocs.yml). After the
     edit, `mkdocs build` (or `make lint` if it runs markdownlint) must stay clean — a stray backtick
     or broken heading anchor breaks the site. Validate before committing. -->
```

## Implementation Blueprint

### Data models and structure

None. Doc-only — no types, no code.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: RE-CONFIRM the no-op half (sanity) — README + BUG-004 + BUG-008 get ZERO edits
  - RUN the §1/§2 sweeps from research/findings.md:
      grep -niE 'linuxbrew|/opt/homebrew|/usr/local|cellar|detect|timeout|WaitDelay|c\.Repo|releases' README.md docs/packaging.md
      grep -rniE 'package-manager query|PM query|query timeout|releases url|repo.*escape' README.md docs/
  - VERIFY: README L90 says "macOS / Linuxbrew"; L161 lists Homebrew under delegate + AUR/Nix under print;
    NOTHING references PM-query timeouts or the GitHub releases URL/repo (so BUG-004/BUG-008 have no
    surface to correct).
  - ACTION: NONE on README/cli.md/any code. The no-op review is recorded in research/findings.md §1/§2.
  - OUTPUT: a one-line mental confirmation. (If a NEW stale claim is found, STOP and surface it — but
    the research found none.)

Task 1: INSERT the `## Homebrew / Linuxbrew` section into docs/packaging.md
  - LOCATE: the boundary between the intro paragraph and `## Chocolatey`. The intro ends:
        …each pushes to its target repo on tag. (AUR publish is currently disabled: its `git_url` is commented out in
        `.goreleaser.yaml` while aur.archlinux.org recovers.)

        ## Chocolatey
  - INSERT (between the `)` line and `## Chocolatey`) — drop-in, mirrors the Chocolatey block:
        ## Homebrew / Linuxbrew

        The tap is `dabstractor/homebrew-stagecoach` (published by `release.yml` on every tag). The same `brew`
        tool serves **both** macOS Homebrew (`/opt/homebrew` on Apple Silicon, `/usr/local` on Intel) **and**
        Linuxbrew (`/home/linuxbrew/.linuxbrew`) on Linux.

        - **`stagecoach upgrade` behavior**: `stagecoach upgrade` detects a brew install by its Cellar path
          (`/opt/homebrew/Cellar/`, `/usr/local/Cellar/`, or `/home/linuxbrew/.linuxbrew/Cellar/`) and
          **delegates** to `brew upgrade stagecoach`. Brew owns the binary under the Cellar, so it is never
          self-swapped (FR-U1/FR-U4); unlike Chocolatey, `brew upgrade` is user-space and is run directly
          (streaming its output), not printed.
  - FOLLOW pattern: the file's `##` channel sections + the Chocolatey block's
    `- **`stagecoach upgrade` behavior**:` bullet; inline `code` for commands/paths; FR-U1/FR-U4 cited inline.
  - GOTCHA: keep the blank line between the intro `)` and the new `##`, and between the new section and
    `## Chocolatey` (markdown needs blank lines around ATX headings). Anchor on the literal `## Chocolatey`
    line — prepend `<new section>\n\n` before it.
  - GOTCHA: do NOT add a `BUG-007` tag (shipped doc — match Chocolatey's no-bug-id style).

Task 2: VERIFY — doc-only: code unaffected, markdownlint clean, mkdocs builds, grep guards, scope guard
  - npx markdownlint-cli2 docs/packaging.md 2>/dev/null || npx -y markdownlint-cli docs/packaging.md || make lint
  - (if mkdocs available) mkdocs build --strict   # packaging.md is a docs-site page; --strict fails on bad anchors
  - make test && make lint                        # green (doc-only — no code touched)
  - git status --porcelain                        # docs/packaging.md ONLY
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (the channel-section structure — mirror Chocolatey): a `## <Channel>` heading, a one-paragraph
     intro (tap + platforms), then a '- **`stagecoach upgrade` behavior**:' bullet with the detect →
     delegate/print → self-swap/not outcome + the FR citation. -->
<!--   ## Homebrew / Linuxbrew
       The tap is `dabstractor/homebrew-stagecoach` … serves both macOS Homebrew … and Linuxbrew …
       - **`stagecoach upgrade` behavior**: detects a brew install by its Cellar path (…3 roots…) and
         delegates to `brew upgrade stagecoach`. Brew owns the binary … never self-swapped (FR-U1/FR-U4);
         unlike Chocolatey, `brew upgrade` is user-space and is run directly, not printed. -->

<!-- PATTERN (brew = DELEGATE not PRINT): state 'delegates to / runs brew upgrade stagecoach', NEVER
     'prints brew upgrade …'. Chocolatey is the print contrast. -->
```

### Integration Points

```yaml
CODE: NONE (doc-only). No .go/go.mod/Makefile/test change.
DOCUMENTATION:
  - docs/packaging.md gains the `## Homebrew / Linuxbrew` section — the brew twin of the Chocolatey block.
  - README.md is a confirmed no-op (L90/L161 accurate).
  - BUG-004 (timeout) + BUG-008 (URL escaping): confirmed no doc surface — zero edits.
SCOPE FENCES:
  - Touches ONLY docs/packaging.md (one inserted section).
  - Does NOT edit README.md (confirmed accurate), cli.md (out of scope), spec/ (human-owned),
    internal/upgrade/* (parallel BUG-004/007/008 code owners), any PRD/task file.
  - Adds NO code, NO test, NO flag, NO config, NO dependency.
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Doc-only: no code touched → build byte-for-byte unaffected.
go build ./...
# Expected: clean (identical to before).

# Markdown lint (repo has .markdownlint.json). Run on the edited file.
npx markdownlint-cli2 docs/packaging.md 2>/dev/null \
  || npx -y markdownlint-cli docs/packaging.md \
  || echo "NOTE: markdownlint not installed; rely on make lint below"
# Expected: clean. If a style violation appears (long line, etc.), fix ONLY the lines you added.

# mkdocs site build (packaging.md is a docs-site page; --strict catches broken anchors/headings).
mkdocs build --strict 2>/dev/null || echo "NOTE: mkdocs not installed locally; CI docs.yml will build it"
# Expected: builds clean (no duplicate-heading or broken-anchor error from the new ## section).

# Project lint (markdown + go via golangci-lint).
make lint
# Expected: zero errors (doc-only).

# Scope guard: ONLY packaging.md changed.
git status --porcelain
# Expected: docs/packaging.md ONLY.
```

### Level 2: Unit Tests (Component Validation)

```bash
# Doc-only: the suite must pass UNCHANGED (no .go touched).
make test
# Expected: green (identical to before).
```

### Level 3: Integration Testing (System Validation)

```bash
# N/A — doc-only change has no runtime behavior. The brew detection/delegation the doc DESCRIBES is
# exercised by internal/upgrade's detect_test.go + delegate_test.go (code-owned, unchanged by this task).
```

### Level 4: Creative & Domain-Specific Validation (grep guards + no-op re-confirm)

```bash
# Guard 1: the new section exists + names all 3 Cellar roots + the delegation command.
grep -n '## Homebrew / Linuxbrew' docs/packaging.md                       # 1 hit
grep -n '/opt/homebrew/Cellar/' docs/packaging.md                         # 1 hit (in the new section)
grep -n '/usr/local/Cellar/' docs/packaging.md                            # 1 hit (in the new section)
grep -n '/home/linuxbrew/.linuxbrew/Cellar/' docs/packaging.md            # 1 hit (BUG-007 root — the key addition)
grep -n 'brew upgrade stagecoach' docs/packaging.md                       # 1 hit (the delegation command)

# Guard 2: brew is documented as DELEGATE (run), NOT print (the accuracy core).
grep -n 'delegates.*brew upgrade stagecoach\|brew upgrade stagecoach.*run directly' docs/packaging.md  # ≥1 hit
grep -n 'prints.*brew upgrade\|brew upgrade.*prints' docs/packaging.md && echo "FAIL: brew mis-stated as PRINT" || echo "OK: brew = delegate (not print)"

# Guard 3: the Linuxbrew platform is named (BUG-007 reflected).
grep -n 'Linuxbrew' docs/packaging.md                                     # ≥1 hit (was 0 before)

# Guard 4: the section mirrors the Chocolatey behavior-bullet pattern.
grep -n '\*\*`stagecoach upgrade` behavior\*\*' docs/packaging.md         # 2 hits now (Chocolatey + Homebrew)

# Guard 5: NO internal BUG-ID leaked into the shipped doc (match Chocolatey's style).
grep -n 'BUG-007\|BUG-004\|BUG-008' docs/packaging.md && echo "FAIL: internal bug-id in shipped doc" || echo "OK: no bug-ids in shipped text"

# Guard 6: README + BUG-004/BUG-008 NO-OP — re-confirm no stale claim (sanity).
grep -niE 'query timeout|WaitDelay|releases url|repo.*escape' README.md docs/packaging.md && echo "check: possible PM-timeout/URL doc surface" || echo "OK: no PM-timeout/URL doc surface (BUG-004/008 no-op)"
grep -n 'macOS / Linuxbrew' README.md                                     # 1 hit (README already accurate — no edit)

# Guard 7: scope — only docs/packaging.md.
git status --porcelain
# Expect: docs/packaging.md ONLY.
git diff --name-only | grep -vE '^docs/packaging\.md$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean (byte-for-byte unaffected — doc-only)
- [ ] `make lint` zero errors (markdown + go)
- [ ] markdownlint clean on docs/packaging.md (or make lint covers it)
- [ ] `mkdocs build --strict` clean (the new ## section doesn't break the docs site)
- [ ] `make test` green + UNCHANGED (no code touched)

### Feature Validation
- [ ] docs/packaging.md has `## Homebrew / Linuxbrew` after the intro, before `## Chocolatey` (grep 1)
- [ ] The section names all 3 Cellar roots incl. `/home/linuxbrew/.linuxbrew/Cellar/` (grep 1, guard 3)
- [ ] The section states brew DELEGATES to `brew upgrade stagecoach` (run directly, NOT printed) +
      never self-swapped (FR-U1/FR-U4) (grep guard 2)
- [ ] The section mirrors the Chocolatey `**\`stagecoach upgrade\` behavior**:` bullet (grep guard 4)
- [ ] README re-confirmed accurate (L90 "macOS / Linuxbrew") — ZERO edits (grep guard 6)
- [ ] BUG-004 + BUG-008: ZERO doc edits (confirmed no PM-timeout/URL doc surface) (grep guard 6)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY docs/packaging.md (grep guard 7)
- [ ] NO edit to README.md, cli.md, spec/, internal/upgrade/* (parallel code owners), any PRD/task file
- [ ] NO internal BUG-007/BUG-004/BUG-008 id in the shipped text (grep guard 5)

### Code Quality & Docs
- [ ] The new section is accurate to the code (pathHeuristics detect.go:372-376; delegate_test.go:60;
      printCommand delegate.go:344 — brew is delegate, not print)
- [ ] Heading level `##` (peer to Chocolatey); markdown/mkdocs clean
- [ ] Cites FR-U1/FR-U4 (matching Chocolatey's style); no internal jargon

---

## Anti-Patterns to Avoid

- ❌ Don't edit README.md. It is accurate (L90 "macOS / Linuxbrew"; L161 brew=delegate, AUR/Nix=print).
  The deliverable for README is the RECORDED REVIEW, not an edit. "Improving" it risks drifting from the
  spec.
- ❌ Don't say brew "prints" `brew upgrade stagecoach`. ChannelBrew is ABSENT from printCommand
  (delegate.go:344) — it is a DELEGATE channel (RUN directly, delegate_test.go:60). Chocolatey is the
  PRINT contrast (it prints because `choco upgrade` needs admin). Stating brew prints is the exact
  inaccuracy this task prevents.
- ❌ Don't omit the Linuxbrew Cellar root. BUG-007's whole point is `/home/linuxbrew/.linuxbrew/Cellar/`
  detection. The section MUST name all 3 roots (detect.go:372-376); dropping Linuxbrew fails to reflect
  the fix.
- ❌ Don't edit cli.md. The contract names packaging.md + README.md only. cli.md's `#upgrade` section is
  a flag reference (no detection-cascade description) and was not flagged — leave it.
- ❌ Don't reference internal BUG-007/BUG-004/BUG-008 ids in packaging.md. It is a shipped maintainer
  doc; the Chocolatey block cites FR-U1/FR-U4 with no bug ids — match that.
- ❌ Don't use `###` for the new section. Channel sections are `##` (Chocolatey, Nix, Documentation site).
  A level shift would reorder the mkdocs nav. Use `## Homebrew / Linuxbrew`.
- ❌ Don't insert the section inside Chocolatey or after Nix. Place it after the intro paragraph, before
  `## Chocolatey` (groups the PM channels at the top; matches the intro's Homebrew-first mention order).
- ❌ Don't add a huge section. Mirror Chocolatey's compactness: one intro paragraph + one behavior bullet.
  No goreleaser secret/bootstrap content (brew is a tap, already covered by the intro's release.yml note).
- ❌ Don't edit `internal/upgrade/*` (releases.go is the parallel P1.M3.T3.S1; detect.go/delegate.go are
  BUG-004/BUG-007 code owners). This item is docs/packaging.md ONLY.
- ❌ Don't add tests. This is DOC-ONLY. The brew detection/delegation the doc describes is already covered
  by detect_test.go + delegate_test.go (unchanged).