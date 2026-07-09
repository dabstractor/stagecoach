//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestE2EConfigPrecedence_AutoStageAll proves Issue 1's *bool fix (P1.M1.T1.S1) changes stagecoach's
// OBSERVABLE behavior through the full config-precedence chain. It is the executable contract that a
// TOML `[defaults] auto_stage_all = false` (or a `git config stagecoach.autoStageAll false`) reaches the
// consumer: the compiled binary exits 2 "Nothing to commit." instead of silently auto-staging (the PRD
// h3.0 reproduction). The positive controls prove the default-true / true paths are unchanged.
//
// These are black-box SUBPROCESS tests: they observe the exit code (§15.4 contractual), commit count,
// and working-tree state — NOT the in-process cfg struct or cosmetic stderr text.
//
// Scope fences (vs. sibling subtasks):
//   - No env-var override case for auto-stage — that is P1.M1.T2.S1 (T2).
//   - No S1 white-box materialize/overlay duplication — S1 owns internal/config/file_test.go.
//   - The stub is selected via the explicit `--provider stub` CLI arg (load.go's self-hosting guard
//     rejects AMBIENT stub selection via STAGECOACH_PROVIDER env / git key).
func TestE2EConfigPrecedence_AutoStageAll(t *testing.T) {
	bin := buildStagecoach(t)
	stub := buildStub(t)

	// defaultsExtras returns the TOML [defaults] extras block for writeStubConfig with the given
	// auto_stage_all setting ("true"/"false"/"" to omit the block entirely ⇒ default true).
	defaultsExtras := func(setting string) string {
		if setting == "" {
			return ""
		}
		return "\n[defaults]\nauto_stage_all = " + setting + "\n"
	}

	// (a) PRD h3.0 HEADLINE REPRODUCTION: TOML auto_stage_all = false ⇒ exit 2, NO commit, dirty tree.
	// This is the mechanical inverse of S2_OneFile_NoPlannerCall's positive control. With the pre-S1
	// bug (false ignored), this would exit 0 and create a spurious commit — the test fails LOUDLY.
	t.Run("a_toml_false_exits2_no_commit_dirty_tree", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "b.txt", "b\n") // UN-staged → dirty tree (do NOT stageFile)
		cfg := writeStubConfig(t, stub, defaultsExtras("false"))
		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: should not happen"})

		before := commitCount(t, repo)
		res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")

		if res.ExitCode != 2 {
			t.Fatalf("exit code = %d, want 2 (NothingToCommit); stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if got := commitCount(t, repo); got != before {
			t.Errorf("commit count = %d, want %d (no commit must be created)", got, before)
		}
		if status := statusPorcelain(t, repo); !strings.Contains(status, "b.txt") {
			t.Errorf("working tree must still be dirty (b.txt un-staged); status:\n%s", status)
		}
		// The auto-stage notice must NOT print (false short-circuits before the AddAll branch).
		if strings.Contains(res.Stdout, "staging all") {
			t.Errorf("stdout must not show the auto-stage notice; got:\n%s", res.Stdout)
		}
	})

	// (b) POSITIVE CONTROL (true): identical setup but auto_stage_all = true ⇒ exit 0, +1 commit, clean
	// tree. Proves the default-true path is unchanged and that the (a) exit-2 result is BECAUSE of
	// the false setting (not a harness/commit-msg defect).
	t.Run("b_positive_control_true_exits0_commits_clean", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "b.txt", "b\n") // un-staged
		cfg := writeStubConfig(t, stub, defaultsExtras("true"))
		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add b"})

		before := commitCount(t, repo)
		res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")

		if res.ExitCode != 0 {
			t.Fatalf("exit code = %d, want 0; stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if got := commitCount(t, repo); got != before+1 {
			t.Errorf("commit count = %d, want %d (+1 new commit)", got, before+1)
		}
		if status := statusPorcelain(t, repo); status != "" {
			t.Errorf("working tree must be CLEAN; status:\n%s", status)
		}
		// Sanity: the new commit actually touches b.txt (auto-staged + committed).
		names := diffTreeNames(t, repo, headSHA(t, repo))
		if !contains(names, "b.txt") {
			t.Errorf("new commit files = %v, want b.txt present", names)
		}
	})

	// (b-omit) POSITIVE CONTROL (omitted ⇒ default true): the strongest regression guard for "no
	// behavior change for the default". auto_stage_all is omitted entirely so the Default()-seeded true
	// wins; result identical to (b). Locks the default-true behavior so a future "default flipped"
	// regression is caught here, not in (a).
	t.Run("b_omitted_defaults_true_exits0_commits", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "b.txt", "b\n")                  // un-staged
		cfg := writeStubConfig(t, stub, defaultsExtras("")) // no [defaults] block ⇒ default true
		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add b"})

		before := commitCount(t, repo)
		res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")

		if res.ExitCode != 0 {
			t.Fatalf("exit code = %d, want 0 (default-true); stderr:\n%s", res.ExitCode, res.Stderr)
		}
		if got := commitCount(t, repo); got != before+1 {
			t.Errorf("commit count = %d, want %d (+1 new commit, default-true)", got, before+1)
		}
	})

	// (d) LAYER PRECEDENCE: global TOML (Layer 2) auto_stage_all = false overridden by repo git-config
	// (Layer 4) stagecoach.autoStageAll true ⇒ TRUE wins ⇒ exit 0 + commit. Proves the higher layer
	// overrides the lower, and that the camelCase git key propagates a true end-to-end. The git key is
	// camelCase (Issue 2: git forbids underscores in the final segment).
	t.Run("d_toml_false_gitconfig_true_wins_exits0", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "b.txt", "b\n")
		cfg := writeStubConfig(t, stub, defaultsExtras("false"))     // Layer 2 = false
		runGit(t, repo, "config", "stagecoach.autoStageAll", "true") // Layer 4 = true (camelCase!)

		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: add b"})
		before := commitCount(t, repo)
		res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")

		if res.ExitCode != 0 {
			t.Fatalf("exit code = %d, want 0 (Layer 4 true must win over Layer 2 false); stderr:\n%s",
				res.ExitCode, res.Stderr)
		}
		if got := commitCount(t, repo); got != before+1 {
			t.Errorf("commit count = %d, want %d (+1: higher layer re-enabled auto-stage)", got, before+1)
		}
	})

	// (d-bonus) GIT-CONFIG FALSE STANDALONE: no TOML setting (default true), repo git-config
	// stagecoach.autoStageAll false (Layer 4) ⇒ exit 2, no commit. Proves the git layer propagates a
	// *false end-to-end on its own (pairs with git_test.go's loadGitConfig sets *false unit proof).
	t.Run("d_gitconfig_false_standalone_exits2", func(t *testing.T) {
		repo := newRepo(t)
		seedCommit(t, repo, "readme.md", "init")
		writeFile(t, repo, "b.txt", "b\n")
		cfg := writeStubConfig(t, stub, defaultsExtras(""))           // no TOML setting (default true)
		runGit(t, repo, "config", "stagecoach.autoStageAll", "false") // Layer 4 = false (camelCase!)

		env := stubEnv(map[string]string{"STAGECOACH_STUB_OUT": "feat: x"})
		before := commitCount(t, repo)
		res := runStagecoach(t, bin, repo, cfg, env, "--provider", "stub")

		if res.ExitCode != 2 {
			t.Fatalf("exit code = %d, want 2 (git-config false must propagate end-to-end); stderr:\n%s",
				res.ExitCode, res.Stderr)
		}
		if got := commitCount(t, repo); got != before {
			t.Errorf("commit count = %d, want %d (git-config false ⇒ no commit)", got, before)
		}
		if status := statusPorcelain(t, repo); !strings.Contains(status, "b.txt") {
			t.Errorf("working tree must still be dirty (b.txt un-staged); status:\n%s", status)
		}
	})
}
