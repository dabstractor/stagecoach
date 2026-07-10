# External Dependencies — Plan 015 (v2.8 Delta)

## No new external dependencies

The v2.8 delta is entirely internal — no new Go modules, no new external tools, no new system
dependencies. The existing dependency set is unchanged:

- **Go 1.22+** (stdlib only for all changes)
- **cobra v1.10.2** (flag registration, already in use)
- **pelletier/go-toml/v2 v2.4.2** (config file parsing, already in use)
- **gopkg.in/yaml.v3 v3.0.1** (lazygit config editing, not touched here)

## Internal package dependencies (unchanged)

The v2.8 changes touch these existing packages only:
- `internal/config` — config structs, loading, precedence
- `internal/cmd` — CLI flag registration
- `internal/generate` — single-commit orchestrator, multi-turn, work-description
- `internal/decompose` — multi-commit pipeline, planner/stager/message/arbiter
- `internal/git` — git wrapper (new StagedNames method for FR-M1e)
- `internal/hook` — hook exec runtime
- `internal/provider` — executor (NO change needed)

No new packages are created. No package dependency graph changes.
