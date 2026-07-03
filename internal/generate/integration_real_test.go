//go:build integration_real
// +build integration_real

// This file is the PRD §20.1 layer-4 REAL-AGENT end-to-end verification suite
// (decisions.md §8). It is OPT-IN via a build tag (this file's first two
// lines mirror internal/provider/exec_unix.go's convention EXACTLY) AND a
// runtime env gate (STAGEHAND_RUN_REAL=1), so the default `go test ./...`
// (Makefile `test`) NEVER compiles it and a stray `-tags integration_real`
// alone still skips every test. It runs only as a manual pre-release gate
// (PRD §20.1 layer 4; §12.7.2 progressive verification).
//
// It is white-box `package generate` for the same reason as the sibling
// layer-3 suite (integration_test.go): it must name the package-private
// `Deps`/`runner`/`gitClient` types AND — critically — it REUSES the
// layer-3 suite's package-private helpers (newTempRepo/writeStage/seedCommit/
// gitRun/headSHA/commitType/commitParentLine/stagedFiles/assertHeadAndIndex-
// Unchanged/containsAll/rescueRendered from integration_test.go) because BOTH
// files compile into the SAME `package generate` test binary under
// `-tags integration_real`. This file ONLY ADDS new helpers + tests — it
// redefines NONE of the layer-3 names (a redefinition is a compile collision).
//
// What it proves (the whole point): the six compiled-in provider manifests
// (pi/claude/gemini/opencode/codex/cursor from provider.Builtins()) each
// actually generate a USABLE commit message end-to-end when driven through the
// REAL generate.CommitStaged against a REAL temp repo, the REAL git binary, and
// the REAL *provider.Executor (the only thing exercised for real is the agent
// subprocess — there is NO stub here). It ALSO resolves the four PRD Appendix E
// open questions by LIVE observation:
//
//   - E.1 (gemini ~300KB stdin vs positional): a stdin-delivery manifest
//     variant is fed a config.DefaultMaxDiffBytes (~300000-byte) payload; if it
//     returns non-empty, stdin is confirmed (flip the manifest), else keep
//     positional + document the cap.
//   - E.2 (claude --tools "" suppresses tool use): the default manifest is run;
//     assertWorktreeUnchanged proves no file mutated. On failure the
//     --disallowed-tools "*" fallback variant is retried and the working form
//     recorded.
//   - E.4a (codex exec writes answer to stdout + exit 0): a codex e2e that
//     succeeds proves it (a non-zero exit yields *AgentError → ErrRescue).
//   - E.4b (cursor --mode ask is read-only over -p's full-tools): a cursor e2e
//     whose working tree is byte-unchanged proves it.
//
// Per PRD §12.7.2 a manifest field is flipped ONLY after a real run confirms
// it; anything unconfirmed keeps its safe default + a `# TO CONFIRM` comment
// (never silently assumed). The verified outcomes are recorded in
// plan/001_f1f80943ac34/architecture/external_deps.md.
//
// Dependencies: stdlib (bytes/context/crypto-sha256/fmt/io-fs/os/path-filepath/
// strings/testing/time) + internal/{config,git,provider,ui} + the in-package
// layer-3 helpers ONLY. NO testify, NO stub binary.
package generate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/ui"
)

// realTimeout is the per-agent-invocation timeout used by this suite. Real
// agents are 10s–minutes; config.Default()'s 120s may be too tight on a cold
// model load. The outer `go test -timeout 60m` bounds the whole suite.
const realTimeout = 10 * time.Minute

// ---------------------------------------------------------------------------
// Gates: the env gate (STAGEHAND_RUN_REAL=1) + the per-agent PATH gate.
// ---------------------------------------------------------------------------

// requireReal is the ENV gate every test in this file calls FIRST. It skips
// unless STAGEHAND_RUN_REAL=1, so the build tag alone (-tags integration_real)
// is NOT enough to run a real agent — the env var is the second, independent
// lock (PRD §20.1 layer 4). Combined with the build tag this keeps real-agent
// runs (and the network/cost they incur) firmly out of `go test ./...`.
func requireReal(t *testing.T) {
	t.Helper()
	if os.Getenv("STAGEHAND_RUN_REAL") != "1" {
		t.Skip("skipping real-agent test: set STAGEHAND_RUN_REAL=1 (and -tags integration_real)")
	}
}

// requireInstalled is the per-agent PATH gate. It skips unless the manifest's
// detect target is resolvable on $PATH via provider.Registry.Detect (which
// honors the Detect field — critical for cursor, whose Detect is "agent", NOT
// "cursor"; a hardcoded exec.LookPath("cursor") would wrongly skip it). It
// returns the registry's OWNED clone of the manifest (Registry.Get deep-copies
// slices/maps), so a test that builds a variant (E.1/E.2) can mutate it
// without bleeding back into the shared Builtins() state.
func requireInstalled(t *testing.T, name string) provider.Manifest {
	t.Helper()
	reg := provider.NewRegistry(provider.Builtins(), nil)
	if !reg.Detect()[name] {
		t.Skipf("skipping: agent %q not on $PATH", name)
	}
	m, _ := reg.Get(name)
	return m
}

// realDeps wires the REAL collaborators into Deps for an end-to-end real-agent
// run: the REAL *git.Git (git.New(dir)), the REAL *provider.Executor with
// Dir=dir (CRITICAL: provider.NewExecutor(dir) — NOT "" — so cmd.Dir is the
// temp repo and the agent runs IN the repo; "" would inherit the test process
// cwd, which is WRONG for real agents even though the layer-3 stub ignores
// cwd), the registry's owned manifest clone, and a *ui.Output capturing
// stdout/stderr (noColor=true so out.Red is a no-op and captured text is plain).
func realDeps(t *testing.T, dir, name string, cfg config.Config) (Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	m := requireInstalled(t, name)
	g, err := git.New(dir)
	if err != nil {
		t.Fatalf("git.New(%q): %v", dir, err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return Deps{
		Git:      g,
		Runner:   provider.NewExecutor(dir), // Dir=repo so the agent runs IN the repo
		Manifest: m,
		Config:   cfg,
		Output:   ui.NewOutput(stdout, stderr, false, true), // noColor=true ⇒ plain captured text
	}, stdout, stderr
}

// ---------------------------------------------------------------------------
// Read-only proof: worktreeHash + assertWorktreeUnchanged.
// ---------------------------------------------------------------------------

// worktreeHash captures a digest of the REPO WORKING TREE (every file under dir
// EXCEPT the .git object store) — filename + content hashed together. It
// deliberately EXCLUDES index/HEAD/ref state (which a stagehand commit
// legitimately advances via commit-tree + update-ref, both of which write only
// into .git and NEVER touch working-tree files). So this hash is STABLE across
// a successful CommitStaged run with a read-only agent: the only way it changes
// is if the AGENT itself wrote/modified/deleted a working-tree file. That is
// exactly the read-only contract E.2/E.4b verify (claude --tools "" /
// cursor --mode ask must not mutate the repo). .git is skipped because
// stagehand writes commit/tree objects + moves HEAD there by design.
func worktreeHash(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir // stagehand writes here by design; not a read-only violation
			}
			return nil
		}
		rel, rerr := filepath.Rel(dir, path)
		if rerr != nil {
			return rerr
		}
		data, ferr := os.ReadFile(path)
		if ferr != nil {
			return ferr
		}
		fmt.Fprintf(h, "FILE %s len=%d\n", rel, len(data))
		h.Write(data)
		h.Write([]byte{'\n'})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("worktreeHash(%q): %v", dir, walkErr)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// assertWorktreeUnchanged fails the test if the working-tree digest differs
// from `before` (captured before CommitStaged), printing `git status` for
// diagnosis. It is the E.2/E.4b read-only proof.
func assertWorktreeUnchanged(t *testing.T, dir, before string) {
	t.Helper()
	if after := worktreeHash(t, dir); after != before {
		t.Errorf("working tree changed during generation (read-only contract broken)\nbefore=%s\n after=%s\n--git status--\n%s",
			before, after, gitRun(t, dir, "status", "--porcelain"))
	}
}

// ---------------------------------------------------------------------------
// Shared per-agent e2e driver (used by the six per-agent tests + E.4a).
// ---------------------------------------------------------------------------

// realAgentE2E is the shared end-to-end driver for the six per-agent tests.
// It builds a fresh unborn temp repo, seeds one baseline commit (so the repo
// has history + a recent-subject set, and the generated subject must differ
// from the seeded "feat: baseline"), stages a single new file, and drives the
// REAL generate.CommitStaged with the REAL *provider.Executor + the agent's
// real manifest. On success it asserts the full success contract: err==nil,
// the returned SHA is a REAL commit object (cat-file -t == "commit"), the
// message is non-empty, the subject is NOT the seeded duplicate, and HEAD
// advanced to the new commit. Agents are non-deterministic — the OUTER dup
// loop may retry internally — but on SUCCESS the subject is guaranteed unique
// vs RecentSubjects(50), so the duplicate-subject assertion is valid here.
func realAgentE2E(t *testing.T, name string) Result {
	t.Helper()
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline") // history + a dedup subject to differ from
	writeStage(t, dir, "feature.go", "package x\n\n// new feature\n")

	cfg := config.Default()
	cfg.Timeout = realTimeout
	deps, _, stderr := realDeps(t, dir, name, cfg)

	res, err := CommitStaged(context.Background(), deps)
	if err != nil {
		t.Fatalf("CommitStaged(%s): %v\n--stderr--\n%s", name, err, stderr.String())
	}
	if res.CommitSHA == "" {
		t.Fatalf("%s: empty CommitSHA (no commit landed)", name)
	}
	if got := commitType(t, dir, res.CommitSHA); got != "commit" {
		t.Errorf("%s: cat-file -t %q = %q; want %q (not a real commit object)", name, res.CommitSHA, got, "commit")
	}
	if strings.TrimSpace(res.Message) == "" {
		t.Errorf("%s: Result.Message is empty (agent produced no usable message)", name)
	}
	if res.Subject == "feat: baseline" {
		t.Errorf("%s: subject %q duplicates the seeded commit (uniqueness contract broken)", name, res.Subject)
	}
	if got := headSHA(t, dir); got != res.CommitSHA {
		t.Errorf("%s: HEAD = %q; want Result.CommitSHA %q (HEAD did not advance to the commit)", name, got, res.CommitSHA)
	}
	short := res.CommitSHA
	if len(short) > 7 {
		short = short[:7]
	}
	t.Logf("%s OK: [%s] %s", name, short, res.Subject)
	return res
}

// ---------------------------------------------------------------------------
// Six per-agent e2e tests — one per built-in manifest.
// ---------------------------------------------------------------------------

func TestIntegrationReal_Pi(t *testing.T) {
	requireReal(t)
	realAgentE2E(t, "pi")
}

func TestIntegrationReal_Claude(t *testing.T) {
	requireReal(t)
	realAgentE2E(t, "claude")
}

func TestIntegrationReal_Gemini(t *testing.T) {
	requireReal(t)
	realAgentE2E(t, "gemini")
}

func TestIntegrationReal_Opencode(t *testing.T) {
	requireReal(t)
	realAgentE2E(t, "opencode")
}

func TestIntegrationReal_Codex(t *testing.T) {
	requireReal(t)
	realAgentE2E(t, "codex")
}

func TestIntegrationReal_Cursor(t *testing.T) {
	requireReal(t)
	realAgentE2E(t, "cursor")
}

// ---------------------------------------------------------------------------
// Appendix E confirmation tests (the four open-question resolutions).
// ---------------------------------------------------------------------------

// TestIntegrationReal_CodexExecStdoutExit0 resolves PRD Appendix E.4a: it
// confirms `codex exec` writes the answer to STDOUT and exits 0. The codex
// manifest uses subcommand=["exec"] + prompt_delivery="stdin" + the
// --sandbox read-only/--ephemeral bare set (--ask-for-approval was REMOVED:
// the real run 2026-07-03 proved `codex exec` exits 2 on it). A successful
// CommitStaged here PROVES the contract: if `codex exec` had exited
// non-zero, provider.Executor.Run would return a *AgentError and CommitStaged
// would turn it into ErrRescue (test.Fatalf); if it had written nothing to
// stdout, Parse would fail ok=false and the inner loop would exhaust into
// ErrRescue. So err==nil + a non-empty message ⟹ codex exec wrote the answer
// to stdout + exited 0.
//
// Host reality (real run 2026-07-03): on this verification host codex reaches
// model invocation (the manifest parses/execs cleanly — the --ask-for-approval
// exit-2 is gone) but fails at the OpenAI auth layer with HTTP 401, so the
// full stdout/exit-0 confirmation is BLOCKED here and remains TO CONFIRM in an
// authenticated environment. The deterministic half (manifest validity) IS
// confirmed by the removal of the exit-2 error.
func TestIntegrationReal_CodexExecStdoutExit0(t *testing.T) {
	requireReal(t)
	res := realAgentE2E(t, "codex")
	if strings.TrimSpace(res.Message) == "" {
		t.Fatal("codex produced an empty message (E.4a stdout/exit-0 contract unproven)")
	}
	short := res.CommitSHA
	if len(short) > 7 {
		short = short[:7]
	}
	t.Logf("E.4a RESOLVED: codex exec writes the answer to stdout + exits 0 (commit %s)", short)
}

// TestIntegrationReal_CursorModeAskReadOnly resolves PRD Appendix E.4b: it
// confirms cursor's `--mode ask` is read-only over `-p`'s default full-tools
// profile. The cursor manifest is print_flag="-p" + bare_flags=["--mode",
// "ask", "--trust"]; -p grants full tools, but --mode ask is documented as
// read-only Q&A. We capture the working-tree digest BEFORE CommitStaged and
// assert it is byte-unchanged AFTER — proving the agent mutated no repo file.
// (stagehand's own commit-tree + update-ref write only into .git, which
// worktreeHash excludes, so a clean read-only agent yields a stable digest.)
func TestIntegrationReal_CursorModeAskReadOnly(t *testing.T) {
	requireReal(t)
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "cursor_change.go", "package cursor\n\n// staged change\n")
	before := worktreeHash(t, dir)

	cfg := config.Default()
	cfg.Timeout = realTimeout
	deps, _, _ := realDeps(t, dir, "cursor", cfg)
	if _, err := CommitStaged(context.Background(), deps); err != nil {
		t.Fatalf("cursor e2e failed (E.4b cannot be resolved): %v", err)
	}
	assertWorktreeUnchanged(t, dir, before)
	t.Log("E.4b RESOLVED: cursor --mode ask left the working tree byte-unchanged (read-only over -p's full-tools)")
}

// TestIntegrationReal_ClaudeToolsSuppressed resolves PRD Appendix E.2: it
// confirms claude's `--tools ""` (the default manifest's BareFlags) suppresses
// tool use — i.e. the agent cannot mutate the repo. We capture the working-tree
// digest BEFORE CommitStaged and assert it unchanged AFTER. On FAILURE (the
// `--tools ""` form proving insufficient), we retry with a manifest VARIANT
// that appends `--disallowed-tools "*"` (host `claude --help` confirms this
// syntax exists alongside --tools) and record which form worked. The variant
// is built by COPYING BareFlags into a fresh slice first — Builtins() shares
// slice backing arrays, so an in-place append would corrupt the shared manifest.
func TestIntegrationReal_ClaudeToolsSuppressed(t *testing.T) {
	requireReal(t)
	dir := newTempRepo(t)
	seedCommit(t, dir, "feat: baseline")
	writeStage(t, dir, "claude_change.go", "package claude\n\n// staged change\n")
	before := worktreeHash(t, dir)

	cfg := config.Default()
	cfg.Timeout = realTimeout
	deps, _, _ := realDeps(t, dir, "claude", cfg)
	if _, err := CommitStaged(context.Background(), deps); err != nil {
		// Fallback (E.2): --tools "" alone was insufficient — retry with the
		// --disallowed-tools "*" variant. Copy BareFlags into a FRESH slice so
		// the shared Builtins() manifest is not corrupted.
		m2 := deps.Manifest
		m2.BareFlags = append(append([]string(nil), m2.BareFlags...), "--disallowed-tools", "*")
		deps.Manifest = m2
		if _, err2 := CommitStaged(context.Background(), deps); err2 != nil {
			t.Fatalf("claude tool-suppression failed both --tools \"\" (%v) and --disallowed-tools \"*\" (%v); "+
				"record in external_deps §B.2 that neither form is sufficient", err, err2)
		}
		t.Logf("E.2: --tools \"\" was insufficient (%v); the --disallowed-tools \"*\" fallback is REQUIRED. "+
			"Record in external_deps §B.2 that claude's manifest must carry --disallowed-tools \"*\".", err)
	}
	assertWorktreeUnchanged(t, dir, before)
	t.Log("E.2 RESOLVED: claude tool use was suppressed and the working tree is byte-unchanged")
}

// TestIntegrationReal_GeminiStdinLargePayload resolves PRD Appendix E.1: it
// tests whether gemini can accept a ~300KB (config.DefaultMaxDiffBytes = 300000)
// payload via STDIN. The shipped gemini manifest is positional delivery with
// an empty print_flag (because -p is DEPRECATED); this test builds a VARIANT
// with PromptDelivery=DeliveryStdin and feeds it a ~300000-byte payload through
// provider.Executor.Run directly (no git, no CommitStaged — the question is
// purely about stdin acceptance). If Run returns err==nil AND a non-empty
// message, stdin delivery is CONFIRMED at ~300KB (the manifest should flip to
// stdin, avoiding ARG_MAX risk on large diffs). Otherwise we keep the shipped
// positional default + document the 300KB cap (FR3) — the documented safe
// default, NOT a failure (the test always passes; the DECISION is in the log).
func TestIntegrationReal_GeminiStdinLargePayload(t *testing.T) {
	requireReal(t)
	m := requireInstalled(t, "gemini")
	// Build a stdin-delivery VARIANT (copy the struct; PromptDelivery is a
	// scalar so a value copy is sufficient — we do NOT mutate m.BareFlags).
	m2 := m
	m2.PromptDelivery = provider.DeliveryStdin // -p stays empty (deprecated); delivery via stdin

	// A ~300KB payload: "// line of diff\n" is 15 bytes; 300000/15 == 20000 lines.
	// config.DefaultMaxDiffBytes is 300000 — the FR3 cap this test exercises.
	const line = "// line of diff\n"
	big := strings.Repeat(line, config.DefaultMaxDiffBytes/len(line))
	payload := big + "\n\nGenerate a commit message for these changes:"
	sys := "Output ONLY the commit message. No preamble, no markdown, no code fence."

	ctx, cancel := context.WithTimeout(context.Background(), realTimeout)
	defer cancel()
	out, err := provider.NewExecutor(t.TempDir()).Run(ctx, m2, "", "", sys, payload)
	if err != nil || strings.TrimSpace(out) == "" {
		t.Logf("E.1: gemini stdin delivery of a ~300KB payload NOT confirmed (err=%v, stdout_empty=%v). "+
			"Decision: KEEP positional delivery + document the 300KB cap (FR3). Record in external_deps §B.3.", err, strings.TrimSpace(out) == "")
		return
	}
	t.Logf("E.1 RESOLVED: gemini accepts a ~300KB stdin payload (stdout=%d non-empty bytes). "+
		"Decision: FLIP gemini's manifest PromptDelivery to stdin in builtin.go + external_deps §B.3.", len(out))
}
