# Architecture Decisions & Porting Map

Synthesis of the research into the binding engineering decisions. Every implementing subtask should be able to trace its contract back to a line here.

## 1. Module path & layout (PRD §14)

- Module: `github.com/dustin/stagehand` (confirm user/org segment with author — affects `go install` path & goreleaser).
- Layout exactly as PRD §14: `cmd/stagehand/main.go`; `internal/{config,provider,prompt,git,generate,ui}`; `pkg/stagehand` (public); `providers/*.toml` (reference manifests); `.goreleaser.yaml`; `Makefile`; `go.mod`.
- **v2-readiness seam (PRD §11.3):** the core MUST be `func CommitStaged(ctx, cfg) (sha, error)` (or in `pkg/stagehand`: `GenerateCommit(ctx, opts) (Result, error)`) that **assumes the index is already staged** and never decides *what* to stage. v1 `main` = `maybeAutoStage(); CommitStaged()`. Do NOT entangle staging policy with commit logic. The auto-stage-all step lives in the CLI layer, not in `generate`.

## 2. The four interfaces that define the contract graph

Every package boundary is a typed contract. Implement these first (or their interfaces) so subtasks can be developed against stubs in parallel:

1. **`internal/git.Git`** (wrapper) — methods: `RevParseHEAD() (string,bool,error)`, `WriteTree() (string,error)`, `CommitTree(parent, msg, tree string) (string,error)`, `UpdateRefCAS(ref, new, expected string) error`, `HasStagedChanges() (bool,error)`, `AddAll() error`, `StagedDiff(cfg) (string,error)`, `CommitCount() (int,error)`, `RecentMessages(n int) (string,error)`, `RecentSubjects(n int) ([]string,error)`. Backed by `exec.Command("git", ...)`. Tested with a temp repo + real git binary (PRD §20.1 layer 2).
2. **`internal/provider.Manifest`** (struct, §12.1) + `Render(...) (cmd *exec.Cmd, stdinPayload string, err error)` (§12.2) + `Executor.Run(ctx, ...) (stdout string, err error)` (timeout, stdin feed) + `parseOutput(raw, m) (msg, ok)` (§12.9).
3. **`internal/prompt`** — `BuildSystemPrompt(ctx, git, cfg) (string, error)` (style learn + multi-line detect + anti-reuse) and `AssemblePayload(diff, instruction, rejected []string) string` (§17.3, ordering = diff-then-instruction per `reference_impl.md` §3).
4. **`internal/generate`** — `CommitStaged(ctx, deps) (Result, error)`: orchestrates diff→snapshot→prompt→generate(two nested loops)→dedupe→commit→CAS, with rescue on failure. Depends on the three above (injected, stubbable for tests).

## 3. The two-nested-loop generate orchestrator (★ THE core logic ★)

`internal/generate/generate.go` implements (from `reference_impl.md` §2):

```
CommitStaged(ctx, deps):
  diff = git.StagedDiff(cfg)             ; if empty → (auto-stage path in CLI layer; here: nothing-to-commit)
  PARENT_SHA, _ = git.RevParseHEAD()
  TREE_SHA = git.WriteTree()             ; on error → abort BEFORE generation
  install signal handler (→ rescue if TREE_SHA set)
  sys = prompt.BuildSystemPrompt(...)
  recentSubjects = git.RecentSubjects(50)
  instruction = "Generate a commit message for these changes:"
  rejected := []string{}
  var msg string; var ok bool
  // OUTER: duplicate rejection (FR30–FR33)
  for dupAttempt := 0; dupAttempt <= cfg.MaxDuplicateRetries; dupAttempt++ {   // = initial + N retries
      payload = prompt.AssemblePayload(diff, instruction, rejected)            // diff+instruction (+rejection list)
      // INNER: parse-correction (FR29) — fresh budget each outer iter
      for parseAttempt := 1; parseAttempt <= 2; parseAttempt++ {
          stdout, err = provider.Run(ctx, sys, payload)                        // timeout-enforced
          if err/timeout → RESCUE
          msg, ok = parseOutput(stdout, manifest)
          if ok { break }
          payload = correctiveInstruction                                      // "Output ONLY the commit message…"
      }
      if !ok → RESCUE                                                          // parse failed twice
      subject = firstLine(msg)
      if !isDuplicate(subject, recentSubjects) { goto COMMIT }                 // unique → done
      rejected = append(rejected, subject)                                     // dup → outer retry
  }
  RESCUE (candidate msg in hand)                                               // exhausted duplicates

COMMIT:
  NEW_SHA = git.CommitTree(PARENT_SHA, msg, TREE_SHA)                          // root repo: omit -p
  err = git.UpdateRefCAS("HEAD", NEW_SHA, PARENT_SHA)                          // 2-arg CAS; root: no expected-old
  if err (HEAD moved) → print msg + manual recovery; exit 1                    // do NOT force (§13.5)
  restore default signal handler BEFORE update-ref                             // §18.4, ref 'trap - INT TERM'
  print [short] subject; git diff-tree --name-status
  return NEW_SHA
```
**Counting:** `dupAttempt` runs `0..MaxDuplicateRetries` inclusive ⇒ 1 initial + N retries (PRD FR32 wording). Each outer iteration has its own 2-try inner budget. `MaxDuplicateRetries` default 3 (configurable).

## 4. Output parsing pipeline (`parseOutput`, PRD §12.9)

`func parseOutput(raw string, m Manifest) (msg string, ok bool)`:
1. `s = TrimSpace(raw)`
2. if `m.strip_code_fence && (s starts with ``` or ~~~)`: drop fence-opener line (incl. lang tag) and everything from last closer; re-trim
3. switch `m.output`:
   - `raw` → `msg = s`
   - `json` → try `json.Unmarshal(s)`; on fail find first `{`…last `}` (balanced) and retry; extract `obj[m.json_field]` as string; any failure → fall back to `raw` (msg=s) and set parse-fallback flag for logging
4. normalize `\r\n`→`\n`; collapse 3+ newlines → 2
5. `msg = TrimSpace(msg); ok = msg != ""`

Table-driven tests: raw, fenced-raw, fenced-json, json-in-prose, fallback-to-raw, empty→(ok=false).

## 5. Command rendering (`Manifest.Render`, PRD §12.2)

Builds `args []string` in this order: `subcommand…`, (`provider_flag`,`provider`), (`model_flag`,`model`), (`system_prompt_flag`,`sys`), `bare_flags…`, `print_flag`. Delivery: stdin → payload to `cmd.Stdin`, nothing appended; positional → `args += [user]`; flag → `args += [prompt_flag, user]`. `cmd.Env = os.Environ() + m.Env`. **No `sh -c`** — direct `exec.Command` with `[]string` (security, §19). Set `SysProcAttr.Setpgid=true` for process-group kill on signal (§18.4). System-prompt-via-stdin fallback when `system_prompt_flag==""`.

## 6. Config precedence (PRD §16, FR34)

Resolved in `internal/config.Load()` lowest→highest: builtin defaults → builtin manifests → global file (`$XDG_CONFIG_HOME/stagehand/config.toml`) → repo file (`./.stagehand.toml`) → repo git-config (`stagehand.*`) → env (`STAGEHAND_*`) → CLI flags. Provider manifests **merge field-by-field** (a user override setting only `default_model` keeps the rest of the built-in manifest). Print a one-line notice when a repo-local config overrides the provider (§19 trust). Defaults: timeout 120s, auto_stage_all true, max_diff_bytes 300000, max_md_lines 100, max_duplicate_retries 3, output raw, strip_code_fence true, subject_target_chars 50.

## 7. Safety invariants (PRD §18.1, §20.2) — tests MUST assert these

- **Idempotent index:** after any failure path, `git diff --cached --name-only` is unchanged. Snapshot before/after.
- **Atomic HEAD:** after CAS failure, `git rev-parse HEAD` is unchanged.
- **Snapshot immutability:** `git cat-file -p <TREE_SHA>` stable regardless of later staging.
- Never call `git update-ref` without the expected-old (except root commit). Never `--force`. Never `git commit`.

## 8. Integration test seam (PRD §20.1 layer 3)

A **stub provider**: a tiny program (Go test binary or shell script) reading stdin and emitting a canned message (or canned failure/dup/timeout). `generate.CommitStaged` is driven end-to-end against a temp repo with this stub injected as the provider. Covers: success, dup-retry-then-success, parse-fail-then-rescue, timeout, CAS-failure (move HEAD mid-test), root commit, auto-stage-all. Real-agent runs are a separate `//go:build integration_real` suite (manual, `STAGEHAND_RUN_REAL=1`).

## 9. Porting map (reference → Go location), PRD Appendix C, enriched

| `commit-pi` section | Go location | Notes |
|---|---|---|
| `handle_error()` rescue | `internal/generate/rescue.go` | verbatim message + PRD enrichment (candidate msg) |
| `trap 'handle_error' INT TERM` | `main.go` signal handler | process-group kill + grace SIGKILL; restore before commit |
| md + other diff capture | `internal/git/diff.go` `StagedDiff` | per-file md 100-line + total 300KB caps; exclusions identical |
| `PARENT_SHA=$(git rev-parse HEAD)` | `git.RevParseHEAD` | empty allowed |
| `TREE_SHA=$(git write-tree)` | `git.WriteTree` | abort on conflict-in-index |
| commit_count / examples / multiline | `internal/prompt/examples.go` | awk multiline heuristic (ref §6) |
| system_prompt construction | `internal/prompt/system.go` | RAW contract (ref used JSON) [D1] |
| agent invocation | `internal/provider` + manifest | 6 manifests, 4 corrected [ext_deps §C] |
| JSON sed parse | `internal/provider/parse.go` | `parseOutput` robust pipeline [§4] |
| duplicate-retry loop | `internal/generate/dedupe.go` + `generate.go` | OUTER loop; wraps INNER parse loop [§3] |
| `commit-tree` + `update-ref` | `git.CommitTree` / `git.UpdateRefCAS` | CAS preserved |
| `git diff-tree --name-status` | `main.go` | identical UX |
| `trap - INT TERM` | signal handler restore | same intent |
