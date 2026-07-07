---
name: "P1.M4.T1.S1 (project rename Layer 5.1, Mode A) — Rename README.md: stagehand → stagecoach (title, hero, badges, install, examples, FAQ)"
description: |

  The documentation rename of the project's primary user-facing surface, README.md, as the final layer of
  the stagehand→stagecoach project rename (plan 012). All Go source, config surfaces, and build/CI have
  already been renamed (M1–M3); the README still says "Stagehand"/"stagehand" (82 occurrences). Apply a
  global case-variant sed, then VERIFY every URL/install/module-path references the canonical
  `dustin/stagecoach` namespace (NOT `dabstractor`).

  CONTRACT (item_description §3): `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g;
  s/STAGEHAND/STAGECOACH/g' README.md`, then manually review (a) badge URL, (b) brew, (c) go install module
  path, (d) .stagecoach.toml, (e) STAGECOACH_ env.

  ⚠️ **#1 — THE NAMESPACE IS `dustin/stagecoach`, NOT `dabstractor` (the contract item (a) is a TRAP).**
     Contract item (a) says the badge URL must be `github.com/dabstractor/stagecoach/...`. This is WRONG.
     Verified against FOUR sources — go.mod (`module github.com/dustin/stagecoach`), .goreleaser.yaml
     (`owner: dustin`, overriding the dabstractor git remote; L5-8 document the decision), rename_surface_map
     §5.1 (`dustin/stagehand` → `dustin/stagecoach`), and PRD §21.3 (`dustin/stagecoach`). The README already
     uses `dustin/stagehand` and contains ZERO `dabstractor`. The sed ALREADY produces `dustin/stagecoach`.
     **Do NOT change `dustin` to `dabstractor`** — the .goreleaser explicitly rejects that namespace.
     (research §1)

  DELIVERABLE (1 file modified; nothing else): MODIFY `README.md` — global case-variant sed + verification.
    ~82 occurrences (title, hero, badge URL, features table, install commands, CLI examples, FAQ) all become
    stagecoach/Stagecoach/STAGECOACH. Badge/install/go-install reference `dustin/stagecoach`.

  SCOPE NOTE (the sed is complete + safe, design §2): the README is a clean marketing surface — there is NO
    "formerly stagehand" note to preserve (the rename-history note is PRD h2.30, not the README). "stagehand"
    never appears as a substring of another word, and no historical/external reference must be kept. So a
    global sed is correct: after it, `grep -ri stagehand README.md` must return ZERO hits (the Layer-5 gate).

  SCOPE NOTE (the "manual review" items are VERIFICATION, not extra edits, design §3): because the README
    already uses `dustin/stagehand`, the sed produces the correct `dustin/stagecoach` for every item (a)-(e).
    The review is a CHECK that the output matches go.mod + .goreleaser + PRD §21.3 — not additional edits
    (except: do NOT apply the contract's wrong `dabstractor` guidance).

  SCOPE BOUNDARY (what this does NOT do): NO edits to docs/*.md (P1.M4.T1.S2), providers/*.toml
    (P1.M4.T2.S1), FUTURE_SPEC.md (P1.M4.T2.S2), plan/ artifacts (P1.M5.T1.S1), or any .go/.yaml/.yml/
    Makefile/go.mod file. This is ONE file: README.md.

  INPUT (upstream — already renamed): go.mod (`github.com/dustin/stagecoach`), the renamed `cmd/stagecoach`
    dir, the renamed Go integration marker (`stagecoach-integration`, internal/cmd/integrate_lazygit.go:20),
    .goreleaser (`owner: dustin`). OUTPUT: README.md uses Stagecoach/stagecoach throughout; badge URLs and
    install commands reference `dustin/stagecoach`; `grep -ri stagehand README.md` = 0 hits.

  ⚠️ Use `dustin/stagecoach` everywhere (NOT dabstractor). The sed already does this — don't "fix" it.
  ⚠️ Verify the go-install path == go.mod module path (`github.com/dustin/stagecoach/cmd/stagecoach`).
  ⚠️ Verify the integration marker `# stagecoach-integration` matches the Go constant (M1.T2, LANDED).

  Deliverable: 1 modified file (README.md); `grep -ri stagehand README.md` = 0; `go build ./... &&
  go test ./...` green & unchanged (no code touched).

---

## Goal

**Feature Goal**: Rename the project's primary user-facing surface (README.md) from stagehand to stagecoach
— title, hero pitch, CI badge, install instructions (brew/scoop/go install/curl|sh), build-from-source, all
CLI examples, the features table, the lazygit/git-alias integration example, and the FAQ — so the README
presents a unified `dustin/stagecoach` identity consistent with the already-renamed go.mod, .goreleaser, and
Go source. Close Layer 5.1 of the project rename.

**Deliverable** (1 file modified; nothing else):
- `README.md` — global case-variant sed (stagehand→stagecoach, Stagehand→Stagecoach, STAGEHAND→STAGECOACH)
  across all ~82 occurrences, then verification that every badge/install/module-path uses the canonical
  `dustin/stagecoach` namespace.

**Success Definition**: `grep -ri stagehand README.md` returns ZERO hits; the title is `# Stagecoach`; the
badge URL is `https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg`; install commands are
`brew install dustin/tap/stagecoach`, `scoop install dustin/stagecoach`,
`go install github.com/dustin/stagecoach/cmd/stagecoach@latest`; the go-install path matches go.mod; the
lazygit marker reads `# stagecoach-integration` (matches the Go constant); `go build ./... && go test ./...`
green & unchanged; `git status` shows ONLY README.md.

## User Persona

**Target User**: The prospective user/evaluator reading the GitHub repo's front page (README.md is the
marketing surface, PRD §21.5). Post-rename they must see a coherent "Stagecoach" identity with install
commands that actually work against the `dustin/stagecoach` repo and a binary called `stagecoach`.

**Use Case**: A visitor lands on the repo, reads the hero pitch, copies an install command, and runs
`stagehand --version` (→ `stagecoach --version`) — every name, path, and URL must agree.

**User Journey**: user reads README → `# Stagecoach` title + hero → Features table (`stagecoach …`) →
Install (`brew install dustin/tap/stagecoach`) → Quick start (`stagehand` → `stagecoach`) → FAQ. No stale
"stagehand" anywhere; no broken `dabstractor` URLs.

**Pain Points Addressed**: A half-renamed README (Stagecoach prose but `stagehand` commands / `dustin/stagehand`
clone URLs) would confuse users and break copy-pasted install commands. This task makes the surface whole.

## Why

- **It IS Layer 5.1 of the project rename.** M1–M3 renamed the code, config, and build/CI; the README is the
  last user-facing surface still saying "stagehand." rename_surface_map §5.1 names it explicitly.
- **The README is the marketing surface (PRD §21.5).** It's the first thing a user/collaborator sees; a stale
  name undermines the whole rename.
- **Install-command correctness.** The badge URL, brew tap, scoop bucket, go-install path, and clone URL must
  all point at `dustin/stagecoach` or copy-pasted installs fail.
- **Trivial, isolated, no-risk.** One markdown file; global sed; no code, no tests, no other docs.

## What

A global case-variant sed over README.md (the one file in scope), followed by a verification pass that every
URL/path/command references `dustin/stagecoach` (not `dabstractor`) and matches go.mod + the renamed cmd dir
+ the renamed Go integration marker. No code, no tests, no other files.

### Success Criteria

- [ ] `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' README.md`
      applied.
- [ ] `grep -ri stagehand README.md` → ZERO hits (the Layer-5 verification gate).
- [ ] Title is `# Stagecoach`; hero pitch says "Stagecoach".
- [ ] Badge URL is `https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg` (NOT
      dabstractor).
- [ ] Install: `brew install dustin/tap/stagecoach`; `scoop install dustin/stagecoach`;
      `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`;
      `curl -fsSL https://github.com/dustin/stagecoach/raw/main/install.sh | bash`.
- [ ] The go-install path `github.com/dustin/stagecoach/cmd/stagecoach` matches go.mod
      (`github.com/dustin/stagecoach`) + the renamed `cmd/stagecoach` dir.
- [ ] Build-from-source: `git clone https://github.com/dustin/stagecoach.git`, `cd stagecoach`,
      `stagecoach --version`, the symlink line uses `stagecoach`.
- [ ] Config surfaces: `.stagecoachignore`, `.stagecoach.toml`, `git config stagecoach.provider`,
      `STAGECOACH_*` env vars.
- [ ] The lazygit integration example's marker is `# stagecoach-integration` and description
      `'stagecoach: AI commit'` (matches the Go constant `lazygitMarker` in integrate_lazygit.go:20).
- [ ] `git status` shows ONLY README.md modified; NO other file touched; `go build ./... && go test ./...`
      GREEN & unchanged.

## All Needed Context

### Context Completeness Check

_Pass._ A technical writer with no prior repo knowledge can implement this from: the verbatim sed command,
the §1 namespace decision (`dustin` NOT `dabstractor`), the post-sed verification checklist (each item with
its expected value + the source of truth), and the LEAVE list. No Go/hooks/generate knowledge required —
this is a global text replacement + verification.

### Documentation & References

```yaml
# MUST READ — the AUTHORITATIVE design decisions (the namespace trap + the verification plan)
- docfile: plan/012_963e3918ec08/P1M4T1S1/research/design-decisions.md
  why: the 6 decisions. §0 (scope: README only; global sed; no historical-preservation exceptions), §1 (THE
       namespace = dustin/stagecoach, NOT dabstractor — the contract item (a) trap; 4 sources), §2 (the sed
       is complete + safe — 82 occs, disjoint case variants, coverage map), §3 (review items are
       VERIFICATION not edits), §4 (integration marker must match Go constant stagecoach-integration), §5
       (no anchor-link breakage), §6 (no conflict with parallel P1.M3.T1.S3).
  critical: §1 (do NOT use dabstractor — the sed already yields the correct dustin/stagecoach; changing to
       dabstractor would break install/badge against the .goreleaser decision), §4 (the marker string is a
       cross-surface contract with the Go code).

# MUST READ — the file being edited (the surface to rename)
- file: README.md   (EDIT; the ONLY file)
  section: title L1 (`# Stagehand`); hero L3; badge L10 (`github.com/dustin/stagehand/actions/…`); features
           table L59-74 (`.stagehandignore`, `stagehand hook install`, `git stagehand`, `stagecoach-integration`);
           install L88-116 (brew/scoop/go install/curl|sh + build-from-source clone/cd/version/symlink);
           examples L117-210 (every `stagehand …` invocation, `.stagehand.toml`, `git config stagehand.*`);
           FAQ L211+ (`stagehand providers list`, etc.).
  why: the EXACT surface the sed transforms. ~82 occurrences across all these sections.
  critical: the sed is global — it covers every occurrence. The post-sed review checks the high-stakes lines
       (badge, install, go-install, marker) against the sources of truth below.

# MUST READ — the module path source of truth (the go-install path MUST match this)
- file: go.mod   (READ ONLY — do NOT edit)
  section: line 1 — `module github.com/dustin/stagecoach`.
  why: the `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` line in README MUST match this
       module path + the renamed `cmd/stagecoach` dir. The sed produces it from the old
       `github.com/dustin/stagehand/cmd/stagehand` form; VERIFY they agree.
  critical: the owner is `dustin` (NOT dabstractor). Do NOT edit go.mod.

# MUST READ — the namespace decision source of truth (badge/brew/scoop URLs MUST match this)
- file: .goreleaser.yaml   (READ ONLY — do NOT edit)
  section: L5-8 (the namespace note: "uses dustin/stagecoach … the current git remote is dabstractor; before
           the first REAL tag the repo must be reachable at github.com/dustin/stagecoach"); L68 (`owner: dustin`,
           overrides dabstractor); L76-80 (dustin/homebrew-tap); L88-92 (dustin/scoop-bucket); L95
           (`github.com/dustin/stagecoach/releases/…`).
  why: the canonical public namespace for EVERY user-facing URL (badge, brew, scoop, clone, releases) is
       `dustin/stagecoach`. The .goreleaser EXPLICITLY overrides the dabstractor git remote to enforce this.
  critical: do NOT use dabstractor in the README. The sed already yields dustin/stagecoach.

# MUST READ — the surface-map research (the rename plan for this layer)
- docfile: plan/012_963e3918ec08/architecture/rename_surface_map.md
  section: §5.1 README.md — "Badge URL: `github.com/dustin/stagehand/actions/...` → `stagecoach`"; the
           Layer-5 verification gate "grep -ri stagehand returns ZERO hits."
  why: confirms (a) README is Layer 5.1, (b) the badge owner stays `dustin`, (c) the zero-hits gate.

# MUST READ — the integration-marker Go constant (the README example MUST match this string)
- file: internal/cmd/integrate_lazygit.go   (READ ONLY — do NOT edit)
  section: L20 `lazygitMarker = "stagecoach-integration"`; L28 `var entryTpl = \'- key: \'%s\' #
           stagecoach-integration\'`.
  why: the README's lazygit YAML example shows `# stagecoach-integration` as the idempotency marker. That
       string MUST match the Go constant (M1.T2.S1 already renamed it) or `stagecoach integrate uninstall`
       couldn't locate its own entry. The sed produces `stagecoach-integration` from `stagehand-integration`.
  critical: the README marker is `# stagecoach-integration` (post-sed) — verify it matches L20. Do NOT edit
       the Go file.

# Confirms the parallel task is CI/.gitignore (no README overlap) + the same namespace
- docfile: plan/012_963e3918ec08/P1M3T1S3/PRP.md
  section: header — renames `.github/workflows/ci.yml` + `.gitignore` (Layer 4.3-4.4). It does NOT touch
           README.md. Its #1 note uses the same global two-branch sed and confirms the `dustin/stagecoach`
           namespace.
  why: confirms no parallel-edit conflict; the README badge references `actions/workflows/ci.yml/badge.svg`
       (the workflow whose CONTENT S3 renames; the filename stays ci.yml, so the badge keeps resolving).

# The install-paths source of truth (PRD §21.3)
- url: (PRD §21.3 — in your context as selected_prd_content `h3.100`)
  why: the authoritative install commands: `brew install dustin/tap/stagecoach`,
       `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`,
       `scoop install dustin/stagecoach`, `curl … github.com/dustin/stagecoach/raw/main/install.sh`.
  critical: all use `dustin/stagecoach` — the README must match (the sed produces this).
```

### Current Codebase tree (relevant slice)

```bash
README.md               # *** EDIT *** — the ONLY file. ~82 stagehand/Stagehand/STAGEHAND → stagecoach.
go.mod                  # READ ONLY — `module github.com/dustin/stagecoach` (the go-install source of truth).
.goreleaser.yaml        # READ ONLY — `owner: dustin` (the badge/brew/scoop namespace source of truth).
internal/cmd/integrate_lazygit.go  # READ ONLY — `lazygitMarker = "stagecoach-integration"` (M1.T2, LANDED).
docs/*.md               # NOT this task (P1.M4.T1.S2).
providers/*.toml        # NOT this task (P1.M4.T2.S1).
FUTURE_SPEC.md          # NOT this task (P1.M4.T2.S2).
.github/workflows/ci.yml, .gitignore  # NOT this task (parallel P1.M3.T1.S3).
# NO .go / Makefile / go.mod edits. Mode A (docs only).
```

### Desired Codebase tree with files to be added/changed

```bash
# NO new files. ONE in-place edit: README.md (global sed + verification).
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (THE namespace is `dustin`, NOT `dabstractor`, design §1): the contract item (a) says the badge
     URL must be `github.com/dabstractor/stagecoach/...` — this is WRONG. go.mod, .goreleaser (owner: dustin,
     overriding the dabstractor remote), surface_map §5.1, and PRD §21.3 all say `dustin/stagecoach`. The
     README already uses `dustin/stagehand` (ZERO dabstractor), so the sed ALREADY yields `dustin/stagecoach`.
     Do NOT change `dustin` to `dabstractor` — that would point badges/install at a namespace .goreleaser
     explicitly rejects. -->

<!-- CRITICAL (the go-install path must match go.mod, design §3): after sed, the line must read
     `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` — module path `github.com/dustin/stagecoach`
     (matches go.mod L1) + cmd dir `cmd/stagecoach` (renamed M1.T1.S2). VERIFY; don't leave a stale segment. -->

<!-- CRITICAL (the integration marker is a cross-surface contract, design §4): the README's lazygit example
     must show `# stagecoach-integration` (the idempotency marker). That string MUST equal the Go constant
     `lazygitMarker` (integrate_lazygit.go:20 = "stagecoach-integration", M1.T2 LANDED). The sed produces it;
     VERIFY they match — a mismatch breaks `stagecoach integrate uninstall`. Similarly `git stagehand` →
     `git stagecoach` matches the renamed `alias.stagecoach` registration. -->

<!-- GOTCHA (the badge workflow filename stays ci.yml): the badge URL path is `actions/workflows/ci.yml/badge.svg`.
     P1.M3.T1.S3 renames the CONTENT of ci.yml, not its filename, so the badge keeps resolving. Don't "rename"
     ci.yml to stagecoach.yml in the URL. -->

<!-- GOTCHA (no historical-preservation exceptions): the README is a clean marketing surface. There is NO
     "formerly stagehand" / "originally stagehand" sentence to preserve (that note is PRD h2.30, not the
     README). Every "stagehand" becomes "stagecoach" — a global sed is correct. After it, grep must be 0. -->

<!-- GOTCHA (case-variant sed order is safe): s/stagehand/stagecoach/g, then s/Stagehand/Stagecoach/g, then
     s/STAGEHAND/STAGECOACH/g — the three patterns are case-disjoint, so no double-substitution. Check for
     any mixed-case stragglers (stageHand/StageHand) post-sed (none expected). -->

<!-- GOTCHA (anchor links don't break): README docs anchors (#multi-turn-generation-fallback,
     #trade-off-inversion-fr-h7, #commit-hooks-on-the-plumbing-path, #integrate-install-target, etc.) contain
     NO "stagehand", so the sed leaves them and P1.M4.T1.S2's docs sed won't break them. VERIFY no README
     anchor contains "stagehand" post-sed (none do). -->
```

## Implementation Blueprint

### Data models and structure

```markdown
<!-- NO data models. A global-text-replace + verification task. The "structure" is the sed + the checklist. -->
```

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: APPLY the global case-variant sed to README.md
  - RUN: sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' README.md
  - This transforms ~82 occurrences (title, hero, badge, features, install, examples, FAQ) in one pass.
  - GOTCHA: run from the repo root (the module dir). The sed is idempotent-ish (re-running is a no-op once
      renamed). Case variants are disjoint — no double-substitution.

Task 2: VERIFY the namespace — `dustin/stagecoach` everywhere, ZERO `dabstractor`
  - RUN: `grep -nE 'dabstractor' README.md` → expect ZERO hits. (The README never had dabstractor; the sed
      must not introduce it. The contract item (a) "use dabstractor" is a TRAP — do NOT act on it.)
  - RUN: `grep -nE 'github.com/dustin/stagecoach|dustin/tap/stagecoach|dustin/stagecoach' README.md` →
      confirm the badge, clone, brew, scoop, go-install, curl|sh, releases URLs all reference dustin/stagecoach.
  - SOURCE OF TRUTH: go.mod (`github.com/dustin/stagecoach`) + .goreleaser.yaml (`owner: dustin`).

Task 3: VERIFY the high-stakes lines (badge, install, go-install, marker) match the renamed code
  - BADGE (was L10): `![CI](https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg)` —
      owner dustin; workflow filename stays ci.yml (P1.M3.T1.S3 renames content, not name).
  - INSTALL (were L113-116): `brew install dustin/tap/stagecoach`; `scoop install dustin/stagecoach`;
      `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`;
      `curl -fsSL https://github.com/dustin/stagecoach/raw/main/install.sh | bash`.
  - GO-INSTALL PATH: `github.com/dustin/stagecoach/cmd/stagecoach` — VERIFY module path matches go.mod L1
      (`github.com/dustin/stagecoach`) and the cmd dir is `cmd/stagecoach` (renamed M1.T1.S2).
  - BUILD-FROM-SOURCE (were L95-107): `git clone https://github.com/dustin/stagecoach.git`; `cd stagecoach`;
      `stagecoach --version`; symlink `ln -s "$(go env GOPATH)/bin/stagecoach" ~/.local/bin/stagecoach`.
  - INTEGRATION MARKER (was L199): `# stagecoach-integration` — VERIFY it equals the Go constant
      `lazygitMarker` (internal/cmd/integrate_lazygit.go:20 = "stagecoach-integration"). And description
      (was L204): `'stagecoach: AI commit'`. And the alias (L72, L185): `git stagecoach`.
  - CONFIG SURFACES: `.stagecoachignore`, `.stagecoach.toml`, `git config stagecoach.provider`,
      `STAGECOACH_*` env.

Task 4: VERIFY the zero-hits gate + no regressions
  - RUN: `grep -ri stagehand README.md` → MUST return ZERO hits (the Layer-5 gate). If any hit remains,
      inspect (a missed case variant or an exception) and fix.
  - RUN: `grep -nE '#[a-z0-9-]*stagehand' README.md` → expect ZERO (no docs anchor contains stagehand; the
      sed shouldn't have touched anchors anyway).
  - RUN: `go build ./... && go test ./...` → GREEN & unchanged (no code touched — this is a README-only edit;
      the build/test is a no-op regression check).
  - RUN: `git status --porcelain` → ONLY README.md modified.

Task 5: VERIFY scope (only README.md changed)
  - `git status` → exactly ONE modified file: README.md.
  - `git diff --exit-code docs/ providers/ FUTURE_SPEC.md .github/workflows/ci.yml .gitignore go.mod
      .goreleaser.yaml Makefile` → all unchanged.
```

### Implementation Patterns & Key Details

```markdown
<!-- THE sed (one global pass, three disjoint case variants):
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' README.md

<!-- THE namespace rule (the one trap): `dustin/stagecoach`, NOT `dabstractor`. The sed already yields
     dustin/stagecoach (README used dustin/stagehand). Do NOT change the owner. Sources: go.mod, .goreleaser
     (owner: dustin), surface_map §5.1, PRD §21.3.

<!-- THE cross-surface consistency checks (the README must match the already-renamed CODE):
     - go-install path == go.mod module path (github.com/dustin/stagecoach) + cmd/stagecoach dir.
     - lazygit marker `# stagecoach-integration` == Go lazygitMarker (integrate_lazygit.go:20).
     - git alias `git stagecoach` == the renamed alias.stagecoach registration.
     - badge URL workflow filename stays ci.yml (P1.M3.T1.S3 renames content, not name).

<!-- THE zero-hits gate: `grep -ri stagehand README.md` → 0. This is the Layer-5 acceptance check.
```

### Integration Points

```yaml
DOCS.FILE (the ONLY edit): README.md (global sed + verification).

LEFT-UNCHANGED (do NOT edit — other layers' scope):
  - docs/*.md            # P1.M4.T1.S2 (Layer 5.2).
  - providers/*.toml     # P1.M4.T2.S1 (Layer 5.3).
  - FUTURE_SPEC.md       # P1.M4.T2.S2 (Layer 5.4).
  - plan/ artifacts      # P1.M5.T1.S1 (historical).
  - .github/workflows/ci.yml, .gitignore  # parallel P1.M3.T1.S3 (Layer 4.3-4.4).
  - go.mod, .goreleaser.yaml, Makefile, every .go file  # already renamed (M1-M3).

CODE.LEFT-UNCHANGED: NO .go / Makefile / go.mod / .goreleaser / CI changes (Mode A — docs only). The build
  is a no-op regression check.

CROSS-SURFACE CONSISTENCY (the README must AGREE with the already-renamed code, not re-rename it):
  - go-install path == go.mod (`github.com/dustin/stagecoach`) + `cmd/stagecoach`.
  - lazygit marker == `lazygitMarker` (`stagecoach-integration`, integrate_lazygit.go:20).
  - badge URL owner == .goreleaser `owner: dustin`.
  - badge URL workflow == `ci.yml` (filename unchanged by P1.M3.T1.S3).

DOWNSTREAM: P1.M5.T2.S1 (final grep audit) will include README.md in its zero-hits sweep — this task lands
  README's zero-hits ahead of that audit.
```

## Validation Loop

### Level 1: The sed landed + zero-hits gate

```bash
sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' README.md
grep -ric stagehand README.md    # EXPECT: 0  (the Layer-5 gate)
grep -nE 'dabstractor' README.md # EXPECT: no match (do NOT use dabstractor)
head -3 README.md                 # EXPECT: "# Stagecoach" title + Stagecoach hero
# Expected: 0 stagehand hits; 0 dabstractor; title is Stagecoach.
```

### Level 2: High-stakes line verification (namespace + cross-surface consistency)

```bash
# Badge (owner dustin, workflow ci.yml):
grep -n 'github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg' README.md
# Install commands (all dustin/stagecoach):
grep -n 'brew install dustin/tap/stagecoach' README.md
grep -n 'scoop install dustin/stagecoach' README.md
grep -n 'go install github.com/dustin/stagecoach/cmd/stagecoach@latest' README.md
grep -n 'github.com/dustin/stagecoach/raw/main/install.sh' README.md
# go-install path matches go.mod:
test "$(head -1 go.mod)" = "module github.com/dustin/stagecoach" && echo "go.mod matches the README go-install path"
# Integration marker matches the Go constant:
grep -n '# stagecoach-integration' README.md
grep -n 'stagecoach-integration' internal/cmd/integrate_lazygit.go   # the Go side (read-only confirm)
# Config surfaces renamed:
grep -nE '\.stagecoachignore|\.stagecoach\.toml|git config stagecoach\.|STAGECOACH_' README.md | head
# Expected: every grep hits; go.mod matches; the README marker == the Go constant.
```

### Level 3: Scope check + regression no-op (no code touched)

```bash
go build ./...   # Expect clean & unchanged (no code touched).
go test ./...    # Expect GREEN & unchanged (docs-only).
git status --porcelain
# Expected: exactly ONE modified file — README.md. NOTHING else.
git diff --exit-code docs/ providers/ FUTURE_SPEC.md .github/workflows/ .gitignore go.mod .goreleaser.yaml Makefile \
  && echo "all other files UNCHANGED (expected)"
! git diff --name-only | grep -vE '^README\.md$' && echo "OK: only README.md modified"
```

### Level 4: The namespace audit (the contract-trap proof)

```bash
# Prove the README uses the canonical dustin/stagecoach namespace (NOT dabstractor) — the contract item (a)
# trap. The .goreleaser explicitly overrides the dabstractor git remote with owner: dustin:
grep -n 'owner: dustin' .goreleaser.yaml           # the namespace decision (read-only confirm)
grep -nE 'dustin/stagecoach' README.md | wc -l     # EXPECT: several (badge + clone + install + releases)
grep -nE 'dabstractor' README.md || echo "OK: zero dabstractor in README (correct — dustin is canonical)"
# Expected: .goreleaser owner=dustin; README has multiple dustin/stagecoach refs; ZERO dabstractor.
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` clean & unchanged; `go test ./...` GREEN & unchanged (docs-only — no code touched).
- [ ] `git status` shows EXACTLY ONE modified file: README.md. No other file touched.
- [ ] go.mod / .goreleaser.yaml / Makefile / .github/ / docs/ / providers/ / FUTURE_SPEC.md byte-unchanged.

### Feature Validation
- [ ] `grep -ri stagehand README.md` → ZERO hits (the Layer-5 gate).
- [ ] Title `# Stagecoach`; hero "Stagecoach"; all CLI examples `stagecoach …`.
- [ ] Badge URL `https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg` (NOT dabstractor).
- [ ] Install: `brew install dustin/tap/stagecoach`; `scoop install dustin/stagecoach`;
      `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`; curl|sh → `dustin/stagecoach`.
- [ ] go-install path matches go.mod (`github.com/dustin/stagecoach`) + `cmd/stagecoach`.
- [ ] `.stagecoachignore`, `.stagecoach.toml`, `git config stagecoach.*`, `STAGECOACH_*` throughout.
- [ ] lazygit marker `# stagecoach-integration` == Go `lazygitMarker` (integrate_lazygit.go:20).
- [ ] ZERO `dabstractor` in README (the namespace is `dustin`).

### Code Quality Validation
- [ ] The rename is global + consistent (no stragglers, no mixed case variants left).
- [ ] No anchor links broken (no README docs-anchor contains stagehand).
- [ ] Anti-patterns avoided (see below); no out-of-scope churn (docs/providers/CI/code frozen).

### Documentation
- [ ] [Mode A] the README presents a unified Stagecoach/`dustin/stagecoach` identity; install commands work
      against the canonical repo; the integration example matches the shipped Go marker. (This IS the docs
      update for the project's primary user-facing surface.)

---

## Anti-Patterns to Avoid

- ❌ **Don't use `dabstractor` for the badge/owner.** The contract item (a) is a TRAP. The canonical
  namespace is `dustin/stagecoach` (go.mod, .goreleaser `owner: dustin`, surface_map §5.1, PRD §21.3). The
  README already used `dustin/stagehand`; the sed yields `dustin/stagecoach`. Do NOT change `dustin` to
  `dabstractor` — .goreleaser explicitly overrides that remote. (§1)
- ❌ **Don't "rename" the workflow filename in the badge URL.** The badge path is
  `actions/workflows/ci.yml/badge.svg`; P1.M3.T1.S3 renames ci.yml's CONTENT, not its name. Leave `ci.yml`.
  (gotcha)
- ❌ **Don't mismatch the go-install path.** It must be `github.com/dustin/stagecoach/cmd/stagecoach` —
  matching go.mod's module path AND the renamed `cmd/stagecoach` dir. Verify post-sed; don't leave a stale
  `cmd/stagehand` segment. (§3)
- ❌ **Don't mismatch the integration marker.** The README's `# stagecoach-integration` must equal the Go
  `lazygitMarker` (integrate_lazygit.go:20). A mismatch breaks `stagecoach integrate uninstall`'s idempotency
  check. (§4)
- ❌ **Don't preserve a "formerly stagehand" note.** The README is a clean marketing surface; the
  rename-history note is PRD h2.30, not the README. Every "stagehand" → "stagecoach" — global sed, no
  exceptions. (§2)
- ❌ **Don't edit any file other than README.md.** docs/*.md is S2; providers/*.toml is P1.M4.T2.S1;
  FUTURE_SPEC.md is P1.M4.T2.S2; CI/.gitignore is the parallel P1.M3.T1.S3; code/build is already done (M1-M3).
  (scope boundary)
- ❌ **Don't run the sed from the wrong directory.** Run from the repo root (the module dir) so the path
  resolves; the plan/ artifacts dir is a different tree (P1.M5.T1.S1's scope).
- ❌ **Don't skip the zero-hits gate.** `grep -ri stagehand README.md` MUST be 0 — that's the Layer-5
  acceptance check and the input P1.M5.T2.S1's final audit expects. (Task 4)
