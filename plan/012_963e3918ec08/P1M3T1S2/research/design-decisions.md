# P1.M3.T1.S2 — Design Decisions (.goreleaser.yaml rename: stagehand → stagecoach)

Ground truth read before writing this note:
- **PRD §21.2 (h3.99)**: goreleaser produces archives + Homebrew formula to `dustin/homebrew-tap`, AUR
  `stagecoach`, Scoop, checksums, changelog; `go install github.com/dustin/stagecoach/cmd/stagecoach@latest`.
- **PRD §21.3 (h3.100)**: `brew install dustin/tap/stagecoach`, `go install github.com/dustin/stagecoach/...`,
  `scoop install dustin/stagecoach` — ALL use `dustin/`.
- **PRD h2.30**: "this project was originally named 'stagehand' and has been renamed. All references to
  'stagehand' must be replaced with 'stagecoach'."
- **The ACTUAL `.goreleaser.yaml`** (read in FULL from `/home/dustin/projects/stagehand/.goreleaser.yaml`):
  23 `stagehand` references, ALL the product name (no partial-word collisions — no `stagehandler` etc.).
  Includes an explicit "Owner note" (L5-9) + `release.github.owner: dustin` (L68, "explicit; WINS over
  git-remote auto-detect which is `dabstractor`").
- **go.mod**: `module github.com/dustin/stagecoach` (ALREADY renamed M1.T1.S1; uses `dustin`).
- **git remote**: `git@github.com:dabstractor/stagecoach` (the CURRENT remote; org = `dabstractor`).
- **cmd/stagecoach/** EXISTS (M1.T1.S2 renamed cmd/stagehand → cmd/stagecoach) — so `main: ./cmd/stagecoach`
  resolves after the sed.
- **goreleaser IS installed** (`/home/dustin/.local/bin/goreleaser`) — `goreleaser check` is a valid gate.
- The S1 CONTRACT (P1.M3.T1S1/PRP.md): the Makefile rename (global sed, producing `./bin/stagecoach`).
  S2 is the .goreleaser.yaml sibling.
- NOTE: `rename_surface_map.md` and `critical_findings.md` (cited by the contract) do NOT EXIST in the plan
  tree — the authoritative source is the ACTUAL `.goreleaser.yaml` (read in full).

---

## §1 — THE namespace decision: KEEP `dustin/`, do NOT change to `dabstractor/`

**The contract's CAUTION** (point 1): "the actual GitHub remote is `dabstractor/stagecoach`... URLs should
become `github.com/dabstractor/stagecoach` to match the real remote, OR verify with the user."

**Decision: KEEP `dustin/`.** The `sed 's/stagehand/stagecoach/g'` changes ONLY the product name; it does
NOT touch the org. The resulting URLs are `github.com/dustin/stagecoach`, `dustin/homebrew-tap`,
`dustin/scoop-bucket` — ALL matching go.mod + the PRD. Do NOT add a `dustin→dabstractor` step.

**Evidence (overwhelming — 3 independent sources agree on `dustin`):**
1. **go.mod** (the authoritative Go module path, ALREADY renamed): `module github.com/dustin/stagecoach`.
   `go install github.com/dustin/stagecoach/cmd/stagecoach@latest` is the PRD's install path. Changing
   goreleaser to `dabstractor/` would make the release URLs INCONSISTENT with the module path.
2. **PRD §21.2/§21.3** (the authoritative spec): `brew install dustin/tap/stagecoach`, `scoop install
   dustin/stagecoach`, `go install github.com/dustin/stagecoach/...` — ALL `dustin/`.
3. **The EXISTING .goreleaser.yaml** (a DELIBERATE prior design decision): the owner note (L5-9) explicitly
   states "uses `dustin/stagehand` per PRD §21.2/§21.3 and the go.mod module path... before the first REAL
   tag the repo must be reachable at github.com/dustin/stagehand (or the namespace is reconciled repo-wide)."
   And `release.github.owner: dustin` (L68) is set EXPLICITLY with the comment "WINS over git-remote
   auto-detect (which is `dabstractor`)." The config DELIBERATELY overrides the git remote.

**Why changing to `dabstractor/` would be WRONG:** it would create a cross-surface inconsistency — go.mod
says `github.com/dustin/stagecoach` but goreleaser would publish to `github.com/dabstractor/stagecoach`;
the PRD's `brew install dustin/tap/stagecoach` wouldn't match a `dabstractor/homebrew-tap` tap. The existing
owner note already resolved this question (dustin/ is canonical; the repo will be reachable there before the
first real tag; snapshot validation publishes nothing so the owner doesn't affect the validation gate).

**Snapshot validation is unaffected:** `goreleaser release --snapshot --clean` publishes NOTHING — the
`dustin/` owner only matters for a REAL release (which is a future, separate concern, documented by the
owner note). So keeping `dustin/` is safe for this task's validation.

---

## §2 — The sed is CLEAN: 23 references, all the product name, zero partial-word collisions

**Decision:** `sed -i 's/stagehand/stagecoach/g' .goreleaser.yaml`. Verified: `grep -c stagehand
.goreleaser.yaml` = 23, ALL of which are the product name in:
- L5-8: the owner-note COMMENTS (`dustin/stagehand`, `dabstractor/stagehand`, `github.com/dustin/stagehand`).
  After sed: `dustin/stagecoach`, `dabstractor/stagecoach` (the latter now ACCURATELY matches the actual
  remote `dabstractor/stagecoach`), `github.com/dustin/stagecoach` (matches go.mod). All comments become
  correct.
- L12: `project_name: stagehand` → `stagecoach`.
- L20-22: build `id: stagehand` → `stagecoach`; `main: ./cmd/stagehand` → `./cmd/stagecoach` (EXISTS);
  `binary: stagehand` → `stagecoach`.
- L39: archive `ids: - stagehand` → `stagecoach` (matches the renamed build id — CONSISTENCY preserved).
- L42: comment `stagehand_1.0.0_...` → `stagecoach_1.0.0_...`.
- L69: `release.github.name: stagehand` → `stagecoach`.
- L74: comment `brew install dustin/tap/stagehand` → `stagecoach`.
- L80/92/95/106: `homepage`/`url_template` URLs `github.com/dustin/stagehand` → `github.com/dustin/stagecoach`.
- L85: scoop `name: stagehand` → `stagecoach`.
- L90: comment `scoop install dustin/stagehand` → `stagecoach`.
- L98-99: AUR comments `stagehand-BIN`/`stagehand` → `stagecoach`.
- L103: AUR `name: stagehand-bin` → `stagecoach-bin`.
- L114/116: AUR `provides: - stagehand` / `conflicts: - stagehand` → `stagecoach`.

**No partial-word collisions**: there is no `stagehandler`, `stagehand_internal`, etc. The string
`stagehand` appears ONLY as the product name. The sed is safe.

**The build-id ↔ archive-id consistency**: `builds[0].id: stagehand` (L20) is referenced by `archives[0].ids:
- stagehand` (L39). After the sed BOTH become `stagecoach` ⇒ the reference stays consistent. (The sed changes
both ends of the reference identically.)

---

## §3 — `main: ./cmd/stagecoach` resolves (M1.T1.S2 renamed the directory)

**Decision:** the sed turns `main: ./cmd/stagehand` → `main: ./cmd/stagecoach`. The directory `cmd/stagecoach/`
EXISTS (M1.T1.S2 renamed it; confirmed `ls cmd/` → `stagecoach stubagent`). So the goreleaser build entry
point resolves correctly. This is the one build-critical mapping (a wrong `main` → goreleaser build failure).
Verified present.

---

## §4 — `goreleaser check` is the validation gate (goreleaser IS installed)

**Decision:** validate with `goreleaser check` (goreleaser is at `/home/dustin/.local/bin/goreleaser`).
The sed changes ONLY the product name (not the YAML structure) ⇒ if `goreleaser check` passed BEFORE the
rename, it passes AFTER. PRE-EXISTING decision gates (the `formats` vs `format` gate at L43; the `aurs` gate
at L100) are UNRELATED to the rename — if they fire, they're pre-existing and not caused by this task. The
rename-specific gate is: zero `stagehand` references + the renamed fields are internally consistent (build id
matches archive ids; main points to the existing cmd/stagecoach).

The ULTIMATE validation (optional, slower): `goreleaser release --snapshot --clean` — builds + packages
locally (publishes NOTHING), producing `stagecoach_*` archives. This proves the binary name + archive naming
end-to-end. It runs `go mod tidy` (network) — may be slow; `goreleaser check` is the primary gate.

---

## §5 — Scope: .goreleaser.yaml ONLY; Makefile is S1 (parallel); docs are M4.T1

**Decision:** S2 edits `.goreleaser.yaml` ONLY. The Makefile is S1 (P1.M3.T1.S1, parallel — global sed
producing `./bin/stagecoach`). The README install instructions (brew/scoop/go-install paths) are M4.T1.S1
(P1.M4). CI workflow is S3 (P1.M3.T1.S3). No conflict: S1=Makefile, S2=.goreleaser.yaml, S3=.github/,
M4=docs/ — disjoint files.

---

## §6 — No new deps; the sed touches only the one config file

**Decision:** ONE file (`.goreleaser.yaml`), ONE sed command. No Go code, no go.mod change, no new deps.
The goreleaser config is a static YAML file — the rename is a pure text substitution.

---

## Summary table (the 6 calls at a glance)

| § | Decision | Source |
|---|----------|--------|
| 1 | KEEP `dustin/` (NOT `dabstractor/`); sed changes ONLY the product name | go.mod + PRD §21.2/§21.3 + existing owner note |
| 2 | `sed 's/stagehand/stagecoach/g'` — 23 refs, all product name, zero collisions | actual .goreleaser.yaml grep |
| 3 | `main: ./cmd/stagecoach` resolves (cmd/stagecoach EXISTS from M1.T1.S2) | `ls cmd/` |
| 4 | `goreleaser check` is the gate (installed); pre-existing decision gates are unrelated | goreleaser installed |
| 5 | .goreleaser.yaml ONLY; Makefile=S1, docs=M4.T1, CI=S3 | scope |
| 6 | One file, one sed; no deps | static YAML |
