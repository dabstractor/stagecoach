## 13. The snapshot-based generation flow (the core IP)

This section is the most important in the document. It is the thing Stagecoach does that no incumbent does, and it is the foundation v2 builds on.

### 13.1 Why `git commit` is the wrong primitive

`git commit` reads the **index at commit time**, packages it into a tree, and advances HEAD. This couples three things that should be decoupled:

1. _What gets committed_ (the index contents at commit time).
2. _When the commit happens_ (synchronously, right now).
3. _Whether the commit can fail safely_ (a `git commit` that errors mid-way can leave the index and HEAD in surprising states, especially with hooks).

For an AI-commit tool, this coupling is actively harmful: the "what" was decided when the user staged files and we snapshotted, but the "when" is whenever the model finishes — potentially tens of seconds later, during which the user has every reason to keep staging. With `git commit`, the user must either sit idle (losing the overlap) or risk sweeping unintended files into the commit.

### 13.2 The plumbing alternative

Stagecoach never calls `git commit`. It uses three plumbing commands:

1. **`git write-tree`** — serializes the _current index_ into a tree object and prints its SHA. Crucially, this **does not modify the index or HEAD**. It is a pure, read-only-with-respect-to-refs operation that freezes a copy of the staging area into the object store. After this call, `TREE_SHA` refers to a permanent, immutable record of "what was staged at time T", regardless of what the user does to the index afterward.

2. **`git commit-tree (-p <parent>) -m <msg> <tree>`** — creates a commit object with the given tree, parent, and message, and prints its SHA. This also **does not touch any ref**. The commit object exists in the object store but is "dangling" (unreferenced) until step 3. `PARENT_SHA` is captured _before_ `write-tree` (actually before generation) for consistency. Because stagecoach invokes `commit-tree` without setting any `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env and without writing `user.name`/`user.email`, the resulting commit's author and committer are exactly the identity git resolves from the user's own config (FR-39a) — stagecoach is invisible in the commit metadata; it commits as the user, never as itself.

3. **`git update-ref HEAD <new-sha> <expected-old-sha>`** — the two-argument (CAS) form atomically updates `HEAD` to `<new-sha>` **only if** its current value equals `<expected-old-sha>` (i.e., `PARENT_SHA`). If HEAD has moved in the meantime (the user committed in another terminal), the update fails cleanly and the repository is untouched.

### 13.3 The resulting workflow

Because the commit is built from `TREE_SHA` (frozen) and applied via CAS `update-ref`, the following all hold simultaneously:

- **The committed content is exactly what was staged when `write-tree` ran.** Files the user stages _after_ that point are not in `TREE_SHA` and therefore not in the commit.
- **Those later-staged files remain staged.** The index is never reset by Stagecoach. After `update-ref`, HEAD's tree equals the snapshot (so the originally-staged files show as clean/committed), while the later-staged files are in the index but not in HEAD's tree — so `git status` shows them as "changes to be committed," ready for the next run.
- **The operation is atomic and safe.** If generation fails, we never reach `update-ref`; HEAD and the index are byte-for-byte unchanged. If `update-ref` fails (HEAD moved), same thing. The only artifacts left behind are a tree object and possibly a dangling commit object in the object store, which `git gc` will eventually reap (they are harmless).
- **Generation latency is overlap-able.** The user can `git add` the next batch in another pane during the blocking model call; the in-flight commit is unaffected.

### 13.4 Stage-while-generating: the user's mental model

```
Pane A (lazygit / shell)        Pane B (shell)
─────────────────────────       ───────────────────────
git add feature/login.js
stagecoach                     ┐
  ↳ snapshotting…             │  (user is free to work here)
  ↳ generating with pi…       │  git add docs/login.md
  ↳ (10s pass)                │  git add tests/login.test.js
  ↳ created abc1234           │  (these stay staged — NOT in abc1234)
                              ┘
                                stagecoach        # next run commits these
```

This is the workflow the author already uses with `commit-pi` and that lazygit's `output: none` binding makes frictionless. v1 preserves it exactly; the implementation simply never touches the index between `write-tree` and `update-ref`. This safety holds for a **single** stagecoach process; launching a second one against the same repo while the first is generating is not safe (the loser's CAS aborts — §13.5), and the per-repo run lock (FR52 / §18.5) makes that race impossible to stumble into.

### 13.5 Edge cases and their handling

- **Rootless repo (no commits yet):** `PARENT_SHA` is empty. `commit-tree` is called without `-p` (creates a root commit). `update-ref HEAD <new>` is called without the expected-old argument. Handled.
- **Unresolved merge conflicts in the index:** `write-tree` fails. Stagecoach aborts before any generation with "resolve merge conflicts first."
- **HEAD moved during generation (user committed elsewhere):** the CAS `update-ref` fails. Stagecoach prints: "HEAD moved from <PARENT> to <actual> while generating; aborting to avoid a non-fast-forward. Your generated message was: <msg>. To commit the snapshot manually: `git commit-tree -p <PARENT> -m \"<msg>\" <TREE> | xargs git update-ref HEAD`." Exit non-zero.
- **Generation timeout / SIGINT:** kill the agent, enter rescue path (print `TREE_SHA` + manual recovery).
- **Empty diff after auto-stage-all:** exit 2, "nothing to commit."
- **Agent not on `$PATH`:** `providers list` would have shown it as absent; on direct use, fail fast with "provider 'X' not found: is <command> installed?"

### 13.6 Multi-commit decomposition (the v2 core, now specified)

§13.1–§13.5 describe the single-commit primitive. This subsection specifies how stagecoach composes that primitive N times to turn one large, _un-staged_ working tree into a sequence of logically-coherent commits. It is the feature formerly deferred to "v2" (old §10.3); it is now in scope. The snapshot machinery in §13.1–§13.5 is exactly what makes it possible — and the reason the v1 foundation was built so deliberately.

Functional requirements live in §9.14; prompts in §17.5–§17.7; config in §16.4. This section is the _flow_.

#### 13.6.1 When it activates (the trigger model)

Decomposition activates **iff** nothing is staged (`git diff --cached --quiet` reports empty) **and** the working tree has changes. This replaces v1's "nothing staged → auto-stage-all into one commit" behavior (FR16) as the default for _that_ state. **The first action on activation is to freeze the entire working-tree change set into `T_start` (FR-M1b)** — the planner, every stager, the arbiter, and all shortcuts then operate on that frozen snapshot, so files created or modified by a concurrent process during the (potentially long) run are excluded from every commit. Three modes:

| Mode                         | Trigger                                         | Planner's job                                                                                                                 |
| ---------------------------- | ----------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Auto-decompose (default)** | nothing staged, no `--commits`                  | decide the count **and** partition; if it judges one commit is correct, also emit the message (single-call shortcut, §13.6.4) |
| **Forced count**             | `--commits N` (N ≥ 2)                           | skip the count decision; partition into **exactly N**                                                                         |
| **Single (escape hatch)**    | `--single` / `--no-decompose`, or `--commits 1` | planner bypassed entirely; old v1 behavior (`git add -A` → one `CommitStaged`)                                                |

If something is already staged, the single-commit primitive (§13.1–§13.5) runs **unchanged** — decomposition never re-partitions a hand-staged index.

`--commits N` is the critical user control: it asserts the answer to "how many commits?" so the planner is never asked to count (it only partitions into N). This both saves a reasoning round-trip and keeps the user in control of commit granularity. `--commits 1` is equivalent to `--single`.

**One-file short-circuit (FR-M2b).** In auto mode, if exactly one file is changed, the planner is skipped entirely — stagecoach stages that one file and generates a single commit message directly (no planner round-trip). One file cannot sensibly decompose, so the planner call is pure churn. `--commits N` (N ≥ 2) overrides this and is honored even for a single file.

#### 13.6.2 The four agent roles

Decomposition is a multi-agent pipeline. Each role is a distinct invocation, independently bindable to its own provider and model (§16.4); all default to the global `provider`/`model`.

| Role        | Mode                              | Job                                                                                                                                                                                     | Output contract                                                               |
| ----------- | --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| **planner** | bare                              | analyze the full working-tree diff; decide count (unless forced) + partition into concepts (each concept carries `files` + a per-file `description`); if single, also write the message | JSON `{count, single, commits:[{title,description,files}], message?}` (§17.5) |
| **stager**  | **tooled** (runs git in the repo) | for one concept, find **all** related changes and stage them (`git add`, hunk-stage via `git apply --cached`)                                                                           | exits 0; mutates the index; returns a short confirmation                      |
| **message** | bare                              | generate one commit message for one frozen snapshot — this **is** the §13.1–§13.5 agent, unchanged                                                                                      | raw text (the message)                                                        |
| **arbiter** | bare                              | after all commits, if changes remain, decide which just-made commit (by SHA) the leftovers belong to, or "new"                                                                          | JSON `{target: "<sha>" \| null}` (§17.7)                                      |

**Only the stager breaks bare mode.** This is the one architectural departure from §12.7.1: the stager must actually mutate the index, so it runs as a full agent with git tool access in the repo (§11.5 tooled mode). Every other role is a pure text-in/text-out call. The manifest system models this with one new field, `tooled_flags` (§12.1), used in place of `bare_flags` for stager invocations. The stager **never** commits, moves refs, or pushes — stagecoach owns all ref mutations (FR-M5, §19).

#### 13.6.3 The pipeline (sequential staging, overlapped generation)

```
planner  ──▶  concepts[0..N-1]      (one call; single-shortcut → done, §13.6.4)
   │
   ▼
for i in 0..N-1:
   stager[i]     ──▶  index now holds concepts[0..i]
   snapshot[i]   ──▶  tree[i] = write-tree   ◀── FROZEN here, before stager[i+1]
   ┌── message[i] : diff(tree[i-1], tree[i])  ──▶ msg[i]
   │        ‖   (parallel; safe — see invariants)
   └── stager[i+1]  (only if i+1 < N)        ──▶ index now holds concepts[0..i+1]
   commit[i]   =  commit-tree -p newSHA[i-1] tree[i] msg[i]
   update-ref HEAD newSHA[i] newSHA[i-1]        ◀── serialized, in order
arbiter   ──▶  (only if working tree ≠ clean) amend-or-new (§13.6.5)
```

Three invariants make `stager[i+1] ∥ message[i]` safe, all consequences of the snapshot design:

1. **`tree[i]` is frozen before `stager[i+1]` starts.** write-tree is a pure, ref/index-read-only operation (§13.2) that captures the current index. Because the orchestrator snapshots _immediately_ after stager[i] returns and _before_ launching stager[i+1], `tree[i]` records exactly concept[i] on top of `tree[i-1]` — whatever stager[i+1] does to the live index afterward cannot reach `tree[i]`.
2. **The concept diff is computed tree-to-tree, never index-vs-HEAD.** `message[i]` reasons over `git diff tree[i-1] tree[i]`, which is exactly what stager[i] added. This is independent of where HEAD points, so it is immune to concurrent staging _and_ to earlier commits landing. (The single-commit path's `StagedDiff` is index-vs-HEAD; the loop deliberately does not reuse it, for this reason.)
3. **`update-ref`s serialize.** commit[i] parents to `newSHA[i-1]` and CAS-moves HEAD only if `HEAD == newSHA[i-1]`. So commits publish in strict order even though their _generation_ overlapped. Two `commit-tree` calls may build dangling objects concurrently, but the chain of `update-ref`s is strictly sequential.

**Index model: accumulate, never reset.** Stagecoach does not reset the index between concepts. The index grows: after stager[i] it holds concepts[0..i]; `tree[i]` is that full accumulation. After commit[N-1] lands, `HEAD.tree == tree[N-1] ==` the full accumulated index, so the index is clean relative to HEAD. Any un-committed residue therefore lives **only in the working tree** (changes no stager claimed) — that is the arbiter's input (§13.6.5), not a staged-area artifact.

**File-disjoint fast-path (FR-M13/M14).** The stager is the pipeline's only **tooled** role and its only inherently serial step — every stager mutates the same live index, so staging cannot run concurrently and message generation is capped at the 1-deep overlap above. But the stager exists solely to **hunk-split** a file shared across concepts; the planner already declares a **file-level** partition (`files` per concept). When that partition is **pairwise file-disjoint** — no path in two concepts' `files` — stagecoach stages each concept **deterministically** with `git add` (adds, modifications, and deletions for those whole paths) under the unchanged accumulate-never-reset index model, **invoking no stager agent at all**. With every `tree[i]` frozen before any message starts, the N message generations then run **concurrently** and publish in CAS order, collapsing the critical path from "planner + ~N sequential steps" to "planner + one message latency." `verifyFreezeSubset` (FR-M1c) still guards every fast-path tree; unclaimed paths still flow to the arbiter (§13.6.5). Any shared file falls back transparently to the tooled stager for the whole run. The tree-to-tree-diff and serialized-CAS invariants below are unchanged; only the 1-deep overlap bound is lifted on the fast-path, and only because there is no serial stager to enforce it.

**Base cases.** `tree[-1]` is the original parent tree (`git rev-parse HEAD^{tree}`, or the empty tree for an unborn repo). For an unborn repo, commit[0] is a root commit and subsequent commits chain normally. Per-concept "what landed" is `diff-tree newSHA[i]` vs `newSHA[i-1]` = exactly concept[i] (FR42 reporting, per commit).

#### 13.6.4 The single-commit shortcut

If the planner (in auto-decompose mode) judges that one commit is the correct call, it returns `single: true` **plus the message in the same response**. Stagecoach then: `git add -A` → snapshot → `commit-tree` → `update-ref`, using the planner's message. **No separate message-generation call is made** — the trivial case stays a single agent round-trip. (If the planner's message fails the duplicate check §9.7, the standard message agent is invoked as a fallback to regenerate, then the normal commit path.)

This is why the planner's output contract carries an optional `message` field (§17.5): present iff `single == true`. It lets the planner, which has already read the whole diff, produce the message for free when N=1, instead of forcing a second call.

#### 13.6.5 The arbiter (leftover reconciliation)

After the loop, stagecoach computes the **frozen leftover** = `diff-names(tipTree, T_start)` — the `T_start` content no stager claimed (`tipTree` = the last committed tree). **The arbiter runs iff this set is non-empty** (FR-M1d). This replaces v2.0–v2.1's `git status --porcelain` gate: the live working tree is never consulted. When the loop committed all of `T_start` but a concurrent process has since dirtied the working tree, the frozen leftover is empty → the arbiter is skipped → the concurrent change is left untouched (the FR-M1b outcome). The arbiter receives the SHAs, messages, and file-lists (`diff-tree`) of every commit made _this run_, plus `TreeDiff(tipTree, T_start)` (with binary placeholders) as the diff of the remaining changes. It returns either a target SHA (one of this run's commits) or `null`. **Stagecoach performs ALL git, and all of it from frozen trees; the arbiter only decides** (FR-M9/M10):

- **`null` / "new":** `tree′ = T_start` (folding all leftovers into the tip yields `T_start`); run the message agent on `TreeDiff(tipTree, T_start)`; `commit-tree T_start -p tip` + `update-ref` as an (N+1)-th commit. No `git add`, no `write-tree` — `T_start` is already a tree SHA. Byte-for-byte the same "commit `T_start` directly" pattern as the one-file/single shortcuts.
- **`target == HEAD` (the tip):** `tree′ = T_start`; `commit-tree T_start -p <tip's parent>` reusing the tip's message verbatim; `update-ref HEAD`. A plumbing amend of the tip to `T_start`'s tree — no `git commit --amend`, no live staging.
- **`target == an earlier commit[i]` (mid-chain):** stagecoach **rebuilds the linear chain** `i..N-1`. Because stagecoach built the whole chain and holds every frozen `tree[j]` and `msg[j]`, this is a deterministic reconstruction: for each `j`, produce `tree′[j] = OverlayTreePaths(tree[j], T_start, leftoverPaths)` (the primitive below), then `commit-tree tree′[j] -p rebuiltParent` reusing `msg[j]` verbatim. The rebuilt tip equals `T_start`. This is **never** an interactive rebase and never touches refs other than HEAD.
- **Ambiguous → default to `null` (new commit).** Stagecoach never amends outside the just-made set, and never force-updates a ref.

On every path the index is then synced to `T_start` (`read-tree T_start`) so `git status` is clean for the committed set; concurrent working-tree changes remain, unstaged/untracked.

If the frozen leftover is empty after the loop, the arbiter does not run — the perfect run (and, by construction, concurrent working-tree changes cannot make it run).

**The `OverlayTreePaths` primitive (new in v2.2; lives in `internal/git`).** `OverlayTreePaths(ctx, baseTree, sourceTree string, paths []string) (treeSHA string, err error)` returns a new tree equal to `baseTree` with each path in `paths` overwritten by its state in `sourceTree`. Present path → `git update-index --cacheinfo <mode>,<blob>,<path>`; absent path (deletion-overlay) → `git update-index --force-remove <path>`. The `(mode, blob)` for present paths is read once via `git ls-tree -r --full-tree <sourceTree> -- <paths...>`. Implementation: `read-tree baseTree` (index = baseTree) → per-path `update-index` → `write-tree`. It mutates only `.git/index` and the object store (same discipline as `FreezeWorkingTree`/`ReadTree`/`WriteTree`); it never touches the working tree and never moves a ref. At its sole call site `paths` is always `diff-names(tipTree, T_start)` and `sourceTree` is `T_start`, so every path is present in `T_start` except the deletion-leftover case (a `T_start` deletion no stager claimed).

#### 13.6.6 Failure handling within the loop

Each concept is independently recoverable (extends §18):

- **Stager stages nothing** (`tree[i] == tree[i-1]`): skip commit[i] — no empty commits (FR-M8); log and continue.
- **Stager exits non-zero:** retry once; on second failure treat as empty (FR-M8) and continue, so one bad concept cannot poison the run.
- **message[i] generation fails** (parse / duplicate-exhausted / timeout): enter the rescue path (§18.3) **for concept i only**. Already-published commits 0..i-1 stand. The frozen `tree[i]` and manual recovery are printed; remaining staged work stays in the index for the user to finish by hand. (The overlapped stager[i+1], if already running, is allowed to complete so its staging is not lost — it remains staged for the user.)
- **CAS failure on commit[i]** (HEAD moved externally): abort the run with the §13.5 "HEAD moved" message; prior commits stand; the in-flight tree[i] recovery command is printed.
- **Planner fails / returns unparseable output:** no commits have been made yet (planning precedes all staging); surface the error and exit non-rescue (nothing was snapshotted).

#### 13.6.7 Why this is safe (the one-paragraph proof)

Every commit is built from a frozen `tree[i]` captured _before_ the next concept's staging begins, and its message is generated from a tree-to-tree diff that never consults the live index or HEAD. Refs move only at `update-ref`, serialized in order, each a CAS that refuses to clobber a moved HEAD. The only agent that mutates the repo is the stager, and it is scoped to `git add`-class operations — it cannot commit, amend, or push. Therefore: a failed, slow, or mis-behaving agent can never corrupt history, never lose staged work, and never produce a commit containing changes meant for a different concept. The worst case is a rescue message pointing at a frozen tree the user commits by hand — the same guarantee v1 makes, extended across a loop.

---

## 14. Package layout (Go)

```
stagecoach/
├── cmd/
│   └── stagecoach/
│       └── main.go                # entrypoint: arg parsing, wiring, exit codes
├── internal/
│   ├── config/
│   │   ├── config.go              # Config struct, Load(), precedence resolution
│   │   ├── defaults.go            # built-in defaults
│   │   ├── file.go                # TOML read (global + repo), git-config read
│   │   └── config_test.go
│   ├── provider/
│   │   ├── manifest.go            # Manifest struct (+ tooled_flags), Render(mode) → exec.Cmd spec
│   │   ├── builtin.go             # compiled-in manifests (pi, claude, agy, ...)
│   │   ├── registry.go            # name → manifest, with override merge
│   │   ├── executor.go            # run manifest, feed stdin, capture stdout, timeout
│   │   ├── parse.go               # parseOutput() pipeline (§12.9)
│   │   └── *_test.go
│   ├── prompt/
│   │   ├── system.go              # buildSystemPrompt() (style learn, anti-reuse)
│   │   ├── examples.go            # fetch last 20, multi-line detection
│   │   ├── payload.go             # assemble user payload (instruction + diff)
│   │   ├── planner.go             # planner system prompt + JSON contract (§17.5)
│   │   ├── stager.go              # stager task prompt (§17.6)
│   │   ├── arbiter.go             # arbiter prompt + JSON contract (§17.7)
│   │   └── *_test.go
│   ├── git/
│   │   ├── git.go                 # Git wrapper interface
│   │   ├── plumbing.go            # WriteTree, CommitTree, UpdateRefCAS, RevParseHEAD
│   │   ├── diff.go                # StagedDiff() with caps, exclusions, binary filtering (FR3a–c)
│   │   ├── binary.go              # detectNonText() via numstat + extension denylist; placeholder line
│   │   ├── tree.go                # RevParseTree, TreeDiff (tree-to-tree concept diff), ReadTree, StatusPorcelain (v2)
│   │   ├── log.go                 # RecentMessages(), RecentSubjects(), CommitCount()
│   │   ├── stage.go               # AddAll(), HasStagedChanges(), StagedFileCount()
│   │   └── *_test.go              # uses a temp repo + real git binary
│   ├── generate/
│   │   ├── generate.go            # CommitStaged(ctx, cfg) — the single-commit orchestrator (§13.1–5)
│   │   ├── dedupe.go              # duplicate-subject check + retry
│   │   ├── rescue.go              # rescue protocol (FR43–FR45)
│   │   └── *_test.go              # integration with a stub provider
│   ├── decompose/                 # v2 multi-commit pipeline (§13.6)
│   │   ├── decompose.go           # Decompose(ctx, cfg) — orchestrates plan→stage→gen→commit→arbitrate
│   │   ├── roles.go               # per-role provider/model resolution (§16.4, FR-R1–R5)
│   │   ├── planner.go             # planner agent call + JSON parse/retry
│   │   ├── stager.go              # tooled stager agent call (mode=tooled); snapshot/overlap scheduling
│   │   ├── arbiter.go             # arbiter agent call + amend/new/rebuild resolution (stagecoach does git)
│   │   ├── chain.go               # linear-chain rebuild for mid-chain amend (FR-M10)
│   │   └── *_test.go              # integration with stub planner/stager/arbiter + a temp repo
│   ├── hook/                      # git hook mode (§9.20)
│   │   ├── hook.go                # install/uninstall/status; script template + marker (FR-H1–H3)
│   │   ├── exec.go                # prepare-commit-msg runtime (`hook exec`, FR-H4/H5)
│   │   └── *_test.go              # temp repo; asserts never-block + foreign-hook refusal
│   ├── integrate/                 # tool-integrations exporter (§9.21)
│   │   ├── integrate.go           # target registry, detection, list/install/remove
│   │   ├── protocol.go            # the no-mangle write protocol (FR-I3): parse→diff→confirm→backup→validate
│   │   ├── gitalias.go            # git-alias target (delegates the edit to `git config`, FR-I4)
│   │   ├── lazygit.go             # lazygit customCommands target (comment-preserving YAML, FR-I5)
│   │   └── *_test.go              # golden-file round-trips; corrupt-input refusal; backup/restore
│   └── ui/
│       ├── output.go              # progress messages, color, TTY detect
│       └── exitcode.go            # canonical exit codes
├── pkg/
│   └── stagecoach/
│       └── stagecoach.go           # PUBLIC API: GenerateCommit(ctx, opts) (Result, error)
│                                  # thin wrapper over internal/generate (for library use)
├── providers/                     # shipped reference manifests (TOML), human-readable
│   ├── pi.toml
│   ├── claude.toml
│   ├── agy.toml                   # Antigravity CLI (experimental; §12.5.1)
│   ├── opencode.toml
│   ├── codex.toml
│   └── cursor.toml
├── spec/                          # product + technical spec (split; SPEC.md is the entry point)
│   ├── SPEC.md                    # this document (imports the rest so reading tools see one spec)
│   ├── 01-product.md … 07-reference.md
│   └── FUTURE_SPEC.md             # deferred + rejected ideas, with rationale (not linked into the spec)
├── docs/                          # user-facing docs, derived from the spec + the shipped binary
├── .goreleaser.yaml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 14.1 The public library surface (`pkg/stagecoach`)

Intentionally tiny. The point is not to be a rich library; it is to let an integrator (a git GUI, a pre-commit hook, a CI step) call the core without reimplementing it.

```go
package stagecoach

type Options struct {
    Provider    string  // manifest name; "" → resolved default
    Model       string  // "" → manifest default_model
    SystemExtra string  // appended to the built system prompt
    DryRun      bool    // if true, return the message without committing
    Timeout     time.Duration
}

type Result struct {
    CommitSHA string    // empty if DryRun or not committed
    Subject   string
    Message   string    // full message (subject [+ body])
    Provider  string    // resolved provider name
    Model     string    // resolved model
}

// GenerateCommit generates and (unless DryRun) creates a commit from the
// currently-staged index. It does NOT decide what to stage. The caller
// stages first (or uses AutoStageAll in the CLI layer). Used for the
// single-commit path and as the per-concept primitive inside Decompose.
func GenerateCommit(ctx context.Context, opts Options) (Result, error)

// DecomposeOptions configures the multi-commit pipeline (§13.6).
type DecomposeOptions struct {
    Options                  // embedded: Provider/Model/DryRun/Timeout apply to the MESSAGE role
    Count          int       // 0 => auto-decompose (planner decides); >0 => force exactly Count commits
    Single         bool      // true => bypass planner, force one CommitStaged (--single)
    MaxCommits     int       // safety cap (default 12); refuses more unless Count forces it
    Planner        RoleModel // planner role provider/model (zero => global default)
    Stager         RoleModel // stager role provider/model (zero => global default)
    Arbiter        RoleModel // arbiter role provider/model (zero => global default)
}

type RoleModel struct { Provider, Model string }

// DecomposeResult is the outcome of Decompose: the ordered commits created this run.
type DecomposeResult struct {
    Commits []Result // one per concept that produced a commit (empty concepts skipped)
    Amended int      // number of those commits the arbiter folded leftovers into
    Provider string   // resolved MESSAGE provider (for display)
}

// Decompose turns a dirty, un-staged working tree into N logically-coherent
// commits (§13.6). It activates the planner→stager→message→arbiter pipeline;
// it is a NO-OP (delegates to GenerateCommit) when Single is true or Count==1.
// Caller must ensure nothing is staged (the CLI gates on HasStagedChanges).
func Decompose(ctx context.Context, opts DecomposeOptions) (DecomposeResult, error)
```

The CLI's `main.go` is essentially: parse flags → decide path (`GenerateCommit` if something staged or `--single`; else `Decompose`) → print result. The single-commit path stays a thin shell over `GenerateCommit`; the multi-commit path composes `GenerateCommit`'s primitives per concept (§13.6).

---

