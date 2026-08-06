//go:build windows

package generate

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// runEditorCommand invokes the resolved editor on Windows. There is no `sh` on
// PATH (Git for Windows keeps its MSYS `sh` inside its install root, not on the
// user PATH), so the editor command string is split into an argv and execed
// directly. This matches git-for-windows' own behavior for a bare editor path
// (the common case) and the test stub (a compiled binary path with no args).
//
// The Unix twin (editor_run.go) uses `sh -c` for full shell interpretation
// (git parity); the split-on-spaces here is the documented Windows limitation
// (no shell quoting) -- acceptable because Windows editors are almost always a
// bare program path or "prog --flag".
func runEditorCommand(ctx context.Context, editor, editMsgPath string) error {
	argv := parseEditorArgv(editor)
	if len(argv) == 0 {
		return errors.New("no editor configured")
	}
	cmd := exec.CommandContext(ctx, argv[0], append(append([]string{}, argv[1:]...), editMsgPath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
