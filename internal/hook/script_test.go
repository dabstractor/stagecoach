package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookScript_NonStrict(t *testing.T) {
	got := hookScript(false)
	want := "#!/bin/sh\n" +
		"# stagehand prepare-commit-msg hook v1\n" +
		`exec stagehand hook exec "$@"` + "\n"
	if got != want {
		t.Fatalf("hookScript(false) = %q, want %q", got, want)
	}
	lines := strings.Split(got, "\n")
	if lines[0] != "#!/bin/sh" {
		t.Fatalf("first line = %q, want shebang", lines[0])
	}
	if lines[1] != Marker {
		t.Fatalf("second line = %q, want Marker %q", lines[1], Marker)
	}
	if !strings.Contains(got, `exec stagehand hook exec "$@"`) {
		t.Fatalf("hookScript(false) missing expected exec line: %q", got)
	}
	if strings.Contains(got, "--strict") {
		t.Fatalf("hookScript(false) must not contain --strict: %q", got)
	}
}

func TestHookScript_Strict(t *testing.T) {
	got := hookScript(true)
	if !strings.HasPrefix(got, "#!/bin/sh\n"+Marker+"\n") {
		t.Fatalf("hookScript(true) does not start with shebang + Marker: %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	last := lines[len(lines)-1]
	want := `exec stagehand hook exec --strict "$@"`
	if last != want {
		t.Fatalf("hookScript(true) last line = %q, want %q", last, want)
	}
}

func TestHookScript_MarkerPresent(t *testing.T) {
	if !strings.Contains(hookScript(false), Marker) {
		t.Fatalf("hookScript(false) does not contain Marker")
	}
	if !strings.Contains(hookScript(true), Marker) {
		t.Fatalf("hookScript(true) does not contain Marker")
	}
}

func TestHookScript_POSIX(t *testing.T) {
	shPath, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not found on PATH; skipping POSIX syntax check")
	}

	for _, tc := range []struct {
		name   string
		script string
	}{
		{"non-strict", hookScript(false)},
		{"strict", hookScript(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "prepare-commit-msg")
			if err := os.WriteFile(f, []byte(tc.script), ScriptMode); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}
			cmd := exec.Command(shPath, "-n", f)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("sh -n %s failed: %v\n%s", f, err, out)
			}
		})
	}
}

func TestScriptMode(t *testing.T) {
	if ScriptMode != 0o755 {
		t.Fatalf("ScriptMode = %v, want 0o755", ScriptMode)
	}
}
