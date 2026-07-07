---
name: "P1.M5.T3.S1 — Verify documentation internal consistency & unified 'stagecoach' identity"
description: |

  THIS IS A VERIFICATION / MODE-B DOCUMENTATION-SYNC TASK. It is the rename's (stagehand→stagecoach,
  PRD h2.30) cross-cutting coherence gate over the USER-FACING DOCUMENTATION surface. It verifies — by
  grepping the live files, not by asserting — that the renamed project presents ONE unified, internally-
  consistent `stagecoach` identity across README.md, docs/*.md, .goreleaser.yaml, go.mod, and the code's
  user-visible string literals, with ZERO stale `stagehand` leakage and ZERO cross-surface disagreement.

  ⚠️ HEADLINE RESEARCH FINDING (executed in the live repo 2026-07-07): THE PROJECT IS ALREADY
  INTERNALLY CONSISTENT. The rename's M3 (build/CI) + M4 (docs) passes — governed by the P1.M3.T1.S2
  "namespace decision tree" (canonical org = `dustin/`, NOT `dabstractor/`) — left every one of the
  contract's 7 checks (a–g) PASSING on internal consistency. This task is therefore primarily a
  VERIFICATION that gathers deterministic grep EVIDENCE for each check, RESOLVES the one open product
  question (the namespace — see §1), and FLAGS the pre-release action items (remote reconciliation,
  install.sh, taps/buckets). It is NOT a bulk-edit task; the only edits are targeted straggler-fixes IF
  re-verification finds drift (research found: none blocking).

  CONTRACT (item_description §1–§5; PRD h2.30; PRD §21.2/§21.3 install paths; PRD §21.5 README surface):
    1. RESEARCH NOTE: "See critical_findings.md F9. After the bulk rename, the README badge URLs,
       goreleaser GitHub URLs, and install instructions must all be internally consistent. The git remote
       is `dabstractor/stagecoach`. The go.mod module is `github.com/dustin/stagecoach`. These may
       conflict — verify the canonical GitHub path."
    2. INPUT: "The fully renamed and tested project (all preceding subtasks complete)."
    3. LOGIC: (a) badge URL = actual GitHub repo URL; (b) `go install` = go.mod module path;
       (c) goreleaser URLs = README badge URL; (d) docs/*.md cross-refs consistent (no old names);
       (e) lazygit example: `command: 'stagecoach'` + marker `# stagecoach-integration`;
       (f) git alias = `git stagecoach`; (g) `.git/stagecoach_EDITMSG` (not `stagehand_EDITMSG`).
    4. OUTPUT: "The project presents a unified, internally-consistent 'stagecoach' identity across all
       user-facing surfaces."
    5. DOCS: "Mode B — this is the cross-cutting changeset-level documentation sync. It depends on ALL
       implementing subtasks and verifies the whole delta is coherent."

  ⚠️ THE NAMESPACE ANSWER (the contract's central open question — RESOLVED here, see §"Namespace
     decision tree"): the canonical org is `dustin/`, NOT `dabstractor/`. THREE independent sources
     mandate `dustin/`: (a) go.mod = `github.com/dustin/stagecoach`; (b) PRD §21.2/§21.3 install
     commands; (c) .goreleaser.yaml's explicit `release.github.owner: dustin` ("WINS over git-remote
     auto-detect"). The git remote `dabstractor/stagecoach` is an EXTERNAL fact (a pre-release action
     item), NOT an internal-doc inconsistency. DO NOT change the docs/goreleaser to `dabstractor/` in
     this task — that would break go.mod/install/PRD consistency.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT touch unless re-verification finds a real straggler):
    - README badge-URL RESOLUTION / brew-tap & scoop-bucket EXISTENCE / `install.sh` publication →
      sibling P1.M5.T3.S2 ("Verify badge URLs, GitHub links, and distribution paths are correct"). S1
      verifies (a)/(b)/(c) at the CONSISTENCY level (badge = go install = go.mod = goreleaser?); S2
      verifies them at the EXTERNAL level (do the URLs actually resolve? do the taps exist?). Do not
      duplicate S2's external checks; reference them.
    - The git REMOTE (`dabstractor/stagecoach`) → a pre-release action item; NOT changed by S1.
    - PRD.md → READ-ONLY (research agent never modifies; the lowercase `stagecoach_EDITMSG` in PRD §9.22
      is loose spec prose, out of scope — see §"EDITMSG casing").
    - plan/012_*, **/tasks.json, prd_snapshot.md → orchestrator-owned; never touch.
    - Go source (the EDITMSG path, finalize.go:78) → already correct (`STAGECOACH_EDITMSG`,
      git-conventional UPPERCASE). Do NOT change its casing.
    - P1.M5.T2.S2 (in-flight, parallel): build/test verification — touches Go, not docs. No file overlap.
    - P1.M5.T1.S1 (Ready): bulk-rename `stagehand` in plan/ historical files — plan/ is NOT a user-facing
      surface and is S1's documented exception; S1 does not verify plan/.

  DELIVERABLE: a verification RECORD (implementation summary — Mode B; NO new doc file required because
  the docs already present a unified identity and a new file risks colliding with future doc-review
  work) demonstrating each of the 7 checks (a–g) executed with its grep evidence + expected output, the
  namespace decision documented, and the pre-release action items flagged. PLUS a scoped locate-and-fix
  IF (and only if) re-verification finds a real straggler (research found: none blocking today). NO new
  files unless an inconsistency is found.

  SUCCESS: all 7 checks (a–g) PASS with grep evidence; ZERO `stagehand` in the tracked user-facing +
  config surface (README.md, docs/, .goreleaser.yaml, .golangci.yml, Makefile, go.mod); the namespace is
  confirmed `dustin/stagecoach` and the remote-mismatch is documented as a pre-release gate; the
  EDITMSG casing non-issue is recorded (not "fixed"); `git status` shows at most targeted straggler
  fixes (likely none).

---

## Goal

**Feature Goal**: Certify — by executing deterministic greps against the live files, not by asserting —
that the renamed `stagecoach` project presents ONE unified, internally-consistent identity across every
user-facing documentation surface: README.md, docs/*.md, .goreleaser.yaml, go.mod, and the code's
user-visible literals. Specifically, prove each of the contract's 7 checks (a–g) holds: the GitHub
namespace is internally consistent (badge = go install = go.mod = goreleaser), zero `stagehand`
stragglers remain, the lazygit/git-alias/EDITMSG examples all say `stagecoach`, and the one open product
question — the canonical GitHub org — is resolved (`dustin/`) with the remote mismatch flagged.

**Deliverable**: A verification record (implementation summary — Mode B; no new doc file) showing each
of the 7 checks (a–g) executed with its precise grep command + expected output, the namespace decision
documented, and the pre-release action items (remote reconciliation, `install.sh`, taps/buckets) flagged
for release-time. A scoped locate-and-fix is applied ONLY if re-verification finds a real straggler
(research found: none blocking today).

**Success Definition**:
- Check (a): README badge URL, clone URL, and all install URLs use `dustin/stagecoach`; confirmed
  CONSISTENT with go.mod and goreleaser. The remote `dabstractor/stagecoach` mismatch is documented as a
  pre-release action item (NOT "fixed" by changing the docs).
- Check (b): `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` in README + docs/README.md
  exactly matches the go.mod module path `github.com/dustin/stagecoach`.
- Check (c): every GitHub URL in .goreleaser.yaml (owner, homepages ×4, url_template) is
  `github.com/dustin/stagecoach`, matching the README badge.
- Check (d): `git grep -il stagehand -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod`
  → ZERO files; every docs cross-reference target file exists.
- Check (e): the lazygit example uses `command: 'stagecoach'` and the marker `# stagecoach-integration`
  (in README.md AND docs/cli.md).
- Check (f): the git alias example uses `git stagecoach` / `alias.stagecoach '!stagecoach'` (README + cli.md).
- Check (g): the code references `STAGECOACH_EDITMSG` (not `stagehand_EDITMSG`); zero `stagehand_EDITMSG`
  anywhere; the casing-vs-PRD non-issue is recorded (not changed).
- `git status` shows at most targeted straggler fixes (research found none); no new files created unless
  an inconsistency is discovered.

## User Persona

**Target User**: the maintainer certifying the rename before the first `stagecoach` tag, and the release
engineer who needs the README/docs/goreleaser to agree with go.mod before publishing. Secondary: any new
user who reads the README and runs an install command — a badge pointing at a dead URL, or a `go install`
that 404s, or a stale `stagehand` reference, would immediately erode trust.

**Use Case**: "After renaming stagehand→stagecoach, does the project look like ONE coherent product, or
are there leftover contradictions a user or the release tooling will trip on?" The maintainer runs the 7
grep checks, eyeballs the namespace, and either ships (all green) or applies a one-line straggler fix.
The most consequential trap: the git remote is `dabstractor/stagecoach` while everything else says
`dustin/stagecoach` — this task resolves that deliberately (canonical = `dustin`) rather than reflexively.

**Pain Points Addressed**: a rename can be "done" at the source level yet still leak the old name in a
README install snippet, a goreleaser URL, a lazygit marker, or an EDITMSG path — or, conversely, can
half-rename the namespace (some files `dustin`, some `dabstractor`). This task is the proof that the
rename reached every user-facing surface and that the surfaces agree with each other and with go.mod.

## Why

- **It is the rename's documentation close-out gate (PRD h2.30), complementary to the build/test gate
  (P1.M5.T2.S2).** S2 certifies the project BUILDS and its BINARY presents the right identity; S1
  certifies the project's DOCS + release config present the right identity and agree with each other and
  with go.mod. Both are needed; neither subsumes the other.
- **Resolves the namespace question the contract explicitly raises.** "verify the canonical GitHub path"
  is not a grep — it is a product decision. This task makes the decision explicit (`dustin/`, with three
  mandates), documents it, and flags the pre-release remote reconciliation so it isn't discovered at tag
  time. (F9's research suggestion of `dabstractor/` was already overridden by the goreleaser PRP; S1
  confirms that override holds end-to-end across the docs.)
- **Catches the stragglers a green build cannot.** `go build` succeeding says nothing about whether a
  README install snippet still says `stagehand`, or a lazygit marker still says `stagehand-integration`,
  or a goreleaser URL points at the old org. The 7 greps are the net that catches those.
- **Establishes the rename's documentation regression baseline.** The verification commands + expected
  outputs become the re-runnable check the next maintainer uses after any future doc change.

## What

Run, observe, and assert on the 7 contract checks (a–g), recording grep evidence. The complete
certification sequence (each verified PASSING in research — re-run at implementation time):

1. **(a) Namespace consistency** — confirm README badge/clone/install URLs, go.mod, and goreleaser ALL
   use `dustin/stagecoach`; record the remote (`dabstractor/stagecoach`) as a pre-release action item.
2. **(b) go install == go.mod** — `github.com/dustin/stagecoach/cmd/stagecoach@latest` (README +
   docs/README.md) == go.mod module path.
3. **(c) goreleaser URLs == badge** — owner `dustin`, homepages + url_template all `dustin/stagecoach`.
4. **(d) docs cross-refs + zero stragglers** — `git grep -il stagehand` over the user-facing surface →
   clean; every referenced docs file exists.
5. **(e) lazygit example** — `command: 'stagecoach'` + `# stagecoach-integration` (README + cli.md).
6. **(f) git alias** — `git stagecoach` / `alias.stagecoach '!stagecoach'` (README + cli.md).
7. **(g) EDITMSG** — code uses `STAGECOACH_EDITMSG`; zero `stagehand_EDITMSG`; record the casing non-issue.

If re-verification finds a real straggler (a check that FAILS), apply the minimal targeted fix to the
offending file (the rename rule is `stagehand`→`stagecoach`, preserving the `dustin/` org). Research
found none blocking; the most likely drift sources are listed in §"Known Gotchas".

### Success Criteria

- [ ] All 7 checks (a–g) PASS with their grep commands producing the expected output (see §"Validation
      Loop" for the exact commands + expected results).
- [ ] `git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod` →
      ZERO files.
- [ ] The namespace is confirmed `dustin/stagecoach` end-to-end (badge = go install = go.mod =
      goreleaser = docs). The remote mismatch (`dabstractor`) is documented as a pre-release action item.
- [ ] The EDITMSG casing (code `STAGECOACH_EDITMSG` vs PRD lowercase) is recorded as a verified
      non-issue — NOT "fixed".
- [ ] Scope respected: S2's external checks (URL resolution, tap/bucket existence, install.sh) are
      referenced, not duplicated; PRD.md and Go source are not modified.

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the namespace decision tree
(§"Namespace decision tree" — THE central question, resolved); the 7-check evidence table with exact
file:line references and expected greps (§"Implementation Blueprint"); the EDITMSG casing non-issue
(§"EDITMSG casing"); the S1/S2 scope split (§"Integration Points"); and the re-runnable verification
commands (§"Validation Loop"). The only genuinely uncertain input (whether any check has drifted since
research) is handled by requiring a FRESH re-run of every grep before declaring PASS.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/012_963e3918ec08/P1M5T3S1/research/findings.md
  why: THE decisive doc. §1 the namespace decision (dustin canonical; dabstractor remote = pre-release)
       with the 3 mandates + the F9-override history; §2 the 7-check current-state MATRIX (file:line
       evidence, all PASS); §3 the EDITMSG casing non-issue; §4 the S1/S2 scope split; §6 the re-runnable
       grep commands; §7 the pre-release action items.
  critical: §1 (namespace), §2 (the 7-check matrix), §3 (do NOT "fix" EDITMSG casing).

- docfile: plan/012_963e3918ec08/architecture/critical_findings.md   (F9 + F10)
  why: F9 (README badge URL referenced the OLD GitHub path) is the contract's cited research note; F10
       (.gitignore stagehand entries) confirms the config surface was renamed. F9 SUGGESTED dabstractor/;
       the goreleaser PRP OVERRADED it to dustin/. S1 confirms the override holds across docs.
  section: F9 (L72-75), F10 (L76-80).

- file: .goreleaser.yaml   (P1.M3.T1.S2 — READ only; the namespace decision is DOCUMENTED here)
  section: the owner-note comment (L5-8) + `release.github.owner: dustin` (L60-62, "WINS over git-remote
           auto-detect") + homepages (L80,92,106) + url_template (L95) + brew/scoop repos (L78,90).
  why: this is where the canonical-org decision is recorded and where every release URL lives. S1 verifies
       all of these are `dustin/stagecoach` (consistent with README badge + go.mod).
  pattern: the comment block (L5-8) is the authoritative statement of the namespace decision + the
           pre-release reconciliation requirement — quote it in the verification record.
  gotcha: do NOT change `owner: dustin` to `dabstractor` — that would break go.mod/install/PRD consistency.

- file: README.md   (P1.M4.T1.S1 — READ only; the primary user-facing surface)
  section: badge (L10), clone (L95), install block (L113-116), lazygit+alias section (L181-201), the
           FUTURE_SPEC.md + docs/*.md cross-links.
  why: checks (a)(b)(e)(f) live here. Verify the badge/clone/install use `dustin/stagecoach`; the lazygit
       example has `command: 'stagecoach'` + `# stagecoach-integration`; the alias is `git stagecoach`.

- file: docs/README.md + docs/cli.md   (P1.M4.T1.S1/S2 — READ only; the docs index + CLI reference)
  section: docs/README.md install block (L14-23) + the "binary is authoritative" note (L11); docs/cli.md
           lazygit target (L293-342), git-alias target (L260-282), the EDITMSG generic mention (L42),
           and `.git/COMMIT_EDITMSG` (L128 — git's OWN file, correct).
  why: checks (b)(d)(e)(f)(g) cross-check against docs. The lazygit/alias EXAMPLES in docs must match
       README's (both stagecoach).

- file: go.mod   (READ only; line 1)
  why: the module path `github.com/dustin/stagecoach` is the ORACLE for check (b) and the primary mandate
       for the `dustin/` namespace decision.

- file: internal/generate/finalize.go + internal/git/git.go   (READ only; the EDITMSG runtime value)
  section: finalize.go:78 (`filepath.Join(gitDir, "STAGECOACH_EDITMSG")`), git.go:390 (the comment).
  why: check (g) — the code writes `STAGECOACH_EDITMSG` (UPPERCASE, git-conventional). This is the SOURCE
       OF TRUTH. It is correct; do NOT change it. The PRD's lowercase `stagecoach_EDITMSG` is loose prose.
  gotcha: do NOT lowercase the code to match PRD — uppercase matches git's own convention (COMMIT_EDITMSG,
          MERGE_MSG, TAG_EDITMSG) and is the runtime value the binary writes.

- docfile: plan/012_963e3918ec08/P1M3T1S2/PRP.md   (the goreleaser PRP — READ only; established the decision)
  section: the "THE FIX — ⚠️ #1 — KEEP `dustin/` as the org" block (L19-28).
  why: this is the authoritative record that `dustin/` was chosen DELIBERATELY (3 mandates) over F9's
       `dabstractor/` suggestion. S1 inherits this decision; it does not relitigate it.

- url: (PRD internal) PRD.md h2.30 (the rename directive), §21.2 (goreleaser / brew tap `dustin/tap`),
       §21.3 (install paths: brew/scoop/go install all `dustin/stagecoach`), §21.5 (README structure),
       §9.22 FR-E1 (the `--edit` / `.git/stagecoach_EDITMSG` spec).
  why: authoritative spec for the rename scope, the install-path namespace, and the EDITMSG feature.
```

### Current Codebase tree (relevant slice)

```bash
README.md                    # P1.M4.T1.S1 — badge, install, lazygit/alias examples. The primary surface. (verify, not edit unless straggler)
docs/                        # P1.M4.T1.S1/S2 — the docs index + references. (verify)
  README.md                  #   docs index; install block; "binary is authoritative" note.
  cli.md                     #   lazygit/git-alias targets; EDITMSG generic mention; COMMIT_EDITMSG (git's own).
  configuration.md           #   cross-refs to providers.md.
  how-it-works.md            #   cross-refs to providers.md/configuration.md/cli.md.
  providers.md               #   provider docs.
.goreleaser.yaml             # P1.M3.T1.S2 — namespace decision (owner: dustin) + all release URLs. (verify)
.golangci.yml                # P1.M3.T1.S3 — path ref pkg/stagecoach/... (S1 grep-audit fixed it). (verify clean)
go.mod                       # P1.M1.T1.S1 — module github.com/dustin/stagecoach. THE namespace oracle. (verify)
internal/generate/finalize.go# the EDITMSG runtime value: STAGECOACH_EDITMSG (L78). (verify; do NOT change)
internal/git/git.go          # the EDITMSG comment (L390). (verify)
FUTURE_SPEC.md               # P1.M4.T2.S2 — referenced by README + docs/cli.md. (verify it exists)
# plan/012_*, PRD.md, .pi-subagents/ → NOT user-facing; documented exceptions; do NOT verify/modify here.
```

### Desired Codebase tree with files to be added

```bash
# NO new files by default (Mode B verification; docs already present a unified identity). The deliverable
# is a verification RECORD (implementation summary). IF re-verification finds a real straggler, apply the
# minimal one-line fix to the offending file (stagehand→stagecoach, preserving dustin/ org). Research
# found: none blocking. Expected `git status` after a clean verification: no changes (or ≤ a few straggler fixes).
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (THE NAMESPACE — do NOT reflexively "fix" the remote mismatch). The git remote is
# `dabstractor/stagecoach` but EVERYTHING user-facing + go.mod + goreleaser uses `dustin/stagecoach`.
# The canonical org is `dustin/` (3 mandates: go.mod, PRD §21.2/21.3, goreleaser owner). DO NOT change
# the docs/goreleaser to `dabstractor/`. The remote mismatch is a PRE-RELEASE ACTION ITEM (make the repo
# reachable at github.com/dustin/stagecoach before the first tag). F9 suggested dabstractor/; the
# goreleaser PRP overrode it to dustin/; S1 confirms the override holds. See §"Namespace decision tree".

# CRITICAL (EDITMSG CASING — do NOT "fix" it). The code writes STAGECOACH_EDITMSG (UPPERCASE) at
# finalize.go:78; PRD §9.22 + the contract write stagecoach_EDITMSG (lowercase). Uppercase is CORRECT —
# it matches git's own convention (COMMIT_EDITMSG, MERGE_MSG, TAG_EDITMSG). Lowercasing the code would be
# non-idiomatic. No user-facing doc names the file, so there is nothing in docs to "correct". Record this
# as a verified non-issue; do NOT edit the code or PRD.

# CRITICAL (CHECK (g) IS ABOUT OLD-NAME LEAKAGE, NOT CASING). The contract's check (g) says "Verify
# .git/stagecoach_EDITMSG is referenced correctly (not .git/stagehand_EDITMSG)". The real test: ZERO
# `stagehand_EDITMSG` anywhere. The code uses `STAGECOACH_EDITMSG` (stagecoach, correct). PASS. The
# lowercase-vs-uppercase is a cosmetic spec-vs-impl difference, documented above, not a check failure.

# GOTCHA (RE-VERIFY FRESH — research is a snapshot). The 7 checks PASS as of 2026-07-07, but
# P1.M5.T2.S2 (build/test) runs in parallel and could, in principle, touch a doc if it finds a straggler.
# Re-run EVERY grep at implementation time; do not trust the research table's "PASS" without re-confirming.

# GOTCHA (THE 7 GREPS EXCLUDE plan/ AND .pi-subagents/). Historical plan/ files (plan/001..011) and
# .pi-subagents/ artifacts DO contain `stagehand` (and `STAGEHAND_EDITMSG`) — they are pre-rename research
# records, NOT user-facing, NOT shipped. They are P1.M5.T1.S1's scope (Ready) and S1's documented
# exception. Scope your greps to the user-facing + config surface only (README, docs/, .goreleaser.yaml,
# .golangci.yml, Makefile, go.mod). `git grep -il stagehand -- ':!plan/' ':!.pi-subagents/'` is the
# whole-repo straggler check if you want a broader net (expect plan/ hits, which are out of scope).

# GOTCHA (install.sh IS EXPECTED TO BE ABSENT pre-release). README + docs reference
# `…/raw/main/install.sh`, but the file does not exist yet (published with the first release; docs/README
# .md:27 says so). This is NOT a broken cross-ref for S1's purposes — it is a documented pre-release gap.
# The EXTERNAL "does install.sh exist / resolve" check is S2's territory; S1 only notes its absence.

# GOTCHA (DOCS ANCHOR LINKS use GitHub's heading→slug mapping). README links to e.g.
# docs/cli.md#lazygit-target (from heading `#### \`lazygit\` target`) and docs/how-it-works.md#multi-
# commit-decomposition. A heading rename during M4 could silently break an anchor. Verify a SAMPLE of the
# most-referenced anchors resolve (the heading exists). Exhaustive anchor auditing is S2's external-link
# scope; S1 spot-checks the lazygit/alias/decomposition anchors since they're in the rename's hot path.

# GOTCHA (S2 SCOPE OVERLAP — coordinate). P1.M5.T3.S2 ("Verify badge URLs, GitHub links, distribution
# paths") is a NARROWER, EXTERNAL check. S1 = internal consistency (a–g, surfaces agree); S2 = external
# correctness (URLs resolve, taps/buckets exist, install.sh published). Do NOT duplicate S2's external
# checks; reference them. If S2 is run first, consume its findings; if not, S1 flags the external items.
```

#### Namespace decision tree (resolves the contract's central question)

```
THE QUESTION (contract §1): git remote = dabstractor/stagecoach; go.mod = github.com/dustin/stagecoach.
"These may conflict — verify the canonical GitHub path."

THE ANSWER: canonical org = `dustin/`. The remote is a pre-release action item, NOT an internal-doc bug.

MANDATES (3 independent sources, all = dustin/):
  (a) go.mod:1                         module github.com/dustin/stagecoach   (the go-install path)
  (b) PRD §21.2/§21.3                  brew install dustin/tap/stagecoach; scoop install dustin/stagecoach;
                                      go install github.com/dustin/stagecoach/cmd/stagecoach@latest
  (c) .goreleaser.yaml:60-62           release.github.owner: dustin  ("WINS over git-remote auto-detect")

VERIFIED CONSISTENT (S1 confirms end-to-end): README badge/clone/install (dustin), docs/README install
(dustin), goreleaser owner+homepages+url_template (dustin), go.mod (dustin) → all agree.

THE F9 OVERRIDE: critical_findings.md F9 SUGGESTED the URLs become dabstractor/ (to match the remote).
The P1.M3.T1.S2 goreleaser PRP OVERRADED that — preserving dustin/ — because changing to dabstractor/
would make goreleaser INCONSISTENT with go.mod + the PRD install paths. S1 confirms the override holds
across the docs (not just goreleaser).

ACTION (do NOT change docs to dabstractor/): flag the remote reconciliation as a PRE-RELEASE GATE —
before the first REAL tag, make the repo reachable at github.com/dustin/stagecoach (rename/transfer/
mirror the GitHub repo), OR reconcile the namespace repo-wide to dabstractor/ (heavier: change go.mod +
every doc + goreleaser; contradicts the 3 mandates). Until then the badge points at a not-yet-live URL.
This is already documented in .goreleaser.yaml:5-8.
```

#### EDITMSG casing (verified non-issue — record, do not change)

```
CODE (runtime truth):  internal/generate/finalize.go:78   filepath.Join(gitDir, "STAGECOACH_EDITMSG")
                       internal/git/git.go:390             comment "…to locate .git/STAGECOACH_EDITMSG"
PRD spec (loose prose): PRD.md:485 (FR-E1)                ".git/stagecoach_EDITMSG"   (lowercase)
CONTRACT check (g):     "Verify .git/stagecoach_EDITMSG…"  (lowercase)

WHY UPPERCASE IS CORRECT: git's own message files are ALL uppercase (COMMIT_EDITMSG, MERGE_MSG,
TAG_EDITMSG). The implementation follows git's convention. Lowercasing would be non-idiomatic and would
diverge from the ecosystem.

WHY NOTHING IN DOCS NEEDS CHANGING: no user-facing doc names the EDITMSG file. docs/cli.md:42 describes
the `--edit` feature generically ("the EDITMSG file includes the tree SHA…"); docs/cli.md:128 references
git's OWN `.git/COMMIT_EDITMSG` (a different, correctly-named file, for the hook-exec context).

VERDICT: check (g) PASSES — zero `stagehand_EDITMSG` anywhere; the code uses the correct `stagecoach`
name (uppercase). Record the casing-vs-PRD difference as a documented non-issue. DO NOT edit the code
(would break git convention) or PRD.md (read-only).
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: ORIENT + confirm the namespace decision (READ + RUN, no edit)
  - RUN: `git remote -v | grep origin` -> expect `dabstractor/stagecoach` (the remote).
  - RUN: `head -1 go.mod` -> expect `module github.com/dustin/stagecoach`.
  - READ: .goreleaser.yaml:5-8 + :60-62 (the owner note + `release.github.owner: dustin` "WINS over
    git-remote auto-detect"). This is the authoritative namespace decision. CONFIRM canonical = `dustin/`.
  - READ: plan/012_963e3918ec08/P1M5T3S1/research/findings.md §1 (the decision tree) + §2 (the 7-check
    matrix with file:line evidence). These are the research snapshot to re-confirm, not to trust blindly.

Task 1: VERIFY checks (a)+(b)+(c) — the namespace consistency (RUN, assert)
  - (a) badge = clone = install = go.mod = goreleaser = `dustin/stagecoach`:
      grep -n 'github.com/dustin/stagecoach' README.md        # badge L10, clone L95, install L113-116
      grep -n 'github.com/dustin/stagecoach' docs/README.md   # install block L14-23
      grep -c 'github.com/dustin/stagecoach' .goreleaser.yaml # expect >=4 (owner note + homepages + url_template)
  - (b) go install == go.mod:
      grep -o 'github.com/dustin/stagecoach' go.mod                                                # expect 1
      grep -c 'go install github.com/dustin/stagecoach/cmd/stagecoach@latest' README.md docs/README.md  # expect 2
  - (c) goreleaser URLs == badge:
      grep -nE 'owner: dustin|homepage: https://github.com/dustin/stagecoach|url_template.*dustin/stagecoach' .goreleaser.yaml
  - ASSERT: every hit is `dustin/stagecoach`; NONE is `dabstractor` or `stagehand`. Record the remote
    (`dabstractor/stagecoach`) as a pre-release action item (do NOT change the docs).
  - IF a straggler is found (a `stagehand` or `dabstractor` URL in these files): apply the minimal
    `stagehand`→`stagecoach` fix (preserve `dustin/`). Research found: none.

Task 2: VERIFY check (d) — docs cross-refs + zero stragglers (RUN, assert)
  - `git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod`
      -> expect ZERO files (clean). If any file matches, it's a real straggler -> fix it.
  - Referenced-files existence (check the cross-link targets resolve to real files):
      for f in docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md FUTURE_SPEC.md; do test -e "$f" && echo "OK $f" || echo "MISS $f"; done
      -> expect all OK (FUTURE_SPEC.md exists; install.sh is intentionally absent pre-release — note it).
  - Spot-check 2-3 docs anchor links resolve (heading exists): e.g. docs/cli.md#lazygit-target (heading
    `#### \`lazygit\` target` at cli.md:293), docs/how-it-works.md#multi-commit-decomposition.
      grep -nE '^#{1,6} .*lazygit.?target|^#{1,6} multi-commit decomposition' docs/cli.md docs/how-it-works.md

Task 3: VERIFY check (e) — lazygit example (RUN, assert)
  - `grep -n "command: 'stagecoach'" README.md docs/cli.md`        -> expect 2 matches (README:201, cli.md:301).
  - `grep -n '# stagecoach-integration' README.md docs/cli.md`     -> expect >=2 matches (README:199, cli.md:299).
  - ASSERT: BOTH the command AND the marker say `stagecoach` (not `stagehand`/`stagehand-integration`).

Task 4: VERIFY check (f) — git alias (RUN, assert)
  - `grep -rn "git stagecoach" README.md docs/cli.md`              -> expect matches (README:72,185; cli.md:262,282).
  - `grep -n "alias.stagecoach" docs/cli.md`                       -> expect `alias.stagecoach '!stagecoach'` (cli.md:262).
  - ASSERT: the alias is `stagecoach`, value `!stagecoach` (not `stagehand`).

Task 5: VERIFY check (g) — EDITMSG (RUN, assert; record non-issue, do NOT change code)
  - `grep -rn 'STAGECOACH_EDITMSG' internal/`                     -> expect finalize.go:78, git.go:390.
  - `git grep -il 'stagehand_EDITMSG' -- ':!plan/' ':!.pi-subagents/'`  -> expect NO output (clean repo-wide,
    excluding the historical plan/ + artifact dirs which legitimately contain the old name).
  - ASSERT: the code uses `STAGECOACH_EDITMSG` (stagecoach, uppercase, git-conventional); zero
    `stagehand_EDITMSG` in the shipped surface. RECORD the casing-vs-PRD non-issue (do NOT edit code/PRD).

Task 6: RECORD + flag (the deliverable)
  - Record the verification outcome in the implementation summary: each of the 7 checks (a–g) with its
    grep command + observed output + PASS/FAIL. This is the Mode-B documentation-sync artifact.
  - Document the namespace decision (canonical `dustin/`; 3 mandates; the F9 override).
  - Flag the pre-release action items (remote reconciliation; install.sh publication; dustin/homebrew-tap
    + dustin/scoop-bucket repo creation) — these are NOT fixed here; they're release-time gates (S2 /
    release engineer territory).
  - `git status --short` -> expect no changes (clean verification) or ≤ a few targeted straggler fixes.
    NO new files unless an inconsistency was found.
```

### Implementation Patterns & Key Details

```bash
# The 7-check verification is a suite of deterministic greps. Each has a precise expected output.
# Run them FRESH (research is a snapshot); assert each matches; fix only real stragglers.

# (a) namespace — every user-facing GitHub path is dustin/stagecoach (canonical); remote is dabstractor.
git remote -v | grep origin                                   # dabstractor/stagecoach (the remote; PRE-RELEASE flag)
grep -n 'github.com/dustin/stagecoach' README.md | head -1   # badge URL (the canonical)

# (b) go install == go.mod
grep -o 'github.com/dustin/stagecoach' go.mod                                               # 1 match
grep -c 'go install github.com/dustin/stagecoach/cmd/stagecoach@latest' README.md docs/README.md  # 2

# (c) goreleaser URLs == badge (all dustin/stagecoach)
grep -c 'github.com/dustin/stagecoach' .goreleaser.yaml                                      # >=4

# (d) zero stagehand in the user-facing+config surface; cross-ref targets exist
git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod   # (no output)

# (e) lazygit: command 'stagecoach' + marker # stagecoach-integration
grep -n "command: 'stagecoach'" README.md docs/cli.md        # 2 matches
grep -n '# stagecoach-integration' README.md docs/cli.md     # >=2 matches

# (f) git alias = 'git stagecoach'
grep -rn "git stagecoach" README.md docs/cli.md              # matches
grep -n "alias.stagecoach" docs/cli.md                       # '!stagecoach'

# (g) EDITMSG: STAGECOACH_EDITMSG in code; ZERO stagehand_EDITMSG repo-wide (excl plan/ + artifacts)
grep -rn 'STAGECOACH_EDITMSG' internal/                                  # finalize.go:78, git.go:390
git grep -il 'stagehand_EDITMSG' -- ':!plan/' ':!.pi-subagents/'         # (no output)

# TARGETED STRAGGLER FIX (only if a check FAILS — research found none):
#   the rename rule is `stagehand` -> `stagecoach`, PRESERVING the `dustin/` org. Use a scoped sed on
#   the SINGLE offending file, never a repo-wide sed. Example (hypothetical, do not run unless needed):
#   sed -i 's/stagehand-integration/stagecoach-integration/g' README.md
#   After ANY fix, re-run the full 7-check suite to confirm green, then `git status` to confirm scope.
```

### Integration Points

```yaml
DOCUMENTATION (the unified identity — verified, not edited unless a straggler is found):
  - README.md, docs/*.md, .goreleaser.yaml all present `stagecoach` consistently. No new doc file is
    created (Mode-B verification; a redundant doc risks colliding with future doc-review work). The
    deliverable is the verification RECORD (implementation summary).

NAMESPACE (the contract's open question — RESOLVED):
  - canonical org = `dustin/` (go.mod + PRD §21.2/21.3 + goreleaser owner). The git remote
    (`dabstractor/stagecoach`) is a PRE-RELEASE ACTION ITEM: the repo must be reachable at
    github.com/dustin/stagecoach before the first real tag. Flagged, not fixed.

SCOPE HANDOFFS (do NOT duplicate):
  - P1.M5.T3.S2 (sibling, Planned): EXTERNAL verification — badge URL RESOLUTION, brew-tap/scoop-bucket
    EXISTENCE, install.sh PUBLICATION, GitHub anchor-link resolution. S1 = internal consistency (a–g,
    surfaces agree); S2 = external correctness (URLs resolve/taps exist). Reference S2; don't duplicate.
  - P1.M5.T2.S2 (in-flight, parallel): build/test identity verification. S1 cross-references its
    `--version`/`--help` result but does not re-verify the binary. No file overlap.
  - P1.M5.T1.S1 (Ready): bulk-rename stagehand in plan/ historical files. plan/ is NOT user-facing; S1's
    documented exception. No overlap.

READ-ONLY (never modify in this task):
  - PRD.md (the lowercase stagecoach_EDITMSG in §9.22 is loose prose, out of scope).
  - Go source (STAGECOACH_EDITMSG casing is correct; do NOT change).
  - go.mod (the namespace oracle; verified, not edited).
  - plan/012_*, tasks.json, prd_snapshot.md (orchestrator-owned).
```

## Validation Loop

### Level 1: Verification commands (the 7 checks — run FRESH, assert each)

```bash
cd <repo-root>   # /home/dustin/projects/stagehand
# Each command below has an EXPECTED output (from research, 2026-07-07). Re-run and confirm.

# (a) namespace consistency (badge=clone=install=go.mod=goreleaser=dustin/stagecoach; remote=dabstractor)
git remote -v | grep origin                                   # EXPECT: dabstractor/stagecoach (the remote)
grep -n 'github.com/dustin/stagecoach' README.md | head -1   # EXPECT: L10 badge URL
grep -c 'github.com/dustin/stagecoach' .goreleaser.yaml      # EXPECT: >=4

# (b) go install == go.mod
grep -o 'github.com/dustin/stagecoach' go.mod                                                # EXPECT: 1 match
grep -c 'go install github.com/dustin/stagecoach/cmd/stagecoach@latest' README.md docs/README.md  # EXPECT: 2

# (c) goreleaser URLs == badge
grep -nE 'owner: dustin|homepage: https://github.com/dustin/stagecoach' .goreleaser.yaml     # EXPECT: owner + >=1 homepage

# (d) zero stagehand in the user-facing+config surface; cross-ref targets exist
git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod    # EXPECT: no output
for f in docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md FUTURE_SPEC.md; do test -e "$f" && echo "OK $f" || echo "MISS $f"; done   # EXPECT: all OK

# (e) lazygit example: command 'stagecoach' + marker # stagecoach-integration
grep -n "command: 'stagecoach'" README.md docs/cli.md        # EXPECT: 2 matches
grep -n '# stagecoach-integration' README.md docs/cli.md     # EXPECT: >=2 matches

# (f) git alias = 'git stagecoach'
grep -rn "git stagecoach" README.md docs/cli.md              # EXPECT: matches
grep -n "alias.stagecoach" docs/cli.md                       # EXPECT: '!stagecoach'

# (g) EDITMSG: STAGECOACH_EDITMSG in code; ZERO stagehand_EDITMSG repo-wide (excl plan/ + artifacts)
grep -rn 'STAGECOACH_EDITMSG' internal/                                  # EXPECT: finalize.go:78, git.go:390
git grep -il 'stagehand_EDITMSG' -- ':!plan/' ':!.pi-subagents/'         # EXPECT: no output
# Expected: all 7 checks PASS (research snapshot). If any FAILS, apply the targeted straggler fix
# (stagehand->stagecoach, preserve dustin/) to the SINGLE offending file, then re-run the suite.
```

### Level 2: Cross-surface agreement (the "unified identity" proof)

```bash
# The single most important assertion: every GitHub path across ALL surfaces is the SAME string.
echo "=== badge / clone / install / goreleaser / go.mod — must all be dustin/stagecoach ==="
grep -rhoE 'github.com/(dustin|dabstractor)/stagecoach' README.md docs/README.md .goreleaser.yaml go.mod | sort | uniq -c
# EXPECT: a single line `N github.com/dustin/stagecoach` (N = total occurrences). ZERO dabstractor, ZERO stagehand.
# If two distinct orgs appear, the namespace is split -> reconcile to dustin/ (the canonical).

# Confirm the lazygit/alias EXAMPLES agree between README and docs (not just present in one):
diff <(grep -o "command: 'stagecoach'" README.md) <(grep -o "command: 'stagecoach'" docs/cli.md)   # EXPECT: identical (both present)
```

### Level 3: Straggler-fix validation (ONLY if a check failed in Level 1)

```bash
# After applying a targeted fix to an offending file, re-run the FULL 7-check suite (Level 1) and:
git status --short                       # EXPECT: only the file(s) you intentionally fixed
git diff --stat                          # review the diff is a pure stagehand->stagecoach rename
git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod   # EXPECT: clean now
# Expected: the failing check now PASSES; no unintended files changed; the fix preserved the dustin/ org.
```

### Level 4: Pre-release action-item flagging (the deliverable's release-gate section)

```bash
# These are NOT fixed by S1 — they are documented as release-time gates (S2 / release engineer).
# Confirm each is a known, documented gap (not a surprise):

# (1) remote reconciliation: remote is dabstractor/, canonical is dustin/
git remote -v | grep origin              # dabstractor/stagecoach — repo must be reachable at dustin/stagecoach pre-tag
grep -n 'dustin/stagecoach' .goreleaser.yaml | head -1   # the documented owner note (goreleaser.yaml:5-8)

# (2) install.sh absent pre-release (documented in docs/README.md:27)
test -e install.sh && echo "EXISTS" || echo "ABSENT (expected pre-release; published at first release)"

# (3) distribution taps/buckets must exist before goreleaser publishes (S2's external-check territory)
grep -nE 'homebrew-tap|scoop-bucket' .goreleaser.yaml   # the repos goreleaser will push to (dustin/homebrew-tap, dustin/scoop-bucket)
# Expected: all three flagged in the verification record as pre-release gates; none acted on by S1.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 7 checks (a–g) executed FRESH with their grep commands; each produced the expected output.
- [ ] `git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod` → clean.
- [ ] Cross-surface agreement (Level 2): a SINGLE `github.com/dustin/stagecoach` string across all surfaces.
- [ ] Any straggler fix (if needed) is a pure `stagehand`→`stagecoach` rename preserving `dustin/`, scoped to
      one file, re-verified green.

### Feature Validation

- [ ] (a) badge/clone/install = go.mod = goreleaser = `dustin/stagecoach`; remote mismatch documented.
- [ ] (b) `go install` path == go.mod module path (exactly).
- [ ] (c) goreleaser owner + all URLs == README badge URL.
- [ ] (d) zero `stagehand` in user-facing surface; all docs cross-ref targets exist.
- [ ] (e) lazygit: `command: 'stagecoach'` + `# stagecoach-integration` (README + cli.md).
- [ ] (f) git alias: `git stagecoach` / `alias.stagecoach '!stagecoach'` (README + cli.md).
- [ ] (g) EDITMSG: code `STAGECOACH_EDITMSG`; zero `stagehand_EDITMSG`; casing non-issue recorded.
- [ ] Namespace decision documented (canonical `dustin/`; F9 override; pre-release remote gate).
- [ ] Pre-release action items flagged (remote reconciliation; install.sh; taps/buckets).

### Code Quality & Scope Validation

- [ ] `git status --short` shows no changes (clean verification) or ≤ a few targeted straggler fixes.
- [ ] NO new files created (unless an inconsistency required a fix).
- [ ] Scope respected: PRD.md, Go source, go.mod NOT modified; S2's external checks referenced, not duplicated.
- [ ] plan/ and .pi-subagents/ NOT verified/modified (S1's documented exceptions; P1.M5.T1.S1's territory).

### Documentation & Deployment

- [ ] Verification record (implementation summary) captures each check's command + output + verdict.
- [ ] The namespace decision + pre-release action items are explicit in the record (not implied).
- [ ] The EDITMSG casing non-issue is recorded so no future maintainer "fixes" it and breaks git convention.

---

## Anti-Patterns to Avoid

- ❌ Don't change the docs/goreleaser to `dabstractor/` to "match the remote". Canonical is `dustin/`
      (3 mandates: go.mod, PRD §21.2/21.3, goreleaser owner). The remote is a pre-release action item.
- ❌ Don't "fix" the EDITMSG casing (lowercase the code to match PRD). Uppercase `STAGECOACH_EDITMSG`
      matches git's own convention (COMMIT_EDITMSG); the code is correct. The PRD lowercase is loose prose.
- ❌ Don't run a repo-wide `sed s/stagehand/stagecoach/`. Scope fixes to the SINGLE offending file, and
      only if a check actually FAILS. Research found none blocking.
- ❌ Don't trust the research "PASS" table without re-running the greps. Research is a snapshot; the
      parallel P1.M5.T2.S2 could in principle touch a doc. Re-verify fresh.
- ❌ Don't include `plan/` or `.pi-subagents/` in the straggler greps and then "fix" the hits — they are
      historical research records (pre-rename), NOT user-facing, and are P1.M5.T1.S1's scope.
- ❌ Don't duplicate S2's external checks (URL resolution, tap/bucket existence, install.sh publication).
      S1 = internal consistency; S2 = external correctness. Reference, don't repeat.
- ❌ Don't create a new doc file "to document the verification". The deliverable is the verification RECORD
      (implementation summary); a redundant doc risks colliding with future doc-review work.
- ❌ Don't edit PRD.md (read-only) or go.mod (the namespace oracle — verified, not changed).
- ❌ Don't declare a check PASS without the grep EVIDENCE in the record. "It looked fine" is not verification.
