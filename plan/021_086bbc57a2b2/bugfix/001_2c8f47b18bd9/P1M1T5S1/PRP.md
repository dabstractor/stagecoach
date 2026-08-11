# PRP — P1.M1.T5.S1: Sync work-description READ-protocol docs

## Goal

**Feature Goal**: `docs/how-it-works.md`'s work-description section accurately describes the fixed
READ/answer protocol — specifically the three behaviors the code fixes (T1–T4) corrected: (a) a
non-staged READ produces a note and continues the loop (it does **not** become the commit subject),
(b) large diffs are returned in chunks whose boundaries hug `@@` hunk edges, labeled "part i of N",
with an "end of diff" terminal note once fully read. README.md's work-description surface (if any) is
reviewed for accuracy.

**Deliverable**: A focused "Read/answer protocol" paragraph added inside the existing
`## Work-description mode (description-first, read-on-demand)` section of `docs/how-it-works.md`,
plus (recommended) a README Features row + "More options" example for `--work-description`. Prose
matches the exact note strings emitted by the code.

**Success Definition**: A reader of `docs/how-it-works.md` learns (without reading source) what a
non-staged READ returns and how large diffs are chunked + terminated; `mkdocs build --strict` and
`markdownlint` both pass; no existing accurate doc (cli.md, configuration.md) is touched.

## Why

- BUG-001 (the silent-garbage-commits bug, now fixed in T1) is invisible to users unless the docs
  explain that a non-staged READ is ignored-with-a-note rather than treated as the message. Documenting
  it converts a confusing edge case into an intentional, spec-aligned protocol (FR-W3).
- BUG-002/005/006 (chunking fixes, T2–T4) changed observable output ("end of diff" note, hunk-aligned
  chunks, correct "part i of N" for multibyte UTF-8). The conceptual doc never described chunking at
  all; this PRP closes that gap so the doc matches FR-W5.
- The changeset (P1.M1) shipped code fixes without a doc update; T5.S1 is the dedicated sync task so
  the shipped behavior and the narrative stay locked together.

## What

User-visible **documentation only** (no code, no config, no behavior change):

1. **`docs/how-it-works.md`** — add a "Read/answer protocol" paragraph inside the work-description
   section covering:
   - **Non-staged READ (FR-W3):** if the model's `READ <path>` names a path that is not staged (or is
     unrecognized), stagecoach replies with a short note ("`<path>` is not in the staged changes.") and
     *continues the read loop* — it does **not** treat the line as the commit message. (This is the
     BUG-001 fix. A response with no `READ` line at all is still the commit message, FR-W7.)
   - **Chunked reads (FR-W5):** a READ returns the file's staged diff; if it exceeds a per-call cap
     (≈16K tokens / 64000 runes internally), only the first chunk is returned, labeled
     "`<path>` — part 1 of 3; `READ <path>` again for the next part." Re-reading the same path advances
     a per-file, session-scoped cursor (the model manages no cursor itself).
   - **Hunk-aligned boundaries (FR-W5):** chunk edges hug `@@` hunk edges so a single change is never
     split mid-hunk; a single hunk larger than the cap falls back to a line cut.
   - **End-of-diff marker (FR-W5):** once the final chunk has been read, a further READ of that path
     returns "`<path>` — end of diff (all parts shown)." instead of an empty body. (BUG-002 fix.)
2. **`README.md`** — review. Finding: README currently has **no** `--work-description` mention at all.
   Recommended (optional, in-scope): add one Features-table row + one `### More options` example line so
   the mode is discoverable from the README (it is a shipped, documented flag).

### Success Criteria
- [ ] `docs/how-it-works.md` work-description section contains accurate prose for: non-staged READ note,
      chunked reads with "part i of N", hunk-edge boundaries, and the "end of diff" terminal note.
- [ ] Prose note strings match the code exactly (see "Known Gotchas" § Exact strings).
- [ ] README reviewed; either a Features row + More-options example added, or an explicit "no change —
      README has no inaccurate work-description content" decision recorded in the commit message.
- [ ] `mkdocs build --strict` passes (no new broken links/anchors).
- [ ] markdownlint passes (`.markdownlint.json`).
- [ ] No changes to `docs/cli.md`, `docs/configuration.md` (already accurate — verified), or any code.

## All Needed Context

### Context Completeness Check
"If someone knew nothing about this codebase, would they have everything needed to implement this
successfully?" — **Yes.** The exact note strings, the spec FRs, the file/section locations, the anchor
format, and the validation commands are all specified below. No code knowledge required — this is a
prose-accuracy task with deterministic cross-checks against source strings.

### Documentation & References

```yaml
# MUST READ — the authority (already correct; do not edit the spec)
- url: spec/01-product.md  (FR-W3, FR-W5, FR-W6, FR-W7, FR-W8 — §9.26 work-description mode)
  why: FR-W3 = non-staged READ ignored with note; FR-W5 = chunked reads with implicit cursor,
       hunk-edge boundaries, and "end of diff" terminal note. These are the accuracy targets.
  critical: The spec is the SOURCE OF TRUTH. Code was wrong; spec was right. Match the spec wording,
            then cross-check the note strings against workdesc.go (see Known Gotchas).

# MUST READ — the exact note strings the prose must quote (the accuracy contract)
- file: internal/generate/workdesc.go
  why: These fmt strings are what the model actually sees. Prose must quote them faithfully.
  pattern: |
    L303 buildNonStagedReadAnswer:  "%s is not in the staged changes."      (non-staged READ)
    L379 buildReadAnswer:           "%s is not in the staged changes (or has no further diff)."  (staged but cursor-exhausted)
    L386 end-of-diff:               "%s — end of diff (all parts shown)."   (after final chunk)
    L402 part label:                "%s — part %d of %d; READ %s again for the next part:"
    L36  readChunkTokenCap = 16000
    L452 chunkRuneBudget() = readChunkTokenCap * 4   => 64000 runes
  gotcha: L303 vs L379 are TWO DIFFERENT notes for two different cases (non-staged vs
          staged-but-exhausted). The conceptual doc should describe the non-staged case (L303 wording)
          and the end-of-diff case (L386); don't conflate them.

# MUST READ — the section you are editing
- file: docs/how-it-works.md   (section "## Work-description mode (description-first, read-on-demand)", lines ~358-382)
  why: This is where the new "Read/answer protocol" paragraph goes — inserted AFTER the existing
       "session_mode = append / bounded rounds / no-READ = message" paragraph, BEFORE the
       "If nothing is staged ... auto-stages all" paragraph.
  pattern: Keep the existing voice: conceptual, second-person, one paragraph per idea, inline code
           spans for `READ <path>` / flag names, no fenced code blocks in this section (it's prose).
  gotcha: Do NOT restate what the existing paragraphs already cover (rounds, append session, auto-stage,
          no multi-turn cascade). Only ADD the missing chunking + non-staged-note detail.

# REFERENCE — sibling docs already accurate (verify, do NOT edit)
- file: docs/cli.md  (lines 45-46)
  why: flags table already documents --work-description / --work-description-file accurately,
       including "read staged file diffs on demand via READ <path>". Confirm it needs no change; leave it.
- file: docs/configuration.md  (lines 114, 148)
  why: work_desc_read_rounds = 5 already documented. Confirm; leave it.

# REFERENCE — README structure for the optional Features row
- file: README.md  (Features table lines 53-76; "More options" lines 191-210)
  why: If adding a README work-description row, follow the table's exact column layout
       (| Capability | Description |) and the "([how it works](docs/how-it-works.md#<anchor>) · [flags](docs/cli.md))"
       link pattern used by sibling rows. More options uses fenced ```bash blocks with one flag per line.
  pattern: anchor for the work-description section is
           docs/how-it-works.md#work-description-mode-description-first-read-on-demand
```

### Current Codebase tree (relevant slice)

```bash
docs/
  how-it-works.md          # EDIT — work-description section (lines ~358-382)
  cli.md                   # verify only (lines 45-46) — already accurate
  configuration.md         # verify only (lines 114,148) — already accurate
README.md                  # EDIT (optional) — Features table + More options
internal/generate/
  workdesc.go              # READ ONLY — source of the note strings (L303, L379, L386, L402, L452)
spec/01-product.md         # READ ONLY — authority (FR-W3/W5/W6/W7/W8, §9.26)
mkdocs.yml                 # may need NO change (new content is inside an existing section)
requirements-docs.txt      # for local mkdocs build
.markdownlint.json         # lint config (MD013/MD033/MD060 OFF; rest ON)
.github/workflows/docs.yml # validation gate: mkdocs build --strict
```

### Desired Codebase tree with files to be added/modified

```bash
docs/how-it-works.md       # MODIFIED — +1 "Read/answer protocol" paragraph in work-description section
README.md                  # MODIFIED (optional) — +1 Features row, +1 More-options example line
# (no new files; no mkdocs.yml change — content is inside an existing section)
```

### Known Gotchas of our codebase & Library Quirks

**Exact note strings the prose MUST quote (the accuracy contract)** — quote these verbatim in backticks
so reviewers can diff against source:
- non-staged READ → `` `<path>` is not in the staged changes. ``  (workdesc.go:303)
- fully-read file → `` `<path>` — end of diff (all parts shown). ``  (workdesc.go:386)
- chunk label → `` `<path>` — part 1 of 3; `READ <path>` again for the next part ``  (workdesc.go:402)

**Two different "not staged" notes — don't conflate.** L303 (non-staged path, via
`buildNonStagedReadAnswer`) says plain "is not in the staged changes." L379 (a staged path whose
cursor is exhausted) says "is not in the staged changes (or has no further diff)." The conceptual doc
describes the **non-staged** case (L303) and the **end-of-diff** case (L386); it should NOT quote L379.

**Chunk sizing units.** Internal cap is `readChunkTokenCap = 16000` tokens realized as
`chunkRuneBudget() = 64000` runes (×4, mirroring git's ceil(runes/4) token estimate). The doc should
say "≈16K tokens" (spec FR-W5 wording) — do NOT expose the 64000-rune implementation detail at the
conceptual level; that belongs in code comments, not how-it-works.md.

**mkdocs `--strict` is unforgiving.** Any new markdown link/anchor that doesn't resolve fails the
build. Since new content is *inside* an existing section (no new heading), there is normally nothing to
break — but if you add a Features row in README linking to the work-description section, the anchor
`#work-description-mode-description-first-read-on-demand` MUST be exact (lowercase, hyphens for
spaces/punctuation). mkdocs-material uses the GitHub-style slug.

**No fenced code blocks in this section.** The existing work-description section is pure prose with
inline backticks; a ```bash block would be stylistically inconsistent. Keep inline code spans only.

**Do not edit the spec or code.** This task is doc-only. The spec (`spec/01-product.md`) is the
authority and is already correct; `workdesc.go` is the source of the note strings (read it, don't
change it). Editing either is out of scope and violates the task contract.

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: READ the authority + the exact strings
  - READ spec/01-product.md FR-W3, FR-W5 (§9.26) for the authoritative protocol wording.
  - READ internal/generate/workdesc.go lines 297-310 (buildNonStagedReadAnswer), 360-410
    (buildReadAnswer incl. end-of-diff + part label), 437-460 (nextChunk/chunkCount hunk-edge logic).
  - WHY: the prose you write is a human-readable restatement of these two sources; quoting the note
    strings verbatim is the success criterion.

Task 2: ADD "Read/answer protocol" paragraph to docs/how-it-works.md
  - EDIT docs/how-it-works.md inside "## Work-description mode (description-first, read-on-demand)".
  - PLACEMENT: insert AFTER the paragraph ending "...parsed through the normal parse + duplicate-
    rejection pipeline (so `--format`, `--locale`, `--template`, `--edit` all apply)." and BEFORE the
    paragraph starting "If nothing is staged when `--work-description` is present, ...".
  - CONTENT (one focused paragraph, or a short lead sentence + paragraph): cover, in this order —
      (1) non-staged READ → note + loop continues (NOT the commit message); cite FR-W3.
      (2) large diff → chunked; "part i of N" label; per-file session cursor the model doesn't manage.
      (3) chunk edges hug @@ hunk edges (single change never split mid-hunk; oversize hunk → line cut).
      (4) fully-read file → "end of diff (all parts shown)." note instead of empty body; cite FR-W5.
  - VOICE: match the section's existing conceptual, second-person prose; inline backticks for
    `READ <path>` and the quoted note strings; NO fenced code block.
  - QUOTE the three note strings from "Known Gotchas" verbatim.
  - GOTCHA: do NOT repeat the rounds/append/auto-stage/no-multi-turn material already in the section.

Task 3: REVIEW README.md (optional: add Features row + More-options line)
  - DECISION point: README has NO work-description mention today. Either—
      (A) RECOMMENDED: add a Features-table row (follow the | Capability | Description | layout and the
          "([how it works](docs/how-it-works.md#work-description-mode-description-first-read-on-demand) ·
          [flags](docs/cli.md))" link pattern) + a one-line ```bash example under "More options":
          `stagecoach --work-description "..."  # lead with your description; model READs staged diffs on demand`
      (B) NO CHANGE: if you judge adding a row is out of scope for a doc-sync/accuracy task, leave
          README untouched and record the decision ("README has no work-description content; nothing
          inaccurate to fix") in the commit message.
  - IF adding the row: ensure the anchor is exact (see Known Gotchas) or mkdocs --strict will fail.

Task 4: VERIFY sibling docs need no change
  - READ docs/cli.md lines 45-46 and docs/configuration.md lines 114 & 148.
  - CONFIRM they already accurately describe the flags and work_desc_read_rounds=5. DO NOT edit them.
  - (If, contrary to expectation, one is stale, fix only the stale sentence — but research says they
     are accurate.)
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN: insert something like this paragraph (adapt wording; keep the quoted strings exact).
     Place it between the "no-READ = message" paragraph and the "If nothing is staged" paragraph. -->

**Read/answer protocol.** A `READ <path>` whose path is *staged* returns that file's diff. A path that
is *not* staged (or isn't recognized) is answered with a short note — "`<path>` is not in the staged
changes." — and the read loop continues; the line is never treated as the commit message (a response
with no `READ` line is the message, as above). Large diffs are returned in chunks: each chunk is labeled
"`<path>` — part 1 of 3; `READ <path>` again for the next part," and re-reading the same path advances a
per-file, session-scoped cursor (the model manages no cursor itself). Chunk edges hug `@@` hunk edges so
a single change is never split mid-hunk; a single hunk larger than the per-call cap falls back to a line
cut. Once the final chunk has been read, a further `READ` of that path returns
"`<path>` — end of diff (all parts shown)." rather than an empty body.
```

### Integration Points

```yaml
DOCUMENTATION:
  - primary edit:   docs/how-it-works.md  (work-description section, ~lines 358-382)
  - optional edit:  README.md            (Features table + More options)
  - no change:      docs/cli.md, docs/configuration.md, mkdocs.yml, any .go file, spec/01-product.md

ANCHORS (mkdocs --strict validates these):
  - work-description section anchor: #work-description-mode-description-first-read-on-demand
    (used if you add a README Features row linking to it)

CONFIG / CODE: none — doc-only task.
```

## Validation Loop

### Level 1: Markdown lint (immediate)

```bash
# If markdownlint-cli2 / markdownlint is available locally:
markdownlint docs/how-it-works.md README.md
# .markdownlint.json disables MD013 (line length), MD033 (inline HTML), MD060.
# All other default rules are ON (MD001 heading order, MD040 fenced-code language, MD041 first-line H1).
# Expected: clean. If MD040 fires, you added a fenced block without a language tag — remove it
# (the work-description section uses inline code spans, not fenced blocks).
```

### Level 2: mkdocs strict build (the real gate)

```bash
# This is the CI gate (.github/workflows/docs.yml). --strict turns broken links/anchors into ERRORS.
pip install -r requirements-docs.txt
mkdocs build --strict
# Expected: "INFO    -  Documentation built successfully" with zero warnings.
# If it fails on an anchor/link, the anchor slug is wrong — fix it (see Known Gotchas).
```

### Level 3: Accuracy cross-check (human diff — this is the substantive gate)

```bash
# Confirm every string the new prose quotes actually exists in the code:
grep -nF 'is not in the staged changes.' internal/generate/workdesc.go        # non-staged note (L303)
grep -nF '— end of diff (all parts shown).' internal/generate/workdesc.go     # terminal note (L386)
grep -nF '— part %d of %d' internal/generate/workdesc.go                      # chunk label (L402)
grep -n 'readChunkTokenCap = ' internal/generate/workdesc.go                  # 16000 (L36)
# Expected: all four match. If a quoted string has no grep hit, the prose is wrong — fix the prose
# to match the code (never edit the code for this task).

# Confirm the spec authority wording aligns (FR-W3 / FR-W5 in §9.26):
grep -n 'FR-W3\|FR-W5' spec/01-product.md
```

### Level 4: Sibling-doc regression check

```bash
# Confirm you did NOT accidentally edit the already-accurate docs:
git diff --stat docs/cli.md docs/configuration.md
# Expected: no output (unchanged). If a diff appears, revert it unless it fixes a genuine inaccuracy.
git diff --stat spec/01-product.md internal/generate/workdesc.go
# Expected: no output (this task is doc-only; never edit spec or code).
```

## Final Validation Checklist

### Technical Validation
- [ ] `mkdocs build --strict` passes with zero warnings.
- [ ] `markdownlint docs/how-it-works.md README.md` clean (or no new violations introduced).
- [ ] All four Level-3 grep hits present (quoted strings match code exactly).
- [ ] `git diff --stat` shows changes ONLY in `docs/how-it-works.md` (+ optional `README.md`);
      spec/01-product.md, internal/**/*.go, docs/cli.md, docs/configuration.md untouched.

### Feature Validation
- [ ] how-it-works.md work-description section describes: non-staged READ note + loop continues;
      chunked reads with "part i of N"; `@@` hunk-edge boundaries; "end of diff" terminal note.
- [ ] No duplication of existing section content (rounds, append session, auto-stage, no multi-turn).
- [ ] README decision made and recorded (row added OR explicit "no change" rationale in commit msg).

### Code Quality / Doc Hygiene
- [ ] Prose matches the section's existing voice (conceptual, second-person, inline backticks, no
      fenced code blocks in this section).
- [ ] No new broken anchors; any new README link slug is exact.
- [ ] Commit message follows repo convention and cites the FRs (FR-W3, FR-W5) and the bug IDs
      (BUG-001, BUG-002, BUG-005, BUG-006) it documents.

---

## Anti-Patterns to Avoid

- ❌ Don't edit `spec/01-product.md` or any `.go` file — this is a doc-accuracy task; the spec is the
  authority and the code is the source of the quoted strings. Read both, change neither.
- ❌ Don't conflate the two "not staged" notes (L303 plain vs L379 "(or has no further diff)") — the
  conceptual doc describes the non-staged case (L303) and the end-of-diff case (L386).
- ❌ Don't expose the 64000-rune implementation detail at the conceptual level — say "≈16K tokens"
  (spec wording); the rune math belongs in code comments.
- ❌ Don't add a fenced ```bash block inside the work-description section — it breaks the section's
  prose style and can trip markdownlint MD040 if untagged.
- ❌ Don't restate rounds / append-session / auto-stage / no-multi-turn-cascade — already in the section.
- ❌ Don't skip `mkdocs build --strict` — a wrong anchor ships a dead link in CI.

---

## Confidence Score: 9/10

This is a low-risk, well-bounded doc-accuracy task. The only residual uncertainty is the README
decision (add a row vs. confirm no change), which the PRP resolves by making it an explicit, recorded
decision rather than an open question. All accuracy targets are pinned to exact, grep-verifiable source
strings, so "done" is deterministically checkable.