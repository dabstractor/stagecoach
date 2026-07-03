# How Stagehand works

Architecture overview of the Stagehand pipeline: snapshot-based commit creation, stage-while-generating, the safety and rescue protocol, and prompt engineering. This is the cross-cutting "architecture overview" — it ties together the git plumbing, orchestrator, rescue protocol, and prompt assembly.

## The snapshot-based flow

### Why not `git commit`

Stagehand does not use `git commit`. The standard `git commit` reads the **live** index and mutates `HEAD` — it locks the repo state for the duration. If you stage a file while a commit is in progress, that file may end up in the commit unexpectedly.

### The plumbing alternative

Instead, Stagehand uses three low-level git plumbing commands:

1. `git write-tree` — freezes the current index into an immutable tree object (the **snapshot**). The index is never reset.
2. `git commit-tree` — creates a dangling commit object from the frozen tree (no ref mutation).
3. `git update-ref HEAD` (compare-and-swap) — advances `HEAD` to the new commit atomically. If `HEAD` changed meanwhile, the CAS fails and the commit is aborted.

### Snapshot invariants

These four invariants hold for every run (PRD §13.3):

1. **Frozen content** — the committed content is exactly what was staged at `write-tree` time. Nothing added afterward can affect it.
2. **Later-staged files stay staged** — the index is never reset. Files staged during generation remain staged for the next run.
3. **Atomic and safe** — `update-ref CAS` is the only ref mutation. A failed generation leaves the repo byte-for-byte unchanged (only orphan tree/commit objects are left for `git gc`).
4. **Overlap-able latency** — generation time is dead time only if the user does nothing. With the snapshot, the user can stage the next batch while the current message generates.

## Stage-while-generating

The snapshot decouples "what's committed" from "what's staged now." The user can keep working while Stagehand generates:

```text
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagehand                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagehand        # next run commits these
```

Generation time is no longer dead time. The in-flight commit only ever contains what was staged when it started.

## Multi-commit decomposition

v2.0's headline feature: run `stagehand` with a dirty working tree and nothing staged, and it automatically splits the changes into a sequence of logically-coherent commits — one per concept — using a four-role agent pipeline.

### Trigger

Decompose activates when **nothing is staged**, **auto-stage-all is on** (the default), and the user has **not opted out** (`--single`, `--no-decompose`, or `--commits 1`). If something is already staged, the single-commit path runs unchanged. `--dry-run` also forces the single-commit preview (decompose commits, so dry-run honors the single preview).

### The four roles

| Role | Mode | Job | Output |
|------|------|-----|--------|
| **planner** | bare | Analyze the full working-tree diff; decide how many commits and what each covers | JSON `{count, single, commits:[...], message?}` |
| **stager** | tooled | Stage one concept's subset of files (`git add`, hunk-level staging) | Mutates the index; exits 0 |
| **message** | bare | Generate a commit message from the concept diff | Raw commit message text |
| **arbiter** | bare | Decide which just-made commit any leftover changes belong to, or create a new commit | JSON `{target: "<sha>"\|null}` |

### Pipeline flow

```text
                 nothing staged + dirty working tree
                              │
                              ▼
            ┌────────────┐   full working-tree diff (binary placeholders)
            │  planner   │◀──── + style examples
            │ (bare)     │
            └─────┬──────┘   JSON: {count, single, commits:[…], message?}
                  │ single? ──yes──▶ git add -A → CommitStaged (one call) → done
                  ▼ no (N concepts)
         for i in 0..N-1:
            ┌────────────┐  concept[i] description        ┌────────────┐
            │  stager[i] │──────────────────────▶ index   │            │
            │ (tooled)   │   (mutates index; no commit)   │            │
            └─────┬──────┘                                │            │
                  ▼ tree[i]=write-tree (FROZEN)            │            │
            ┌────────────┐  diff(tree[i-1],tree[i])  ═══▶ │  message[i]│ (bare)
            │            │                                │ (overlaps) │
            │            │  ‖ stager[i+1] runs here       │            │
            └─────┬──────┘                                └─────┬──────┘
                  ▼ msg[i]                                      │
            commit-tree -p newSHA[i-1] tree[i] msg[i] ◀──────────┘
            update-ref HEAD newSHA[i] newSHA[i-1]   (serialized)
                  ▼
         git status clean? ──yes──▶ done
                  │ no
                  ▼
            ┌────────────┐  commits made + leftover diff   target SHA or null
            │  arbiter   │◀───────────────────────────▶  (stagehand does all git)
            │ (bare)     │
            └────────────┘
```

### Key design points

**Overlapped staging and generation.** `stager[i+1]` runs in parallel with `message[i]` — the stager prepares the next concept's index while the message agent generates the current commit message. This 1-deep overlap keeps latency low.

**Stage-while-editing (FR-E2).** With `--edit`, the snapshot is frozen *before* the editor opens. You can `git add` in another pane during the edit session — the in-flight commit is unaffected. This is the same stage-while-generating property, extended through the editor. This is the one thing `git commit -e`-style flows cannot offer on top of generation.

**Frozen tree snapshots.** After each stager returns, `write-tree` freezes the accumulated index into an immutable tree object (`tree[i]`). This is the SAME snapshot mechanism as the single-commit path, composed N times.

**Tree-to-tree diffs.** `message[i]` reasons over `diff(tree[i-1], tree[i])` — never `index-vs-HEAD`. This makes each concept diff immune to concurrent staging and to earlier commits landing.

**Serialized publication.** Even though generation overlaps, `commit-tree` + `update-ref` are serialized per concept (CAS). If `HEAD` moved externally, the CAS fails and prior commits stand.

**Start-of-run freeze (T_start).** The instant decomposition activates, the entire working-tree change set (every modified/added/deleted/untracked path and its byte content) is captured as an immutable tree object T_start. The planner partitions T_start's diff (never a fresh re-read of the live tree); every stager, the arbiter's leftover staging, and the one-file/single shortcuts stage content drawn strictly from T_start. A file created or modified after T_start is captured is invisible to the run.

**Freeze enforcement.** Because the stager is an external agent running `git` against the live tree, after each staging step stagehand verifies the resulting tree is a content-subset of T_start (only T_start paths, T_start content). Any deviation — a concurrent change swept in, or a stager that ran a bare `git add -A` — is a hard abort (non-rescue; already-landed commits stand per FR-M12).

**One-file short-circuit.** In auto-decompose, if exactly one path changed, the planner is bypassed entirely: stage that file's T_start content, generate one message, create one commit (FR-M2b). Deterministic, not model judgment. `--commits N` (N≥2) overrides this shortcut.

**Arbiter leftover reconciliation.** After all N concepts are committed, if `git status --porcelain` shows remaining changes, the arbiter decides whether they belong to an existing commit (amend) or warrant a new (N+1)th commit.

### Safety

The same snapshot-based safety invariants from the single-commit path apply to every decompose iteration:

- **Atomic and safe** — `update-ref CAS` is the only ref mutation per commit; stagehand owns all `commit-tree`, `update-ref`, and `push` operations. The stager is the ONE role that touches the index. Its scoping differs by provider: claude is structurally constrained to a staging-only git allowlist (`git add`/`apply`/`status`/`diff`); pi is constrained instructionally (its task prompt) plus a HEAD-movement guard that aborts the run if the stager moves a ref. See [providers.md](providers.md#tooled-mode-and-the-stager-role).
- **Frozen content** — `tree[i]` captures exactly what was staged at `write-tree` time. Nothing added afterward can affect it.
- **No index resets** — the index accumulates across concepts. After the final commit, HEAD.tree == tree[N-1] == full accumulated index, so the index is clean relative to HEAD.
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. Each staging step is verified as a content-subset of T_start.

See [configuration.md](configuration.md) for per-role model configuration and [cli.md](cli.md) for the decompose and per-role flags.

### Binary and non-text file filtering

Binary files, lock files, snapshots, sourcemaps, and vendor directories are **excluded from every diff payload** — staged diff, working-tree snapshot, and concept diff. They are replaced with a `<status>\t[binary] <path>` placeholder so the agent sees *that* the file changed without the useless binary hunk. This applies identically in the single-commit and multi-commit paths.

### Payload exclusions (.stagehandignore)

Exclusion patterns from `.stagehandignore`, the `[generation] exclude` config key, or the `--exclude`/`-x` CLI flag hide a file's **diff body** from every payload while still committing the file exactly as it stands. Excluded files emit a `<status>\t[excluded] <path>` placeholder (same shape as the `[binary]` placeholder, distinguishable by tag) so the agent sees *that* the file changed without its contents.

**Payload-only guarantee (FR-X5):** Exclusion is payload-only — it never alters staging or commit content. The excluded file is committed exactly as staged, and `git diff-tree` of the resulting commit includes it. Only what the agent *sees* is affected.

The built-in noise denylist (lock files, snapshots, sourcemaps, vendor directories) always applies alongside any user exclusions — the two sets are unioned, never replaced. See [configuration.md](configuration.md) for `.stagehandignore` syntax.

## Safety and the rescue protocol

### Safety invariant

No provider mutates the repository (PRD §18.1). Every built-in manifest constrains the agent to a read-only mode — either via explicit tool-disable flags (pi, claude) or read-only constraint flags (codex, cursor, gemini). The agent receives the diff via stdin/argv and writes the commit message to stdout — it never runs `git add`, `git commit`, or any write command.

### Failure modes and exit codes

| Failure | Exit code | Recovery |
|---------|-----------|----------|
| Agent missing on `$PATH` | 1 (Error) | Check the `[provider.<name>] command` path; install the agent |
| Unresolved merge conflicts in the index | 1 (Error) | Resolve the conflicts, then re-run `stagehand` (caught before the snapshot) |
| Generation failed (parse/retry exhaustion) | 3 (Rescue) | Rescue message with tree SHA |
| Generation timed out | 124 (Timeout) | Rescue message with tree SHA |
| CAS failure (HEAD moved meanwhile) | 1 (Error) | HEAD-moved message |
| Nothing to commit (clean tree) | 2 (NothingToCommit) | Stage files and retry |
| General error | 1 (Error) | Inspect error message |

The rescue (3) and timeout (124) rows are the real-commit path; under `--dry-run`, a generation failure reports exit 1 instead — see [Rescue protocol](#rescue-protocol).

See [cli.md](cli.md#exit-codes) for the full exit-code table.

### Rescue protocol

When generation fails after the snapshot is taken on a real commit (exit 3 or 124), Stagehand prints a recovery block to stderr with the frozen tree SHA and the exact `git commit-tree` command to commit manually:

```text
❌ Commit generation failed.
------------------------------------------------------------
Your staged files were safely snapshotted before generation.
Tree ID: <TREE_SHA>

To commit the originally staged files manually:
  git commit-tree -p <PARENT_SHA> -m "Your message" <TREE_SHA> | xargs git update-ref HEAD

(omit "-p <PARENT_SHA>" if this is the repository's first commit)
------------------------------------------------------------
```

If a candidate commit message was produced but rejected (duplicate subject or parse failure), it is appended to the rescue block so the user can paste it into the manual command.

Under `--dry-run`, the full pipeline still runs and the snapshot is still taken, but a generation failure (timeout or parse/duplicate-check exhaustion) exits **1** with a short stderr message and omits this recovery recipe — no commit was ever intended. The recipe and exit codes 3/124 apply to a real `stagehand` commit.

## Prompt engineering

### System prompt (mature repos)

For repos with more than one commit, Stagehand builds a system prompt from the last 20 commit messages:

- **Style learning** — the agent sees recent messages as examples of the project's conventions.
- **Anti-reuse** — a prohibition against copying the wording of any recent commit. Combined with a separate 50-subject dedupe check, this ensures every generated subject is unique.
- **Subject length** — the target is ~50 characters (configurable via `subject_target_chars`).
- **Multi-line rule** — if recent commits use multi-line messages, the agent is instructed to follow the same convention.

### System prompt (new repos)

For repos with zero or one commit (including unborn repos), Stagehand falls back to a **conventional-commit** system prompt (PRD §17.2): "Use Conventional Commits format (type: description)."

### Format modes and locale

`--format` (default `auto`) controls how the system prompt shapes the commit message, and applies everywhere a message is produced: the message role, the planner's single-commit shortcut, and the arbiter's leftover-commit message.

- **`auto`** — the default described above: learn style from recent commit history.
- **`conventional`** — replaces the learned-style examples with an explicit `type(scope): description` contract (`feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`).
- **`gitmoji`** — replaces the examples with an instruction to begin the subject with one [gitmoji](https://gitmoji.dev) emoji, followed by the compiled-in emoji reference table (no network fetch).
- **`plain`** — replaces the examples with nothing: no learned style, no format contract, just the essence of the change.

For any mode other than `auto`, the recent-commit history examples are omitted entirely — useful for repos with an idiosyncratic or empty history. The multi-line rule and subject-length target still apply in every mode.

`--locale` (e.g. `--locale French`, `--locale ja`) appends one line — "Write the commit message in `<locale>`." — to the system prompt in every format mode. The value is passed through as-is with no translation or validation.

### User payload

The user payload combines the staged diff with the rejection list (previously rejected subjects). On a parse-failure retry, the retry instruction ("Output ONLY the commit message. No preamble, no markdown, no quotes.") is prepended as a corrective preamble.

### Why raw output, not JSON

Stagehand requests raw text output from agents (`output = "raw"`) rather than structured JSON (PRD §17.4). Reasons:

- Agents that produce raw text are easier to invoke — no need to negotiate a JSON schema.
- A raw contract is more robust across different agent versions and providers.
- The parser handles code-fence stripping and newline normalization, which covers the common raw-output quirks.
- JSON mode is available as a fallback for agents that only produce structured output.

## Hook mode vs the snapshot-based flow

### Trade-off inversion (FR-H7)

Stagehand offers two ways to generate commit messages, each with different trade-offs:

**Snapshot-based flow** (the default `stagehand` command):
- **Atomic**: uses `git write-tree` to freeze the index, then `git commit-tree` + `git update-ref` to publish — the repo is byte-for-byte unchanged on failure (no orphan commits, no partial state).
- **Bypasses pre-commit hooks**: because the commit is built via plumbing (not `git commit`), tools like husky, lint-staged, and `.pre-commit-config.yaml` do NOT run on the generated commit.
- **Stage-while-generating**: the snapshot decouples staged content from generation time, so you can keep staging while the message generates.
- **Rescue protocol**: if generation fails after the snapshot, the frozen tree SHA is printed so you can commit manually.

**Hook mode** (`stagehand hook install` + `git commit`):
- **Pre-commit hooks honored**: the commit flows through the standard `git commit` path, so husky, lint-staged, and any other `pre-commit` hooks run normally.
- **No snapshot guarantees**: the index is live during generation — if you stage more files while the hook runs, they may affect the commit. Generation latency is inside the commit flow (no overlap).
- **Never-block contract**: any failure leaves the message file untouched and exits 0, so the commit proceeds to an empty editor — the commit is never aborted by a model hiccup (unless `--strict` opts in).
- **No rescue protocol**: there is no frozen tree to recover — the commit simply proceeds without an AI message.

### When to use which

- Use **hook mode** for day-to-day commits in your IDE or lazygit — zero ceremony, pre-commit hooks run, never blocked.
- Use the **snapshot-based flow** (`stagehand` directly) when you need atomicity, stage-while-generating overlap, or are scripting/batch-committing.
- The two **compose**: install the hook for `git commit`, and run `stagehand` directly when you want the atomic path.
