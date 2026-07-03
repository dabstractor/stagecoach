package decompose

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
)

// ErrArbiterResolutionFailed is the sentinel for arbiter-RESOLUTION infra failures (AddAll/Add/
// WriteTree/ReadTree/CommitTree infra + NON-CAS UpdateRefCAS). Wrapped (%w) so errors.Is works.
// generateMessage failure (null path) → propagate *generate.RescueError DIRECTLY (not wrapped).
// CAS failure → propagate *generate.CASError DIRECTLY (not wrapped).
var ErrArbiterResolutionFailed = errors.New("decompose: arbiter resolution failed")

// ChainEntry is one commit made this run, carrying the rebuild data the mid-chain path needs. It is
// PARALLEL to CommitInfo (P3.M3.T1.S1): same length, same order, same SHAs (chainData[i].SHA ==
// commits[i].SHA). The orchestrator builds it as it publishes each commit (it already holds tree[i],
// msg[i], newSHA[i]). resolveArbiter locates the target index via commits (per the contract) then reads
// rebuild data (Tree/Message/Parent) from chainData.
type ChainEntry struct {
	SHA     string // full commit SHA (== commits[i].SHA — parallel arrays).
	Tree    string // the commit's tree SHA (ReadTree/WriteTree target for the rebuild).
	Message string // full message — REUSED VERBATIM on amend/rebuild (NO regeneration).
	Parent  string // rebuild base: chainData[i].Parent == chainData[i-1].SHA for i>0; for i==0 the
	//   pre-run HEAD (or "" for a root commit on an unborn repo).
}

// resolveArbiter reconciles the leftover working-tree changes per runArbiter's target decision.
// It is the RESOLUTION step (PRD §13.6.5 / FR-M10); runArbiter (P3.M3.T1.S1) only DECIDED.
//
// PRECONDITION (documented; the orchestrator guarantees): the per-concept loop made ≥1 commit;
// HEAD.tree == index == tree[N-1] (the full accumulated index, clean); the WORKING TREE holds the
// leftovers (StatusPorcelain != ""). The orchestrator already called runArbiter and passes its
// ArbiterOutput.Target (nil ⇒ new; &sha ⇒ amend). commits []CommitInfo and chainData []ChainEntry are
// PARALLEL (same length, order, SHAs).
//
// Branching: target==nil || N==0 → resolveNewCommit. Else find idx where chainData[idx].SHA == *target;
// not found (runArbiter should have nulled it — defensive) → resolveNewCommit. idx==N-1 → resolveTipAmend.
// idx<N-1 → resolveMidChain. (N==1 ⇒ the only in-run commit is the tip ⇒ any non-nil target is tip amend;
// mid-chain requires N≥2.)
//
// On success the working tree is CLEAN (index == HEAD.tree == working tree). On any failure HEAD is
// UNCHANGED (refs move ONLY at the final UpdateRefCAS — §18.1). *generate.RescueError (null path) and
// *generate.CASError (any CAS failure) propagate DIRECTLY (not wrapped); other infra failures wrap
// ErrArbiterResolutionFailed.
func resolveArbiter(ctx context.Context, deps Deps, target *string, commits []CommitInfo, chainData []ChainEntry) error {
	N := len(chainData)
	if target == nil || N == 0 {
		return resolveNewCommit(ctx, deps, commits, chainData)
	}
	idx := findTargetIndex(*target, chainData)
	if idx < 0 {
		return resolveNewCommit(ctx, deps, commits, chainData) // not found → defensive null
	}
	if idx == N-1 {
		return resolveTipAmend(ctx, deps, chainData)
	}
	return resolveMidChain(ctx, deps, idx, chainData)
}

// findTargetIndex returns the index of sha in chainData, or -1 if absent.
func findTargetIndex(sha string, chainData []ChainEntry) int {
	for i, c := range chainData {
		if c.SHA == sha {
			return i
		}
	}
	return -1
}

// resolveNewCommit (path A, null): AddAll → WriteTree → generateMessage → CommitTree → UpdateRefCAS.
// Lands an (N+1)-th commit whose message is generated from the leftovers (concept diff = TreeDiff(tip,
// treePrime) = exactly the leftovers). generateMessage (P3.M2.T4.S1) is REUSED — same package.
func resolveNewCommit(ctx context.Context, deps Deps, commits []CommitInfo, chainData []ChainEntry) error {
	N := len(chainData)
	// tipSHA = current HEAD; tipTree = the base for the concept diff. On an empty run (N==0) this path
	// is reached only defensively (the arbiter does not run on a clean tree) — treat HEAD as the base.
	tipSHA, isUnborn, err := deps.Git.RevParseHEAD(ctx)
	if err != nil {
		return fmt.Errorf("%w: rev-parse head: %w", ErrArbiterResolutionFailed, err)
	}
	tipTree := ""
	if N > 0 {
		tipSHA = chainData[N-1].SHA // authoritative (chainData tracks this run's commits)
		tipTree = chainData[N-1].Tree
	} else if !isUnborn {
		tree, _ := deps.Git.RevParseTree(ctx, "HEAD") // empty "" on unborn (the EmptyTree base)
		tipTree = tree
	}

	// 1. Stage all leftovers (index == tree[N-1] ⇒ AddAll stages ONLY the leftovers).
	if err := deps.Git.AddAll(ctx); err != nil {
		return fmt.Errorf("%w: add -A: %w", ErrArbiterResolutionFailed, err)
	}
	// 2. Snapshot the staged index.
	treePrime, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return fmt.Errorf("%w: write-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// 3. Generate the message from the leftover concept diff (generateMessage derives its own parent).
	treeA := tipTree
	if treeA == "" {
		treeA = git.EmptyTreeSHA // unborn base — generateMessage's TreeDiff treats it as a tree arg
	}
	msg, err := generateMessage(ctx, deps, treeA, treePrime)
	if err != nil {
		return err // *generate.RescueError — propagate DIRECTLY (not wrapped)
	}
	// 4. Commit (parent = tipSHA; root if tipSHA=="").
	var parents []string
	if tipSHA != "" {
		parents = []string{tipSHA}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treePrime, parents, msg)
	if err != nil {
		return fmt.Errorf("%w: commit-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// 5. CAS-advance HEAD (expected-old = tipSHA = CURRENT HEAD).
	expectedOld := tipSHA
	if tipSHA == "" {
		expectedOld = strings.Repeat("0", 40) // root commit on an unborn repo
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		return handleUpdateRefErr(ctx, deps, treePrime, expectedOld, msg, err)
	}
	return nil
}

// resolveTipAmend (path B, target==tip): AddAll → WriteTree → CommitTree(tree', [tipParent], tipMsg)
// reusing the tip's message VERBATIM (NO regeneration) → UpdateRefCAS(expectedOld = tipSHA). A plumbing
// amend — no `git commit --amend`. publishCommit is NOT used (its expectedOld=parentSHA is wrong: HEAD
// currently == tipSHA, not tipParent).
func resolveTipAmend(ctx context.Context, deps Deps, chainData []ChainEntry) error {
	N := len(chainData)
	tip := chainData[N-1]
	tipSHA, tipParent, tipMsg := tip.SHA, tip.Parent, tip.Message

	if err := deps.Git.AddAll(ctx); err != nil {
		return fmt.Errorf("%w: add -A: %w", ErrArbiterResolutionFailed, err)
	}
	treePrime, err := deps.Git.WriteTree(ctx)
	if err != nil {
		return fmt.Errorf("%w: write-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// Reuse the tip's message VERBATIM (no regeneration). Parent = tipParent (the amend); root if "".
	var parents []string
	if tipParent != "" {
		parents = []string{tipParent}
	}
	newSHA, err := deps.Git.CommitTree(ctx, treePrime, parents, tipMsg)
	if err != nil {
		return fmt.Errorf("%w: commit-tree: %w", ErrArbiterResolutionFailed, err)
	}
	// CAS expected-old = tipSHA (CURRENT HEAD), NOT tipParent. (publishCommit would use tipParent = WRONG.)
	expectedOld := tipSHA
	if tipParent == "" && tipSHA == "" {
		expectedOld = strings.Repeat("0", 40) // unborn root (defensive)
	}
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", newSHA, expectedOld); err != nil {
		return handleUpdateRefErr(ctx, deps, treePrime, expectedOld, tipMsg, err)
	}
	return nil
}

// resolveMidChain (path C, target==earlier commit[i], i<N-1): deterministic linear-chain rebuild.
// NEVER interactive rebase; HEAD only; refs move ONLY at the final UpdateRefCAS.
//
//  1. Capture leftoverPaths = parse(StatusPorcelain()) — MUST be BEFORE any ReadTree (G-STATUS-FIRST).
//  2. rebuiltParent = chainData[i].Parent (for i>0 == chainData[i-1].SHA; for i==0 the pre-run HEAD / "").
//  3. for j := i; j < N; j++:
//     ReadTree(chainData[j].Tree)   // index = tree[j]
//     Add(leftoverPaths)            // fold leftovers onto tree[j] → index = tree[j]+leftovers
//     treePrime = WriteTree()
//     parent := rebuiltParent (or nil if rebuiltParent=="" for the root case at j==i==0)
//     newSHA = CommitTree(treePrime, parent, chainData[j].Message)  // REUSE msg[j] verbatim
//     rebuiltParent = newSHA
//  4. UpdateRefCAS(HEAD, rebuiltParent, tipSHA)  // single atomic move; tipSHA = chainData[N-1].SHA
//
// The fold runs at EVERY j ∈ [i, N-1] (G-FOLD): trees are cumulative, so leftovers folded into commit[i]
// must also appear in every subsequent rebuilt tree, else commit[i+1] reverts them (dirty tree).
func resolveMidChain(ctx context.Context, deps Deps, i int, chainData []ChainEntry) error {
	N := len(chainData)
	tipSHA := chainData[N-1].SHA

	// 1. Capture leftover paths BEFORE any ReadTree (StatusPorcelain is index-relative).
	status, err := deps.Git.StatusPorcelain(ctx)
	if err != nil {
		return fmt.Errorf("%w: status: %w", ErrArbiterResolutionFailed, err)
	}
	paths := leftoverPaths(status)

	// 2. Rebuild base.
	rebuiltParent := chainData[i].Parent

	// 3. Walk j = i..N-1, rebuilding each commit with leftovers folded in.
	for j := i; j < N; j++ {
		if err := deps.Git.ReadTree(ctx, chainData[j].Tree); err != nil {
			return fmt.Errorf("%w: read-tree[%d]: %w", ErrArbiterResolutionFailed, j, err)
		}
		if len(paths) > 0 {
			if err := deps.Git.Add(ctx, paths); err != nil { // fold (NOT AddAll — G-ADDALL)
				return fmt.Errorf("%w: add[%d]: %w", ErrArbiterResolutionFailed, j, err)
			}
		}
		treePrime, err := deps.Git.WriteTree(ctx)
		if err != nil {
			return fmt.Errorf("%w: write-tree[%d]: %w", ErrArbiterResolutionFailed, j, err)
		}
		var parents []string
		if rebuiltParent != "" {
			parents = []string{rebuiltParent}
		}
		newSHA, err := deps.Git.CommitTree(ctx, treePrime, parents, chainData[j].Message) // msg[j] verbatim
		if err != nil {
			return fmt.Errorf("%w: commit-tree[%d]: %w", ErrArbiterResolutionFailed, j, err)
		}
		rebuiltParent = newSHA
	}

	// 4. Single CAS move (expected-old = tipSHA = CURRENT HEAD).
	if err := deps.Git.UpdateRefCAS(ctx, "HEAD", rebuiltParent, tipSHA); err != nil {
		return handleUpdateRefErr(ctx, deps, "", tipSHA, "", err) // no single tree/msg for the rebuilt chain
	}
	return nil
}

// leftoverPaths parses `git status --porcelain` output into the leftover path set (mid-chain only).
// Each non-empty line "XY <path>" → path = line[3:]; rename/copy "XY <orig> -> <dst>" → the part after
// " -> " (destination). Lines shorter than 4 chars are skipped. core.quotepath (default on) C-quotes
// non-ASCII paths — v1 ASSUMES ASCII (documented limitation). After the per-concept loop index ==
// HEAD.tree, so ONLY leftovers (unstaged + untracked + deletions) appear — exactly the fold set.
func leftoverPaths(status string) []string {
	var paths []string
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, " \t") // only strip trailing whitespace (leading space is index status)
		if len(line) < 4 {                    // "XY <path>" minimum (2 status + 1 space + ≥1 path char)
			continue
		}
		rest := line[3:] // skip "XY "
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			rest = rest[idx+len(" -> "):] // rename/copy: take the destination
		}
		if rest != "" {
			paths = append(paths, rest)
		}
	}
	return paths
}

// handleUpdateRefErr centralizes the two UpdateRefCAS failure kinds: ErrCASFailed → *generate.CASError
// (re-read HEAD for the §13.5 Actual; errors.As-able; NOT wrapped); otherwise → wrapped
// ErrArbiterResolutionFailed (non-CAS infra). Mirrors publishCommit's CAS handling in message.go.
func handleUpdateRefErr(ctx context.Context, deps Deps, tree, expectedOld, msg string, err error) error {
	if errors.Is(err, git.ErrCASFailed) {
		actual, _, _ := deps.Git.RevParseHEAD(ctx) // re-read for the §13.5 message's Actual (D5)
		actualTree := ""                           // + Actual^{tree} for the already-committed fast path
		if actual != "" {
			actualTree, _ = deps.Git.RevParseTree(ctx, actual)
		}
		return &generate.CASError{TreeSHA: tree, Expected: expectedOld, Actual: actual, ActualTree: actualTree, Message: msg}
	}
	return fmt.Errorf("%w: update-ref: %w", ErrArbiterResolutionFailed, err)
}
