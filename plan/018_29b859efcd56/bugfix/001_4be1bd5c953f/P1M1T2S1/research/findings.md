# Research: P1.M1.T2.S1 — Fix configVersionNotice newer branch + harden dead branches + update tests (BUG-002)

**Scope**: Fix BUG-002 in internal/config/load.go configVersionNotice (drop the destructive
`config init --force` suggestion from all branches) + update TestConfigVersionNotice in load_test.go.
Pure-function edit + test update — no I/O, no other file. All line numbers verified this session.
Sibling BUG-001 (preservedDefaultProvider) is in internal/cmd/config.go — DIFFERENT file, no conflict.

## 1. The bug + the live/dead branch reality

**configVersionNotice** (load.go:629-645) — a PURE function (no I/O), 4 switch branches:
| Branch | Current suggestion | LIVE in Load()? |
|---|---|---|
| `version == CurrentConfigVersion` → "" | (none) | yes (no-op) |
| `version == 0` | `Run 'stagecoach config upgrade' or 'stagecoach config init --force'.` | **DEAD** |
| `version < CurrentConfigVersion` | `Run 'stagecoach config upgrade' or 'stagecoach config init --force'.` | **DEAD** |
| `default` (version > current) | `Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.` | **LIVE (the bug)** |

**Why only the default branch is live** (load.go:192-201): Load()'s migration branch handles
`cfg.ConfigVersion < CurrentConfigVersion` (which includes 0) BEFORE configVersionNotice is called —
it runs `migrateV2ToV3` + prints `migrationNotice(orig)`. The `else if msg := configVersionNotice(...)`
runs ONLY for the remaining case (version > current). The comment at load.go:198-199 states this
explicitly: "version > current (ahead) — the only remaining live configVersionNotice case in Load".

So BUG-002's user-visible defect is the DEFAULT branch. But the `version==0` and `version<current`
branches ALSO suggest `config init --force` — they are dead today but would re-live if the migration
branch is ever refactored, so the PRD recommends hardening them too (defense-in-depth).

**Why `config init --force` is harmful here** (FR-B4): for a NEWER-than-binary file, regenerating at
the OLD binary's schema (3) discards the newer config the binary cannot read — the opposite of the
remedy (upgrade stagecoach). FR-B4 elsewhere forbids suggesting `config init --force` for the OLDER
case ("regenerates from a template and is a re-bootstrap, not an upgrade"); a fortiori for the NEWER case.

## 2. The exact edits (load.go configVersionNotice, lines 629-645)

**default branch (LIVE — load.go:642-644):**
- OLD: `"Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n"`
- NEW: `"Upgrade stagecoach.\n"`

**version==0 branch (DEAD, harden — load.go:636-638):**
- OLD: `"Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n"`
- NEW: `"Run 'stagecoach config upgrade'.\n"`

**version<current branch (DEAD, harden — load.go:639-641):**
- OLD: `"Run 'stagecoach config upgrade' or 'stagecoach config init --force'.\n"`
- NEW: `"Run 'stagecoach config upgrade'.\n"`

(The `version==CurrentConfigVersion → ""` and `!fileLoaded → ""` branches are unchanged.)

## 3. Mode A doc-comment update (load.go:625-628)

Add one sentence to the existing doc comment noting the FR-B4 rationale: the newer-than-binary remedy
is upgrade-only (never `config init --force`, which would regenerate at the old schema and discard the
unreadable newer config); the older/missing branches advise `config upgrade` (also not `config init --force`).

## 4. Test update (load_test.go TestConfigVersionNotice, 1962-2004)

The current table has a `contains []string` field (positive assertions only). ADD a `notContains []string`
field + loop so the BUG-002 invariant — "`config init --force` must NEVER appear in any advisory" — is
machine-enforced. Updated rows:

| Case | contains (was → new) | notContains |
|---|---|---|
| missing (0) | `["has no config_version", "config upgrade", "config init --force"]` → `["has no config_version", "config upgrade"]` | `["config init --force"]` |
| older (1) | `["schema version 1", "current is 3", "config upgrade", "config init --force"]` → `["schema version 1", "current is 3", "config upgrade"]` | `["config init --force"]` |
| ahead (4) | `["schema version 4", "supports up to 3", "config init --force"]` → `["schema version 4", "supports up to 3", "Upgrade stagecoach"]` | `["config init --force"]` |

(The two `wantEmpty=true` cases — "no file" × 2, "current version" — are unchanged; they `return` early
before the contains/notContains loops, so notContains is nil/unused for them.)

**New loop** (after the existing contains loop, before the `\n`-suffix check):
```go
for _, sub := range tc.notContains {
    if strings.Contains(got, sub) {
        t.Errorf("configVersionNotice(%v, %d) = %q, must NOT contain %q", tc.fileLoaded, tc.version, got, sub)
    }
}
```

**Verified against CurrentConfigVersion=3** (config.go:20): the ahead(4) row produces "schema version 4;
this binary supports up to 3. Upgrade stagecoach.\n" (contains "Upgrade stagecoach" ✓, no "config init
--force" ✓); older(1) produces "schema version 1; current is 3. Run 'stagecoach config upgrade'.\n"
(contains "current is 3" + "config upgrade" ✓); missing(0) produces "has no config_version; current is
3. Run 'stagecoach config upgrade'.\n" (contains "has no config_version" + "config upgrade" ✓).

## 5. COORDINATION WITH PARALLEL SIBLINGS (no conflict)

- **P1.M1.T1.S1** (Complete) + **P1.M1.T1.S2** (Implementing): BUG-001 — `preservedDefaultProvider`
  lives in `internal/cmd/config.go:449` (+ test in `internal/cmd/config_test.go:1521`). They edit
  `internal/cmd/config.go` (runConfigInit). DIFFERENT file from `internal/config/load.go`. No overlap.
- `configVersionNotice` appears ONLY in `internal/config/load.go` + `internal/config/load_test.go`
  (grep-confirmed) — no sibling touches it.
- **P1.M1.T3.S1** (Planned): docs/cli.md update — separate; my task is Mode A (the load.go doc comment).

## 6. What this task does NOT do (scope fences)

- Does NOT touch internal/cmd/config.go (BUG-001 = P1.M1.T1.S1/S2).
- Does NOT touch docs/cli.md (changeset-level docs = P1.M1.T3.S1).
- Does NOT change Load()'s migration branch (load.go:192-201) — it is correct; only configVersionNotice's
  strings change.
- Does NOT change the function signature, the `version==CurrentConfigVersion → ""` / `!fileLoaded → ""`
  branches, or the `\n`-termination.
- Does NOT change CurrentConfigVersion (stays 3).

## 7. Validation

- `go build ./...` (pure-string edit; compiles).
- `go vet ./internal/config/...`.
- `gofmt -l internal/config/load.go internal/config/load_test.go` → empty.
- `go test ./internal/config/ -run TestConfigVersionNotice -v` (the updated table).
- `go test ./internal/config/ -count=1` (full package; the item's regression gate).
- `make test && make lint`.
- Grep guard: `grep -c "config init --force" internal/config/load.go` → 0 (the invariant).
- Regression-property check: pre-fix the ahead(4) notContains assertion would FAIL (the old string
  contained "config init --force"); post-fix it PASSES. This is the test's reason to exist.