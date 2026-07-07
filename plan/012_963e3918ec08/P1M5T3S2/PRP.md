---
name: "P1.M5.T3.S2 — Verify badge URLs, GitHub links, and distribution paths are correct (stagecoach rename close-out)"
description: |

  THIS IS A VERIFICATION / MODE-B DISTRIBUTION-SURFACE GATE. It is the rename's (stagehand→stagecoach,
  PRD h2.30) final cross-cutting coherence gate over the DISTRIBUTION surface: every GitHub URL, every
  distribution-channel name (Homebrew tap, Scoop bucket, AUR package), the curl|sh install URL, and the
  namespace. It verifies — by grepping the LIVE files and running the binary, not by asserting — that the
  renamed project's distribution paths are correct, consistent, and agreed across README.md, .goreleaser.yaml,
  docs/, and the built binary. Complementary to S1 (internal identity consistency) and P1.M5.T2.S2 (build/test).

  ⚠️ HEADLINE RESEARCH FINDING (executed in the live repo + web, 2026-07-07): THE DISTRIBUTION SURFACE IS
  ALREADY CORRECT AND CONSISTENT. The rename's M3 (build/CI) + M4 (docs) left all 5 contract checks (a–e)
  PASSING. Every GitHub URL is `github.com/dustin/stagecoach` (13 occurrences, ZERO stagehand stragglers,
  ZERO `dabstractor` in any URL/config value — it appears ONLY in explanatory goreleaser COMMENTS). The
  distribution-channel names agree (Homebrew tap `dustin/homebrew-tap`; Scoop bucket `dustin/scoop-bucket`;
  AUR `stagecoach-bin`). The binary builds and `providers list` shows 8 providers under `stagecoach`
  branding. This task is therefore primarily a VERIFICATION that gathers deterministic evidence, RESOLVES
  the one open product question (the namespace — see §"Namespace resolution"), and FLAGS the release-time
  prerequisites. It is NOT a bulk-edit task; targeted fixes are applied ONLY if re-verification finds drift
  (research found: none blocking).

  CONTRACT (P1.M5.T3.S2):
    1. RESEARCH NOTE: "See external_deps.md and critical_findings.md F9. Distribution channels: Homebrew
       (dustin/tap/stagecoach), Scoop (dustin/scoop-bucket/stagecoach), AUR (stagecoach-bin), GitHub
       Releases (github.com/<org>/stagecoach). The actual remote is `dabstractor/stagecoach`. The goreleaser
       and README must agree."
    2. INPUT: "The verified-consistent project from S1."
    3. LOGIC: (a) Check all URLs in README.md, .goreleaser.yaml, and docs/ point to the same GitHub repo.
       (b) Verify Homebrew tap name, Scoop bucket name, and AUR package name are consistent across goreleaser
       and README. (c) Verify the curl|sh install script URL (if install.sh exists). (d) If any URL is wrong
       (e.g., `dustin/stagecoach` when remote is `dabstractor/stagecoach`), fix it. (e) Run `make build &&
       ./bin/stagecoach providers list` as a final smoke test — should list all providers with 'stagecoach'
       branding.
    4. OUTPUT: "All distribution paths, URLs, and package names are correct and consistent. The rename is
       complete."
    5. DOCS: "Mode B — final cross-cutting verification of the distribution surface."

  ⚠️ THE NAMESPACE RESOLUTION (the contract's central open question — RESOLVED here, §"Namespace resolution"):
     canonical org = `dustin/`, NOT `dabstractor/`. The git remote `dabstractor/stagecoach` is a
     PRE-RELEASE GATE (the repo must be reachable at github.com/dustin/stagecoach before the first real
     tag), NOT a doc bug. The contract check (d) example (`dustin/stagecoach` "wrong" vs remote) names the
     SYMPTOM, but the mechanically-correct FIX is "reconcile the REMOTE to dustin/stagecoach," NOT "change
     the docs to dabstractor" (that would break `go install`, which resolves at go.mod's declared path
     `github.com/dustin/stagecoach`, and would contradict PRD §21.2/§21.3 + S1's decision). Do NOT change
     docs/goreleaser to `dabstractor/`.

  SCOPE BOUNDARY (frozen / owned elsewhere — do NOT touch unless re-verification finds a real straggler):
    - README/docs/goreleaser CONTENT (the install commands, channel names) → ALREADY consistent; verify, do
      not bulk-rewrite. Targeted one-line straggler fix ONLY if a check FAILS (research: none).
    - S1 (P1.M5.T3.S1) internal-identity checks (a–g: badge=go install=go.mod=goreleaser, zero stagehand,
      lazygit/git-alias/EDITMSG) → S1's territory; S2 references, does not duplicate.
    - The git REMOTE (`dabstractor/stagecoach`) → a pre-release GitHub action (rename/transfer/mirror); NOT
      changed by S2.
    - `install.sh` (absent pre-release) → a release artifact; S2 flags its absence, does NOT create it.
    - `homebrew_casks:` in .goreleaser.yaml → CORRECT (goreleaser v2.10 deprecated `brews`/formulas; §"homebrew_casks
      is CORRECT"). Do NOT revert to `brews:`.
    - PRD.md, go.mod, Go source, plan/012_*, tasks.json, prd_snapshot.md → READ-ONLY / orchestrator-owned.

  DELIVERABLE: a verification RECORD (implementation summary — Mode B; NO new doc file) demonstrating each
  of the 5 checks (a–e) executed with its grep/command + expected output, the namespace resolution
  documented, and the release-time prerequisites flagged. PLUS a scoped locate-and-fix IF (and only if)
  re-verification finds a real straggler (research found: none blocking today).

  SUCCESS: all 5 checks (a–e) PASS with evidence; every distribution URL is `github.com/dustin/stagecoach`;
  the channel names agree across goreleaser + README + docs; `make build && ./bin/stagecoach providers list`
  lists all providers under `stagecoach` branding; the namespace is confirmed `dustin/stagecoach` with the
  remote mismatch documented as a pre-release gate; `git status` shows at most targeted straggler fixes
  (likely none).

---

## Goal

**Feature Goal**: Certify — by executing deterministic greps against the live files AND running the built
binary, not by asserting — that the renamed `stagecoach` project's DISTRIBUTION surface is correct and
consistent end-to-end. Prove each of the contract's 5 checks (a–e) holds: (a) every URL in README.md,
.goreleaser.yaml, and docs/ points to the SAME GitHub repo; (b) the Homebrew tap name, Scoop bucket name,
and AUR package name are consistent across goreleaser and README; (c) the curl|sh install URL is correct
(whether or not install.sh exists yet); (d) the namespace is resolved (`dustin/` canonical, remote flagged);
(e) `make build && ./bin/stagecoach providers list` lists all providers under `stagecoach` branding.

**Deliverable**: A verification record (implementation summary — Mode B; no new doc file) showing each of
the 5 checks (a–e) executed with its precise command + observed output, the namespace resolution
documented, and the release-time prerequisites (remote reconciliation, install.sh publication, tap/bucket
creation) flagged. A scoped locate-and-fix is applied ONLY if re-verification finds a real straggler
(research found: none blocking today).

**Success Definition**:
- Check (a): a SINGLE `github.com/dustin/stagecoach` string across README.md, docs/, .goreleaser.yaml,
  go.mod (no split namespace, no `dabstractor` in any URL/config value, no `stagehand` straggler).
- Check (b): Homebrew tap = `dustin/homebrew-tap` (brew `dustin/tap`); Scoop bucket = `dustin/scoop-bucket`;
  AUR = `stagecoach-bin` — each consistent between .goreleaser.yaml and README/docs.
- Check (c): the install.sh URL is `github.com/dustin/stagecoach/raw/main/install.sh` (consistent); the
  file is ABSENT pre-release (documented in docs/README.md:27) — flagged, not created.
- Check (d): canonical org = `dustin/` (go.mod + PRD + goreleaser owner). The remote `dabstractor/stagecoach`
  is documented as a pre-release gate; docs are NOT changed to `dabstractor`.
- Check (e): `make build` produces `./bin/stagecoach`; `./bin/stagecoach providers list` lists all providers
  (agy, claude, codex, cursor, gemini, opencode, pi, qwen-code) under `stagecoach` branding; `--version`
  prints `stagecoach version …`.
- `git status` shows at most targeted straggler fixes (research found none); no new files created unless an
  inconsistency is discovered.

## User Persona

**Target User**: the maintainer certifying the rename before the first `stagecoach` tag, and the release
engineer who needs README + .goreleaser.yaml + docs to AGREE with go.mod (and with each other) before
publishing. Secondary: any user who copy-pastes an install command — a `go install` that 404s, a Scoop
manifest that doesn't match the bucket, or a stale `stagehand` URL would immediately break trust.

**Use Case**: "After renaming stagehand→stagecoach, do all the distribution paths actually work and agree?
Does `brew install dustin/tap/stagecoach` match what goreleaser publishes? Does the Scoop command match the
bucket goreleaser pushes to? Does the badge URL match the go.mod module path? Is there any leftover
`stagehand` URL?" The maintainer runs the 5 checks, eyeballs the namespace, and either ships (all green)
or applies a one-line straggler fix. The most consequential trap: the git remote is `dabstractor/stagecoach`
while everything else says `dustin/stagecoach` — this task resolves that deliberately (canonical = `dustin`)
rather than reflexively rewriting the docs to match a remote that itself needs reconciling.

**Pain Points Addressed**: a rename can be "done" at the source level yet still leak a stale URL, a
mismatched channel name (goreleaser pushes to one repo while the README install command implies another),
or a half-renamed namespace (some files `dustin`, some `dabstractor`). Worse, a well-meaning "fix" can
reintroduce a deprecated goreleaser pipe (`brews:`→`homebrew_casks:` was a deliberate v2.10 change). This
task is the net that catches those — and the guardrail that prevents the wrong fix.

## Why

- **It is the rename's distribution-surface close-out gate (PRD h2.30), complementary to S1 (internal
  identity) and P1.M5.T2.S2 (build/test).** S1 certifies the project's IDENTITY is unified (badge = go
  install = go.mod = goreleaser, zero `stagehand`, lazygit/alias/EDITMSG); S2 certifies the project's
  DISTRIBUTION PATHS are correct and agreed (channel names, install URLs, external resolution, binary smoke
  test). Both are needed; neither subsumes the other.
- **Resolves the namespace question the contract explicitly raises.** "the actual remote is
  `dabstractor/stagecoach`... verify the canonical GitHub path" is not a grep — it is a product decision
  with a mechanically-forced answer. This task makes the decision explicit (`dustin/`, per go.mod + PRD +
  goreleaser), documents WHY a literal "change to dabstractor" fix would break `go install`, and flags the
  remote reconciliation so it isn't discovered at tag time.
- **Catches the cross-surface channel-name disagreements a green build cannot.** `go build` succeeding says
  nothing about whether goreleaser's Scoop bucket matches the README's `scoop install` command, or whether
  the Homebrew tap name is `homebrew-tap` in goreleaser and `dustin/tap` in the README. The greps are the net.
- **Prevents the wrong fix.** Research confirmed `homebrew_casks:` is the CURRENT goreleaser pipe (v2.10
  deprecated `brews`/formulas). A reviewer who doesn't know this would "fix" goreleaser to `brews:` and
  reintroduce a deprecated pipe. This PRP records the verified fact so the fix isn't applied.

## What

Run, observe, and assert on the 5 contract checks (a–e), recording grep/command evidence. The complete
certification sequence (each verified PASSING in research — re-run at implementation time):

1. **(a) One-repo consistency** — every GitHub URL across README.md, docs/, .goreleaser.yaml, go.mod is the
   SAME string `github.com/dustin/stagecoach`. No split namespace (no `dabstractor` in any URL/value), no
   `stagehand` straggler.
2. **(b) Channel-name consistency** — Homebrew tap (`dustin/homebrew-tap` ↔ `brew install dustin/tap/…`),
   Scoop bucket (`dustin/scoop-bucket` ↔ `scoop install dustin/stagecoach`), AUR (`stagecoach-bin`)
   each agree between .goreleaser.yaml and README/docs.
3. **(c) install.sh URL** — the curl|sh URL is `github.com/dustin/stagecoach/raw/main/install.sh`
   (consistent); the file is ABSENT pre-release (documented in docs/README.md:27) → flagged, not created.
4. **(d) Namespace resolution** — canonical org = `dustin/` (go.mod + PRD + goreleaser owner); the remote
   `dabstractor/stagecoach` is a pre-release gate; docs NOT changed to `dabstractor`.
5. **(e) Smoke test** — `make build` → `./bin/stagecoach`; `providers list` → 8 providers under `stagecoach`
   branding; `--version` → `stagecoach version …`.

If re-verification finds a real straggler (a check that FAILS), apply the minimal targeted fix to the
offending file (the rename rule is `stagehand`→`stagecoach`, preserving the `dustin/` org). Research found
none blocking; the most likely drift sources are listed in §"Known Gotchas".

### Success Criteria

- [ ] All 5 checks (a–e) PASS with their commands producing the expected output (see §"Validation Loop").
- [ ] `grep -rhoE 'github\.com/dustin/stagecoach' README.md docs/ .goreleaser.yaml go.mod` → N matches (≥10);
      `grep -rn 'dabstractor' <same>` → only `.goreleaser.yaml` COMMENTS (L7, L68), never a URL/value.
- [ ] `grep -rni 'stagehand' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml` → ZERO.
- [ ] Channel names agree: goreleaser `homebrew-tap`/`scoop-bucket`/`stagecoach-bin` ↔ README `dustin/tap`/
      `dustin/stagecoach`/install paths.
- [ ] `make build && ./bin/stagecoach providers list` lists all providers under `stagecoach` branding;
      `./bin/stagecoach --version` prints `stagecoach version …`.
- [ ] Namespace confirmed `dustin/stagecoach`; remote mismatch documented as a pre-release gate; docs NOT
      changed to `dabstractor`.
- [ ] Scope respected: S1's internal checks referenced, not duplicated; `homebrew_casks` NOT reverted;
      PRD/go.mod/source not modified; install.sh not created.

## All Needed Context

### Context Completeness Check

_Pass._ An author who has never seen this repo can implement this from: the namespace resolution (§"Namespace
resolution" — THE central question, resolved with the mechanical proof); the homebrew_casks-correctness
note (§"homebrew_casks is CORRECT — do NOT revert"); the 5-check evidence table with exact file:line
references and expected greps (§"Implementation Blueprint"); the S1/S2 scope split (§"Integration Points");
the smoke-test expected output (§findings.md §7); and the re-runnable verification commands (§"Validation
Loop"). The only genuinely uncertain input (whether any check has drifted since research) is handled by
requiring a FRESH re-run of every check before declaring PASS.

### Documentation & References

```yaml
# MUST READ - Include these in your context window
- docfile: plan/012_963e3918ec08/P1M5T3S2/research/findings.md
  why: THE decisive doc. §1 the ground-truth namespace sweep (13× dustin, 0 stagehand, dabstractor only in
       comments); §2 the namespace RESOLUTION (the mechanical proof that changing to dabstractor breaks go
       install); §3 homebrew_casks IS correct (v2.10 deprecates formulas, web-verified URLs); §4 scoop
       convention; §5 install.sh absent (documented); §6 AUR; §7 the smoke-test output (8 providers); §8 the
       S1/S2 split; §9 the cited URLs.
  critical: §2 (namespace resolution), §3 (do NOT revert homebrew_casks → brews).

- docfile: plan/012_963e3918ec08/P1M5T3S1/PRP.md   (the PARALLEL sibling — READ only; treat as CONTRACT)
  why: S1 owns INTERNAL identity consistency (checks a–g). S2 OWNS distribution-path correctness + smoke
       test. S1's §"Namespace decision tree" independently reaches the SAME `dustin/` conclusion S2 inherits
       (3 mandates: go.mod, PRD §21.2/21.3, goreleaser owner). Do NOT duplicate S1's lazygit/alias/EDITMSG
       checks; reference them.
  section: §"Namespace decision tree" (the shared decision), §"Integration Points" (the S1/S2 split).

- docfile: plan/012_963e3918ec08/architecture/critical_findings.md   (F9 + F10)
  why: F9 (README badge URL referenced the OLD path) is the contract's cited research note; F9 SUGGESTED
       dabstractor/, but go.mod + PRD + the goreleaser PRP OVERRADED it to dustin/. F10 (.gitignore) confirms
       the config surface was renamed. S2 confirms the override holds across the DISTRIBUTION surface.
  section: F9, F10.

- file: .goreleaser.yaml   (P1.M3.T1.S2 — READ only; the distribution config under verification)
  section: owner note (L5-8) + `release.github.owner: dustin` (L60-62) + `homebrew_casks:` (L64-75) +
           `scoops:` (L78-91) + `aurs:` (L94-118). 
  why: this is where every release URL + channel name lives. S2 verifies they are `dustin/stagecoach` and
       agree with README. The `homebrew_casks:` (NOT `brews:`) is CORRECT per v2.10 — do NOT revert.
  pattern: the owner-note comment (L5-8) is the authoritative namespace statement — quote it in the record.
  gotcha: `homebrew_casks` is current (formulas deprecated in v2.10); reverting to `brews` = reintroducing a
          deprecated pipe. (Web-verified: see findings.md §3 / §9.)

- file: README.md   (P1.M4.T1.S1 — READ only; the primary user-facing surface)
  section: badge (L10), clone (L95), install block (L113-116: brew/scoop/go install/install.sh).
  why: checks (a)(b)(c) live here. Verify the 4 install paths + badge + clone are all `dustin/stagecoach`
       and match goreleaser's channel names.

- file: docs/README.md   (P1.M4.T1.S2 — READ only; the docs index install block)
  section: install block (L14-23) + the install.sh absence note (L27).
  why: check (c) — the install.sh URL + the documented "published with the first release" note (confirms
       absence is intentional, not a broken cross-ref).

- file: go.mod   (READ only; line 1)
  why: `module github.com/dustin/stagecoach` is the ORACLE for the namespace + the `go install` path. The
       reason docs MUST stay `dustin/` (changing to dabstractor makes `go install` 404).

- file: Makefile   (P1.M3.T1.S1 — READ only; the smoke-test entry point)
  section: `build:` (L52: `go build -ldflags … -o bin/stagecoach ./cmd/stagecoach`), `BIN := bin/stagecoach`
           (L28), `MAIN_PKG := ./cmd/stagecoach` (L30).
  why: check (e) — `make build` produces `./bin/stagecoach`; the binary name + main package are `stagecoach`.

# --- web-verified facts (consult if the homebrew_casks question is challenged) ---
- url: https://goreleaser.com/customization/publish/homebrew_formulas/
  why: states "Homebrew Formulas (deprecated). Deprecated in v2.10. Homebrew Casks should be used instead."
  critical: CONFIRMS `.goreleaser.yaml`'s `homebrew_casks:` is correct and the comment is accurate.
- url: https://goreleaser.com/blog/goreleaser-v2.10/
  why: "introduces the new Homebrew Casks feature." (the version that made casks the recommendation)
- url: https://github.com/goreleaser/goreleaser/discussions/5563
  why: "Brew packages should be casks, not formulae" — the rationale (precompiled binaries as formulae
       confused Homebrew users).
- url: https://goreleaser.com/customization/publish/homebrew_casks/
  why: the CURRENT casks pipe doc — the reference for the config block under verification.
- url: https://github.com/ScoopInstaller/Scoop/wiki/Buckets
  why: scoop bucket convention (bucket = git repo of JSON manifests; `scoop bucket add` then `scoop install
       bucket/app`). Confirms `scoop install dustin/stagecoach` is the standard shorthand and that the
       manifest name `stagecoach` + bucket `dustin/scoop-bucket` are consistent.

# --- PRD (authoritative spec) ---
- doc: PRD.md h2.30 (the rename directive), §21.2 (goreleaser / brew `dustin/tap` / scoop / AUR), §21.3
       (install paths: brew/scoop/go install all `dustin/stagecoach`), §21.5 (README structure).
  why: authoritative spec for the rename scope + the install-path namespace (mandates `dustin/`).
```

### Current Codebase tree (relevant slice)

```bash
README.md                    # badge L10, clone L95, install block L113-116. (verify; edit only if straggler)
docs/                        # install block in docs/README.md L14-23 + install.sh note L27. (verify)
.goreleaser.yaml             # owner note L5-8, owner L60-62, homebrew_casks L64-75, scoops L78-91, aurs L94-118. (verify)
go.mod                       # module github.com/dustin/stagecoach. THE namespace oracle. (verify; READ-ONLY)
Makefile                     # build L52, BIN L28, MAIN_PKG L30. (smoke-test entry)
bin/stagecoach               # produced by `make build`. (the smoke-test binary)
install.sh                   # ABSENT (pre-release; documented in docs/README.md:27). (flag, do not create)
# plan/012_*, PRD.md, Go source, .pi-subagents/ → NOT distribution-surface; documented exceptions; do NOT verify/modify.
```

### Desired Codebase tree with files to be added

```bash
# NO new files by default (Mode-B verification; the distribution surface is already consistent). The
# deliverable is a verification RECORD (implementation summary). IF re-verification finds a real straggler,
# apply the minimal one-line fix to the offending file (stagehand→stagecoach, preserving the dustin/ org).
# Research found: none blocking. Expected `git status` after a clean verification: no changes (or ≤ a few
# straggler fixes). install.sh is NOT created (it is a release artifact, out of scope).
```

### Known Gotchas of our codebase & Library Quirks

```yaml
# CRITICAL (THE NAMESPACE — do NOT reflexively "fix" the remote mismatch). git remote =
# `dabstractor/stagecoach`; EVERY user-facing URL + go.mod + goreleaser uses `dustin/stagecoach`. Canonical
# org = `dustin/` (3 mandates: go.mod:1, PRD §21.2/21.3, .goreleaser.yaml:60-62). DO NOT change the
# docs/goreleaser to `dabstractor/`. The contract check (d) example names the SYMPTOM (URL ≠ remote); the
# correct FIX is "reconcile the REMOTE to dustin/stagecoach," not "rewrite the docs." PROOF: changing docs to
# dabstractor while go.mod says dustin makes `go install github.com/dustin/stagecoach/...` 404 (Go resolves
# the module at its DECLARED path). See §"Namespace resolution". This matches S1's independent conclusion.

# CRITICAL (homebrew_casks IS CORRECT — do NOT revert to brews). `.goreleaser.yaml` uses `homebrew_casks:`
# (L64). A reviewer unfamiliar with goreleaser v2.10 may "fix" this to `brews:` (formulas) thinking casks are
# for GUI apps only. WRONG: goreleaser v2.10 DEPRECATED formulas for precompiled binaries and made casks the
# recommendation (web-verified: homebrew_formulas page "Deprecated in v2.10"; discussion #5563). Reverting =
# reintroducing a deprecated pipe. The config + its comment are accurate. LEAVE IT.

# CRITICAL (install.sh is EXPECTED to be ABSENT pre-release). README (L116) + docs/README.md (L20) reference
# `…/raw/main/install.sh`, but the file does not exist. docs/README.md:27 documents: "published with the first
# release." This is NOT a broken cross-ref — it is a documented pre-release gap. The URL is consistent
# (`dustin/stagecoach/raw/main`). Do NOT create install.sh (it is a release artifact, out of doc-verification
# scope); flag its absence + the release-time publication task.

# GOTCHA (RE-VERIFY FRESH — research is a snapshot, 2026-07-07). The 5 checks PASS as of research, but
# P1.M5.T2.S2 (build/test) and S1 (internal consistency) run in parallel and could in principle touch a doc
# if either finds a straggler. Re-run EVERY check at implementation time; do not trust the findings table's
# "PASS" without re-confirming.

# GOTCHA (THE CHECKS EXCLUDE plan/, .pi-subagents/, PRD.md). Historical plan/ files + .pi-subagents/ artifacts
# DO contain `stagehand` (pre-rename research records, NOT user-facing, NOT shipped). PRD.md h2.30 itself
# mentions the old name. They are P1.M5.T1.S1's scope / read-only. Scope your greps to the DISTRIBUTION +
# user-facing surface (README, docs/, .goreleaser.yaml, .golangci.yml, Makefile, go.mod).

# GOTCHA (S1/S2 OVERLAP — coordinate). S1 = internal identity consistency (badge=go install=go.mod=goreleaser,
# zero stagehand, lazygit/alias/EDITMSG, checks a–g). S2 = distribution-path correctness + channel-name
# agreement + smoke test (checks a–e). Both touch "namespace consistency"; S2's DISTINCT value is the
# CHANNEL-NAME cross-check (brew tap / scoop bucket / AUR), the homebrew_casks correctness, the install.sh
# check, the external-resolution flag, and the binary smoke test. Reference S1; don't re-run its lazygit/alias
# checks.

# GOTCHA (`dabstractor` APPEARS LEGITIMATELY in .goreleaser.yaml comments). grep will hit `.goreleaser.yaml:7`
# and `:68` for `dabstractor` — but these are COMMENTS explaining the remote mismatch, not URLs/config values.
# The actual `owner: dustin` (L68) is correct. A `dabstractor` hit in a URL or a `repository.owner`/`name`
# VALUE would be a real bug; a hit in a comment is documentation. Distinguish before "fixing."

# GOTCHA (SCOOP README SHORTHAND omits `scoop bucket add`). README says `scoop install dustin/stagecoach`
# (a one-liner). The full scoop flow is `scoop bucket add dustin https://github.com/dustin/scoop-bucket` then
# `scoop install dustin/stagecoach`. The one-liner is the standard Go-CLI shorthand and matches PRD §21.3
# VERBATIM — it is NOT a goreleaser-vs-README inconsistency (the manifest name `stagecoach` + owner `dustin`
# agree). Leave it; do not "complete" the README scoop instructions (that's a docs-UX call, out of scope).
```

#### Namespace resolution (resolves the contract check (d) tension — THE centerpiece)

```
THE CONTRACT CHECK (d): "If any URL is wrong (e.g., `dustin/stagecoach` when remote is
`dabstractor/stagecoach`), fix it." Read literally → change docs to `dabstractor`. THAT IS THE WRONG FIX.

THE MECHANICAL PROOF (why docs MUST stay `dustin/`):
  1. `go install github.com/<module-path>/cmd/stagecoach@latest` resolves the module at its DECLARED path.
     go.mod:1 = `module github.com/dustin/stagecoach`. So the documented `go install` command (README L115,
     docs/README.md L17) is `github.com/dustin/stagecoach/...`. If the docs said `dabstractor` but go.mod
     says `dustin`, the command 404s (Go fetches at the module path, not the doc string).
  2. PRD §21.2/§21.3 MANDATE `dustin/*` (brew `dustin/tap/stagecoach`, scoop `dustin/stagecoach`, go install
     `github.com/dustin/stagecoach/...`). PRD is read-only, human-owned. The product's canonical org is `dustin`.
  3. .goreleaser.yaml:60-62 `release.github.owner: dustin` ("WINS over git-remote auto-detect"). goreleaser
     publishes the GitHub Release to `dustin/stagecoach` — consistent with go.mod + the PRD install paths.
  4. S1 (parallel sibling) independently reached `dustin/` (same 3 mandates). Reversing it = cross-work-item
     contradiction. go.mod is the structural source of truth; changing it to `dabstractor` is a module-rename
     (M1 territory), out of a doc-verification task's scope.

THE RESOLUTION: canonical org = `dustin/`. The git REMOTE (`dabstractor/stagecoach`) is the thing that is
"wrong" — it must be reconciled so the repo is reachable at `github.com/dustin/stagecoach` before the first
REAL tag (a GitHub rename/transfer/mirror). THAT is the pre-release gate. The docs are correct as-is.

WHAT THIS MEANS FOR CHECK (d): verify (1) NO SPLIT namespace (every URL = `dustin/stagecoach`, zero
`dabstractor` in any URL/value, zero `stagehand` straggler); (2) the canonical org is confirmed `dustin/`;
(3) the remote mismatch is FLAGGED as a pre-release gate. Do NOT rewrite any URL to `dabstractor`.

This is IDENTICAL to S1's conclusion (S1 PRP §"Namespace decision tree"). S2 does not relitigate; it confirms
the decision holds across the DISTRIBUTION surface (channels + install URLs) and adds the binary smoke test.
```

#### homebrew_casks is CORRECT — do NOT revert to brews (the second-most-important guardrail)

```
.goreleaser.yaml:64 uses `homebrew_casks:`. The comment (L63) says "replaces deprecated brews/formulas in
goreleaser v2.10+." VERIFIED ACCURATE (web, 2026-07-07):
  - https://goreleaser.com/customization/publish/homebrew_formulas/ : "Homebrew Formulas (deprecated).
    Deprecated in v2.10. Homebrew Casks should be used instead."
  - https://goreleaser.com/blog/goreleaser-v2.10/ : "introduces the new Homebrew Casks feature."
  - https://github.com/goreleaser/goreleaser/discussions/5563 : precompiled binaries distributed as formulae
    confused Homebrew users → casks are now recommended for precompiled-binary CLIs.

CONSISTENCY WITH README: goreleaser publishes cask `stagecoach` (name = project_name) to repo
`dustin/homebrew-tap` (= brew tap `dustin/tap`). README says `brew install dustin/tap/stagecoach`. Modern
Homebrew (4.0+) resolves `brew install owner/tap/name` to a cask OR formula by name in the tap → the command
works with a cask. CONSISTENT. (Release-time confirmation: the cask is actually published; `goreleaser check`
+ a real publish validates the cask name.)

VERDICT: `homebrew_casks:` is correct and current. Reverting to `brews:` would reintroduce a DEPRECATED pipe
(and a recent goreleaser binary may warn/error on `brews:`). Do NOT "fix" it. If a reviewer flags it, point
them at the 3 URLs above.
```

## Implementation Blueprint

### Implementation Tasks (ordered by dependencies)

```yaml
Task 0: ORIENT + confirm the namespace decision (READ + RUN, no edit)
  - RUN: `git remote -v | grep origin` -> expect `dabstractor/stagecoach` (the REMOTE).
  - RUN: `head -1 go.mod` -> expect `module github.com/dustin/stagecoach` (the MODULE PATH = oracle).
  - READ: .goreleaser.yaml:5-8 + :60-62 + :63-64 (the owner note + `owner: dustin` + the homebrew_casks
    comment). CONFIRM canonical = `dustin/` + that homebrew_casks is intentional (v2.10).
  - READ: plan/012_963e3918ec08/P1M5T3S2/research/findings.md §2 (namespace resolution) + §3 (homebrew_casks).
    These are the research snapshot to re-confirm, not trust blindly.

Task 1: VERIFY check (a) — one-repo consistency (RUN, assert)
  - `grep -rhoE 'github\.com/dustin/stagecoach' README.md docs/ .goreleaser.yaml go.mod | sort | uniq -c`
      -> expect a SINGLE line `  13 github.com/dustin/stagecoach` (N≥10; research=13). ZERO other orgs.
  - `grep -rn 'dabstractor' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml`
      -> expect ONLY `.goreleaser.yaml:7` and `:68`, BOTH comments (not URLs/values). The `owner: dustin`
      on L68 is the actual value (correct).
  - `grep -rni 'stagehand' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml`
      -> expect ZERO output (clean).
  - ASSERT: a single `github.com/dustin/stagecoach` string; no split namespace; no straggler. (If a
    `dabstractor` URL/value or a `stagehand` hit appears, it's a real straggler -> Task 5 fix.)

Task 2: VERIFY check (b) — channel-name consistency (RUN, assert)
  - Homebrew: goreleaser tap repo vs README brew command.
      grep -nE 'name: homebrew-tap|dustin/tap' .goreleaser.yaml README.md docs/README.md
      -> goreleaser `name: homebrew-tap` (L70); README/docs `brew install dustin/tap/stagecoach`. The brew
         tap `dustin/tap` == repo `dustin/homebrew-tap` (brew convention). CONSISTENT.
  - Scoop: goreleaser bucket vs README scoop command.
      grep -nE 'name: scoop-bucket|scoop install' .goreleaser.yaml README.md docs/README.md
      -> goreleaser `name: scoop-bucket` (L86) + `name: stagecoach` (L80 manifest); README `scoop install
         dustin/stagecoach`. Manifest name `stagecoach` + owner `dustin` AGREE. CONSISTENT. (The README
         one-liner omits `scoop bucket add` — standard shorthand, matches PRD §21.3 verbatim; not a bug.)
  - AUR: goreleaser package name vs PRD.
      grep -n 'name: stagecoach-bin' .goreleaser.yaml   -> L96. PRD §21.2: "stagecoach + stagecoach-bin
      (possibly community)". CONSISTENT (from-source `stagecoach` is community/out-of-scope per PRD).
  - ASSERT: all 3 channel names agree across goreleaser + README + docs.

Task 3: VERIFY check (c) — install.sh URL (RUN, assert)
  - `grep -rn 'install.sh' README.md docs/`
      -> README L116 + docs/README.md L20 reference `https://github.com/dustin/stagecoach/raw/main/install.sh`
         (URL is `dustin/stagecoach` -> consistent with check (a)).
  - `test -e install.sh && echo EXISTS || echo ABSENT`   -> ABSENT (pre-release).
  - `grep -n 'published with the first release' docs/README.md`   -> L27 (documents the absence).
  - ASSERT: the install.sh URL is consistent (`dustin/stagecoach`); the file is ABSENT and the absence is
    DOCUMENTED (docs/README.md:27) -> a known pre-release gap, not a broken cross-ref. FLAG for release-time
    publication; do NOT create install.sh.

Task 4: VERIFY check (e) — smoke test (RUN, assert)
  - `make build`   -> `go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach`; expect
    `./bin/stagecoach` to exist.
  - `./bin/stagecoach providers list`   -> expect 8 providers (agy, claude, codex, cursor, gemini, opencode,
    pi [default], qwen-code) under `stagecoach` branding. (qwen-code may show ✗ DETECTED if its CLI isn't
    installed — an environment fact, not a rename defect; the provider is LISTED.)
  - `./bin/stagecoach --version`   -> expect `stagecoach version …` (branding = stagecoach).
  - ASSERT: the binary builds, lists all providers, and presents `stagecoach` branding throughout.

Task 5: FIX stragglers IF (and only if) a check FAILED (TARGETED, one file at a time)
  - If Task 1 found a `dabstractor` URL/VALUE or a `stagehand` straggler: apply the minimal scoped fix to the
    SINGLE offending file. The rename rule is `stagehand`→`stagecoach`, PRESERVING the `dustin/` org.
    Example (hypothetical; do NOT run unless needed):
      sed -i 's#github.com/dabstractor/stagecoach#github.com/dustin/stagecoach#g' <offending-file>
      # OR for a stagehand straggler:
      sed -i 's/stagehand/stagecoach/g' <offending-file>
  - Do NOT run a repo-wide sed. Do NOT touch go.mod / PRD / Go source. Do NOT revert homebrew_casks → brews.
  - After ANY fix, re-run the FULL 5-check suite (Tasks 1–4) to confirm green, then `git status`.
  - Research found: none blocking. Expected: no edits needed (clean verification).

Task 6: RECORD + flag (the deliverable)
  - Record the verification outcome in the implementation summary: each of the 5 checks (a–e) with its
    command + observed output + PASS/FAIL.
  - Document the namespace resolution (canonical `dustin/`; the mechanical proof; the F9 override; the
    pre-release remote gate).
  - Record that `homebrew_casks` is CORRECT (v2.10) — so no future maintainer reverts it.
  - Flag the pre-release prerequisites: (1) reconcile the remote to github.com/dustin/stagecoach (rename/
    transfer/mirror); (2) publish install.sh at first release; (3) create dustin/homebrew-tap +
    dustin/scoop-bucket repos (+ PATs) before goreleaser publishes; (4) AUR account + SSH key if `aurs:`
    is kept. These are release-time gates, NOT fixed here.
  - `git status --short` -> expect no changes (clean verification) or ≤ a few targeted straggler fixes.
    NO new files unless an inconsistency was found.
```

### Implementation Patterns & Key Details

```bash
# The 5-check verification is a suite of deterministic greps + a build. Each has a precise expected output.
# Run them FRESH (research is a snapshot); assert each matches; fix only real stragglers.

# (a) one-repo consistency — a SINGLE org string across all surfaces; zero dabstractor URLs; zero stagehand.
grep -rhoE 'github\.com/dustin/stagecoach' README.md docs/ .goreleaser.yaml go.mod | sort | uniq -c   # single line, N≥10
grep -rn 'dabstractor' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml | grep -v '^[^:]*:[0-9]*:#'  # expect empty (only comments allowed)
grep -rni 'stagehand' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml                  # expect empty

# (b) channel-name agreement — goreleaser repos vs README install commands
grep -nE 'name: homebrew-tap|dustin/tap' .goreleaser.yaml README.md docs/README.md                     # homebrew-tap <-> dustin/tap
grep -nE 'name: scoop-bucket|scoop install' .goreleaser.yaml README.md docs/README.md                  # scoop-bucket + manifest stagecoach <-> dustin/stagecoach
grep -n 'name: stagecoach-bin' .goreleaser.yaml                                                        # AUR package

# (c) install.sh URL consistent + absence documented
grep -rn 'install.sh' README.md docs/                                                                  # URL = dustin/stagecoach/raw/main/install.sh
test -e install.sh && echo EXISTS || echo ABSENT                                                        # ABSENT (pre-release)
grep -n 'published with the first release' docs/README.md                                              # L27 documents absence

# (d) namespace resolution (confirm canonical = dustin; remote flagged, NOT fixed)
git remote -v | grep origin            # dabstractor/stagecoach (the REMOTE — pre-release gate)
head -1 go.mod                         # module github.com/dustin/stagecoach (the ORACLE — canonical)
grep -n 'owner: dustin' .goreleaser.yaml   # L60-62 ("WINS over git-remote auto-detect")

# (e) smoke test
make build                             # -> ./bin/stagecoach
./bin/stagecoach providers list        # 8 providers, stagecoach branding
./bin/stagecoach --version             # "stagecoach version …"

# TARGETED STRAGGLER FIX (only if a check FAILS — research found none). NEVER repo-wide; preserve dustin/.
```

### Integration Points

```yaml
DISTRIBUTION SURFACE (verified, not bulk-edited):
  - README.md, docs/*.md, .goreleaser.yaml already present `stagecoach` consistently with `dustin/`. No new
    doc file is created (Mode-B verification; a redundant doc risks colliding with future doc-review work).
    The deliverable is the verification RECORD (implementation summary).

NAMESPACE (the contract's open question — RESOLVED):
  - canonical org = `dustin/` (go.mod + PRD §21.2/21.3 + goreleaser owner). The git remote
    (`dabstractor/stagecoach`) is a PRE-RELEASE GATE: the repo must be reachable at github.com/dustin/stagecoach
    before the first real tag. Flagged, not fixed. Identical to S1's conclusion.

SCOPE HANDOFFS (do NOT duplicate):
  - S1 (P1.M5.T3.S1, parallel): INTERNAL identity consistency (badge=go install=go.mod=goreleaser, zero
    stagehand, lazygit/git-alias/EDITMSG). S2 = distribution-path correctness + smoke test. Reference S1;
    don't re-run its lazygit/alias/EDITMSG checks.
  - P1.M5.T2.S2 (parallel): build/test identity verification. S2's smoke test overlaps at `make build`; S2's
    DISTINCT value is `providers list` + the URL/channel greps. Coordinate if both touch bin/.
  - P1.M5.T1.S1 (Ready): bulk-rename stagehand in plan/ historical files. plan/ is NOT a distribution
    surface; S2's documented exception. No overlap.

RELEASE-TIME PREREQUISITES (flagged, NOT fixed by S2):
  - Reconcile the remote so github.com/dustin/stagecoach resolves (rename/transfer/mirror the GitHub repo).
  - Publish install.sh at the first release (currently absent; documented in docs/README.md:27).
  - Create dustin/homebrew-tap + dustin/scoop-bucket repos (+ fine-grained PATs) before goreleaser publishes.
  - AUR account + AUR_SSH_PRIVATE_KEY if the `aurs:` block is kept.

READ-ONLY (never modify in this task):
  - PRD.md, go.mod, Go source, .golangci.yml (unless a straggler), plan/012_*, tasks.json, prd_snapshot.md.
  - install.sh (do not create; it's a release artifact).
  - `homebrew_casks:` (do NOT revert to `brews:` — it is correct per goreleaser v2.10).
```

## Validation Loop

### Level 1: The 5 checks — run FRESH, assert each (the core verification)

```bash
cd <repo-root>   # /home/dustin/projects/stagehand (cwd unchanged; the PRODUCT is stagecoach)
# Each command has an EXPECTED output (from research, 2026-07-07). Re-run and confirm.

# (a) one-repo consistency
grep -rhoE 'github\.com/dustin/stagecoach' README.md docs/ .goreleaser.yaml go.mod | sort | uniq -c
  # EXPECT: a single line `  13 github.com/dustin/stagecoach` (N≥10). ZERO other orgs.
grep -rn 'dabstractor' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml
  # EXPECT: only .goreleaser.yaml:7 and :68, BOTH comments (the owner-note + the auto-detect comment).
grep -rni 'stagehand' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml
  # EXPECT: no output (clean).

# (b) channel-name agreement
grep -nE 'name: homebrew-tap|dustin/tap' .goreleaser.yaml README.md docs/README.md
  # EXPECT: .goreleaser.yaml name: homebrew-tap; README/docs `brew install dustin/tap/stagecoach`. CONSISTENT.
grep -nE 'name: scoop-bucket|scoop install' .goreleaser.yaml README.md docs/README.md
  # EXPECT: .goreleaser.yaml name: scoop-bucket + manifest name: stagecoach; README `scoop install dustin/stagecoach`. CONSISTENT.
grep -n 'name: stagecoach-bin' .goreleaser.yaml
  # EXPECT: L96. PRD §21.2 names stagecoach-bin. CONSISTENT.

# (c) install.sh URL + documented absence
grep -rn 'install.sh' README.md docs/
  # EXPECT: README L116 + docs/README.md L20 -> https://github.com/dustin/stagecoach/raw/main/install.sh
test -e install.sh && echo EXISTS || echo ABSENT
  # EXPECT: ABSENT (pre-release).
grep -n 'published with the first release' docs/README.md
  # EXPECT: L27 (documents the absence — known gap, not a broken cross-ref).

# (d) namespace resolution (confirm canonical = dustin; remote flagged, NOT fixed)
git remote -v | grep origin            # EXPECT: dabstractor/stagecoach (the REMOTE — pre-release gate)
head -1 go.mod                         # EXPECT: module github.com/dustin/stagecoach (the ORACLE)
grep -n 'owner: dustin' .goreleaser.yaml   # EXPECT: L60-62 ("WINS over git-remote auto-detect")

# (e) smoke test
make build && test -x ./bin/stagecoach          # EXPECT: builds ./bin/stagecoach
./bin/stagecoach providers list                 # EXPECT: 8 providers (agy/claude/codex/cursor/gemini/opencode/pi/qwen-code), stagecoach branding
./bin/stagecoach --version                      # EXPECT: "stagecoach version …"
# Expected: all 5 checks PASS (research snapshot). If any FAILS, apply the targeted straggler fix
# (stagehand->stagecoach, preserve dustin/) to the SINGLE offending file, then re-run the suite.
```

### Level 2: Cross-surface agreement (the "single org" proof + channel cross-check)

```bash
# The single most important assertion: every GitHub path across ALL surfaces is the SAME string.
echo "=== badge / clone / install / goreleaser / go.mod — must all be dustin/stagecoach ==="
grep -rhoE 'github\.com/(dustin|dabstractor)/stagecoach' README.md docs/README.md .goreleaser.yaml go.mod | sort | uniq -c
# EXPECT: a single line `  N github.com/dustin/stagecoach`. ZERO dabstractor, ZERO stagehand.
# If two distinct orgs appear, the namespace is split -> reconcile to dustin/ (the canonical; see §"Namespace resolution").

# Confirm the channel names agree between goreleaser and README (the contract's check (b) heart):
echo "=== brew tap: goreleaser repo <-> README command ==="
grep -oE 'homebrew-tap' .goreleaser.yaml | head -1 ; grep -oE 'dustin/tap/stagecoach' README.md docs/README.md   # homebrew-tap == dustin/tap
echo "=== scoop: goreleaser bucket+manifest <-> README command ==="
grep -oE 'scoop-bucket' .goreleaser.yaml | head -1 ; grep -oE 'scoop install dustin/stagecoach' README.md docs/README.md   # bucket + stagecoach manifest == dustin/stagecoach
```

### Level 3: Straggler-fix validation (ONLY if a check failed in Level 1)

```bash
# After applying a targeted fix to an offending file, re-run the FULL 5-check suite (Level 1) and:
git status --short                       # EXPECT: only the file(s) you intentionally fixed
git diff --stat                          # review the diff is a pure stagehand->stagecoach (or dabstractor->dustin) rename
grep -rni 'stagehand' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml   # EXPECT: clean now
grep -rn 'dabstractor' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml | grep -v '^[^:]*:[0-9]*:#'  # EXPECT: clean (no URL/value)
# Expected: the failing check now PASSES; no unintended files changed; the fix preserved the dustin/ org
# and did NOT revert homebrew_casks -> brews.
```

### Level 4: Release-time prerequisite flagging (the deliverable's release-gate section)

```bash
# These are NOT fixed by S2 — they are documented as release-time gates. Confirm each is a known, documented
# gap (not a surprise):

# (1) remote reconciliation: remote is dabstractor/, canonical is dustin/
git remote -v | grep origin              # dabstractor/stagecoach — repo must be reachable at dustin/stagecoach pre-tag
grep -n 'dustin/stagecoach' .goreleaser.yaml | head -1   # the documented owner note (.goreleaser.yaml:5-8)

# (2) install.sh absent pre-release (documented in docs/README.md:27)
test -e install.sh && echo EXISTS || echo ABSENT   # ABSENT (expected pre-release; published at first release)

# (3) distribution taps/buckets must exist before goreleaser publishes
grep -nE 'homebrew-tap|scoop-bucket' .goreleaser.yaml   # dustin/homebrew-tap, dustin/scoop-bucket (repos goreleaser pushes to)

# (4) AUR account + SSH key (if `aurs:` kept)
grep -n 'AUR_SSH_PRIVATE_KEY' .goreleaser.yaml          # the secret the aurs pipe needs
# Expected: all four flagged in the verification record as pre-release gates; none acted on by S2.
```

## Final Validation Checklist

### Technical Validation

- [ ] All 5 checks (a–e) executed FRESH with their commands; each produced the expected output.
- [ ] `grep -rhoE 'github\.com/dustin/stagecoach' … | uniq -c` → a single `dustin/stagecoach` line (N≥10).
- [ ] `grep -rni 'stagehand' …` over the distribution surface → clean; `dabstractor` only in goreleaser comments.
- [ ] Channel names agree (homebrew-tap↔dustin/tap; scoop-bucket+stagecoach↔dustin/stagecoach; stagecoach-bin).
- [ ] `make build && ./bin/stagecoach providers list` → 8 providers, stagecoach branding; `--version` = `stagecoach …`.
- [ ] Any straggler fix (if needed) is a pure rename preserving `dustin/`, scoped to one file, re-verified green.

### Feature Validation

- [ ] (a) every URL in README/docs/.goreleaser.yaml/go.mod = `github.com/dustin/stagecoach` (no split).
- [ ] (b) Homebrew tap + Scoop bucket + AUR package names consistent across goreleaser + README + docs.
- [ ] (c) install.sh URL consistent; absence documented (docs/README.md:27); flagged, not created.
- [ ] (d) namespace confirmed `dustin/`; remote mismatch documented as a pre-release gate; docs NOT changed to dabstractor.
- [ ] (e) binary builds + lists all providers under `stagecoach` branding.
- [ ] `homebrew_casks:` confirmed CORRECT (v2.10) — recorded, NOT reverted.
- [ ] Release-time prerequisites flagged (remote reconciliation; install.sh; tap/bucket creation; AUR key).

### Code Quality & Scope Validation

- [ ] `git status --short` shows no changes (clean verification) or ≤ a few targeted straggler fixes.
- [ ] NO new files created (unless an inconsistency required a fix). install.sh NOT created.
- [ ] Scope respected: PRD.md, go.mod, Go source NOT modified; S1's internal checks referenced, not duplicated;
      plan/ and .pi-subagents/ not verified (P1.M5.T1.S1's territory).
- [ ] `homebrew_casks` NOT reverted to `brews:` (verified correct per goreleaser v2.10).

### Documentation & Deployment

- [ ] Verification record (implementation summary) captures each check's command + output + verdict.
- [ ] The namespace resolution (canonical `dustin/` + the mechanical proof) is explicit in the record.
- [ ] The `homebrew_casks` correctness + the release-time prerequisites are explicit (so no future maintainer
      reverts the pipe or is surprised by the remote/install.sh gaps).

---

## Anti-Patterns to Avoid

- ❌ Don't change the docs/goreleaser to `dabstractor/` to "match the remote" (contract check (d) literal
      reading). Canonical is `dustin/` (go.mod + PRD + goreleaser owner); changing docs breaks `go install`
      (resolves at go.mod's declared path). The remote is the pre-release gate. See §"Namespace resolution".
- ❌ Don't revert `homebrew_casks:` to `brews:`/formulas. goreleaser v2.10 DEPRECATED formulas for
      precompiled binaries; casks are now the recommendation (web-verified). Reverting reintroduces a
      deprecated pipe. See §"homebrew_casks is CORRECT".
- ❌ Don't create `install.sh`. It's a release artifact (absent pre-release, documented in docs/README.md:27).
      S2 flags its absence; it does not author the script.
- ❌ Don't run a repo-wide `sed`. Scope any straggler fix to the SINGLE offending file, and only if a check
      actually FAILS. Research found none blocking.
- ❌ Don't trust the research "PASS" table without re-running the checks. Research is a snapshot; the parallel
      S1 / P1.M5.T2.S2 could in principle touch a doc. Re-verify fresh.
- ❌ Don't include `plan/`, `.pi-subagents/`, or PRD.md in the straggler greps and then "fix" the hits —
      they are historical research records / read-only spec, NOT user-facing distribution surface, and are
      P1.M5.T1.S1's scope.
- ❌ Don't duplicate S1's internal-identity checks (lazygit/git-alias/EDITMSG). S1 = internal consistency;
      S2 = distribution-path correctness + smoke test. Reference, don't repeat.
- ❌ Don't "complete" the README scoop instructions by adding `scoop bucket add` — the one-liner
      `scoop install dustin/stagecoach` matches PRD §21.3 verbatim and is the standard Go-CLI shorthand.
      Changing it is a docs-UX call, out of scope.
- ❌ Don't create a new doc file "to document the verification". The deliverable is the verification RECORD
      (implementation summary); a redundant doc risks colliding with future doc-review work.
- ❌ Don't edit PRD.md (read-only) or go.mod (the namespace oracle — verified, not changed).
- ❌ Don't declare a check PASS without the command EVIDENCE in the record. "It looked fine" is not verification.
```
