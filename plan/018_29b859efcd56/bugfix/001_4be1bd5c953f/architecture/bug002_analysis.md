# BUG-002: Newer-than-binary config version notice suggests destructive config init --force

## Root Cause

### The Code
`configVersionNotice` in `internal/config/load.go:629-645` has a `default` branch for
`version > CurrentConfigVersion` that produces:

```
stagecoach: config file uses schema version 99; this binary supports up to 3.
Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.
```

The `config init --force` suggestion is actively harmful: it would regenerate the config at the
OLD binary's schema version (3), discarding the newer config the binary cannot read. Per FR-B4,
the remedy for a newer-than-binary file is to **upgrade stagecoach** — full stop.

### Why Only the Newer Branch is Live
In `Load()` (load.go:192-201):
```go
if fileLoaded && cfg.ConfigVersion < CurrentConfigVersion {
    // migration branch — handles older + missing (0) versions with migrationNotice
    orig := cfg.ConfigVersion
    migrateV2ToV3(&cfg)
    cfg.ConfigVersion = CurrentConfigVersion
    if !globalInert {
        fmt.Fprint(noticeOut, migrationNotice(orig))
    }
} else if msg := configVersionNotice(fileLoaded, cfg.ConfigVersion); msg != "" {
    // version > current (ahead) — the ONLY remaining live configVersionNotice case in Load
    fmt.Fprint(noticeOut, msg)
}
```

The `version < CurrentConfigVersion` and `version == 0` branches of `configVersionNotice` are
**dead code** — the migration branch handles those cases before `configVersionNotice` is ever called.
Only the `default` (version > current) branch is reachable.

## Fix Design

### Part 1: Fix the Live (Newer) Branch — REQUIRED
**File**: `internal/config/load.go`, line 642-644.

**Current:**
```go
default: // version > CurrentConfigVersion
    return fmt.Sprintf("stagecoach: config file uses schema version %d; this binary supports up to %d. "+
        "Upgrade stagecoach, or run 'stagecoach config init --force' to regenerate.\n", version, CurrentConfigVersion)
```

**Fixed:**
```go
default: // version > CurrentConfigVersion
    return fmt.Sprintf("stagecoach: config file uses schema version %d; this binary supports up to %d. "+
        "Upgrade stagecoach.\n", version, CurrentConfigVersion)
```

### Part 2: Defense-in-Depth on Dead Branches — RECOMMENDED
The `version == 0` and `version < CurrentConfigVersion` branches also suggest `config init --force`.
These are currently dead code (migration branch handles them first), but for defense-in-depth:

- **Option A (remove the suggestion)**: Change them to advise only `config upgrade`.
- **Option B (leave as-is)**: They're dead code, so they can't fire.

The PRD recommends "Consider also removing the stale config init --force suggestion from the (currently
dead) older/missing branches for defense-in-depth." We should do this as a low-risk hardening step.

### Test Changes
**File**: `internal/config/load_test.go:1962-2004` (`TestConfigVersionNotice`)

Current test case:
```go
{"file loaded, ahead (4)", true, 4, false, []string{"schema version 4", "supports up to 3", "config init --force"}},
```

Fixed test case:
```go
{"file loaded, ahead (4)", true, 4, false, []string{"schema version 4", "supports up to 3", "Upgrade stagecoach"}},
// Also add a NOT-contains check: must NOT mention "config init --force"
```

If we do Part 2, also update:
```go
{"file loaded, missing (0)", true, 0, false, []string{"has no config_version", "config upgrade"}},      // removed "config init --force"
{"file loaded, older (1)", true, 1, false, []string{"schema version 1", "current is 3", "config upgrade"}}, // removed "config init --force"
```

## Documentation Impact
- `docs/cli.md:214`: "At load time, a missing or outdated `config_version` triggers an advisory
  pointing at `config upgrade` or `config init --force`." — Should clarify the newer-than-binary case
  advises only "Upgrade stagecoach".
- Code comment on `configVersionNotice` (load.go:625) should note the FR-B4 rationale for the newer case.