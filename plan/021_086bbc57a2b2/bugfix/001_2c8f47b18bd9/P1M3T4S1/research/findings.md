# P1.M3.T4.S1 — Research Findings: Upgrade-Detection Docs Sync (BUG-004/BUG-007/BUG-008)

> DOC-ONLY review task. Goal: verify docs/packaging.md + README.md accurately reflect the upgrade-
> detection improvements from P1.M3.T1–T3, and correct any stale claim. Per the contract RESEARCH NOTE
> the three fixes are mostly INTERNAL behavior changes; this file records the per-surface verdict.

## §0 The corrected behavior (the fixes that LANDED — what "accurate" means)

- **BUG-004 (Major, P1.M3.T1.S1 — Complete)**: `cmdRunner.Run` (`internal/cmd/upgrade_run.go`) — the
  PRODUCTION detection runner — gained a per-query `context.WithTimeout` + `cmd.WaitDelay` (3s, mirroring
  `osRunner`'s `defaultQueryTimeout` in `internal/upgrade/detect.go:121`). A hung package-manager query
  (`brew list`, `scoop prefix`, etc.) no longer hangs `stagecoach upgrade` indefinitely. PURE INTERNAL.
- **BUG-007 (Minor, P1.M3.T2.S1 — Complete)**: `pathHeuristics` (`internal/upgrade/detect.go:370-377`)
  gained `/home/linuxbrew/.linuxbrew/Cellar/` → `ChannelBrew`. A Linuxbrew install is now detected as
  `brew` (was mis-detected as `direct` → self-swapped instead of delegating to brew).
- **BUG-008 (Minor, P1.M3.T3.S1 — Implementing)**: `releases.go` validates + segment-escapes `c.Repo`
  before building GitHub API paths (split-and-escape, NOT whole-string). PURE INTERNAL.
- **ChannelBrew delegation (verified, unchanged by the fixes)**: a brew install is detected via
  tier-(c) pathHeuristics (the 3 Cellar roots) OR the tier-(b) `brew list stagecoach` (exit-0) probe
  (detect.go:288, goos darwin+linux), then **DELEGATED** — `stagecoach upgrade` RUNS
  `brew upgrade stagecoach` directly (delegate_test.go:60: `{"brew", ChannelBrew, [["brew","upgrade",
  "stagecoach"]]}`). Brew is NOT a PRINT channel (printCommand at delegate.go:344 handles only
  AUR/Nix/Deb/Rpm/Chocolatey). Brew owns the binary under the Cellar → never self-swapped (FR-U1/FR-U4).

## §1 README.md — ALL ACCURATE → NO-OP (zero edits)

| Line | Text | Verdict |
|------|------|---------|
| L10 | "v3.0 adds `stagecoach upgrade` (delegate-first self-update) and expands distribution (Homebrew, Scoop, Chocolatey, PowerShell installer, npm, Nix, mise/asdf)" | ✅ Accurate. Lists brew as a channel. Unchanged by the fixes. |
| L90 | "# Homebrew (macOS / Linuxbrew)" + `brew install dabstractor/stagecoach/stagecoach` | ✅ **Already acknowledges Linuxbrew** as a supported install channel. The BUG-007 fix makes detection actually work, matching this doc — no stale claim. |
| L161 | "it detects how you installed it and delegates to that channel's own updater (Homebrew, Scoop, npm, mise, asdf, `go install`), prints the command where running it needs privileges (AUR) or is declarative (Nix), and self-swaps only for a direct (curl\|sh / manual) install…" | ✅ Accurate. Lists Homebrew under DELEGATE (correct — brew → `brew upgrade stagecoach`, run directly) and AUR/Nix under PRINT (correct). Does NOT reference PM-query timeouts or URL escaping → BUG-004/BUG-008 have no surface here. |
| L164 | `stagecoach upgrade  # detect → delegate (or self-swap), confirm, apply` | ✅ Accurate. |

**README verdict: ZERO stale claims.** It describes the upgrade behavior generically ("detects →
delegates"), which the fixes fulfill. BUG-004 (timeout) and BUG-008 (URL escaping) are internal and have
NO doc surface in README. README L90 already says "macOS / Linuxbrew". → **no-op for README.**

## §2 docs/packaging.md — NO stale claim, but ONE structural gap → small enrichment

Sweep of packaging.md (grep fast-path/upgrade/detect/brew/linuxbrew/timeout):
- Intro (L3-7): "Homebrew (tap `dabstractor/homebrew-stagecoach`), Scoop (…), and AUR (…) are all wired
  into `release.yml`". **Platform-AGNOSTIC** (no "macOS only" claim) and **publishing-focused** (release.yml
  pushes). Does NOT describe the `stagecoach upgrade` detection cascade. → NOT stale.
- `## Chocolatey` (L9-40): has a detailed `- **`stagecoach upgrade` behavior**:` block (L33): "detects a
  Chocolatey install and **prints** `choco upgrade stagecoach -y` … does **not** self-swap — Chocolatey
  owns the binary under `ProgramData\chocolatey`". ✅ Accurate (Chocolatey is a PRINT channel per
  printCommand).
- `### PowerShell installer` (L42): "the installer tags it `STAGECOACH_INSTALL_METHOD=direct` so
  `stagecoach upgrade` self-swaps it like any direct install". ✅ Accurate.
- `## Nix`, `## Documentation site`: no upgrade-detection claims. ✅.
- **BUG-004/BUG-008 surfaces**: ZERO. packaging.md never references PM-query timeouts, WaitDelay, or the
  GitHub releases URL/repo escaping. → no-op for those two bugs.

**The gap**: packaging.md is the MAINTAINER doc that documents per-channel "`stagecoach upgrade`
behavior" (Chocolatey has a block; PowerShell has one). The brew channel — a first-class delegate channel
that BUG-007 specifically improved (Linuxbrew now detected) — has NO such block (only a passing intro
mention as a publishing tap). The contract LOGIC step explicitly directs: "Review docs/packaging.md for
Homebrew/Linuxbrew coverage." packaging.md is platform-agnostic (not "macOS only"), so the literal
"if it mentions only macOS Homebrew" condition is borderline — but the channel's upgrade-detection
behavior is UNDOCUMENTED, and BUG-007 is an upgrade-detection improvement the OUTPUT
("user docs accurately reflect the upgrade detection improvements") asks us to capture. → **add a compact
`## Homebrew / Linuxbrew` section mirroring the Chocolatey block** (the actionable deliverable, §3).

## §3 The actionable change — exact placement + drop-in text (docs/packaging.md)

**Placement**: insert a new `## Homebrew / Linuxbrew` section immediately AFTER the intro paragraph
(closing paren of the AUR-disabled note, L7) and BEFORE `## Chocolatey` (L9). This groups it with the
package-manager channels at the top and matches the intro's mention order (Homebrew is listed first).

**Anchor**: the intro ends with:
```
…each pushes to its target repo on tag. (AUR publish is currently disabled: its `git_url` is commented out in
`.goreleaser.yaml` while aur.archlinux.org recovers.)

## Chocolatey
```
Insert the new section between the `)` line and `## Chocolatey`.

**Drop-in text** (mirrors the Chocolatey section's `- **`stagecoach upgrade` behavior**:` pattern; no
internal bug-ID jargon in the shipped doc):
```markdown
## Homebrew / Linuxbrew

The tap is `dabstractor/homebrew-stagecoach` (published by `release.yml` on every tag). The same `brew`
tool serves **both** macOS Homebrew (`/opt/homebrew` on Apple Silicon, `/usr/local` on Intel) **and**
Linuxbrew (`/home/linuxbrew/.linuxbrew`) on Linux.

- **`stagecoach upgrade` behavior**: `stagecoach upgrade` detects a brew install by its Cellar path
  (`/opt/homebrew/Cellar/`, `/usr/local/Cellar/`, or `/home/linuxbrew/.linuxbrew/Cellar/`) and
  **delegates** to `brew upgrade stagecoach`. Brew owns the binary under the Cellar, so it is never
  self-swapped (FR-U1/FR-U4); unlike Chocolatey, `brew upgrade` is user-space and is run directly
  (streaming its output), not printed.
```

**Accuracy cross-check** (every claim verified against the code):
- "both macOS Homebrew … and Linuxbrew" ← pathHeuristics has all 3 Cellar roots (detect.go:372-376).
- "delegates to `brew upgrade stagecoach`" ← delegate_test.go:60; ChannelBrew is NOT in printCommand
  (delegate.go:344) → it is RUN, not printed.
- "never self-swapped (FR-U1/FR-U4)" ← only ChannelDirect self-swaps (detect.go:50).
- "unlike Chocolatey … run directly … not printed" ← Chocolatey IS a printCommand channel (needs admin);
  brew is not.

## §4 Scope fences / no-conflict

- **DOC-ONLY**: no code, no tests, no spec/ (human-owned), no tasks.json/prd_snapshot, no cli.md.
- **ONE file changed**: `docs/packaging.md` (the new `## Homebrew / Linuxbrew` section). README.md gets
  ZERO edits (confirmed accurate §1). docs/cli.md is OUT of scope (the contract names packaging.md +
  README.md only; cli.md's `#upgrade` section is a flag reference, not a detection-cascade description,
  and was not flagged).
- **No conflict with parallel P1.M3.T3.S1**: that edits `internal/upgrade/releases.go` +
  `releases_test.go` (CODE). This item edits a shipped DOC. Different files; no merge conflict.
- BUG-004 + BUG-008 are **confirmed no-ops** for ALL docs (internal fixes, no doc surface).

## §5 Validation

Doc-only → no build/test impact. Validation = (a) markdown lint clean (`.markdownlint.json`; run
`npx markdownlint-cli2 docs/packaging.md` or rely on `make lint`); (b) the mkdocs site still builds
(`mkdocs build` — packaging.md is a docs-site page; a malformed heading/anchor breaks the build);
(c) grep guards: the new section names all 3 Cellar roots + `brew upgrade stagecoach` + "Linuxbrew";
(d) `git status --porcelain` == `docs/packaging.md` ONLY; (e) re-confirm README + packaging.md have no
stale PM-timeout / URL-escaping claim (sanity). `make test`/`make lint` remain green (no code touched).