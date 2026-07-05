# Research Findings — P3.M1.T1.S2 (docs/how-it-works.md holistic decompose-section reconciliation)

Scope: a Mode B docs-only edit to ONE file, `docs/how-it-works.md`. Reconcile the decompose section AS A
WHOLE so the two Mode A edits (P1.M1.T2.S2 arbiter freeze narrative; P2.M1.T2S2 planner mode-conditional/
files/soft-target) cohere, no stale pre-freeze references remain, and the Safety bullets reflect FR-M1d
(the arbiter as the third freeze surface). Then VERIFY `docs/cli.md` + `docs/configuration.md` need NO
changes and leave them untouched. Zero code, zero new files.

## §1 — What the two Mode A edits already did (so this task does NOT duplicate them)

**P1.M1.T2.S2 — "Sync arbiter doc narrative to freeze-safe gate" (commit 26fcf0b).** Touched THREE spots
in `docs/how-it-works.md` (its PRP line 188: "3 edits"):
1. Rewrote the "Arbiter leftover reconciliation" key-design-point paragraph (now line 75): the gate is
   the frozen `diff-names(tipTree, T_start)`, the diff shown is `TreeDiff(tipTree, T_start)`, staging is
   tree-only from T_start, the index is synced to T_start.
2. Refined the "Start-of-run freeze (T_start)" key-design-point PARAGRAPH (now line 67): "every stager,
   the arbiter (its gate, its diff, and its leftover staging), and the one-file/single shortcuts draw
   strictly from T_start."
3. Refined the pipeline-diagram GATE label (now line 90 of the code fence): "git status clean?" →
   "frozen leftover empty?".
   Its PRP explicitly says (line 90): **"No edits to: the four-roles table, the format-modes paragraph,
   the Safety bullets, the lock paragraph, …"** — i.e. it deliberately LEFT the Safety bullets and the
   rest of the diagram for later.

**P2.M1.T2.S2 — "Document planner files and mode-conditional rules" (commit 64e9016).** Touched TWO spots:
1. The four-roles table planner row (line 59): the JSON contract gained `files` —
   `{count, single, commits:[{title,description,files}], message?}`.
2. Added the "Mode-conditional planner rules" key-design-point paragraph (line 73): mode-conditional
   Rules block, soft target `max_commits / 2` (default 6), hard cap `max_commits` (default 12), per-file
   `files` lists, deterministic coverage check.
   It did NOT touch the diagram, the Safety bullets, or the arbiter paragraph.

**Net:** the two edits are mutually consistent (no conflict) and each is internally correct. The gaps
this reconciliation closes are the spots BOTH edits deliberately skipped: the Safety bullets and the
two pre-freeze remnants in the diagram.

## §2 — The THREE reconciliation edits (the precise gaps)

### Edit 1 — the pipeline diagram's single-shortcut arrow (STALE pre-freeze wording)
Diagram code-fence line 5 (the branch right after the planner returns `single?`):
```
                  │ single? ──yes──▶ git add -A → CommitStaged (one call) → done
```
This is STALE on two counts. The `single? ──yes` branch is the FR-M11 single-call shortcut (planner
judged one commit + supplied a message). Per FR-M11 (as updated by the freeze work): "stage T_start
(FR-M1b) → snapshot → commit-tree → update-ref. No separate message-agent call." And FR-M10's null path:
"commit T_start directly." So the shortcut commits the frozen T_start with the PLANNER'S message — it
neither runs a live `git add -A` nor calls `CommitStaged` (the v1 message-regenerating function). The
implemented `runSingleShortcut` does `treePrime := tStart` → `publishCommit`. Replace with:
```
                  │ single? ──yes──▶ commit T_start (planner's message) → done
```
This matches the "commit T_start directly" language FR-M10/FR-M11 use elsewhere and removes the last
`git add -A`-as-a-commit-path reference in the diagram (the `git add -A` in the "Freeze enforcement"
paragraph at line 69 is DIFFERENT — it describes a stager MISBEHAVIOR that is aborted; that one stays).

### Edit 2 — the pipeline diagram's planner-input label (imprecise post-freeze)
Diagram code-fence line 1:
```
            ┌────────────┐   full working-tree diff (binary placeholders)
```
Post-freeze, the planner receives `TreeDiff(baseTree, T_start)` — the frozen tree-to-tree diff, NOT a
fresh read of the live tree. The freeze narrative (line 67) already says "The planner partitions T_start's
diff (never a fresh re-read of the live tree)." The diagram's "full working-tree diff" undercuts that (a
reader skimming the diagram could think it's a live read). For coherence with the narrative, replace with:
```
            ┌────────────┐   T_start diff (binary placeholders)
```
(The `(binary placeholders)` clause is preserved — FR3c binary filtering applies identically.)

### Edit 3 — the Safety "Start-of-run freeze" bullet (must name the arbiter per FR-M1d)
Safety section, the 4th bullet (line 128):
```
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. Each staging step is verified as a content-subset of T_start.
```
This names ONLY the stager ("Each staging step is verified…"). The work item is explicit: "the 'Safety'
bullets reflect that the arbiter now derives strictly from T_start" and "'Start-of-run freeze' bullet
should name the arbiter as a freeze surface, not just stager." FR-M1d makes the arbiter the THIRD freeze
surface (gate + diff + trees all from T_start + tipTree, never live). The two key-design-point paragraphs
(lines 67 + 75) already state this; the Safety bullet — the section readers scan for the guarantee — does
not. Replace with a version naming BOTH freeze surfaces:
```
- **Start-of-run freeze** — T_start captures the full working-tree change set at decompose activation; concurrent edits never enter any commit. The stager is verified as a content-subset of T_start after each staging step (FR-M1c), and the arbiter — the third freeze surface — derives its gate, its diff, and every tree it commits strictly from T_start and tipTree, never a live re-read (FR-M1d).
```
This makes the Safety bullet consistent with the narrative paragraphs (line 67 "the arbiter (its gate,
its diff, and its leftover staging)"; line 75 "The live working tree is never consulted for the gate")
and with FR-M1d.

## §3 — Spots CONSIDERED and LEFT UNCHANGED (no edit needed — prevents over-editing)

- **The "Freeze enforcement" paragraph (line 69)** — "a stager that ran a bare `git add -A` … is a hard
  abort." This `git add -A` reference is CORRECT (it describes a stager MISBEHAVIOR that the enforcement
  catches). It is NOT a commit-path reference. Leave it. (Edit 1 removes the only STALE `git add -A` — the
  one in the diagram's commit path.)
- **The "No index resets" Safety bullet (line 127)** — "the index accumulates across concepts. After the
  final commit, HEAD.tree == tree[N-1] == full accumulated index." This describes the LOOP's accumulate-
  never-reset invariant (a safety property that makes overlapped staging safe). The arbiter's post-loop
  index-sync to T_start is a SEPARATE step, covered in the arbiter paragraph (line 75: "then syncs the
  index to T_start"). After the arbiter, HEAD.tree == T_start and the index == T_start, so "the index is
  clean relative to HEAD" still holds. No contradiction; no edit.
- **The four-roles table (lines 59-62)** — already updated by P2.M1.T2.S2 (planner JSON has `files`).
  Coherent. No edit.
- **The arbiter box in the diagram (line 97)** — "commits made + leftover diff". The diagram's gate box
  (line 93) already says "frozen leftover empty?" (fixed by P1.M1.T2.S2). The arbiter's "leftover diff"
  input is the frozen `TreeDiff(tipTree, T_start)`; the narrative (line 75) carries the precision. No edit.
- **The "Trigger", "Overlapped staging", "Stage-while-editing", "Frozen tree snapshots", "Tree-to-tree
  diffs", "Serialized publication", "One-file short-circuit" paragraphs** — all accurate, unaffected by
  the freeze-parity / planner-files delta. No edit.
- **The rest of the file** (single-commit sections, diff-capture pipeline, run lock, rescue protocol,
  prompt engineering) — out of scope (not the decompose section). No edit.

## §4 — The cli.md / configuration.md verification (the task's "verify … untouched" step)

The work item: "VERIFY docs/cli.md and docs/configuration.md require NO changes (FR-M4 soft target is
derived from existing max_commits; FR-M3 files is automatic — no new flags/keys) and leave them untouched."

Confirmed by grep:
- **`docs/cli.md`** already documents `--commits` (line 33), `--single`/`--no-decompose` (34-35),
  `--max-commits` (line 36: "Safety cap on auto-decompose commit count (also `[generation].max_commits`
  in config)"), and the per-role `--planner-*`/`--arbiter-*` flags (44-56). The FR-M4 soft target is
  `max_commits / 2` — DERIVED from the existing `max_commits` knob, NOT a new flag. The FR-M3 `files`
  field is automatic planner output — NOT a user-facing flag. So cli.md needs NO new flag rows.
- **`docs/configuration.md`** already documents `[generation].max_commits` (line 217: "Max commits …
  Safety cap on auto-decompose count") and the `[role.planner]`/`[role.arbiter]` blocks (lines 90, 99).
  No new config key for the soft target (derived) or `files` (automatic). The git-config-no-per-role-keys
  note (line 209) is still accurate.
- Neither file contains `soft target`, `soft-target`, `files field`, or `per-file` (grep returned none).
  There is NOTHING to add. **Leave both files byte-unchanged.** (`git status --short` must show only
  `M docs/how-it-works.md`.)

## §5 — Scope fence + parallel-work safety

- **EDIT ONLY `docs/how-it-works.md`.** The decompose section is ~lines 47-130. The 3 edits are: diagram
  line 1 (planner input label), diagram line 5 (single-shortcut arrow), Safety bullet line 128 (Start-of-
  run freeze). Nothing else in the file.
- **DO NOT EDIT** `docs/cli.md`, `docs/configuration.md`, `docs/providers.md`, `README.md` (the sibling
  P3.M1.T1.S1 owns README), any `.go` file, go.mod/go.sum, PRD.md, tasks.json, prd_snapshot.md.
- **Parallel-work safety:** the sibling P3.M1.T1.S1 (README.md) does NOT touch how-it-works.md (its PRP
  explicitly lists how-it-works.md as READ-ONLY, owned by THIS task). The implementing subtasks
  (P1.M1.*/P2.M1.*) are COMPLETE; their code is frozen. No merge conflict.
- **Docs-only:** zero Go code change. `go build`/`go test` are unaffected (a no-op confirmation only).

## §6 — Diagram ASCII-alignment gotcha (the one mechanical care)

The pipeline diagram is a ```` ```text ```` code fence with box-drawing characters (┌─┐│└┘▼◀═) and
carefully aligned columns. The two diagram edits (§2 Edits 1 + 2) change ONLY the LABEL TEXT to the right
of / below the boxes — they do NOT move any box-drawing character or change column positions:
- Edit 1: the label is on the SAME line as `┌────────────┐`, to the right of it (after 3 spaces). The
  replacement "T_start diff (binary placeholders)" is SHORTER than "full working-tree diff (binary
  placeholders)" — alignment is preserved (shorter text just leaves more trailing space; the box is
  untouched).
- Edit 2: the arrow line `│ single? ──yes──▶ …` — the replacement text follows the `▶`; the `│` and the
  arrow glyphs are preserved. "commit T_start (planner's message) → done" fits the line width.
P1.M1.T2.S2's PRP (line 208) flagged the same care ("Preserve alignment when changing the gate label");
the same discipline applies. Eyeball the rendered diagram after the edit.

## §7 — Why this is low-risk

A 3-edit markdown reconciliation to one file, closing exactly the gaps the two Mode A edits deliberately
left (their PRPs explicitly excluded the Safety bullets + the non-gate diagram lines). The current
narrative is already correct (the Mode A edits are consistent); this task only (a) removes two pre-freeze
diagram remnants and (b) lifts the arbiter-as-freeze-surface into the Safety bullet. The cli.md/
configuration.md verification is a grep-confirmed no-op. High one-pass confidence.
