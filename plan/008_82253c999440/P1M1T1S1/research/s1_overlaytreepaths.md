# S1 Implementation Notes — OverlayTreePaths git primitive

> Scope: P1.M1.T1.S1. Add `OverlayTreePaths(ctx, baseTree, sourceTree, paths)` to the Git interface +
> gitRunner impl + a NEW internal/git/overlaytree_test.go. Leaf primitive for the FR-M1d arbiter mid-chain
> rebuild (consumed by P1.M1.T2.S1). Verified against live source + arch arbiter_freeze_parity.md §3 (2026-07-04).

## 1. Confirmed: OverlayTreePaths does NOT exist; EmptyTreeSHA + siblings PRESENT

- `grep OverlayTreePaths internal/git` → nothing (arch §1.4). `ls-tree`/`update-index`/`--cacheinfo`/`--force-remove`
  → NOT used anywhere. This is a genuinely new primitive.
- `EmptyTreeSHA` const at git.go:644 (`4b825dc642cb6eb9a060e54bf8d69288fbee4904`).
- The structural template is `FreezeWorkingTree` (git.go:1554) — thin orchestration of AddAll+WriteTree+ReadTree,
  index/object-store-only mutation, NO ref, NO working-tree touch. OverlayTreePaths mirrors this discipline.

## 2. Placement (arch §3.4)

- **Interface doc-block**: add immediately AFTER `DiffTreeNames` (the interface block around git.go:283-289) —
  keeps the freeze/tree-primitive cluster (FreezeWorkingTree, DiffTreeNames, OverlayTreePaths) together.
- **gitRunner impl**: near `FreezeWorkingTree` (git.go ~1554) + `DiffTreeNames` (git.go ~1572). Place
  `OverlayTreePaths` + the unexported `parseLsTree` helper right after `DiffTreeNames`.
- **Tests**: NEW file `internal/git/overlaytree_test.go` (package git, same temp-repo helper style).

## 3. The verbatim implementation (arch §3.2-§3.3)

```go
func (g *gitRunner) OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (string, error) {
	if len(paths) == 0 {
		return baseTree, nil // early no-op — avoids a pointless write-tree
	}
	// 1. read-tree baseTree → index = baseTree
	if _, stderr, code, err := g.run(ctx, g.workDir, "read-tree", baseTree); err != nil {
		return "", err
	} else if code != 0 {
		return "", fmt.Errorf("git read-tree (overlay base): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	// 2. ONE ls-tree sourceTree for the requested paths
	lsArgs := append([]string{"ls-tree", "-r", "--full-tree", sourceTree, "--"}, paths...)
	lsOut, stderr, code, err := g.run(ctx, g.workDir, lsArgs...)
	if err != nil {
		return "", err
	} else if code != 0 {
		return "", fmt.Errorf("git ls-tree (overlay source): failed (exit %d): %s", code, strings.TrimSpace(stderr))
	}
	blobs := parseLsTree(lsOut) // map[path]→{mode, blob}
	// 3. per-path update-index (cacheinfo if present in source, force-remove if absent = deletion-overlay)
	for _, p := range paths {
		if ent, ok := blobs[p]; ok {
			if _, stderr, code, err := g.run(ctx, g.workDir, "update-index", "--cacheinfo",
				fmt.Sprintf("%s,%s,%s", ent.mode, ent.blob, p)); err != nil {
				return "", err
			} else if code != 0 {
				return "", fmt.Errorf("git update-index --cacheinfo %s: failed (exit %d): %s", p, code, strings.TrimSpace(stderr))
			}
		} else {
			if _, stderr, code, err := g.run(ctx, g.workDir, "update-index", "--force-remove", p); err != nil {
				return "", err
			} else if code != 0 {
				return "", fmt.Errorf("git update-index --force-remove %s: failed (exit %d): %s", p, code, strings.TrimSpace(stderr))
			}
		}
	}
	// 4. write-tree → new tree SHA
	return g.WriteTree(ctx)
}
```
**parseLsTree** (unexported helper, same file):
```go
type lsTreeEntry struct{ mode, blob string }
func parseLsTree(out string) map[string]lsTreeEntry {
	m := map[string]lsTreeEntry{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" { continue }
		// "<mode> <type> <blob>\t<path>" — split on \t for path, split left on spaces for mode/type/blob
		tab := strings.IndexByte(line, '\t')
		if tab < 0 { continue }
		path := line[tab+1:]
		leftFields := strings.Fields(line[:tab]) // [mode, type, blob]
		if len(leftFields) < 3 { continue }
		m[path] = lsTreeEntry{mode: leftFields[0], blob: leftFields[2]} // [1]=type ("blob") ignored
	}
	return m
}
```
**Exit-code convention**: every sub-command uses the MUTATION form (`code != 0 ⇒ error`, NO 128-as-non-error
special case) — identical to ReadTree/WriteTree/Add. `ls-tree` is read-only but follows the same convention.

## 4. The interface doc-block (arch §3.1, verbatim)

Add after `DiffTreeNames` (git.go ~289):
```go
	// OverlayTreePaths returns a NEW tree equal to baseTree with each path in paths overwritten by its
	// state in sourceTree (PRD §13.6.5 / FR-M10 mid-chain rebuild). For each path in paths:
	//   - present in sourceTree → overwritten with sourceTree's (mode, blob) for that path
	//     (git update-index --cacheinfo <mode>,<blob>,<path>).
	//   - absent in sourceTree  → removed from the result (deletion-overlay)
	//     (git update-index --force-remove <path>).
	// The (mode, blob) pairs come from ONE `git ls-tree -r --full-tree <sourceTree> -- <paths...>`.
	// Implementation: read-tree baseTree (index = baseTree) → per-path update-index → write-tree.
	// EMPTY paths ⇒ return baseTree verbatim (no-op early return, NO index mutation).
	// It mutates ONLY .git/index and the object store (same discipline as FreezeWorkingTree/ReadTree/
	// WriteTree); it NEVER touches the working tree and NEVER moves a ref. At its sole call site paths is
	// always diff-names(tipTree, T_start) and sourceTree is T_start, so every path is present in T_start
	// except the deletion-leftover case (a T_start deletion no stager claimed). Bad/unresolvable tree SHA
	// ⇒ a wrapped error (code != 0; NO 128-as-non-error special case — mirror ReadTree/DiffTreeNames).
	OverlayTreePaths(ctx context.Context, baseTree, sourceTree string, paths []string) (treeSHA string, err error)
```

## 5. Test design — NEW internal/git/overlaytree_test.go (real git binary, temp repos)

Reuse the package's helpers: `initRepo(t, dir)` (git_test.go:13), `writeFile(t,dir,name,body)` +
`stageFile(t,dir,name)` (committree_test.go:31/39), `execGit(t,dir,args...)` oracle (revparsetree_test.go:115),
`runGit` (git_test.go:285). Pattern: in-package (`package git`), t.TempDir(), real git, no testify.

Build trees via the oracle: `execGit(t, dir, "write-tree")` after staging. To build distinct trees, stage
different file sets and write-tree each.

**Named cases (contract point 3 + arch §3.4):**
1. `TestOverlayTreePaths_OverlayOnlyListedPaths` — base={a.go=A,b.go=B}; source={a.go=A',c.go=C}; paths=[a.go,c.go]
   → result tree has a.go=A', b.go=B (untouched), c.go=C. Assert via `execGit ls-tree -r` oracle.
2. `TestOverlayTreePaths_DeletionOverlay` — base={a.go,b.go}; source={a.go}; paths=[b.go] (b.go absent in source)
   → result has a.go (untouched from base), b.go REMOVED.
3. `TestOverlayTreePaths_EmptyPathsNoop` — paths nil AND paths=[] → returns baseTree verbatim; assert NO index
   mutation (index == baseTree before AND after — capture `write-tree` before, assert equal after).
4. `TestOverlayTreePaths_MidChainLeftoverSimulation` — build tree1 (partial), tStart (full); paths=DiffTreeNames(tree1,tStart);
   OverlayTreePaths(tree1, tStart, paths) → result's changed-path set == DiffTreeNames(tree1, result) (leftovers folded in).
5. The standard 4-case negative set — `TestOverlayTreePaths_BadTree` / `_NotARepo` / `_GitBinaryMissing` /
   `_ContextCancelled` (mirror readtree_test.go:96/111/124/139 exactly).
6. `TestOverlayTreePaths_DoesNotTouchWorkingTree` — write a working-tree file, call OverlayTreePaths, assert
   the file is UNCHANGED on disk (mirror freezeworkingtree_test.go:122 TestFreezeWorkingTree_LeavesWorkingTreeUnchanged).

## 6. Scope discipline (S1 vs T2/T3 / P2 / P3)

S1 = the interface decl + gitRunner impl + parseLsTree + the NEW overlaytree_test.go. NOTHING ELSE.
- NOT S1: the arbiter gate/resolution rewrite (resolveArbiter → three paths using OverlayTreePaths) = P1.M1.T2.S1.
- NOT S1: the arbiter freeze-parity invariant integration tests = P1.M1.T3.
- NOT S1: the Mode A doc edit (docs/how-it-works.md arbiter narrative) = P1.M1.T2.S2.
- NOT S1: planner files / soft target = P2; README/docs sweep = P3.
- DOCS: none in S1 (internal git primitive; the narrative belongs to P1.M1.T2.S2 — Mode A).

## 7. Edge cases / gotchas

- `--cacheinfo` single-arg comma form `<mode>,<blob>,<path>` (git 2.0+). A path containing a comma would break
  the form — pre-existing limitation (arch §6 risk 5: path-quotepath edge case); tests use simple paths.
- `ls-tree` quotes paths with special chars (core.quotePath) by default; spaces are NOT special (safe). The
  parseLsTree \t-split is correct for the common case. Non-ASCII/quote paths are a pre-existing limitation.
- The empty-paths early return returns `baseTree` VERBATIM (no write-tree, no read-tree) — assert the index is
  untouched in the test (capture write-tree before, assert equal after).
- `g.run` returns (stdout, stderr, code, err); err != nil ⇒ git binary missing / context cancelled / start
  failure (UNWRAPPED, propagate); code != 0 ⇒ git non-zero exit (wrap with stderr). This is the established
  convention — mirror ReadTree/DiffTreeNames exactly.
