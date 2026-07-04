# Research Note — P1.M1.T1.S3 (docs/how-it-works.md:155 "No-op fast path" qualification)

## Scope of this subtask

Doc-only, single-paragraph rewrite of the `**No-op fast path.**` paragraph at
`docs/how-it-works.md:155`, under the `### Per-repo run lock (FR52)` heading (line 144).
Third sibling of the three doc fixes for Issue 1 (S1 = README:330, S2 = docs/cli.md:379,
S3 = this). All three carry one consistent per-path story in their own voices.

## Current text (verbatim, line 155)

```
**No-op fast path.** When a lock is held, the holder publishes its frozen tree SHA via `SetSnapshot()`. A contender with nothing new staged since that snapshot can exit 0 immediately (no-op fast path).
```

Problem: "can exit 0 immediately" is stated **unconditionally** — false on the decompose path.

## Surrounding subsection style (the contract to match)

The `### Per-repo run lock (FR52)` section is a stack of **bold-lead-in, single-logical-line,
concise** paragraphs. Verified siblings (each one markdown line):

- L151: `**Per-host limit.** …` (2 sentences)
- L153: `**Never-in-repo location.** …` (2 sentences)
- L155: `**No-op fast path.** …` (← TARGET; currently 2 sentences)
- L157: `**Auto-release.** …` (3 sentences)

Implication: the rewrite must stay ONE logical markdown line, start with
`**No-op fast path.**`, and stay concise (≈3 sentences, like Auto-release). The contract's
suggested phrasing already meets this bar — use it.

## markdownlint config (`.markdownlint.json`)

```json
{ "default": true, "MD013": false, "MD033": false, "MD060": false }
```

- **MD013 (line-length) is DISABLED** → long single-line paragraphs are allowed/expected.
  The existing L151/L153/L155/L157 are each one long line. Do NOT hard-wrap the rewrite.

## Root cause (why the unconditional claim is false) — authoritative table

From `architecture/system_context.md` "Snapshot publish axis mismatch":

| Path | What holder publishes as `snapshot=` | What contender computes via `WriteTree()` |
|------|--------------------------------------|-------------------------------------------|
| Single-commit (staged) | `WriteTree()` result (index tree) | `WriteTree()` result (index tree) — **same axis → can match** |
| Decompose (nothing staged) | `T_start` (working-tree tree, from `FreezeWorkingTree` step 2) | `baseTree` = `HEAD^{tree}` (index reset to baseTree by `FreezeWorkingTree` step 3) — **different axis → NEVER matches** |

Decompose activates iff nothing is staged (FR-M1) → contender's index == HEAD == baseTree, but
holder published working-tree `T_start` ≠ baseTree → `contenderTree == snap` is **always false**
on the decompose path → contender exits Busy(5), never 0.

Code anchors (do NOT edit — context only): `internal/decompose/decompose.go:169`
(`lock.SetSnapshot(tStart)`), `internal/git/git.go:1340-1361` (`FreezeWorkingTree`),
`internal/cmd/default_action.go` (`handleLockContention`, `contenderTree = g.WriteTree(ctx)`).

## Authoritative target phrasing (supplied by the item contract)

> **No-op fast path.** On the single-commit path (changes staged), the holder publishes its
> frozen index-tree SHA via `SetSnapshot()`, and a contender whose staged snapshot is
> byte-identical to it exits 0 immediately. On the decompose path (nothing staged, dirty
> working tree), an accidental double-run exits **5 (Busy)** instead — the holder publishes a
> working-tree snapshot (`T_start`) that a lock-free contender cannot reproduce from the index,
> so it conservatively refuses.

This phrasing:
1. Keeps the `**No-op fast path.**` lead-in (subsection identity).
2. Scopes exit 0 to the single-commit (staged) path.
3. Adds the decompose-path → Busy(5) note with the working-tree-vs-index snapshot-axis rationale.
4. Is concise (3 sentences, one paragraph) — matches Auto-release's density.
5. Stays ONE logical markdown line (markdownlint MD013 disabled).

## Explicit OUT OF SCOPE (must NOT touch)

1. The **"Failure modes and exit codes" table** at `docs/how-it-works.md:163-173` — it does NOT
   list code 5/Busy at all. The item contract explicitly states this is a **pre-existing gap
   unrelated to this bug**; do NOT expand scope to add a Busy row. Leave the table untouched.
2. The other bold-lead-in subsections (Per-host limit L151, Never-in-repo location L153,
   Auto-release L157) and the heading (L144).
3. The "The rescue (3)…" note (L175) and the "See [cli.md]…" pointer (L177).
4. Any `.go` file / test file — **no code changes** (Option 1 doc fix; Option 2 code change is
   explicitly rejected by `issue_analysis.md`).
5. `README.md` (S1), `docs/cli.md` (S2) — sibling subtasks, same per-path semantics, their own voices.
6. `PRD.md`, `tasks.json`, `prd_snapshot.md`, anything under `plan/`.

## Cross-doc coherence (informational — S1/S2 are the coherence reference)

S1 (README:330) and S2 (docs/cli.md:379) carry the identical per-path semantics
(single-commit → 0 or 5; decompose → 5) in their own voices. S3 is the how-it-works.md
architecture-narrative voice. The "nothing to do — an in-progress run already covers your
staged changes" string is NOT present in how-it-works.md:155 (it lives in README/cli.md), so S3
need not reproduce it — just the per-path scoping + the working-tree-`T_start` rationale. The
final coherence sweep is P1.M3.T1 (Mode B).

## Validation summary

- Doc-only: `go build ./...` + `go test ./...` must stay green (sanity, not prose validation).
- `git diff --stat` → ONLY `docs/how-it-works.md` (1 file).
- Prose check: exit 0 scoped to single-commit path; decompose→Busy(5) with `T_start` rationale;
  `**No-op fast path.**` lead-in kept; ONE logical line; exit-code table (163-173) untouched;
  other subsections untouched.
