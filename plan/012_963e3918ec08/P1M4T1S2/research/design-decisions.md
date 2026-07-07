# P1.M4.T1.S2 — Design Decisions

Rename docs/*.md (cli.md, configuration.md, how-it-works.md, providers.md, README.md) from stagehand →
stagecoach — Layer 5.2 of the project rename (plan 012). The Go code (M1–M3) is ALREADY renamed; the docs
must MATCH it. This is Mode A: the docs ARE the updates. Runs in parallel with P1.M4.T1.S1 (repo-root
README.md) — disjoint files, no conflict.

---

## §0 — Scope: exactly 5 files in docs/, ~526 references, one global sed

The 5 files (from `ls docs/` + rename_surface_map.md §5.2), with per-file case-variant counts:

| File | lower | Title | UPPER | Total |
|------|-------|-------|-------|-------|
| docs/cli.md (~32KB) | 182 | 4 | 52 | 238 |
| docs/configuration.md (~24KB) | 145 | 4 | 59 | 208 |
| docs/how-it-works.md (~34KB) | 40 | 13 | 0 | 53 |
| docs/providers.md (~16KB) | 7 | 3 | 0 | 10 |
| docs/README.md (~4KB, docs index) | 13 | 3 | 1 | 17 |
| **TOTAL** | | | | **~526** |

The contract's sed (verbatim): `sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g;
s/STAGEHAND/STAGECOACH/g' docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md
docs/README.md`. Three case-disjoint patterns ⇒ order-independent, no double-substitution. (Grep confirmed
ZERO non-canonical case variants like `stageHand`/`StageHand` in any docs file.)

docs/README.md is the DOCS INDEX — it is a DIFFERENT file from the repo-root README.md (P1.M4.T1.S1's
target). Both are renamed; neither overlaps.

## §1 — The sed is complete + safe (no manual per-occurrence edits needed)

Every "stagehand"/"Stagehand"/"STAGEHAND" in the 5 files is a rename target — there are NO exceptions to
preserve:
- **No provenance/rename-history notes.** The only "originally" hit is `how-it-works.md:212 "To commit the
  originally staged files manually:"` — that's the snapshot/rescue recipe ("the files as they were staged"),
  NOT a rename-history note. The rename-history note lives ONLY in PRD h2.30 (READ-ONLY, not a docs file).
- **No `commit-pi` refs** in docs (grep confirmed) — nothing foreign to preserve.
- **"stagehand" never appears as a substring of an unrelated word** that shouldn't change (it's always the
  project name, a config key prefix, an env prefix, a path segment, or the binary name).

So a global sed is correct: after it, `grep -ri stagehand docs/*.md` MUST return ZERO hits (the Layer-5 gate).

## §2 — The contract's review items (a)–(e) are VERIFICATION, not extra edits

The sed ALREADY produces every target in the contract's review checklist:
- (a) env vars → `STAGECOACH_` (the `s/STAGEHAND/STAGECOACH/g` branch; cli.md:52, configuration.md:59).
- (b) git config keys → `stagecoach.*` (the `s/stagehand/stagecoach/g` branch).
- (c) config paths → `~/.config/stagecoach/config.toml` + `.stagecoach.toml`.
- (d) exclusion file → `.stagecoachignore`.
- (e) CLI binary → `stagecoach`.
The review is a CHECK that the output matches the already-renamed Go code (M1–M3) — not additional edits.

## §3 — Cross-surface consistency: the docs must MATCH the renamed Go code (M1–M3 LANDED)

The Go code is already `stagecoach`. The docs (post-sed) must AGREE with these verified constants:
- **Integration marker**: docs/cli.md has 3 refs to `# stagehand-integration` (L299 YAML example, L318, L327)
  → sed → `# stagecoach-integration`. The Go constant `lazygitMarker` (internal/cmd/integrate_lazygit.go:20)
  is `"stagecoach-integration"` (VERIFIED). The sed produces exactly that. A mismatch would break
  `stagecoach integrate uninstall`'s idempotency lookup — VERIFY they match post-sed.
- **go-install path**: docs/README.md:17 `go install github.com/dustin/stagehand/cmd/stagehand@latest` → sed →
  `github.com/dustin/stagecoach/cmd/stagecoach@latest`. Must match go.mod (`module github.com/dustin/stagecoach`,
  VERIFIED) + the renamed `cmd/stagecoach` dir.
- **Exclusion file**: docs reference `.stagehandignore` → `.stagecoachignore`. Must match the Go constant
  `StagecoachIgnoreFile = ".stagecoachignore"` (internal/exclude/exclude.go:28, VERIFIED).

## §4 — Namespace is `dustin`, NOT `dabstractor` (same trap as S1, but docs are clean)

docs/README.md has 2 GitHub-namespace URL refs (the ONLY namespace refs in docs):
- L17 `go install github.com/dustin/stagehand/cmd/stagehand@latest`
- L20 `curl -fsSL https://github.com/dustin/stagehand/raw/main/install.sh | bash`
Both use owner `dustin` (the canonical namespace per go.mod + .goreleaser `owner: dustin` + PRD §21.3). The
sed yields `dustin/stagecoach`. docs contain ZERO `dabstractor` (grep confirmed) — so unlike S1's contract
trap, there's no wrong-namespace guidance to suppress here. Still VERIFY post-sed: zero `dabstractor`, all
namespace refs are `dustin/stagecoach`.

## §5 — Anchor-link safety: the sed renames heading + link fragment consistently in ONE pass (the key insight)

Several docs HEADINGS contain "stagehand", so their GitHub-generated anchor slugs contain it too — and other
docs files LINK to those anchors. CRITICAL: because the sed runs over ALL 5 docs files in ONE pass, it
renames BOTH the heading text (→ the slug) AND the link's `#fragment` consistently, so every intra-docs link
stays valid. Verified pairs:
- how-it-works.md:156 `### Payload exclusions (.stagehandignore)` → `### Payload exclusions (.stagecoachignore)`
  (slug `#payload-exclusions-stagehandignore` → `#payload-exclusions-stagecoachignore`); docs/README.md:42
  links `](how-it-works.md#payload-exclusions-stagehandignore)` → sed renames the fragment to match. ✓
- configuration.md:253 `### .stagehandignore` → `### .stagecoachignore` (slug `#stagehandignore` → `#stagecoachignore`).
- how-it-works.md:1 `# How Stagehand works` → `# How Stagecoach works`; docs/README.md:1 `# Stagehand documentation`
  → `# Stagecoach documentation`.

Cross-tree link safety (docs ↔ repo-root README): docs/README.md links `](../README.md)` + `](../README.md#contributing)`.
The repo-root README is S1's target (same sed). The filename `README.md` and the `#contributing` anchor
contain NO "stagehand" ⇒ unchanged ⇒ the link resolves. Any docs anchor a repo-root README link targets is
renamed on BOTH ends (S1 sed's the link, S2 sed's the heading) because both passes use the identical sed.
⇒ no broken cross-tree anchors.

VERIFY post-sed: `grep -rniE '^#{1,6} .*stagehand' docs/` → 0 (no heading still contains stagehand); and the
one known link fragment `#payload-exclusions-stagehandignore` → now `#payload-exclusions-stagecoachignore`
on both the heading and the docs/README.md link.

## §6 — Coordination with the parallel sibling (P1.M4.T1.S1) + downstream tasks

- **S1** (repo-root README.md) and **S2** (docs/*.md) edit DISJOINT files — no merge hazard. Both use the
  identical 3-branch sed. The docs/README.md index links to `../README.md`; S1 renames that file's content,
  the link target filename/anchor are stagehand-free ⇒ resolves.
- **DO NOT touch**: providers/*.toml (P1.M4.T2.S1), FUTURE_SPEC.md (P1.M4.T2.S2), plan/ artifacts
  (P1.M5.T1.S1), any .go/.yaml/Makefile/go.mod (already renamed M1–M3). `git status` must show ONLY the 5
  docs files.
- **Downstream**: P1.M5.T2.S1 (final grep audit) sweeps ALL tracked files for zero stagehand — this task
  lands docs' zero-hits ahead of that audit. The audit's gate: `grep -ri stagehand --include='*.md' ...` == 0.

## §7 — Mode A validation: zero-hits + cross-surface agreement + scope

Mode A (these ARE the docs updates). Validation = (a) `grep -ri stagehand docs/*.md` == 0 (the Layer-5 gate);
(b) the 5 review criteria each verified via grep (STAGECOACH_, stagecoach., the paths, .stagecoachignore,
the `stagecoach` binary); (c) cross-surface: docs/cli.md marker == Go `lazygitMarker`; docs/README.md
go-install path == go.mod; (d) anchor consistency: no heading contains stagehand, the known link fragment
matches its heading; (e) `git status` shows ONLY the 5 docs files; (f) `go build ./... && go test ./...`
green & unchanged (no code touched — the build/test is a no-op regression check confirming no .go file was
accidentally edited).
