# Delta PRD — Chocolatey channel + PowerShell installer (replacing Winget) + model-example cleanup

**Scope of this delta:** bring the **code, CI config, and user-facing docs** into conformance with
the v3.3 spec (the gap between the v3.2 PRD the prior session worked against and the current v3.3
spec). The headline change is a **Windows distribution channel swap**: Winget is removed and replaced
by **Chocolatey** (goreleaser-native `chocolatey:` pipe) + a **PowerShell installer** (`install.ps1`,
the Windows `curl|sh` analog). A secondary, small change refreshes the pi multi-backend **example
model** from the fictional `zai/glm-5.2` to the realistic `anthropic/claude-haiku` in the few code
comments/strings that still carry the old token.

The v3.3 revision is **distribution + upgrade-detection plumbing only** — no commit/CAS/rescue/lock/
index/provider surface (per the spec's own framing, spec/01-product.md v3.3 row).

---

## What changed in the spec (v3.2 → v3.3) — and what's already done

| Spec delta | Status in code | Action this delta |
| --- | --- | --- |
| **v3.3 — Winget REMOVED; Chocolatey added** (goreleaser-native `chocolatey:` pipe; `choco install stagecoach` / `choco upgrade`). Self-update detects Chocolatey (`choco list --local-only stagecoach` + `ProgramData\chocolatey` path) and **PRINTs** `choco upgrade stagecoach -y` (admin — never self-swap). (§9.29 FR-U1–U4, §21.2, §21.3; G26/G27) | ❌ **Spec-only.** `internal/upgrade/{detect,delegate}.go` still ship `ChannelWinget` + a `winget upgrade` RUN delegation; `.goreleaser.yaml` still has WinGet comments; `.github/workflows/release.yml` still runs the `winget` job. | **Implement** (Phase 1) |
| **v3.3 — PowerShell installer** (`install.ps1`, `irm \| iex`; Windows analog of `curl\|sh`; tags `STAGECOACH_INSTALL_METHOD=direct`, rides the existing direct self-swap channel). (§21.2 "Beyond goreleaser", §21.3) | ❌ `install.ps1` does not exist; no Windows `irm\|iex` path. | **Implement** (Phase 1) |
| **v3.3 — model example renamed** `zai/glm-5.2` → `anthropic/claude-haiku` (and `anthropic/claude-sonnet-4` → `anthropic/claude-haiku` as the opencode example) across the spec. (SPEC header, §12, §12.3, §12.6, §15.5, §16, App. B/F) | ⚠️ **Spec done; code stale.** Spec is fully renamed, but a handful of **code comments/strings** still carry `zai/glm-5.2` / `glm-5.2` (`providers/pi.toml`, `internal/cmd/config_init_interactive.go`, `internal/cmd/default_action.go`). The `config_test.go` v2→v3 migration fixtures use `zai/glm-5.2` as input — those test *mechanics* (the values are arbitrary) and are functionally fine, but should be refreshed for coherence. | **Cleanup** (Phase 2, small) |
| **v3.3 (undocumented in the revision row but present) — FR-D3 "fast by default" rewrite + FR-D4 table rewrite** (every role ships `fast` except stager=`mid`; pi/opencode ship BLANK; claude=`haiku`/`sonnet`; agy Flash-Low/Medium; cursor composer-2.5; qwen-code non-stager) | ✅ **Already shipped** (`5445ff2`, `8d3fc42`; `role_defaults_test.go` + `bootstrap_test.go` assert fast-by-default). | None |
| **v3.3 (undocumented but present) — opencode + cursor are now stager-capable** (§12.6 opencode `tooled_flags = ["--agent","build"]`; §12.7 cursor `tooled_flags = ["--trust","--yolo"]` + free-plan note) | ✅ **Already shipped** in `internal/provider/builtin.go` (`c9e5fb3`, `f88042c`, `5980a44`) + `docs/providers.md` (Phase 2 of the prior session). | None |

**Conclusion:** the only code/CI/doc gap is the **Winget → Chocolatey + PowerShell installer swap**
(Phase 1) plus a tiny **model-example consistency cleanup** (Phase 2). The fast-by-default models
and the opencode/cursor stagers are already implemented — the spec now merely *matches* the code, so
they carry no tasks.

---

## Phase 1 — Replace Winget with Chocolatey + add the PowerShell installer

### Why (one paragraph)

`microsoft/winget-pkgs` runs an install-in-a-clean-VM Microsoft Defender scan (`validationDefender`)
that hard-blocks the unsigned stagecoach binary on every release — an unbounded per-release tax the
project will not pay. The fix is to swap the Windows channel: **Chocolatey** (a goreleaser-native
`chocolatey:` pipe, in goreleaser since Nov 2022 — older than goreleaser's own winget pipe) reaches
the Windows package-manager audience via `choco install stagecoach` without winget-pkgs's Defender
gate, and a **PowerShell installer** (`irm | iex`) is the no-package-manager fallback (the
`rustup`/`starship`/`uv` pattern). Because `choco upgrade` needs admin, `stagecoach upgrade` must
**PRINT** the command (not run it) and **never self-swap** a choco-owned binary — mirroring the
AUR/.deb/.rpm print-don't-run rows. The PowerShell installer places a package-manager-unowned binary
and tags `STAGECOACH_INSTALL_METHOD=direct`, so it rides the existing direct self-swap channel
unchanged. **Spec is already final** (spec/01-product.md v3.3 row; §9.29 FR-U1–U4; §21.2/§21.3).

### What "done" looks like

- `stagecoach upgrade` detects a Chocolatey install (`choco list --local-only stagecoach` DB query +
  `ProgramData\chocolatey` path heuristic) and **PRINTs** `choco upgrade stagecoach -y` (exits 0),
  never running it and never self-swapping (FR-U1/FR-U4).
- No `ChannelWinget` constant, no `winget` detection query, no `winget upgrade` delegation remain.
- `.goreleaser.yaml` has a `chocolatey:` pipe section; the `.github/workflows/release.yml` `winget`
  job is removed; the `WINGET_TOKEN` secret dependency is gone.
- `install.ps1` exists at the repo root: `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex`
  detects arch, downloads the windows zip from the latest Release, SHA256-verifies against
  `checksums.txt`, extracts to a user-local dir, prepends to PATH, and exports
  `STAGECOACH_INSTALL_METHOD=direct`.
- Distribution docs (`docs/packaging.md`, `docs/cli.md`, `README.md`) say Chocolatey + PowerShell,
  never Winget.
- All upgrade unit tests are green with Chocolatey in place of Winget.

### Mode A docs (ride with the work)

- **`docs/packaging.md`** — its entire WinGet section (the `## WinGet (\`dabstractor.Stagecoach\`)`
  heading + the release-day checklist + the `wingetcreate` bootstrap) is **entirely stale** and must
  be replaced with a Chocolatey section (the goreleaser `chocolatey:` pipe; no per-release VM scan;
  `choco install stagecoach`) plus a short PowerShell-installer subsection. This rides with the
  release-plumbing task because the doc describes exactly that plumbing.
- **`docs/cli.md`** line ~404 — the `stagecoach upgrade` blurb's channel list ("Homebrew, Scoop,
  winget, …") must read Chocolatey. Rides with the code task.
- **`README.md`** — (a) the v3.0 features blurb (line ~6) "expands distribution (Homebrew, Scoop,
  **Winget**, npm, …)"; (b) the "Not yet available" note (lines ~132–134) about WinGet pending
  winget-pkgs acceptance. Both must move to Chocolatey (+ PowerShell installer); the "pending
  acceptance" caveat is deleted entirely (Chocolatey does not have an acceptance gate). Rides with
  the docs task.

### Milestone P1.M1 — Upgrade subsystem: Chocolatey replaces Winget

#### Task P1.M1.T1 — Replace Winget with Chocolatey in detect + delegate (+ unit tests)

**Story points:** 3 · **Dependencies:** none

**prd_selectors:** `§9.29` (FR-U1, FR-U2, FR-U3, FR-U4), `§21.2`, `§21.3`, `§6.1` (G26/G27)

**CONTRACT DEFINITION:**

1. **RESEARCH NOTE — the spec is final, the code is stale-but-present.** Verified this session:
   `internal/upgrade/detect.go` defines `ChannelWinget Channel = "winget"` (line 41), a Windows-only
   PM probe `{channel: ChannelWinget, goos: ["windows"], name: "winget", args: ["list"], confirm:
   grepConfirm("stagecoach")}` (line 272), and references it in the valid-channels list (line 64),
   the `--install-method` allowed-values slice (line 242), and the cascade comments. `delegate.go`
   RUNs winget: `case ChannelWinget: return [][]string{{"winget","upgrade","stagecoach"}}` (lines
   273–274) — winget is a **RUN** channel today, but Chocolatey must be a **PRINT** channel (admin).
   The PRINT channel set is `case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm:` (delegate.go:212)
   with `printCommand()` (delegate.go:342) carrying the per-channel text. The spec authoritatively
   specifies Chocolatey as **PRINT** (`choco upgrade stagecoach -y`; admin — FR-U4; never self-swap
   — FR-U1). No web search needed.

2. **INPUT.** The Channel constant block (`internal/upgrade/detect.go:39–50`), the `pmProbes` table
   (`detect.go:269–278`), the path-heuristic prefix table (`detect.go:350–353`: `{prefix:
   "/opt/homebrew/Cellar/", …}`, `{prefix: "\\scoop\\shims\\", channel: ChannelScoop}`, …), the
   `Delegate` dispatch switch (`delegate.go:205`), `runArgv` (`delegate.go:267`), and `printCommand`
   (`delegate.go:342`). The path table is cross-GOOS (prefixes normalized to `/` — see
   `detect.go:374–380`), so add the Chocolatey prefix using forward slashes
   (`ProgramData/chocolatey/`) to match the existing Scoop convention.

3. **LOGIC — surgical swap, not a new channel family.**
   (a) **detect.go:** rename `ChannelWinget` → `ChannelChocolatey Channel = "chocolatey"` (Windows;
     update the doc comment to cite FR-U2/U3 PRINT channel). In `pmProbes`, replace the winget probe
     with `{channel: ChannelChocolatey, goos: []string{"windows"}, name: "choco", args: []string{
     "list", "--local-only", "stagecoach"}, confirm: exit0Confirm}` (choco `list --local-only
     <pkg>` exits 0 iff installed — an `exit0Confirm` predicate, NOT a grep; matches the spec's
     `choco list --local-only stagecoach`). Update the valid-channels list (line 64) and the
     `--install-method` allowed-values slice (line 242). In the path-heuristic prefix table
     (`detect.go:350`), add `{prefix: "ProgramData/chocolatey/", channel: ChannelChocolatey}` (forward
     slash, cross-GOOS-normalized like Scoop). Update the four narrative comments that name winget
     (lines 7, 87, 150, 265, 283, 311).
   (b) **delegate.go:** REMOVE winget from the RUN set — delete the `case ChannelWinget: return
     [][]string{{"winget","upgrade","stagecoach"}}` arm in `runArgv` (lines 273–274). ADD Chocolatey
     to the PRINT set: extend the `case ChannelAUR, ChannelNix, ChannelDeb, ChannelRpm:` arm at
     `delegate.go:212` to include `ChannelChocolatey`, and add a `case ChannelChocolatey:` arm to
     `printCommand` (delegate.go:342) emitting `primary = "choco upgrade stagecoach -y"` and `full`
     = primary + a comment noting choco owns the binary (FR-U1) and needs admin (FR-U4), mirroring
     the AUR/.deb/.rpm comment style. Update the RUN/PRINT comments in delegate.go (lines 4, 74,
     195, 221) to name Chocolatey in the PRINT set and drop winget from the RUN list.
   (c) **internal/cmd/upgrade.go + upgrade_run.go:** update the two narrative comments naming winget
     (upgrade.go lines 4, 79; upgrade_run.go line 266) to the Chocolatey-equivalent PRINT channel.
   (d) **Tests:** in `internal/upgrade/detect_test.go`, convert the `"winget installed (windows)"`
     case (line ~221) + the GOOS-gating assertions (lines ~307–322 that assert the winget probe is
     banned on linux/darwin) to Chocolatey (`choco` probe, same Windows-only GOOS gate). In
     `delegate_test.go`, convert the `{"winget", ChannelWinget, …}` RUN-table row (line ~62) and the
     channel-set assertion (line ~338) to assert Chocolatey is a **PRINT** channel (move it out of
     the runArgv table test and into a printCommand test asserting `choco upgrade stagecoach -y`).
     Add a case asserting a choco PRINT delegation exits 0 and records the command (mirror the
     existing AUR/.deb/.rpm printCommand tests).

4. **OUTPUT.** `go test ./internal/upgrade/...` green; `ChannelWinget` and the string `winget` are
   gone from non-test production code under `internal/upgrade/` (a `rg -n winget
   internal/upgrade/` returns zero non-comment hits, and ideally zero hits outside archived test
   fixtures). No behavioral change to any non-Windows channel; the direct self-swap path is
   untouched.

---

### Milestone P1.M2 — Release plumbing: goreleaser Chocolatey pipe, drop the WinGet CI job, add install.ps1

#### Task P1.M2.T1 — goreleaser `chocolatey:` pipe + remove the WinGet CI job + add install.ps1

**Story points:** 3 · **Dependencies:** P1.M1.T1 (so the install-method story is coherent end-to-end)

**prd_selectors:** `§21.2`, `§21.3`, `§6.1` (G27), App. F (decision log)

**CONTRACT DEFINITION:**

1. **RESEARCH NOTE.** `.goreleaser.yaml` has native `brews:` (line 89), `scoops:` (100), `nfpms:`
   (121), `aurs:` (146) pipes and four WinGet **comments** (lines 78, 87, 104, 142) listing WinGet
   as a "Beyond goreleaser" channel. The actual WinGet automation lives in
   `.github/workflows/release.yml` as a `winget:` job (lines ~109–139) using
   `vedantmgoyal9/winget-releaser@v2` + a `WINGET_TOKEN` secret. goreleaser's `chocolatey:` pipe is
   native (docs: goreleaser.net/customization/chocolatey) and publishes a `.nupkg` to a Chocolatey
   source — confirm the exact 2026 field names (`package_name`, `owners`, `title`, `url_template`,
   `icon`, `copyright`, `license_url`, `require_license_acceptance`, `project_url`, `description`,
   `release_notes`, `api_key`, `source_repo`) against `goreleaser check` / current goreleaser docs at
   implementation (FR-D5 discipline). The `install.ps1` follows the `rustup`/`starship`/`uv` pattern;
   reference `install.sh` (the existing Unix one-liner) for the SHA256-verify-against-checksums +
   release-asset-resolution shape. No web search beyond confirming the goreleaser `chocolatey:`
   schema is needed.

2. **LOGIC.**
   (a) **`.goreleaser.yaml`:** add a `chocolatey:` section (place it alongside the other native
     pipes, e.g. after `nfpms:`/`aurs:`). It must publish a `.nupkg` for the windows builds to the
     Chocolatey community source; gate the API key behind `{{ .Env.CHOCOLATEY_API_KEY }}` (a new
     secret). Per the §21.2 spec note, choco installs to `ProgramData\chocolatey`. Update the four
     WinGet comments (lines 78, 87, 104, 142) to name Chocolatey (goreleaser-native pipe) instead of
     WinGet (which was "Beyond goreleaser"). Run `goreleaser check` and confirm green (the
     DECISION-GATE pattern from the existing `aurs:` block applies: if `goreleaser check` rejects a
     field, comment it out and ship the rest — Chocolatey must not block the core contract).
   (b) **`.github/workflows/release.yml`:** DELETE the entire `winget:` job (lines ~109–139) and its
     `WINGET_TOKEN` secret references. If a Chocolatey publish step is wanted in CI (the goreleaser
     `chocolatey:` pipe runs in the main goreleaser job, so this may be a no-op), add the
     `CHOCOLATEY_API_KEY` to the goreleaser job's env. Do NOT add a winget-releaser-style external
     PR job for Chocolatey (the native pipe publishes directly).
   (c) **`install.ps1` (new, repo root):** a PowerShell script invokable via
     `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex`. It must: detect
     arch (`$env:PROCESSOR_ARCHITECTURE` → amd64/arm64); resolve the latest release tag (GitHub
     Releases API, unauthenticated, mirroring the upgrade `releases.go` resolution); download
     `stagecoach_<v>_windows_<arch>.zip` + `checksums.txt`; **SHA256-verify** the zip against the
     checksums line (hard gate — abort on mismatch, like `internal/upgrade/download.go`); extract
     `stagecoach.exe` to `$LOCALAPPDATA\stagecoach`; prepend that dir to the **user** `PATH`
     (`[Environment]::SetEnvironmentVariable(..., 'User')`, not Machine — no admin); and set
     `STAGECOACH_INSTALL_METHOD=direct` in the **user** environment so `stagecoach upgrade`
     recognizes it as a self-swap-eligible direct install (FR-U2/U5). Print a "re-open your shell"
     notice on success. Keep it dependency-free (no PowerShell gallery modules). Add a short header
     comment cross-referencing spec §21.3.

3. **OUTPUT.** `goreleaser check` passes with the `chocolatey:` section; `rg -n winget .github/
   .goreleaser.yaml` returns zero hits; `install.ps1` exists and a dry-run (`-DryRun` or a syntax
   `powershell -NoExec -Command "…parse…"`) confirms it is valid PowerShell.

---

### Milestone P1.M3 — Distribution docs sync (Mode A) + changeset README sweep (Mode B)

#### Task P1.M3.T1 — Sync docs/packaging.md, docs/cli.md, README.md off Winget → Chocolatey + PowerShell

**Story points:** 2 · **Dependencies:** P1.M2.T1 (so the docs describe the plumbing that now exists)

**prd_selectors:** `§21.2`, `§21.3`, `§15.3` (upgrade subcommand), App. F

**CONTRACT DEFINITION:**

1. **RESEARCH NOTE.** Verified stale Winget references this session: `docs/packaging.md` (lines 4, 8,
   10–73 — the whole `## WinGet` section + release-day checklist + `wingetcreate` bootstrap);
   `docs/cli.md` line ~404 (`stagecoach upgrade` channel list); `README.md` lines ~6 (v3.0 features
   blurb), ~132–134 ("Not yet available: Windows WinGet … pending acceptance into
   microsoft/winget-pkgs"). The spec (spec/06-reliability.md §21.2/§21.3, already updated by
   b0105e5) is the source of truth for the replacement text.

2. **LOGIC — surgical edits only (do not reflow unrelated sections).**
   (a) **docs/packaging.md:** replace the `## WinGet` section with `## Chocolatey` — describe the
     goreleaser-native `chocolatey:` pipe, `choco install stagecoach`, `choco upgrade stagecoach -y`
     (admin), the `CHOCOLATEY_API_KEY` secret, and explicitly note the **non-reason**: NO winget-pkgs
     Defender gate (the v3.3 rationale). Add a short `### PowerShell installer (no package manager)`
     subsection documenting `irm | iex`, the install dir, PATH update, and the
     `STAGECOACH_INSTALL_METHOD=direct` tag. Delete the `wingetcreate` bootstrap and the
     "pending winget-pkgs acceptance" checklist entirely (inapplicable).
   (b) **docs/cli.md** line ~404: `winget` → `chocolatey` in the channel list; note choco is printed
     (admin), consistent with the upgraded §15.3 upgrade-subcommand text.
   (c) **README.md:** line ~6 v3.0 blurb — `Winget` → `Chocolatey` (and add PowerShell installer to
     the parenthetical if space allows); lines ~132–134 — delete the "Not yet available: Windows
     WinGet … pending acceptance" note (Chocolatey + the PowerShell installer have no acceptance
     gate). If a Windows install section exists, ensure it lists `choco install stagecoach` and the
     `irm | iex` PowerShell fallback (mirroring spec §21.3).

3. **OUTPUT.** `rg -ni winget docs/ README.md` returns zero hits. The docs match the code (Phase 1)
   and the spec (§21.2/§21.3).

---

## Phase 2 — Model-example consistency cleanup (zai/glm-5.2 → anthropic/claude-haiku)

### Why (one paragraph)

The v3.3 spec renamed the pi multi-backend **example model** from the fictional `zai/glm-5.2` (and
the personal `zai/glm-5-turbo` override) to the realistic `anthropic/claude-haiku` across every spec
file — to show a real, slash-namespaced model a user would actually configure. The rename is
spec-only (no behavior change), but a few **code comments and user-facing prompt strings** still
carry the old token, leaving the codebase internally inconsistent with the spec it implements. This
phase sweeps those stale references. (The `config_test.go` v2→v3 migration fixtures use `zai/glm-5.2`
as *input* — those test the migration mechanics where the literal value is arbitrary — and are
refreshed here only for coherence, not correctness.)

### What "done" looks like

- `rg -n 'zai/glm|glm-5\.2|glm-5-turbo|zai/glm-5'` across the repo (excluding `spec/` and `plan/`)
  returns zero hits, OR every remaining hit is a deliberate migration-test fixture with a comment
  explaining the value is arbitrary.

### Milestone P2.M1 — Sweep stale zai/glm-5.2 references in code

#### Task P2.M1.T1 — Refresh the example-model token in code comments + interactive prompt + test fixtures

**Story points:** 1 · **Dependencies:** none (independent of Phase 1; can run in parallel)

**prd_selectors:** `§12.3` (pi manifest), `§9.15` (FR-R5b), `§9.17` (FR-B7), spec header "This revision"

**CONTRACT DEFINITION:**

1. **RESEARCH NOTE — verified stale references this session.**
   - `providers/pi.toml` — the personal-override example comments (`zai/glm-5-turbo`, `glm-5.2`,
     `# (e.g. "glm-5.2" → --provider zai --model …)`, line ~53 `The zai/glm-5-turbo setup is a
     personal override`, line ~59 `pi routes to backends: zai | anthropic | google | …`).
   - `internal/cmd/config_init_interactive.go` line ~184 + ~200 — the user-facing prompt string
     `e.g. zai/glm-5.2` (shown to a user during `config init --interactive` for a multi-backend
     provider).
   - `internal/cmd/default_action.go` line ~234 — comment `bare "glm-5.2" on pi — FR-R5b`.
   - `internal/cmd/config_test.go` — the v2→v3 migration test fixtures use `default_provider = "zai"`
     + `default_model = "glm-5.2"` → `model = "zai/glm-5.2"` (lines ~1349–1399). These test the
     prefix-folding mechanics; the literal value is arbitrary. The spec's FR-B7 example was similarly
     generalized (`zai/<model>` → `<inference-provider>/<model>`).

2. **LOGIC — mechanical rename, no behavior change.**
   (a) `providers/pi.toml`: update the personal-override example comments to a realistic shape. The
     spec keeps the *concept* of a personal override but no longer uses the z.ai/GLM token as the
     canonical example — replace with `anthropic/claude-haiku` (matching spec §12.3) and keep a note
     that any `inference/model` works. Update line ~59's backend list to drop `zai` as the lead
     example if it implies a default (it must not — FR-D2: pi ships BLANK).
   (b) `internal/cmd/config_init_interactive.go`: change the prompt example string from
     `e.g. zai/glm-5.2` to `e.g. anthropic/claude-haiku` (both occurrences, lines ~184, ~200).
   (c) `internal/cmd/default_action.go`: change the comment from `bare "glm-5.2" on pi` to
     `bare "claude-haiku" on pi` (matching spec FR-R5b's hard-error example).
   (d) `internal/cmd/config_test.go`: refresh the migration fixtures from `zai`/`glm-5.2` to
     `anthropic`/`claude-haiku` (input `default_provider = "anthropic"` + `default_model =
     "claude-haiku"` → `model = "anthropic/claude-haiku"`). This keeps the fixtures internally
     consistent with the spec example and exercises the identical prefix-folding path. All
     assertions stay structurally identical (just the token changes).

3. **OUTPUT.** `go test ./internal/cmd/...` green; `rg -n 'zai/glm|glm-5\.2' --type-not md` (excluding
   `plan/`) returns zero hits. No behavior change — this is comment/string/fixture hygiene only.

**Mode B (changeset-level docs):** none beyond P1.M3. The model rename is code-internal; `README.md`
does not show a pi model example (verified: README has no `zai/glm` or `claude-haiku` reference), so
there is no changeset-level README edit for Phase 2. The Phase 1 README edits (P1.M3.T1) ARE the
changeset-level docs sweep for this delta.

---

## Already implemented — no tasks (noted for awareness)

These spec deltas between v3.2 and v3.3 are **already shipped in code**; the spec now matches the
code. They are listed so the breakdown/review process knows they were considered and deliberately
excluded from the work scope:

- **FR-D3/FR-D4 "fast by default"** (spec/01-product.md §9.16) — every role ships `fast` except
  stager=`mid`; pi/opencode ship BLANK. Shipped in `5445ff2` + `8d3fc42`; asserted by
  `internal/config/role_defaults_test.go` and `internal/config/bootstrap_test.go`. No task.
- **opencode stager** (`tooled_flags = ["--agent","build"]`, spec/02-providers.md §12.6) — shipped in
  `c9e5fb3`; `internal/provider/builtin.go:353`. No task.
- **cursor stager** (`tooled_flags = ["--trust","--yolo"]` + free-plan note, spec/02-providers.md
  §12.7) — shipped in `f88042c` + `5980a44`; `internal/provider/builtin.go:472`. No task.
  (`docs/providers.md` was synced to these stagers in the prior session's Phase 2.)

If a reviewer finds the code out of sync with these on re-inspection, that is a *bug* (a
spec-vs-code drift), not scope for this delta — open an issue rather than expanding this PRD.

---

## Out of scope

- **The Chocolatey community-repository publication itself** (getting `stagecoach` listed on
  chocolatey.org) — that is a one-time manual/publish action gated on a real `CHOCOLATEY_API_KEY`,
  not code. The goreleaser pipe + secret wiring is in scope; the first successful publish is a
  release-day ops step (documented in `docs/packaging.md` per P1.M3.T1).
- **Re-adding Winget.** The v3.3 decision is final: winget-pkgs's Defender gate is an unbounded
  per-release tax. Do not re-introduce a winget CI job or `ChannelWinget` under any task here.
- **Fast-by-default model refresh / stager re-verification.** Already done; see "Already
  implemented" above.
- **Any commit-path, provider-render, CAS/rescue/lock, or config-schema change.** The v3.3 revision
  is explicitly distribution + upgrade-detection plumbing only (spec v3.3 row: "no commit-path FR
  surface").