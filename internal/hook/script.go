// Package hook holds the primitives for stagehand's git hook mode (PRD §9.20): the
// prepare-commit-msg script template stagehand installs, plus the constants that identify and
// permission it. P1.M3.T1.S2 builds the `hook install|uninstall|status` commands on top of these.
package hook

import "os"

// Marker is the identity line stagehand writes as the SECOND line of its prepare-commit-msg hook (after the
// shebang). Its presence is how `hook status`/`hook uninstall` (P1.M3.T1.S2) recognize a stagehand-owned
// hook (marker present → ours, rewrite/remove; absent → foreign, refuse — PRD §9.20 FR-H2/FR-H3).
const Marker = "# stagehand prepare-commit-msg hook v1"

// ScriptMode is the file mode stagehand writes the hook with (executable — PRD §9.20 FR-H1).
const ScriptMode os.FileMode = 0o755

// hookScript returns the exact bytes of the prepare-commit-msg hook stagehand installs (PRD §9.20 FR-H1).
// It is strict POSIX sh (no bashisms) so it runs under git-for-windows' sh (Appendix E #15). When strict is
// true the runtime call gets `--strict` (PRD §9.20 FR-H5: failures then abort the commit). The trailing
// newline keeps the file POSIX-clean.
func hookScript(strict bool) string {
	run := `exec stagehand hook exec "$@"`
	if strict {
		run = `exec stagehand hook exec --strict "$@"`
	}
	return "#!/bin/sh\n" + Marker + "\n" + run + "\n"
}
