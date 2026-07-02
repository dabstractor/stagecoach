# Plan Overview & Decomposition Rationale

Synthesis of how the v1.0 PRD ship list (PRD §10.1) was decomposed into the
`tasks.json` hierarchy. Read alongside `decisions.md`, `reference_impl.md`,
and `external_deps.md`.

## Milestone ordering (dependency-true DAG, acyclic)

| # | Milestone | Depends on | Rationale |
|---|---|---|---|
| M1 | Project Skeleton & UI | — | Module, cmd stub, Makefile, `internal/ui` (exitcodes + output). No provider/config types. |
| M2 | Provider System | M1 | `internal/provider`: Manifest type, Render, 6 built-ins (corrected), registry merge+detect, parse, executor. Self-contained; owns the canonical `Manifest` type. |
| M3 | Git Plumbing | M1 | `internal/git`: wrapper + exec helper + temp-repo test util, plumbing (atomicity primitives), diff (caps/exclusions), log (history queries). Self-contained. |
| M4 | Prompt Construction | M3 | `internal/prompt`: multi-line detect, system-prompt builder (raw contract), payload assembly (reference ordering). Depends on git history only. Decoupled from config (takes scalar settings). |
| M5 | Configuration Model | M1, M2 | `internal/config`: Config struct, defaults, file load (global+repo TOML), git-config reader, Load() precedence + provider-override field-merge. References `provider.Manifest` (M2). |
| M6 | Generation Orchestrator | M2, M3, M4, M5 | `internal/generate`: the two-nested-loop `CommitStaged`, dedupe, rescue, signal handling, + stub-provider e2e harness & safety-invariant proofs. **The core IP.** |
| M7 | CLI & Public Library API | M5, M6, M2, M3 | `cmd/stagehand` (cobra flags, providers/config subcommands, default action, exit codes, verbose/color) + `pkg/stagehand` public `GenerateCommit`. |
| M8 | Manifests, Distribution, Docs & Release | M7, M2 | `providers/*.toml`, `.goreleaser.yaml`, Makefile finalize, README (§21.5), docs sync (Mode B), real-agent progressive verification (resolves PRD Appendix E). |

## Key decisions encoded in the plan

1. **`Manifest` type lives in `provider` (M2); `config` (M5) imports it.** `provider/registry` does NOT import `config` — it receives provider overrides via constructor injection from the CLI layer. ⇒ no import cycle.
2. **`prompt` (M4) is decoupled from `config`.** Builders take scalar settings / a small `prompt.Settings`, so M4 can precede M5. `generate` (M6) is the integrator that bridges config ↔ prompt.
3. **Two-nested-loop is M6.T1.S1's binding contract** (outer duplicate-rejection wrapping inner parse-correction), per `reference_impl.md §2` and `decisions.md §3`. A flat retry counter is wrong.
4. **Payload byte order = `<diff>\n\n<instruction>`** (reference ordering, D5), NOT the PRD §17.3 prose. Flagged for review in M4.T3.S1.
5. **Raw output default** (D1); JSON optional. `parseOutput` pipeline = M2.T3.S1.
6. **v2-readiness seam:** `CommitStaged(ctx, deps)` assumes the index is already staged; auto-stage-all lives ONLY in the CLI layer (M7.T2.S2). Staging policy is never entangled with commit logic (PRD §11.3).
7. **Safety invariants (§18.1, §20.2)** are asserted in M6.T3.S2: idempotent index, atomic HEAD, snapshot immutability. Never `git commit`, never `update-ref --force`, never 1-arg update-ref (except root commit).
8. **Implicit TDD:** every implementation subtask carries its tests in-scope (RESEARCH→INPUT→LOGIC+TESTS→OUTPUT). Only the genuinely-separate *test harness/infra* (stub provider, temp-repo util, invariant proofs) and *progressive real-agent verification* (M8.T4) stand as their own units.

## Status legend
Phase = **Ready** (research complete). All milestones/tasks/subtasks = **Planned** until picked up.
