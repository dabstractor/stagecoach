## 23. Glossary

- **Coding plan** — a flat-fee subscription (Claude Max, Codex Pro, Gemini Advanced) whose usage is billed against the CLI product via proprietary OAuth, not the public API. Stagecoach's reason to exist.
- **Manifest** — a TOML description of how to invoke one agent (§12.1).
- **Provider** — a named manifest; roughly synonymous with "agent" in the UI.
- **Snapshot** — the tree object produced by `git write-tree`, freezing the index at a point in time.
- **CAS update-ref** — `git update-ref HEAD <new> <expected-old>`; updates HEAD only if unchanged since expected-old. Stagecoach's atomicity primitive.
- **Rescue** — the protocol triggered when generation fails after the snapshot (§18.3): print the tree SHA and manual recovery command.
- **Bare mode** — invoking an agent with no tools, no session, no extensions/skills/chrome, for a pure ephemeral text generation.
- **Stage-while-generating** — the workflow property (§13.4) whereby the user can stage the next batch during the current generation without affecting the in-flight commit.
- **Decomposition** — the v2 flow (§13.6) that turns a dirty, un-staged working tree into N coherent commits via the snapshot machinery.
- **Concept** — one logical unit output by the planner (a title + a description of which changes belong together); becomes one commit.
- **Planner / stager / message / arbiter** — the four agent roles in decomposition (§13.6.2): plan the partition; stage one concept (tooled); write one commit message; reconcile leftovers.
- **Bare mode vs tooled mode** — bare = tools off, text-in/text-out (planner/message/arbiter); tooled = git tools on, repo-scoped (stager only). (§11.5)
- **`tooled_flags`** — the manifest field (§12.1) that makes a provider able to serve as the stager; empty means the provider is bare-roles-only.

---

# Appendices

## Appendix A — Full v1 system prompt templates

(See §17. These are the canonical strings to be committed verbatim to `internal/prompt/system.go` as Go string constants, with the diff/examples/rejection-list interpolated at runtime.)

## Appendix B — Example terminal sessions

### B.1 Happy path

```
$ git add src/login.go src/login_test.go
$ stagecoach
↳ Snapshotting 2 staged files…  (tree 9f3a1c…)
↳ Generating with anthropic/claude-haiku in pi…
↳ Created abc1234  feat(auth): accept SAML tokens for enterprise login
   M  src/login.go
   A  src/login_test.go
```

### B.2 Stage-while-generating

```
# pane A
$ git add src/a.go && stagecoach
↳ Generating with sonnet in claude…   (takes 8s)

# pane B, during those 8s
$ git add src/b.go src/c.go          # these are NOT in the commit below

# pane A resumes
↳ Created def5678  refactor: extract auth helper
# git status now shows src/b.go, src/c.go as staged-for-next-commit
```

### B.3 Dry run

```
$ stagecoach --dry-run
↳ Generating with anthropic/claude-haiku in pi…
feat(auth): accept SAML tokens for enterprise login

(no commit created)
```

### B.4 Duplicate retry

```
$ stagecoach -v
↳ Attempt 1: subject "fix: handle null user" matches an existing commit — retrying.
↳ Attempt 2: "fix: guard against missing user record" — accepted.
↳ Created ghi9012  fix: guard against missing user record
```

### B.5 Rescue

```
$ stagecoach
↳ Generating with Gemini 3.5 Flash (Low) in agy…
^C
❌ Commit generation failed (interrupted).
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: 9f3a1c...

To commit the originally staged files manually:
  git commit-tree -p abc1234 -m "Your message" 9f3a1c... | xargs git update-ref HEAD
------------------------------------------------------------
```

## Appendix C — Line-by-line porting map from `commit-pi`

| `commit-pi` section                         | Stagecoach location                   | Notes                                                                  |
| ------------------------------------------- | ------------------------------------- | ---------------------------------------------------------------------- |
| `handle_error()` rescue                     | `internal/generate/rescue.go`         | Identical message; richer (includes candidate message on dedupe fail). |
| `trap 'handle_error' INT TERM`              | `main.go` signal handler (§18.4)      | Process-group kill of child; rescue if snapshot taken.                 |
| staged-diff capture (md + other)            | `internal/git/diff.go` `StagedDiff()` | Caps/exclusions identical, configurable.                               |
| `PARENT_SHA=$(git rev-parse HEAD)`          | `git.RevParseHEAD()`                  | Empty allowed (root repo).                                             |
| `TREE_SHA=$(git write-tree)`                | `git.WriteTree()`                     | Abort on conflict-in-index.                                            |
| commit_count / examples / multi-line detect | `internal/prompt/examples.go`         | Same heuristics, Go.                                                   |
| system_prompt construction                  | `internal/prompt/system.go`           | Raw-output contract (not JSON) per §17.4.                              |
| `pi --no-tools … -p` invocation             | `internal/provider` + manifest        | Manifest-driven; pi manifest reproduces this exactly (§12.3).          |
| JSON sed parse                              | `internal/provider/parse.go`          | Replaced by raw + robust pipeline (§12.9). JSON still available.       |
| duplicate-retry loop                        | `internal/generate/dedupe.go`         | Last 50 subjects; up to 3 retries; rejection list appended.            |
| `commit-tree` + `update-ref`                | `git.CommitTree` / `git.UpdateRefCAS` | CAS form preserved.                                                    |
| `git diff-tree --name-status` success print | `main.go`                             | Identical UX.                                                          |
| `trap - INT TERM` before commit             | signal handler restore                | Same intent.                                                           |

## Appendix D — Built-in manifest quick reference

| Provider | command        | delivery         | print        | model flag              | sys-prompt flag   | bare essentials                                                                                | output                      |
| -------- | -------------- | ---------------- | ------------ | ----------------------- | ----------------- | ---------------------------------------------------------------------------------------------- | --------------------------- |
| pi       | `pi`           | stdin            | `-p`         | `--model`               | `--system-prompt` | `--no-tools --no-extensions --no-skills --no-prompt-templates --no-context-files --no-session` | raw                         |
| claude   | `claude`       | stdin            | `-p`         | `--model`               | `--system-prompt` | `--tools "" --setting-sources "" --no-session-persistence`                                     | raw (json optional)         |
| **agy**  | `agy`          | stdin            | —            | `--model`               | _(prepend)_       | `--mode plan`                                                                                  | raw (experimental; §12.5.1) |
| opencode | `opencode run` | positional       | —            | `-m` (`provider/model`) | _(prepend)_       | —                                                                                              | raw                         |
| codex    | `codex exec`   | positional       | (exec)       | `-m`                    | _(prepend)_       | `--sandbox read-only --ask-for-approval never`                                                 | raw                         |
| cursor   | `agent`        | positional       | `-p`         | `--model`               | _(prepend)_       | `--mode ask --trust`                                                                           | raw                         |

> **`tooled_flags` (v2, not shown above):** each provider's stager profile is provider-specific and carried as `tooled_flags` (§12.1) — a git/read/edit allowlist + a non-interactive approval mode. Providers with empty `tooled_flags` (notably **`agy`** until §12.5.1.1 item 4 clears, and **opencode**) cannot serve as the stager; they can still serve the bare roles (planner/message/arbiter).

## Appendix E — Open questions (to resolve before/during v1 implementation)

1. **Gemini delivery:** confirm `stdin` accepts a ~300 KB payload without truncation; if not, fall back to positional and document the diff cap as the mitigation.
2. **Claude tools-disable:** confirm `--tools ""` fully suppresses tool use in `-p` mode for current Claude Code versions; if a model still "thinks" about tools, add `--disallowed-tools "*"` (verify syntax).
3. **opencode system prompt:** decide whether to (a) prepend to payload (simple, v1), or (b) document an `--agent` workflow where users define a `stagecoach` agent persona in `opencode.json` (nicer, v1.1).
4. **Codex / Cursor (mostly resolved):** flag surfaces verified against real `--help` (§12.7). Two residual confirmations carried as inline `# TO CONFIRM`: (a) `codex exec` writes the final answer to stdout and exits 0; (b) cursor `--mode ask` wins over `-p`'s default full-tools profile. Both are expected from the docs and are quick to confirm during the first real run.
5. **`.stagecoach.toml` trust:** finalize the v1.1 hardening (restrict repo-local overrides to non-`command` fields unless explicitly trusted).
6. **Public API stability:** decide whether `pkg/stagecoach.GenerateCommit` is v1-stable or marked experimental until v1.1. Recommendation: ship it, mark it `// Stable as of v1.0`, keep the `Options` struct additive-only.
7. **`agy` non-TTY stdout (§12.5.1.1 item 1, blocking):** confirm whether a PTY-shim makes `agy -p` emit stdout reliably under stagecoach's subprocess model, or wait for upstream issue [#76](https://github.com/google-antigravity/antigravity-cli/issues/76). Gates all `agy` roles.
8. **`agy` tooled (stager) flags (§12.5.1.1 item 4):** determine the exact non-interactive, git-scoped, non-bypass flag combination. Gates `agy` (and any provider) as a stager.
9. **Mid-chain amend plumbing (§13.6.5):** finalize the exact `read-tree`/`write-tree`/`commit-tree` reconstruction sequence (what `read-tree` base for each j, how leftovers fold in at the target) during implementation planning; prove via the §20.2 fidelity invariant.
10. **Stager toolset scope per provider:** pin the minimal allowlist each tooled profile needs (git add, read, edit, apply) so no provider's stager can do more than stage.
11. **Verify current model names per provider (FR-D5, blocking for defaults):** confirm the live flagship/mid/fast model token for each of pi, opencode, cursor, agy, qwen-code, codex, claude (e.g. is it `gpt-5.4` or newer? `gemini-3.1-pro` or newer? Claude Opus/Sonnet/Haiku current versions? `qwen3-coder-plus` current?). Record names + verification date in the manifest source.
12. **pi OpenAI routing:** determine which of pi's current sub-providers routes to an OpenAI model (openrouter? a native openai sub-provider?) so pi's shipped `backend/model` default is wired end-to-end; if none is universal, ship pi's model empty and let `config init` prompt for the prefix.
13. **Config `upgrade` mechanics:** finalize how `config upgrade` preserves user values vs. comments-out renamed keys (FR-B5) — keep it simple (no value-type migration) until a real rename occurs. _(Resolved: surgical in-place edit, settled by FR-B5/FR-B7/FR-B8/FR-B9. The upgrade-clobber incident traced to an inert all-commented file: a commented `config_version` read as "legacy," the load notice nudged the user to `config upgrade`, and upgrade left the inert file all-commented while appending a stray version line and reporting success. FR-B9 kills the false alarm on inert files; FR-B8 makes every write preserve active settings; FR-B4 makes the advisory advisory-only and upgrade-only.)_
14. **lazygit `customCommands` schema (gates FR-I5):** verify against the current lazygit release the exact field names (`output` vs the older `subprocess`/`showOutput`), the `context` value for the files panel, and config-dir resolution via `lazygit --print-config-dir`; confirm the chosen comment-preserving YAML approach (e.g. `yaml.v3` Node API) round-trips a real hand-maintained config byte-identically outside the edited node. Record names + verification date (FR-D5 discipline).
15. **Hook script portability (gates FR-H1):** confirm the POSIX-sh `prepare-commit-msg` script runs under git-for-windows' sh, and that `git rev-parse --git-path hooks` resolves correctly under `core.hooksPath` and linked worktrees.
16. **Gitmoji table currency (gates FR-F3):** embed the canonical gitmoji set at build time; verify the list against the spec at implementation and record the date.
17. **`list_models_command` per provider (gates FR-L1/L2):** determine which agent CLIs actually expose a model listing (opencode's `opencode models` is the known case) and populate the field only where verified; everyone else falls back to the curated FR-D4 table.

## Appendix F — Decision log (key calls and why)

- **Shell out to agents, not call APIs.** Because coding-plan quotas are unreachable over the public API. This is the product. (§2.2, §4.3)
- **Go, not TS.** Distribution fit (Homebrew/binary) matches the lazygit/gh audience; zero runtime dependency. (§2.3)
- **Raw output default, not JSON.** Removes the double-quote constraint and fragile parsing; JSON remains an option per-provider. (§17.4)
- **Shells out to real `git`, no go-git.** Matches the proven reference; identical semantics; smaller dependency surface. (§22.3)
- **v1 = single commit; multi-commit is v2.** Keeps v1 shippable; the snapshot foundation makes v2 a loop over v1. (§10.1, §11.3)
- **Auto-stage-all on by default in v1.** Per author's explicit request; the quickest path to a checkpoint commit; `--no-auto-stage` escapes it. (§9.4, FR16–FR20)
- **Multi-commit promoted from v2 into the core spec (this revision).** The snapshot foundation made it a loop over v1; the dirty-tree-and-nothing-staged state is the natural trigger; `--commits N` / `--single` give the user count control + an escape hatch. (§13.6, §10.3)
- **Only the stager is tooled; everything else stays bare.** One new manifest field (`tooled_flags`) instead of a second schema; keeps the bare-mode safety story intact for planner/message/arbiter. (§11.5, §12.1)
- **Concept diffs are tree-to-tree, not index-vs-HEAD.** This is what makes `stager[i+1] ∥ message[i]` safe — the diff is immune to concurrent staging and to commits landing. (§13.6.3 invariant 2)
- **Per-role models with a global fallback.** Different tasks warrant different agents, but one global model must still cover everything; `[defaults]` is the fallback, `[role.*]` is opt-in granularity, and it stays back-compatible with v1. (§16.4)
- **Binaries replaced with filename+status placeholders, never dropped silently.** The decomposition planner needs to know a binary asset changed to group it correctly. (§9.1, FR3b)
- **`agy` ships experimental behind its `# TO CONFIRM` block.** Honest about the non-TTY stdout bug (#76) rather than pretending the manifest is verified; matches the §12.7.2 progressive-verification ethos. (§12.5.1)
- **Decoupled defaults from the author's z.ai subscription.** pi no longer ships `glm-*`/`zai`; defaults are account-agnostic and the z.ai setup is a documented personal override. (§9.16 FR-D2, §12.3)
- **Cascading provider priority (pi → opencode → cursor → agy → codex → claude).** Open/self-hostable harnesses first; closed subscription CLIs last; highest-priority _installed_ one wins. (§9.16 FR-D1)
- **Tier-based per-role defaults (smart/mid/fast), materialized by a populated `config init`.** Each role is sized to its job (stager mid not fast — it needs tool-use; message fast — it's bare), and the bootstrap config writes them uncommented so it works out of the box. (§9.16, §9.17)
- **Model defaults are research-driven and refreshable, not pinned from stale knowledge.** The implementing agent verifies current names per provider; a future automated refresh process keeps them current. (§9.16 FR-D5)
- **Config schema versioning + advisory staleness warning.** Simple integer version + a warning + `config upgrade`; no auto-migration (no existing users). (§9.17 FR-B4/B5) _(Updated: `config_version` is bumped only on a genuinely breaking schema change; additive changes never trigger an advisory. The load-time advisory points only at the non-destructive `config upgrade`, never `config init --force`.)_
- **Config writes never clobber active settings (FR-B8).** Every config-writing command (`init`, `init --force`/`--template`, `upgrade`, bootstrap) first detects every active `key = value` grouped by its `[table]` heading and carries it verbatim into the written file, with a timestamped backup of the prior file — config analogue of FR-H2's never-clobber rule. Covers **every** write path including `config upgrade` (no exemption). Added to close the upgrade-clobber incident, in which `config upgrade` itself replaced a user's config with one containing no uncommented settings. (§9.17 FR-B8)
- **No false "legacy" alarm on inert files (FR-B9).** The load-time migration notice must not fire on a file with zero active settings — a commented `config_version` in an all-commented/template file is not "missing," and there is no `default_provider` to fold. Root cause of the upgrade-clobber incident: an inert file was mis-classified as legacy, the user was nudged to `config upgrade`, and upgrade left it all-commented (appending a stray version line at the end) while reporting success. FR-B9 kills the false alarm and makes upgrade on an inert file an honest no-op. (§9.17 FR-B9)
- **Manifests are config-overridable, compiled-in as defaults.** Decouples "support a new agent" from "cut a release." (§12.1, §12.8)
- **Competitor parity decided by rule, not by taste (v2.1).** Features both incumbents share → accepted; features contradicting the core (no HTTP/API keys, non-interactive atomic default, style learning, scope discipline) → disqualified even when both have them; the rest judged on simplicity/value. `COMPETITOR-ANALYSIS.md` is the evidence base. (§10.4)
- **Format modes are an opt-in override, not a new default (v2.1).** Style learning stays the flagship; `--format` exists for teams with a mandated convention and for repos with no history worth learning. An explicit mode drops the history examples entirely rather than mixing two masters. (§9.19)
- **Hook mode ships with a never-block contract (v2.1).** A generation failure must never stop a commit; the hook exits 0 and leaves the message file untouched. `--strict` inverts this for those who want it. Hook mode covers plain `git commit` from IDEs (§9.20, FR-H5/H7); the plumbing path's own hooks are specified separately in §9.25.
- **Hooks run on the commit path, scoped to the snapshot (v2.4).** The plumbing path runs the repo's `pre-commit`/`prepare-commit-msg`/`commit-msg`/`post-commit` around each commit without surrendering the atomic-commit core: hooks are scoped to the frozen `T_start` (subset-enforced like the stager, FR-M1c), a hook abort is a pre-`update-ref` rescue, and `--no-verify` skips only `pre-commit`+`commit-msg` (git parity). This closes the old "plumbing bypasses hooks" caveat directly. (§9.25, FR-V1–V8)
- **Payload exclusion never means commit exclusion (v2.1).** `.stagecoachignore`/`--exclude` shape what the agent sees; the snapshot always commits the staged truth. A tool whose pitch is "never corrupts your repo" does not grow a knob that makes commits diverge from the index. (§9.18, FR-X5)
- **GitHub Action rejected (v2.1).** A headless runner can only spend a coding-plan quota by exporting OAuth credentials into repo-level secrets — per-provider, fragile, ToS-hostile, and the repo's CI would drain one person's personal plan. opencommit's Action works precisely because it is an API-key tool; that is the architecture stagecoach exists to refuse. (`FUTURE_SPEC.md`)
- **PR generation stays out despite ranking #2 in the analysis (v2.1).** §6.3 is permanent: stagecoach writes commit messages. Scope discipline beats parity scoring. (`FUTURE_SPEC.md`)
- **Self-update SUPERSEDED (v3.0; was rejected v2.1).** The v2.1 rejection ("package managers own binary updates") assumed self-update meant a tool that downloads and overwrites its own binary — which fights whatever package manager installed it and gets silently reverted on that manager's next upgrade. With the distribution surface widening to npm/Chocolatey/Nix/mise/asdf (+ a Windows PowerShell installer) on top of Brew/Scoop/AUR/go-install, users genuinely lose track of which channel they used, so the concern got sharper, not softer. v3.0 reverses the rejection on the strength of the **inverse architecture — install-method-aware, delegate-first** (§9.29): `stagecoach upgrade` detects the install method and delegates to that channel's native updater wherever one exists, prints the command where running it needs privileges (AUR) or is declarative (Nix), and self-swaps ONLY for the direct-binary channel. It never overwrites a package-manager-owned file, so it cannot fight a manager. Clipboard and chunking stay rejected: `--dry-run --no-color | wl-copy` is clipboard mode, and 200k-token agent contexts + byte caps + decomposition keep chunk-and-combine a quality regression.
- **Integrations ship git-alias + lazygit only, behind a no-mangle protocol (v2.1).** gitui is blocked upstream (keybinds can only remap built-in actions — verified against its changelog 2026-07-02). Every file edit: parse-first, preview + confirm, backup, post-write validation with auto-restore, marker idempotency. The git alias delegates the edit to `git config` itself. (§9.21)
- **Run lock, not a run queue (FR52).** Two stagecoach processes on one repo race on HEAD (the loser's CAS aborts — §13.5). A lock makes that race impossible to stumble into; a queue that _auto-commits_ the second batch was rejected because the shared git index has no per-run marker, so the snapshot freeze is the only batch separator — which means the queue can isolate batch 2 only when run 1 commits first (it can't on run-1-failure) and is fundamentally ambiguous when both batches touch the _same file_ (path-level subtraction can't split one file's two edits; hunk-merge can conflict). The queue's real reliability boundary is disjoint files across batches, not queue depth, and it would auto-fire on the very accidental double-run we're guarding against. Of the queue idea we adopted exactly one piece today — the **no-op-on-empty-delta** fast path: a contending run with nothing new staged since the holder's snapshot exits 0 instead of erroring, so the accidental double-run degrades to a graceful "nothing to do" rather than a refused run. The depth-1 subtractive queue (auto-commit batch 2 via `diff(T1, T2)`, with a disjoint-files precondition and a manual-fallback on overlap) is recorded here as a future possibility, out of scope for now. (§18.5)
- **Self-update un-rejected via delegate-first (v3.0).** See the superseded entry above and §9.29: the single command that works across every channel without fighting any package manager, because it delegates rather than overwrites. The direct-binary swap is the exception (curl|sh/manual installs), hardened with SHA256 verification + a sanity-run + atomic rename + one-step rollback (FR-U5/U7/U8).
- **npm is a real channel for a Go binary (v3.0).** The `esbuild`/`turbo`/`prisma` pattern — a JS wrapper whose `postinstall` fetches the platform binary from GitHub Releases — reaches the huge JS/TS audience that overlaps the tool's target (devs with AI coding agents). Cost: a Node dependency for an otherwise-Node-free tool, and an `--ignore-scripts` fallback. The wrapper tags the install (`STAGECOACH_INSTALL_METHOD=npm`) so `upgrade` delegates to `npm` rather than self-swapping the cached binary (FR-U2). Chocolatey, Nix flake, and mise/asdf ride along for the Windows-package-manager, Nix-loyal, and version-manager audiences respectively, with a Windows `install.ps1` (`irm | iex`) covering the no-package-manager case.
- **§19 "no network calls" scoped to the commit path (v3.0).** The upgrade command necessarily fetches a binary from GitHub Releases, so the security claim is narrowed: stagecoach's commit-generation path (§9.1–§9.28) makes no network calls; `stagecoach upgrade` is the explicit, named exception. No provider credential, no diff, no repo data ever crosses that call — it fetches only the project's own release artifacts and checksums.

Note: this project was originally named "stagehand" and has been renamed. All references to "stagehand" must be replaced with "stagecoach".
---
