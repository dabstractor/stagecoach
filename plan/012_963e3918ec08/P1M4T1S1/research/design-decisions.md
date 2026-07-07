# P1.M4.T1.S1 — Design Decisions & Research Notes

> Research backing `PRP.md`: rename README.md stagehand→stagecoach (project rename Layer 5.1, Mode A). A
> global `sed` over the case variants does the work; the PRP's value is the namespace-resolution trap and
> the post-sed verification gates.

## 0. Scope: README.md ONLY. Global sed. Mode A.

Edit ONLY the top-level `README.md` (the primary user-facing surface — title, hero, badges, install,
examples, FAQ). `docs/*.md` is P1.M4.T1.S2's scope; `providers/*.toml` is P1.M4.T2.S1; `FUTURE_SPEC.md` is
P1.M4.T2.S2; plan/ artifacts are P1.M5.T1.S1. This task is ONE file. The mechanism is a global
case-variant `sed` (stagehand/Stagehand/STAGEHAND → stagecoach/Stagecoach/STAGECOACH); 82 occurrences, no
exceptions (the README is a clean marketing surface — there is no "formerly stagehand" note to preserve;
the rename-history note lives in the PRD h2.30, not the README).

## 1. THE namespace decision: `dustin/stagecoach` — NOT `dabstractor` (the contract item (a) trap)

The contract's item (a) says the badge URL must be `github.com/dabstractor/stagecoach/...`. This is
**WRONG**. The canonical public namespace is **`dustin/stagecoach`**, confirmed by FOUR independent sources:
- **go.mod**: `module github.com/dustin/stagecoach` (M1.T1.S1, LANDED — the import-path source of truth).
- **.goreleaser.yaml**: `owner: dustin` (lines 68/77/80/89/92/95) — explicitly OVERRIDES the
  `dabstractor` git remote. Lines 5-8 document the decision: "uses `dustin/stagecoach` … per PRD §21.2/§21.3
  and the go.mod module path … the current git remote is `dabstractor/stagecoach`; before the first REAL tag
  the repo must be reachable at github.com/dustin/stagecoach."
- **rename_surface_map.md §5.1**: "Badge URL: `github.com/dustin/stagehand/actions/...` → `stagecoach`"
  (owner stays `dustin`).
- **PRD §21.3** (h3.100): `github.com/dustin/stagecoach`, `brew install dustin/tap/stagecoach`,
  `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`, `scoop install dustin/stagecoach`.

The README already uses `dustin/stagehand` throughout (badge L10, clone L95, install L113-116) and contains
**ZERO** `dabstractor` (grep confirmed). So the sed (stagehand→stagecoach) ALREADY produces the correct
`dustin/stagecoach` namespace. **Do NOT change `dustin` to `dabstractor`.** The contract item (a) is the
one trap in this task — following it would point badges/install at a namespace the .goreleaser explicitly
rejects.

## 2. The sed is complete + safe (82 occurrences, no exceptions)

`sed -i 's/stagehand/stagecoach/g; s/Stagehand/Stagecoach/g; s/STAGEHAND/STAGECOACH/g' README.md`. The
three case variants are disjoint (no double-substitution). Verified coverage:
- `# Stagehand` (title) → `# Stagecoach`; hero pitch; all prose ("Stagehand does one thing…").
- CLI invocations in code blocks: `stagehand`, `stagehand --version`, `stagehand -a`, `stagehand --dry-run`,
  `stagehand --push`, `stagehand --edit`, `stagehand --format`, `stagehand --exclude`, `stagehand
  --reasoning`, `stagehand --commits`, `stagehand --single`, `stagehand integrate …`, `stagehand hook
  install`, `stagehand models`, `stagehand providers list`.
- The `git stagehand` alias (L72, L185) → `git stagecoach` (the alias name matches the new binary).
- Config surfaces: `.stagehandignore` (L66) → `.stagecoachignore`; `.stagehand.toml` (L168) →
  `.stagecoach.toml`; `git config stagehand.provider` (L232) → `git config stagecoach.provider`.
- The **integration idempotency marker** `# stagehand-integration` (L199) → `# stagecoach-integration` and
  `description: 'stagehand: AI commit'` (L204) → `stagecoach: AI commit` — these MUST match the renamed Go
  constant (see §4).
- Install paths: `brew install dustin/tap/stagehand`, `scoop install dustin/stagehand`,
  `go install github.com/dustin/stagehand/cmd/stagehand@latest`, the curl|sh URL — all → stagecoach.

No false positives: "stagehand" never appears as a substring of another word, and there is no historical
"originally stagehand" sentence to preserve. After the sed, `grep -ri stagehand README.md` must return ZERO
hits (the Layer-5 verification gate).

## 3. The "manual review" items are VERIFICATION, not extra edits (the sed already handles them)

The contract's review items (a)-(e) become post-sed CHECKS (the sed produced them correctly because the
README already used `dustin/stagehand`):
- **(a) Badge URL** → `https://github.com/dustin/stagecoach/actions/workflows/ci.yml/badge.svg` (NOT
  dabstractor; §1). The workflow FILENAME stays `ci.yml` (P1.M3.T1.S3 renames its content, not its name).
- **(b) brew** → `brew install dustin/tap/stagecoach` (matches .goreleaser `dustin/homebrew-tap`).
- **(c) go install** → `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` — VERIFY the module
  path matches go.mod (`github.com/dustin/stagecoach`) AND the cmd dir (`cmd/stagecoach`, renamed M1.T1.S2).
- **(d)** `.stagecoach.toml`; **(e)** `STAGECOACH_*` env vars.
- **Build-from-source** (L95-107): clone `https://github.com/dustin/stagecoach.git`, `cd stagecoach`,
  `stagehand --version` → `stagecoach --version`, the symlink line `ln -s "$(go env GOPATH)/bin/stagehand"
  ~/.local/bin/stagehand` → `…/bin/stagecoach ~/.local/bin/stagecoach`.

## 4. The integration marker + git alias must MATCH the renamed Go constant (M1.T2, LANDED)

The README's lazygit example shows the idempotency marker `# stagecoach-integration` (post-sed). That string
MUST match the Go constant, which M1.T2.S1 already renamed: `internal/cmd/integrate_lazygit.go:20
lazygitMarker = "stagecoach-integration"` + `:28 var entryTpl = \'- key: \'%s\' # stagecoach-integration\'`.
The sed produces exactly this. This is a cross-surface consistency point: the README example must match the
emitted marker or `stagehand integrate uninstall` (→ `stagecoach integrate uninstall`) couldn't find its own
entry. The git alias `git stagehand` → `git stagecoach` similarly matches the renamed `alias.stagecoach`
registration in the Go code.

## 5. Anchor links: NONE contain "stagehand" → no cross-file breakage

The README links to docs anchors: `#multi-turn-generation-fallback`, `#trade-off-inversion-fr-h7`,
`#commit-hooks-on-the-plumbing-path`, `#integrate-install-target`, `#models-provider`,
`#exclusion-globs-generationexclude`, `#built-in-defaults`, `#multi-commit-decomposition`. None contain
"stagehand", so the sed leaves them and P1.M4.T1.S2's docs sed won't break them (the docs headings those
anchors derive from don't contain "stagehand" either — they're things like "## Multi-turn generation
fallback"). The link TEXT "How Stagehand works" → "How Stagecoach works" is cosmetic. VERIFY post-sed that
no README anchor (`docs/…#…`) contains "stagehand" (none do).

## 6. No conflict with the parallel P1.M3.T1.S3 (CI/.gitignore)

The running P1.M3.T1.S3 edits `.github/workflows/ci.yml` + `.gitignore` (Layer 4.3-4.4). It does NOT touch
README.md. No overlap, no merge conflict. It also confirms the `dustin/stagecoach` namespace + the global-sed
approach (its #1 note uses the same two-branch sed). The badge URL in README references
`actions/workflows/ci.yml/badge.svg` — that workflow file is the one S3 renames the CONTENT of (filename
stays ci.yml), so the badge keeps resolving. ✓

## Sources
- `README.md` (read in full via grep — 82 stagehand/Stagehand/STAGEHAND hits; the key lines: title L1, hero
  L3, badge L10, features L59-74, install L88-116, examples L117-210, FAQ L211+).
- `go.mod` — `module github.com/dustin/stagecoach` (the import-path source of truth).
- `.goreleaser.yaml` L5-8, 68-95 — `owner: dustin` (overrides dabstractor; dustin/homebrew-tap,
  dustin/scoop-bucket); the namespace decision note.
- `plan/012…/architecture/rename_surface_map.md` §5.1 — README badge `dustin/stagehand` → `dustin/stagecoach`;
  the Layer-5 "grep -ri stagehand returns ZERO hits" gate.
- `internal/cmd/integrate_lazygit.go:20,28` — `lazygitMarker = "stagecoach-integration"` (M1.T2, LANDED) —
  the marker the README example must match.
- PRD §21.3 (h3.100) — the install paths (`dustin/stagecoach`, `dustin/tap/stagecoach`).
- `plan/012…/P1M3T1S3/PRP.md` — confirms CI/.gitignore scope (no README overlap) + the dustin namespace.
