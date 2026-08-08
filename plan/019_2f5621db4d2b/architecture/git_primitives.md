# Git Primitives & Concurrency Model (for the file-disjoint fast-path)

Verified by direct read of `internal/git/git.go`. No files modified.

## The per-path Add primitive the fast-path needs — ALREADY EXISTS

- **`Add(ctx context.Context, paths []string) error`**
  - Interface: `git.go:158` ✅ (matches PRD)
  - Impl: `git.go:1339` ⚠️ (PRD said `:1332`; line 1332 is the doc-comment start, the `func` decl is at 1339 — trivial citation drift)
  - Docstring (verbatim intent): "stages the given paths (modifications, additions/untracked, AND
    deletions) into the index via `git add -- <paths...>`. MUTATES THE INDEX (writes .git/index),
    touches NO ref. Path-specific companion to AddAll. Empty paths ⇒ no-op nil."
  - Currently consumed ONLY by the arbiter's chain rebuild; the fast-path is its **second call site** — call `deps.Git.Add(ctx, concept.Files)` directly, NO wrapper.

- **`AddAll(ctx) error`** — `git.go:148`/`:1321` — whole-tree `git add -A`. NOT what the fast-path wants (would collapse the chain).

## Freeze / write-tree primitives

- **`WriteTree(ctx) (sha, err)`** — interface `git.go:103`, impl `git.go:651` — materializes the LIVE
  index into a tree object; read-only w.r.t. refs/index-content. This is what `freezeSnapshot` wraps.
- **`WriteTreeFrom(ctx, indexFile) (sha, err)`** — interface `git.go:112`, impl `git.go:673` — scoped
  throwaway index via `GIT_INDEX_FILE`; **does NOT touch `.git/index`**. The concurrent-safe variant
  (the codebase's escape hatch for index-free work).
- **`FreezeWorkingTree(ctx, baseTree) (tStart, err)`** — impl `git.go:1824` — orchestration:
  `AddAll` → `WriteTree` (=T_start) → `ReadTree(baseTree)` (reset index to clean base). After it
  returns, **index == baseTree (clean)**.

## CAS publish primitives

- **`CommitTree(ctx, tree, parents []string, msg) (sha, err)`** — interface `git.go:116`, impl `git.go:707` —
  message via stdin `-F -`; `parents==nil` ⇒ root commit; builds a DANGLING object (moves NO ref).
  No `GIT_AUTHOR_*`/`GIT_COMMITTER_*` env set (FR-39a).
- **`UpdateRefCAS(ctx, ref, newSHA, expectedOld) error`** — interface `git.go:121`, impl `git.go:757` —
  3-arg CAS; mismatch ⇒ `ErrCASFailed` (`git.go:733`, `errors.Is`). The SOLE point stagecoach mutates
  a ref. Never the 2-arg force form. Root commit ⇒ `expectedOld` = all-zeros hash.
- **`DiffTreeNames(ctx, treeA, treeB) (paths, err)`** — interface `git.go:319`, impl `git.go:1847` —
  `git diff-tree -r --name-only --no-commit-id`; SORTED+DEDUPED path set; identical trees ⇒ `(nil,nil)`.
  The FR-M1c subset baseline (`runLoop` computes `DiffTreeNames(baseTree, tStart)` once).

## ⚠️ CRITICAL CONCURRENCY FINDING — there is NO in-process index lock

- `gitRunner` struct (`git.go:483-485`) has ONE field (`workDir string`). **No mutex.** `grep -rn
  "Mutex|sync\." internal/git/*.go` → zero matches. Each method spawns a stateless `exec` process.
- **`.git/index` is shared DISK state.** git takes a `.git/index.lock` per command (atomicity
  per-command), but the Go layer provides NO ordering/serialization across commands.
- **Implication for the fast-path (stage serially → generate messages concurrently):**
  - The **staging sweep** (per-concept `Add` + `WriteTree` live) MUTATES `.git/index` → MUST run
    **strictly serially** (the caller serializes; this matches runLoop's serial staging).
  - The **message-generation phase** MUST NOT touch the live `.git/index`. As long as each goroutine
    reasons only over **tree-to-tree diffs** (`generateMessage` does `diff(tree[i-1], tree[i])`,
    which is read-only tree reads via `DiffTreeNames`/`TreeDiff`) — no `Add`/`ReadTree`/`WriteTree`(live)
    during message gen — concurrency is safe. `generateMessage`'s existing contract already uses
    tree-to-tree diffs (§13.6.3 invariant 2), so it does NOT touch the live index. ✅
  - **publishCommit** runs `CommitTree` (dangling object, no index) + `UpdateRefCAS` (no index) — safe,
    but the `update-ref`s MUST serialize in CAS order (FR-M7). The publish loop is the serialization point.
- **Safe-to-concurrentize set (read-only w.r.t. index):** `DiffTreeNames`, `TreeDiff`, `RevParseTree`,
  `DiffTree`, `CommitTree`, `UpdateRefCAS`. **Index-mutating (keep serial):** `Add`, `AddAll`, `ReadTree`,
  `WriteTree`(live), `FreezeWorkingTree`, `OverlayTreePaths`.
- **Net:** the fast-path's design (serial staging sweep freezes all trees up front → concurrent
  message gen over frozen trees → serial CAS publish) is SOUND and needs NO new mutex, PROVIDED the
  implementer keeps the staging sweep strictly serial and confines message goroutines to tree reads.
  There is no existing concurrent-index test precedent — the new regression suite is the net.

## No concurrency cap in spec

FR-M14 says "concurrently" with no cap. The only numeric bound on N is `max_commits` (default 12,
FR-M4, `spec/01-product.md:354`). Launch all N (≤12 typical) unless a real provider rate-limits ≥12
one-shot calls; if a semaphore is added, default it ≥12 and DO NOT introduce a new config key.