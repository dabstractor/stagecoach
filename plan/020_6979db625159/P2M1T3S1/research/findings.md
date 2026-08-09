# Research Findings — P2.M1.T3.S1

**Item:** Final README.md + overview coherence sweep across the entire delta
**(Mode B final task — depends on ALL implementing subtasks of P1 + P2)**

This is primarily a **VERIFICATION + conditional-fix** task, not greenfield implementation.
The implementing subtasks (P1.M3.T1.S2 README winget→chocolatey, P2.M1.T1.S2 README model
examples, P2.M1.T2.S1/S2 test-fixture token refresh) did the actual token replacement. This
task is the FINAL GATE that proves the whole delta is coherent and fixes any straggler the
siblings missed. All line numbers verified 2026-08-08.

---

## 0. The two contract acceptance greps (THE hard gates)

```bash
# (a) Winget must be GONE from README + docs/
rg -ni winget README.md docs/
# STATUS (verified): CLEAN — zero hits. P1.M3.T1.S1 (packaging.md) + P1.M3.T1.S2 (cli.md+README)
# already landed. ✅

# (b) zai/glm-5.2 / glm-5-turbo must be GONE everywhere EXCEPT spec/ and plan/
rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"
# STATUS (verified): ONLY internal/{config,cmd}/*_test.go hits remain — that is P2.M1.T2.S2's
# in-flight scope (it is "Implementing"). NO non-test hits (no code, no docs). ✅ once T2.S2 lands.
```

**CRITICAL dependency on P2.M1.T2.S2:** grep (b) does NOT exclude internal test files, so it reaches
zero ONLY after T2.S2 (config-subsystem test token refresh) ships. This task runs LAST; treat T2.S2 as
a hard prerequisite. T2.S2's contract: every `glm-5.2`/`zai/glm-5.2` test token → `claude-haiku`/
`anthropic/claude-haiku` (+ the bare `default_provider="zai"`→`"anthropic"` M3 the contract grep misses,
gated by `rg 'zai' internal/cmd/config_test.go internal/config/migrate_test.go`=zero).

---

## 1. Current state — what is ALREADY coherent (verified, no edit needed)

### README.md (P1.M3.T1.S2 + P2.M1.T1.S2 — both Complete)
- **Install section (L81-134):** Homebrew (L94), Scoop (L96-98), **Chocolatey** `choco install stagecoach`
  (L100-101), **PowerShell installer** `irm ... install.ps1 | iex` (L103-104), npm (L106-107), Nix (L110),
  mise/asdf (L113), apt (L120), dnf (L124), raw .deb/.rpm (L126-128), Go install (L130-131), curl|sh (L134).
  Matches PRD §21.3 verbatim. NO winget. ✅
- **Features blurb (L6):** "Homebrew, Scoop, Chocolatey, PowerShell installer, npm, Nix, mise/asdf" — no winget. ✅
- **Model examples (L307, L311-314, L316):** `anthropic/claude-haiku` everywhere. NO glm. ✅
- **"Not yet available" notes:** GONE (grep clean). P1.M3.T1.S2 deleted them. ✅

### docs/packaging.md (P1.M3.T1.S1 — Complete)
- **Chocolatey section (L8-33):** `choco install` / `choco upgrade`, the `chocolateys:` goreleaser pipe,
  `CHOCOLATEY_API_KEY` secret, the "Why Chocolatey, not the Windows store channel (v3.3)" rationale.
- **PowerShell installer:** referenced (L4). NO winget. ✅

### docs/cli.md (P1.M3.T1.S2 + P2.M1.T1.S2 — Complete)
- **`upgrade` channels (L404):** "Homebrew, Scoop, Chocolatey, npm, mise, asdf, Nix, AUR, go install" +
  "printing the command where it needs privileges (Chocolatey, AUR)". NO winget. ✅
- **Model examples:** clean (no glm). ✅

### docs/providers.md + docs/configuration.md (P2.M1.T1.S2 — Complete)
- **providers.md:72:** `anthropic/claude-haiku` as the multi-backend example. ✅
- **configuration.md:40, L232:** `anthropic/claude-haiku`, `model = claude-haiku`. NO glm. ✅
- **NOTE (intentional, grep-compliant, DO NOT "fix"):** `docs/providers.md:127` uses `zai/gpt-5.4` as a
  personal-override illustration of the backend/model prefix form. The contract grep (b)
  (`zai/glm|glm-5\.2|glm-5-turbo`) does NOT match `zai/gpt-5.4` — this is the SAME documented
  out-of-scope pattern as `internal/cmd/config_init_interactive_test.go` L105/107/119/120 (P2.M1.T2.S2's
  guard). It is the author's z.ai personal-override illustration per PRD §12.3, deliberately preserved.
  Leaving it is CORRECT.

### go build ./...
- **STATUS (verified): OK** — the whole module compiles with the full delta applied.

---

## 2. Discovered STRAGGLERS — coherence defects the siblings did NOT touch

These are the actual work this sweep task finds. **None fail the contract greps** (a)/(b) — grep (a) is
scoped to README+docs only; grep (b) matches `zai/glm` not `winget` — so they are discovered by the
"verify consistency with the code" / "entire delta coherence" mandate (contract e + the task title), not
by the literal greps. They are all user-facing references to the now-DROPPED Winget channel (PRD §21.2:
"Chosen OVER winget (v3.3)").

### 2a. install.sh (the Unix curl|sh installer) — L39 + L45
```sh
# (1) Platform detect: ... Windows is intentionally unsupported here —
# Windows users use Scoop/Winget (PRD §21.3). Reject anything else with a clear, actionable error.   ← L39
  *)
    err "unsupported OS: $(uname -s). This installer supports macOS + Linux."
    err "Windows users: see https://github.com/${owner}/${name}#install (Scoop or Winget)."            ← L45
    exit 1
```
**Defect:** the comment + the user-facing error both tell Windows users to use "Winget", which PRD §21.2/§21.3
DROPPED. A Windows user who runs `curl|sh` and hits the rejection gets pointed at a non-existent channel.
**Fix:** point them at the three §21.3 Windows channels: Scoop, Chocolatey, and the PowerShell installer.

### 2b. plugins/asdf-stagecoach/README.md — L88-91 (the "Supported platforms" section)
```md
Windows is **not** supported via this plugin (asdf/mise are Unix-only). Windows
users should use [Scoop](https://scoop.sh/) or
[Winget](https://learn.microsoft.com/en-us/windows/package-manager/winget/) —
see the [stagecoach install guide](https://github.com/dabstractor/stagecoach#install).
```
**Defect:** same — lists Winget, which is dropped. **Fix:** Scoop / Chocolatey / PowerShell installer.

### 2c. plugins/asdf-stagecoach/bin/install — L27-28 (the platform-detect comment)
```sh
# (2) uname -> goreleaser GOOS/GOARCH (asdf/mise are Unix-only → windows/zip is out of scope;
# Windows users use Scoop/Winget per PRD §21.3). Reject anything else with a clear error.   ← L28
```
**Defect:** the comment cites "Scoop/Winget per PRD §21.3" — but §21.3 no longer lists Winget, so the
comment is factually stale. **Fix:** Scoop/Chocolatey/PowerShell.

**Why these ARE in scope:** the task title is "coherence sweep across the ENTIRE DELTA"; contract (e)
explicitly says "consistent with each other and **with the code**"; install.sh + the asdf plugin ARE
distribution code/docs; and they reference a channel the delta deliberately dropped. Leaving them means
the "final coherence sweep" missed obvious stale references. **Why they're LOW-RISK:** text-only edits in
shell comments + one user-facing error string + markdown prose. No logic, no behavior, no test impact.

---

## 3. Cross-reference (anchor) health — contract (f)

The delta is text-token replacement (winget→choco, glm→claude-haiku), NOT section renames, so the internal
doc anchors are very likely intact. Verified inventory of internal links (README + docs/*.md): all point at
existing files (cli.md, providers.md, how-it-works.md, configuration.md, README.md) with plausible anchors
(`#upgrade`, `#global-flags`, `#models-provider`, `#the-schema`, `#multi-commit-decomposition`, etc.).
`docs/README.md` is a docs INDEX with a `#contributing` link to `../README.md` (exists). **No winget/glm
anchors exist** (grep (a)/(b) clean ⇒ no such text ⇒ no such anchors). The sweep's job here is a
spot-check pass, not a bulk fix — see PRP Task 4 for the verification recipe. If the spot-check finds a
broken anchor, fix it; if not (expected), nothing to do.

---

## 4. The go test ./... gate — contract (g)

The full delta must leave the whole test suite green. **Baseline (verified):** `go build ./...` is OK now.
The config tests that still carry `glm-5.2` tokens are INTERNALLY consistent (input + assertion move
together), so `go test ./...` passes NOW and stays green after T2.S2's consistent rename. This gate is the
proof that P1 (detect/delegate/goreleaser/install.ps1) + P2 (token refresh across code+tests) compose
without a breakage. Run it AFTER confirming T2.S2 landed (the prerequisite).

---

## 5. Sibling contracts (what each owned — do NOT redo their work)

| Sibling | Status | Owned (DISJOINT from this sweep) |
|---|---|---|
| P1.M3.T1.S1 | Complete | docs/packaging.md WinGet→Chocolatey section |
| P1.M3.T1.S2 | Complete | docs/cli.md + README.md winget→chocolatey + delete "Not yet available" |
| P2.M1.T1.S2 | Complete | runtime code + user-facing docs (README, cli.md, configuration.md, providers.md) glm→claude-haiku |
| P2.M1.T2.S1 | Complete | provider-subsystem test fixtures (9 files) glm→claude-haiku |
| P2.M1.T2.S2 | Implementing (PREREQ) | config-subsystem test fixtures (8 files) glm→claude-haiku |

This sweep owns: the two grep gates + README/docs coherence verification + the install.sh/plugins stragglers
+ cross-reference spot-check + go test. It does NOT redo any sibling's token-replacement work.

---

## 6. Scope fence — what MUST NOT be touched

- `spec/**` (human-owned PRD source — AGENTS.md rule 1), `PRD.md`, `plan/`, `tasks.json`,
  `prd_snapshot.md`, `delta_prd.md` (orchestrator-owned).
- `docs/providers.md:127` `zai/gpt-5.4` (intentional personal-override illustration — grep-compliant).
- The runtime code + docs the siblings already landed (unless the sweep finds an ACTUAL defect — none found).
- `.goreleaser.yaml`, `release.yml`, `install.ps1` (P1.M2 — already correct; grep confirms no winget).

## 7. Validation commands (verified)

```bash
rg -ni winget README.md docs/                                                     # gate (a) → zero
rg -n "zai/glm|glm-5\.2|glm-5-turbo" --glob "!spec/**" --glob "!plan/**"          # gate (b) → zero (after T2.S2)
go test ./...                                                                     # gate (g) → green
git status --porcelain                                                            # scope: only intended files
```