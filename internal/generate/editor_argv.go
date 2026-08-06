package generate

import "strings"

// parseEditorArgv splits an editor command string into an argv suitable for
// exec.Command. It performs a simple whitespace split (the common case: a path
// or "prog --flag"); shell quoting/escapes are intentionally NOT supported -- a
// caller needing those should set a bare program path. Used by the Windows
// editor invocation (editor_run_windows.go) where `sh` is not on PATH so the
// command cannot be shell-interpreted. Empty editor returns nil.
func parseEditorArgv(editor string) []string {
	editor = strings.TrimSpace(editor)
	if editor == "" {
		return nil
	}
	return strings.Fields(editor)
}
