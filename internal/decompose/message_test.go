package decompose

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dustin/stagehand/internal/config"
	"github.com/dustin/stagehand/internal/generate"
	"github.com/dustin/stagehand/internal/git"
	"github.com/dustin/stagehand/internal/provider"
	"github.com/dustin/stagehand/internal/stubtest"
	"github.com/dustin/stagehand/internal/ui"
)

// --- Fixture helpers (msg*-prefixed to avoid colliding with planner_test.go's un-prefixed
//     copies AND stager_test.go's stg* copies — all in package decompose) ---

func msgInitRepo(t *testing.T, dir string) {
	t.Helper()
	msgRunGit(t, dir, "init")
	msgRunGit(t, dir, "config", "user.name", "Test")
	msgRunGit(t, dir, "config", "user.email", "test@example.com")
}

func msgWriteFile(t *testing.T, dir, name, body string) {
	t.Helper()
	full := dir + string(os.PathSeparator) + name
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("msgWriteFile %s: %v", full, err)
	}
}

func msgStageFile(t *testing.T, dir, name string) {
	t.Helper()
	msgRunGit(t, dir, "add", name)
}

func msgCommitRaw(t *testing.T, dir, msg string) {
	t.Helper()
	msgRunGit(t, dir, "commit", "--allow-empty", "-m", msg)
}

func msgRunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func msgGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return msgRunGit(t, dir, args...)
}

func msgHeadSHA(t *testing.T, dir string) string {
	t.Helper()
	return msgGitOut(t, dir, "rev-parse", "HEAD")
}

var msgShaRe = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// messageDeps builds a minimal Deps for message tests (no ResolveRoles).
func messageDeps(t *testing.T, repo string, m provider.Manifest) Deps {
	t.Helper()
	return Deps{
		Git:     git.New(repo),
		Config:  config.Defaults(),
		Roles:   RoleManifests{Message: m},
		Verbose: nil,
	}
}

// --- generateMessage tests ---

func TestGenerateMessage_Success(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	// Build two trees via git add + write-tree.
	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add b"})
	deps := messageDeps(t, repo, m)

	msg, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err != nil {
		t.Fatalf("generateMessage: %v", err)
	}
	if msg != "feat: add b" {
		t.Errorf("msg = %q, want %q", msg, "feat: add b")
	}
}

func TestGenerateMessage_DedupeRetryThenSuccess(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "feat: existing") // HEAD subject = "feat: existing"

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.NewScript(t, bin, []string{"feat: existing", "feat: fresh"})
	deps := messageDeps(t, repo, m)

	msg, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err != nil {
		t.Fatalf("generateMessage: %v", err)
	}
	if msg != "feat: fresh" {
		t.Errorf("msg = %q, want %q (duplicate should have been rejected)", msg, "feat: fresh")
	}
}

func TestGenerateMessage_ParseFailRescue(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.NewScript(t, bin, []string{""})
	cfg := config.Defaults()
	cfg.MaxDuplicateRetries = 0 // single attempt → blank → loop exhausted → rescue
	deps := Deps{
		Git:    git.New(repo),
		Config: cfg,
		Roles:  RoleManifests{Message: m},
	}

	_, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err == nil {
		t.Fatal("expected error on parse-fail rescue, got nil")
	}

	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	if re.Kind != generate.ErrRescue {
		t.Errorf("re.Kind = %v, want ErrRescue", re.Kind)
	}
	if re.TreeSHA != treeB {
		t.Errorf("re.TreeSHA = %q, want %q", re.TreeSHA, treeB)
	}
}

func TestGenerateMessage_Timeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	cfg := config.Defaults()
	cfg.Timeout = 100 * time.Millisecond
	m := stubtest.Manifest(bin, stubtest.Options{SleepMS: 2000})
	deps := messageDeps(t, repo, m)
	deps.Config = cfg

	_, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err == nil {
		t.Fatal("expected error on timeout, got nil")
	}

	var re *generate.RescueError
	if !errors.As(err, &re) {
		t.Fatalf("error type = %T, want *RescueError", err)
	}
	if re.Kind != generate.ErrTimeout {
		t.Errorf("re.Kind = %v, want ErrTimeout", re.Kind)
	}
	if !errors.Is(err, generate.ErrTimeout) {
		t.Errorf("errors.Is(err, generate.ErrTimeout) = false, want true")
	}
}

func TestGenerateMessage_EmptyDiff(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	tree := msgGitOut(t, repo, "write-tree")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: x"})
	deps := messageDeps(t, repo, m)

	_, err := generateMessage(context.Background(), deps, tree, tree)
	if err == nil {
		t.Fatal("expected error on empty diff, got nil")
	}
	if !errors.Is(err, ErrMessageFailed) {
		t.Errorf("errors.Is(err, ErrMessageFailed) = false, error = %v", err)
	}
	if !strings.Contains(err.Error(), "empty concept diff") {
		t.Errorf("error message does not contain 'empty concept diff': %v", err)
	}
}

// --- publishCommit tests ---

func TestPublishCommit_Success(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo)

	msgWriteFile(t, repo, "new.txt", "hello\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	newSHA, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: add new")
	if err != nil {
		t.Fatalf("publishCommit: %v", err)
	}
	if !msgShaRe.MatchString(newSHA) {
		t.Errorf("newSHA = %q, want hex SHA", newSHA)
	}
	if got := msgHeadSHA(t, repo); got != newSHA {
		t.Errorf("HEAD = %q, want %q", got, newSHA)
	}
	logMsg := msgGitOut(t, repo, "log", "--format=%B", "-n1", newSHA)
	if logMsg != "feat: add new" {
		t.Errorf("git log message = %q, want %q", logMsg, "feat: add new")
	}
	headTree := msgGitOut(t, repo, "rev-parse", "HEAD^{tree}")
	if headTree != tree {
		t.Errorf("HEAD tree = %q, want %q", headTree, tree)
	}
}

func TestPublishCommit_RootCommit(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo) // UNBORN — no commits yet

	msgWriteFile(t, repo, "root.txt", "x\n")
	msgStageFile(t, repo, "root.txt")
	tree := msgGitOut(t, repo, "write-tree")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	newSHA, err := publishCommit(context.Background(), deps, tree, "", "feat: root")
	if err != nil {
		t.Fatalf("publishCommit: %v", err)
	}
	if got := msgHeadSHA(t, repo); got != newSHA {
		t.Errorf("HEAD = %q, want %q", got, newSHA)
	}
	// Verify no parent line.
	parents := msgGitOut(t, repo, "log", "--format=%P", "-n1")
	if parents != "" {
		t.Errorf("root commit has parent = %q, want empty", parents)
	}
}

func TestPublishCommit_CASFailure(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")
	parentSHA := msgHeadSHA(t, repo) // == X

	// Pre-move HEAD via a concurrent commit (HEAD X→Z).
	msgCommitRaw(t, repo, "concurrent")
	actualZ := msgHeadSHA(t, repo) // == Z

	msgWriteFile(t, repo, "new.txt", "data\n")
	msgStageFile(t, repo, "new.txt")
	tree := msgGitOut(t, repo, "write-tree")

	deps := messageDeps(t, repo, stubtest.Manifest(bin, stubtest.Options{}))

	_, err := publishCommit(context.Background(), deps, tree, parentSHA, "feat: msg")
	if err == nil {
		t.Fatal("expected error on CAS failure, got nil")
	}

	var ce *generate.CASError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CASError", err)
	}
	if ce.Expected != parentSHA {
		t.Errorf("ce.Expected = %q, want %q (parentSHA)", ce.Expected, parentSHA)
	}
	if ce.Actual != actualZ {
		t.Errorf("ce.Actual = %q, want %q (actualZ)", ce.Actual, actualZ)
	}
	// HEAD UNMOVED — the CAS refused to clobber.
	if got := msgHeadSHA(t, repo); got != actualZ {
		t.Errorf("HEAD = %q, want %q (unchanged)", got, actualZ)
	}
	if !strings.Contains(ce.Error(), "HEAD moved") {
		t.Errorf("CASError.Error() does not contain 'HEAD moved': %s", ce.Error())
	}
}

func TestGenerateMessage_ResolvesSubProvider(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	msgInitRepo(t, repo)
	msgCommitRaw(t, repo, "initial")

	// Build two trees via git add + write-tree (non-empty diff so generateMessage doesn't short-circuit).
	msgWriteFile(t, repo, "a.txt", "a\n")
	msgStageFile(t, repo, "a.txt")
	treeA := msgGitOut(t, repo, "write-tree")

	msgWriteFile(t, repo, "b.txt", "b\n")
	msgStageFile(t, repo, "b.txt")
	treeB := msgGitOut(t, repo, "write-tree")

	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: add b"})
	pflag, dp := "--provider", "openrouter"
	m.ProviderFlag, m.DefaultProvider = &pflag, &dp // pi-shaped: merged DefaultProvider MUST be honored

	deps := messageDeps(t, repo, m)
	deps.Config.Provider = "pi" // the manifest NAME — the conflation source; must NOT be emitted

	var buf bytes.Buffer
	deps.Verbose = ui.NewVerbose(&buf, true)

	msg, err := generateMessage(context.Background(), deps, treeA, treeB)
	if err != nil {
		t.Fatalf("generateMessage: %v", err)
	}
	_ = msg

	cmd := buf.String()
	if !strings.Contains(cmd, "--provider openrouter") {
		t.Errorf("message command missing --provider openrouter\ngot: %s", cmd)
	}
	if strings.Contains(cmd, "--provider pi") {
		t.Errorf("message command emits manifest name as sub-provider (conflation)\ngot: %s", cmd)
	}
}
