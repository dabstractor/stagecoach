# Research — P1.M7.T1.S1: pkg/stagehand/stagehand.go — GenerateCommit(ctx, opts) (Result, error)

The tiny, stable public surface (PRD §14.1): `GenerateCommit` + `Options` + `Result`,
a thin wrapper over `internal/generate.CommitStaged`. This note records the verified
dependency signatures and the **two design gaps** in the shipped `internal/generate`
that this task must close additively, so the implementer does not re-derive them.

## 0. The shipped `internal/generate.CommitStaged` has NO DryRun / SystemExtra seam

`internal/generate/generate.go` (shipped in P1.M6.T1.S1) implements the full
snapshot pipeline and ALWAYS commits on a unique subject. Its `Deps` struct is:

```go
type Deps struct {
    Git      gitClient
    Runner   runner
    Manifest provider.Manifest
    Config   config.Config
    Output   *ui.Output
}
```

There is **no DryRun field** and **no SystemExtra field**. The PRD §14.1 public
`Options` requires BOTH (`DryRun bool`, `SystemExtra string`). Therefore this task
makes an **ADDITIVE** change to `internal/generate` (decision below) — it does NOT
re-implement the pipeline in pkg/stagehand (that would violate the thin-wrapper
principle and decisions.md §1).

## 1. Verified dependency signatures (exact call shapes)

- `generate.CommitStaged(ctx context.Context, deps generate.Deps) (generate.Result, error)`.
- `generate.Deps{Git, Runner, Manifest, Config, Output}` — `Git` = `*git.Git`
  (satisfies the unexported `gitClient` structurally); `Runner` = `*provider.Executor`
  (satisfies `runner` structurally via Run+Parse).
- `generate.Result{CommitSHA, Subject, Message string}` — NOTE: the PUBLIC
  `stagehand.Result` adds `Provider, Model string` (resolved), so GenerateCommit
  MAPS internal→public (does not alias the types).
- `generate.ErrNothingToCommit / ErrRescue / ErrHeadMoved` — exported sentinels
  (errors.New). CommitStaged RETURNS them (never os.Exit).
- `git.New(dir string) (*git.Git, error)` — empty dir = inherit cwd; resolves `git`
  via LookPath at construction (errors if git missing).
- `provider.NewExecutor(dir string) *Executor` — `Executor.Run(ctx, m, model, provider, sys, payload) (string, error)`
  owns DefaultModel/DefaultProvider resolution (model=="" ⇒ m.DefaultModel).
- `provider.Registry`: `NewRegistry(builtins, overrides)`, `Get(name) (Manifest, bool)`,
  `List() []string` (sorted), `Detect() map[string]bool` (LookPath on m.Detect||m.Command).
- `config.Load(flags config.Flags, repoDir string) (cfg config.Config, reg *provider.Registry, trustNotice string, err error)`.
- `config.Flags{Env FlagsLayer; Flag FlagsLayer}`; `config.FlagsLayer{ConfigPath, Provider, Model *string; Timeout *time.Duration; Verbose, NoColor *bool}`.
  **GOTCHA:** a NON-NIL pointer to the ZERO value ("") COUNTS AS SET (overwrites).
  So only set a FlagsLayer pointer when the source value is genuinely non-empty/non-zero.
- `config.Config` scalars consumed by CommitStaged: `Provider, Model string; Timeout time.Duration; MaxDiffBytes, MaxMdLines, MaxDuplicateRetries, SubjectTargetChars int; Output string; StripCodeFence bool`. No DryRun/SystemExtra.
- `ui.NewOutput(stdout, stderr io.Writer, verbose, noColor bool) *ui.Output`;
  `out.Resultf`→stdout (the ONLY stdout writer), `out.Progressf`→stderr always,
  `out.Verbosef`→stderr gated. Color auto-disables when stdout is NOT a TTY (a pipe),
  so `--dry-run | tee` is byte-clean BY CONSTRUCTION with noColor=false.
- `prompt.BuildSystemPrompt(examples []string, hasMultiline, newRepo bool, target int) string` —
  the seam point for SystemExtra (append after this call).
- Default-provider resolution (mirrors `cmd/stagehand/providers.go resolveDefault`):
  `name = cfg.Provider`; if "" → first `reg.List()` entry with `Detect()==true`; when
  nothing detected, name=="". model = cfg.Model, else resolved manifest's DefaultModel.

## 2. Decision #1 — ADDITIVE `DryRun bool` + `SystemExtra string` on `generate.Deps`

Because CommitStaged is the single owner of the two-nested-loop pipeline (decisions.md §3),
the only contract-faithful way to "run the full pipeline but SKIP commit-tree/update-ref"
(PRD FR49) and "append SystemExtra to the system prompt" (PRD §14.1) WITHOUT duplicating
the pipeline is to add two fields to `generate.Deps` and two short-circuits to CommitStaged.
Both are strictly additive: when zero-valued, CommitStaged behaves IDENTICALLY to M6.T1.S1
(existing tests stay green).

- **SystemExtra:** after `sys := prompt.BuildSystemPrompt(...)`, append
  `if deps.SystemExtra != "" { sys += "\n\n" + deps.SystemExtra }`.
- **DryRun:** AFTER the dup-exhaustion `if !committed {...}` block and BEFORE the
  COMMIT block (`newSHA,err := deps.Git.CommitTree(...)`), insert:
  ```go
  if deps.DryRun {
      restoreSignalHandler()                       // disarm the post-snapshot handler
      out.Resultf("%s\n", msg)                     // FR49: print message to stdout (byte-clean)
      return Result{CommitSHA: "", Subject: firstLine(msg), Message: msg}, nil
  }
  ```
  This skips CommitTree + UpdateRefCAS + the FR42 success block; the dangling tree from
  WriteTree is GC'd (no ref ever points at it). Rescue on generation FAILURE still runs
  (the contract says "run the full diff→snapshot→generate→parse→dedupe pipeline" — the
  post-snapshot failure machinery is part of "snapshot").

## 3. Decision #2 — GenerateCommit builds Config+Registry internally (no env reading)

The PRD §14.1 `Options` carries no Config/Registry/writers. Per the contract
("resolved Config+Registry built internally or accepted in opts"), the public API builds
them via `config.Load(config.Flags{}, ".")` (repoDir=cwd), then layers `opts` on top of
the resolved cfg (opts is highest precedence). It deliberately does NOT read STAGEHAND_*
env: env/flag parsing is the CLI layer's job (T2.S1 folds env→opts before calling
GenerateCommit), and a library reading process env would surprise integrators (US12).
Precedence inside GenerateCommit: opts > config files > Default().

## 4. Decision #3 — re-export the sentinel errors for US12 (external integrators)

`pkg/stagehand` imports `internal/*` fine (same module), but an EXTERNAL module importing
`pkg/stagehand` CANNOT import `internal/generate`. So for `errors.Is` to be usable by a
library consumer, pkg/stagehand re-exports the three sentinels as package-level vars that
ALIAS the generate values (same identity ⇒ errors.Is works across packages):
```go
var (
    ErrNothingToCommit = generate.ErrNothingToCommit
    ErrRescue          = generate.ErrRescue
    ErrHeadMoved       = generate.ErrHeadMoved
)
```

## 5. Test strategy (two layers)

- **internal/generate/generate_test.go (ADDITIVE, hermetic stubGit/stubRunner):**
  - `TestCommitStaged_DryRun`: baseDeps with DryRun=true; assert Result.CommitSHA=="",
    Subject/Message set, stubGit.CommitTree + UpdateRefCAS NEVER called, no rescue.
  - `TestCommitStaged_SystemExtra`: extend `stubRunner.Run` to capture the `sys` arg
    (add `lastSys string`); assert `strings.Contains(lastSys, opts.SystemExtra)`.
- **pkg/stagehand/stagehand_test.go (integration: real git temp repo + stub binary):**
  - Compile `../internal/generate/testdata/stubagent` (mirrors BuildStubBinary's
    `go build -o <bin> ./testdata/stubagent`).
  - Write a `.stagehand.toml` in a temp repo registering `[provider.stub]`
    (command/detect = stub binary, prompt_delivery=stdin, print_flag=-p, output=raw,
    strip_code_fence=true, default_model=stub-model, name=stub).
  - Feed the stub via `t.Setenv("STAGEHAND_STUB_SCRIPT", ...)` + `STAGEHAND_STUB_STATE`
    (propagates to the child through the executor's `os.Environ()` — NO env subtable
    needed in the TOML, which sidesteps JSON/quoting hell).
  - `os.Chdir(tempRepo)` (with defer restore; NO `t.Parallel`) since GenerateCommit
    uses cwd for git.New + config.Load.
  - DryRun case: prior commit + newly staged change → Result.CommitSHA=="" AND HEAD SHA
    UNCHANGED (no ref moved) AND Provider=="stub" AND Model=="stub-model".
  - Default case: prior commit + newly staged change → Result.CommitSHA == new `git rev-parse HEAD`
    (40 hex) AND `git log` shows the stub subject.

## 6. DOCS impact (Mode A, per-item)
godoc `// Stable as of v1.0` on the exported `Options`, `Result`, and `GenerateCommit`
(PRD Appendix E.6). No separate docs/ file for this item (Options stays additive-only).

## 7. Scope boundaries (DO NOT)
- Do NOT re-implement the pipeline in pkg/stagehand (thin wrapper only).
- Do NOT read STAGEHAND_* env from GenerateCommit (CLI concern).
- Do NOT stage / call git add / AddAll / HasStagedChanges (v2 seam — staging is CLI-only).
- Do NOT change CommitStaged's existing behavior when DryRun/SystemExtra are zero
  (additive only — M6.T1.S1's tests must stay green).
- Do NOT os.Exit from GenerateCommit (return the sentinel errors; the CLI maps codes).

## 8. Confidence
9/10. Every dependency signature is verified in the shipped code; the two generate seams
are additive and behavior-preserving; the test harness (stub binary + temp repo +
os.Chdir) is proven by internal/generate's integration suite. Residual risk: the
[provider.stub] TOML + stub-binary detection path (mitigated by setting the stub script
via t.Setenv instead of a TOML env subtable).
