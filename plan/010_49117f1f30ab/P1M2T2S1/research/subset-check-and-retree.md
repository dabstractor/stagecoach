# Research — P1.M2.T2.S1: Subset-check helper + re-tree-on-permitted-mutation (FR-V3)

> Scope: the FR-V3 freeze-enforcement backstop for scoped `pre-commit`. After the hook runs against a
> throwaway index primed from the snapshot tree (P1.M2.T1.S2's `ReadTreeInto`/`WriteTreeFrom`), stagehand
> must verify the hook introduced NO new paths (a formatter modifying an existing snapshot file is fine;
> a hook that stages a path not in the snapshot is a hard error — it would sweep concurrent work in).
> This task = the subset-check helper `enforceSubset` + its typed error, placed in a NEW `internal/hooks`
> package (policy, not a git primitive). The re-tree decision (use postTree when permitted) is the caller's
> trivial rule, documented here.
>
> **PREREQUISITE (parallel, assume LANDED): P1.M2.T1.S2.** It added `ReadTreeInto(ctx, tree, indexFile)`
> + `WriteTreeFrom(ctx, indexFile)` (scoped via `GIT_INDEX_FILE`). THIS task consumes `WriteTreeFrom`'s
> `postTree` output + the existing `DiffTreeNameStatus` (read-only, index-agnostic).

---

## 1. The logic — `'A'` is the definitive new-path signal (DiffTreeNameStatus has NO `-M`)

`git.Git.DiffTreeNameStatus(ctx, treeA, treeB)` (git.go:1920, on the interface at L371) runs:
```
git diff-tree --no-commit-id --name-status -r <treeA> <treeB>
```
**No `-M`, no `-C`.** So for `DiffTreeNameStatus(snapshotTree, postTree)`:
- **`A\tpath`** — path in postTree, NOT in snapshotTree (ADDED). ← the hard-error signal.
- **`M\tpath`** — in both, content differs (formatter reformatted a snapshot file). ← PERMITTED.
- **`D\tpath`** — in snapshotTree, not postTree (hook removed a staged file). ← PERMITTED.
- **`T\tpath`** — type-change (e.g. symlink↔file) of an existing path. ← PERMITTED (path was in snapshot).
- A rename by the hook (old→new) appears as `D\told` + `A\tnew` (no -M ⇒ no `R` line) ⇒ the `A\tnew`
  triggers the hard error. ✓ (Consistent with FR-V3: a rename stages a new path.)
- A copy would appear as `A\tnew` (no -C) ⇒ hard error. ✓

**THEREFORE: an `'A'` status in `DiffTreeNameStatus(snapshotTree, postTree)` is, BY DEFINITION, a path in
postTree NOT in snapshotTree → a subset violation.** No `ListTreePaths`/`ls-tree` primitive is needed; the
status letter IS the check. (Mathematically: postTree's path set ⊆ snapshotTree's path set iff the
snapshot→post diff has no `A` lines. `M`/`D`/`T` don't violate subset — they modify/remove existing paths.)

### Defensive note on `R`/`C`
`R`/`C` lines never appear today (no `-M`/`-C`). But the letter-check is trivially robust to a future
`-M` addition: a `R<score>\told\tnew` or `C<score>\tsrc\tdst` line also introduces a new path (the
destination). So flagging status letters `A`, `C`, `R` (first byte of the status field) — with the
offending path = the LAST tab-field — is correct now AND stays correct if `-M` is ever added. `M`/`D`/`T`
are permitted. (This is defensive; the contract's explicit trigger is `'A'`.)

### The offending-path parse
`--name-status` lines are tab-separated: `STATUS\tpath` for A/M/D/T (2 fields); `STATUS\tsrc\tdst` for R/C
(3 fields). The NEW path (the violation) is the LAST tab-field. So:
```go
fields := strings.Split(line, "\t")
status := fields[0]              // "A", "M", "D", "T", or "R100"/"C050" (defensive)
if len(status) > 0 && (status[0]=='A' || status[0]=='C' || status[0]=='R') {
    added = append(added, fields[len(fields)-1])  // the new/destination path
}
```

---

## 2. The FR-M1c twin to mirror — `verifyFreezeSubset` + `ErrFreezeViolation` (decompose/stager.go)

The decompose stager ALREADY has the content-axis freeze-enforcement twin (stager.go:60-180):
- **Sentinel**: `var ErrFreezeViolation = errors.New("decompose: freeze violation")`.
- **Wrapped with `%w`**: `fmt.Errorf("%w: concept %d staged paths not present in T_start: %s",
  ErrFreezeViolation, i, strings.Join(extra, ", "))` — so `errors.Is(err, ErrFreezeViolation)` works.
- Uses a `pathSet` helper (stager.go:199: `map[string]struct{}`) + `strings.Join(extra, ", ")`.
- NON-RESCUE (hard error): the violation is detected BEFORE the commit; there's no snapshot-then-CAS to
  rescue. Already-landed commits stand; the in-flight concept's staging remains.

**THIS task's hook twin** (the FR-V3 backstop) mirrors that shape EXACTLY, in `internal/hooks`:
- **Sentinel**: `var ErrHookSweptConcurrentWork = errors.New("hooks: pre-commit swept concurrent work")`
  (the hook analogue of `ErrFreezeViolation`; both are freeze-enforcement, content-axis, hard errors).
- **Wrapped**: `fmt.Errorf("%w: pre-commit staged a path not in the snapshot: %s — refusing to sweep
  concurrent work into the commit (FR-V3)", ErrHookSweptConcurrentWork, strings.Join(added, ", "))`.
- NON-RESCUE (FR-V7: a pre-commit that stages a non-snapshot path aborts the run; HEAD/index untouched).

The contract: "The hard error mirrors the stager's FR-M1c discipline — reuse the same error-reporting
shape." The shape = sentinel + `%w` wrap + name offending paths + `errors.Is`-able. ✓

---

## 3. Placement — NEW `internal/hooks` package (policy, not a git primitive)

The contract: "PREFER internal/hooks since it's policy, not a primitive." `internal/hooks` does NOT exist
yet (P1.M3.T1 "RunCommitHooks runner module (new internal/hooks)" creates it — but MY task creates the
PACKAGE first with the subset helper; P1.M3.T1 ADDS `runner.go` to it). This mirrors the decompose
precedent: `verifyFreezeSubset` lives in `internal/decompose` (the CONSUMER package), NOT in internal/git.
The subset-check is POLICY (what mutations are permitted), so it belongs with its consumer (hooks).

- **`internal/hooks/subset.go`** — `package hooks`. `enforceSubset(ctx, g git.Git, snapshotTree, postTree
  string) error` + `ErrHookSweptConcurrentWork`. Imports `internal/git` (the Git interface — for
  `DiffTreeNameStatus`) + stdlib (`context`, `errors`, `fmt`, `strings`). One-way dep (hooks → git); NO
  cycle (git does not import hooks).
- **`internal/hooks/subset_test.go`** — `package hooks` (white-box) OR `package hooks_test` (black-box via
  the real `git.New(repo)`). Tests build a real repo (initRepo helpers), snapshot a tree, mutate the index
  (permitted M / forbidden A), capture postTree via `WriteTreeFrom` (P1.M2.T1.S2 — exercise BOTH the
  scoped primitives AND the subset check end-to-end), and assert nil vs `ErrHookSweptConcurrentWork`.

**Why `g git.Git` (the interface), not `*gitRunner`:** `DiffTreeNameStatus` is ON the Git interface
(L371). Taking the interface makes `enforceSubset` testable via the public `git.New(repo)` constructor and
keeps `internal/hooks` decoupled from `*gitRunner` internals. (The runner P1.M3.T1 also takes `git.Git`.)

### The signature (final)
```go
// enforceSubset verifies the FR-V3 freeze backstop: postTree's path set must be a SUBSET of
// snapshotTree's — i.e. the hook introduced NO new paths. Returns nil if the subset holds (a formatter
// that modified/deleted existing snapshot paths is fine), or ErrHookSweptConcurrentWork naming the
// offending path(s) if the hook ADDED a path not in the snapshot (would sweep concurrent work in).
func enforceSubset(ctx context.Context, g git.Git, snapshotTree, postTree string) error
```

---

## 4. The re-tree decision (the caller's trivial rule — OUTPUT clause 4)

`enforceSubset` returns nil/error ONLY. The re-tree decision is the CALLER's (the runner P1.M3.T1), per
the contract's OUTPUT clause:
- `postTree == snapshotTree` → no mutation → commit uses `snapshotTree` (the original frozen tree).
- `postTree != snapshotTree` AND `enforceSubset(...) == nil` → permitted mutation → commit uses `postTree`
  (the hook-fixed tree; git-commit parity: the commit reflects the hook's output).
- `enforceSubset(...) != nil` → hard error → abort (FR-V7 rescue state; no update-ref ran).

This is a one-line decision at the call site (`tree := snapshotTree; if postTree != snapshotTree { tree =
postTree }`, gated by the subset check). It does NOT warrant a separate function (the runner owns the
`commit-tree` call); `enforceSubset` is the testable unit. The item title's "re-tree-on-permitted-mutation
logic" = this documented rule the caller applies. (If the implementer prefers a tiny
`resolvePostHookTree(snapshot, post) string` helper, that's fine — but it's `postTree != snapshot ?
postTree : snapshotTree`, too trivial to be the deliverable; enforceSubset is.)

---

## 5. The resolved design (open_questions.md §1 + external_deps.md §8)

**open_questions.md §1** (the resolved scoped-index mechanism) names THIS check explicitly:
> "subset check: ... Use `DiffTreeNameStatus` to enumerate what the hook changed; if any ADDED path is not
> in the snapshot's path set → hard error (FR-V3). (A formatter modifying an existing snapshot path is
> permitted → re-tree.)"

And prescribes the two-layer split: git layer (scoped variants — P1.M2.T1.S2) + runner layer (internal/hooks
— P1.M3.T1). `enforceSubset` is the subset-check portion of the runner layer, factored out as a testable
helper (the runner's `RunPreCommit` will CALL it).

**external_deps.md §8** (the faithful sequence):
```
tmp := mktemp(); defer os.Remove(tmp)
GIT_INDEX_FILE=<abs tmp>
git read-tree <frozenTree>          # ReadTreeInto (P1.M2.T1.S2)
run hook "pre-commit"
  └─ non-zero → rescue (FR-V7); hook staging a non-T_start path → hard error (FR-V3 subset) ← THIS TASK
newTree := git write-tree           # WriteTreeFrom (P1.M2.T1.S2)
```
`enforceSubset(ctx, g, frozenTree, newTree)` is the "FR-V3 subset" step.

---

## 6. Tests — exercise the FULL scoped sequence (ReadTreeInto → mutate → WriteTreeFrom → enforceSubset)

The subset_test.go tests build a real repo, create a snapshot tree, then simulate hook mutations via the
scoped primitives (P1.M2.T1.S2) and assert enforceSubset's verdict. Mirror the git test helpers
(`initRepo`/`writeFile`/`stageFile`/`makeEmptyCommit`/`writeTreeOf`/`execGit` — in internal/git's
`*_test.go`, BUT those are `package git` internal helpers NOT importable from `package hooks`). So
`internal/hooks` tests re-create minimal helpers (or use `exec.Command("git", ...)`) — see the sketch.

### Cases (the keystone matrix)
| Scenario | Setup | enforceSubset verdict |
|---|---|---|
| **No mutation** (postTree == snapshotTree) | prime tmp from tree; run NO hook; WriteTreeFrom → postTree==tree | nil (and caller uses snapshotTree) |
| **Permitted M** (formatter reformats a snapshot file) | snapshot has a.go; prime tmp; `GIT_INDEX_FILE=<tmp> git update-index --add` a MODIFIED a.go (content change); WriteTreeFrom → postTree | nil (M status; caller uses postTree) |
| **Permitted D** (hook removes a staged file) | snapshot has a.go+b.go; prime tmp; scoped `git rm --cached a.go` (or update-index --remove); WriteTreeFrom → postTree | nil (D status; subset holds) |
| **Forbidden A — new file** (formatter adds a NEW file) | snapshot has a.go; prime tmp; scoped `git update-index --add` a NEW c.go; WriteTreeFrom → postTree | **ErrHookSweptConcurrentWork** naming c.go |
| **Forbidden A — rename** (hook renames a.go→a2.go) | snapshot has a.go; prime tmp; scoped add a2.go + remove a.go; WriteTreeFrom → postTree | **ErrHookSweptConcurrentWork** (rename shows as A a2.go + D a.go under no-`-M`) |
| **Multiple violations** | two new files added | error names BOTH (comma-joined) |
| **DiffTreeNameStatus git failure** | bad tree SHA | wrapped non-sentinel error (NOT ErrHookSweptConcurrentWork) |

The scoped mutation is applied via an INDEPENDENT oracle (`exec.Command("git", "-C", repo, "update-index",
"--add", file)` with `cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmp)`) — mirroring P1.M2.T1.S2's
independent-oracle test discipline (test the subset check, not the scoped primitives — those have their own
tests). Then `WriteTreeFrom(ctx, tmp)` captures postTree; `enforceSubset(ctx, g, snapshotTree, postTree)`
returns the verdict.

---

## 7. Scope fences (NOT this task)

- **NOT the scoped primitives** (ReadTreeInto/WriteTreeFrom — P1.M2.T1.S2, parallel). THIS task CONSUMES
  WriteTreeFrom's postTree; it does NOT implement or modify the scoped variants.
- **NOT the hook runner** (RunPreCommit / the sequence / hook discovery / env / timeout / `--no-verify` —
  P1.M3.T1). THIS task is the subset-check helper the runner CALLS. Do not implement the runner.
- **NOT DiffTreeNames/DiffTreeNameStatus** (read-only git primitives — UNCHANGED; consumed via the interface).
- **NOT a ListTreePaths primitive** — NOT needed ('A' status is the check; §1).
- **NOT the commit-tree / update-ref** (the plumbing commit — generate/decompose own it; P1.M3.T2/T3 wire
  the runner in). The re-tree DECISION is documented (§4) but the commit is the caller's.
- **NOT config/cli/docs** (NoVerify/HookTimeout are P1.M1; the FR-V3 doc is P1.M4 via M3.T2.S1's Mode A).
- **NOT the decompose verifyFreezeSubset** (the FR-M1c twin — unchanged; THIS is the hook analogue).

---

## 8. Validation commands

```bash
gofmt -w internal/hooks/subset.go internal/hooks/subset_test.go
go vet ./internal/hooks/        # catches a bad interface call / unused import.
go build ./...                  # the new package compiles; no caller breaks (additive).
go test -race ./internal/hooks/ -v   # the keystone matrix (no-mutation/permitted-M/permitted-D/forbidden-A/forbidden-rename/multi/git-failure).
go test -race ./...             # no regression (the new package is additive; nothing imports it yet except its own tests).
git diff --exit-code go.mod go.sum
# Confirm DiffTreeNameStatus is UNCHANGED (consumed, not modified):
git diff --exit-code internal/git/git.go 2>/dev/null || echo "(git.go may have P1.M2.T1.S2's scoped variants — confirm only those, NOT DiffTreeNameStatus)"
# Confirm the error is errors.Is-able (the FR-M1c twin shape):
grep -n 'ErrHookSweptConcurrentWork\|%w' internal/hooks/subset.go
```
