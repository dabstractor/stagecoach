//go:build !windows

package generate

import (
	"context"
	"os"
	"os/exec"
)

// runEditorCommand invokes the resolved editor command string on the EDITMSG
// file path, plumbing interactive stdio. The editor string is shell-interpreted
// (it may contain args, e.g. "code --wait"); on Unix we honor that via `sh -c`
// (git parity — git itself runs the editor through the shell). On Windows there
// is no `sh` on PATH (Git for Windows keeps it inside its MSYS root, not on the
// user's PATH), so the editor is split into an argv and execed directly. This
// handles every common editor (a bare path, "code --wait", "vim -f", ...) and
// the test stubs (a compiled binary path with no args). See editor_run_windows.go.
func runEditorCommand(ctx context.Context, editor, editMsgPath string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", editor+" \"$@\"", "--", editMsgPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

