package stubtest

import (
	"io"
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
		build := exec.Command(goPath, "build", "-buildvcs=false", "-o", editorPath, "github.com/dabstractor/stagecoach/cmd/stubeditor")
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
		build := exec.Command(goPath, "build", "-buildvcs=false", "-o", teePath, "github.com/dabstractor/stagecoach/cmd/stubtee")
		if root := moduleRoot(); root != "" {
			build.Dir = root
		}
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stubtee: %v\n%s", err, out)
		}
	})
	return teePath
}

var (
	cliOnce sync.Once
	cliPath string
)

// BuildCLI compiles ./cmd/stubcli ONCE per test process (cached) and returns its
// path. stubcli is the cross-platform compiled twin of the historical #!/bin/sh
// provider stubs used by the `models` live-list tests (echo model lines / exit N /
// sleep), driven by STAGECOACH_STUBCLI_* env vars. Skips t if the go toolchain
// isn't on PATH.
func BuildCLI(t testing.TB) string {
	t.Helper()
	cliOnce.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build stubcli: %v", err)
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-stubcli-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		name := "stubcli"
		if runtime.GOOS == "windows" {
			name = "stubcli.exe"
		}
		cliPath = filepath.Join(dir, name)
		build := exec.Command(goPath, "build", "-buildvcs=false", "-o", cliPath, "github.com/dabstractor/stagecoach/cmd/stubcli")
		if root := moduleRoot(); root != "" {
			build.Dir = root
		}
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build stubcli: %v\n%s", err, out)
		}
	})
	return cliPath
}

// CLIOptions configures the stub provider CLI via STAGECOACH_STUBCLI_* env vars.
type CLIOptions struct {
	Out     string // STAGECOACH_STUBCLI_OUT (stdout model lines; "" prints nothing)
	Exit    int    // STAGECOACH_STUBCLI_EXIT (default 0)
	SleepMS int    // STAGECOACH_STUBCLI_SLEEP_MS (default 0; >0 simulates a slow/hanging provider)
}

// PlaceCLI builds stubcli and installs it into dir under the given provider name
// (e.g. "opencode"), returning the env assignments for o. The caller prepends dir
// to PATH (e.g. t.Setenv("PATH", dir+...)). On Windows the name gets a .exe suffix
// and a symlink-or-copy fallback (symlink needs a privilege). Mirrors the old
// #!/bin/sh stubs that wrote a script named after the provider into a tmp dir.
func PlaceCLI(t *testing.T, dir, name string, o CLIOptions) []string {
	t.Helper()
	bin := BuildCLI(t)
	linkName := name
	if runtime.GOOS == "windows" {
		linkName = name + ".exe"
	}
	dst := filepath.Join(dir, linkName)
	if err := os.Symlink(bin, dst); err != nil {
		if cerr := copyFile(bin, dst); cerr != nil {
			t.Fatalf("symlink stubcli %s: %v; copy fallback: %v", name, err, cerr)
		}
	}
	var env []string
	env = append(env, "STAGECOACH_STUBCLI_OUT="+o.Out)
	if o.Exit != 0 {
		env = append(env, "STAGECOACH_STUBCLI_EXIT="+itoa(o.Exit))
	}
	if o.SleepMS > 0 {
		env = append(env, "STAGECOACH_STUBCLI_SLEEP_MS="+itoa(o.SleepMS))
	}
	return env
}

// copyFile copies src to dst (symlink-fallback for privilege-less Windows).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// buildOnceFor compiles a cmd/stub* binary ONCE per process (cached), mirroring
// Build/BuildEditor/BuildTee/BuildCLI. name is the output basename (gets .exe on
// Windows); pkg is the import path. Returns the binary path. Shared by the
// decompose arbiter/capture stubs.
func buildOnceFor(t testing.TB, once *sync.Once, path *string, name, pkg, label string) string {
	t.Helper()
	once.Do(func() {
		goPath, err := exec.LookPath("go")
		if err != nil {
			t.Skipf("go toolchain not on PATH; cannot build %s: %v", label, err)
			return
		}
		dir, err := os.MkdirTemp("", "stagecoach-"+label+"-*")
		if err != nil {
			t.Fatalf("mkdtemp: %v", err)
		}
		binName := name
		if runtime.GOOS == "windows" {
			binName = name + ".exe"
		}
		*path = filepath.Join(dir, binName)
		build := exec.Command(goPath, "build", "-buildvcs=false", "-o", *path, pkg)
		if root := moduleRoot(); root != "" {
			build.Dir = root
		}
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v\n%s", label, err, out)
		}
	})
	return *path
}

var (
	arbiterOnce sync.Once
	arbiterPath string
)

// BuildArbiter compiles ./cmd/stubarbiter (cached) — the cross-platform twin of
// the decompose arbiter.sh /bin/sh script. Mode is selected at RUN time via the
// STAGECOACH_ARBITER_MODE env var ("tip"|"mid"); use ArbiterModeEnv to build it.
func BuildArbiter(t testing.TB) string {
	t.Helper()
	return buildOnceFor(t, &arbiterOnce, &arbiterPath, "stubarbiter",
		"github.com/dabstractor/stagecoach/cmd/stubarbiter", "stubarbiter")
}

// ArbiterModeEnv returns the env assignment selecting the arbiter's SHA choice.
func ArbiterModeEnv(mode string) string { return "STAGECOACH_ARBITER_MODE=" + mode }

var (
	captureOnce sync.Once
	capturePath string
)

// BuildCapture compiles ./cmd/stubcapture (cached) — the cross-platform twin of
// the decompose capture.sh /bin/sh script (writes stdin to a file, prints a
// canned response). Capture file + output are selected at RUN time via
// STAGECOACH_CAPTURE_FILE / STAGECOACH_CAPTURE_OUT; use CaptureEnv to build them.
func BuildCapture(t testing.TB) string {
	t.Helper()
	return buildOnceFor(t, &captureOnce, &capturePath, "stubcapture",
		"github.com/dabstractor/stagecoach/cmd/stubcapture", "stubcapture")
}

// CaptureEnv returns the env assignments driving stubcapture.
func CaptureEnv(file, out string) []string {
	return []string{
		"STAGECOACH_CAPTURE_FILE=" + file,
		"STAGECOACH_CAPTURE_OUT=" + out,
	}
}
