name: "P1.M4.T4.S1 — Sync user docs to the lock/watchdog fixes: refine the docs/cli.md `orphaned:` hint explanation (single-file doc edit) + verify the rest of README.md/docs already match"
description: >
  A DOCUMENTATION-SYNC task (the DOCS clause of the P1.M4 lock/watchdog work). P1.M4.T1–T3 were ALL
  code-only (one defense-in-depth re-read gate in `reapStaleLocks` + two comment-only rewrites); they
  deliberately left README.md / docs/ untouched (T1/T2 committed as 7cfca67/6bef7f6; T3 is Mode-A =
  code comment only). This task reviews the user-facing docs for accuracy about lock-status display,
  orphan detection, and stale-file reaping, and refines the ONE genuine mismatch found. FINDING: the
  docs are ~99% accurate — the output example, the lock-status command description, the reaping prose,
  the Windows-no-op prose, and the watchdog prose all already match the fixed code (the fixes changed
  NO user-visible behavior). The single mismatch is `docs/cli.md:~400`'s explanation of the `orphaned:`
  line: it says "the holder's parent pid changed" — but the orphan HINT (`orphan_unix.go:appearsOrphaned`)
  actually tests `ppid == 1` (reparented to INIT), whereas the parent-pid-CHANGE test is what the WATCHDOG
  (`arm_unix.go:osGetppid() != originalPpid`, FR-K2) uses. Under a subreaper (systemd/Docker/supervisord)
  a reparented orphan's ppid is the subreaper's pid (≠1) ⇒ the hint reads `false` even though orphaned, so
  cli.md:400's "parent pid changed" / "alive and not reparented" wording is both imprecise and (for the
  subreaper case) wrong. FIX: rewrite ONLY that paragraph in docs/cli.md to (a) describe the hint's actual
  `ppid == 1`→init mechanism, (b) NOT conflate it with the watchdog's parent-pid-change, (c) add a brief
  display-only subreaper false-negative note (consistent with how-it-works.md:222 which already names
  subreapers), and (d) cross-ref the authoritative watchdog. ONE FILE, docs-only. NO code / test / spec /
  README / how-it-works change. The rest of the docs are verified-accurate via grep guards (this is a
  review-and-refine task; "if docs already match, no changes needed" — they match EXCEPT this paragraph).
  Validated by markdownlint (`.markdownlint.json`: MD013 OFF, default ON) + grep guards proving accuracy +
  `go build ./...` sanity (docs can't break it). Git scope = docs/cli.md ONLY.

---

## Goal

**Feature Goal**: After the code-only lock/watchdog fixes (P1.M4.T1–T3), make the user-facing documentation
**accurately reflect the lock/watchdog behavior** — specifically the `lock status` orphan hint. Review
README.md + docs/*.md; confirm the fixes introduced no doc drift; refine the one mismatched explanation
(docs/cli.md's `orphaned:` outcomes paragraph) so it describes the hint's real mechanism (`ppid == 1` →
reparented to init) instead of the watchdog's (parent-pid-change), and notes the display-only subreaper
false-negative.

**Deliverable**: ONE edit — rewrite the `orphaned:` outcomes paragraph in `docs/cli.md` (the single
sentence-block currently spanning lines ~400). The output EXAMPLE (cli.md:388–398) and the command
description (cli.md:387) are UNCHANGED (they already match `internal/cmd/lock.go` byte-for-byte).
README.md, docs/how-it-works.md, and all other docs are verified-accurate and NOT edited.

**Success Definition**:
- `docs/cli.md`'s `orphaned:` outcomes paragraph accurately describes the hint as `ppid == 1` (reparented to
  init), names the subreaper (systemd/Docker/containerd/supervisord) display-only false-negative, and
  cross-references the authoritative parent-death watchdog in how-it-works.md.
- The paragraph no longer claims the hint detects "parent pid changed" (that is the watchdog's mechanism).
- `markdownlint docs/cli.md` clean (`.markdownlint.json`: default true, MD013/MD033/MD060 off).
- Grep guards pass: no doc repeats the false "lock files aren't created on Windows" claim; the watchdog prose
  (how-it-works.md:224) still names parent-pid-change + subreaper-safe; the Windows-no-op prose
  (how-it-works.md:220) is intact; the `lock status` output example still matches the code.
- `go build ./...` clean (sanity — a doc edit cannot break it, but confirm); `make test`/`make lint` unaffected.
- `git status --porcelain` == `docs/cli.md` ONLY.

## User Persona (if applicable)

**Target User**: The Stagecoach user who runs `stagecoach lock status` to diagnose a stranded run, reads the
`orphaned:` line, and consults `docs/cli.md#lock-status` to interpret `true` / `false` / `unknown`.
**Use Case**: User killed their IDE/lazygit mid-run; `lock status` shows the holder alive. They want to know
whether it's orphaned (launcher gone) and whether it will self-exit. They read cli.md's explanation.
**User Journey**: run `lock status` → read the `orphaned:` field → open docs/cli.md#lock-status → understand
what each value means and that the watchdog is the real backstop → decide to wait (watchdog will self-exit)
or `kill <pid>` / `rm <path>`.
**Pain Points Addressed**: the current paragraph conflates the hint with the watchdog mechanism, so a user on
systemd/Docker who sees `orphaned: false` for a genuinely-reparented holder is misled into thinking the
holder is NOT orphaned. The refined paragraph explains the `ppid==1` mechanism, the subreaper gap, and points
at the authoritative watchdog.

## Why

- **The DOCS clause of the P1.M4 work**: T1–T3 were code-only and explicitly deferred the README/docs review
  to this task. Someone must verify the user docs still match the fixed code. This task is that verification.
- **The fixes changed no user-visible behavior**: T1 (re-read gate) still reaps dead-pid files; T2 (Windows
  comment) corrected a false claim IN THE CODE COMMENT (no doc ever made that claim); T3 (orphan comment) is
  Mode-A code-only. So the review's expected outcome is "docs match" — and they do, EXCEPT cli.md:400.
- **cli.md:400 is a real (if minor) accuracy bug**: it attributes "parent pid changed" to the `orphaned:`
  hint, but the hint tests `ppid == 1`. Under a subreaper the hint reads `false` for a reparented holder, so
  the current wording ("parent pid changed" / "alive and not reparented") is both imprecise and wrong in that
  case. T3 documented this limitation in the code comment; surfacing it briefly in the user doc (where the
  hint is explained) closes the loop — consistent with how-it-works.md:222 which already names subreapers.

## What

A single-paragraph rewrite in `docs/cli.md` (the `orphaned:` outcomes paragraph at ~line 400). Doc-only.
No code, no tests, no spec, no other doc file.

### Success Criteria
- [ ] `docs/cli.md`'s `orphaned:` outcomes paragraph states the hint reports `ppid == 1` (reparented to init),
      NOT "parent pid changed".
- [ ] The paragraph notes the display-only subreaper false-negative (names ≥2 of systemd/Docker/containerd/
      supervisord) and that the hint is a snapshot (not a runtime-change detector).
- [ ] The paragraph cross-references the parent-death watchdog in how-it-works.md as the authoritative
      backstop that detects parent-pid *change* at runtime.
- [ ] The `false` outcome is reworded to "alive and its parent is not init" (not the wrong "not reparented").
- [ ] The output EXAMPLE block (cli.md:388–398) and the `lock status` description (cli.md:387) are UNCHANGED.
- [ ] `markdownlint docs/cli.md` clean; `go build ./...` clean; git scope == docs/cli.md.
- [ ] Grep guards pass (see Validation Loop Level 4): no false "files aren't created" claim anywhere;
      watchdog prose intact; Windows-no-op prose intact; output example matches code.

## All Needed Context

### Context Completeness Check
_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim current paragraph (the exact text to replace), the verbatim replacement, the code
facts that justify each change (the `appearsOrphaned` body = `ppid == 1`; the watchdog = parent-pid-change),
the full doc audit (which docs are accurate and why), the markdownlint config, and the grep guards.

### Documentation & References

```yaml
# MUST EDIT — the one file, the one paragraph.
- file: docs/cli.md
  section: "### `lock status` (line ~385); the `orphaned:` outcomes paragraph (line ~400)"
  why: "The paragraph to rewrite. It is the ONLY place the docs overstate the orphan hint. The output
        example block above it (lines 388–398) and the command description (line 387) ALREADY match the
        code — leave them UNCHANGED."
  pattern: "Markdown prose paragraph with inline `code` spans + a **bold** clause. Match the surrounding
            doc voice (it uses `—` em-dashes, parenthetical asides, and cross-refs like
            [how-it-works.md — Per-repo run lock](how-it-works.md#per-repo-run-lock-fr52))."
  gotcha: "`.markdownlint.json` = {default:true, MD013:false, MD033:false, MD060:false} → line-length OFF,
           inline-HTML OFF, but default rules ON (no trailing spaces; balanced `**`/`*`/backticks). Keep the
           markdown link target EXACTLY `how-it-works.md#per-repo-run-lock-fr52` (it is referenced elsewhere
           with that anchor — verify it resolves)."

# MUST READ — the code the paragraph must match (READ-ONLY; do NOT edit).
- file: internal/lock/orphan_unix.go
  why: "appearsOrphaned (the HINT). Its body is `ppid, err := ppidOf(pid); if err != nil {return false};
        return ppid == 1`. The paragraph MUST describe THIS (ppid==1 → reparented to init), not the
        watchdog. The strengthened doc comment (P1.M4.T3.S1, BUG-011) already documents the subreaper
        false-negative + cross-refs arm_unix.go — the user-facing paragraph should mirror that substance."
  critical: "ppid==1 is the ONLY `true`. Under a subreaper the orphan's ppid is the subreaper's pid (≠1) ⇒
             false. This is display-only; the watchdog is the backstop."

# MUST READ — the watchdog (the mechanism cli.md:400 currently WRONGLY attributes to the hint).
- file: internal/watchdog/arm_unix.go
  why: "armImpl polls `osGetppid() != originalPpid` (FR-K2, subreaper-safe): captures the parent pid at
        startup, detects the CHANGE on reparenting (to init OR a subreaper), and self-exits through the
        rescue + lock-release path. THIS is the 'parent-pid changed' detector — NOT the appearsOrphaned
        hint. The refined paragraph must credit the watchdog, not the hint, with parent-pid-change."

# MUST READ — the status command that PRODUCES the `orphaned:` line (READ-ONLY; do NOT edit).
- file: internal/cmd/lock.go
  why: "runLockStatus prints: `orphaned: true (holder reparented — launcher has exited)` (when appearsOrphaned
        true); `orphaned: false` (alive, incl. Windows where processAlive is always true); `orphaned: unknown
        (holder is dead)`. The cli.md output EXAMPLE matches this byte-for-byte — UNCHANGED. The paragraph
        BELOW the example is what we refine."

# MUST READ (verify accuracy, do NOT edit) — the docs that are ALREADY correct.
- file: docs/how-it-works.md
  section: "Per-repo run lock (lines 207–232)"
  why: "Confirms the watchdog prose is accurate: line 220 (Windows no-op + CAS guarantee — NO false
        'files aren't created' claim), line 222 (reparented to init 'or a subreaper'), line 224 (watchdog
        detection 'by parent-pid change … not the brittle getppid==1 test — subreaper-safe'), line 228
        (lock status read-only). All ACCURATE — do NOT edit how-it-works.md. The cli.md cross-ref TARGET
        anchor is `#per-repo-run-lock-fr52` (verify it resolves in how-it-works.md)."
- file: README.md
  section: "lines 360, 406, 408"
  why: "Confirms README is accurate: line 360 (status summary), line 408 (watchdog self-exit + status path/
        liveness + never-force-break). NO edit. (Line 24 'locked down' / line 161 'run lock' are unrelated.)"

# CONTEXT — the sibling PRPs that produced the (code-only) fixes this task syncs docs to.
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M4T1S1/PRP.md
  why: "BUG-009: the re-read-before-remove gate. Confirms dead-pid files are STILL reaped (the gate only
        skips a remove when a concurrent acquirer rewrote the file) — so how-it-works.md:211's 'reaped by
        pid-liveness on the next Acquire' is still accurate. NO doc drift from T1."
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M4T2S1/PRP.md
  why: "BUG-010: the Windows processAlive comment-only rewrite. Confirms it corrected the false 'files
        aren't created on Windows' claim IN THE CODE COMMENT — and that NO doc ever made that claim (grep
        verifies). NO doc drift from T2."
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M4T3S1/PRP.md
  why: "BUG-011: the appearsOrphaned comment-only rewrite (Mode A — code comment only, explicitly NO
        user-facing doc change). This defers the doc surface to THIS task (P1.M4.T4.S1). The cli.md:400
        refinement is the user-facing counterpart to T3's code comment."

# CONTEXT — the bug definitions (the contract source).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/architecture/bugfix_subsystems.md
  section: "§BUG-009 / §BUG-010 / §BUG-011 (lines 146–190)"
  why: "Confirms all three are minor / defense-in-depth / documentation, and that BUG-011's orphan hint is
        display-only (the watchdog is the safety property)."

# CONTEXT — the review findings (the full doc audit).
- docfile: plan/021_086bbc57a2b2/bugfix/001_2c8f47b18bd9/P1M4T4S1/research/findings.md
  why: "The line-by-line doc audit: every lock/orphan/watchdog/reap match in README.md + docs/*.md, marked
        accurate or the-one-mismatch. The grep commands to re-verify are in §4."
```

### Current Codebase tree (relevant slice)

```bash
docs/
  cli.md               # EDIT — the `orphaned:` outcomes paragraph (~line 400). ONE paragraph only.
  how-it-works.md      # READ-ONLY — accurate (watchdog + Windows-no-op + reaping prose); cross-ref TARGET.
  configuration.md     # READ-ONLY — accurate (no_parent_watchdog opt-out + lock-file location).
  overview.md          # READ-ONLY — accurate (lock-reclamation summary).
  windows-test-support.md  # READ-ONLY — accurate (lock_windows.go is a documented no-op).
README.md              # READ-ONLY — accurate (status summary + watchdog self-exit).
internal/lock/
  orphan_unix.go       # READ-ONLY — appearsOrphaned body = `ppid == 1` (the mechanism cli.md must match).
  lock_windows.go      # READ-ONLY — processAlive `return true` (Windows always → orphaned: false).
  lock.go              # READ-ONLY — Status/IsOrphaned (callers); reapStaleLocks (T1 gate).
internal/watchdog/
  arm_unix.go          # READ-ONLY — osGetppid()!=originalPpid (the watchdog = parent-pid-change).
internal/cmd/
  lock.go              # READ-ONLY — runLockStatus (produces the `orphaned:` line; output example matches).
.markdownlint.json     # READ-ONLY — {default:true, MD013:false, MD033:false, MD060:false}
```

### Desired Codebase tree with files to be modified

```bash
docs/cli.md   # EDIT — rewrite the `orphaned:` outcomes paragraph (~line 400) ONLY.
# NOTHING ELSE. Doc-only. No code, no tests, no other doc, no spec, no README.
```

### Known Gotchas of our codebase & Library Quirks

```markdown
<!-- CRITICAL (the hint vs the watchdog — do NOT conflate): appearsOrphaned (internal/lock/orphan_unix.go)
tests `ppid == 1` (reparented to INIT). The parent-pid-CHANGE test is the WATCHDOG
(internal/watchdog/arm_unix.go: `osGetppid() != originalPpid`, FR-K2). cli.md:400 currently attributes
"parent pid changed" to the HINT — that is the bug. The refined paragraph must credit the watchdog with
parent-pid-change and describe the hint as ppid==1. -->

<!-- CRITICAL (the subreaper false-negative is the whole point): under a subreaper (systemd, Docker/
containerd/runc, supervisord, podman — PR_SET_CHILD_SUBREAPER) a reparented orphan's ppid is the SUBREAPER's
pid, not 1, so the hint reads `false`. This is display-only (the watchdog still catches it). The current
"alive and not reparented" wording for `false` is WRONG in this case — refine it to "alive and its parent is
not init". T3 already documented this in the code comment; mirror it briefly in the user doc. -->

<!-- CRITICAL (AGENTS.md rule #2 — never edit spec/ autonomously): this is a USER-DOC edit (docs/cli.md), not
a spec edit. Do NOT touch spec/SPEC.md, spec/*.md, PRD.md, tasks.json, or prd_snapshot.md. FR-K2 in the spec
is correct; the bug is in the user doc's explanation of the hint, not the spec. -->

<!-- GOTCHA (the output EXAMPLE must stay byte-identical): the fenced ```text block at cli.md:388–398
matches internal/cmd/lock.go:runLockStatus byte-for-byte (field names, the `orphaned:  true (holder
reparented — launcher has exited)` string, the snapshot "only shown once the snapshot is armed" comment).
Do NOT "fix" the example — it is already correct. Edit ONLY the prose paragraph BELOW the example. -->

<!-- GOTCHA (markdownlint): `.markdownlint.json` has MD013 (line-length) OFF, so the long paragraph is fine,
but default rules are ON. Avoid trailing whitespace, keep backtick/asterisk emphasis balanced, and keep the
markdown link target EXACTLY `how-it-works.md#per-repo-run-lock-fr52`. Run `markdownlint docs/cli.md` (or
`npx markdownlint-cli2 docs/cli.md` if that's the installed flavor) — must be clean. -->

<!-- GOTCHA (scope fence — README.md and how-it-works.md are NOT edited): the review FOUND them accurate.
Do not "improve" them. Editing them widens scope and risks the grep guards (which assert they're untouched).
One file changes: docs/cli.md. -->

<!-- GOTCHA (doc-only — no build/test impact): `go build ./...` / `make test` / `make lint` cannot be broken
by a markdown edit. Run them as a sanity check (prove no collateral from a stray edit), not as the gate. The
REAL gates are markdownlint + the grep guards. -->
```

## Implementation Blueprint

### Data models and structure
None — documentation-only. No types, code, config, or schemas.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT docs/cli.md — rewrite the `orphaned:` outcomes paragraph (the ONLY edit)
  - LOCATE the paragraph immediately BELOW the fenced ```text output example in the `### lock status`
    section (~line 400). It begins "The `orphaned:` line has three outcomes:" and ends "… the action
    (kill/rm) is yours."
  - CURRENT TEXT (verbatim — replace this whole paragraph):
      The `orphaned:` line has three outcomes: `true (holder reparented — launcher has exited)` (Unix; the
      holder's parent pid changed — its launcher closed without killing it), `false` (alive and not
      reparented — Windows always lands here), or `unknown (holder is dead)` (the holder process is no longer
      alive). With no lock held, the output is `no run lock for <repo>` (exit 0). Exit is **0 in all cases**
      — even when the holder is dead or orphaned — the read is the help; the action (kill/rm) is yours.
  - REPLACEMENT (verbatim substance — wording may be lightly trimmed, but ALL 4 content points MUST appear:
      (1) ppid==1→init mechanism, (2) NOT parent-pid-change (that's the watchdog), (3) subreaper
      display-only false-negative, (4) watchdog cross-ref):
      The `orphaned:` line has three outcomes: `true (holder reparented — launcher has exited)` (Unix; the
      holder's parent is now process 1 — its launcher closed without killing it, so the kernel reparented
      the holder to init), `false` (alive and its parent is not init — Windows always lands here), or
      `unknown (holder is dead)` (the holder process is no longer alive). This is a *display-only snapshot
      hint*: it reports whether the holder currently appears reparented to init (`ppid == 1`), not whether
      its parent changed at runtime, so under a subreaper (systemd, Docker/containerd, supervisord) a
      reparented holder can read `false`. The authoritative backstop is the parent-death watchdog, which
      detects a parent-pid *change* at runtime and self-exits + releases the lock regardless (see
      [how-it-works.md — Per-repo run lock](how-it-works.md#per-repo-run-lock-fr52)). With no lock held, the
      output is `no run lock for <repo>` (exit 0). Exit is **0 in all cases** — even when the holder is dead
      or orphaned — the read is the help; the action (kill/rm) is yours.
  - PRESERVE: the fenced ```text output example ABOVE the paragraph (cli.md:388–398 — matches code); the
    command description (cli.md:387); the `### lock status` heading (cli.md:385); the example ```bash block
    BELOW the paragraph (cli.md:403–405). Edit ONLY the one prose paragraph.
  - NAMING/PLACEMENT: in place; no new heading; no new link except the how-it-works.md cross-ref (target
    anchor `#per-repo-run-lock-fr52` — verify it exists in how-it-works.md before relying on it).
  - GOTCHA: keep ≥2 subreaper names (systemd + Docker/containerd OR supervisord); keep `ppid == 1`; keep
    "parent-pid *change*" (with the emphasis on change) credited to the watchdog; keep "display-only".

Task 2: VERIFY — markdownlint + grep guards + build sanity + scope guard
  - markdownlint docs/cli.md        # .markdownlint.json: MD013 off, default on → clean
  - go build ./...                  # sanity (doc edit can't break it; confirms no stray code edit)
  - grep guards (see Validation Loop Level 4)
  - git status --porcelain          # docs/cli.md ONLY
```

### Implementation Patterns & Key Details

```markdown
<!-- PATTERN (the 4 mandatory content points in the rewritten paragraph). KEEP these exact ideas; trim
     wording to taste but do not drop any:
       (1) "the holder's parent is now process 1" / "reparented the holder to init" — the ppid==1 mechanism.
       (2) "not whether its parent changed at runtime" — explicitly distinguishes from the watchdog.
       (3) "under a subreaper (systemd, Docker/containerd, supervisord) a reparented holder can read false"
           — the display-only false-negative.
       (4) "the parent-death watchdog … detects a parent-pid change at runtime and self-exits + releases
            the lock" + the how-it-works.md cross-ref — the authoritative backstop.
     The `false` outcome must read "alive and its parent is not init" (NOT the old "not reparented"). -->

<!-- PATTERN (doc voice — match the surrounding cli.md style): `—` em-dashes for asides, inline `code` spans
     for field values (`true`, `false`, `ppid == 1`), *single-asterisk italics* for "display-only snapshot
     hint" / "change", **double-asterisk bold** for "0 in all cases". One paragraph; no bullet list (the
     surrounding section is prose). Cross-ref format mirrors existing links in cli.md (e.g. line 458). -->
```

### Integration Points

```yaml
DOCS (the single edit):
  - docs/cli.md: the `orphaned:` outcomes paragraph (rewrite). The output example + command description
    ABOVE it are UNCHANGED.
CROSS-REFERENCES:
  - the paragraph links to how-it-works.md#per-repo-run-lock-fr52 (VERIFY the anchor resolves before
    relying on it; it is used elsewhere in the repo with that exact spelling).
NO code / test / spec / config / route / exit-code / go.mod / README / how-it-works change.
SCOPE FENCES:
  - Touches ONLY: docs/cli.md (one paragraph).
  - Does NOT touch: README.md (accurate), docs/how-it-works.md (accurate), docs/configuration.md,
    docs/overview.md, docs/windows-test-support.md, any *.go file, any *_test.go, any spec/*.md file
    (AGENTS.md rule #2), PRD.md, tasks.json, prd_snapshot.md, go.mod, .markdownlint.json.
  - Parallel-safe: P1.M4.T3.S1 (orphan_unix.go code comment) is the working-tree change; its Mode-A
    contract forbids doc edits — zero overlap with this doc-only item.
```

## Validation Loop

### Level 1: Markdown Lint & Style (Immediate Feedback)

```bash
# markdownlint on the edited file (.markdownlint.json: default true, MD013/MD033/MD060 off).
markdownlint docs/cli.md 2>/dev/null || npx markdownlint-cli2 docs/cli.md 2>/dev/null || npx markdownlint-cli docs/cli.md
# Expected: clean. If a default rule fires (trailing space, unbalanced emphasis), fix it. Line-length
#           (MD013) is OFF, so the long paragraph is fine.

# Sanity: the fenced output EXAMPLE block is byte-identical to the code's output (do NOT have edited it).
sed -n '/^```text$/,/^```$/p' docs/cli.md
# Expected: the Lock:/pid:/hostname:/repo:/timestamp:/snapshot:/alive:/orphaned: block unchanged.
grep -c 'orphaned:  true (holder reparented — launcher has exited)' docs/cli.md   # ≥1 (the example, unchanged)
```

### Level 2: Build Sanity (a doc edit cannot break code — confirm)

```bash
# Docs don't affect the build, but run to prove no stray code edit snuck in.
go build ./...
# Expected: clean.

# Full suite + lint untouched by a doc change; run to prove no collateral.
make test && make lint
# Expected: all green (identical to before — this change touches zero .go files).
```

### Level 3: Cross-Reference Integrity

```bash
# The paragraph links to how-it-works.md#per-repo-run-lock-fr52 — verify the anchor exists.
grep -n 'per-repo-run-lock-fr52' docs/how-it-works.md
# Expected: ≥1 hit (the heading the anchor targets). If missing, the anchor is stale → fix the link target
#           to the actual heading in how-it-works.md (do NOT change how-it-works.md).

# Render check (optional): open docs/cli.md in a markdown previewer and click the link; it should jump to
# the "Per-repo run lock" section of how-it-works.md.
```

### Level 4: Accuracy Grep Guards (the REAL gates — prove the docs match the code)

```bash
# Guard 1 (the fix): cli.md now describes the hint as ppid==1→init, NOT "parent pid changed".
grep -n 'ppid == 1\|parent is now process 1\|reparented the holder to init' docs/cli.md   # ≥1
grep -n 'parent pid changed' docs/cli.md && echo "FAIL: stale 'parent pid changed' wording remains" || echo "OK: hint no longer conflated with watchdog"

# Guard 2 (the subreaper note): cli.md names ≥2 subreapers + the display-only false-negative.
for s in systemd Docker containerd supervisord; do grep -qi "$s" docs/cli.md && echo "OK: $s" || echo "MISSING: $s"; done
grep -niE 'display.only|snapshot hint|can read .?false' docs/cli.md   # ≥1

# Guard 3 (the watchdog cross-ref): cli.md credits the watchdog with parent-pid-CHANGE + links how-it-works.
grep -niE 'parent-death watchdog|parent-pid .?change|self-exits' docs/cli.md   # ≥1
grep -n 'how-it-works.md#per-repo-run-lock-fr52' docs/cli.md                     # ≥1

# Guard 4 (NO false "files aren't created on Windows" claim anywhere — T2 corrected it in code; docs never had it).
grep -rniE 'lock file.*(not|aren.t|isn.t|never) created|not created.*lock file' README.md docs/*.md && echo "FAIL: false claim present" || echo "OK: no false 'files not created' claim"
# (The pre-existing cli.md:449 "commit not created" is a rescue-condition false positive — not a lock-file claim.)

# Guard 5 (the watchdog prose in how-it-works.md is UNCHANGED + accurate).
grep -n 'parent-pid change\|subreaper-safe' docs/how-it-works.md   # ≥1 (FR-K2 watchdog detection, untouched)
grep -n 'not the brittle .getppid==1' docs/how-it-works.md          # ≥1 (still distinguishes from ppid==1)

# Guard 6 (the Windows-no-op prose in how-it-works.md is UNCHANGED + accurate).
grep -n 'On Windows.*flock.*no-op.*reaping is a no-op.*CAS is the guarantee' docs/how-it-works.md   # ≥1

# Guard 7 (the output example still matches the code — `orphaned:` strings unchanged).
grep -n 'orphaned:  true (holder reparented — launcher has exited)' docs/cli.md   # the example block (≥1)
grep -n 'unknown (holder is dead)' docs/cli.md                                      # the example/paragraph (≥1)

# Guard 8 (scope — ONLY docs/cli.md changed).
git status --porcelain
test "$(git status --porcelain | wc -l)" -eq 1 && echo "OK: one file" || echo "FAIL: expected one file"
git diff --name-only | grep -vE '^docs/cli\.md$' && echo "FAIL: out-of-scope file" || echo "OK: scope clean"

# Guard 9 (NO spec/PRD/task edit — AGENTS.md rules #2 + forbidden-operations).
git diff --name-only | grep -qE 'PRD\.md|spec/|tasks\.json|prd_snapshot|README\.md|how-it-works\.md|\.go$' && echo "FAIL: edited a forbidden/out-of-scope file" || echo "OK: only docs/cli.md"
```

## Final Validation Checklist

### Technical Validation
- [ ] `markdownlint docs/cli.md` clean (default rules; MD013 off)
- [ ] `go build ./...` clean; `make test` + `make lint` green (no collateral — zero .go files touched)
- [ ] The how-it-works.md cross-ref anchor (`#per-repo-run-lock-fr52`) resolves

### Feature Validation (the paragraph accurately describes the hint)
- [ ] The hint is described as `ppid == 1` (reparented to init), NOT "parent pid changed" (grep guard 1)
- [ ] The subreaper display-only false-negative is noted with ≥2 named subreapers (grep guard 2)
- [ ] The watchdog is credited with parent-pid-CHANGE + cross-referenced (grep guard 3)
- [ ] The `false` outcome reads "alive and its parent is not init" (not the wrong "not reparented")
- [ ] The output example + command description ABOVE the paragraph are UNCHANGED (grep guard 7)

### Review Validation (the rest of the docs are confirmed accurate — no edit)
- [ ] No doc repeats the false "lock files aren't created on Windows" claim (grep guard 4)
- [ ] how-it-works.md watchdog prose intact (parent-pid-change + subreaper-safe) (grep guard 5)
- [ ] how-it-works.md Windows-no-op prose intact (flock no-op + reaping no-op + CAS guarantee) (grep guard 6)
- [ ] README.md lock/watchdog lines (360/406/408) accurate and UNCHANGED

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `docs/cli.md` (grep guards 8, 9)
- [ ] NO edit to README.md, how-it-works.md, configuration.md, overview.md, windows-test-support.md, any
      `*.go` / `*_test.go`, any `spec/*.md` (AGENTS.md rule #2), PRD.md, tasks.json, prd_snapshot.md, go.mod

### Code Quality & Docs
- [ ] The paragraph matches the surrounding cli.md voice (em-dashes, inline code, cross-ref link style)
- [ ] Every claim in the paragraph is backed by the code (orphan_unix.go: `ppid == 1`; arm_unix.go: watchdog;
      cmd/lock.go: the `orphaned:` output strings)
- [ ] The paragraph is self-contained: a user reading ONLY it understands what `orphaned: true/false/unknown`
      means, the subreaper gap, and that the watchdog is the backstop

---

## Anti-Patterns to Avoid

- ❌ Don't conflate the orphan HINT with the WATCHDOG. The hint (`appearsOrphaned`) tests `ppid == 1`
  (reparented to init); the watchdog (`arm_unix.go`) tests parent-pid-CHANGE. cli.md:400's CURRENT wording
  attributes "parent pid changed" to the hint — that is the bug. Credit the watchdog with parent-pid-change;
  describe the hint as ppid==1.
- ❌ Don't claim the hint catches all orphans. Under a subreaper (systemd/Docker/supervisord) a reparented
  holder's ppid is the subreaper's pid (≠1) ⇒ the hint reads `false`. State this display-only false-negative
  (T3 documented it in the code comment; mirror it briefly here).
- ❌ Don't edit the output EXAMPLE block (cli.md:388–398) or the command description (cli.md:387). They already
  match `internal/cmd/lock.go` byte-for-byte. Edit ONLY the prose paragraph BELOW the example.
- ❌ Don't edit README.md or how-it-works.md. The review FOUND them accurate (README:360/406/408; how-it-works
  211/220/222/224/228). "Improving" them widens scope and fails the grep guards. One file: docs/cli.md.
- ❌ Don't autonomously edit the spec/PRD (AGENTS.md rule #2). FR-K2 in the spec is correct; the bug is in the
  user doc's explanation, not the spec. Don't touch spec/SPEC.md, spec/*.md, PRD.md, tasks.json, prd_snapshot.
- ❌ Don't add a CLI/runtime note to `lock status` output. This is a DOC edit, not a code change. The status
  output (`internal/cmd/lock.go`) is UNCHANGED — the example already matches it. (T3's Mode-A contract
  forbade a user-facing runtime note; this task's user-facing change is the DOC PARAGRAPH, not the output.)
- ❌ Don't drop the how-it-works.md cross-reference. It is the load-bearing pointer that tells the user where
  the authoritative (subreaper-safe) watchdog is explained. Verify the `#per-repo-run-lock-fr52` anchor
  resolves before relying on it.
- ❌ Don't introduce content the code can't back. Every clause in the rewritten paragraph must be derivable
  from orphan_unix.go (`ppid == 1`), arm_unix.go (watchdog = parent-pid-change), and cmd/lock.go (the output
  strings). No aspirational claims.
- ❌ Don't treat "no changes needed" as the outcome without checking. The fixes are code-only and introduced
  no doc drift, BUT cli.md:400 is a genuine pre-existing imprecision surfaced by the review (it references
  the orphan hint and doesn't match the `ppid == 1` code). Refine it; that is the deliverable.

---

**Confidence Score: 9/10** for one-pass success. The edit is a single markdown paragraph with the verbatim
current text, the verbatim replacement, the exact code facts that justify each clause, the markdownlint
config, and exhaustive grep guards. The only residual risk is the cross-ref anchor (`#per-repo-run-lock-fr52`)
needing verification — Guard in Level 3/4 catches it.