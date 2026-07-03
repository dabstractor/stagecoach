# Research: P1.M8.T3.S1 — integration_real suite + resolve Appendix E

## 0. Task contract (verbatim essence)
Build a `//go:build integration_real` suite gated behind `STAGEHAND_RUN_REAL=1`
that, for each installed agent (pi/claude/gemini/opencode/codex/cursor), runs a
real end-to-end commit generation against a temp repo with a small staged diff
and asserts a non-empty, non-duplicate commit message is produced and the commit
lands. RESOLVE Appendix E.1 (gemini stdin ~300KB), E.2 (claude `--tools ""`),
E.4a (codex exec stdout+exit0), E.4b (cursor `--mode ask` read-only). For any
field that cannot be confirmed, set a safe default + leave a `# TO CONFIRM`
comment. Record findings back into architecture/external_deps.md. NO mocking; NOT
in CI. Deps: P1.M6.T1.S1 (CommitStaged), P1.M2.T3.S1 (built-ins).

## 1. Where the suite lives + what it reuses
- **File**: `internal/generate/integration_real_test.go`, white-box
  `package generate`, with `//go:build integration_real` (Go 1.17+ form) +
  the legacy `// +build integration_real` line, matching
  `internal/provider/exec_unix.go`'s two-line convention.
- **Reuse**: when compiled with `-tags integration_real`, the ALWAYS-compiled
  `integration_test.go` (package generate, NO tag) is in the same test binary,
  so the real-agent file can call its exported-to-the-test helpers directly:
  `newTempRepo`, `writeStage`, `seedCommit`, `gitRun`, `headSHA`, `commitType`,
  `commitTreeLine`, `commitParentLine`, `stagedFiles`,
  `assertHeadAndIndexUnchanged`, `containsAll`, `rescueRendered`. DO NOT
  redefine any of these (compile collision).
- **Drive target**: `generate.CommitStaged(ctx, Deps)` directly (NOT
  `pkg/stagehand.GenerateCommit`). This mirrors integration_test.go and keeps
  the harness white-box so the package-private `Deps`/`runner` types and the
  helpers above are reachable.
- **Wiring per agent**: `Deps{ Git: git.New(dir), Runner: provider.NewExecutor(dir),
  Manifest: provider.Builtins()[name], Config: cfg, Output: ui.NewOutput(...) }`.
  NOTE: the Executor's `Dir` MUST be the temp repo so the agent subprocess runs
  there (the executor sets `cmd.Dir = e.Dir`). `provider.NewExecutor("")`
  (used by the stub suite) inherits the test process cwd — WRONG for real agents;
  pass the repo dir.

## 2. The two gates (compilation + runtime) + per-agent skip
1. **Compile gate**: `//go:build integration_real` ⇒ `go test ./...` (Makefile
   `test`) and `go test ./internal/generate/` NEVER compile it → CI is clean by
   default (PRD §20.1 layer 4: "opt-in, not in CI").
2. **Runtime gate**: each test starts with `requireReal(t)` → `t.Skip` unless
   `os.Getenv("STAGEHAND_RUN_REAL") == "1"`. Even with the tag, a plain
   `go test -tags integration_real ./internal/generate/` skips unless the env
   var is set (matches PRD wording "if installed and STAGEHAND_RUN_REAL=1").
3. **Per-agent skip**: `requireInstalled(t, name)` → `t.Skip` unless the
   manifest's binary is on $PATH. Use
   `provider.NewRegistry(provider.Builtins(), nil).Detect()[name]` (registry
   honors the manifest's `Detect` field, e.g. cursor's `agent`), or direct
   `exec.LookPath`. Either is fine; registry reuses shipped logic.

## 3. Appendix E — what each confirmation actually checks (host-verified)
Host verification (this session, all six installed) of the in-question flags:

- **E.2 claude** — `claude --help` shows BOTH `--tools <tools...>` ("Use "" to
  disable all tools") AND `--disallowedTools`/`--disallowed-tools <tools...>`.
  So the fallback `--disallowed-tools "*"` is a REAL, valid syntax if `--tools ""`
  proves insufficient (Appendix E.2's documented fallback). The default manifest
  already uses `--tools ""`; the E.2 test confirms suppression by asserting the
  agent produced a message with exit 0 AND the working tree is byte-unchanged
  (no tool mutated a file). On failure, retry with a manifest variant adding
  `--disallowed-tools "*"` to BareFlags.
- **E.4a codex** — `codex exec --help`: `[PROMPT] instructions are read from
  stdin. If stdin is piped and a prompt is also provided, stdin is appended as a
  <stdin> block`. Confirms stdin delivery + non-interactive `exec`. The E.4a
  check = the codex e2e test succeeding (non-empty stdout, exit 0 ⇒ no
  *AgentError, commit lands).
- **E.4b cursor** — `agent --help` (also `cursor agent`): `--mode ask: Q&A
  style... (read-only)` and `-p/--print: Print responses to console`. The E.4b
  check = cursor e2e succeeding AND the working tree unchanged (read-only proof).
- **E.1 gemini** — ⚠ KEY GOTCHA. `gemini --help` says BOTH "`-p/--prompt` is
  DEPRECATED, use the positional prompt" (external_deps §B.3) AND "`query`:
  Runs in interactive mode by default; use -p/--prompt for non-interactive."
  These are in tension. The current manifest is positional + empty print_flag.
  Resolution path in the suite:
  (a) run the positional e2e; if it HANGS (interactive) or errors, that proves
      positional-alone is interactive and `-p` (or stdin) is REQUIRED despite the
      deprecation;
  (b) separately test a ~300KB payload via a gemini manifest VARIANT with
      `PromptDelivery = provider.DeliveryStdin` (+ the `-p` headless flag if
      needed) — assert non-empty stdout, no truncation;
  (c) write the VERIFIED decision back: if stdin accepts ~300KB cleanly → flip
      gemini to stdin (preferred); else keep positional AND add the cap note
      (MaxDiffBytes=300000 = FR3's 300KB mitigation) + a `# TO CONFIRM`/RESOLVED
      note in external_deps.md §B.3 and providers/gemini.toml.

## 4. Read-only assertion helper (for E.2 / E.4b)
Add `assertWorktreeUnchanged(t, dir, before)` — capture `git ls-files -s` + the
content hash of every tracked file (and untracked entries via `git status
--porcelain`) BEFORE CommitStaged, and assert byte-identical AFTER. This proves
the agent's tools did not mutate the working tree (the §18.1 idempotent-index
invariant plus full worktree read-only for the read-only-constrained agents).
MIRROR integration_test.go's `assertHeadAndIndexUnchanged` style.

## 5. Real-agent timing / robustness
- Real agents are slow (10s–several minutes). Each per-agent test MUST set a
  generous `cfg.Timeout` (e.g. `10 * time.Minute`) AND the `go test` invocation
  needs `-timeout 0` or `-timeout 60m` (Go default 10m will kill the suite).
- Agents are non-deterministic: a model may occasionally emit a duplicate subject
  and trip the OUTER dup loop (still succeeds within MaxDuplicateRetries). The
  assertion is "succeeds with a unique subject" (CommitStaged guarantees the
  committed subject is unique vs recent on success), NOT "first try".
- If an agent needs interactive auth / a TTY / first-run onboarding, the test may
  fail on a fresh host — that is an acceptable Skip-or-fail with a clear message;
  record it as a TO CONFIRM rather than masking.

## 6. Recording findings (the OUTPUT deliverable)
- MODIFY `plan/001_f1f80943ac34/architecture/external_deps.md`: in §B.3
  (gemini/E.1), §B.2 (claude/E.2), §B.5 (codex/E.4a), §B.6 (cursor/E.4b) replace
  the "carried to integration / strongly indicated" language with the VERIFIED
  finding + date; update the §C summary's correction notes; append a dated
  "Appendix E resolved" note. This is the persisted-for-future-maintainers artifact.
- Optionally mirror the resolution into `providers/{gemini,claude,cursor,codex}.toml`
  TO-CONFIRM comments and `internal/provider/builtin.go` (keep a `# TO CONFIRM`
  ONLY for any field that genuinely could not be confirmed; never silently assume).

## 7. Scope boundaries (do NOT cross)
- This is PRD §20.1 LAYER 4 only. Do NOT alter the stub suite (M6.T3.S1), the
  e2e suite (M6.T3.S2), or the invariant suite (M6.T3.S3) — those run in CI.
- Do NOT change the manifest SCHEMA or the six default field values speculatively;
  only flip a verified field (e.g. gemini delivery) AFTER the real run confirms it,
  and record why. The schema (PRD §12.1) is FIXED.
- Do NOT wire the suite into the Makefile `test` target (it must stay CI-opt-in).
  A convenience `test-real` target is OPTIONAL/bonus.
- Distribution (goreleaser/Makefile/providers/*.toml) is M8.T1/T2 — coordinate
  doc edits, don't duplicate them.

## 8. References confirmed this session
- PRD.md: §20.1 layer 4 (line 1217), §12.7.2 (731), Appendix E.1/E.2/E.4 (1428+).
- external_deps.md §B.1–B.6, §C.1–C.4 (the open items).
- internal/generate/integration_test.go (helpers to reuse: newTempRepo, writeStage,
  seedCommit, gitRun, headSHA, commitType, ...).
- internal/generate/generate.go (CommitStaged, Deps, Result, sentinel errors).
- internal/provider/{builtin.go (Builtins), executor.go (Run/Parse), manifest.go
  (Manifest/Render/Delivery*), registry.go (Detect)}.
- internal/config/defaults.go (Default()), config.go (Config).
- Host `--help` for claude/gemini/codex/cursor (this session) — confirms E.2
  fallback, E.4a stdin, E.4b read-only, and the E.1 positional-vs-interactive tension.
