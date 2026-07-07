# P1.M5.T3.S1 — Research Findings (Verify documentation internal consistency & unified identity)

Decisive findings only. The PRP (../PRP.md) consumes these. All evidence gathered LIVE in the repo
(`/home/dustin/projects/stagehand`, the renamed project; git remote = `dabstractor/stagecoach`,
go.mod = `github.com/dustin/stagecoach`) on 2026-07-07.

## 0. HEADLINE: the project is ALREADY internally consistent (like S2 found the build green)

The rename's M3 (build/CI) + M4 (docs) passes left the documentation surface fully `stagecoach` and
fully self-consistent. Every one of the contract's 7 checks (a–g) PASSES on internal consistency today.
This task is therefore primarily a VERIFICATION that gathers grep EVIDENCE + resolves the ONE open
product question (the namespace, §1) + flags pre-release action items. It is NOT a bulk-edit task.
(Re-verify at implementation time — do not assume research holds; P1.M5.T2.S2 runs in parallel but
touches build/test, not README/docs/goreleaser.)

## 1. THE NAMESPACE DECISION (the contract's central open question) — RESOLVED & DOCUMENTED

The contract's RESEARCH NOTE flags a conflict: git remote = `dabstractor/stagecoach` vs go.mod =
`github.com/dustin/stagecoach`. "These may conflict — verify the canonical GitHub path."

**RESOLUTION (firmly established by the P1.M3.T1.S2 goreleaser PRP, with THREE independent mandates):**
the canonical org is **`dustin/`** (NOT `dabstractor/`). F9 (research) had SUGGESTED `dabstractor/` to
match the remote, but the implementation OVERRODE that, preserving `dustin/`. The three mandates:
  (a) `go.mod` module path = `github.com/dustin/stagecoach` — the path `go install` uses.
  (b) PRD §21.2/§21.3 install commands = `brew install dustin/tap/stagecoach`,
      `scoop install dustin/stagecoach`, `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`.
  (c) `.goreleaser.yaml` `release.github.owner: dustin` — explicitly "WINS over git-remote auto-detect
      (which is `dabstractor`)" (goreleaser.yaml:60-62, 5-8 owner note).

**CURRENT STATE — internally consistent on `dustin/stagecoach`** (verified):
  - README.md badge (L10): `https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg`
  - README.md clone (L95): `github.com/dustin/stagecoach.git`
  - README.md install (L113-116): brew `dustin/tap`, scoop `dustin/stagecoach`, go install `dustin/stagecoach`, curl `dustin/stagecoach`
  - docs/README.md install (L14,17,20,23): ALL `dustin/stagecoach`
  - .goreleaser.yaml: owner `dustin`, homepages ×4 `github.com/dustin/stagecoach`, url_template `dustin/stagecoach/releases/...`
  - go.mod: `github.com/dustin/stagecoach`
  → badge = go install = go.mod = goreleaser = docs. **100% internally consistent.**

**THE ONE EXTERNAL FACT (not an internal-doc bug) = the git remote ≠ canonical.** `git remote -v` →
`dabstractor/stagecoach`. This is a PRE-RELEASE ACTION ITEM (already documented in goreleaser.yaml:5-8):
before the first REAL tag, the repo must be reachable at `github.com/dustin/stagecoach` (rename/transfer
the GitHub repo, or set up `dustin/stagecoach` as a mirror/redirect), OR the namespace is reconciled
repo-wide to `dabstractor/` (which would mean changing go.mod + every doc + goreleaser — the heavier
path, and it contradicts the 3 mandates). **Do NOT change the docs/goreleaser to `dabstractor/` in this
task** — that would break go.mod/install/PRD consistency. Flag the remote reconciliation as a release gate.

## 2. THE 7 CONTRACT CHECKS — current-state matrix (all PASS on internal consistency)

| # | check | evidence (live) | verdict |
|---|-------|-----------------|---------|
| (a) | README badge URL = actual repo URL | badge `dustin/stagecoach` (README:10); remote `dabstractor/stagecoach`. Canonical = `dustin` (§1). | PASS (internal) + FLAG remote (pre-release) |
| (b) | `go install` in README = go.mod module | both `github.com/dustin/stagecoach` (README:115; go.mod:1) | PASS |
| (c) | goreleaser URLs = README badge | both `dustin/stagecoach` (goreleaser:60-62,80,92,95,106; README:10) | PASS |
| (d) | docs/*.md cross-refs consistent, no old names | `git grep -il stagehand -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod` → clean (0 files). All referenced docs files exist (cli/configuration/how-it-works/providers/README + FUTURE_SPEC.md). `install.sh` intentionally absent pre-release (docs/README.md:27 says so). | PASS |
| (e) | lazygit: `command: 'stagecoach'` + marker `# stagecoach-integration` | README:199 `# stagecoach-integration`, README:201 `command: 'stagecoach'`; docs/cli.md:299/301 same | PASS |
| (f) | git alias = `git stagecoach` | README:72,185 `git stagecoach`; docs/cli.md:262 `alias.stagecoach '!stagecoach'` | PASS |
| (g) | `.git/stagecoach_EDITMSG` (not `stagehand_EDITMSG`) | Go code uses `STAGECOACH_EDITMSG` (finalize.go:78, git.go:390); ZERO `stagehand_EDITMSG` anywhere; no user-facing doc names the file | PASS (see §3 casing note) |

## 3. EDITMSG CASING — a verified NON-ISSUE (do NOT "fix" it)

- **Go runtime** writes `<gitDir>/STAGECOACH_EDITMSG` (UPPERCASE) — internal/generate/finalize.go:78,
  internal/git/git.go:390. This is the SOURCE OF TRUTH (what the binary writes).
- **PRD.md:485** (FR-E1) writes `.git/stagecoach_EDITMSG` (lowercase). **The contract check (g) also
  writes lowercase.** This is loose spec prose, NOT a code/doc bug.
- **Why uppercase is correct:** git itself uses UPPERCASE for its message files (`COMMIT_EDITMSG`,
  `MERGE_MSG`, `TAG_EDITMSG`). The implementation follows git's convention. Lowercasing the code would
  be non-idiomatic and would diverge from git.
- **No user-facing doc names the file** (docs/cli.md:42 describes `--edit` generically as "the EDITMSG
  file"; docs/cli.md:128 references git's own `.git/COMMIT_EDITMSG` for the hook context — a DIFFERENT,
  correctly-named file). So there is no doc that could "lie" about the name.
- **Verdict:** check (g) is about old-name leakage (`stagehand_EDITMSG` → none) + correct reference
  (`stagecoach` present). Both hold. **Do NOT change the code casing, and do NOT edit PRD.md** (it is
  read-only and the lowercase is out of scope). Document this as a verified non-issue.

## 4. SCOPE OVERLAP with sibling P1.M5.T3.S2 — coordinate, don't duplicate

- **S2 (Planned)**: "Verify badge URLs, GitHub links, and distribution paths are correct." This is a
  NARROWER, EXTERNAL check (do the badge URL resolve? does the `dustin/tap` brew tap exist? is
  `install.sh` published? do GitHub anchor links resolve?).
- **S1 (THIS)**: "Verify documentation INTERNAL consistency and unified identity" — checks (a)–(g),
  the cross-cutting coherence (docs agree with each other + go.mod + goreleaser; lazygit/alias/EDITMSG
  examples; no old names). S1 verifies (a)/(b)/(c) at the CONSISTENCY level (badge = go install = go.mod
  = goreleaser?); S2 verifies them at the EXTERNAL level (URLs actually resolve / taps exist).
- **Clean split for this PRP:** S1 owns internal-coherence evidence (a–g) + the namespace DECISION
  documentation (§1) + the pre-release remote flag. S1 DEFERS the external URL-resolution + tap/bucket/
  install.sh existence checks to S2 (note this in the PRP scope boundary so the two don't collide).

## 5. Sibling / parallel context (do NOT conflict)

- **P1.M5.T2.S2** (in-flight): build/test verification. Touches Go build/test, NOT README/docs/goreleaser.
  No file overlap with S1. Its green result confirms the binary identity (`--version`/`--help`) which S1
  can cross-reference but does not re-verify.
- **P1.M5.T1.S1** (Ready): bulk-rename `stagehand` in `plan/` historical files. `plan/` is NOT a
  user-facing surface and is S1's documented exception (not compiled/shipped). S1 does NOT verify plan/
  (that's T1.S1's own scope). No overlap.
- **P1.M5.T2.S1** (Complete): the grep audit. Already fixed the 3 production stragglers (git.go comment,
  .goreleaser.yaml comment, .golangci.yml path). S1's `git grep` over the user-facing surface confirms
  those fixes hold (clean).

## 6. Re-runnable verification commands (the implementer runs these FRESH; grep-based, deterministic)

```bash
cd <repo-root>   # /home/dustin/projects/stagehand

# (a) namespace: badge vs remote vs go.mod (the decision is dustin=canonical; remote is pre-release)
git remote -v | grep origin                            # expect: dabstractor/stagecoach (the remote)
grep -n 'github.com/dustin/stagecoach' README.md | head -1   # expect: badge URL (the canonical)

# (b) go install == go.mod
grep -o 'github.com/dustin/stagecoach' go.mod                                   # expect: 1 match
grep -c 'go install github.com/dustin/stagecoach/cmd/stagecoach' README.md docs/README.md  # expect: 2

# (c) goreleaser URLs == badge (all dustin/stagecoach)
grep -c 'github.com/dustin/stagecoach' .goreleaser.yaml                          # expect: >=4

# (d) ZERO stagehand in the user-facing+config surface; all docs cross-ref targets exist
git grep -il 'stagehand' -- README.md docs/ .goreleaser.yaml .golangci.yml Makefile go.mod   # expect: no output
for f in docs/cli.md docs/configuration.md docs/how-it-works.md docs/providers.md docs/README.md FUTURE_SPEC.md; do test -e "$f" && echo "OK $f" || echo "MISS $f"; done

# (e) lazygit integration example: command 'stagecoach' + marker
grep -n "command: 'stagecoach'" README.md docs/cli.md          # expect: 2 matches
grep -n '# stagecoach-integration' README.md docs/cli.md       # expect: >=2 matches

# (f) git alias = 'git stagecoach'
grep -rn "git stagecoach" README.md docs/cli.md                # expect: matches
grep -n "alias.stagecoach" docs/cli.md                         # expect: '!stagecoach'

# (g) EDITMSG: STAGECOACH_EDITMSG in code; ZERO stagehand_EDITMSG repo-wide (excl plan/ + artifacts)
grep -rn 'STAGECOACH_EDITMSG' internal/                         # expect: finalize.go:78, git.go:390
git grep -il 'stagehand_EDITMSG' -- ':!plan/' ':!.pi-subagents/'   # expect: no output
```

## 7. Pre-release action items (flagged by this verification, NOT fixed here — out of scope to act)

1. **Remote reconciliation:** make the repo reachable at `github.com/dustin/stagecoach` before the
   first real tag (rename/transfer/mirror), OR reconcile the namespace repo-wide to `dabstractor/`
   (heavier; contradicts go.mod/PRD/goreleaser). Until then the README badge points at a not-yet-live
   URL. (This is the documented pre-release gate from goreleaser.yaml:5-8.)
2. **`install.sh`:** does not exist yet (README/docs reference `…/raw/main/install.sh`). Published with
   the first release (docs/README.md:27). S2 likely owns verifying its presence at release time.
3. **Distribution taps/buckets:** `dustin/homebrew-tap` and `dustin/scoop-bucket` repos must exist
   before goreleaser can publish (goreleaser.yaml:78,90). S2's external-path verification territory.
