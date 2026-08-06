package stubtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// EditorOptions configures the stub editor's behavior via STAGECOACH_EDITOR_*
// env vars. It mirrors the old fakeeditor.sh scripts (rewrite message / truncate
// to empty -> ErrEmptyMessage / non-zero exit -> abort).
type EditorOptions struct {
	// Msg is the message to write into the editor's argv[1] file. "" truncates
	// the file (the empty-message -> ErrEmptyMessage case).
	Msg string
	// Exit is the editor's exit code. 0 (default) = success; non-zero simulates
	// an editor abort (e.g. vim :cq) -> a wrapped error, NOT a commit.
	Exit int
}

var (
	editorOnce sync.Once
	editorPath string
)

// BuildEditor compiles ./cmd/stubeditor ONCE per test process (cached) and
// returns its path. The compiled binary is fully cross-platform (no shell, no
// .bat, no quoting) and replaces the historical fakeeditor.sh /bin/sh scripts
// that cannot run on Windows. Skips t if the go toolchain isn't on PATH.
//
// Set GIT_EDITOR to the returned path, then SetEditorEnv to drive the behavior.
func BuildEditor(t testing.TB) string {
	t.Helper()
	editorOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build stubeditor: %v", err)
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-stubeditor-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stubeditor"
		if runtime.GOOS == "windows" {
			name = "stubeditor.exe"
		}
		editorPath = filepath.Join(dir, name)
		build := exec.Command(goPath, "build", "-o", editorPath, "github.com/dabstractor/stagecoach/cmd/stubeditor")
		if root := moduleRoot(); root != "" {
			build.Dir = root
		}
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stubeditor: %v\n%s", err, out)
		}
	})
	return editorPath
}

// EditorEnv returns the STAGECOACH_EDITOR_* env assignments for o, suitable for
// t.Setenv (each element is a "KEY=VALUE" string). Msg maps to
// STAGECOACH_EDITOR_MSG; Exit to STAGECOACH_EDITOR_EXIT (only when non-zero).
func EditorEnv(o EditorOptions) []string {
	var env []string
	env = append(env, "STAGECOACH_EDITOR_MSG="+o.Msg)
	if o.Exit != 0 {
		env = append(env, "STAGECOACH_EDITOR_EXIT="+itoa(o.Exit))
	}
	return env
}

// SetEditorEnv applies EditorEnv(o) via t.Setenv (the test-scoped setter).
func SetEditorEnv(t *testing.T, o EditorOptions) {
	t.Helper()
	for _, kv := range EditorEnv(o) {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				t.Setenv(kv[:i], kv[i+1:])
				break
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

var (
	teeOnce sync.Once
	teePath string
)

// BuildTee compiles ./cmd/stubtee ONCE per test process (cached) and returns its
// path. stubtee is the cross-platform compiled twin of the historical
// tee-wrap.sh /bin/sh script (it cannot be execed directly on Windows, where
// CreateProcess ignores the shebang). The wrapper tees stdin to
// $STAGECOACH_TEST_CAPTURE then re-pipes it into the real stub (argv[1]).
// Skips t if the go toolchain isn't on PATH.
func BuildTee(t testing.TB) string {
	t.Helper()
	teeOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build stubtee: %v", err)
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-stubtee-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stubtee"
		if runtime.GOOS == "windows" {
			name = "stubtee.exe"
		}
		teePath = filepath.Join(dir, name)
		build := exec.Command(goPath, "build", "-o", teePath, "github.com/dabstractor/stagecoach/cmd/stubtee")
		if root := moduleRoot(); root != "" {
			build.Dir = root
		}
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stubtee: %v\n%s", err, out)
		}
	})
	return teePath
}
