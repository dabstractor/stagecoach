# System Context: Stagecoach Bugfix

## Project Overview

Stagecoach is a Go CLI tool (`module github.com/dustin/stagecoach`, go 1.22) that generates
AI-assisted git commit messages using pluggable agent providers (pi, claude, agy, opencode,
qwen-code, codex, cursor). It supports single-commit and multi-commit decomposition workflows,
with configurable roles (planner, stager, message, arbiter).

## Package Layout (relevant to these bugs)

```
cmd/stagecoach/main.go          — entry point; error printing (adds "stagecoach:" prefix)
internal/cmd/config.go          — `config init`, `config upgrade`, `config init --force` commands
internal/cmd/default_action.go  — root command default action; auto-stage notice (Issue 6)
internal/config/
  bootstrap.go                  — GenerateBootstrapConfig / buildBootstrapConfig (Issues 1 & 2)
  role_defaults.go              — FR-D4 roleDefaults table (bare pi models — root cause)
  backup.go                     — WriteTimestampedBackup (the correct backup helper, Issue 3)
  migrate.go                    — v2→v3 in-memory migration
  roles.go                      — ResolveRoleModel (runtime role resolution)
  config.go                     — Config struct, CurrentConfigVersion (=3)
  load.go                       — Load() first-run fallback → bootstrapWriteConfig
  file.go                       — config file I/O helpers
internal/provider/
  manifest.go                   — Manifest.ValidateModel (FR-R5b enforcement)
  render.go                     — Render() (runtime FR-R5b check, splits inference/model)
  builtin.go                    — built-in provider manifests; pi is ONLY ProviderFlag provider
internal/git/
  tokengate.go                  — FR3d/FR3i/FR3j token-limit gate (Issue 4)
  git.go                        — StagedDiff/TreeDiff/WorkingTreeDiff (call closedLoopGate)
internal/generate/
  finalize.go                   — ErrEmptyMessage (Issue 5)
  generate.go                   — Generate() propagates ErrEmptyMessage bare
internal/decompose/
  message.go                    — mirrors ErrEmptyMessage on hooks-empty path
```

## Bootstrap Flow (relevant to Issues 1 & 2)

```
config init --provider <X>
  → runConfigInit (cmd/config.go:450)
    → config.GenerateBootstrapConfig(prov)  [or GenerateBootstrapConfigWithOverrides for --interactive]
      → buildBootstrapConfig(target, installed, overrides)  [bootstrap.go:143]
          │
          ├─ models = DefaultModelsForProvider(target)     // COPY from roleDefaults
          ├─ piBlanked = (target == "pi")                   // ONLY true when target IS pi
          ├─ if piBlanked: blank all models to ""           // correct for target==pi
          ├─ stagerName, stagerModel = StagerFallback(target, models)
          │     └─ if target can't stage (agy/opencode/qwen-code/codex/cursor):
          │        → finds pi in preferredBuiltins → returns ("pi", "gpt-5.4-mini")  ← BARE
          ├─ if piBlanked: stagerModel = ""                 // SKIPPED when target≠pi (BUG)
          │
          ├─ writeRoleBlock("stager", "pi", "gpt-5.4-mini")  ← writes BARE model (Issue 1)
          │
          └─ for other installed providers:
               writeCommentedRoleBlock(... pi bare models ...)  ← writes BARE models (Issue 2)

config init --force
  → runConfigInit → writeBootstrapFile (cmd/config.go:504)
    → config.WriteTimestampedBackup(path)  ← BACKUP before overwrite
    → os.WriteFile(path, content)

config upgrade
  → runConfigUpgrade (cmd/config.go:157)
    → upgradeConfigVersion(data, ver) → (newContent, changed)
    → os.WriteFile(path, newContent)  ← NO BACKUP (Issue 3)
```

## Provider Rendering (FR-R5b enforcement)

At runtime, when a config with `provider = "pi"` and a bare `model = "gpt-5.4-mini"` is loaded:

```
config Load → ResolveRoleModel("stager", cfg) → ("pi", "gpt-5.4-mini")
→ provider.Render("gpt-5.4-mini") on pi manifest
→ Manifest.ValidateModel("gpt-5.4-mini")
→ checks: ProviderFlag="--provider" (non-empty) AND model has no "/" → HARD ERROR
→ "provider render \"pi\": model \"gpt-5.4-mini\" on pi must be inference/model, e.g. \"zai/glm-5.2\""
```

Pi is the ONLY provider with a non-empty `ProviderFlag` (`"--provider"`, builtin.go:53).
All others use `ProviderFlag: ""` — so FR-R5b only bites on pi.

## ValidateModel Signature

`internal/provider/manifest.go:136`:
```go
func (m Manifest) ValidateModel(model string) error
```
- Returns nil if model is valid (has `/` when ProviderFlag is set, or ProviderFlag is empty).
- Returns error if ProviderFlag is set and model has no `/`.
- Called by Render() at runtime; can be called at test time for post-bootstrap validation.

## Key Design Decision: pi is Multi-Backend

Pi routes through multiple inference backends (OpenAI, ZAI, etc.) via a `--provider <backend>` flag.
Therefore the model must carry the backend as a slash-prefix: `zai/glm-5.2`, NOT `glm-5.2`.
When pi is the **target** provider in `config init`, all four role models are written BLANK with
a guidance comment (FR-D2: "there is no universally-correct inference backend"). The bug is that
this blanking is NOT applied when pi appears as the **stager fallback** or in **commented blocks**.
