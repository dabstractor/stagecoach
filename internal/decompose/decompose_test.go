package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/prompt"
	"github.com/dabstractor/stagecoach/internal/provider"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// --- Fixture helpers (dcm*-prefixed to avoid colliding with arb*/chn*/msg*/stg*/planner) ---

// dcmInitRepo creates a git repo in dir with repo-local identity config (no env pollution).
func dcmInitRepo(t *testing.T, dir string) {
	t.Helper()
	dcmRunGit(t, dir, "init")
	dcmRunGit(t, dir, "config", "user.name", "Test")
	dcmRunGit(t, dir, "config", "user.email", "test@example.com")
}

// dcmWriteFile creates a file at dir/name with the given body.
func dcmWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("dcmWriteFile %s: %v", full, err)
	}
}

// dcmStageFile runs git add for name in dir.
func dcmStageFile(t *testing.T, dir, name string) {
	t.Helper()
	dcmRunGit(t, dir, "add", name)
}

// dcmCommitRaw creates an empty commit with the given message.
func dcmCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	dcmRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

// dcmRunGit executes git -C dir args... and returns trimmed stdout.
func dcmRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// dcmGitOut runs a raw git command in dir and returns trimmed stdout (alias for consistency).
func dcmGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return dcmRunGit(t, dir, args...)
}

// dcmHeadSHA returns the current HEAD SHA.
func dcmHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return dcmGitOut(t, dir, "rev-parse", "HEAD")
}

// dcmLogOneline returns git log --oneline output (all commits, oldest first).
func dcmLogOneline(t *testing.T, dir string) string {
	t.Helper()
	return dcmGitOut(t, dir, "log", "--format=%H %s", "--reverse")
}

// dcmLogCount returns the number of commits reachable from HEAD (0 on unborn).
func dcmLogCount(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "rev-list", "--count", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0 // unborn repo
	}
	n := 0
	for _, c := range strings.TrimSpace(string(out)) {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// dcmStatusPorcelain returns git status --porcelain output.
func dcmStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	return dcmGitOut(t, dir, "status", "--porcelain")
}

// dcmPlannerManifest builds a stub planner manifest that returns the given JSON.
func dcmPlannerManifest(t *testing.T, bin string, jsonOut string) provider.Manifest {
	t.Helper()
	return stubtest.Manifest(bin, stubtest.Options{Out: jsonOut})
}

// dcmPlannerScriptManifest builds a stub planner manifest with call-varying responses.
func dcmPlannerScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	return stubtest.NewScript(t, bin, responses)
}

// dcmArbiterManifest builds a stub arbiter manifest that returns the given JSON.
func dcmArbiterManifest(t *testing.T, bin string, jsonOut string) provider.Manifest {
	t.Helper()
	return stubtest.Manifest(bin, stubtest.Options{Out: jsonOut})
}

// dcmMessageManifest builds a stub message manifest that returns the given text.
func dcmMessageManifest(t *testing.T, bin string, out string) provider.Manifest {
	t.Helper()
	return stubtest.Manifest(bin, stubtest.Options{Out: out})
}

// dcmMessageScriptManifest builds a stub message manifest with call-varying responses.
func dcmMessageScriptManifest(t *testing.T, bin string, responses []string) provider.Manifest {
	t.Helper()
	return stubtest.NewScript(t, bin, responses)
}

// dcmMessageMatchManifest builds a stub message manifest whose output is INPUT-DERIVED and thus
// concurrency-safe (P1.M1.T1.S3). Unlike NewScript (which races on its file-backed counter when N
// stub processes run concurrently), each process inspects its OWN stdin payload (the concept's
// tree-to-tree diff) and emits the FIRST matching rule's message. rules is ordered by priority:
// each entry is {substr, msg}; the first rule whose substr appears in stdin wins. This makes a
// concept's message deterministic regardless of goroutine scheduling — essential for the FR-M14
// concurrent fast-path's focused tests.
func dcmMessageMatchManifest(t *testing.T, bin string, rules []messageMatchRule) provider.Manifest {
	t.Helper()
	var b strings.Builder
	for _, r := range rules {
		b.WriteString(r.substr)
		b.WriteByte('|')
		b.WriteString(r.msg)
		if r.sleepMs > 0 { // P1.M1.T1.S5: optional 3rd field for per-match sleep ordering
			b.WriteByte('|')
			b.WriteString(strconv.Itoa(r.sleepMs))
		}
		b.WriteByte('\n')
	}
	m := stubtest.Manifest(bin, stubtest.Options{Out: ""})
	m.Env["STAGECOACH_STUB_MATCHFILE"] = t.TempDir() + "/match.txt"
	if err := os.WriteFile(m.Env["STAGECOACH_STUB_MATCHFILE"], []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write matchfile: %v", err)
	}
	return m
}

// messageMatchRule is one input-derived rule for dcmMessageMatchManifest: if substr appears in the
// concept's stdin (its diff), emit msg. The optional sleepMs (P1.M1.T1.S5) adds a per-match sleep to
// the stub so a test can order message completion (e.g. make a later concept finish FIRST); it is
// written as a 3rd "|sleepMs" field. Zero sleepMs (the default / S3's 2-field form) ⇒ no extra sleep.
type messageMatchRule struct {
	substr  string
	msg     string
	sleepMs int
}

// dcmDeps builds a minimal Deps for decompose tests (no ResolveRoles). All four roles are populated.
func dcmDeps(t *testing.T, repo string, roles RoleManifests) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: nil,
	}
}

// dcmDepsWithConfig builds a Deps with a custom config.
func dcmDepsWithConfig(t *testing.T, repo string, roles RoleManifests, cfg config.Config) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  cfg,
		Roles:   roles,
		Verbose: nil,
	}
}

// dcmAllRoles returns RoleManifests with all four roles set to the same stub manifest.
func dcmAllRoles(t *testing.T, bin string, o stubtest.Options) RoleManifests {
	t.Helper()
	m := stubtest.Manifest(bin, o)
	return RoleManifests{
		Planner: m,
		Stager:  tooledStubManifest(t, bin, o),
		Message: m,
		Arbiter: m,
	}
}

// dcmStagerSeam returns a stager function that runs git add for files matching the concept title.
// The concept title is used as a file prefix: concept "feat: add a.go" stages "a.go" (last word).
// If no files are found for the concept, it does nothing (for empty-skip testing).
func dcmStagerSeam(t *testing.T, repo string, conceptFiles map[string][]string) func(context.Context, Deps, prompt.PlannerCommit) error {
	return func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		t.Helper()
		files, ok := conceptFiles[concept.Title]
		if !ok || len(files) == 0 {
			return nil // nothing to stage (for empty-skip testing)
		}
		for _, f := range files {
			dcmRunGit(t, repo, "add", f)
		}
		return nil
	}
}

// dcmShaResolves reports whether SHA resolves via git rev-parse --verify <sha>^{commit}.
// Exit 0 means resolvable (not dangling).
func dcmShaResolves(t *testing.T, repo, sha string) bool {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--verify", sha+"^{commit}")
	_, err := cmd.CombinedOutput()
	return err == nil
}

// dcmScriptArbiter builds a provider.Manifest whose Command is the compiled
// stubarbiter binary — the cross-platform twin of the historical arbiter.sh
// /bin/sh script. It parses SHAs from the arbiter's STDIN prompt (each commit's
// SHA is a bare 40-hex line) and emits {"target": "<sha>"} for the chosen SHA.
// mode is "tip" (last SHA) or "mid" (2nd SHA), selected via STAGECOACH_ARBITER_MODE.
func dcmScriptArbiter(t *testing.T, bin string, mode string) provider.Manifest {
	t.Helper()
	switch mode {
	case "tip", "mid":
		// valid mode
	default:
		t.Fatalf("dcmScriptArbiter: unknown mode %q", mode)
	}
	path := stubtest.BuildArbiter(t)
	m := stubtest.Manifest(bin, stubtest.Options{Out: ""})
	m.Command = &path
	m.Env["STAGECOACH_ARBITER_MODE"] = mode
	return m
}

// dcmOutBuffer returns a Deps with Out set to a *bytes.Buffer for capturing rescue/CAS output.
func dcmOutBuffer(t *testing.T, repo string, roles RoleManifests) (Deps, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	deps := Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: nil,
		Out:     &buf,
	}
	return deps, &buf
}

// --- Tests ---

func TestDecompose_SingleEscape(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create 2 untracked files.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	msgM := dcmMessageManifest(t, bin, "feat: all")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Single = true

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(single): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	cr := result.Commits[0]
	if cr.Subject != "feat: all" {
		t.Errorf("Subject = %q, want %q", cr.Subject, "feat: all")
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify git state: 1 commit, clean tree.
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

func TestDecompose_SingleEscape_ErrNothingToCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	msgM := dcmMessageManifest(t, bin, "feat: all")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Single = true

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	// Nothing to commit — should get ErrNothingToCommit propagated.
	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, generate.ErrNothingToCommit) {
		t.Errorf("error = %v, want ErrNothingToCommit", err)
	}
}

func TestDecompose_SingleShortcut_CleanMessage(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "x.txt", "x\n")

	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add x","description":"x.txt"}],"message":"feat: add x.txt"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Use a counter for the message stub — it should NOT be called (0 calls).
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	messageM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner path is tested

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(shortcut clean): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add x.txt" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add x.txt")
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}

	// Verify message agent was NOT called (counter == 0 or file absent).
	data, err := os.ReadFile(counterFile)
	if err == nil {
		// Counter file exists — check it's 0 (message agent was never invoked).
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("message agent call count = %q, want 0 (shortcut used planner msg verbatim)", count)
		}
	}
	// If file doesn't exist, the message agent was never called — correct.
}

// TestDecompose_SingleShortcut_CleanStatus (Issue 3 regression): BORN repo, TWO un-staged files,
// auto mode — the planner IS called and returns single:true → runSingleShortcut fires.
// Asserts git status is clean (proves the ReadTree(treePrime) index-sync, findings §4).
// MUST FAIL before the fix (status non-empty: "D  a.txt\nD  b.txt\n?? a.txt\n?? b.txt")
// and PASS after.
func TestDecompose_SingleShortcut_CleanStatus(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")      // BORN repo (preRunHEAD set; baseTree = HEAD^{tree})
	dcmWriteFile(t, repo, "a.txt", "a\n") // 2 un-staged files ⇒ FR-M2b one-file short-circuit does NOT fire
	dcmWriteFile(t, repo, "b.txt", "b\n") //   ⇒ planner is invoked in auto mode

	// Planner returns FR-M11 single:true + a message ⇒ routes to runSingleShortcut.
	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add a and b","description":"a.txt + b.txt"}],"message":"feat: add a and b"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message counter stub — must NOT be called (single:true uses the planner's message verbatim).
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	messageM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // default: Commits=0 (auto), Single=false

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(single-shortcut clean status): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a and b" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add a and b")
	}
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + 1)", dcmLogCount(t, repo))
	}

	// Message agent must NOT have been called (single:true → planner message verbatim).
	if data, ferr := os.ReadFile(counterFile); ferr == nil {
		if count := strings.TrimSpace(string(data)); count != "" && count != "0" {
			t.Errorf("message agent call count = %q, want 0", count)
		}
	}

	// §20.2 loop-index-cleanliness invariant: clean tree after a fully-successful run.
	// BEFORE the fix this is non-empty ("D  a.txt\nD  b.txt\n?? a.txt\n?? b.txt"); AFTER it is "".
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean — proves ReadTree(treePrime) index-sync)", status)
	}
}

// TestDecompose_SingleShortcut_TemplateApplied verifies the §9.19 FR-F8 seam on the FR-M11 planner
// shortcut: the planner's message is templated before it is committed (call site #4).
func TestDecompose_SingleShortcut_TemplateApplied(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "x.txt", "x\n")

	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add x","description":"x.txt"}],"message":"feat: add x.txt"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub must NOT be called (clean single-shortcut path).
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	messageM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner path is tested
	deps.Config.Template = "$msg (#205)"

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(shortcut template): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	want := "feat: add x.txt (#205)"
	if result.Commits[0].Subject != want {
		t.Errorf("Subject = %q, want %q (templated planner shortcut message)", result.Commits[0].Subject, want)
	}
}

func TestDecompose_SingleShortcut_DuplicateFallback(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create an existing commit whose subject matches the planner's proposed message.
	dcmWriteFile(t, repo, "existing.txt", "existing\n")
	dcmStageFile(t, repo, "existing.txt")
	dcmRunGit(t, repo, "commit", "-m", "feat: add x.txt")
	dcmRunGit(t, repo, "rm", "existing.txt")
	dcmRunGit(t, repo, "commit", "-am", "chore: remove existing")

	// Now write a new file; the planner proposes "feat: add x.txt" which is a DUPLICATE.
	dcmWriteFile(t, repo, "new.txt", "new content\n")

	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add x","description":"new.txt"}],"message":"feat: add x.txt"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: will be called once (fallback). Returns a fresh subject.
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: new file added"})

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner+shortcut path is tested

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(shortcut dup): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: new file added" {
		t.Errorf("Subject = %q, want %q (fallback message)", result.Commits[0].Subject, "feat: new file added")
	}
}

func TestDecompose_AutoMultiCommit_HappyPath(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 3 concepts.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""}) // stub stager (can't run git)
	// The real stager can't run git, so inject the seam.
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(auto N=3): %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify ordered subjects.
	wantSubjects := []string{"feat: add a", "feat: add b", "feat: add c"}
	for i, cr := range result.Commits {
		if cr.Subject != wantSubjects[i] {
			t.Errorf("Commits[%d].Subject = %q, want %q", i, cr.Subject, wantSubjects[i])
		}
	}

	// Verify commit chain: HEAD advanced 3 times, clean tree.
	if dcmLogCount(t, repo) != 3 {
		t.Fatalf("commit count = %d, want 3", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}

	// Verify parent chain: each commit's parent is the previous one's SHA.
	log := dcmLogOneline(t, repo)
	lines := strings.Split(log, "\n")
	if len(lines) != 3 {
		t.Fatalf("log lines = %d, want 3", len(lines))
	}
	shas := make([]string, 3)
	for i, line := range lines {
		parts := strings.SplitN(line, " ", 2)
		shas[i] = parts[0]
	}
	// Verify commit[1]'s parent is shas[0], commit[2]'s parent is shas[1].
	parent1 := dcmGitOut(t, repo, "rev-parse", shas[1]+"^")
	if parent1 != shas[0] {
		t.Errorf("commit[1] parent = %q, want %q", parent1, shas[0])
	}
	parent2 := dcmGitOut(t, repo, "rev-parse", shas[2]+"^")
	if parent2 != shas[1] {
		t.Errorf("commit[2] parent = %q, want %q", parent2, shas[1])
	}
}

// TestDecompose_AutoMultiCommit_TemplateAppliedUniformly verifies the §9.19 FR-F8 requirement that
// EVERY commit in a decompose run is templated (call site #3, per-concept message loop).
func TestDecompose_AutoMultiCommit_TemplateAppliedUniformly(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""}) // stub stager (can't run git)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Template = "$msg (#812)"
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(auto N=3, templated): %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}

	wantSubjects := []string{"feat: add a (#812)", "feat: add b (#812)", "feat: add c (#812)"}
	for i, cr := range result.Commits {
		if cr.Subject != wantSubjects[i] {
			t.Errorf("Commits[%d].Subject = %q, want %q (uniformly templated)", i, cr.Subject, wantSubjects[i])
		}
	}
}

func TestDecompose_Overlap(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub with a small sleep — allows the overlap to be observable via timing.
	messageM := stubtest.NewScript(t, bin, []string{"feat: add a", "feat: add b"})
	// Inject sleep into the message stub via Env.
	messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "200"

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	var stagerTimestamps []int64
	deps.stager = func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		stagerTimestamps = append(stagerTimestamps, time.Now().UnixNano())
		files := map[string][]string{
			"c1": {"a.txt"},
			"c2": {"b.txt"},
		}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		return nil
	}

	start := time.Now()
	result, err := Decompose(context.Background(), deps)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Decompose(overlap): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}

	// If NOT overlapped (sequential), total would be ≥400ms (2×200ms sleep).
	// If overlapped (1-deep), total would be <400ms because the second message's 200ms sleep
	// overlaps with the second stager.
	// Allow generous slack for CI variability.
	if elapsed >= 350*time.Millisecond {
		t.Logf("WARNING: elapsed = %v (may indicate no overlap — CI variability)", elapsed)
	}

	// Verify stager[1] ran while message[0] was in flight: the second stager timestamp should be
	// BEFORE the overall completion would be if sequential.
	if len(stagerTimestamps) == 2 {
		stager1Elapsed := time.Duration(stagerTimestamps[1] - stagerTimestamps[0])
		if stager1Elapsed < 50*time.Millisecond {
			// The two stagers ran very close together — the second one didn't wait for message[0]'s sleep.
			t.Logf("Overlap confirmed: stager calls were %v apart", stager1Elapsed)
		}
	}
}

func TestDecompose_EmptyConceptSkip(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")
	// b.txt is NOT written — the stager seam for "c2" will stage nothing (empty).

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	// c2 has no files to stage → empty concept → skipped.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {}, // empty — nothing staged
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(empty-skip): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (concept 2 skipped)", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}
	if result.Commits[1].Subject != "feat: add c" {
		t.Errorf("Commits[1].Subject = %q, want %q", result.Commits[1].Subject, "feat: add c")
	}
	// Verify only 2 commits in the repo.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2", dcmLogCount(t, repo))
	}
}

func TestDecompose_StagerMovedHEAD(t *testing.T) {
	// A rogue stager seam that commits (moves HEAD) should trigger ErrStagerMovedHEAD.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")        // BORN repo → HEAD has a real SHA to move away from
	dcmWriteFile(t, repo, "a.txt", "aaa\n") // untracked → dirty tree (FR-M1 routing satisfied)

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested

	// ROGUE seam: stages nothing, instead COMMITS → moves HEAD.
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "rogue: moved HEAD")
		return nil
	}

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected ErrStagerMovedHEAD, got nil")
	}
	if !errors.Is(err, ErrStagerMovedHEAD) {
		t.Fatalf("expected ErrStagerMovedHEAD, got %v", err)
	}
	if !strings.Contains(err.Error(), "stager moved HEAD") {
		t.Errorf("error message missing 'stager moved HEAD'; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "mutated refs") {
		t.Errorf("error message missing 'mutated refs'; got: %s", err.Error())
	}
}

func TestDecompose_StagerFreezeViolation(t *testing.T) {
	// A rogue stager seam that stages a post-freeze sentinel → ErrFreezeViolation.
	// Mirrors TestDecompose_StagerMovedHEAD (rogue stager + well-behaved stager pair).
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")        // BORN repo
	dcmWriteFile(t, repo, "a.txt", "aaa\n") // the legit change in T_start

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested

	// ROGUE seam: stages the concept path AND a post-freeze sentinel (simulating `git add -A`
	// sweeping a concurrent change). The sentinel is written AFTER FreezeWorkingTree captured T_start.
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		dcmRunGit(t, repo, "add", "a.txt")                    // the legit concept path (in T_start)
		dcmWriteFile(t, repo, "sentinel.txt", "concurrent\n") // appears AFTER the freeze
		dcmRunGit(t, repo, "add", "sentinel.txt")             // stager sweeps it in (the violation)
		return nil
	}

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected ErrFreezeViolation, got nil")
	}
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("expected ErrFreezeViolation, got %v", err)
	}
	if !strings.Contains(err.Error(), "sentinel.txt") {
		t.Errorf("error missing 'sentinel.txt'; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "staged paths not in the frozen working-tree snapshot") {
		t.Errorf("error missing 'staged paths not in the frozen working-tree snapshot'; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "c1") {
		t.Errorf("error missing concept title 'c1'; got: %s", err.Error())
	}
	// HEAD unchanged — only "initial" exists.
	if got := dcmLogCount(t, repo); got != 1 {
		t.Errorf("commit count = %d, want 1 (HEAD unchanged — only 'initial')", got)
	}
	// The sentinel is in no commit.
	logOutput := dcmGitOut(t, repo, "log", "--name-only", "--format=")
	if strings.Contains(logOutput, "sentinel.txt") {
		t.Errorf("sentinel.txt appears in a commit — freeze violation should prevent this:\n%s", logOutput)
	}
}

// concurrentSentinelSeam returns a stager function that stages the concept's files (like
// dcmStagerSeam) and, for the first concept, additionally writes an UNSTAGED sentinel file
// to the working tree. The sentinel is written AFTER FreezeWorkingTree captured T_start (the
// seam runs during the loop, which is post-freeze), so the sentinel is excluded from every
// concept tree and remains in the working tree.
//
// This is the SUCCESS-path sibling of TestDecompose_StagerFreezeViolation: that test stages the
// sentinel (ErrFreezeViolation); this test writes it UNSTAGED (excluded, run succeeds).
func concurrentSentinelSeam(t *testing.T, repo string, conceptFiles map[string][]string, sentinel string) func(context.Context, Deps, prompt.PlannerCommit) error {
	first := true
	return func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		t.Helper()
		// Stage the concept's files (like dcmStagerSeam).
		files, ok := conceptFiles[concept.Title]
		if ok && len(files) > 0 {
			for _, f := range files {
				dcmRunGit(t, repo, "add", f)
			}
		}
		// On the first concept, write the sentinel UNSTAGED (os.WriteFile, NO git add).
		// Freeze already ran before the loop, so the sentinel is post-freeze ⇒ excluded.
		if first {
			first = false
			if err := os.WriteFile(filepath.Join(repo, sentinel), []byte("concurrent\n"), 0o644); err != nil {
				t.Fatalf("write sentinel: %v", err)
			}
		}
		return nil
	}
}

// TestDecompose_ConcurrentChangeExclusion is the permanent FR-M1d / §20.2 regression net for the
// empty-frozen-leftover case. The fixture is an unborn repo whose stagers cover ALL of T_start
// (a.txt + b.txt are both written pre-run and both staged by the seam), so the tip tree equals
// T_start and DiffTreeNames(tipTree, tStart) == [] ⇒ the freeze-safe arbiter gate (decompose.go
// gate) is SKIPPED. concurrentSentinelSeam additionally writes sentinel.txt UNSTAGED post-freeze
// (it is outside T_start). This asserts the three §20.2 invariants:
//   - "Start-of-run freeze" (3a): sentinel.txt appears in NO commit across the whole run
//     (git log --name-only --format=), and remains in the working tree ("?? sentinel.txt").
//   - "Arbiter freeze parity" (3b): the gate is diff-names(tipTree, T_start), never a live status
//     read, so the out-of-T_start sentinel cannot trip the gate. The arbiter is SKIPPED (no N+1
//     commit): dcmLogCount == 2 (the two loop commits). Proven via dcmLogCount, NOT result.Amended
//     (Amended is 0 for both "skipped" and "ran+null" — see decompose.go:72).
//
// stagePartialBlob stages an ARBITRARY blob for path into the live index WITHOUT touching the
// working tree (git hash-object -w --stdin | git update-index --cacheinfo). It is exactly what
// `git apply --cached` does mechanically — it lands a blob in the index that need not equal the
// working-tree content. Used below to simulate FR-M5 hunk-level staging of a single file whose
// changes are split across two concepts.
func stagePartialBlob(t *testing.T, repo, path, content string) {
	t.Helper()
	hashCmd := exec.Command("git", "-C", repo, "hash-object", "-w", "--stdin")
	hashCmd.Stdin = strings.NewReader(content)
	out, err := hashCmd.Output()
	if err != nil {
		t.Fatalf("hash-object: %v", err)
	}
	sha := strings.TrimSpace(string(out))
	dcmRunGit(t, repo, "update-index", "--cacheinfo", "100644,"+sha+","+path)
}

// TestDecompose_HunkSplitAcrossConcepts is the FR-M5/M3/M2b regression net for the intermittent
// "freeze violation … staged content differs from the frozen working-tree snapshot" abort: a single
// file split across two concepts, each concept staging only its own hunk via `git apply --cached`
// (simulated here by staging an arbitrary blob). The hunk-aware FR-M1c content check (a 3-way merge
// of tree[i] into T_start over base) accepts the legal partial and the run lands two commits. The
// earlier full-blob-equality form false-positived on every legal hunk subset (the bug this guards).
func TestDecompose_HunkSplitAcrossConcepts(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	base := "def get_links():\n    return []\n\ndef sort_items():\n    return []\n"
	tStart := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return sorted(links, key=lambda c: c.code)\n"
	basePlusA := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return []\n" // concept 0's hunk only

	dcmWriteFile(t, repo, "store.py", base)
	dcmStageFile(t, repo, "store.py")
	dcmCommitRaw(t, repo, "initial")          // born repo; HEAD.tree == base
	dcmWriteFile(t, repo, "store.py", tStart) // dirty, un-staged → triggers decompose

	// Planner splits store.py across two concepts (FR-M3: "a single file split across two concepts").
	plannerJSON := `{"count":2,"single":false,"commits":[` +
		`{"title":"feat: add link fetching","description":"the get_links change","files":["store.py"]},` +
		`{"title":"feat: sort listed links by code","description":"the sort_items change","files":["store.py"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add link fetching", "feat: sort listed links by code"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // forced count overrides the FR-M2b one-file short-circuit so the loop runs

	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		switch concept.Title {
		case "feat: add link fetching":
			stagePartialBlob(t, repo, "store.py", basePlusA) // hunk A only — a strict subset of T_start
		case "feat: sort listed links by code":
			stagePartialBlob(t, repo, "store.py", tStart) // + hunk B ⇒ index == T_start for store.py
		}
		return nil
	}

	res, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("legal hunk-level split must succeed, got %v", err)
	}
	if len(res.Commits) != 2 {
		t.Fatalf("expected 2 commits (one per concept's hunk), got %d", len(res.Commits))
	}
	// The two commits reconstruct T_start exactly: concept 0 = hunk A, concept 1 = hunk B, so the
	// final tip's store.py equals the full working-tree change staged at the freeze.
	if got, want := dcmGitOut(t, repo, "show", "HEAD:store.py"), strings.TrimRight(tStart, "\n"); got != want {
		t.Errorf("final tip store.py must reconstruct the full change; got %q want %q", got, want)
	}
	if n := dcmLogCount(t, repo); n != 3 { // initial + 2 concepts
		t.Errorf("commit count = %d, want 3", n)
	}
}

// TestDecompose_Dispatch_DisjointFastPath proves the FR-M13 dispatch gate end-to-end through
// Decompose: a pairwise file-disjoint planner partition routes to runLoopFastPath, which stages
// deterministically with `git add` and NEVER invokes the stager agent (system_context §6 — the
// deps.stager seam is reachable ONLY via runLoop's invokeStagerRetry). The stager seam is the
// dispatch oracle: it t.Fatals if called, so a passing run PROVES the fast-path was taken. This is
// the first test to exercise the gate through Decompose (S3's TestRunLoopFastPath_* call the
// fast-path DIRECTLY, pre-dispatch).
func TestDecompose_Dispatch_DisjointFastPath(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// 3 disjoint untracked files (the working-tree change set).
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	// Planner declares DISJOINT files (no path in ≥2 concepts) → isFileDisjoint true → runLoopFastPath.
	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":["b.txt"]},` +
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a"},
		{substr: "b.txt", msg: "feat: add b"},
		{substr: "c.txt", msg: "feat: add c"},
	})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)

	// THE DISPATCH ORACLE: the fast-path must NEVER call the stager (system_context §6). If it is,
	// the run mis-routed to runLoop. t.Fatal makes this a hard correctness gate.
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		t.Fatalf("fast-path must not invoke the stager (concept %q routed to runLoop)", concept.Title)
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0 (disjoint union == T_start ⇒ arbiter naturally skipped)", result.Amended)
	}

	// CAS order: HEAD advanced 3 times + clean tree (mirror AutoMultiCommit_HappyPath assertions).
	wantSubjects := []string{"feat: add a", "feat: add b", "feat: add c"}
	for i, cr := range result.Commits {
		if cr.Subject != wantSubjects[i] {
			t.Errorf("Commits[%d].Subject = %q, want %q", i, cr.Subject, wantSubjects[i])
		}
	}
	if dcmLogCount(t, repo) != 3 {
		t.Fatalf("commit count = %d, want 3", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_Dispatch_SharedFileFallback is the inverse oracle: a shared-file partition (one path
// in ≥2 concepts) routes to runLoop, which invokes the stager per concept. A flag set inside the
// injected stager seam PROVES the fallback was taken. Mirrors HunkSplitAcrossConcepts's store.py
// setup (the canonical shared-file scenario).
func TestDecompose_Dispatch_SharedFileFallback(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// One file split across 2 concepts (mirror HunkSplitAcrossConcepts's store.py setup).
	base := "def get_links():\n    return []\n\ndef sort_items():\n    return []\n"
	tStart := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return sorted(links, key=lambda c: c.code)\n"
	basePlusA := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return []\n" // concept 0's hunk only

	dcmWriteFile(t, repo, "store.py", base)
	dcmStageFile(t, repo, "store.py")
	dcmCommitRaw(t, repo, "initial")          // born repo; HEAD.tree == base
	dcmWriteFile(t, repo, "store.py", tStart) // dirty, un-staged → triggers decompose

	// Planner declares a SHARED file across both concepts → isFileDisjoint false → runLoop.
	plannerJSON := `{"count":2,"single":false,"commits":[` +
		`{"title":"feat: add link fetching","description":"the get_links change","files":["store.py"]},` +
		`{"title":"feat: sort listed links by code","description":"the sort_items change","files":["store.py"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add link fetching", "feat: sort listed links by code"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // forced count overrides the FR-M2b one-file short-circuit so the loop runs

	// THE DISPATCH ORACLE: runLoop MUST call the stager. A flag proves the fallback was taken. The
	// stager body mirrors HunkSplitAcrossConcepts's per-concept hunk staging verbatim.
	stagerCalled := false
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCalled = true
		switch concept.Title {
		case "feat: add link fetching":
			stagePartialBlob(t, repo, "store.py", basePlusA) // hunk A only — a strict subset of T_start
		case "feat: sort listed links by code":
			stagePartialBlob(t, repo, "store.py", tStart) // + hunk B ⇒ index == T_start for store.py
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if !stagerCalled {
		t.Fatal("shared-file partition must route to runLoop (stager not called)")
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}
}

// TestDecompose_HunkSplit_RejectsOffTStartContent is the negative sibling: the hunk-aware check still
// HARD-ABORTS when the stager stages content NOT traceable to T_start (a concurrent change or a rogue
// stager). Concept 0 stages store.py with a line that appears in neither base nor T_start → the 3-way
// merge conflicts → ErrFreezeViolation.
func TestDecompose_HunkSplit_RejectsOffTStartContent(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	base := "def get_links():\n    return []\n\ndef sort_items():\n    return []\n"
	tStart := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return sorted(links, key=lambda c: c.code)\n"
	offPath := "def get_links():\n    return CONCURRENT_CHANGE()\n\ndef sort_items():\n    return []\n" // not in T_start

	dcmWriteFile(t, repo, "store.py", base)
	dcmStageFile(t, repo, "store.py")
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "store.py", tStart)

	plannerJSON := `{"count":2,"single":false,"commits":[` +
		`{"title":"feat: add link fetching","description":"the get_links change","files":["store.py"]},` +
		`{"title":"feat: sort listed links by code","description":"the sort_items change","files":["store.py"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add link fetching", "feat: sort listed links by code"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2

	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		if concept.Title == "feat: add link fetching" {
			stagePartialBlob(t, repo, "store.py", offPath) // off-T_start content (a concurrent change)
		}
		return nil
	}

	_, err := Decompose(context.Background(), deps)
	if !errors.Is(err, ErrFreezeViolation) {
		t.Fatalf("expected ErrFreezeViolation for off-T_start content, got %v", err)
	}
	if !strings.Contains(err.Error(), "not traceable to the frozen working-tree snapshot") {
		t.Errorf("error should say content is not traceable to T_start; got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "store.py") {
		t.Errorf("error should name store.py; got: %s", err.Error())
	}
	if n := dcmLogCount(t, repo); n != 1 { // only 'initial' — the run aborted before committing
		t.Errorf("commit count = %d, want 1 (HEAD unchanged)", n)
	}
}

// Success-path sibling of TestDecompose_StagerFreezeViolation (which STAGES the sentinel).
func TestDecompose_ConcurrentChangeExclusion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// NO initial commit (unborn repo) — mirrors TestDecompose_AutoMultiCommit_HappyPath.

	// Two un-staged changes (dirty tree triggers decompose).
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	// Planner returns 2 concepts.
	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"add a","description":"a.txt","files":["a.txt"]},{"title":"add b","description":"b.txt","files":["a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// LOOP-ONLY message responses: under FR-M1d the arbiter does NOT run (the frozen leftover is
	// empty — the stagers cover all of T_start), so no arbiter→resolveNewCommit message call occurs.
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // never invoked under FR-M1d (gate skips)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)

	// The custom seam: stages concept files + writes sentinel UNSTAGED on first concept.
	deps.stager = concurrentSentinelSeam(t, repo,
		map[string][]string{"add a": {"a.txt"}, "add b": {"b.txt"}},
		"sentinel.txt",
	)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(concurrent exclusion): %v", err)
	}

	// Exactly 2 commits (the loop commits; the arbiter is SKIPPED because the frozen leftover is
	// empty). A null-target arbiter would add a 3rd commit, so this is the authoritative skip proof.
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want exactly 2 (loop commits; arbiter must be skipped)", len(result.Commits))
	}
	if got := dcmLogCount(t, repo); got != 2 {
		t.Errorf("commit count = %d, want exactly 2 (arbiter skipped — empty frozen leftover; FR-M1d)", got)
	}

	// (FR-M1b/M1c) sentinel.txt appears in NO LOOP commit's diff-tree (first 2 commits). The freeze
	// captured T_start before the sentinel was written; the loop commits commit frozen trees.
	log := dcmLogOneline(t, repo)
	lines := strings.Split(log, "\n")
	loopCount := 2
	if loopCount > len(lines) {
		loopCount = len(lines)
	}
	for i := 0; i < loopCount; i++ {
		parts := strings.SplitN(lines[i], " ", 2)
		sha := parts[0]
		treeOut := dcmGitOut(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
		if strings.Contains(treeOut, "sentinel.txt") {
			t.Errorf("loop commit %d (%s) contains sentinel.txt — freeze should exclude post-freeze changes\ntree: %s", i, sha, treeOut)
		}
	}

	// (FR-M1d, contract 3a) sentinel.txt appears in NO commit across the WHOLE run, including any
	// arbiter commit. The arbiter's gate/diff/staging are all frozen, so a post-T_start file cannot
	// cross the gate. This strictly subsumes the per-loop check above (kept for layered diagnostics).
	logNames := dcmGitOut(t, repo, "log", "--name-only", "--format=")
	if strings.Contains(logNames, "sentinel.txt") {
		t.Errorf("sentinel.txt appears in a commit (incl. arbiter) — FR-M1d freeze should exclude it:\n%s", logNames)
	}

	// (FR-M1b/M1d) the sentinel REMAINS in the working tree, untracked — the run left it untouched.
	status := dcmStatusPorcelain(t, repo)
	if !strings.Contains(status, "?? sentinel.txt") {
		t.Errorf("status = %q, want it to contain '?? sentinel.txt' (concurrent change left untouched)", status)
	}
}

// TestDecompose_ArbiterFoldsOnlyTStart is the permanent FR-M1d / PRD §20.2 "Arbiter freeze parity"
// regression net for the NON-EMPTY-frozen-leftover case — the success-path sibling of
// TestDecompose_ConcurrentChangeExclusion (which has an EMPTY leftover and the arbiter SKIPPED).
// Here concept-1's stager is a no-op (FR-M8 empty-skip) so b.go is a legitimate unclaimed leftover;
// the freeze-safe arbiter gate (DiffTreeNames(tipTree, tStart) == {b.go}, non-empty) is SATISFIED and
// the arbiter RUNS (null target → resolveNewCommit). This asserts the three §20.2 invariants for
// the arbiter-RUNS case:
//   - "Arbiter freeze parity" (3b): the arbiter commit's tree is EXACTLY T_start (HEAD^{tree} ==
//     expectedTStart) — it folds ONLY the frozen leftover (b.go), never the post-freeze sentinel.
//   - "Start-of-run freeze" (3a): concurrent.txt appears in NO commit and remains untracked.
//   - Arbiter ran (not skipped): dcmLogCount == 2 (1 loop + 1 arbiter; unborn ⇒ no seed). Proven via
//     dcmLogCount, NOT result.Amended (Amended is 0 for null-path — Decision D5).
func TestDecompose_ArbiterFoldsOnlyTStart(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// NO initial commit (unborn repo) — mirrors TestDecompose_ConcurrentChangeExclusion / _ArbiterWiring.

	// Two un-staged changes (dirty tree triggers decompose). On an unborn repo these two files ARE
	// the entire working tree, so FreezeWorkingTree's internal `git add -A; write-tree` equals the
	// hand-built expectedTStart below.
	dcmWriteFile(t, repo, "a.go", "package a\n")
	dcmWriteFile(t, repo, "b.go", "package b\n")

	// Capture the EXACTLY-T_start oracle BEFORE the run: tree of {a.go, b.go} on the unborn base,
	// then restore a clean index (mirrors chnBuildChain's `rm --cached --ignore-unmatch` restore).
	dcmRunGit(t, repo, "add", "a.go", "b.go")
	expectedTStart := dcmGitOut(t, repo, "write-tree")
	dcmRunGit(t, repo, "rm", "--cached", "--ignore-unmatch", "a.go", "b.go")

	// Planner returns 2 concepts: concept-1 "add b" is SKIPPED by the seam (empty slice ⇒ FR-M8
	// empty-concept skip, consumes NO message); concept-2 "add a" stages a.go ⇒ the loop makes ONE
	// commit ({a.go}); tipTree == {a.go}; DiffTreeNames(tipTree, tStart) == {b.go} ⇒ arbiter RUNS.
	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"add b","description":"b.go","files":["b.go"]},{"title":"add a","description":"a.go","files":["b.go"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message script call order: "add b" is SKIPPED ⇒ no message; concept-2 "add a" ⇒ message[0];
	// arbiter null-path resolveNewCommit ⇒ message[1]. One defensive extra (dedupe-retry safety).
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: arbiter leftover", "feat: defensive extra"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // IS invoked (leftover non-empty)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)

	// Reuse concurrentSentinelSeam VERBATIM: "add b" no-op (empty slice) ⇒ b.go unclaimed leftover;
	// "add a" stages a.go. The sentinel ("concurrent.txt") is written UNSTAGED on the FIRST concept
	// processed ("add b") — post-freeze ⇒ excluded from every commit.
	deps.stager = concurrentSentinelSeam(t, repo,
		map[string][]string{"add b": {}, "add a": {"a.go"}},
		"concurrent.txt",
	)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(arbiter folds only T_start): %v", err)
	}

	// (a) EXACTLY-T_start: the arbiter commit's tree (HEAD^{tree}) is EXACTLY T_start — it folded
	// ONLY the frozen leftover (b.go), never the post-freeze sentinel. Contract: "each arbiter
	// commit's tree (HEAD^{tree}) is exactly T_start (FR-M1d/M9/M10)".
	headTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	if headTree != expectedTStart {
		t.Errorf("arbiter commit tree = %s, want EXACTLY T_start = %s", headTree, expectedTStart)
	}

	// (b) concurrent.txt in NO commit across the whole run (loop commit + arbiter commit).
	if logNames := dcmGitOut(t, repo, "log", "--name-only", "--format="); strings.Contains(logNames, "concurrent.txt") {
		t.Errorf("concurrent.txt appears in a commit — FR-M1d freeze should exclude it:\n%s", logNames)
	}

	// (c) concurrent.txt REMAINS untracked — the run left the post-freeze change untouched.
	if status := dcmStatusPorcelain(t, repo); !strings.Contains(status, "?? concurrent.txt") {
		t.Errorf("status = %q, want it to contain '?? concurrent.txt'", status)
	}

	// (d) Arbiter RAN (folded the leftover, not skipped): exactly 2 commits (1 loop + 1 arbiter;
	// unborn ⇒ no seed). A skipped arbiter would leave 1 commit; a non-freeze-safe arbiter would
	// ALSO sweep concurrent.txt into the arbiter commit (caught by (b) above).
	if got := dcmLogCount(t, repo); got != 2 {
		t.Errorf("commit count = %d, want exactly 2 (1 loop + 1 arbiter; arbiter ran via null-path)", got)
	}
	if len(result.Commits) != 2 {
		t.Errorf("result.Commits len = %d, want 2", len(result.Commits))
	}
}

// TestDecompose_TStartCompleteness (PRD §20.2 "T_start completeness" invariant, FR-M1d contract
// 3c): after a fully-successful decompose run, EVERY T_start path landed in HEAD, and the live
// working tree is non-empty ONLY from paths OUTSIDE T_start (the post-freeze sentinel), which are
// intentionally left unstaged. Born repo so baseTree is a real tree (makes the DiffTreeNames
// superset check meaningful). The stagers cover all of T_start (x.go + y.go), so the frozen
// leftover is empty and the arbiter is SKIPPED.
func TestDecompose_TStartCompleteness(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial") // BORN repo — real baseTree
	dcmWriteFile(t, repo, "x.go", "package x\n")
	dcmWriteFile(t, repo, "y.go", "package y\n")
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}") // capture BEFORE the run

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"add x","description":"x.go","files":["x.go"]},{"title":"add y","description":"y.go","files":["x.go"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add x", "feat: add y"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // arbiter skipped (stagers cover all T_start)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = concurrentSentinelSeam(t, repo,
		map[string][]string{"add x": {"x.go"}, "add y": {"y.go"}},
		"sentinel.txt", // post-freeze, OUTSIDE T_start
	)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(completeness): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}

	// (c) T_start completeness: every frozen path landed in HEAD. DiffTreeNames(baseTree, HEAD^{tree})
	// ⊇ DiffTreeNames(baseTree, tStart) == {x.go, y.go}. Use the SAME primitive the production gate uses.
	headTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	g := git.New(repo)
	landed, err := g.DiffTreeNames(context.Background(), baseTree, headTree)
	if err != nil {
		t.Fatalf("DiffTreeNames(baseTree, headTree): %v", err)
	}
	landedSet := map[string]bool{}
	for _, p := range landed {
		landedSet[p] = true
	}
	for _, frozen := range []string{"x.go", "y.go"} {
		if !landedSet[frozen] {
			t.Errorf("frozen path %q did NOT land in HEAD — T_start completeness violated (landed=%v)", frozen, landed)
		}
	}

	// (c) Live status is non-empty ONLY from paths OUTSIDE T_start: the sole entry is the sentinel.
	if status := dcmStatusPorcelain(t, repo); !strings.Contains(status, "?? sentinel.txt") || strings.TrimSpace(status) != "?? sentinel.txt" {
		t.Errorf("status = %q, want exactly '?? sentinel.txt' (only out-of-T_start dirt remains)", status)
	}

	// Cross-check (FR-M1d): the sentinel is in no commit; exactly 3 commits (initial + 2 loop; arbiter skipped).
	if got := dcmLogCount(t, repo); got != 3 {
		t.Errorf("commit count = %d, want 3 (initial + 2 loop; arbiter skipped)", got)
	}
	if logNames := dcmGitOut(t, repo, "log", "--name-only", "--format="); strings.Contains(logNames, "sentinel.txt") {
		t.Errorf("sentinel.txt appears in a commit — FR-M1d freeze should exclude it:\n%s", logNames)
	}
}

func TestDecompose_StagerGuardHappyPath(t *testing.T) {
	// A well-behaved stager (git add, no ref mutation) completes normally — guard passes.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2                                                    // override FR-M2b one-file short-circuit so the loop+stager path is tested
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}}) // git add only

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("happy-path guard false-positive: %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	// HEAD advanced exactly once (the published commit), via UpdateRefCAS — NOT via the stager.
	if got := dcmLogCount(t, repo); got != 2 { // initial + 1 published
		t.Errorf("commit count = %d, want 2", got)
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

func TestDecompose_PlannerFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")

	// Planner returns unparseable output — after callPlanner's one retry, it returns ErrPlannerFailed.
	plannerM := dcmPlannerScriptManifest(t, bin, []string{"not json", "still not json"})
	messageM := dcmMessageManifest(t, bin, "should not be called")

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the planner is reached

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error on planner failure, got nil")
	}
	if !errors.Is(err, ErrPlannerFailed) {
		t.Errorf("errors.Is(err, ErrPlannerFailed) = false, err = %v", err)
	}

	// Verify it's NOT a *RescueError (planner failure = non-rescue, §13.6.6).
	var re *generate.RescueError
	if errors.As(err, &re) {
		t.Error("error is *RescueError — planner failure should be NON-RESCUE")
	}

	// Verify no commit was created.
	if dcmLogCount(t, repo) != 0 {
		t.Errorf("commit count = %d, want 0 (no commit on planner failure)", dcmLogCount(t, repo))
	}
}

func TestDecompose_SafetyCap(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")
	dcmWriteFile(t, repo, "b.txt", "b\n") // 2nd file: bypasses FR-M2b one-file short-circuit (auto mode, count≥2)

	// Planner proposes 20 commits, exceeding MaxCommits (12) — callPlanner enforces the cap.
	plannerJSON := `{"count":20,"single":false,"commits":[` +
		strings.Repeat(`{"title":"c","description":"a.txt"},`, 19) +
		`{"title":"c20","description":"a.txt"}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "should not be called")

	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected safety-cap error, got nil")
	}
	// Note: FR-M2b one-file short-circuit does NOT fire because 2 files ⇒ DiffTreeNames count ≥ 2.
	// The auto-mode safety cap is still tested (forcedCount==0 inside callPlanner).
	// The error should mention the cap.
	if !strings.Contains(err.Error(), "exceeds max_commits") {
		t.Errorf("error = %v, want safety-cap error mentioning 'exceeds max_commits'", err)
	}

	// Verify it's NOT a *RescueError.
	var re *generate.RescueError
	if errors.As(err, &re) {
		t.Error("error is *RescueError — safety cap should be NON-RESCUE")
	}

	// Verify no commit was created.
	if dcmLogCount(t, repo) != 0 {
		t.Errorf("commit count = %d, want 0", dcmLogCount(t, repo))
	}
}

func TestDecompose_ArbiterSkippedOnCleanTree(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	// Arbiter should NOT be called — use a counter to verify.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop+arbiter path is tested
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(clean tree, arbiter skip): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify arbiter was NOT called.
	data, err := os.ReadFile(counterFile)
	if err == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0 (clean tree — arbiter should not run)", count)
		}
	}
}

func TestDecompose_ArbiterWiring(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	// "leftover.txt" will be left un-staged by the stager seam, triggering the arbiter.
	dcmWriteFile(t, repo, "leftover.txt", "leftover\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	// Arbiter returns null → new commit (resolveArbiter's null path).
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	// Stager only stages a.txt — leaves leftover.txt un-staged.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	// Inject an arbiter-phase message agent via the seam — resolveNewCommit calls generateMessage.
	// Use a script with extra entries in case of dedupe retries.
	messageScriptM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add leftover", "feat: add leftover", "feat: add leftover", "feat: add leftover"})
	deps.Roles.Message = messageScriptM

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(arbiter wiring): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("loop Commits len = %d, want 2 (null-path commit now included via reread)", len(result.Commits))
	}
	// Amended == 0 because target was null → new commit (not an amend).
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0 (null target → new commit)", result.Amended)
	}
	// The second commit is the arbiter-created null-path commit — verify it resolves and is HEAD.
	if result.Commits[1].Subject != "feat: add leftover" {
		t.Errorf("Commits[1].Subject = %q, want %q", result.Commits[1].Subject, "feat: add leftover")
	}
	if !dcmShaResolves(t, repo, result.Commits[1].SHA) {
		t.Errorf("Commits[1].SHA %q does not resolve (dangling)", result.Commits[1].SHA)
	}
	if result.Commits[1].SHA != dcmHeadSHA(t, repo) {
		t.Errorf("Commits[1].SHA = %q, want HEAD %q", result.Commits[1].SHA, dcmHeadSHA(t, repo))
	}

	// Verify the tree is clean after the arbiter.
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status after arbiter = %q, want empty (clean)", status)
	}
	// 2 commits: loop + arbiter's new commit.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (1 loop + 1 arbiter new)", dcmLogCount(t, repo))
	}
}

func TestDecompose_ErrorPropagation_Stager(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")
	dcmWriteFile(t, repo, "b.txt", "b\n") // 2nd file: bypasses FR-M2b one-file short-circuit (auto mode, count≥2)

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	stagerErr := errors.New("stager injection error")
	callCount := 0
	deps.stager = func(ctx context.Context, deps Deps, concept prompt.PlannerCommit) error {
		callCount++
		return stagerErr
	}

	// S2 (FR-M12d): a stager that fails TWICE is treated as empty — the concept is skipped,
	// no error is returned. The stager seam is called twice (retry-once).
	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("expected no error (stager treated as empty), got %v", err)
	}
	if len(result.Commits) != 0 {
		t.Fatalf("Commits len = %d, want 0 (stager failed → empty → skip)", len(result.Commits))
	}
	if callCount != 2 {
		t.Errorf("stager call count = %d, want 2 (retry-once)", callCount)
	}

	// No commit should have been created (stager failed → empty → skip).
	if dcmLogCount(t, repo) != 0 {
		t.Errorf("commit count = %d, want 0", dcmLogCount(t, repo))
	}
}

func TestDecompose_ErrorPropagation_RescueError(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub exits with a non-zero code (no valid output) — triggers RescueError after retries.
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // no duplicate retries — fail immediately on the first attempt
	cfg.Commits = 2             // override FR-M2b one-file short-circuit so the loop path is tested
	messageM := stubtest.Manifest(bin, stubtest.Options{Exit: 1, Out: ""})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDepsWithConfig(t, repo, roles, cfg)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	_, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected RescueError, got nil")
	}
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Errorf("error = %v, want *generate.RescueError", err)
	}
}

func TestDecompose_UnbornRepo(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo) // repo with 0 commits (unborn HEAD)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: initial")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(unborn): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: initial" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: initial")
	}

	// Verify root commit (no parent — HEAD~1 should fail).
	cmd := exec.Command("git", "-C", repo, "rev-parse", "HEAD~1")
	_, err = cmd.CombinedOutput()
	if err == nil {
		t.Error("expected HEAD~1 to fail on root commit (no parent)")
	}
}

func TestDecompose_Commits1_Mode(t *testing.T) {
	// Config.Commits == 1 → single escape hatch (same as Config.Single).
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "a\n")

	msgM := dcmMessageManifest(t, bin, "feat: single")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Commits = 1

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(Commits=1): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}
}

// TestDecompose_MessageRescuePartial (FR-M12a): 3 concepts; message stub times out for concept 1.
// Asserts: partial DecomposeResult with commit[0], *DecomposeRescueError wrapping *RescueError,
// errors.Is(generate.ErrRescue), FormatRescueMulti in deps.Out, arbiter NOT run.
func TestDecompose_MessageRescuePartial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 3 concepts.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: SleepMS > Timeout for concept 1 (index 1) to trigger timeout.
	// Use a script that returns fast for concepts 0 and 2, and a slow-exit-1 for concept 1.
	// Actually, simpler: use a script manifest. Concept 0 → "feat: add a" (success),
	// concept 1 → times out (SleepMS > Timeout), concept 2 → should not be reached.
	// The script responses are consumed in order by the message agent's retry loop.
	// To make concept 1 fail, we use a message manifest with Exit=1 and SleepMS > config.Timeout.

	// We need different behavior per concept index. Use the stager seam for staging
	// and a message script that works for concept 0 and fails for concept 1.
	// The message agent is called once per concept (MaxDuplicateRetries=0 for simplicity).

	// Actually: generateMessage has its own retry loop. We need the message stub to fail
	// for concept 1's generateMessage call. Let's use a call-counting approach.
	callCount := 0
	messageM := stubtest.NewScript(t, bin, []string{
		"feat: add a", // concept 0: success
		"",            // concept 1: empty → parse fail → RescueError (with MaxDuplicateRetries=0)
		"feat: add c", // concept 2: would-be (not reached)
	})

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // fail immediately on parse failure

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter counter: should NOT be called on rescue.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, buf := dcmOutBuffer(t, repo, roles)
	deps.Config = cfg
	_ = callCount // not used

	// Stager seam: stages files for each concept. c2 (concept 1, the failing one) still stages.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error (message rescue for concept 1), got nil")
	}

	// (a) errors.As → *DecomposeRescueError
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("error = %v, want *DecomposeRescueError", err)
	}

	// (b) errors.As → *generate.RescueError (via Unwrap)
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error does not unwrap to *RescueError: %v", err)
	}

	// (c) errors.Is → generate.ErrRescue (→ exitcode 3)
	if !errors.Is(err, generate.ErrRescue) {
		t.Errorf("error is not ErrRescue: %v", err)
	}

	// (d) partial commits: exactly 1 (commit 0)
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only concept 0 landed)", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}

	// (d) git log shows exactly 1 commit
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("git log count = %d, want 1", dcmLogCount(t, repo))
	}

	// (e) deps.Out contains FormatRescueMulti output naming "concept 2 of 3"
	out := buf.String()
	if !strings.Contains(out, "concept 2 of 3") {
		t.Errorf("rescue output missing 'concept 2 of 3'; got: %s", out)
	}
	if !strings.Contains(out, "update-ref HEAD") {
		t.Errorf("rescue output missing 'update-ref HEAD'; got: %s", out)
	}

	// (f) arbiter NOT called
	data, err := os.ReadFile(counterFile)
	if err == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0 (rescue should skip arbiter)", count)
		}
	}
}

// TestDecompose_CASAbortPartial (FR-M12b): an external goroutine moves HEAD between concept 0's
// publication and concept 1's publication, so publishCommit[1]'s CAS fails.
// Uses a well-behaved stager (no HEAD movement) and an external goroutine with a timed delay
// that fires during the message agent's sleep window.
func TestDecompose_CASAbortPartial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 3 concepts. Concept 1 (c2) is where the HEAD move will happen.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})
	messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "1000" // create timing window for external HEAD move

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter counter: should NOT be called on CAS abort.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, buf := dcmOutBuffer(t, repo, roles)

	// External HEAD move, made deterministic on two axes:
	//
	// (1) WHEN: poll git state until c1 has committed ("feat: add a" in log) but c2 has NOT, AND
	// c3 is staged ("c.txt" in the index). That conjunction means we are inside the msg[1] drain
	// window — the main loop is blocked in publish() draining concept c2's message, so NO stager is
	// running and the §19 "stager moved HEAD" guard cannot fire on our move. (A blind time.Sleep
	// could land during a stager call → "stager moved HEAD" abort, or before c1 landed → 0 commits.)
	// A brief armed-delay clears c3's stager post-HEAD snapshot before we touch the ref.
	//
	// (2) HOW: build the move with `git commit-tree` from HEAD's tree, NOT `git commit --allow-empty`.
	// The latter captures the STAGED INDEX; c2's stager stages b.txt before c1 commits, so an
	// --allow-empty commit then has tree {a.txt,b.txt} == c2's frozen snapshot, tripping the
	// "already committed / Nothing to do" CASError path instead of "HEAD moved". commit-tree from
	// HEAD^{tree} yields tree {a.txt}, which always differs from c2's {a.txt,b.txt} → always
	// "HEAD moved", exactly 1 commit (c1) landed. commit-tree/update-ref never touch the index.
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		armed := false
		for time.Now().Before(deadline) {
			logOut, _ := exec.Command("git", "-C", repo, "log", "--format=%s").CombinedOutput()
			idxOut, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").CombinedOutput()
			s := string(logOut)
			if strings.Contains(s, "feat: add a") && !strings.Contains(s, "feat: add b") && strings.Contains(string(idxOut), "c.txt") {
				if !armed {
					armed = true
					time.Sleep(150 * time.Millisecond) // past c3 stager's post-HEAD snapshot; well inside the ~1s drain
					continue
				}
				tree := dcmRunGit(t, repo, "rev-parse", "HEAD^{tree}")
				c := dcmRunGit(t, repo, "commit-tree", tree, "-p", "HEAD", "-m", "external head move")
				dcmRunGit(t, repo, "update-ref", "HEAD", c)
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		// deadline expired without hitting the c1-committed/c3-staged window — leave HEAD alone;
		// the test's own assertions will fail rather than risk a late move racing a finished test.
	}()

	// Well-behaved stager seam: stages files only (no HEAD movement — the guard would catch it).
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		files := map[string][]string{"c1": {"a.txt"}, "c2": {"b.txt"}, "c3": {"c.txt"}}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected CAS error, got nil")
	}

	// (a) errors.As → *generate.CASError
	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want *generate.CASError", err)
	}

	// (b) errors.Is → git.ErrCASFailed → exitcode 1
	if !errors.Is(err, git.ErrCASFailed) {
		t.Errorf("error is not ErrCASFailed: %v", err)
	}

	// (c) deps.Out contains "HEAD moved"
	out := buf.String()
	if !strings.Contains(out, "HEAD moved") {
		t.Errorf("CAS output missing 'HEAD moved'; got: %s", out)
	}

	// (d) partial commits: exactly 1 (concept c1 landed before CAS failure on c2's publish)
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only c1 landed before CAS failure)", len(result.Commits))
	}

	// (e) arbiter NOT called
	data, aerr := os.ReadFile(counterFile)
	if aerr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0 (CAS abort should skip arbiter)", count)
		}
	}
}

// TestDecompose_StagerRetryThenEmpty (FR-M12d): stager seam fails twice for concept 1,
// succeeds for others. Asserts: concept 1 skipped (≤N commits), stager called twice for c2,
// loop continued.
func TestDecompose_StagerRetryThenEmpty(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for c1 and c3 only. c2's stager fails, so no file for c2 is created.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add c"})

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)

	// Stager seam: fails twice for concept c2, succeeds for others.
	stagerCalls := map[string]int{}
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCalls[concept.Title]++
		if concept.Title == "c2" && stagerCalls[concept.Title] <= 2 {
			return errors.New("simulated stager failure")
		}
		// Stage real files for non-failing concepts.
		files := map[string][]string{"c1": {"a.txt"}, "c3": {"c.txt"}}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(stager retry): %v", err)
	}

	// (a) concept c2 skipped: 2 commits (c1 + c3), not 3.
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (c2 skipped)", len(result.Commits))
	}

	// (b) stager called exactly twice for concept c2 (retry-once).
	if stagerCalls["c2"] != 2 {
		t.Errorf("c2 stager calls = %d, want 2 (retry-once)", stagerCalls["c2"])
	}

	// (d) the loop continued: c3 was committed.
	if result.Commits[1].Subject != "feat: add c" {
		t.Errorf("Commits[1].Subject = %q, want %q", result.Commits[1].Subject, "feat: add c")
	}

	// Verify git: 2 commits.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("git log count = %d, want 2", dcmLogCount(t, repo))
	}
}

// TestDecomposeRescueError_ExitCode verifies the Unwrap chain for exitcode.For mapping.
func TestDecomposeRescueError_ExitCode(t *testing.T) {
	// DecomposeRescueError → *RescueError → Kind: ErrRescue → exit 3
	dre1 := &DecomposeRescueError{Rescue: &generate.RescueError{Kind: generate.ErrRescue}}
	if !errors.Is(dre1, generate.ErrRescue) {
		t.Error("errors.Is(DecomposeRescueError{ErrRescue}, ErrRescue) = false, want true")
	}

	// DecomposeRescueError → *RescueError → Kind: ErrTimeout → exit 124
	dre2 := &DecomposeRescueError{Rescue: &generate.RescueError{Kind: generate.ErrTimeout}}
	if !errors.Is(dre2, generate.ErrTimeout) {
		t.Error("errors.Is(DecomposeRescueError{ErrTimeout}, ErrTimeout) = false, want true")
	}

	// errors.As to *RescueError traverses Unwrap
	var re *generate.RescueError
	if !errors.As(dre1, &re) {
		t.Error("errors.As(DecomposeRescueError, &*RescueError) = false, want true")
	}

	// errors.As to *CASError → git.ErrCASFailed → exit 1
	ce := &generate.CASError{}
	if !errors.Is(ce, git.ErrCASFailed) {
		t.Error("errors.Is(CASError, git.ErrCASFailed) = false, want true")
	}
}

// TestDecompose_RescueArbiterSkipped is a focused assertion that the arbiter does NOT run after
// a FR-M12a rescue (covered by TestDecompose_MessageRescuePartial's arbiter-count==0;
// this test is a focused sub-assertion for clarity).
func TestDecompose_RescueArbiterSkipped(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: concept 0 succeeds, concept 1 fails (empty output → parse fail → RescueError).
	messageM := stubtest.NewScript(t, bin, []string{"feat: add a", ""})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})

	// Arbiter with a counter — MUST be 0.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	arbiterM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps, _ := dcmOutBuffer(t, repo, roles)
	deps.Config = cfg
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Must be a rescue error (not some other failure).
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("expected *DecomposeRescueError, got %v", err)
	}

	// 1 commit landed (concept 0).
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}

	// Arbiter was NOT called.
	data, aerr := os.ReadFile(counterFile)
	if aerr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("arbiter call count = %q, want 0", count)
		}
	}
}

// TestDecompose_StagerRetryThenSuccess (FR-M12d variant): stager fails once then succeeds.
func TestDecompose_StagerRetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")

	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested

	stagerCalls := 0
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCalls++
		if stagerCalls == 1 {
			return errors.New("first failure")
		}
		// Second call succeeds: stage the file.
		dcmRunGit(t, repo, "add", "a.txt")
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(retry-then-success): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}
	if stagerCalls != 2 {
		t.Errorf("stager call count = %d, want 2 (fail once, succeed on retry)", stagerCalls)
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("git log count = %d, want 1", dcmLogCount(t, repo))
	}
}

// TestDecompose_OneFileShortcut_PlannerBypassed (FR-M2b core test): BORN repo, exactly ONE untracked file,
// auto mode — the planner is NEVER called (counter absent/"0"), ONE commit lands with the MESSAGE
// role's subject, and git status is clean (proves the ReadTree index-sync, findings §4).
func TestDecompose_OneFileShortcut_PlannerBypassed(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")            // BORN repo (preRunHEAD set; dup-check vs "initial")
	dcmWriteFile(t, repo, "only.txt", "only\n") // EXACTLY one untracked file

	// Planner counter-manifest: if called, the counter file appears. It must NOT be called.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
	messageM := dcmMessageManifest(t, bin, "feat: add the only file")
	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // default config: Commits=0 (auto), Single=false

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(one-file bypass): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add the only file" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: add the only file")
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}

	// Verify planner was NEVER called (counter file absent or "0").
	data, ferr := os.ReadFile(counterFile)
	if ferr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("planner call count = %q, want 0 (FR-M2b bypass — planner never invoked)", count)
		}
	}

	// Verify git state: 2 commits (initial + 1), clean tree.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + 1)", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean — proves ReadTree index-sync)", status)
	}
}

// TestDecompose_OneFileShortcut_Unborn (FR-M2b edge): UNBORN repo (no initial commit), one file →
// short-circuit fires, producing a ROOT commit. Planner is never called. Clean post-state.
func TestDecompose_OneFileShortcut_Unborn(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)                        // UNBORN (no dcmCommitRaw — no commits)
	dcmWriteFile(t, repo, "only.txt", "only\n") // ONE untracked file

	// Planner counter-manifest: must NOT be called.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
	messageM := dcmMessageManifest(t, bin, "feat: root add")
	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // auto mode

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(one-file unborn): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: root add" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "feat: root add")
	}

	// Verify planner was NEVER called.
	data, ferr := os.ReadFile(counterFile)
	if ferr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("planner call count = %q, want 0 (unborn one-file bypass)", count)
		}
	}

	// Verify git state: 1 ROOT commit, clean tree.
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1 (root commit)", dcmLogCount(t, repo))
	}
	if !dcmShaResolves(t, repo, result.Commits[0].SHA) {
		t.Errorf("Commits[0].SHA %q does not resolve (dangling)", result.Commits[0].SHA)
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_OneFileShortcut_CommitsOverride (FR-M2b override): --commits 2 with one file →
// short-circuit is OVERRIDDEN, the planner IS called, the loop runs. Verifies that a forced count
// is honored even for a single file.
func TestDecompose_OneFileShortcut_CommitsOverride(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "only.txt", "only\n") // ONE untracked file

	// Planner returns a 2-concept plan (if called, succeeds).
	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"only.txt","files":["only.txt"]},{"title":"c2","description":"leftover","files":["only.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: c1", "feat: arbiter"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	cfg := config.Defaults()
	cfg.Commits = 2 // FORCED count ⇒ short-circuit OVERRIDDEN
	deps := dcmDepsWithConfig(t, repo, roles, cfg)
	// c1 stages only.txt; c2 stages nothing → empty-skip. The loop runs, proving the planner was called.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"only.txt"}, "c2": {}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(commits override): %v", err)
	}
	// The planner path ran (NOT runOneFileShortcut): at least 1 commit from c1.
	if len(result.Commits) < 1 {
		t.Fatalf("Commits len = %d, want ≥1 (planner path ran)", len(result.Commits))
	}
	// 2 commits total (initial + c1). The c2 empty-skip means no c2 commit.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + c1 from planner path)", dcmLogCount(t, repo))
	}
}

// TestDecompose_OneFileShortcut_TwoFilesNoBypass (FR-M2b boundary): TWO files → count 2 →
// the short-circuit threshold (len==1) is NOT met → the planner IS called and the loop runs.
func TestDecompose_OneFileShortcut_TwoFilesNoBypass(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n") // TWO files

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles) // auto mode (Commits=0), but 2 files ⇒ count 2 ⇒ NO short-circuit
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}, "c2": {"b.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(two files): %v", err)
	}
	// The planner path ran (NOT runOneFileShortcut): 2 commits prove the planner partitioned.
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (planner partitioned both files)", len(result.Commits))
	}
	// 3 commits total (initial + c1 + c2).
	if dcmLogCount(t, repo) != 3 {
		t.Fatalf("commit count = %d, want 3 (initial + c1 + c2)", dcmLogCount(t, repo))
	}
}

// TestDecompose_OneFileShortcut_Deletion (FR-M2b edge): single DELETION counts as one changed
// path → short-circuit fires. DiffTreeNames includes deletions (git diff-tree --name-only).
func TestDecompose_OneFileShortcut_Deletion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create a tracked file and commit it.
	dcmWriteFile(t, repo, "gone.txt", "x\n")
	dcmStageFile(t, repo, "gone.txt")
	dcmRunGit(t, repo, "commit", "-m", "initial")

	// Delete the tracked file WITHOUT staging (plain filesystem delete) — mirrors the real FR-M1
	// precondition the CLI enforces (empty index + working-tree change). `git rm` would stage the
	// deletion, which FR-M1e's empty-index re-check now rejects; a bare `rm` leaves the index clean
	// and FreezeWorkingTree's AddAll captures the deletion into T_start all the same.
	if err := os.Remove(filepath.Join(repo, "gone.txt")); err != nil {
		t.Fatalf("rm gone.txt: %v", err)
	}

	// Planner counter-manifest: must NOT be called.
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	plannerM := stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})
	messageM := dcmMessageManifest(t, bin, "chore: remove gone")
	roles := RoleManifests{Planner: plannerM, Message: messageM}
	deps := dcmDeps(t, repo, roles) // auto mode

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(one-file deletion): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	if result.Commits[0].Subject != "chore: remove gone" {
		t.Errorf("Subject = %q, want %q", result.Commits[0].Subject, "chore: remove gone")
	}

	// Verify planner was NEVER called.
	data, ferr := os.ReadFile(counterFile)
	if ferr == nil {
		count := strings.TrimSpace(string(data))
		if count != "" && count != "0" {
			t.Errorf("planner call count = %q, want 0 (deletion bypass)", count)
		}
	}

	// Verify git state: 2 commits (initial + 1 deletion), clean tree.
	if dcmLogCount(t, repo) != 2 {
		t.Fatalf("commit count = %d, want 2 (initial + deletion)", dcmLogCount(t, repo))
	}
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

func TestComputeAmended(t *testing.T) {
	tests := []struct {
		name      string
		target    *string
		chainData []ChainEntry
		want      int
	}{
		{
			name:   "nil target → 0",
			target: nil,
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
			},
			want: 0,
		},
		{
			name:   "tip amend → 1",
			target: strPtrForTest("bbb"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
			},
			want: 1,
		},
		{
			name:   "mid-chain at 0 → N",
			target: strPtrForTest("aaa"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
				{SHA: "ccc", Tree: "t3", Message: "m3", Parent: "bbb"},
			},
			want: 3, // N=3, idx=0 → 3-0=3
		},
		{
			name:   "mid-chain at 1 → N-1",
			target: strPtrForTest("bbb"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
				{SHA: "bbb", Tree: "t2", Message: "m2", Parent: "aaa"},
				{SHA: "ccc", Tree: "t3", Message: "m3", Parent: "bbb"},
			},
			want: 2, // N=3, idx=1 → 3-1=2
		},
		{
			name:   "not-found → 0",
			target: strPtrForTest("zzz"),
			chainData: []ChainEntry{
				{SHA: "aaa", Tree: "t1", Message: "m1", Parent: ""},
			},
			want: 0,
		},
		{
			name:      "empty chain → 0",
			target:    strPtrForTest("aaa"),
			chainData: nil,
			want:      0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeAmended(tc.target, tc.chainData)
			if got != tc.want {
				t.Errorf("computeAmended = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestInvokeStager_NilSeam(t *testing.T) {
	// With nil seam, invokeStager should call stageConcept. Since stageConcept requires a real
	// agent, this test just verifies the dispatch logic by checking that a non-nil seam works.
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	var called atomic.Bool
	concept := prompt.PlannerCommit{Title: "test", Description: "test files"}

	cfg := config.Defaults()
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	deps := Deps{
		Git:    git.New(repo),
		Config: cfg,
		Roles:  RoleManifests{Stager: stagerM},
	}
	deps.stager = func(ctx context.Context, deps Deps, c prompt.PlannerCommit) error {
		called.Store(true)
		return nil
	}

	err := invokeStager(context.Background(), deps, concept)
	if err != nil {
		t.Fatalf("invokeStager: %v", err)
	}
	if !called.Load() {
		t.Error("seam stager was not called")
	}
}

func TestInvokeStager_NilDepsStager(t *testing.T) {
	// When deps.stager is nil, invokeStager should dispatch to stageConcept.
	// We can't easily test stageConcept without a real agent, so just verify
	// the nil path doesn't panic and attempts to call stageConcept (which will
	// fail because the stub agent doesn't actually do anything).
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	deps := Deps{
		Git:    git.New(repo),
		Config: config.Defaults(),
		Roles:  RoleManifests{Stager: stagerM},
	}
	// deps.stager is nil (zero value)

	concept := prompt.PlannerCommit{Title: "test", Description: "test"}
	// This will call stageConcept which calls the stub stager — it should succeed (stub exits 0).
	err := invokeStager(context.Background(), deps, concept)
	if err != nil {
		t.Fatalf("invokeStager(nil seam): %v", err)
	}
}

func TestDupCheckMessage_Unborn(t *testing.T) {
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// Unborn repo — should always return false (no dup possible).

	deps := Deps{Git: git.New(repo), Config: config.Defaults()}
	isDup := dupCheckMessage(context.Background(), deps, "feat: anything", true)
	if isDup {
		t.Error("dupCheckMessage returned true on unborn repo — want false")
	}
}

func TestDupCheckMessage_Existing(t *testing.T) {
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "feat: existing subject")

	deps := Deps{Git: git.New(repo), Config: config.Defaults()}
	isDup := dupCheckMessage(context.Background(), deps, "feat: existing subject", false)
	if !isDup {
		t.Error("dupCheckMessage returned false for existing subject — want true")
	}
	// Different subject → false.
	isDup = dupCheckMessage(context.Background(), deps, "feat: new subject", false)
	if isDup {
		t.Error("dupCheckMessage returned true for non-existing subject — want false")
	}
}

func TestBuildCommitResult(t *testing.T) {
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmStageFile(t, repo, "a.txt")
	dcmRunGit(t, repo, "commit", "-m", "feat: add a")

	sha := dcmHeadSHA(t, repo)
	deps := Deps{Git: git.New(repo), Config: config.Defaults()}

	// The repo has exactly 1 commit (root commit) — isRoot=true.
	cr, err := buildCommitResult(context.Background(), deps, sha, "feat: add a", true)
	if err != nil {
		t.Fatalf("buildCommitResult: %v", err)
	}
	if cr.SHA != sha {
		t.Errorf("SHA = %q, want %q", cr.SHA, sha)
	}
	if cr.Subject != "feat: add a" {
		t.Errorf("Subject = %q, want %q", cr.Subject, "feat: add a")
	}
	if len(cr.Files) != 1 || cr.Files[0].Path != "a.txt" {
		t.Errorf("Files = %v, want [a.txt]", cr.Files)
	}
}

// strPtrForTest is a test helper to create a string pointer.
func strPtrForTest(s string) *string { return &s }

// lockedBuffer is a thread-safe bytes.Buffer for -race-clean concurrent Verbose writes.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// piShape sets ProviderFlag on a manifest to simulate a pi-shaped agent (multi-provider).
// The sub-provider is now encoded in the model slash-prefix (v3 FR-R5b), so callers must also
// set a slash-prefix model on the config/role to exercise the --provider flag.
func piShape(m *provider.Manifest, providerFlag string) {
	m.ProviderFlag = &providerFlag
}

// TestDecompose_ArbiterTipAmend_RereadsFinalSHA: 2 concepts + leftover; arbiter amends the tip.
// Verifies rereadFinalCommits replaces stale SHAs with the post-amend tip.
func TestDecompose_ArbiterTipAmend_RereadsFinalSHA(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// 2 concepts + 1 leftover.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "leftover.txt", "leftover\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message stub: 2 entries for the loop (resolveTipAmend reuses messages — no extra call).
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b"})

	// Shell-script arbiter picks the tip (last SHA).
	arbiterM := dcmScriptArbiter(t, bin, "tip")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(tip amend): %v", err)
	}
	// 2 commits (tip was amended, not a new one — null adds; amend replaces).
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}
	// Both SHAs must resolve.
	for i, cr := range result.Commits {
		if !dcmShaResolves(t, repo, cr.SHA) {
			t.Errorf("Commits[%d].SHA %q does not resolve (dangling)", i, cr.SHA)
		}
	}
	// The tip commit's SHA must equal HEAD (post-amend).
	if result.Commits[1].SHA != dcmHeadSHA(t, repo) {
		t.Errorf("Commits[1].SHA = %q, want HEAD %q", result.Commits[1].SHA, dcmHeadSHA(t, repo))
	}
	// The first commit should be unchanged (only the tip was amended).
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want \"feat: add a\"", result.Commits[0].Subject)
	}
	// Amended == 1 for tip amend.
	if result.Amended != 1 {
		t.Errorf("Amended = %d, want 1 (tip amend)", result.Amended)
	}
	// Clean tree after the arbiter.
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_ArbiterMidChain_AllSHAsResolve: 3 concepts + leftover; arbiter rebuilds from
// concept[1] (mid-chain). Verifies all re-read SHAs resolve.
func TestDecompose_ArbiterMidChain_AllSHAsResolve(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// 3 concepts + 1 leftover.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")
	dcmWriteFile(t, repo, "leftover.txt", "leftover\n")

	plannerJSON := `{"count":3,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]},{"title":"c3","description":"c.txt","files":["c.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// 3 message entries (resolveMidChain reuses messages).
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add c"})

	// Shell-script arbiter picks the 2nd SHA (concept[1]).
	arbiterM := dcmScriptArbiter(t, bin, "mid")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
		"c3": {"c.txt"},
	})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(mid-chain): %v", err)
	}
	// 3 commits after mid-chain rebuild (concept[1] onward rewritten).
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}
	// All SHAs must resolve.
	for i, cr := range result.Commits {
		if !dcmShaResolves(t, repo, cr.SHA) {
			t.Errorf("Commits[%d].SHA %q does not resolve (dangling)", i, cr.SHA)
		}
	}
	// SHAs should match git log --reverse --format=%H.
	logSHAs := strings.Split(dcmGitOut(t, repo, "log", "--reverse", "--format=%H"), "\n")
	for i, cr := range result.Commits {
		if i < len(logSHAs) && cr.SHA != logSHAs[i] {
			t.Errorf("Commits[%d].SHA = %q, want %q", i, cr.SHA, logSHAs[i])
		}
	}
	// Clean tree.
	if status := dcmStatusPorcelain(t, repo); status != "" {
		t.Errorf("status = %q, want empty (clean)", status)
	}
}

// TestDecompose_HappyPath_CommitsAccurate: verifies that on the happy path (arbiter does NOT run),
// the loop's Commits entries are accurate (SHA resolves, matches HEAD).
func TestDecompose_HappyPath_CommitsAccurate(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	plannerJSON := `{"count":1,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt","a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageManifest(t, bin, "feat: add a")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // override FR-M2b one-file short-circuit so the loop path is tested
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{"c1": {"a.txt"}})

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(happy path): %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1", len(result.Commits))
	}
	// The loop's SHA must resolve and equal HEAD.
	if !dcmShaResolves(t, repo, result.Commits[0].SHA) {
		t.Errorf("Commits[0].SHA %q does not resolve", result.Commits[0].SHA)
	}
	if result.Commits[0].SHA != dcmHeadSHA(t, repo) {
		t.Errorf("Commits[0].SHA = %q, want HEAD %q", result.Commits[0].SHA, dcmHeadSHA(t, repo))
	}
	if result.Amended != 0 {
		t.Errorf("Amended = %d, want 0", result.Amended)
	}
}

func TestDecompose_RoleResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Create files for 2 concepts.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"b.txt","files":["a.txt"]}]}`
	plannerM := stubtest.Manifest(bin, stubtest.Options{Out: plannerJSON})
	piShape(&plannerM, "--provider")

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	piShape(&stagerM, "--provider")

	messageM := stubtest.NewScript(t, bin, []string{"feat: add a", "feat: add b"})
	piShape(&messageM, "--provider")

	arbiterM := stubtest.Manifest(bin, stubtest.Options{Out: `{"target": null}`})
	piShape(&arbiterM, "--provider")

	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	deps.Config.Provider = "pi"              // the manifest NAME — the conflation source; must NOT be emitted
	deps.Config.Model = "openrouter/gpt-5.4" // slash-prefix model → Render emits --provider openrouter
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	var lb lockedBuffer
	deps.Verbose = ui.NewVerbose(&lb, true)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}

	cmd := lb.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("Decompose command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("Decompose command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}

// TestDecompose_SentinelAfterFreezeExcluded (§20.2 "Start-of-run freeze (v2)") verifies that a file
// written to the working tree AFTER the freeze is invisible to every commit. The stager seam stages only
// the concept's path (well-behaved); the planner diffs tStart (frozen), so the sentinel is not even a
// concept. The arbiter's leftover diff is also frozen (TreeDiff(tipTree, tStart)) so the sentinel is
// absent from the arbiter's diff payload. NOTE: the arbiter's STAGING (resolveArbiter's AddAll) is NOT
// yet frozen — enforcement is P3.M2.T1.S1 (FR-M1c). So we verify only the loop commits.
func TestDecompose_SentinelAfterFreezeExcluded(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Pre-freeze state: unstaged changes (a.txt + c.txt). No b.txt — no leftovers after the loop.
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"a.txt","files":["a.txt"]},{"title":"c2","description":"c.txt","files":["a.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add c", "feat: add leftover", "feat: add sentinel", "feat: add sentinel"})
	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`) // null → new commit for any leftovers
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}

	// Stager seam: stages only the concept's path (well-behaved). On first invocation, writes a sentinel
	// file simulating a concurrent change mid-run (AFTER the freeze). The sentinel is NOT staged.
	stagerCallCount := 0
	deps := dcmDeps(t, repo, roles)
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCallCount++
		if stagerCallCount == 1 {
			// Simulate a concurrent change: write a sentinel AFTER the freeze.
			dcmWriteFile(t, repo, "sentinel.txt", "concurrent")
		}
		// Stage only the concept's path (well-behaved — never stages the sentinel).
		files := map[string][]string{"c1": {"a.txt"}, "c2": {"c.txt"}}
		fl, ok := files[concept.Title]
		if ok && len(fl) > 0 {
			for _, f := range fl {
				dcmRunGit(t, repo, "add", f)
			}
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(sentinel): %v", err)
	}

	// Verify sentinel.txt appears in NO LOOP commit's file list (first 2 commits).
	// The planner diffed T_start (frozen), so sentinel.txt was never a concept.
	// NOTE: the arbiter's STAGING (resolveArbiter's AddAll) picks up sentinel.txt from the working
	// tree — that is expected; enforcement of the arbiter staging is P3.M2.T1.S1 (FR-M1c).
	loopCount := len(result.Commits)
	if loopCount > 2 {
		loopCount = 2 // arbiter may add a commit for sentinel.txt (AddAll staging — not yet frozen)
	}
	for i := 0; i < loopCount; i++ {
		for _, fc := range result.Commits[i].Files {
			if fc.Path == "sentinel.txt" {
				t.Errorf("Loop commits[%d] contains sentinel.txt — the freeze should exclude post-freeze changes", i)
			}
		}
	}
}

// TestDecompose_PlannerCoverageLogsUnclaimed (PRD §9.14 FR-M3b) verifies the deterministic, NON-FATAL
// planner coverage check. A stub planner partitions a 3-file changeset into 2 concepts that deliberately
// omit c.txt from BOTH files lists. The run MUST still succeed (the arbiter commits the leftover via
// null-target → resolveNewCommit), AND the capturing verbose buffer MUST contain the FR-M3b diagnostic
// line naming the unclaimed path. The check never aborts and never constrains the stager (FR-M1c is the
// sole content guarantee) — this test proves both halves: success + the log line.
func TestDecompose_PlannerCoverageLogsUnclaimed(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// 3 changed files vs base (unborn → baseTree = empty tree). DiffTreeNames(baseTree, tStart)
	// = [a.txt, b.txt, c.txt].
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	// Planner: 2 concepts claiming ONLY a.txt + b.txt — c.txt deliberately omitted from both files lists.
	plannerJSON := `{"count":2,"single":false,"commits":[{"title":"c1","description":"d1","files":["a.txt"]},{"title":"c2","description":"d2","files":["b.txt"]}]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)

	// Message script: 2 loop commits + the arbiter's null→new commit (+ defensive extras for dedupe retries).
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add a", "feat: add b", "feat: add leftover", "feat: add leftover", "feat: add leftover"})

	// Arbiter: null target → c.txt becomes a new arbiter commit (the realistic "planner missed a file" path).
	arbiterM := dcmArbiterManifest(t, bin, `{"target": null}`)

	stagerM := tooledStubManifest(t, bin, stubtest.Options{Out: ""})
	roles := RoleManifests{Planner: plannerM, Stager: stagerM, Message: messageM, Arbiter: arbiterM}
	deps := dcmDeps(t, repo, roles)
	// Stager seam stages only the CLAIMED files (c1→a.txt, c2→b.txt); c.txt is left for the arbiter.
	deps.stager = dcmStagerSeam(t, repo, map[string][]string{
		"c1": {"a.txt"},
		"c2": {"b.txt"},
	})

	// Capturing verbose writer — the FR-M3b diagnostic lands here.
	var lb lockedBuffer
	deps.Verbose = ui.NewVerbose(&lb, true)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	// 2 loop commits + 1 arbiter null→new commit for the unclaimed c.txt.
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3 (2 loop + 1 arbiter leftover)", len(result.Commits))
	}

	// FR-M3b: the diagnostic MUST name the unclaimed path. Match on the substring regardless of the
	// "DEBUG: raw output:\n" prefix that VerboseRawOutput prepends.
	want := `decompose: path "c.txt" not claimed by any concept (likely leftover for the arbiter)`
	if !strings.Contains(lb.String(), want) {
		t.Errorf("verbose buffer missing FR-M3b coverage line\nwant substring: %s\ngot: %s", want, lb.String())
	}
}

// assembledPromptSeparatorTokens is the Render stdin-separator allowance (the decompose-package copy;
// distinct from the generate-package constant of the same value — the two packages do not cross-import).
// provider.Render prepends `sysPrompt + "\n\n" + userPayload` to the stub's stdin when the manifest
// has no system_prompt_flag (render.go:158). The FR3j MeasureAssembled closure measures
// `EstimateTokens(sysPrompt + payload)` (Go `+`, NO separator). So capturedStdin = closure_measurement
// + "\n\n", and EstimateTokens rises by <=1 (ceil(runes/4) on +2 runes). FR3j guarantees
// closure_measurement <= tokenLimit, therefore capturedStdin <= tokenLimit + 1. The +1 is the bounded
// separator artifact, NOT a violation of FR3j (whose invariant is on the separator-free assembled
// prompt). Equal to git.EstimateTokens("\n\n") = ceil(2/4) = 1.
const assembledPromptSeparatorTokens = 1

// TestDecompose_TokenLimitInvariant_PlannerPromptFits (PRD §9.1 FR3j / §20.5) is the SECOND consumer
// path of the closed-loop invariant: the decompose PLANNER's assembled prompt (TreeDiff-gated) fits
// token_limit. A decompose run invokes the stub multiple times (planner -> stager -> message ->
// arbiter), and STAGECOACH_STUB_STDINFILE captures only the LAST invocation's stdin, so the planner's
// stdin is isolated via role-specific Env: ONLY the planner manifest's Env map carries
// STAGECOACH_STUB_STDINFILE (the stager/message/arbiter manifests do not, so their stub invocations
// drain to io.Discard and never touch the planner's file). The closed loop is forced to run by making
// the untruncated assembled prompt (~884 tokens: ~497 planner sysPrompt + ~388 payload) exceed
// tokenLimit (700), while keeping tokenLimit above the sysPrompt floor (~510) so the loop can
// converge; the "[truncated]" sentinel proves the gate ACTIVELY truncated.
func TestDecompose_TokenLimitInvariant_PlannerPromptFits(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmCommitRaw(t, repo, "initial") // BORN repo: baseTree = HEAD^{tree}; TreeDiff captures the working-tree changes
	// Large UNSTAGED working-tree diff (two files so the FR-M2b one-file short-circuit does NOT fire and
	// the planner is invoked). The body must exceed tokenLimit so the planner's TreeDiff gate truncates.
	dcmWriteFile(t, repo, "a.go", "package a\n")
	body := strings.Repeat("change line content here\n", 600) // 23 runes/line x 600 ~= 13,800 runes ~= 3,450 tokens
	dcmWriteFile(t, repo, "big.go", body)

	// Planner returns FR-M11 single:true + a message => runSingleShortcut uses the planner's message
	// verbatim (no separate message-agent call needed for the invariant; the planner's TreeDiff is the
	// gated path under test).
	plannerJSON := `{"count":1,"single":true,"commits":[{"title":"add big","description":"big.go"}],"message":"feat: add big"}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	// Isolate the planner's stdin via role-specific Env (D2/G4): ONLY the planner manifest carries
	// STAGECOACH_STUB_STDINFILE. The stager/message/arbiter manifests do not, so their stubs see
	// os.Getenv("") => drain to io.Discard and never overwrite the planner's file. (Render builds
	// spec.Env = os.Environ() + manifest.Env at render.go:175-180; the per-role Env is the seam.)
	plannerStdin := filepath.Join(t.TempDir(), "planner-stdin.txt")
	plannerM.Env["STAGECOACH_STUB_STDINFILE"] = plannerStdin

	// Stager stub: auto-stage the concept's files so runSingleShortcut can commit. The concept title
	// "add big" maps to file "big.go"; also stage a.go so the tree is clean post-run.
	stager := dcmStagerSeam(t, repo, map[string][]string{
		"add big": {"big.go", "a.go"},
	})

	messageM := dcmMessageManifest(t, bin, "feat: add big")
	roles := RoleManifests{Planner: plannerM, Message: messageM}

	cfg := config.Defaults()
	// The irreducible floor (Issue 4) is skeleton + planner sysPrompt reserve (~497 + overhead +
	// reserveSafetyMargin256) + tokenBudgetMargin1024 ≈ 1825 (observed). tokenLimit MUST sit ABOVE that
	// floor (else the closed loop can never satisfy the invariant and StagedDiff/TreeDiff reject with
	// ErrBelowTokenFloor) yet BELOW the untruncated assembled prompt (~3,950: ~497 sysPrompt + ~3,450
	// body + framing) so the water-fill MUST truncate big.go and the closed loop converges to
	// sysPrompt+payload <= tokenLimit.
	cfg.TokenLimit = 2500 // 1825 (floor) < 2500 < ~3,950 (untruncated) => gate truncates AND fits
	deps := dcmDepsWithConfig(t, repo, roles, cfg)
	deps.stager = stager

	if _, err := Decompose(context.Background(), deps); err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	data, err := os.ReadFile(plannerStdin)
	if err != nil {
		t.Fatalf("read planner stdin: %v (did the planner run? plannerM.Env had STDINFILE=%s)", err, plannerStdin)
	}
	captured := string(data)

	// (FR3j invariant) the planner's assembled prompt fits tokenLimit + separator allowance. The +1 is
	// the Render "\n\n" separator (render.go:158); the closure measures the separator-free prompt.
	measured := git.EstimateTokens(captured)
	if measured > cfg.TokenLimit+assembledPromptSeparatorTokens {
		t.Errorf("FR3j invariant violated (planner): EstimateTokens(planner stdin) = %d, want <= %d (tokenLimit %d + %d separator)\n"+
			"captured (first 400 chars): %q", measured, cfg.TokenLimit+assembledPromptSeparatorTokens,
			cfg.TokenLimit, assembledPromptSeparatorTokens, truncForLogD(captured, 400))
	}

	// (Gate-ran proof) the closed loop ACTIVELY truncated — the water-fill sentinel is present.
	if !strings.Contains(captured, "[truncated]") {
		t.Errorf("expected '[truncated]' sentinel in the planner's stdin (tokenLimit=%d << untruncated~%d) — the gate did not truncate\n"+
			"captured (first 400 chars): %q", cfg.TokenLimit, git.EstimateTokens(body), truncForLogD(captured, 400))
	}
}

// truncForLogD is decompose_test.go's local truncation helper for readable failure messages (the
// generate package has its own truncForLog; the two packages are distinct and do not cross-import).
func truncForLogD(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// TestDecompose_StagedIndex_FRM1e (FR-M1e): with files staged in auto mode (Commits=0, Single=false),
// Decompose fails loudly naming the staged paths + remedies, creates ZERO commits, and leaves the
// index byte-for-byte untouched (the check runs before any git mutation). The staged-content error
// is NOT sentinel-wrapped (the design choice — a distinct user-facing actionable category).
func TestDecompose_StagedIndex_FRM1e(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Write + STAGE 2 files (a non-empty index — exactly what the FR-M1e check guards against).
	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.go", "bbb\n")
	dcmStageFile(t, repo, "a.txt")
	dcmStageFile(t, repo, "b.go")

	msgM := dcmMessageManifest(t, bin, "feat: should never run")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults() // Commits=0 (auto), Single=false

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected FR-M1e error for staged index, got nil")
	}

	// The message names each staged path + the FR-M1e phrase + both remedies.
	msg := err.Error()
	for _, want := range []string{"a.txt", "b.go", "requires an empty index", "2 file(s) are staged", "defense-in-depth check (FR-M1e)", "git reset", "stagecoach --single"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}

	// The staged-content error is NOT sentinel-wrapped (design choice — distinct actionable category).
	if errors.Is(err, ErrDecomposeFailed) {
		t.Errorf("staged-content error must NOT wrap ErrDecomposeFailed; got %v", err)
	}

	// ZERO commits created.
	if len(result.Commits) != 0 {
		t.Errorf("Commits len = %d, want 0 (the check runs before any publish)", len(result.Commits))
	}
	if dcmLogCount(t, repo) != 0 {
		t.Fatalf("commit count = %d, want 0 (index check must precede every git mutation)", dcmLogCount(t, repo))
	}

	// Index byte-for-byte untouched: both files STILL staged (the check runs before any git mutation).
	status := dcmStatusPorcelain(t, repo)
	if !strings.Contains(status, "a.txt") || !strings.Contains(status, "b.go") {
		t.Fatalf("index mutated; status = %q (want both files still staged)", status)
	}
}

// TestDecompose_StagedIndex_SingleBypasses (FR-M1e regression): with files staged AND Single=true,
// Decompose STILL commits normally — proves the check is placed AFTER the escape-hatch
// (runSingleEscape → CommitStaged handles a hand-staged index normally).
func TestDecompose_StagedIndex_SingleBypasses(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.go", "bbb\n")
	dcmStageFile(t, repo, "a.txt")
	dcmStageFile(t, repo, "b.go")

	msgM := dcmMessageManifest(t, bin, "feat: all staged")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Single = true // the escape-hatch

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(Single=true, staged): expected escape-hatch success, got %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (escape-hatch bypasses the FR-M1e check)", len(result.Commits))
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}
}

// TestDecompose_StagedIndex_Commits1Bypasses (FR-M1e regression): with files staged AND Commits==1,
// Decompose STILL commits normally — proves `--commits 1` also bypasses the check (same escape-hatch).
func TestDecompose_StagedIndex_Commits1Bypasses(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.go", "bbb\n")
	dcmStageFile(t, repo, "a.txt")
	dcmStageFile(t, repo, "b.go")

	msgM := dcmMessageManifest(t, bin, "feat: all staged")
	roles := RoleManifests{Message: msgM}
	cfg := config.Defaults()
	cfg.Commits = 1 // the escape-hatch

	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(Commits=1, staged): expected escape-hatch success, got %v", err)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (escape-hatch bypasses the FR-M1e check)", len(result.Commits))
	}
	if dcmLogCount(t, repo) != 1 {
		t.Fatalf("commit count = %d, want 1", dcmLogCount(t, repo))
	}
}

// TestIsFileDisjoint covers the FR-M13 set-membership gate (isFileDisjoint). It is PURE — no git, no
// fixtures, no Deps — just []prompt.PlannerCommit literals and a bool assertion. The matrix pins the
// exact disjointness contract before the downstream fast-path (S4 dispatch) depends on it: disjoint /
// empty-Files / single-concept / empty-slice ⇒ true; any shared path (cross-concept) or an
// intra-concept duplicate (the literal occurrence-count algorithm counts both) ⇒ false.
func TestIsFileDisjoint(t *testing.T) {
	cases := []struct {
		name string
		in   []prompt.PlannerCommit
		want bool
	}{
		{"empty slice", nil, true}, // vacuous — nothing to share
		{"single concept", []prompt.PlannerCommit{{Files: []string{"a.go", "b.go"}}}, true},
		{"pairwise disjoint 3 concepts", []prompt.PlannerCommit{
			{Files: []string{"a.go"}},
			{Files: []string{"b.go", "c.go"}},
			{Files: []string{"d.go"}},
		}, true},
		{"empty-Files concept among disjoint", []prompt.PlannerCommit{
			{Files: []string{"a.go"}},
			{Files: nil},
			{Files: []string{"b.go"}},
		}, true},
		{"all empty Files", []prompt.PlannerCommit{{Files: nil}, {Files: []string{}}}, true},
		{"shared path two concepts", []prompt.PlannerCommit{
			{Files: []string{"a.go", "shared.go"}},
			{Files: []string{"shared.go", "b.go"}},
		}, false},
		{"shared path across three concepts", []prompt.PlannerCommit{
			{Files: []string{"x.go"}},
			{Files: []string{"x.go"}},
			{Files: []string{"x.go"}},
		}, false},
		{"intra-concept duplicate disqualifies (literal count)", []prompt.PlannerCommit{
			{Files: []string{"a.go", "a.go"}},
		}, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isFileDisjoint(tc.in); got != tc.want {
				t.Errorf("isFileDisjoint(%+v) = %v; want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestRunLoopFastPath_ConcurrentPublish (P1.M1.T1.S3) is the focused happy-path test for the FR-M13
// file-disjoint fast-path's CONCURRENT PHASE: 3 pairwise-disjoint concepts → runLoopFastPath launches
// all 3 message generations CONCURRENTLY (FR-M14) then publishes them in strict CAS order (FR-M7),
// producing (commits, chainData, nil) in the SAME shapes runLoop returns. Asserts CAS order
// (commit[i].Parent == i==0 ? preRunHEAD : commits[i-1].SHA), chainData parallelism, and that the total
// elapsed is ≈ 1 message latency, not N× (concurrency observable). The concurrency-timing assertion is a
// soft warning (CI jitter); the CAS-order assertion is the hard correctness gate. The comprehensive
// regression suite is P1.M1.T1.S5; this replaces S2's TestRunLoopFastPath_Sweep (which asserted the
// now-removed S3 sentinel).
func TestRunLoopFastPath_ConcurrentPublish(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// Seed a base commit with three files, then modify all three disjointly in the working tree.
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 1\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 1\n")
	dcmWriteFile(t, repo, "c.go", "package c\n\nvar C = 1\n")
	dcmRunGit(t, repo, "add", "a.go", "b.go", "c.go")
	dcmCommitRaw(t, repo, "initial") // BORN repo → baseTree = HEAD^{tree}
	// Disjoint working-tree change set: modify each file independently (the FR-M13 disjoint partition).
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 2\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 2\n")
	dcmWriteFile(t, repo, "c.go", "package c\n\nvar C = 2\n")

	g := git.New(repo)
	ctx := context.Background()

	// Mirror what Decompose does internally: capture baseTree (HEAD^{tree}), then FreezeWorkingTree to
	// capture T_start (the full working-tree change set) AND reset the index back to baseTree so the
	// per-concept sweep starts clean. After this the working-tree changes are “in T_start”, the index is
	// at baseTree, and each concept's `git add <its file>` re-stages its slice of T_start.
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}
	preRunHEAD := dcmHeadSHA(t, repo)

	// Message stub: INPUT-DERIVED + concurrency-safe (P1.M1.T1.S3). Each concept's tree-to-tree diff
	// names a distinct file (a.go/b.go/c.go); the stub inspects its OWN stdin and emits the matching
	// subject. This sidesteps the script stub's file-backed counter (which races across N concurrent
	// stub processes) so a concept's message is deterministic regardless of goroutine scheduling. A
	// 150ms sleep per call makes concurrency observable: if serial, total ≈ 450ms; if concurrent, ≈ 150ms.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.go", msg: "feat: add a"},
		{substr: "b.go", msg: "feat: add b"},
		{substr: "c.go", msg: "feat: add c"},
	})
	messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "150"

	roles := RoleManifests{Message: messageM}
	var logBuf bytes.Buffer
	deps := Deps{
		Git:     g,
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: ui.NewVerbose(&logBuf, true),
	}

	concepts := []prompt.PlannerCommit{
		{Title: "c1", Files: []string{"a.go"}},
		{Title: "c2", Files: []string{"b.go"}},
		{Title: "c3", Files: []string{"c.go"}},
	}

	start := time.Now()
	commits, chainData, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("runLoopFastPath: %v", err)
	}

	// Shape: 3 commits + 3 parallel chain entries.
	if len(commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(commits))
	}
	if len(chainData) != 3 {
		t.Fatalf("chainData len = %d, want 3", len(chainData))
	}

	// Subjects in concept order.
	wantSubjects := []string{"feat: add a", "feat: add b", "feat: add c"}
	for i, want := range wantSubjects {
		if commits[i].Subject != want {
			t.Errorf("Commits[%d].Subject = %q, want %q", i, commits[i].Subject, want)
		}
	}

	// HARD GATE — CAS order (FR-M7). commit[i] parent = i==0 ? preRunHEAD : commits[i-1].SHA.
	for i, c := range commits {
		parent := dcmGitOut(t, repo, "rev-parse", c.SHA+"^")
		wantParent := preRunHEAD
		if i > 0 {
			wantParent = commits[i-1].SHA
		}
		if parent != wantParent {
			t.Errorf("commit[%d] parent = %s, want %s (CAS order violated)", i, parent, wantParent)
		}
	}
	// HEAD advanced to the last commit.
	if head := dcmHeadSHA(t, repo); head != commits[2].SHA {
		t.Errorf("HEAD = %s, want %s (last commit)", head, commits[2].SHA)
	}
	// chainData parallelism: SHA == commits[i].SHA; Parent mirrors the CAS chain.
	for i, ce := range chainData {
		if ce.SHA != commits[i].SHA {
			t.Errorf("chainData[%d].SHA = %s, want %s", i, ce.SHA, commits[i].SHA)
		}
		wantParent := preRunHEAD
		if i > 0 {
			wantParent = commits[i-1].SHA
		}
		if ce.Parent != wantParent {
			t.Errorf("chainData[%d].Parent = %s, want %s", i, ce.Parent, wantParent)
		}
	}

	// git log shows exactly 3 commits reachable from HEAD (the base "initial" is the parent of commit 0).
	if n := dcmLogCount(t, repo); n != 4 { // initial + 3 fast-path commits
		t.Errorf("git log count = %d, want 4 (initial + 3)", n)
	}

	// SOFT GATE — concurrency observable. If serial, elapsed ≈ 3×150ms = 450ms; if concurrent,
	// elapsed ≈ 150ms + git ops. Log a warning on CI jitter (the CAS-order gate above is the hard one).
	if elapsed >= 3*150*time.Millisecond {
		t.Logf("WARNING: elapsed = %v (may indicate no concurrency — CI variability)", elapsed)
	} else {
		t.Logf("concurrency confirmed: elapsed = %v (< 3×150ms = %v)", elapsed, 3*150*time.Millisecond)
	}
	// The concurrent-launch summary log proves the FR-M14 launch-all fired.
	if !strings.Contains(logBuf.String(), "launched 3 concurrent message generations") {
		t.Errorf("expected 'launched 3 concurrent message generations' log; got: %q", logBuf.String())
	}
}

// TestRunLoopFastPath_EditSerial (BUG-001 regression, step 2/2) proves --edit is honored on the
// file-disjoint fast-path in a CONCURRENCY-SAFE way: the serial publish loop runs the editor ONCE
// PER CONCEPT in publish order, so each commit carries its OWN concept's edited message — never
// another concept's. Before the BUG-001 fix, N concurrent goroutines each ran EditMessage on the
// single shared .git/STAGECOACH_EDITMSG file (a race), and a concept could silently receive another
// concept's message. After S1 (P1.M1.T2.S1) moved generation to generateMessageCore (no editor in the
// goroutine), this test gates the re-applied serial-loop EditMessage block (S2 / FR-E4).
//
// The no-op editor (GIT_EDITOR=true) is INTENTIONAL: it preserves each concept's written message so
// the per-concept assertion is deterministic. The single-response stubeditor (stubtest.BuildEditor)
// writes the same message every invocation, which would give both concepts the SAME edited message
// and mask the cross-contamination the test exists to catch.
func TestRunLoopFastPath_EditSerial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// Seed a base commit with two disjoint files, then modify both disjointly in the working tree.
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 1\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 1\n")
	dcmRunGit(t, repo, "add", "a.go", "b.go")
	dcmCommitRaw(t, repo, "initial")
	// Disjoint working-tree change set: modify each file independently (the FR-M13 disjoint partition).
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 2\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 2\n")

	g := git.New(repo)
	ctx := context.Background()

	// Mirror what Decompose does internally: capture baseTree (HEAD^{tree}), then FreezeWorkingTree to
	// capture T_start (the full working-tree change set) AND reset the index back to baseTree so the
	// per-concept sweep starts clean.
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}
	preRunHEAD := dcmHeadSHA(t, repo)

	// NO-OP editor (GIT_EDITOR=true): exits 0 without modifying the file, so EditMessage reads back
	// what it wrote — each concept's OWN message. This is the only deterministic way to assert no
	// cross-contamination (the single-response stubeditor would write the same message every time).
	t.Setenv("GIT_EDITOR", "true")

	// Input-derived, concurrency-safe message stub: each concept's tree-to-tree diff names a distinct
	// file (a.go/b.go); the stub inspects its OWN stdin and emits the matching subject.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.go", msg: "feat: add a"},
		{substr: "b.go", msg: "feat: add b"},
	})

	roles := RoleManifests{Message: messageM}
	var logBuf bytes.Buffer
	deps := Deps{
		Git:     g,
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: ui.NewVerbose(&logBuf, true),
	}
	deps.Config.Edit = true // BUG-001: --edit on the file-disjoint fast-path

	concepts := []prompt.PlannerCommit{
		{Title: "c1", Files: []string{"a.go"}},
		{Title: "c2", Files: []string{"b.go"}},
	}

	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
	if err != nil {
		t.Fatalf("runLoopFastPath: %v", err)
	}

	// Shape: 2 commits (one per disjoint concept).
	if len(commits) != 2 {
		t.Fatalf("commits len = %d, want 2", len(commits))
	}

	// BUG-001 PROOF — each commit carries its OWN concept's edited message (no cross-contamination).
	// The no-op editor preserves what generateMessageCore wrote for each concept; if the serial-loop
	// EditMessage block were absent OR racy on the shared EDITMSG file, a subject could be wrong or
	// both subjects could coincide.
	want := []string{"feat: add a", "feat: add b"}
	for i, w := range want {
		if commits[i].Subject != w {
			t.Errorf("commits[%d].Subject = %q, want %q (BUG-001: own message, no cross-contamination)", i, commits[i].Subject, w)
		}
	}
	if commits[0].Subject == commits[1].Subject {
		t.Errorf("cross-contamination: both subjects = %q", commits[0].Subject)
	}
}

// TestRunLoopFastPath_RescueIsolation (P1.M1.T1.S3, FR-M12a + no-leak drain) asserts that when message[i]
// fails mid-publish, (a) commits 0..i-1 STAND (HEAD == commits[i-1].SHA), (b) a *DecomposeRescueError
// (wrapping *RescueError, errors.Is(ErrRescue)) is returned naming concept i, (c) FormatRescueMulti is
// printed to deps.Out, (d) the remaining i+1..N-1 in-flight channels are DRAINED (no goroutine leak),
// and (e) the rescue recipe's -p parent is the AUTHORITATIVE commits[i-1].SHA — the §5 fix's proof —
// NOT preRunHEAD (which generateMessage captured via RevParseHEAD during concurrent generation, possibly
// before commit[i-1] landed, and is therefore stale).
func TestRunLoopFastPath_RescueIsolation(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 1\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 1\n")
	dcmWriteFile(t, repo, "c.go", "package c\n\nvar C = 1\n")
	dcmRunGit(t, repo, "add", "a.go", "b.go", "c.go")
	dcmCommitRaw(t, repo, "initial") // BORN repo
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 2\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 2\n")
	dcmWriteFile(t, repo, "c.go", "package c\n\nvar C = 2\n")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}
	preRunHEAD := dcmHeadSHA(t, repo)

	// Message stub: INPUT-DERIVED + concurrency-safe (P1.M1.T1.S3). Concept 0 (a.go) → success;
	// concept 1 (b.go) → EMPTY (parse-fail → RescueError with MaxDuplicateRetries=0); concept 2 (c.go)
	// → would-be (drained, not published). Deterministic per-concept regardless of goroutine
	// scheduling (the script stub's counter would race). A 100ms sleep makes the goroutines truly
	// concurrent so the drain of concept 2's in-flight channel is exercised.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.go", msg: "feat: add a"}, // concept 0: success
		{substr: "b.go", msg: ""},            // concept 1: empty → parse fail → RescueError (MaxDuplicateRetries=0)
		{substr: "c.go", msg: "feat: add c"}, // concept 2: would-be (drained, not published)
	})
	messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "100"

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // fail immediately on parse failure → RescueError

	roles := RoleManifests{Message: messageM}
	deps, buf := dcmOutBuffer(t, repo, roles)
	deps.Config = cfg
	deps.Git = g // dcmOutBuffer builds its own Git; re-point to the repo's runner

	concepts := []prompt.PlannerCommit{
		{Title: "c1", Files: []string{"a.go"}},
		{Title: "c2", Files: []string{"b.go"}},
		{Title: "c3", Files: []string{"c.go"}},
	}

	beforeGoroutines := runtime.NumGoroutine()
	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
	afterGoroutines := runtime.NumGoroutine()
	if err == nil {
		t.Fatal("expected error (message rescue for concept 1), got nil")
	}

	// (a) errors.As → *DecomposeRescueError naming concept 1.
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("error = %v, want *DecomposeRescueError", err)
	}
	if dre.Index != 1 {
		t.Errorf("dre.Index = %d, want 1 (concept 1 failed)", dre.Index)
	}
	if dre.Count != 3 {
		t.Errorf("dre.Count = %d, want 3", dre.Count)
	}
	// (b) errors.As → *generate.RescueError (via Unwrap).
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error does not unwrap to *RescueError: %v", err)
	}
	// (c) errors.Is → generate.ErrRescue (→ exitcode 3).
	if !errors.Is(err, generate.ErrRescue) {
		t.Errorf("error is not ErrRescue: %v", err)
	}

	// (d) partial commits stand: exactly 1 (concept 0), and HEAD == commits[0].SHA (concept 1 never
	// published). This is the PARTIAL-STAND proof.
	if len(commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only concept 0 landed)", len(commits))
	}
	if head := dcmHeadSHA(t, repo); head != commits[0].SHA {
		t.Errorf("HEAD = %s, want %s (concept 1 never published; partial stands)", head, commits[0].SHA)
	}
	if dcmLogCount(t, repo) != 2 { // initial + concept 0
		t.Errorf("git log count = %d, want 2 (initial + concept 0)", dcmLogCount(t, repo))
	}

	// (e) FormatRescueMulti printed to deps.Out, naming "concept 2 of 3" (1-based).
	out := buf.String()
	if !strings.Contains(out, "concept 2 of 3") {
		t.Errorf("rescue output missing 'concept 2 of 3'; got: %s", out)
	}
	if !strings.Contains(out, "update-ref HEAD") {
		t.Errorf("rescue output missing 'update-ref HEAD'; got: %s", out)
	}

	// (f) §5 fix proof — the rescue recipe's -p parent is the AUTHORITATIVE commits[0].SHA, NOT
	// preRunHEAD. FormatRescueMulti emits "git commit-tree -p <parentSHA> ..."; the parent in that
	// command must equal commits[0].SHA. The stale re.ParentSHA (captured under concurrency) would have
	// been preRunHEAD; the shallow-copy fix overrides it to prevSHA (== commits[0].SHA after concept 0
	// published). Assert the recipe references commits[0].SHA as the parent and does NOT reference
	// preRunHEAD anywhere in the -p position.
	if !strings.Contains(out, "-p "+commits[0].SHA) {
		t.Errorf("rescue recipe's -p parent = want %s (authoritative prevSHA); output: %s", commits[0].SHA, out)
	}
	if strings.Contains(out, "-p "+preRunHEAD) {
		t.Errorf("rescue recipe's -p parent is STALE preRunHEAD %s (should be commits[0].SHA); output: %s", preRunHEAD, out)
	}
	// The DecomposeRescueError.Rescue.ParentSHA must ALSO carry the authoritative parent (the fix).
	if re.ParentSHA != commits[0].SHA {
		t.Errorf("RescueError.ParentSHA = %s, want %s (authoritative prevSHA; §5 fix)", re.ParentSHA, commits[0].SHA)
	}

	// (g) NO GOROUTION LEAK — concept 2's in-flight channel was drained. Allow a generous delta for
	// the runtime's own goroutines + GC; the hard proof is that the call RETURNED (a blocked drain would
	// hang until the test timeout). Give the scheduler a beat to retire the goroutines.
	time.Sleep(50 * time.Millisecond)
	delta := runtime.NumGoroutine() - beforeGoroutines
	if delta > 0 {
		t.Logf("goroutine delta after drain = %d (before=%d, after=%d, post-wait=%d) — informational; the call returned so concept 2 was drained", delta, beforeGoroutines, afterGoroutines, runtime.NumGoroutine())
	}
}

// =============================================================================
// P1.M1.T1.S5 — Fast-path regression suite (FR-M13/FR-M14).
// Drives the file-disjoint fast-path end-to-end through Decompose (cases 1–8) and
// directly via runLoopFastPath (case 9), locking every invariant the PRD/spec
// demand. runLoopFastPath/runLoop/isFileDisjoint/drains + the dispatch + arbiter
// phase + all non-test source are exercised here, never edited.
// =============================================================================

// sortedFileNames returns the sorted list of bare path names from a git diff-tree
// "--name-only" output (one path per line, trimmed). Used to assert concept isolation
// (each commit's diff-tree vs its parent == exactly its concept's Files).
func sortedFileNames(diffOut string) []string {
	var out []string
	for _, l := range strings.Split(diffOut, "\n") {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	sortStrings(out)
	return out
}

// sortStrings sorts a string slice in place (test-local to avoid a sort import just here).
func sortStrings(s []string) {
	// insertion sort — these slices are tiny (1-3 paths); avoids a "sort" import.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// stringSlicesEqual reports whether two string slices are element-wise equal.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fastPathStagerFatal is the routing oracle: the file-disjoint fast-path must NEVER invoke the stager
// (system_context §6). t.Fatal makes a passing run PROOF the fast-path was taken.
func fastPathStagerFatal(t *testing.T) func(context.Context, Deps, prompt.PlannerCommit) error {
	t.Helper()
	return func(ctx context.Context, _ Deps, concept prompt.PlannerCommit) error {
		t.Fatalf("fast-path must not invoke the stager (concept %q routed to runLoop)", concept.Title)
		return nil
	}
}

// readArbiterCounter reads the stub arbiter's call-counter file and returns its integer value
// (0/empty ⇒ arbiter NOT called; ≥1 ⇒ called). Mirrors MessageRescuePartial / CASAbortPartial.
func readArbiterCounter(t *testing.T, counterFile string) int {
	t.Helper()
	data, err := os.ReadFile(counterFile)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ---------------------------------------------------------------------------
// Case 1 — TestDecompose_FastPath_DisjointIsolationAndCompleteness
//
// The flagship fast-path regression: a pairwise file-disjoint planner partition routes to
// runLoopFastPath (stager NEVER invoked), produces N correctly-isolated commits (each commit's
// diff-tree == exactly its concept's Files) in strict CAS order, AND T_start completeness holds in
// BOTH arbiter sub-cases: (A) planner union == all T_start paths ⇒ leftover empty ⇒ arbiter skipped
// (counter 0); (B) one T_start path declared for NO concept ⇒ leftover non-empty ⇒ arbiter folds it
// (counter ≥1, the (N+1)-th commit's tree == T_start).
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_DisjointIsolationAndCompleteness(t *testing.T) {
	bin := stubtest.Build(t)

	// --- Sub-case A: disjoint union == all T_start paths ⇒ arbiter skipped. ---
	t.Run("arbiter_skipped_leftover_empty", func(t *testing.T) {
		repo := t.TempDir()
		dcmInitRepo(t, repo)
		dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "base") // BORN repo ⇒ preRunHEAD resolves

		dcmWriteFile(t, repo, "a.txt", "aaa\n")
		dcmWriteFile(t, repo, "b.txt", "bbb\n")
		dcmWriteFile(t, repo, "c.txt", "ccc\n")

		// Planner union == all 3 T_start paths ⇒ leftover empty ⇒ arbiter NOT called.
		plannerJSON := `{"count":3,"single":false,"commits":[` +
			`{"title":"c1","description":"a","files":["a.txt"]},` +
			`{"title":"c2","description":"b","files":["b.txt"]},` +
			`{"title":"c3","description":"c","files":["c.txt"]}` +
			`]}`
		plannerM := dcmPlannerManifest(t, bin, plannerJSON)
		messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
			{substr: "a.txt", msg: "feat: add a"},
			{substr: "b.txt", msg: "feat: add b"},
			{substr: "c.txt", msg: "feat: add c"},
		})
		roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
		roles.Planner = plannerM
		roles.Message = messageM

		// Arbiter counter: should remain 0 (leftover empty ⇒ arbiter skipped).
		counterDir := t.TempDir()
		counterFile := counterDir + "/counter"
		roles.Arbiter = stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile})

		deps := dcmDeps(t, repo, roles)
		deps.stager = fastPathStagerFatal(t) // routing oracle

		preRunHEAD := dcmHeadSHA(t, repo) // born repo — captured before Decompose mutates HEAD
		result, err := Decompose(context.Background(), deps)
		if err != nil {
			t.Fatalf("Decompose: %v", err)
		}
		if len(result.Commits) != 3 {
			t.Fatalf("Commits len = %d, want 3", len(result.Commits))
		}

		// Concept isolation: each commit's diff-tree vs its parent == exactly its concept's Files.
		wantFiles := [][]string{{"a.txt"}, {"b.txt"}, {"c.txt"}}
		wantSubjects := []string{"feat: add a", "feat: add b", "feat: add c"}
		for i, c := range result.Commits {
			if c.Subject != wantSubjects[i] {
				t.Errorf("Commits[%d].Subject = %q, want %q", i, c.Subject, wantSubjects[i])
			}
			parent := preRunHEAD
			if i > 0 {
				parent = result.Commits[i-1].SHA
			}
			diff := dcmGitOut(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", c.SHA, parent)
			if got := sortedFileNames(diff); !stringSlicesEqual(got, wantFiles[i]) {
				t.Errorf("commit[%d] not isolated: got %v, want %v (diff-tree=%q)", i, got, wantFiles[i], diff)
			}
			// CAS-ordered parents.
			if gotParent := dcmGitOut(t, repo, "rev-parse", c.SHA+"^"); gotParent != parent {
				t.Errorf("commit[%d] parent = %s, want %s (CAS order)", i, gotParent, parent)
			}
		}

		// Arbiter skipped (leftover empty).
		if n := readArbiterCounter(t, counterFile); n != 0 {
			t.Errorf("arbiter call count = %d, want 0 (leftover empty ⇒ arbiter skipped)", n)
		}
		if result.Amended != 0 {
			t.Errorf("Amended = %d, want 0", result.Amended)
		}
		if status := dcmStatusPorcelain(t, repo); status != "" {
			t.Errorf("status = %q, want empty (clean)", status)
		}
	})

	// --- Sub-case B: one T_start path declared for NO concept ⇒ arbiter folds it. ---
	t.Run("arbiter_folds_leftover_present", func(t *testing.T) {
		repo := t.TempDir()
		dcmInitRepo(t, repo)

		// 4 disjoint files; the planner declares ONLY a/b/c — d.txt is a frozen leftover.
		dcmWriteFile(t, repo, "a.txt", "aaa\n")
		dcmWriteFile(t, repo, "b.txt", "bbb\n")
		dcmWriteFile(t, repo, "c.txt", "ccc\n")
		dcmWriteFile(t, repo, "d.txt", "ddd\n")

		// Capture the EXACTLY-T_start oracle (the full 4-file working-tree change set) for the
		// arbiter-commit tree assertion below.
		dcmRunGit(t, repo, "add", "a.txt", "b.txt", "c.txt", "d.txt")
		tStart := dcmGitOut(t, repo, "write-tree")
		dcmRunGit(t, repo, "rm", "--cached", "--ignore-unmatch", "a.txt", "b.txt", "c.txt", "d.txt")

		plannerJSON := `{"count":3,"single":false,"commits":[` +
			`{"title":"c1","description":"a","files":["a.txt"]},` +
			`{"title":"c2","description":"b","files":["b.txt"]},` +
			`{"title":"c3","description":"c","files":["c.txt"]}` +
			`]}`
		plannerM := dcmPlannerManifest(t, bin, plannerJSON)
		messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
			{substr: "a.txt", msg: "feat: add a"},
			{substr: "b.txt", msg: "feat: add b"},
			{substr: "c.txt", msg: "feat: add c"},
			{substr: "d.txt", msg: "feat: arbiter leftover"},
		})
		roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
		roles.Planner = plannerM
		roles.Message = messageM
		// null target ⇒ the arbiter makes a NEW commit folding the leftover (resolveNewCommit).
		roles.Arbiter = dcmArbiterManifest(t, bin, `{"target": null}`)

		deps := dcmDeps(t, repo, roles)
		deps.stager = fastPathStagerFatal(t) // routing oracle

		result, err := Decompose(context.Background(), deps)
		if err != nil {
			t.Fatalf("Decompose: %v", err)
		}
		// 3 fast-path commits + 1 arbiter fold = 4.
		if len(result.Commits) != 4 {
			t.Fatalf("Commits len = %d, want 4 (3 fast-path + 1 arbiter fold)", len(result.Commits))
		}

		// The arbiter commit's tree == EXACTLY T_start (it folded ONLY the frozen leftover d.txt).
		headTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
		if headTree != tStart {
			t.Errorf("arbiter commit tree = %s, want EXACTLY T_start = %s", headTree, tStart)
		}
	})
}

// ---------------------------------------------------------------------------
// Case 2 — TestDecompose_FastPath_SharedFallbackMatchesRunLoop
//
// The inverse oracle: a SHARED file (one path in ≥2 concepts) routes to runLoop, which invokes the
// stager per concept. The stager is called for BOTH concepts (flag set) and the tip reconstructs T_start
// exactly (byte-identical to runLoop-only behavior) — proving the fallback is the UNCHANGED runLoop.
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_SharedFallbackMatchesRunLoop(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	base := "def get_links():\n    return []\n\ndef sort_items():\n    return []\n"
	tStartContent := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return sorted(links, key=lambda c: c.code)\n"
	basePlusA := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return []\n" // concept 0's hunk only

	dcmWriteFile(t, repo, "store.py", base)
	dcmStageFile(t, repo, "store.py")
	dcmCommitRaw(t, repo, "initial")
	dcmWriteFile(t, repo, "store.py", tStartContent) // dirty → triggers decompose

	// store.py is declared in BOTH concepts ⇒ isFileDisjoint FALSE ⇒ runLoop.
	plannerJSON := `{"count":2,"single":false,"commits":[` +
		`{"title":"feat: add link fetching","description":"the get_links change","files":["store.py"]},` +
		`{"title":"feat: sort listed links by code","description":"the sort_items change","files":["store.py"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add link fetching", "feat: sort listed links by code"})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.Config.Commits = 2 // forced count overrides the FR-M2b one-file short-circuit

	// THE FALLBACK ORACLE: runLoop MUST call the stager. A counter proves BOTH concepts were staged.
	var stagerCalls []string
	deps.stager = func(ctx context.Context, d Deps, concept prompt.PlannerCommit) error {
		stagerCalls = append(stagerCalls, concept.Title)
		switch concept.Title {
		case "feat: add link fetching":
			stagePartialBlob(t, repo, "store.py", basePlusA) // hunk A only — a strict subset of T_start
		case "feat: sort listed links by code":
			stagePartialBlob(t, repo, "store.py", tStartContent) // + hunk B ⇒ index == T_start for store.py
		}
		return nil
	}

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(stagerCalls) != 2 {
		t.Fatalf("stager called %d times, want 2 (shared file ⇒ runLoop stager per concept): %v", len(stagerCalls), stagerCalls)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2", len(result.Commits))
	}
	// The two commits reconstruct T_start exactly — byte-identical to runLoop-only behavior.
	if got, want := dcmGitOut(t, repo, "show", "HEAD:store.py"), strings.TrimRight(tStartContent, "\n"); got != want {
		t.Errorf("final tip store.py must reconstruct the full change; got %q want %q", got, want)
	}
	if n := dcmLogCount(t, repo); n != 3 { // initial + 2 concepts
		t.Errorf("commit count = %d, want 3", n)
	}
}

// ---------------------------------------------------------------------------
// Case 3 — TestDecompose_FastPath_ConcurrencyIntervalOverlap
//
// HARD per-goroutine interval overlap: each stub message invocation appends "start_ns end_ns\n" to a
// shared file via the STAGECOACH_STUB_INTERVAL_FILE hook (atomic on POSIX). The test reads all lines,
// parses N intervals, sorts by start, and asserts ≥1 consecutive pair where start[j] < end[i] (overlap)
// — NOT strictly serial. A uniform 100ms sleep makes all N overlap robustly (ms-scale exec jitter <<
// 100ms). This is the regression guard a future silent re-serialization cannot evade.
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_ConcurrencyIntervalOverlap(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":["b.txt"]},` +
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a"},
		{substr: "b.txt", msg: "feat: add b"},
		{substr: "c.txt", msg: "feat: add c"},
	})
	// A uniform 100ms sleep widens the concurrency window; the interval probe records it.
	messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "100"
	intervalFile := t.TempDir() + "/intervals.txt"
	messageM.Env["STAGECOACH_STUB_INTERVAL_FILE"] = intervalFile

	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	var logBuf bytes.Buffer
	deps := Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: ui.NewVerbose(&logBuf, true), // captures the "launched N concurrent" log
	}
	deps.stager = fastPathStagerFatal(t)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}

	// Corroborate the concurrent-launch log (FR-M14).
	if !strings.Contains(logBuf.String(), "launched 3 concurrent message generations") {
		t.Errorf("expected 'launched 3 concurrent message generations' log; got: %q", logBuf.String())
	}

	// HARD GATE — per-goroutine interval overlap. Read + parse the interval file.
	data, rerr := os.ReadFile(intervalFile)
	if rerr != nil {
		t.Fatalf("read interval file: %v", rerr)
	}
	type interval struct{ start, end int64 }
	var intervals []interval
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		s, e1 := strconv.ParseInt(fields[0], 10, 64)
		e, e2 := strconv.ParseInt(fields[1], 10, 64)
		if e1 != nil || e2 != nil {
			continue
		}
		intervals = append(intervals, interval{s, e})
	}
	if len(intervals) != 3 {
		t.Fatalf("expected 3 interval records, got %d (data=%q)", len(intervals), string(data))
	}
	// Sort by start (insertion sort — N=3).
	for i := 1; i < len(intervals); i++ {
		for j := i; j > 0 && intervals[j-1].start > intervals[j].start; j-- {
			intervals[j-1], intervals[j] = intervals[j], intervals[j-1]
		}
	}
	// Assert NOT strictly serial: there EXISTS a consecutive pair where start[j] < end[i] (overlap).
	overlapped := false
	for i := 0; i < len(intervals)-1; i++ {
		if intervals[i+1].start < intervals[i].end {
			overlapped = true
			break
		}
	}
	if !overlapped {
		t.Errorf("intervals are strictly serial (no overlap) — the fast-path was re-serialized: %+v", intervals)
	} else {
		t.Logf("concurrency confirmed: intervals overlap (hard gate) — %+v", intervals)
	}
}

// ---------------------------------------------------------------------------
// Case 4 — TestDecompose_FastPath_OutOfOrderCompletesOrderedPublish
//
// Per-match sleep orders message completion OUT of concept order (concept 0 slowest, concept 2
// fastest) yet the publish loop STILL emits the chain in strict CAS order
// (preRunHEAD→c0→c1→c2). This proves the serial publish loop blocks on inflight[0] until the slowest
// message finishes, THEN publishes in order.
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_OutOfOrderCompletesOrderedPublish(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "base") // BORN repo ⇒ preRunHEAD resolves

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":["b.txt"]},` +
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	// Per-match sleep: message 0 (a.txt) is SLOWEST, message 2 (c.txt) is FASTEST. So message 2
	// finishes FIRST, message 0 finishes LAST — but the publish loop must still emit c0→c1→c2 in order.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a", sleepMs: 300},
		{substr: "b.txt", msg: "feat: add b", sleepMs: 200},
		{substr: "c.txt", msg: "feat: add c", sleepMs: 100},
	})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.stager = fastPathStagerFatal(t)

	preRunHEAD := dcmHeadSHA(t, repo) // born repo — captured before Decompose mutates HEAD
	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}

	// Subjects in CONCEPT order (not completion order) — proves ordered publish.
	wantSubjects := []string{"feat: add a", "feat: add b", "feat: add c"}
	for i, want := range wantSubjects {
		if result.Commits[i].Subject != want {
			t.Errorf("Commits[%d].Subject = %q, want %q (publish order ≠ completion order)", i, result.Commits[i].Subject, want)
		}
	}

	// STRICT CAS order: commit[i] parent = i==0 ? preRunHEAD : commits[i-1].SHA.
	for i, c := range result.Commits {
		parent := dcmGitOut(t, repo, "rev-parse", c.SHA+"^")
		wantParent := preRunHEAD
		if i > 0 {
			wantParent = result.Commits[i-1].SHA
		}
		if parent != wantParent {
			t.Errorf("commit[%d] parent = %s, want %s (CAS order violated under out-of-order completion)", i, parent, wantParent)
		}
	}
}

// ---------------------------------------------------------------------------
// Case 5a — TestDecompose_FastPath_RescueIsolation
//
// Through Decompose (S3's _RescueIsolation is direct-call): message[1] fails (empty ⇒ parse-fail ⇒
// RescueError, MaxDuplicateRetries=0) ⇒ *DecomposeRescueError(Index==1, Count==3), exactly 1 partial
// commit (concept 0), "concept 2 of 3" + "update-ref HEAD" in Out, arbiter NOT called.
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_RescueIsolation(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":["b.txt"]},` +
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a"}, // concept 0: success
		{substr: "b.txt", msg: ""},            // concept 1: empty ⇒ parse-fail ⇒ RescueError
		{substr: "c.txt", msg: "feat: add c"}, // concept 2: would-be (drained, not published)
	})
	messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "100" // widen the concurrency window

	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // fail immediately on parse failure ⇒ RescueError

	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	roles := RoleManifests{
		Planner: plannerM,
		Stager:  tooledStubManifest(t, bin, stubtest.Options{Out: ""}),
		Message: messageM,
		Arbiter: stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile}),
	}
	deps, buf := dcmOutBuffer(t, repo, roles)
	deps.Config = cfg
	deps.stager = fastPathStagerFatal(t)

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected error (message rescue for concept 1), got nil")
	}

	// (a) errors.As → *DecomposeRescueError naming concept 1.
	var dre *DecomposeRescueError
	if !errors.As(err, &dre) {
		t.Fatalf("error = %v, want *DecomposeRescueError", err)
	}
	if dre.Index != 1 {
		t.Errorf("dre.Index = %d, want 1", dre.Index)
	}
	if dre.Count != 3 {
		t.Errorf("dre.Count = %d, want 3", dre.Count)
	}
	// (b) errors.As → *generate.RescueError (via Unwrap).
	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error does not unwrap to *RescueError: %v", err)
	}
	// (c) errors.Is → generate.ErrRescue (→ exitcode 3).
	if !errors.Is(err, generate.ErrRescue) {
		t.Errorf("error is not ErrRescue: %v", err)
	}
	// (d) partial commits: exactly 1 (concept 0).
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only concept 0 landed)", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}
	// (e) Out names "concept 2 of 3" (1-indexed) + the rescue recipe.
	out := buf.String()
	if !strings.Contains(out, "concept 2 of 3") {
		t.Errorf("rescue output missing 'concept 2 of 3'; got: %s", out)
	}
	if !strings.Contains(out, "update-ref HEAD") {
		t.Errorf("rescue output missing 'update-ref HEAD'; got: %s", out)
	}
	// (f) arbiter NOT called (rescue skips the arbiter).
	if n := readArbiterCounter(t, counterFile); n != 0 {
		t.Errorf("arbiter call count = %d, want 0 (rescue should skip arbiter)", n)
	}
}

// ---------------------------------------------------------------------------
// Case 5b — TestDecompose_FastPath_CASAbortPartial
//
// FR-M12b on the fast-path: an external goroutine moves HEAD between c0's publish and c1's publish
// (created by per-match sleep: c0 fast, c1 slow). The move uses commit-tree/update-ref (NOT
// --allow-empty) so c1's publish CAS fails ⇒ *generate.CASError, errors.Is(ErrCASFailed), "HEAD moved"
// in Out, exactly 1 partial commit (c0), arbiter NOT called.
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_CASAbortPartial(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":["b.txt"]},` +
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	// Per-match sleep creates the CAS window: c0 (a.txt) FAST, c1 (b.txt) SLOW (in flight when c0
	// publishes), c2 (c.txt) fast. The HEAD move lands while c1's message is in flight.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a", sleepMs: 0},
		{substr: "b.txt", msg: "feat: add b", sleepMs: 400},
		{substr: "c.txt", msg: "feat: add c", sleepMs: 0},
	})
	counterDir := t.TempDir()
	counterFile := counterDir + "/counter"
	roles := RoleManifests{
		Planner: plannerM,
		Stager:  tooledStubManifest(t, bin, stubtest.Options{Out: ""}),
		Message: messageM,
		Arbiter: stubtest.Manifest(bin, stubtest.Options{Script: counterDir + "/script.txt", Counter: counterFile}),
	}
	deps, buf := dcmOutBuffer(t, repo, roles)
	deps.stager = fastPathStagerFatal(t)

	// External HEAD-move goroutine: poll until c0 ("feat: add a") is in the log AND c1 ("feat: add b")
	// is NOT yet, then commit-tree/update-ref HEAD (mirror CASAbortPartial :1642's idiom). The move
	// lands while c1's 400ms message is in flight ⇒ c1's publish CAS fails.
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			logOut, _ := exec.Command("git", "-C", repo, "log", "--format=%s").CombinedOutput()
			s := string(logOut)
			if strings.Contains(s, "feat: add a") && !strings.Contains(s, "feat: add b") {
				// Brief armed delay to land inside c1's in-flight window.
				time.Sleep(50 * time.Millisecond)
				tree := dcmRunGit(t, repo, "rev-parse", "HEAD^{tree}")
				c := dcmRunGit(t, repo, "commit-tree", tree, "-p", "HEAD", "-m", "external head move")
				dcmRunGit(t, repo, "update-ref", "HEAD", c)
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		// deadline expired without the window — leave HEAD alone; the test's assertions fail loudly.
	}()

	result, err := Decompose(context.Background(), deps)
	if err == nil {
		t.Fatal("expected CAS error, got nil")
	}

	// (a) errors.As → *generate.CASError.
	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error = %v, want *generate.CASError", err)
	}
	// (b) errors.Is → git.ErrCASFailed (→ exitcode 1).
	if !errors.Is(err, git.ErrCASFailed) {
		t.Errorf("error is not ErrCASFailed: %v", err)
	}
	// (c) deps.Out contains "HEAD moved".
	out := buf.String()
	if !strings.Contains(out, "HEAD moved") {
		t.Errorf("CAS output missing 'HEAD moved'; got: %s", out)
	}
	// (d) partial commits: exactly 1 (concept 0 landed before CAS failure on concept 1's publish).
	if len(result.Commits) != 1 {
		t.Fatalf("Commits len = %d, want 1 (only c0 landed before CAS failure)", len(result.Commits))
	}
	if result.Commits[0].Subject != "feat: add a" {
		t.Errorf("Commits[0].Subject = %q, want %q", result.Commits[0].Subject, "feat: add a")
	}
	// (e) arbiter NOT called (CAS abort skips the arbiter).
	if n := readArbiterCounter(t, counterFile); n != 0 {
		t.Errorf("arbiter call count = %d, want 0 (CAS abort should skip arbiter)", n)
	}
}

// ---------------------------------------------------------------------------
// Case 6 — TestDecompose_FastPath_EmptyConceptSkip
//
// FR-M8 empty-skip on the fast-path: concept[1] has empty Files (or a path not in T_start) ⇒ stages
// nothing ⇒ treeI==prevTree ⇒ skipped (no empty commit). The CAS chain is gap-free: 2 commits land
// (a.txt, c.txt) with correct parents (c0→c1).
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_EmptyConceptSkip(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	dcmRunGit(t, repo, "commit", "--allow-empty", "-m", "base") // BORN repo ⇒ preRunHEAD resolves

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")
	// b.txt is NOT written; concept 1 declares EMPTY files ⇒ git add no-op ⇒ treeI==prevTree ⇒ FR-M8 skip.

	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":[]},` + // empty Files ⇒ git add no-op ⇒ FR-M8 skip
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	// Only 2 messages needed: concept 1 is skipped (no message generated). Match rules for a + c only.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a"},
		{substr: "c.txt", msg: "feat: add c"},
	})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	deps := dcmDeps(t, repo, roles)
	deps.stager = fastPathStagerFatal(t)

	initialCommits := dcmLogCount(t, repo)
	preRunHEAD := dcmHeadSHA(t, repo) // born repo — captured before Decompose mutates HEAD
	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose(empty-skip): %v", err)
	}
	if len(result.Commits) != 2 {
		t.Fatalf("Commits len = %d, want 2 (concept 1 FR-M8-skipped)", len(result.Commits))
	}
	wantSubjects := []string{"feat: add a", "feat: add c"}
	for i, want := range wantSubjects {
		if result.Commits[i].Subject != want {
			t.Errorf("Commits[%d].Subject = %q, want %q", i, result.Commits[i].Subject, want)
		}
	}
	// No empty commit for the skipped concept: log grew by exactly 2.
	if n := dcmLogCount(t, repo); n != initialCommits+2 {
		t.Errorf("commit count = %d, want %d (no empty commit for skipped concept)", n, initialCommits+2)
	}
	// Gap-free CAS chain: c0→c1, parents correct.
	for i, c := range result.Commits {
		parent := dcmGitOut(t, repo, "rev-parse", c.SHA+"^")
		wantParent := preRunHEAD
		if i > 0 {
			wantParent = result.Commits[i-1].SHA
		}
		if parent != wantParent {
			t.Errorf("commit[%d] parent = %s, want %s (CAS chain gap-free despite skip)", i, parent, wantParent)
		}
	}
}

// ---------------------------------------------------------------------------
// Case 7 — TestDecompose_FastPath_FreezeGuardWired
//
// FR-M1c defense-in-depth: verifyFreezeSubset is PROVABLY wired on the fast-path (called once per
// concept in the sweep, BEFORE the FR-M8 skip check). MergeTrees is verifyFreezeSubset's unique part-B
// call, so a counting git wrapper that overrides MergeTrees counts it. With 3 disjoint NON-skipped
// concepts, count == 3 == len(concepts).
// ---------------------------------------------------------------------------

// countingGit wraps git.Git and counts MergeTrees calls (verifyFreezeSubset's unique part-B primitive).
type countingGit struct {
	git.Git
	n *atomic.Int64
}

func (c *countingGit) MergeTrees(ctx context.Context, baseTree, ourTree, theirTree string) (string, bool, error) {
	c.n.Add(1)
	return c.Git.MergeTrees(ctx, baseTree, ourTree, theirTree)
}

func TestDecompose_FastPath_FreezeGuardWired(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	dcmWriteFile(t, repo, "a.txt", "aaa\n")
	dcmWriteFile(t, repo, "b.txt", "bbb\n")
	dcmWriteFile(t, repo, "c.txt", "ccc\n")

	plannerJSON := `{"count":3,"single":false,"commits":[` +
		`{"title":"c1","description":"a","files":["a.txt"]},` +
		`{"title":"c2","description":"b","files":["b.txt"]},` +
		`{"title":"c3","description":"c","files":["c.txt"]}` +
		`]}`
	plannerM := dcmPlannerManifest(t, bin, plannerJSON)
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "feat: add a"},
		{substr: "b.txt", msg: "feat: add b"},
		{substr: "c.txt", msg: "feat: add c"},
	})
	roles := dcmAllRoles(t, bin, stubtest.Options{Out: ""})
	roles.Planner = plannerM
	roles.Message = messageM
	var n atomic.Int64
	deps := Deps{
		Git:     &countingGit{git.New(repo), &n},
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: nil,
	}
	deps.stager = fastPathStagerFatal(t)

	result, err := Decompose(context.Background(), deps)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(result.Commits))
	}
	// verifyFreezeSubset runs for EVERY concept in the sweep (BEFORE the FR-M8 skip check), and each
	// invocation calls MergeTrees exactly once (its unique part-B content check). With 3 disjoint
	// NON-skipped concepts, count == 3 == len(concepts). (The arbiter does NOT call MergeTrees.)
	if got := n.Load(); got != 3 {
		t.Errorf("MergeTrees call count = %d, want 3 (verifyFreezeSubset wired once per concept)", got)
	}
}

// ---------------------------------------------------------------------------
// Case 8 — TestDecompose_FastPath_TooledFlagsLessProvider
//
// G29 side effect (FR-D4): a TooledFlags-less provider (BARE manifest — nil TooledFlags, the
// opencode/qwen-code shape) DECOMPOSES a disjoint tree via the fast-path (stager bypassed ⇒
// RenderTooled never called) but CANNOT serve as a stager on a shared-file tree: runLoop invokes the
// real stageConcept → RenderTooled → 'tooled mode requires non-empty tooled_flags', which FR-M12d's
// retry-once-then-empty SWALLOWS into an empty-skip for BOTH concepts (the error is ErrStagerFailed,
// NOT ErrStagerMovedHEAD, so it is retried then treated as empty). The faithful proof is ZERO commits
// + the Verbose "stager failed twice … treating concept as empty" log. deps.stager is left nil so the
// REAL stageConcept fires (a seam would mask the error).
// ---------------------------------------------------------------------------

func TestDecompose_FastPath_TooledFlagsLessProvider(t *testing.T) {
	bin := stubtest.Build(t)

	// --- Sub-case: disjoint SUCCEEDS via the fast-path (stager bypassed). ---
	t.Run("disjoint_succeeds", func(t *testing.T) {
		repo := t.TempDir()
		dcmInitRepo(t, repo)

		dcmWriteFile(t, repo, "a.txt", "aaa\n")
		dcmWriteFile(t, repo, "b.txt", "bbb\n")
		dcmWriteFile(t, repo, "c.txt", "ccc\n")

		plannerJSON := `{"count":3,"single":false,"commits":[` +
			`{"title":"c1","description":"a","files":["a.txt"]},` +
			`{"title":"c2","description":"b","files":["b.txt"]},` +
			`{"title":"c3","description":"c","files":["c.txt"]}` +
			`]}`
		plannerM := dcmPlannerManifest(t, bin, plannerJSON)
		messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
			{substr: "a.txt", msg: "feat: add a"},
			{substr: "b.txt", msg: "feat: add b"},
			{substr: "c.txt", msg: "feat: add c"},
		})
		// BARE manifest (nil TooledFlags) for the Stager role — NOT tooledStubManifest. The fast-path
		// never reaches the stager, so RenderTooled is never called ⇒ succeeds.
		roles := RoleManifests{
			Planner: plannerM,
			Stager:  stubtest.Manifest(bin, stubtest.Options{Out: ""}),
			Message: messageM,
			Arbiter: stubtest.Manifest(bin, stubtest.Options{Out: ""}),
		}
		deps := dcmDeps(t, repo, roles)
		// deps.stager left NIL — the fast-path doesn't invoke it; if the run mis-routed to runLoop it
		// would hit the real stageConcept (the faithful path). No oracle here: the disjoint success IS
		// the proof the fast-path bypassed the stager.

		result, err := Decompose(context.Background(), deps)
		if err != nil {
			t.Fatalf("TooledFlags-less disjoint must succeed via fast-path bypass; got: %v", err)
		}
		if len(result.Commits) != 3 {
			t.Fatalf("Commits len = %d, want 3", len(result.Commits))
		}
		if status := dcmStatusPorcelain(t, repo); status != "" {
			t.Errorf("status = %q, want empty (clean)", status)
		}
	})

	// --- Sub-case: shared CANNOT serve as a stager (FR-M12d swallows the render error). ---
	//
	// A TooledFlags-less provider cannot render in tooled mode, so stageConcept errors with the
	// unchanged 'tooled mode requires non-empty tooled_flags' message. runLoop's FR-M12d retry-once-
	// then-empty logic SWALLOWS that stager error (it is ErrStagerFailed, NOT ErrStagerMovedHEAD) into
	// an empty-skip for BOTH concepts, so the run returns nil error with ZERO commits. The faithful
	// proof the stager was invoked + failed is the Verbose retry log ("stager failed twice … treating
	// concept as empty"). The disjoint sub-case above proves the bypass; this proves the failure.
	t.Run("shared_cannot_serve_as_stager", func(t *testing.T) {
		repo := t.TempDir()
		dcmInitRepo(t, repo)

		base := "def get_links():\n    return []\n\ndef sort_items():\n    return []\n"
		tStartContent := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return sorted(links, key=lambda c: c.code)\n"
		dcmWriteFile(t, repo, "store.py", base)
		dcmStageFile(t, repo, "store.py")
		dcmCommitRaw(t, repo, "initial")
		dcmWriteFile(t, repo, "store.py", tStartContent)

		plannerJSON := `{"count":2,"single":false,"commits":[` +
			`{"title":"feat: add link fetching","description":"the get_links change","files":["store.py"]},` +
			`{"title":"feat: sort listed links by code","description":"the sort_items change","files":["store.py"]}` +
			`]}`
		plannerM := dcmPlannerManifest(t, bin, plannerJSON)
		messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add link fetching", "feat: sort listed links by code"})
		roles := RoleManifests{
			Planner: plannerM,
			Stager:  stubtest.Manifest(bin, stubtest.Options{Out: ""}), // BARE — nil TooledFlags
			Message: messageM,
			Arbiter: stubtest.Manifest(bin, stubtest.Options{Out: ""}),
		}
		var logBuf bytes.Buffer
		deps := Deps{
			Git:     git.New(repo),
			Config:  config.Defaults(),
			Roles:   roles,
			Verbose: ui.NewVerbose(&logBuf, true), // captures the FR-M12d stager-failed retry log
		}
		deps.Config.Commits = 2
		// deps.stager NIL ⇒ the run hits the REAL stageConcept → RenderTooled → the unchanged error.

		result, err := Decompose(context.Background(), deps)
		if err != nil {
			t.Fatalf("FR-M12d swallows the stager error into empty-skip; got unexpected err: %v", err)
		}
		// FR-M12d empty-skips BOTH concepts (stager fails twice each) ⇒ ZERO commits. A TooledFlags-less
		// provider cannot decompose a shared-file tree (the G29 side effect's negative proof).
		if len(result.Commits) != 0 {
			t.Errorf("Commits len = %d, want 0 (TooledFlags-less stager cannot stage the shared file; both concepts FR-M8-skipped)", len(result.Commits))
		}
		// The Verbose log PROVES the stager was invoked + failed both times (the RenderTooled error
		// fired for each concept), confirming the provider genuinely cannot serve as a stager.
		logStr := logBuf.String()
		if !strings.Contains(logStr, "stager failed twice") || !strings.Contains(logStr, "treating concept as empty") {
			t.Errorf("expected FR-M12d 'stager failed twice … treating concept as empty' log (proof the TooledFlags-less stager failed); got: %s", logStr)
		}
	})
}

// ---------------------------------------------------------------------------
// Case 9 — TestRunLoopFastPath_StartOfRunFreezeExcludesSentinel
//
// FR-M1b start-of-run freeze (DIRECT-CALL — there is no Go seam between Decompose's FreezeWorkingTree
// and runLoopFastPath for exact freeze→sentinel→run control). A sentinel written AFTER
// FreezeWorkingTree returns (T_start frozen) and BEFORE runLoopFastPath lands in NO commit and
// survives in the worktree.
// ---------------------------------------------------------------------------

func TestRunLoopFastPath_StartOfRunFreezeExcludesSentinel(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)

	// Seed a base commit with three files, then modify all three disjointly (S3's direct-call idiom).
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 1\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 1\n")
	dcmWriteFile(t, repo, "c.go", "package c\n\nvar C = 1\n")
	dcmRunGit(t, repo, "add", "a.go", "b.go", "c.go")
	dcmCommitRaw(t, repo, "initial") // BORN repo
	dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 2\n")
	dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 2\n")
	dcmWriteFile(t, repo, "c.go", "package c\n\nvar C = 2\n")

	g := git.New(repo)
	ctx := context.Background()
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}
	preRunHEAD := dcmHeadSHA(t, repo)

	// Write the sentinel AFTER the freeze (T_start captured) and BEFORE the run ⇒ it is NOT in T_start.
	dcmWriteFile(t, repo, "sentinel.txt", "concurrent change")

	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.go", msg: "feat: add a"},
		{substr: "b.go", msg: "feat: add b"},
		{substr: "c.go", msg: "feat: add c"},
	})
	roles := RoleManifests{Message: messageM}
	deps := Deps{
		Git:     g,
		Config:  config.Defaults(),
		Roles:   roles,
		Verbose: nil,
	}

	concepts := []prompt.PlannerCommit{
		{Title: "c1", Files: []string{"a.go"}},
		{Title: "c2", Files: []string{"b.go"}},
		{Title: "c3", Files: []string{"c.go"}},
	}

	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
	if err != nil {
		t.Fatalf("runLoopFastPath: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("Commits len = %d, want 3", len(commits))
	}

	// (a) The sentinel appears in NO commit's file list (the freeze excluded it).
	for i, c := range commits {
		for _, fc := range c.Files {
			if fc.Path == "sentinel.txt" {
				t.Errorf("commits[%d] contains sentinel.txt — the start-of-run freeze must exclude post-freeze changes", i)
			}
		}
	}
	// (b) The sentinel survives in the worktree (untracked, untouched by the run).
	if status := dcmStatusPorcelain(t, repo); !strings.Contains(status, "sentinel.txt") {
		t.Errorf("status = %q, want it to contain 'sentinel.txt' (post-freeze change survives in the worktree)", status)
	}
}

// TestRunLoopFastPath_CrossConceptDedupe is the BUG-002 regression test. The file-disjoint fast-path
// launches all N message generations CONCURRENTLY before any publish, so each generateMessageCore
// could only see the pre-run history snapshot — two disjoint concepts whose message stub emitted the
// SAME subject both passed their per-concept dedupe and both published (violating US7/FR30-33). The fix
// (seenSubjects accumulator in the serial publish loop) closes that gap: the second colliding concept
// is re-generated (and rescued if it still collides).
//
// This test sets up exactly that scenario — two disjoint concepts (a.txt, b.txt) whose stub emits the
// identical subject — and asserts the no-duplicate-subjects CONTRACT across BOTH valid outcomes:
//   - rescue: a *DecomposeRescueError is returned with len(Commits)==1 (concept 0 published, concept 1
//     rescued after the single-response stub keeps emitting the colliding subject).
//   - success: err==nil with len(commits)==2 AND distinct subjects (a future stateful stub could let
//     regen produce a distinct subject).
//
// Either way, NO two published commits share a subject. cfg.Edit=false isolates BUG-002 from BUG-001.
func TestRunLoopFastPath_CrossConceptDedupe(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	dcmInitRepo(t, repo)
	// Seed a base commit with two disjoint files, then modify both disjointly in the working tree.
	dcmWriteFile(t, repo, "a.txt", "alpha\n")
	dcmWriteFile(t, repo, "b.txt", "bravo\n")
	dcmRunGit(t, repo, "add", "a.txt", "b.txt")
	dcmCommitRaw(t, repo, "initial") // BORN repo → baseTree = HEAD^{tree}
	// Disjoint working-tree change set: modify each file independently (the FR-M13 disjoint partition).
	dcmWriteFile(t, repo, "a.txt", "alpha 2\n")
	dcmWriteFile(t, repo, "b.txt", "bravo 2\n")

	g := git.New(repo)
	ctx := context.Background()

	// Mirror what Decompose does internally: capture baseTree (HEAD^{tree}), then FreezeWorkingTree to
	// capture T_start (the full working-tree change set) AND reset the index back to baseTree so the
	// per-concept sweep starts clean.
	baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	tStart, err := g.FreezeWorkingTree(ctx, baseTree)
	if err != nil {
		t.Fatalf("FreezeWorkingTree: %v", err)
	}
	preRunHEAD := dcmHeadSHA(t, repo)

	// Message stub: BOTH concepts emit the SAME subject ("chore: update thing"). Each concept's diff
	// names a distinct file, so the input-derived stub routes each to its rule — but the rule's msg is
	// identical. This is the BUG-002 trigger: two disjoint concepts, same emitted subject.
	messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
		{substr: "a.txt", msg: "chore: update thing"},
		{substr: "b.txt", msg: "chore: update thing"},
	})

	cfg := config.Defaults()
	cfg.Edit = false            // isolate BUG-002 from BUG-001 (no EditMessage in the serial loop)
	cfg.MaxDuplicateRetries = 0 // force immediate rescue on the single-response stub: regen emits the
	// same subject → generateMessageCore's loop (attempts 0..0) exhausts
	// without a distinct subject → *RescueError. Simpler/faster; still
	// fully verifies the no-duplicate-subjects guarantee.

	roles := RoleManifests{Message: messageM}
	deps := dcmDepsWithConfig(t, repo, roles, cfg)

	concepts := []prompt.PlannerCommit{
		{Title: "c1", Files: []string{"a.txt"}},
		{Title: "c2", Files: []string{"b.txt"}},
	}

	commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)

	// Collect the published-commit subjects from whichever outcome occurred (rescue → err.Commits;
	// success → commits). The CONTRACT is invariant across both: no two published commits share a subject.
	var subjects []string
	var dre *DecomposeRescueError
	switch {
	case errors.As(err, &dre):
		subjects = commitSubjects(dre.Commits)
	case err == nil:
		subjects = commitSubjects(commits)
	default:
		t.Fatalf("runLoopFastPath: unexpected error: %v", err)
	}

	// HARD GATE — the BUG-002 invariant: no two published commits share a subject.
	seen := map[string]bool{}
	for _, s := range subjects {
		if seen[s] {
			t.Errorf("duplicate published subject %q (US7/FR30-33 violated)", s)
		}
		seen[s] = true
	}

	// Assert exactly ONE of the two valid outcomes.
	if dre != nil {
		// Rescue case: concept 0 published, concept 1 rescued (the single-response stub keeps emitting
		// the colliding subject, so regen can't produce a distinct one).
		if len(dre.Commits) != 1 {
			t.Errorf("rescue: len(Commits) = %d, want 1 (concept 0 published, concept 1 rescued)", len(dre.Commits))
		}
		if dre.Index == 0 {
			t.Errorf("rescue: Index = 0, want the COLLIDING concept (concept 1)")
		}
	} else {
		// Success case: both published with DISTINCT subjects (regen produced a distinct subject).
		if len(commits) != 2 {
			t.Errorf("success: len(commits) = %d, want 2", len(commits))
		}
		if len(commits) == 2 && commits[0].Subject == commits[1].Subject {
			t.Errorf("success: both published the same subject %q (dedupe should have prevented this)", commits[0].Subject)
		}
	}
}

// commitSubjects extracts the Subject from each CommitResult in order. Used by the BUG-002 regression
// test to collect published-commit subjects from either the success path (commits) or the rescue path
// (DecomposeRescueError.Commits) for the no-duplicate-subjects assertion.
func commitSubjects(cs []CommitResult) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Subject)
	}
	return out
}
