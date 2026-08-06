// Command stubeditor is a tiny fake-GIT_EDITOR binary for Stagecoach's --edit
// gate tests (PRD §9.22 FR-E1). git invokes it as $GIT_EDITOR <commit-msg-path>;
// it either rewrites that file with a configured message or truncates it, driven
// entirely by STAGECOACH_EDITOR_* environment variables. STDLIB ONLY. It is the
// cross-platform twin of the historical fakeeditor.sh /bin/sh script (a compiled
// binary works on every OS with no shell-quoting headaches).
package main

import (
	"os"
	"strconv"
)

func main() {
	path := ""
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	// Exit-code simulation (e.g. vim :cq). Non-zero aborts the commit upstream.
	if code := os.Getenv("STAGECOACH_EDITOR_EXIT"); code != "" {
		if n, err := strconv.Atoi(code); err == nil && n != 0 {
			os.Exit(n)
		}
	}

	if path == "" {
		os.Exit(0)
	}

	// Rewrite mode: overwrite the file with the configured message (the
	// "edited subject" case). Empty STAGECOACH_EDITOR_MSG => truncate (the
	// ErrEmptyMessage case).
	msg := os.Getenv("STAGECOACH_EDITOR_MSG")
	if err := os.WriteFile(path, []byte(msg), 0o644); err != nil {
		// Best-effort; a write failure surfaces upstream as a non-zero exit.
		os.Exit(1)
	}
	os.Exit(0)
}
