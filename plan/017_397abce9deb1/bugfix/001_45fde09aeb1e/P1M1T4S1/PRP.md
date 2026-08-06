name: "P1.M1.T4.S1 — Mode B doc sweep: verify README.md / docs/cli.md / docs/configuration.md against the three upgrade fixes"
description: >
  [Mode B — changeset-level documentation sweep.] After the three upgrade fixes are merged
  (P1.M1.T1.S1 BUG-001 sanity-run v-normalization, P1.M1.T2.S1 BUG-002 ~/go default-GOPATH fallback,
  P1.M1.T3.S1 BUG-003 non-semver tag deprioritization), review the THREE user-facing docs for drift:
  README.md (the `### Updating` section + the Install-section `go install` line), docs/cli.md (the
  `### upgrade` command section incl. `--prerelease`), and docs/configuration.md (the `[upgrade]`
  section incl. `channel`). The fixes are pure internal-logic fixes — they change NO CLI flag, config
  key, or API surface, and the per-function source doc comments are already updated via Mode A inside
  each fix subtask. Expected outcome: the docs ALREADY describe the intended (now-restored) behavior;
  the only candidate edit is ONE optional clarifying sentence in README.md noting that a `go install`
  binary under ~/go/bin is auto-detected even when GOPATH is unset (the exact condition BUG-002 fixed).
  A ZERO-EDIT outcome is a fully valid deliverable: record the verification finding (which files were
  touched vs confirmed accurate) in the task result + commit message. Edit ONLY README.md if at all;
  do NOT edit docs/cli.md or docs/configuration.md (confirmed accurate). Do NOT touch PRD.md,
  .gitignore, any source file, Mode A comments, docs/how-it-works.md, or FUTURE_SPEC.md.

---

## Goal

**Feature Goal**: Confirm — and where a single clarifying line materially helps a user, strengthen —
that README.md, docs/cli.md, and docs/configuration.md accurately reflect the behavior RESTORED by the
three v3.0 self-update bugfixes (BUG-001/002/003). None of the fixes changed any CLI/config/API surface;
they fixed internal logic that had made the docs' (already-correct) claims FALSE in practice.

**Deliverable**: A documented doc-accuracy verification of the three named files. Concretely EITHER:
  (A) **zero edits** — the three docs are confirmed accurate post-fix; the task result + commit message
      record the review (which files were checked, what claims were verified, the no-drift finding); OR
  (B) **one edit** — README.md `### Updating` gains a single clarifying sentence noting that a
      `go install` binary under `~/go/bin` is auto-detected even when `GOPATH` is unset (so it delegates
      rather than self-swaps); docs/cli.md and docs/configuration.md remain untouched (confirmed
      accurate). The task result states which file was touched and why.
Either outcome is a complete deliverable. The decision between (A) and (B) follows the rubric in
"Implementation Tasks → Task 2".

**Success Definition**:
- All three docs' upgrade-related claims are verified against each fix (matrix in research notes).
- No doc statement remains that describes the BUGGY behavior (none is expected to exist — confirmed in
  research; the fixes make the docs' existing claims true).
- IF an edit is made: it is the single README.md Updating-section sentence (or an equivalent one-liner),
  applies ONLY to README.md, changes no anchor/cross-reference, and the markdown still renders.
- IF no edit is made: the task result + commit message explicitly say "verified accurate, no drift,
  no edits" and enumerate the three reviewed files.
- `git diff --stat` (when this subtask lands) shows AT MOST README.md among the three target docs
  (and never PRD.md / .gitignore / any source file / tasks.json / prd_snapshot.md).

## User Persona (if applicable)

**Target User**: A developer who installed stagecoach via `go install …@latest` (binary at `~/go/bin`,
`GOPATH` unset — the modern-Go default) and reads the README before running `stagecoach upgrade`.

**Use Case**: The user wants to know whether `stagecoach upgrade` will "just work" for their install
method, or whether they must re-run the install command themselves.

**User Journey**: `go install …@latest` → reads README `### Updating` → runs `stagecoach upgrade` →
(with the BUG-002 fix) stagecoach detects `~/go/bin` and delegates to `go install …@latest`.

**Pain Points Addressed**: BUG-002 made this exact population misroute to a self-swap that (with
BUG-001) failed entirely. The fix restores delegation; the optional note reassures the user the
default-`GOPATH` case is honored.

## Why

- **The fixes are logic fixes, not surface changes.** BUG-001 (sanity-run v-normalization), BUG-002
  (~/go default-GOPATH fallback), and BUG-003 (non-semver tag deprioritization) restore behavior the
  docs already describe. Pre-fix, the docs' claims ("self-swaps only for a direct install"; "delegates
  to … `go install`"; `--prerelease` admits pre-release tags) were FALSE in practice; post-fix they are
  TRUE. So this task is a VERIFICATION, not a rewrite.
- **Mode B, not Mode A.** Per-function source doc comments were already updated inside each fix
  subtask (Mode A). This task is the changeset-level doc sweep across README + the two overview docs.
- **Bounded.** Three files, narrow sections (named with line ranges below). No new docs, no structural
  changes, no CLI/config-key documentation to add (the surface is unchanged).
- **The audit is already done** in `architecture/bug_analysis.md` ("Documentation Surface Check") and
  re-verified in `research/doc_accuracy_audit.md`: all three docs are accurate; the only candidate edit
  is the single go-install ~/go/bin note. This PRP operationalizes that finding.

## What

**User-visible behavior**: none (documentation only). If the optional note is applied, the README
Updating section additionally tells go-install users their install is auto-detected via `~/go/bin`.

**Technical change**: AT MOST one sentence appended to the README.md `### Updating` paragraph (L146).
No change to docs/cli.md or docs/configuration.md. No change to any source file, PRD.md, .gitignore,
Mode A comments, docs/how-it-works.md, or FUTURE_SPEC.md.

### Success Criteria
- [ ] README.md `### Updating` (L144–154) reviewed against BUG-001 + BUG-002; claims verified accurate
- [ ] README.md Install `go install` line (L112–113) + `make install`/`$GOPATH/bin` note (L132–142)
      reviewed against BUG-002 (fixed ~/go detection) — consistent
- [ ] docs/cli.md `### upgrade` (L400–446), incl. `--prerelease` (L412) + `--channel` (L417), reviewed
      against BUG-001 + BUG-003 — accurate (flag behavior unchanged)
- [ ] docs/configuration.md `[upgrade]` template (L121–123) + defaults table (L158) + global-only note
      (L163) reviewed against BUG-003 — accurate
- [ ] No doc statement describes the buggy sanity-run or buggy detection behavior (none found)
- [ ] Decision recorded: applied the single README note (path A) OR no edits (path B), with rationale
- [ ] `git diff` touches AT MOST README.md among the three target docs; never PRD.md / .gitignore /
      any `*.go` / tasks.json / prd_snapshot.md / docs/how-it-works.md / FUTURE_SPEC.md

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the exact files + line ranges to review, the exact claim to verify per fix, the verbatim
current text of every reviewed section, the exact proposed edit (text + placement), the edit-vs-no-edit
rubric, and the hard scope constraints are all enumerated below and in the two research notes
(`research/doc_accuracy_audit.md`, `research/candidate-edit.md`). The three fixes' semantics are
summarized inline so the implementer need not re-derive them; the implementing PRPs are referenced for
depth.

### Documentation & References

```yaml
# MUST READ — the three target docs (the ONLY files this task may edit, and only README.md at that)
- file: README.md
  why: "Target #1. `### Updating` paragraph @L146 (the delegation/self-swap claim — was FALSE pre-fix,
        TRUE post-fix); Install > Package managers `go install` line @L112-113; Build from source
        `make install` @L132 + `$GOPATH/bin (usually ~/go/bin)` @L135 + the ~/.local/bin symlink TIP
        @L142. The `(#updating)` anchor is referenced @L87, L392, L422 — must stay valid."
  pattern: "single dense paragraph @L146 beginning 'Keep stagecoach current with one command …'"
  critical: "the ONLY file an edit may land in. If editing, append ONE sentence after '…manual) install.'
             (see Task 2 exact text). Do NOT restructure the paragraph or touch other sections."

- file: docs/cli.md
  why: "Target #2 (CONFIRM ACCURATE — do NOT edit). `### upgrade` @L400; delegation sentence @L402;
        flag table @L409-418 incl. `--prerelease` 'Admit pre-release tags' @L412 and `--channel
        <stable|prerelease>` @L417; flag-contract rules @L420; examples @L424-429; exit codes @L432."
  pattern: "flag table row: `| --prerelease | Admit pre-release tags (shorthand for --channel
            prerelease) (FR-U10) |`"
  critical: "BUG-003 changes WHICH tag is selected when non-semver tags exist, NOT what --prerelease
             admits — so L412 is still accurate. Do NOT add a 'non-semver tags are skipped' note
             (maintainer noise; flag behavior unchanged)."

- file: docs/configuration.md
  why: "Target #3 (CONFIRM ACCURATE — do NOT edit). `[upgrade]` template block @L121-123 (`channel`,
        `source_repo`); defaults table rows `channel` @L158 + `source_repo` @L159; global-only note
        @L163."
  pattern: "`# channel = \"stable\"  # stable | prerelease (admits -rc/-beta tags; = --prerelease)` @L122"
  critical: "No install-method / non-semver surface here. Confirmed accurate. Do NOT edit."

# DECISION PROVENANCE — the audit conclusions (read these; they pre-empt re-derivation)
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/architecture/bug_analysis.md
  why: "§'Documentation Surface Check' already concluded the three docs are accurate post-fix and named
        the single go-install ~/go/bin note as the only candidate. This task operationalizes it."
  section: "Documentation Surface Check"
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T4S1/research/doc_accuracy_audit.md
  why: "Fix→doc-claim matrix (BUG-001/002/003 × README/cli.md/configuration.md) with per-doc verdict;
        anchor/cross-ref verification; Makefile/go-env context."
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T4S1/research/candidate-edit.md
  why: "The exact proposed README sentence, placement, wording rationale, and the edit-vs-no-edit rubric."

# THE THREE FIXES (semantics summary — enough to verify the docs without re-reading each PRP)
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T1S1/PRP.md
  why: "BUG-001: sanityCheck (stage.go) now accepts the binary's --version output if it contains
        release.Tag OR its v-stripped form. Internal; no CLI/config change. Makes 'self-swaps only for
        a direct install' TRUE for real goreleaser releases."
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T2S1/PRP.md
  why: "BUG-002: detectPath (detect.go) now resolves ~/go (HOME/go) when GOPATH is unset, so a binary
        at ~/go/bin is detected as go-install and upgrade delegates to `go install …@latest`. Makes the
        README/docs/cli.md 'delegates to … go install' claim TRUE for the common case."
- docfile: plan/017_397abce9deb1/bugfix/001_45fde09aeb1e/P1M1T3S1/PRP.md
  why: "BUG-003: LatestAdmittingPrereleases (releases.go) now deprioritizes non-semver tags so a moving
        tag (e.g. nightly) can't win over a valid release. Does NOT change what --prerelease admits;
        --prerelease's user-facing description stays accurate."
```

### Current Codebase tree (relevant slice)

```bash
# The three target docs (this task reviews these; may edit ONLY README.md)
README.md                   # `### Updating` @L144-154; Install `go install` @L112-113; make install @L132-142
docs/cli.md                 # `### upgrade` @L400-446 (--prerelease @L412; --channel @L417)   [CONFIRM, no edit]
docs/configuration.md       # `[upgrade]` template @L121-123; defaults @L158-159; note @L163   [CONFIRM, no edit]

# Out-of-scope (do NOT touch)
docs/how-it-works.md        # only references `upgrade` as the network exception @L197 (consistent)
FUTURE_SPEC.md              # not in scope
PRD.md                      # FORBIDDEN (human-owned)
**/*.go                     # FORBIDDEN — Mode A source comments already done in T1/T2/T3.S1
.gitignore, **/tasks.json, **/prd_snapshot.md   # FORBIDDEN
```

### Desired Codebase tree with files to be added/changed

```bash
README.md                   # AT MOST: +1 sentence in `### Updating` (path A), or unchanged (path B)
docs/cli.md                 # UNCHANGED (confirmed accurate)
docs/configuration.md       # UNCHANGED (confirmed accurate)
# (no new files; no source/PRD/.gitignore/tasks changes)
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (README Updating has a live anchor): the `### Updating` heading @L144 backtargets the
     `(#updating)` links @L87 (Install NOTE), L392 (FAQ network-calls), and L422 (FAQ future-spec).
     If you edit the Updating paragraph, do NOT rename or remove the `### Updating` heading and do NOT
     change its heading text — the auto-generated GitHub anchor `#updating` must stay intact. Append
     only; do not restructure. -->

<!-- CRITICAL (Makefile `make install` interaction is NOT a fix target): `make install` (Makefile
     L58-61) runs `go install` to $(GOBIN) THEN symlinks into ~/.local/bin/stagecoach. A make-install
     user's PATH entry is the symlink. This is a pre-existing Makefile/README relationship and is NOT
     what BUG-002 fixes (BUG-002 is the GOPATH-unset ~/go fallback in detect.go, not symlink
     resolution). Do NOT "fix" the make-install note or claim detection resolves the symlink — that is
     out of scope and unverified. The L132 '$GOPATH/bin' note and L135 'usually ~/go/bin' note are
     accurate as-is. -->

<!-- CRITICAL (do NOT document internal details): BUG-001's sanity-run and BUG-003's non-semver
     selection are INTERNAL logic. No user doc describes them. Do NOT add a "the upgrade verifies the
     binary's --version against the tag" note (BUG-001) or a "non-semver tags are skipped" note
     (BUG-003) — those are implementation details / maintainer concerns and would add noise, not help. -->

<!-- GOTCHA (the contract's "~L113 FAQ" reference is actually the Install section): the item says
     'the FAQ that mentions go install as an install method ~L113'. The actual go-install line @L113
     is in `## Install > ### Package managers` (NOT the FAQ, which starts @L370 and contains no
     'go install'). Treat L112-113 (Install) as the target; also scan the FAQ for any go-install
     mention (confirmed: none). -->

<!-- GOTCHA (a no-edit outcome is VALID): the contract explicitly permits 'If all docs are already
     accurate, record that finding and make no edits'. The audit found them accurate. Choosing path B
     (zero edits) is a complete, correct deliverable as long as the task result + commit message
     document the verification. Do not invent edits to "look productive". -->

<!-- SCOPE: edit ONLY README.md, and ONLY the single Updating sentence, and ONLY if the rubric (Task 2)
     says to. docs/cli.md and docs/configuration.md are CONFIRMED ACCURATE — never edit them here. -->
```

## Implementation Blueprint

### Data models and structure
None. This is a documentation review/optional-edit task. No code, no schemas.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: REVIEW the three target docs against each fix (read-only; record findings)
  - READ README.md L112-113 (Install go install), L132-142 (make install / $GOPATH/bin), L144-154
        (### Updating). Verify:
        (a) L112-113 `go install …@latest` is consistent with fixed ~/go detection (BUG-002). [YES]
        (b) L132 `make install # installs the binary to $GOPATH/bin` + L135 'usually ~/go/bin' are
            consistent with the fixed ~/go detection. [YES — do not edit; see Makefile gotcha]
        (c) L146 Updating: 'delegates to … go install' + 'self-swaps only for a direct install' are
            now TRUE post-fix (BUG-001/002). [YES — no drift; the fixes restore these claims]
        (d) No statement describes the buggy sanity-run or detection. [NONE FOUND]
  - READ docs/cli.md L400-446. Verify `--prerelease` 'Admit pre-release tags' @L412 and `--channel`
        @L417 are accurate post-BUG-003. [YES — BUG-003 changes selection, not what the flag admits]
  - READ docs/configuration.md L121-123, L158-159, L163. Verify `channel` docs accurate. [YES]
  - RECORD the per-section verdict (a one-line table) for the task result. Expected: all ACCURATE.
  - DEPENDENCIES: the three fixes (T1/T2/T3.S1) should be MERGED first (per the contract INPUT) so the
        review reflects shipped behavior and Mode A source comments are already in place. If reviewing
        before merge, treat the PRPs as the contract (the fixes are deterministic).

Task 2: DECIDE edit-vs-no-edit (the judgment call) — and apply AT MOST the single README sentence
  - APPLY this rubric to decide path A (one edit) vs path B (zero edits):
        Path B (zero edits, RECOMMENDED IF the note reads redundant): choose when you judge a reader
          already infers go-install auto-detection from the existing 'delegates to … go install' list
          entry. Record 'verified accurate, no drift, no edits' + enumerate the three reviewed files.
        Path A (one edit, RECOMMENDED IF it materially helps the go-install user): choose when you judge
          the ~/go/bin + GOPATH-unset specificity adds reassurance the bare list entry lacks.
    RATIONALE FOR RECOMMENDING PATH A: the contract itself names this as the example 'materially
    helpful' note; it serves the exact user population BUG-002 affected; ~/go/bin is stable Go
    convention (README L135 already uses the phrase); it is one short sentence, very low risk.
  - IF PATH A, EDIT README.md `### Updating` (L146) — append ONE sentence immediately after the
        clause ending '…self-swaps only for a direct (curl|sh / manual) install.' and before
        'It never overwrites a package-manager-owned file…':
            A `go install` binary under `~/go/bin` is detected automatically even when `GOPATH` is
            unset, so it delegates to `go install …@latest` rather than self-swapping.
    - KEEP the `### Updating` heading text/level byte-identical (the `(#updating)` anchor is live).
    - DO NOT touch any other README section, docs/cli.md, or docs/configuration.md.
  - DEPENDENCIES: Task 1.

Task 3: VALIDATE doc integrity (regardless of path A/B)
  - git diff --stat -- README.md docs/cli.md docs/configuration.md   # path B → README unchanged too;
        path A → only README.md listed. NEVER docs/cli.md or docs/configuration.md here.
  - git diff --stat -- PRD.md .gitignore '**/*.go' docs/how-it-works.md FUTURE_SPEC.md   # EMPTY
  - git diff --stat -- $(find . -name tasks.json -o -name prd_snapshot.md)   # EMPTY
  - ANCHOR CHECK: the `### Updating` heading text is unchanged → the `(#updating)` links @L87/L392/L422
        still resolve. (If you renamed it, FIX by reverting — never rename it.)
  - MARKDOWN RENDER: eyeball the `### Updating` paragraph still reads as one coherent block; no broken
        list/table/code-fence. (Optional: `glow README.md` / GitHub preview if available.)
  - DEPENDENCIES: Task 2.

Task 4: RECORD the finding (this IS part of the deliverable, for BOTH paths)
  - In the task result (and a one-line commit-message body), state: which files were reviewed, the
        per-fix verdict (accurate), and the path chosen (A: 'README.md updated — go-install ~/go/bin
        note; docs/cli.md + docs/configuration.md confirmed accurate' OR B: 'all three docs verified
        accurate, no drift, no edits'). Per the contract OUTPUT, this documents the sweep.
  - DEPENDENCIES: Task 3.
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN — the single appended sentence (path A only), placed mid-paragraph at L146.
     BEFORE:
       …and self-swaps only for a direct (curl|sh / manual) install. It never overwrites a
       package-manager-owned file, so it never fights your package manager. …
     AFTER:
       …and self-swaps only for a direct (curl|sh / manual) install. A `go install` binary under
       `~/go/bin` is detected automatically even when `GOPATH` is unset, so it delegates to
       `go install …@latest` rather than self-swapping. It never overwrites a package-manager-owned
       file, so it never fights your package manager. …
     Why this wording: 'under ~/go/bin' mirrors README L135 ('usually ~/go/bin'); 'even when GOPATH is
     unset' is the exact BUG-002 condition; 'delegates to go install …@latest' mirrors FR-U3 + the
     section's own verb. -->

<!-- PATTERN — the review verdict table to record (for the task result, both paths):
     | File                 | Section                    | vs fix      | Verdict   |
     | README.md            | ### Updating (L146)        | BUG-001/002 | accurate  |
     | README.md            | Install go install (L113)  | BUG-002     | accurate  |
     | README.md            | make install / $GOPATH (L132-142) | BUG-002 | accurate |
     | docs/cli.md          | --prerelease (L412)        | BUG-003     | accurate  |
     | docs/cli.md          | upgrade delegation (L402)  | BUG-001/002 | accurate  |
     | docs/configuration.md| [upgrade] channel (L122/158) | BUG-003   | accurate  |
     (Expected every row = accurate; the fixes restore the documented behavior.) -->
```

### Integration Points

```yaml
NO code / config / API / build / test integration. Documentation only.

DOCS:
  - README.md `### Updating` (L146) — AT MOST one appended sentence (path A); or unchanged (path B)
  - docs/cli.md, docs/configuration.md — CONFIRMED ACCURATE; never edited by this task

ANCHORS (must stay valid):
  - README `### Updating` @L144 ← `(#updating)` links @L87, L392, L422  (do not rename the heading)
  - docs/cli.md `### upgrade` @L400 ← `docs/cli.md#upgrade` links in README @L392  (not edited here)

FORBIDDEN (do NOT touch):
  - PRD.md, .gitignore, **/*.go (Mode A source comments already done in T1/T2/T3.S1),
    docs/how-it-works.md, FUTURE_SPEC.md, **/tasks.json, **/prd_snapshot.md
```

## Validation Loop

> This is a documentation task. There is no `go test`/`ruff`/`mypy` gate for the docs themselves; the
> gates below are scope/integrity checks. `make lint`/`make test` are NOT required to run for this
> task (no source change) but MAY be run to confirm no accidental source touch.

### Level 1: Scope (no forbidden file touched)

```bash
# Path B (zero edits): README.md is UNCHANGED here. Path A: only README.md listed.
git diff --stat -- README.md docs/cli.md docs/configuration.md
# Expected: path A → exactly README.md; path B → empty.
# MUST be empty (never edit these for this sweep):
git diff --stat -- docs/cli.md docs/configuration.md
git diff --stat -- PRD.md .gitignore
git diff --stat -- 'internal/**/*.go' 'cmd/**/*.go' 'pkg/**/*.go'
git diff --stat -- docs/how-it-works.md FUTURE_SPEC.md
# Expected: all of the above EMPTY.
# Also confirm no tasks/prd-snapshot churn:
git diff --stat -- $(find . -name tasks.json -o -name prd_snapshot.md 2>/dev/null)
# Expected: EMPTY.
```

### Level 2: Doc integrity (anchors + markdown)

```bash
# The Updating heading text is unchanged → (#updating) links still resolve.
grep -n '^### Updating$' README.md                          # Expected: exactly one match (L144)
grep -n '(#updating)' README.md                             # Expected: L87, L392, L422 still present
# The appended sentence (path A only) is present and well-formed.
grep -n 'is detected automatically even when `GOPATH` is unset' README.md   # path A → 1 match; path B → none
# The markdown still parses (optional, if a renderer is available):
glow README.md 2>/dev/null | sed -n '/Updating/,/Quick start/p' || true     # eyeball the paragraph
```

### Level 3: Cross-reference consistency

```bash
# The docs mutually agree on the upgrade story (no contradiction introduced):
#   README Updating ↔ docs/cli.md#upgrade ↔ docs/configuration.md [upgrade]
grep -n 'self-swaps only for a direct' README.md docs/cli.md   # both still say direct-only self-swap
grep -n 'go install' README.md docs/cli.md                     # both still list go install as delegated
grep -n 'admits -rc/-beta\|Admit pre-release' docs/cli.md docs/configuration.md  # channel wording consistent
```

### Level 4: Build sanity (only if a source file was accidentally touched — should be a no-op)

```bash
# If Level 1 showed ANY *.go change, that is a SCOPE VIOLATION — revert it. Otherwise these are no-ops:
go build ./... 2>&1 | tail -5    # Expected: clean (unchanged source)
go test ./internal/upgrade/ -count=1 2>&1 | tail -5   # Expected: PASS (unchanged source)
```

## Final Validation Checklist

### Scope Validation (the hard gate for a doc task)
- [ ] `git diff --stat` touches AT MOST README.md among the three target docs
- [ ] docs/cli.md UNCHANGED (confirmed accurate)
- [ ] docs/configuration.md UNCHANGED (confirmed accurate)
- [ ] PRD.md, .gitignore, **/*.go, docs/how-it-works.md, FUTURE_SPEC.md all UNCHANGED
- [ ] **/tasks.json and **/prd_snapshot.md UNCHANGED

### Review Validation
- [ ] README Updating + Install go-install + make-install reviewed vs BUG-001/002 → accurate
- [ ] docs/cli.md upgrade + --prerelease reviewed vs BUG-001/003 → accurate (flag behavior unchanged)
- [ ] docs/configuration.md [upgrade] channel reviewed vs BUG-003 → accurate
- [ ] No doc statement describes the buggy sanity-run or detection behavior

### Decision Validation
- [ ] Edit-vs-no-edit rubric applied; chosen path recorded with rationale
- [ ] IF path A: the single README Updating sentence is present, anchors intact, paragraph still coherent
- [ ] IF path B: task result + commit message state 'verified accurate, no drift, no edits' + file list

### Doc-Integrity Validation
- [ ] `### Updating` heading text/level unchanged (so `(#updating)` links @L87/L392/L422 resolve)
- [ ] No broken markdown (lists/tables/code-fences still parse)

---

## Anti-Patterns to Avoid

- ❌ **Don't edit docs/cli.md or docs/configuration.md.** Both are confirmed accurate post-fix; BUG-003
  changes tag *selection*, not what `--prerelease` *admits*, so the flag/channel docs are unchanged.
- ❌ **Don't document internal details.** No "the upgrade checks the binary's --version against the tag"
  (BUG-001 sanity-run) or "non-semver tags are skipped" (BUG-003 selection) note in user docs — those
  are implementation details/maintainer concerns and add noise, not help.
- ❌ **Don't "fix" the make-install note or claim symlink resolution.** The Makefile symlinks
  `make install` output into `~/.local/bin`; how detection resolves that symlink is NOT what BUG-002
  fixes (BUG-002 is the GOPATH-unset ~/go fallback) and is unverified/out of scope. The L132/L135 notes
  are accurate as-is.
- ❌ **Don't rename or restructure the `### Updating` heading.** The `(#updating)` anchor is live
  (linked @L87/L392/L422). Append only.
- ❌ **Don't invent edits to look productive.** A zero-edit outcome with a documented verification is a
  fully valid, contract-sanctioned deliverable. The audit found the docs accurate.
- ❌ **Don't touch PRD.md / .gitignore / any *.go / tasks.json / prd_snapshot.md / docs/how-it-works.md
  / FUTURE_SPEC.md.** Mode A source comments are already done in T1/T2/T3.S1; this is Mode B (README +
  overview docs only).
- ❌ **Don't conflate `stagecoach upgrade` (binary) with `config upgrade` (schema).** The docs already
  distinguish them (docs/cli.md L404); don't introduce confusion.

---

## Confidence Score

**9/10** — one-pass success likelihood. This is a documentation review/optional-edit task with a narrow,
fully-enumerated surface (three files, named line ranges, verbatim current text) and a pre-completed
audit (`research/doc_accuracy_audit.md`) that already concluded all three docs are accurate post-fix
with exactly one candidate edit. The implementing PRPs are summarized inline so the fix semantics need
not be re-derived. The two residual uncertainties — (1) whether the implementer chooses path A (one
note) vs path B (no edits), and (2) exact wording if path A — are explicitly governed by a rubric with
verbatim proposed text, and BOTH outcomes are contract-sanctioned complete deliverables. The -1 is for
the inherent judgment in the A/B call (no deterministic answer exists, by design). Scope-risk is low:
the Level-1 scope gate + the "edit only README.md, and only the Updating sentence" constraint make a
stray edit to PRD.md / source / the other two docs immediately catchable.