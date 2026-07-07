# Research Findings — P1.M5.T3.S2 (Verify badge URLs, GitHub links, distribution paths)

> Verification task (Mode B). All evidence gathered LIVE in the repo + web, 2026-07-07.
> The repo root is `/home/dustin/projects/stagehand` (cwd unchanged; the *product* is `stagecoach`).

## 0. The headline

The distribution surface is **already internally consistent and correct**. This task is a
VERIFICATION that gathers deterministic evidence + resolves the one open product question (the
namespace) + flags the release-time prerequisites. No blocking fixes were found. The one subtle
catch (homebrew `homebrew_casks` vs the deprecated `brews`/formulas) is **CORRECT as-is** — a
naive "fix" would reintroduce a deprecated pipe.

## 1. Ground-truth namespace sweep (the contract's central question)

```
git remote -v | grep origin      →  git@github.com:dabstractor/stagecoach   (the REMOTE)
head -1 go.mod                   →  module github.com/dustin/stagecoach     (the MODULE PATH = go-install identity)
```

| Surface | GitHub path | Verdict |
|---|---|---|
| go.mod:1 | `github.com/dustin/stagecoach` | canonical (oracle) |
| README badge (L10) | `github.com/dustin/stagecoach/actions/...` | dustin ✓ |
| README clone (L95) | `github.com/dustin/stagecoach.git` | dustin ✓ |
| README install brew (L113) | `dustin/tap/stagecoach` | dustin ✓ |
| README install scoop (L114) | `dustin/stagecoach` | dustin ✓ |
| README go install (L115) | `github.com/dustin/stagecoach/cmd/stagecoach@latest` | dustin ✓ |
| README install.sh (L116) | `github.com/dustin/stagecoach/raw/main/install.sh` | dustin ✓ |
| docs/README.md (L14,17,20,23) | same 4 install paths | dustin ✓ |
| .goreleaser.yaml owner (L60-62) | `release.github.owner: dustin` | dustin ✓ |
| .goreleaser.yaml homepages (×3) + url_template | `github.com/dustin/stagecoach` | dustin ✓ |
| .goreleaser.yaml brew/scoop/aur repos | `dustin/homebrew-tap`, `dustin/scoop-bucket` | dustin ✓ |

- `grep -rhoE 'github\.com/dustin/stagecoach' README.md docs/ .goreleaser.yaml go.mod` → **13** matches.
- `grep -rn 'dabstractor' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml` → ONLY
  `.goreleaser.yaml:7` and `:68` — both are **COMMENTS** explaining the remote mismatch (not URLs/values).
  The actual `owner: dustin` (L68) is correct. → no leak.
- `grep -rni 'stagehand' README.md docs/ .goreleaser.yaml go.mod Makefile .golangci.yml` → **ZERO**. Clean.

**VERDICT: no split namespace, no stragglers. Every URL/config value is `dustin/stagecoach`.**

## 2. Namespace RESOLUTION (resolves the contract check (d) tension)

The contract check (d) gives the example: "`dustin/stagecoach` [is wrong] when remote is
`dabstractor/stagecoach`" → "fix it." Taken literally, that means change docs → `dabstractor`.
That is the WRONG fix. Mechanical proof:

- `go install github.com/<module-path>/cmd/stagecoach@latest` resolves the module at its DECLARED
  path (go.mod:1 = `github.com/dustin/stagecoach`). If docs say `dabstractor` but go.mod says
  `dustin`, the documented `go install` 404s. → docs MUST stay `dustin` to match go.mod.
- PRD §21.2/§21.3 MANDATE `dustin/*` (brew `dustin/tap`, scoop `dustin/stagecoach`, go install
  `github.com/dustin/stagecoach/...`). PRD is read-only, human-owned. The product's canonical org is `dustin`.
- S1 (parallel sibling) established `dustin` canonical via 3 mandates. S2 reversing it = contradiction.
- Changing go.mod to `github.com/dabstractor/stagecoach` is a structural module rename (M1 territory),
  out of a doc-verification task's scope.

**RESOLUTION: canonical org = `dustin/`. The git remote (`dabstractor/stagecoach`) is the thing that's
"wrong" — it must be reconciled to `github.com/dustin/stagecoach` before the first REAL tag (a GitHub
rename/transfer/mirror). That is a PRE-RELEASE GATE, not a doc edit.** This satisfies check (d)'s INTENT
(catch URLs that don't match the actual repo) by: confirming no SPLIT namespace, verifying external
resolution, and FLAGGING the remote reconciliation. Do NOT change the docs to `dabstractor`.

This is consistent with S1's identical finding (S1 PRP §"Namespace decision tree").

## 3. homebrew_casks is CORRECT — do NOT "fix" it to brews/formulas

The `.goreleaser.yaml` uses `homebrew_casks:` with a comment "replaces deprecated brews/formulas in
goreleaser v2.10+." VERIFIED ACCURATE via web search:

- https://goreleaser.com/customization/publish/homebrew_formulas/ — **"Homebrew Formulas (deprecated).
  Deprecated in v2.10. Homebrew Casks should be used instead."**
- https://goreleaser.com/blog/goreleaser-v2.10/ — "This version introduces the new Homebrew Casks feature."
- https://github.com/goreleaser/goreleaser/discussions/5563 — "Brew packages should be casks, not formulae"
  (the discussion that motivated the change: precompiled binaries distributed as formulae confused users).

So `homebrew_casks:` is the CURRENT (v2.10+) recommended pipe for a precompiled-binary CLI. A naive
"fix" reverting to `brews:`/formulas would reintroduce a DEPRECATED pipe (and `goreleaser check` on a
recent binary may warn/error). The config is correct.

Consistency with README: goreleaser publishes cask `stagecoach` (name derived from project_name) to
repo `dustin/homebrew-tap` (= brew tap `dustin/tap`). README says `brew install dustin/tap/stagecoach`.
Modern Homebrew (4.0+) resolves `brew install owner/tap/name` to a cask OR formula by name in the tap,
so the command works with a cask. → CONSISTENT. (Release-time confirmation: the cask is actually
published to the tap; `goreleaser check` + a real publish validates the cask name = `stagecoach`.)

## 4. Scoop bucket naming — consistent

- goreleaser `scoops:` → manifest `stagecoach.json` (name: stagecoach) pushed to repo `dustin/scoop-bucket`.
- README/docs: `scoop install dustin/stagecoach`.

Scoop convention (https://github.com/ScoopInstaller/Scoop/wiki/Buckets): a bucket is a git repo of JSON
manifests; the full flow is `scoop bucket add <name> https://github.com/<owner>/<bucket>` then
`scoop install <name>/<app>`. The README one-liner `scoop install dustin/stagecoach` is the standard
shorthand (bucket=`dustin`, app=`stagecoach`) used by most Go CLIs and matches PRD §21.3 verbatim.
Owner (`dustin`) + manifest name (`stagecoach`) AGREE with goreleaser. → CONSISTENT. (Minor: the README
omits the `scoop bucket add dustin https://github.com/dustin/scoop-bucket` step — a docs-UX
simplification, not a goreleaser-vs-README inconsistency; matches the PRD verbatim, so leave as-is.)

## 5. install.sh — referenced, ABSENT (pre-release publication item)

`install.sh` does NOT exist (`ls install.sh` → no such file). README (L116) + docs/README.md (L20)
reference `github.com/dustin/stagecoach/raw/main/install.sh`. docs/README.md:27 explicitly documents:
"The `install.sh` script is published with the first release." → NOT a broken cross-ref; a documented
pre-release gap. The URL itself is consistent (`dustin/stagecoach/raw/main`). Flag for release-time
publication; do NOT create the file in this task (it is a release artifact, out of doc-verification scope).

## 6. AUR — consistent with PRD's "possibly community"

goreleaser `aurs:` → `stagecoach-bin` (prebuilt binary PKGBUILD). PRD §21.2: "AUR `stagecoach` +
`stagecoach-bin` (via a maintained PKGBUILD; possibly community)." The from-source `stagecoach` AUR
package is OUT of goreleaser's scope (manual/community PKGBUILD) — matches PRD wording. The `-bin`
package name + homepage (`dustin/stagecoach`) are consistent. → CONSISTENT (best-effort; release-time
gated on the AUR_SSH_PRIVATE_KEY + AUR account).

## 7. Smoke test (contract check e) — PASS

```
make build                         →  ./bin/stagecoach   (go build -ldflags "-X main.version=dev" -o bin/stagecoach ./cmd/stagecoach)
./bin/stagecoach providers list    →  8 providers, all "stagecoach" branding:
   agy ✓, claude ✓, codex ✓, cursor ✓, gemini ✓, opencode ✓, pi ✓ (default), qwen-code ✗
./bin/stagecoach --version         →  "stagecoach version dev (6dabcdb)"
```

Branding is uniformly `stagecoach` (the binary name, the version prefix, the command surface). The
rename is complete at the binary/CLI surface. (qwen-code not DETECTED is an environment fact — the
`qwen-code` CLI isn't installed on this machine — not a rename defect; the provider is listed.)

## 8. S1/S2 scope split (avoid duplication)

- **S1 (P1.M5.T3.S1)** = INTERNAL consistency of the unified identity: badge=go install=go.mod=
  goreleaser, zero stagehand, lazygit/git-alias/EDITMSG examples (checks a–g). VERDICT: also green.
- **S2 (this)** = DISTRIBUTION-PATH/CHANNEL correctness + EXTERNAL resolution + smoke test:
  (a) all URLs → same repo; (b) Homebrew tap / Scoop bucket / AUR package names consistent across
  goreleaser+README; (c) install.sh URL; (d) namespace resolution (does it resolve?); (e) build+providers.
  Distinct from S1: the channel-name consistency, the homebrew_casks correctness, the install.sh check,
  the external-resolution flag, and the binary smoke test.

Both inherit the SAME namespace decision (`dustin/`). Neither changes the docs to `dabstractor/`.

## 9. URLs cited (web-verified 2026-07-07)

- https://goreleaser.com/customization/publish/homebrew_formulas/ — "Deprecated in v2.10" (CONFIRMS homebrew_casks is correct).
- https://goreleaser.com/blog/goreleaser-v2.10/ — "introduces the new Homebrew Casks feature".
- https://github.com/goreleaser/goreleaser/discussions/5563 — "Brew packages should be casks, not formulae".
- https://goreleaser.com/customization/publish/homebrew_casks/ — the current casks pipe doc.
- https://github.com/ScoopInstaller/Scoop/wiki/Buckets — scoop bucket convention (bucket = git repo of manifests).
