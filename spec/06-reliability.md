## 18. Error handling, rescue protocol, and safety

### 18.1 The invariant

**The repository's refs and index are modified only at the final `update-ref` step, and only if HEAD is unchanged since the snapshot.** Every code path that does not reach a successful `update-ref` leaves the repository byte-for-byte unchanged (modulo harmless dangling objects).

### 18.2 Failure modes and responses

| Failure                                           | When                   | Response                                                       | Exit  |
| ------------------------------------------------- | ---------------------- | -------------------------------------------------------------- | ----- |
| Nothing staged, `--no-auto-stage`                 | pre-snapshot           | "Nothing staged."                                              | 2     |
| Nothing staged, auto-stage on, but clean tree     | pre-snapshot           | "Nothing to commit."                                           | 2     |
| Merge conflicts in index                          | `write-tree`           | "Resolve merge conflicts first."                               | 1     |
| Agent missing on `$PATH`                          | pre-generation         | "Provider 'X': command 'Y' not found. Is the agent installed?" | 1     |
| Generation timeout                                | executor               | kill agent, rescue                                             | 124/3 |
| SIGINT/SIGTERM                                    | any time post-snapshot | kill agent, rescue                                             | 3     |
| Empty/invalid output after retries                | parse                  | rescue                                                         | 3     |
| Duplicate after all retries                       | dedupe                 | rescue                                                         | 3     |
| `update-ref` CAS failure (HEAD moved)             | commit                 | print message + manual recovery (do NOT force)                 | 1     |
| Planner unparseable / fails (v2)                  | pre-staging            | surface error; nothing snapshotted yet                         | 1     |
| Decompose would exceed `max_commits` (v2)         | pre-staging            | error: raise `--max-commits` / `--commits`                     | 1     |
| Stager stages nothing / exits non-zero twice (v2) | mid-loop               | skip concept (no empty commit); log; continue                  | 0     |
| `message[i]` fails mid-loop (v2)                  | mid-loop               | rescue **for concept i only** (§13.6.6); prior commits stand   | 3     |
| Arbiter returns invalid/unknown target (v2)       | post-loop              | default to a NEW commit (null)                                 | 0     |

### 18.3 The rescue message (FR43–FR45)

When `TREE_SHA` is set and `NEW_SHA` is not:

```
❌ Commit generation failed.
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: <TREE_SHA>

To commit the originally staged files manually:
  git commit-tree -p <PARENT_SHA> -m "Your message" <TREE_SHA> | xargs git update-ref HEAD

(omit "-p <PARENT_SHA>" if this is the repository's first commit)
------------------------------------------------------------
```

If the failure was a duplicate-exhaustion or parse failure _with a candidate message in hand_, additionally print: _"A candidate message was produced but rejected: \"<msg>\". You can use it manually in the command above."_ — so the user's wait wasn't wasted.

**Multi-commit variant (v2; §13.6.6).** When a single concept fails mid-loop, the rescue is scoped to that concept's frozen `tree[i]`: print `tree[i]`, its parent (`newSHA[i-1]`), and the same `commit-tree | update-ref` recipe. Already-published commits 0..i-1 are final and untouched; any concepts whose staging completed remain staged for the user to finish. The arbiter is not run when the loop aborts via rescue.

### 18.4 Signal handling

Stagecoach installs a `signal.Notify` handler for `SIGINT`, `SIGTERM`, and (Unix) `SIGHUP` (FR-K3). On receipt of `SIGINT`/`SIGTERM`:

1. If a child process (agent) is running, send it SIGTERM (then SIGKILL after a grace period) via its process group (`SysProcAttr.Setpgid = true` so we can kill the whole tree).
2. If the snapshot has been taken, run the rescue path; else just exit.
3. Restore the default signal handler before the final `update-ref` so a Ctrl-C at the very last instant isn't mistaken for a failure (matching `commit-pi`'s `trap - INT TERM` before commit).

`SIGHUP` (Unix; FR-K3) takes the same rescue path: when the controlling terminal closes and the kernel hangs up the process group, stagecoach runs the rescue path and releases the lock file (§18.5) instead of dying under Go's default disposition and leaving the file for the next run's reaper. It is the terminal-hangup complement to the parent-death watchdog (FR-K1), which catches the detach/orphan case where no signal is delivered at all. The three signals share `RestoreDefault` (step 3) for the `update-ref` window.

### 18.5 Concurrency: the per-repo run lock (FR52)

The stage-while-generating workflow (§13.4) is safe for a **single** stagecoach process: the snapshot is frozen before generation, and staging files during generation cannot move HEAD, so the commit's CAS (§13.5) cannot be tripped by staging alone. What is **not** safe is two stagecoach processes running against the same repo at the same time — whichever commits first moves HEAD, and the loser's CAS aborts (§13.5), leaving a dangling snapshot and, in the common duplicate-run case, the "already committed" message. The run lock makes that race structurally impossible to stumble into. It is the **first** line of defense (prevents the common local double-run); the §13.5 CAS is the **second** (the never-clobber-HEAD guarantee, which holds even on a shared filesystem the lock cannot cover). Both stay — defense in depth.

**Scope.** Every commit-producing action acquires the lock: the default action in **both** its single-commit and decompose modes. Read-only subcommands (`config`, `providers`, `--version`, `--help`) bypass it — they never mutate refs.

**Location — global per system, never inside the repo.** A lock file living in the repo (`.git/stagecoach.lock` or the working tree) is rejected: it pollutes `git status`, can be committed accidentally, is ambiguous across worktrees/checkouts, and is lost on clone. Instead the lock lives in a **per-system, per-user runtime directory**, keyed by the repository's absolute path:

- `$XDG_RUNTIME_DIR/stagecoach/locks/<hash>.lock` when `XDG_RUNTIME_DIR` is set — the preferred location (tmpfs, per-login, auto-cleaned at logout; exactly what runtime locks are for).
- Otherwise `$XDG_CACHE_HOME/stagecoach/locks/<hash>.lock`, falling back to `~/.cache/stagecoach/locks/<hash>.lock` (the XDG convention used for the global config, §16.1).
- `<hash>` = `sha256` of the repository's **canonical absolute path**, hex-encoded. Hashing keeps the filename charset/length safe and yields exactly one lock file per repo. Two different repos hash differently and lock independently; two terminals in the **same** repo hash identically and contend — which is precisely the case to serialize.

**Contents.** The lock file holds `pid`, `hostname`, the repo path, a start timestamp, and — once the holder freezes its snapshot — `snapshot=<frozen-tree-sha>` (the `WriteTree` result on the single-commit path; `T_start` on the decompose path; one `key=value` per line). The `pid`/`hostname`/repo are diagnostic (they let the contention message name _who_ holds the lock); the `snapshot` enables the no-op fast path below. The `pid`/`hostname` are also reused for stale-_file_ reaping (below) — never for stale-_lock_ decisions, which `flock` makes unnecessary.

**Mechanism — advisory `flock`, not a sentinel file.** The lock is taken with `flock(2)` (`LOCK_EX | LOCK_NB`) on the file descriptor of `<hash>.lock` (created if absent). `flock` is released **automatically** when the process exits — including under `SIGKILL` or a crash — so the _lock_ is never stale: a contender's `LOCK_NB` never blocks on a dead holder, and no PID-liveness heuristic is needed to decide whether the lock is _held_. This deliberately avoids the fragile `O_CREAT|O_EXCL`-plus-PID-check pattern, whose stale-lock bugs are the classic failure mode for hand-rolled locks. What `flock`'s auto-release does **not** clean up is the lock _file_ — inert on disk after any exit that bypasses the deferred cleanup (the signal-rescue `os.Exit`, `SIGKILL`, a crash), and reaped separately below.

**Stale-file reaping (lock vs file).** Because `flock` auto-releases on exit, a contender's `LOCK_NB` never sees a dead holder — `Acquire` simply re-`flock`s the reused path (`O_CREATE`) and proceeds. The orphaned _file_, though, is inert disk litter: harmless to correctness (no live process holds its fd) but unbounded over time (every interrupted run leaves one). To keep the directory clean, `Acquire` reaps: after taking its own lock, it removes every `*.lock` in the lock directory whose recorded `pid` is **not a live process on its recorded `hostname`** (`kill(pid, 0)` returning `ESRCH`). A dead `pid` holds no open fd and therefore no `flock`, so unlinking its file is safe — it cannot defeat contention the way unlinking a _live_ holder's file would (the inode-bound-`flock` hazard). A live `pid` is **never** reaped, even if it appears stuck: this preserves FR52's "never force-break" guarantee, and the pid-liveness check is precisely what makes reaping safe (reaping by age/timestamp is rejected — a slow-but-live run must never have its file pulled out from under it). Hostname-matching scopes reaping to this host; a recycled `pid` _on this host_ is a benign miss (the file is skipped this run and reaped once the `pid` is free).

**Exit-path release (prevention).** The most frequent staleness producer is the signal-rescue path: a `SIGINT`/`SIGTERM` after the snapshot runs the §18.3 rescue and calls `os.Exit(3)`, which **skips** the deferred cleanup that would otherwise remove the lock file (`flock` still auto-releases via fd close at process teardown — only the _file_ is orphaned). The signal handler therefore releases the lock file immediately before exiting, via the same injected-seam used for the rescue formatter (the signal package stays stdlib-only and cannot import the lock package). This does not replace reaping — `SIGKILL` and crashes still skip it — but it removes the common producer, so reaping is a backstop rather than the hot path.

**Orphaned-but-alive — the launcher-closed case (FR-K1–K7).** The two states above (dead holder, file reaped; live holder, never touched) miss a third that arises from stagecoach's primary launch path (§9.21): a parent process — the lazygit TUI, an IDE, a detaching terminal — that **closes without killing its child**. The child is reparented to init (or a subreaper) and **keeps running**: it still holds the `flock`, its `pid` is still alive (so stale-file reaping never fires), and it is generating a commit nobody will see. Because §18.4's handler catches only `SIGINT`/`SIGTERM` and the orphaning parent delivers neither, the run never reaches the exit path above — the lock file outlives the launcher until the orphan finishes (or forever, if its provider subprocess is wedged by the vanished terminal). This is the "lock stays forever" report. The remedy is **self-termination, never contender-side force-breaking** (FR-K1): on startup stagecoach records its parent pid and arms a watchdog that, on parent death, routes the process through the same rescue + `ReleaseCurrent` exit path the signal handler uses — abandoning the in-flight commit is always safe (HEAD moves only at `update-ref`; the snapshot is a gc'able orphan whose SHA the rescue recipe prints). Detection is by **parent-pid change** (reparenting), not the brittle `getppid()==1` test (FR-K2); `SIGHUP` is added to the caught signals so a terminal hangup also routes through rescue instead of a raw terminate (FR-K3); `stagecoach lock status` prints the holder's path/liveness/orphan-status read-only so a blocked user can decide to `kill` or `rm` themselves (FR-K4); the watchdog is Unix-only and has an opt-out for intentional detachment (`nohup`/`setsid`/`systemd-run`) (FR-K6–K7). FR52's "never force-break" guarantee is preserved unchanged: the guarantee is that _another_ process never breaks a live lock, and the watchdog is the _same_ process abandoning its own unwanted work.

**Contention behavior.** If `LOCK_NB` fails (another stagecoach holds the lock), stagecoach does **not** block — the user is interactive, and blocking would hang their terminal. First it tries the **no-op fast path**: if the holder has published a `snapshot=` and the contending run's own staged snapshot (`write-tree`, which is index-read-only and therefore safe to take without the lock) is byte-identical to it — i.e. the path-diff is empty — then nothing new has been staged since the holder began, so the contending run is redundant (the common accidental-double-run). It exits **0** with _“nothing to do — an in-progress run already covers your staged changes.”_ If a genuine second batch _is_ staged (the diff is non-empty), it instead reads the holder's `pid`/`hostname`/repo and exits non-zero with a message of the form:

> stagecoach: another stagecoach run is already in progress on `<repo>` (pid `<N>` on `<host>`). Your newly-staged changes will remain staged — re-run `stagecoach` after it finishes.
>
> Lock: `<path>`
>
> (If the holder's launcher has exited — e.g. you closed lazygit — it is orphaned and holding this lock uselessly; `stagecoach lock status` (FR-K4) confirms, then `kill <N>` or `rm <path>` to clear it.)

The non-zero exit code is distinct from the commit-failure codes so scripts can tell "busy, retry" from "failed" (§15.4; add exit `Busy`). Stagecoach never force-breaks the lock. (Auto-committing that second batch instead of refusing is the depth-1 subtractive queue — deferred; see Appendix F.)

**Limits.** The lock is **per-host**: on a shared/network filesystem mounted by two machines, two stagecoach processes on different hosts can still race (their `flock`s are local to each host) — the §13.5 CAS catches that. The lock serializes stagecoach with stagecoach only, not against other tools (an editor, another coding agent); excluding those is the snapshot/freeze boundary's job (FR-M1b), not the lock's.

---

## 19. Security considerations

- **No shell interpolation.** Commands are built as `[]string` and run via `exec.Command` directly, never via `sh -c` / `zsh -c`. The diff payload is delivered via stdin, never interpolated into an argument. This eliminates the entire class of shell-injection bugs that a naive port could introduce. (The original `commit-pi` ran under `zsh -c` because of the git-alias mechanism; Stagecoach execs directly and is safer.)
- **Commit-identity transparency (FR-39a).** Stagecoach never sets, overrides, or injects a git author/committer identity — not `user.name`/`user.email` in any config scope, and not `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env on any git subprocess. Every commit is authored and committed as whoever git resolves from the user's own configuration; if git cannot resolve an identity, stagecoach surfaces git's error verbatim and exits non-zero rather than falling back to a stagecoach-branded identity. A commit made by stagecoach is indistinguishable in metadata from one the user made by hand (no `"stagecoach agent"` author, no stagecoach-domain email, no `Co-Authored-By:` trailer, no generated-by footer). Branding the commit as machine-made would be exactly the kind of unsolicited mutation this tool refuses to do.
- **No secret handling.** Stagecoach never reads, logs, or transmits the agent's credentials. The agent owns its own auth; stagecoach only spawns it with the inherited environment (plus any manifest-declared `[env]` additions). Logs in `--verbose` print the command and flags and the **payload size** (byte count + token estimate — the size only), but never the stdin **contents** unless `stagecoach_VERBOSE=2`.
- **Diff content is local.** The diff never leaves the machine except via the user's own agent over the user's own authenticated channel. Stagecoach's commit-generation path (§9.1–§9.28) makes no network calls itself; the sole exception is `stagecoach upgrade` (§9.29), which fetches the project's own release artifacts and checksums from GitHub Releases — never provider credentials, never a diff, never repo data.
- **Config file trust.** Config files are user-owned (`~/.config` and repo-local). A repo-local `.stagecoach.toml` could be committed by an attacker to change a user's provider — but it can only redirect commit generation to another _installed_ agent the user already trusts; it cannot exfiltrate credentials or run arbitrary commands (manifests specify a `command` + flags, not arbitrary shell). Still, Stagecoach will print a one-line notice when a repo-local config is loaded that overrides the provider, so the redirection is visible. (Hardening for v1.1: restrict repo-local configs to non-`command` fields unless `stagecoach_TRUST_REPO_CONFIG=1`.)
- **`--dangerously-*` flags never auto-set.** Stagecoach will not pass `--dangerously-skip-permissions` or equivalent to any agent. Bare mode means "no tools, no session, no chrome" — not "skip safety checks." For agents where disabling tools requires an empty allowlist (Claude's `--tools ""`), we use that; we never use the bypass-permissions flags.
- **The stager is the one tooled exception (v2).** The per-concept staging agent runs with git tools on (§11.5, §13.6.2). Its toolset is **scoped** — a git/read/edit allowlist expressed via `tooled_flags` — and it is instructed (and structurally constrained) to only update the index (`git add`-class ops); it cannot commit, amend, or push, because stagecoach owns every ref mutation via `update-ref`. The unscoped `--dangerously-skip-permissions` bypass is **never** used to achieve this; a provider whose only non-interactive tool-execution path is the unscoped bypass cannot serve as a stager (its `tooled_flags` stays empty).

---

## 20. Testing and QA strategy

### 20.1 Layers

1. **Unit — pure functions.** `parseOutput` (table-driven: raw, fenced, json, json-in-prose, fallback), command rendering (per provider, golden files), prompt construction (style-learning, multi-line detection, anti-reuse text), duplicate detection, config precedence resolution.
2. **Unit — git wrapper, with a real git binary.** Each `internal/git/*` test creates a temp directory, `git init`, stages known content, and asserts on `WriteTree`/`CommitTree`/`UpdateRefCAS`/`StagedDiff`/`RecentMessages`. These are fast (git is fast) and catch real plumbing regressions.
3. **Integration — full flow with a stub provider.** A fake agent: a tiny Go binary (or shell script) that reads stdin and writes a canned message to stdout. Drives `generate.CommitStaged` end-to-end and asserts the resulting commit exists in the repo with the right tree, parent, and message. Covers: success, duplicate-retry-then-success, parse-failure-then-rescue, timeout, CAS failure (simulate by moving HEAD mid-test), root commit, auto-stage-all. **v2 adds a parallel stub suite for `decompose.Decompose`**: stub planner (canned JSON partition), stub stager (a scripted `git add` of named paths — no real tooled agent in CI), stub arbiter (canned target/null). Covers: auto-decompose into N, `--commits N` forced count, single-shortcut, empty-concept skip, mid-loop rescue, arbiter new/tip-amend/mid-chain-rebuild, binary-placeholder propagation, the `stager[i+1] ∥ message[i]` overlap (assert tree[i] is frozen before stager[i+1] runs via interleaving checks), and **concurrent-change exclusion (FR-M1b/M1c/M1d)**: the harness writes a new file to the working tree mid-run and asserts it lands in no commit and remains in the working tree post-run — _including when the loop otherwise commits all of `T_start` and the arbiter gate is reached_ (FR-M1d: the arbiter must skip, not sweep, the concurrent file). A second case writes the concurrent file _and_ forces a legitimate frozen leftover (empty-skip one concept) and asserts the arbiter folds only `T_start` content; the concurrent file still lands in no commit.
4. **Integration — real agents (opt-in, not in CI).** A `//go:build integration_real` suite that invokes the actual `pi`/`claude`/etc. if installed and `stagecoach_RUN_REAL=1`. Used manually before releases; skipped in CI.

### 20.2 Property/invariant tests

- **Idempotent index:** after any failure path, `git status` output is identical to before the run (no index mutation). Asserted by snapshotting `git diff --cached --name-only` before and after.
- **Atomic HEAD:** after a CAS failure, `git rev-parse HEAD` is unchanged.
- **Commit-identity transparency (FR-39a):** every commit created by a stagecoach run has `author` and `committer` identical to what the user's `git config user.name`/`user.email` (and any `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env) resolve — never a stagecoach-branded identity. Asserted by resolving the expected identity before the run and comparing against `git log -1 --format='%an <%ae> | %cn <%ce>'` of the resulting commit; also assert the commit message contains no `Co-Authored-By:`/`Generated-by:` trailer that stagecoach did not put there. A test that pre-seeds a repo-local `user.name = "stagecoach agent"` confirms stagecoach inherits it (transparency) but that no code path ever _writes_ such a key.
- **Snapshot immutability:** `git cat-file -p <TREE_SHA>` is stable across the run regardless of subsequent staging.
- **Concept isolation (v2):** for every commit in a decompose run, `diff-tree <newSHA[i]>` (vs its parent) equals exactly the concept's files — no leakage from sibling concepts. Asserted by comparing each commit's file set to the stager's recorded paths.
- **`T_start` completeness (v2.2, replaces "Loop index cleanliness"):** after a fully-successful run, every `T_start` leftover is committed — either the loop committed all of `T_start` (frozen leftover empty → arbiter skipped) or the arbiter folded the remaining `T_start` content into a commit. The live `git status --porcelain` may still be non-empty, but ONLY from concurrent changes outside `T_start`, which are intentionally left unstaged (FR-M1d).
- **Mid-chain amend fidelity (v2):** after an arbiter-driven mid-chain rebuild, the rebuilt chain's non-target commits are byte-identical (same tree, same message) to the originals, and only the target commit's tree grew by the leftover set.
- **Start-of-run freeze (v2):** a file created or modified in the working tree _after_ decomposition begins appears in no commit of the run and remains in the working tree afterward. Asserted by writing a sentinel file between decompose start and completion and checking it is untracked/unchanged across every produced commit (FR-M1b/M1c).
- **Arbiter freeze parity (v2.2):** the arbiter gate is `diff-names(tipTree, T_start)` (frozen), never `git status --porcelain`; and the resolution stages from `T_start`, never the live tree. Asserted by writing a sentinel file mid-run, driving the loop to commit all of `T_start`, reaching the arbiter gate, and checking (a) the sentinel is in no commit, (b) the sentinel remains in the working tree post-run, (c) no arbiter commit is created when the frozen leftover is empty. A paired case with a legitimate frozen leftover asserts the arbiter commit's tree is exactly `T_start` (FR-M1d/M9/M10).

### 20.3 Coverage target

≥85% on `internal/git`, `internal/provider`, `internal/generate`, `internal/config`. Lower bar for `internal/ui` (hard to test, low risk). Enforced in CI with a coverage gate.

### 20.4 CI matrix

GitHub Actions: build + test on `{linux, macos, windows} × {amd64, arm64}`, Go `1.22` and `1.23`. `golangci-lint`. `govulncheck`. Release on tag via goreleaser.

### 20.5 End-to-end scenario harness (strongly encouraged)

The concurrency and routing invariants above are easy to specify, easy to regress, and — as repeated field discoveries have shown — easy to break silently (unit tests with stub agents cannot reach them). Maintain a throwaway-repository harness (a script or a `//go:build e2e` test) that, **per scenario, creates a fresh `git init` temp repo, seeds it, runs `stagecoach`, and asserts the resulting history** — driving the real agent where feasible (the `integration_real` suite) or a stub. **Every bug found in the wild becomes a scenario here.** The current must-cover set:

- nothing staged, N unrelated files → N commits (auto _and_ `--commits N`);
- exactly one file changed → single commit, **no planner call** (FR-M2b);
- a file created/modified by a concurrent process mid-run → excluded from every commit, left in the working tree (FR-M1b/M1c), **including across the arbiter gate** — concurrent file + loop commits all of `T_start` → arbiter skips, file stays untracked (FR-M1d);
- a model pinned on a multi-backend agent with no inference provider → **hard error**, not silent empty output (FR-R5b);
- arbiter leftover reconciliation (new commit / tip amend / mid-chain rebuild), where each arbiter commit's tree is built from `T_start` only and a concurrent working-tree change is never swept in (FR-M1d);
- rescue mid-loop; CAS abort (HEAD moved concurrently).
- **orphaned-run lock reclamation (v2.7):** launch stagecoach from a short-lived parent that exits mid-generation (e.g. `sh -c 'stagecoach' &` whose shell returns) and assert the holder self-exits via the parent-death watchdog, the lock file is removed, and HEAD + index are unchanged (no commit landed); separately, drive a launcher that delivers `SIGHUP` on close and assert the rescue path + lock release fire; then assert `stagecoach lock status` reports holder-liveness and orphan-status correctly for a live holder, a dead holder, and a reparented (orphaned) holder (FR-K1/K3/K4). The opt-out (`no_parent_watchdog`) is asserted to suppress the watchdog without affecting `SIGHUP` or `lock status` (FR-K6).
- **self-update (v3.0):** against a direct-binary install in a temp dir, `upgrade --check` reports current vs. latest and exits `6` when behind; `upgrade` downloads, SHA256-verifies against `checksums.txt`, sanity-runs the new binary (`--version` matches), backs up the prior, and atomically renames it into place (assert `os.Executable()` now reports the new version, and the prior `.stagecoach-backup` exists); a corrupted/tampered asset (bad SHA256) and a new binary that exits non-zero on `--version` both abort BEFORE any swap with the on-disk binary unchanged (FR-U5 steps 4 & 6, FR-U11); `--rollback` restores the backup; install-method detection is asserted to delegate to (not overwrite) a simulated Homebrew/npm/Scoop install path, and to refuse a self-swap of a manager-owned binary unless `--force` (FR-U1). Windows swap uses the `.exe.old` deferred-delete dance and is asserted to leave a runnable `stagecoach.exe` (FR-U7).

This harness is the regression net for the behaviors that only manifest against a real repo and (ideally) a real agent — the gap that let the planner-empty-output and concurrent-file bugs ship.

---

## 21. Distribution and release

### 21.1 Build

Go modules. `make build` → `./bin/stagecoach`. `make test`, `make lint`, `make coverage`. Version injected via `-ldflags "-X main.version=…"` at release (goreleaser sets it to the tag; `make build VERSION=vX.Y.Z` overrides). A build with no `VERSION` override (bare `go install`, default `make install`) leaves `version = "dev"`; in that case `--version` enriches it from the VCS info Go 1.18+ embeds automatically (`debug.ReadBuildInfo` → `vcs.revision` + `vcs.modified`) — e.g. `stagecoach version dev (19f4df7-dirty)` — so every build self-identifies by commit and clean/dirty state without ldflags discipline. A tagged release prints its real version verbatim; a build with no embedded VCS (`-buildvcs=false`, or a non-VCS tarball) falls back to plain `dev`.

### 21.2 goreleaser

`.goreleaser.yaml` produces:

- Archives + standalone binaries for `linux/darwin/windows × amd64/arm64`.
- Homebrew formula to a tap repo (`dabstractor/homebrew-stagecoach`; tap namespace = `dabstractor/stagecoach`).
- AUR `stagecoach-bin` (prebuilt-binary package via goreleaser; `yay -S stagecoach` resolves to it via `provides`).
- Scoop manifest (Windows) to a bucket (`dabstractor/stagecoach-bucket`).
- Chocolatey package (Windows) via goreleaser's native `chocolatey:` pipe — publishes a `.nupkg` to the community repository (chocolatey.org), so `choco install stagecoach` / `choco upgrade stagecoach` work. Chosen OVER winget (v3.3): `microsoft/winget-pkgs` runs an install-in-clean-VM Microsoft Defender scan that hard-blocks the unsigned binary every release (`validationDefender`), an unbounded per-release tax Chocolatey does not impose. `choco upgrade` needs admin, so `stagecoach upgrade` detects a Chocolatey install (FR-U2) and PRINTs `choco upgrade stagecoach -y` (FR-U4; do NOT self-swap — FR-U1: choco owns the binary).
- `.deb` (Debian/Ubuntu/Mint) + `.rpm` (Fedora/RHEL/Rocky/Alma/openSUSE) packages via goreleaser's native `nfpms:` pipe — `stagecoach_<v>_linux_{amd64,arm64}.{deb,rpm}` in every Release; install to `/usr/bin`. There is no apt/dnf **repo** (the PM updaters cannot fetch a new version), so `stagecoach upgrade` detects the `deb`/`rpm` channels (FR-U2) and prints the manual reinstall recipe (FR-U3) instead of self-swapping a PM-owned file (FR-U1).
- Checksums + a changelog.
- `go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest` works from the tagged commit.

**Beyond goreleaser (v3.0, → G27)** — the channels goreleaser has no native pipe for, built by separate CI steps:
- **npm wrapper** (`stagecoach-ai`): a thin JS package whose `postinstall` detects `process.platform`/`process.arch`, downloads the matching prebuilt binary from GitHub Releases into a cache, SHA256-verifies it against `checksums.txt`, and whose `bin` field execs the cached binary — the `esbuild`/`turbo`/`prisma` pattern. Gives `npx stagecoach-ai` (zero-install trial) and `npm i -g stagecoach-ai`. The wrapper sets `STAGECOACH_INSTALL_METHOD=npm` on every invocation so `stagecoach upgrade` recognizes the install and delegates to `npm` instead of self-swapping the cached binary (FR-U2). Handle `--ignore-scripts`/corporate-npm with a fallback message pointing at the direct binary.
- **PowerShell installer (`install.ps1`)**: the Windows analog of the Unix `curl|sh` one-liner (§21.3) — `irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex` detects `process.arch`, downloads the matching `stagecoach_<v>_windows_<arch>.zip` from the latest Release, SHA256-verifies against `checksums.txt`, extracts `stagecoach.exe` to a user-local dir (e.g. `$LOCALAPPDATA\stagecoach`), and prepends it to the user `PATH` (the `rustup`/`starship`/`uv` pattern). It is the fallback for Windows users with NO package manager. The installer places a package-manager-unowned binary and tags the install (`STAGECOACH_INSTALL_METHOD=direct`), so `stagecoach upgrade` self-swaps it (FR-U5) like any `direct` install.
- **Nix flake** (`flake.nix` in-repo): no external repo, no secret, no registry gatekeeping; `nix run`/`nix profile install`. Also powers devbox/nix-shell.
- **mise/asdf plugins**: ~30-line shell-script plugins pointing at the GitHub Release archives.

`stagecoach upgrade` (§9.29) gives every channel a single memorable update command that delegates to the channel's own updater, so the proliferation here does not multiply the user's mental load.

### 21.3 Install paths

```bash
# Homebrew (macOS / Linuxbrew)
brew install dabstractor/stagecoach/stagecoach

# Go install (anywhere with Go)
go install github.com/dabstractor/stagecoach/cmd/stagecoach@latest

# Direct binary (curl|sh one-liner from GitHub Releases)
curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/install.sh | bash

# Windows (Scoop)
scoop bucket add stagecoach https://github.com/dabstractor/stagecoach-bucket
scoop install stagecoach/stagecoach

# Windows (Chocolatey)
choco install stagecoach

# Windows (PowerShell installer — no package manager needed)
irm https://github.com/dabstractor/stagecoach/raw/main/install.ps1 | iex

# npm (zero-install trial: npx stagecoach-ai; or global install)
npm install -g stagecoach-ai

# Nix (flake — no channel/registry needed)
nix profile install github:dabstractor/stagecoach

# mise / asdf (version-manager users)
mise use stagecoach@latest   # or: asdf plugin add stagecoach && asdf install stagecoach latest

# Debian/Ubuntu (apt repo — updates flow through apt)
curl -fsSL https://github.com/dabstractor/stagecoach/raw/main/apt-archive-keyring.asc \
  | sudo gpg --dearmor -o /etc/apt/keyrings/stagecoach.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/stagecoach.gpg] https://dabstractor.github.io/stagecoach/apt stable main" \
  | sudo tee /etc/apt/sources.list.d/stagecoach.list
sudo apt update && sudo apt install stagecoach

# Fedora/RHEL (dnf repo — updates flow through dnf)
sudo curl -fsSL https://dabstractor.github.io/stagecoach/rpm/stagecoach.repo -o /etc/yum.repos.d/stagecoach.repo
sudo dnf install stagecoach

# No repo? Grab the raw .deb/.rpm from the latest GitHub Release and install directly:
#   sudo apt install ./stagecoach_<version>_linux_amd64.deb
#   sudo dnf install ./stagecoach_<version>_linux_amd64.rpm
```

### 21.4 Versioning

Semantic versioning. v1.0.0 = feature-complete against this PRD's P0/P1 set. Provider-manifest additions (new agents) are minor bumps if built-in, patch bumps if docs-only. Breaking changes to the manifest schema or public `pkg/stagecoach` API are major bumps.

### 21.5 README structure (the marketing surface)

1. Hero: the one-sentence pitch (§5).
2. The 30-second demo (asciinema/gif).
3. "Why not opencommit/aicommits?" — the coding-plan paragraph, in 3 sentences.
4. Install (the four paths above).
5. Quick start (one `stagecoach` invocation).
6. Configure your agent (`providers list` → set `stagecoach.provider`; for multi-backend providers prefix the model, e.g. `stagecoach.model anthropic/claude-haiku`).
7. The snapshot workflow (§13.4 diagram) — the "stage while it thinks" payoff.
8. Full CLI + config reference (link to docs).
9. Adding a new agent (§12.8) — the contributor hook.
10. FAQ / "Stagecoach is not for you if…"

### 21.6 Documentation website (GitHub Pages)

The `gh-pages` branch hosts the public documentation site at `https://dabstractor.github.io/stagecoach/` **in addition to** the apt/dnf package repos (§21.2/§21.3) that live under `apt/` and `rpm/` on the same branch. The site is built from `docs/*.md` with [mkdocs-material](https://squidfunk.github.io/mkdocs-material/) (`mkdocs.yml`, `requirements-docs.txt`) and deployed by `.github/workflows/docs.yml`; the apt/dnf repos are built and deployed by `release.yml`'s `apt-dnf-repo` job. Two workflows commit to one branch, so coexistence is by contract, not by partition:

- The docs deploy uses `keep_files: true`, so it only adds/updates site files at the branch root and **never deletes** `apt/` or `rpm/`.
- The release deploy clones `gh-pages` (carrying the site files through) before its force-push, so a release does not wipe the docs site.
- The docs deploy writes a `.nojekyll` so GitHub serves the tree as raw static — required for mkdocs output and harmless (strictly safer) for the apt/dnf repo.

This is presentation infrastructure over the derived docs (`docs/`, which track the shipped binary — §17), **not a product requirement or a commit-path FR surface**. It carries no `stagecoach` runtime behavior, no config, and no provider surface; it is documented here only so a future agent does not mistake the docs files on `gh-pages` for orphans and remove them.

---

## 22. Risks, assumptions, dependencies

### 22.1 Risks

| Risk                                                                                                                                                                                                                                                                                                                                     | Likelihood                    | Impact                                                                            | Mitigation                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Agent CLI surfaces change (flags renamed/removed).                                                                                                                                                                                                                                                                                       | Medium                        | Medium (an agent breaks).                                                         | Manifests are config-overridable without a release; `providers show` aids debugging; pin known-good manifest versions in docs; community can ship fixes.                                                                                                                                                                                                                                                                     |
| An agent's raw output is unreliable (preambles, fences).                                                                                                                                                                                                                                                                                 | Medium                        | Low (retry handles it).                                                           | Robust parse pipeline + retry; JSON fallback per-provider.                                                                                                                                                                                                                                                                                                                                                                   |
| Large diffs exceed an agent's context or arg limits.                                                                                                                                                                                                                                                                                     | Low                           | Medium.                                                                           | Diff cap (300 KB default, configurable); stdin delivery avoids arg limits; surface a clear "diff truncated" notice.                                                                                                                                                                                                                                                                                                          |
| `update-ref` CAS semantics misunderstood.                                                                                                                                                                                                                                                                                                | Low                           | High (data integrity).                                                            | Exhaustive tests (§20.2); never use force-update; rescue on failure.                                                                                                                                                                                                                                                                                                                                                         |
| Users expect multi-commit in v1.                                                                                                                                                                                                                                                                                                         | Medium                        | Low (disappointment).                                                             | README states v1 = single commit clearly; roadmap links to v2.                                                                                                                                                                                                                                                                                                                                                               |
| Agent invokes tools despite bare flags (e.g., a model "reads" a file).                                                                                                                                                                                                                                                                   | Low                           | Low (slower, maybe wrong message).                                                | Bare flags disable tools; output is still just a message; worst case is a slightly slower or odd commit, never repo damage.                                                                                                                                                                                                                                                                                                  |
| Codex / Cursor manifests drift or the two `# TO CONFIRM` items fail (§12.7).                                                                                                                                                                                                                                                             | Low–Medium                    | Low (safe read-only profile either way).                                          | Flag surfaces verified against real `--help`; two residual checks carried inline. `experimental` flag remains available for any future docs-only provider; manifests are config-overridable without a release.                                                                                                                                                                                                               |
| **`agy` non-TTY stdout drop (issue #76; §12.5.1)** — `agy -p` may emit no stdout when spawned as a subprocess, breaking all `agy` roles. **RESOLVED 2026-07-08 (agy v1.1.0):** no longer reproduces; agy reads piped stdin and returns stdout correctly. Retained here for history.                                                                                                                                                                                                 | ~~High~~ → resolved.                                          | ~~Block `agy` behind `experimental` + a PTY-shim workaround.~~ **Fixed:** re-verified end-to-end on v1.1.0 (stdin delivery, no `-p`, `--mode plan`); agy stays `experimental` only for the unrelated §12.5.1.1 item 4 (stager flags). No other provider is affected.                                                                                                                                                        |
| **Stager mutates the working tree/index (v2)** — the only tooled agent could stage the wrong hunks or touch unrelated files.                                                                                                                                                                                                             | Medium                        | Medium (messy index, not history corruption — it cannot commit).                  | Scoped git toolset (`tooled_flags`); instruction guardrails; stagecoach owns all refs; arbiter + empty-concept skip contain mistakes; user can always inspect `git status` before any commit lands.                                                                                                                                                                                                                          |
| **Mid-chain amend rebuild is wrong (v2)** — reconstructing the chain for a non-tip arbiter target could misorder or drop a commit.                                                                                                                                                                                                       | Low–Medium                    | High (rewrites just-made history).                                                | Deterministic `read-tree`/`write-tree`/`commit-tree` reconstruction owned entirely by stagecoach (no interactive rebase); covered by the §20.2 "mid-chain amend fidelity" invariant test; ambiguous arbiter output defaults to a safe new commit.                                                                                                                                                                            |
| **Arbiter sweeps a concurrent change into a commit (v2.0–v2.1; fixed in v2.2)** — the arbiter gate read live `git status --porcelain` and the resolution ran `git add -A`/`git add` against the live tree, so a change added after `T_start` (e.g. an editor save during the planner call) was silently committed, contradicting FR-M1b. | Medium                        | High (wrong commit contents; the headline concurrency guarantee silently broken). | FR-M1d: the arbiter gate is the frozen `diff-names(tipTree, T_start)`; the diff shown is `TreeDiff(tipTree, T_start)`; the resolution builds trees from `T_start` only (`tree′ = T_start` for new/tip; `OverlayTreePaths` for mid-chain). The live working tree is never consulted for gate, diff, or staging. Covered by the §20.2 "arbiter freeze parity" invariant and the §20.5 concurrent-across-arbiter-gate scenario. |
| **Planner context overflow (v2)** — a very large working-tree diff exceeds the planner's context window.                                                                                                                                                                                                                                 | Low                           | Medium (planner fails pre-staging).                                               | Same diff cap as v1 + binary filtering reduces payload; surface a clear "diff too large; use `--commits` or stage manually" error; no partial state (planning precedes all staging).                                                                                                                                                                                                                                         |
| **Concurrency race on the index (v2)** — stager[i+1] and snapshot[i] could overlap incorrectly.                                                                                                                                                                                                                                          | Low                           | High (wrong commit contents).                                                     | Enforced ordering: snapshot[i] is taken synchronously before stager[i+1] starts (§13.6.3 invariant 1); concept diffs are tree-to-tree, not index-vs-HEAD, so they are immune to the race by construction.                                                                                                                                                                                                                    |

**Self-update (v3.0; §9.29).** Three residual risks, each with its mitigation baked into the spec. (1) **Fighting a package manager** — a self-overwrite of a brew/scoop/chocolatey/npm-managed binary is silently reverted on that manager's next upgrade and corrupts its bookkeeping. Mitigation: the install-method detection cascade (FR-U2) plus the hard rule that the command NEVER overwrites a manager-owned binary except via `--force` with a warning (FR-U1); the failure mode degrades to "delegated to the manager" rather than a fighting overwrite. (2) **Installing a broken/tampered binary** — a corrupt download or a MITM'd asset would brick the tool. Mitigation: mandatory SHA256 verification against `checksums.txt` before any write, plus a mandatory sanity-run of the new binary (`--version` matches target, exit 0) before the atomic swap (FR-U5 steps 4 & 6); a binary that fails either is never swapped in. (3) **Platform swap bugs** — Windows locks the running `.exe`. Mitigation: the Unix path is a single atomic `os.Rename`; the Windows path is the two-step rename-old/move-new/deferred-delete dance that never leaves zero binaries, with `.old` cleanup on next launch (FR-U7). Across all three, the swap is atomic by construction, so a network/checksum/sanity failure aborts before any on-disk change — there is no "half-upgraded" state (FR-U11).

### 22.2 Assumptions

- The user has at least one supported coding-agent CLI installed and authenticated. (Anti-persona §7.4 is explicitly unsupported.)
- Git ≥ 2.20 (for `write-tree`/`commit-tree`/`update-ref` CAS semantics — all ancient, but we state a floor).
- A POSIX-ish environment for the curl|sh installer; Homebrew/Scoop/Go-install paths cover the rest.
- The agent's non-interactive mode writes the answer to stdout and exits non-zero on failure (true for pi, claude, opencode per their `--help`).

### 22.3 Dependencies

- **Go 1.22+** (stdlib `os/exec`, `os/signal`, `encoding/json`, `flag` or cobra).
- **cobra** (recommended) for CLI/subcommands, or `urfave/cli/v3`; or bare `flag` to minimize deps. (Recommendation: cobra, for `providers`/`config` subcommands and familiar UX.)
- **pelletier/go-toml/v2** for config parsing.
- **No git library dependency.** Stagecoach shells out to the real `git` binary (matching `commit-pi`). go-git is tempting but adds a large dependency and re-implements plumbing we trust the real binary for. Shelling out is simpler, matches the reference implementation, and guarantees identical semantics to the user's git.

---

