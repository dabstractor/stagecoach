# P3.M1.T1.S1 — research findings

Mode B changeset-level docs task: sync README + FUTURE_SPEC + how-it-works security narrative +
.goreleaser comments for v3.0 (self-update + expanded distribution). **No code changes** — Markdown
+ YAML comments only.

This file is the verified-fact ledger the PRP cites. Every quote below was read directly from the
working tree on 2026-08-06.

---

## §0. Task scope boundary (what NOT to do)

- **Does NOT duplicate** docs/cli.md `upgrade` command docs (P1.M4.T1.S1, Mode A — done) or
  docs/configuration.md `[upgrade]` table docs (P1.M1.T2.S1, Mode A — done). The README's
  "Updating" subsection + FAQ must LINK to those, not re-derive the flag table.
- **OUTPUT constraint**: "Modified README.md, FUTURE_SPEC.md, docs/how-it-works.md (or README
  security section), .goreleaser.yaml comments. **No code changes.**" → .go, .sh, install scripts,
  flake.nix, npm/*, release.yml, ci.yml, providers/*.toml are ALL read-only.
- .goreleaser.yaml edits are **YAML comments only** (the `#` kind), never a field/value change
  (goreleaser config is release infra owned by P2/release).
- This is the LAST task in plan 017 (the v3.0 plan). P1 (self-update core) = Complete; P2
  (distribution) = Complete/Implementing; this P3 syncs the public docs to match.

---

## §1. The §19 scoping language (the amendment) — VERBATIM sources

The exact narrative to reproduce (in the README FAQ + how-it-works.md Safety invariant):

**v3_scope_boundary.md "§19 'no network calls' — now scoped (amendment, not a relaxation)":**
> - The commit path (§9.1–§9.28) makes NO network calls — **unchanged**. Verified: `net/http` is
>   used nowhere in the repo today.
> - `stagecoach upgrade` is the **explicit, named exception**: it fetches ONLY the project's own
>   GitHub release artifacts + checksums. Never provider credentials, never a diff, never repo data.
> - The security narrative in README/docs must reflect this scoping (Mode B doc task): "no network
>   calls on the commit path; `stagecoach upgrade` is the one exception and it touches only release
>   artifacts."

**PRD §19 (Security), "Diff content is local" bullet (already amended in the merged PRD):**
> Stagecoach's commit-generation path (§9.1–§9.28) makes no network calls itself; the sole exception
> is `stagecoach upgrade` (§9.29), which fetches the project's own release artifacts and checksums
> from GitHub Releases — never provider credentials, never a diff, never repo data.

**PRD §9.29 intro (2nd para):**
> It is also the one stagecoach surface that makes network calls — §19's "stagecoach makes no
> network calls itself" is scoped to the commit path (§9.1–§9.28), and this section is its explicit,
> named exception (the call fetches only the project's own release artifacts and checksums; never
> credentials, never a diff, never repo data).

**The phrasing to land in user-facing docs** (from v3_scope_boundary.md): *"no network calls on the
commit path; `stagecoach upgrade` is the one exception and it touches only release artifacts."*

---

## §2. README.md — current text to edit (exact, with line numbers as of 2026-08-06)

### 2a. `## Install` block (lines 82–115) — the main rewrite

```
82: ## Install
83:
84: **Prerequisite:** a coding-agent CLI already installed and on `$PATH` ...
85:
86: > [!NOTE]
87: > Stagecoach is pre-release and still being tested locally — **build from source** is the only
88: > working install method today. The package-managed channels below are coming with the first
89: > public release.
90:
91: ### Build from source (works today)
... (lines 92–106: make install + verify + symlink TIP) ...
107:
108: ### Coming soon
109:
110: These will land with the first release, once the tap/bucket repos are published:
111:
112: - **Homebrew** (macOS / Linuxbrew) — `brew install dabstractor/tap/stagecoach`
113: - **Scoop** (Windows) — `scoop install dabstractor/stagecoach`
114: - **`go install`** — `go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest`
115: - **Direct binary** (curl​|​sh one-liner) — `curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash`
```

REWRITE TARGETS:
- The `> [!NOTE]` (lines 86–89): the "build from source is the only working method / coming soon"
  claim is now FALSE for v3.0. Rewrite to reflect that the package-managed channels are live.
- `### Build from source (works today)` (line 91): demote — it is now ONE option among many, not
  "the only working method". Keep the block (still valid), drop the "(works today)" framing.
- `### Coming soon` (lines 108–115): REPLACE entirely with the live install commands for ALL 8
  channels (Homebrew/Scoop/Winget/npm/Nix/mise·asdf/go-install/curl|sh) per PRD §21.3 (see §4 below).
- ADD a new `### Updating` subsection (after the install commands, before `## Quick start` at line
  117) documenting `stagecoach upgrade`.

### 2b. The v3.0 version note (line 6)
```
6: A snapshot-based AI commit message generator that uses YOUR local CLI agent. v2.1 adds payload
   exclusions, message shaping, git hook mode, editor/git integrations, `--edit`/`--push`, and model
   discovery — see [Features](#features) below.
```
This is a "what's new" one-liner. For accuracy it should append a v3.0 clause (self-update +
expanded distribution). OPTIONAL but recommended for doc consistency — the README otherwise
contradicts the FUTURE_SPEC reconciliation (which says self-update shipped).

### 2c. FAQ — `### Does it send my code anywhere new?` (lines 347–349) — the security narrative
```
347: ### Does it send my code anywhere new?
348:
349: No. It shells out to *your* agent under *your* existing auth and billing. Stagecoach never opens
350: an HTTP connection to any API — your agent does, exactly as it would if you ran it manually.
```
The sentence "Stagecoach never opens an HTTP connection to any API" is now OVERBROAD — it is false
for `stagecoach upgrade`. Scope it to the commit path + name the upgrade exception (§1 language).
This is the README security narrative the task means by "(c) README security narrative".

### 2d. FAQ — ADD `### Does stagecoach make network calls?` (new entry, per task)
A NEW FAQ entry is explicitly required by the task, citing the §19 scoping. Place it adjacent to
"Does it send my code anywhere new?" (the security cluster). Content: commit path = none; upgrade =
the one named exception (release artifacts + checksums only; never credentials/diff/repo data).
Cross-link docs/cli.md#upgrade.

### 2e. FAQ — `### What about PR generation, ... self-update ...` (lines 377–379) — reconcile
```
377: ### What about PR generation, editor extensions, a GitHub Action, API-key providers?
378:
379: Stagecoach writes commit messages — nothing else. Ideas we considered but deferred or rejected —
380: VS Code/neovim extensions, a GitHub Action, gitui integration, API-key HTTP providers, generate-
381: N-and-pick, diff chunking, self-update, and more — each with its reason — live in FUTURE_SPEC.md.
```
"self-update" in this list (line 381) is now WRONG — self-update SHIPPED in v3.0 (`stagecoach
upgrade`, §9.29). Remove "self-update" from the deferred/rejected list (it is the one item that
graduated). Keep the rest.

---

## §3. FUTURE_SPEC.md — the self-update row (line 101) — reconcile to SUPERSEDED

Current row (line 101), under "## 3. Rejected — deliberate, with reasons":
```
| **Self-update command** (aicommits) | Distribution is Homebrew/AUR/Scoop/`go install`; a self-updating binary fights its package manager and breaks checksums. |
```

The task: "update the 'Self-update command' rejection row (line ~101) to note it is SUPERSEDED by
PRD §9.29 (v3.0) with the delegate-first rationale, mirroring Appendix F's superseded entry."

**Appendix F text to mirror (PRD line 2456):**
> **Self-update SUPERSEDED (v3.0; was rejected v2.1).** The v2.1 rejection ("package managers own
> binary updates") assumed self-update meant a tool that downloads and overwrites its own binary —
> which fights whatever package manager installed it and gets silently reverted on that manager's
> next upgrade. With the distribution surface widening to npm/Winget/Nix/mise/asdf on top of
> Brew/Scoop/AUR/go-install, users genuinely lose track of which channel they used, so the concern
> got sharper, not softer. v3.0 reverses the rejection on the strength of the **inverse architecture
> — install-method-aware, delegate-first** (§9.29): `stagecoach upgrade` detects the install method
> and delegates to that channel's native updater wherever one exists, prints the command where it
> needs privileges (AUR) or is declarative (Nix), and self-swaps ONLY for the direct-binary channel.
> It never overwrites a package-manager-owned file, so it cannot fight a manager.

**In-file precedent for a graduated/superseded row** (the chunking row, line ~99): keep the row,
add a NOTE that it "has graduated to the spec — see PRD §X.XX (FR-…)"; clarify what the rejection
still applies to. Follow that exact shape for self-update (keep the row, prepend **SUPERSEDED**,
cite §9.29 FR-U1–U12 + Appendix F, give the delegate-first rationale, note the original concern is
resolved by never overwriting a manager-owned file).

The FUTURE_SPEC footer ("When promoting anything out of this file ... delete it here") is OVERRIDDEN
by the explicit task instruction to MIRROR APPENDIX F's superseded entry (keep + annotate, not
delete) — Appendix F keeps its superseded entry rather than deleting it.

---

## §4. PRD §21.3 — the canonical install commands for all 8 channels (the README source of truth)

From PRD §21.3 (lines 2203–2228), VERBATIM — reproduce these in the README:
```bash
# Homebrew (macOS / Linuxbrew)
brew install dabstractor/tap/stagecoach

# Go install (anywhere with Go)
go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest

# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash

# Windows (Scoop)
scoop install dabstractor/stagecoach

# Windows (Winget — the Win11 default)
winget install dabstractor.stagecoach

# npm (zero-install trial: npx stagecoach; or global install)
npm install -g @dabstractor/stagecoach

# Nix (flake — no channel/registry needed)
nix profile install github:dabstractor/stagecoach

# mise / asdf (version-manager users)
mise use stagecoach@latest   # or: asdf plugin add stagecoach && asdf install stagecoach latest
```

Note: the README's existing "Coming soon" order was Homebrew/Scoop/go-install/curl|sh. PRD §21.3
groups by platform/audience (macOS+Linux, then Windows×2, then npm, Nix, mise/asdf, curl|sh). Use
the §21.3 grouping. The task's channel list "Homebrew/Scoop/Winget/npm/Nix/mise/asdf/go-install/
curl|sh" is the SET; the §21.3 commands are the canonical wording. mise+asdf share ONE command line
in §21.3 (mise asdf-compat — the same plugin scripts serve both; see P2.M4.T1.S1 PRP).

---

## §5. ⚠️ CRITICAL GAP: `install.sh` does NOT exist

VERIFIED (exhaustive `find . -iname install.sh` over the working tree, excluding .git): **there is
no `install.sh` anywhere in the repo.** Yet PRD §21.3 AND the current README both give the curl|sh
one-liner `curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash`.

Evidence the gap is known-but-unfilled:
- `plan/017_397abce9deb1/architecture/system_context.md:49`: "No `install.sh`, `package.json`,
  `flake.nix`, winget manifests, or mise/asdf plugin scripts." (the "before" state the plan built
  FROM — package.json/flake.nix/winget/mise-asdf were built by P2; install.sh was NOT).
- None of the P2 PRPs (P2.M1 npm, P2.M2 winget, P2.M3 nix, P2.M4 mise/asdf) create install.sh.
- `grep -rn install.sh plan/017_397abce9deb1/` → only prd_snapshot (the §21.3 command) + system_context.

**Implication for THIS task**: the curl|sh channel's bootstrap (install.sh) is a missing release
artifact. Creating it is a code/script change, which is OUT OF SCOPE for this Mode B docs task
(OUTPUT: "No code changes").

**Resolution (the PRP's instruction)**:
1. Document the curl|sh one-liner per PRD §21.3 VERBATIM in the README — §21.3 is the canonical
   source of truth and the task INPUT explicitly lists curl|sh as a channel. Do NOT invent a
   different command.
2. Do NOT create install.sh (out of scope — "No code changes").
3. FLAG the gap as a residual risk in the PRP + record it in the subtask completion: install.sh
   MUST be created before the v3.0 release for the curl|sh channel to function. The README line is
   correct per spec; install.sh is a release-day deliverable tracked separately.
4. The other 7 channels (Homebrew/Scoop/Winget/npm/Nix/mise/asdf/go-install) have working plumbing
   (goreleaser brews/scoops/aurs + release.yml npm-publish/winget jobs + flake.nix + plugins/
   asdf-stagecoach) — they are the "live" channels.

---

## §6. docs/how-it-works.md — "Safety invariant" section (line 195) — the technical security narrative

Current text (line 197):
```
No provider mutates the repository (PRD §18.1). Every built-in manifest constrains the agent to a
read-only mode ... The agent receives the diff via stdin/argv and writes the commit message to
stdout — it never runs `git add`, `git commit`, or any write command. ...
```
This is the TECHNICAL security narrative (vs. the README's USER narrative). The §19 scoping belongs
here too (different audience): append a sentence that the commit-generation path makes no network
calls itself, with `stagecoach upgrade` (§9.29) as the one named exception (release artifacts only).
This is the "(c) docs/how-it-works.md" half of the task (do BOTH the README FAQ §2c/2d AND this —
they serve different readers and both are cheap).

---

## §7. docs/cli.md `upgrade` + docs/configuration.md `[upgrade]` — what is ALREADY documented (DO NOT duplicate)

These are the Mode A per-command/per-table docs (done in earlier tasks). The README's "Updating"
subsection must LINK to them, not re-derive:

**docs/cli.md `### upgrade`** (lines 400–432): full flag table (--check/-c, --version, --prerelease,
--force, --rollback, --install-method, --yes/-y, --channel, --source-repo), flag-contract rules,
repo-independent note, the Network bullet ("upgrade is the one named exception to the no-network-
calls commit path"), examples, exit codes 0/1/6. → README links `docs/cli.md#upgrade`.

**docs/configuration.md `[upgrade]` table** (lines 121–163): `channel`/`source_repo` keys, global-
only note, `CurrentConfigVersion` unchanged (3), `LoadUpgradeConfig()` reader. → README's "Updating"
does NOT need the config detail (it's a per-file doc); a one-line mention + link suffices.

**Exit codes** (docs/cli.md lines 437–446 + 446): 6 = Update available (upgrade-path only). The
README FAQ/Updating only needs the --check→exit 6 fact for the CI/cron mention, not the full table.

---

## §8. .goreleaser.yaml — comment placement (YAML `#` comments only; no field/value edits)

The task: "add comments under release/brews/scoops pointing at the 'Beyond goreleaser' channels
(npm/winget/nix/mise-asdf) implemented in Phase 2 CI steps, so a future reader sees the full
distribution picture in one place."

Comment-anchor locations (current line numbers; the FILE HEADER already cites PRD §21.2/§21.3):
- **`release:` block** (line ~50): the GitHub Release is the single source ALL channels fetch from.
  Add a comment that goreleaser publishes ONLY brews/scoops/aurs + archives/checksums here; the
  other channels (npm/winget/nix/mise-asdf) are "Beyond goreleaser" — implemented as separate CI
  steps (see release.yml npm-publish + winget jobs; flake.nix; plugins/asdf-stagecoach/).
- **`brews:` block** (line ~62): note this is ONE of goreleaser's native pipes; cross-reference the
  beyond-goreleaser channels.
- **`scoops:` block** (line ~78): same cross-reference.
- **`aurs:` block** (line ~95, already commented): optionally add the same pointer for completeness
  (it's best-effort; see release.yml).

The "Beyond goreleaser" phrase comes from PRD §21.2 ("Beyond goreleaser") and external_deps.md §3
(release.yml npm-publish comment header: "npm wrapper publish (PRD §21.2 'Beyond goreleaser')").
Use that exact framing so the comments are discoverable by that name.

GOTCHA: the goreleaser file header ALREADY has an owner note about dabstractor/stagecoach + the tap
and scoop-bucket repos. New comments must NOT contradict it or duplicate the brew/scoop/aur field
comments already present. They ADD the "here are the OTHER channels" picture.

---

## §9. release.yml + sibling pointers for the goreleaser comments (verified)

The beyond-goreleaser channels and where they live (for the .goreleaser comments to point at):
- **npm** → `release.yml` job `npm-publish` (publishes `@dabstractor/stagecoach`; npm/ wrapper).
- **winget** → `release.yml` job `winget` (`vedantmgoyal9/winget-releaser@v2` → microsoft/winget-pkgs
  manifest PR; `dabstractor.Stagecoach`). Docs: docs/packaging.md.
- **nix** → `flake.nix` (repo root; `nix profile install github:dabstractor/stagecoach`). Docs:
  docs/packaging.md "Nix (flakes)".
- **mise/asdf** → `plugins/asdf-stagecoach/` (canonical source; mirrored to
  `dabstractor/asdf-stagecoach`; P2.M4.T1.S1, the parallel sibling — its PRP is the contract).

curl|sh + go-install + Homebrew/Scoop/AUR are goreleaser-native (brews/scoops/aurs) or direct
(GitHub Releases archive); the comments name npm/winget/nix/mise-asdf as the "Beyond goreleaser"
four (matching PRD §21.2 + the task's parenthetical).

---

## §10. Consistency checks the PRP must enforce

- The README "Updating" subsection + FAQ must NOT contradict docs/cli.md's `upgrade` flag table
  (the README is a summary + link; cli.md is authoritative).
- The §19 scoping must use IDENTICAL framing across README FAQ + how-it-works.md (commit path =
  none; upgrade = named exception, release artifacts only) — use §1's verbatim phrasing.
- FUTURE_SPEC self-update row + README "What about…" list + the line-6 version note must all agree:
  self-update SHIPPED in v3.0 (no source may still call it deferred/rejected).
- The README install commands must match PRD §21.3 wording exactly (§4).
- .goreleaser comments are additive `#` lines only — `goreleaser check` must still pass (comments
  never break YAML parsing, but validate anyway).

---

## §11. Validation approach (this is a Markdown/YAML-comment task)

No `go build`, no tests, no ruff/mypy/pytest (those are the template's Python defaults — N/A). The
validation loop is:
1. `markdownlint` (the repo has `.markdownlint.json` — MD013 line-length may be relaxed; check).
2. Link integrity: the README internal/external links resolve; the docs/cli.md#upgrade anchor
   exists; the curl|sh/FUTURE_SPEC links are valid.
3. `.goreleaser.yaml` still parses (`goreleaser check` if available, else a YAML parse).
4. grep-guards: self-update removed from README's deferred list; §19 scoping present in both
   security narratives; FUTURE_SPEC row says SUPERSEDED; goreleaser has the beyond-goreleaser
   pointer comments; git scope = only the 4 files.
5. Consistency grep: no source still calls self-update "deferred"/"rejected"/"coming soon".