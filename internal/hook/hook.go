// Package hook provides the lifecycle logic for stagehand's per-repo prepare-commit-msg hook
// (PRD §9.20 FR-H1/FR-H2/FR-H3/FR-H5). Detect, Install, and Uninstall operate on a hooks
// directory path (no git dependency) so they are unit-testable with a bare temp dir.
// This file extends P1.M3.T1.S1's script.go primitives (Marker, ScriptMode, hookScript).
package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// HookFilename is the git hook stagehand manages (PRD §9.20 — prepare-commit-msg only).
const HookFilename = "prepare-commit-msg"

// Status is the state of a repo's prepare-commit-msg hook (PRD §9.20 FR-H3).
type Status int

const (
	StatusNone      Status = iota // no prepare-commit-msg file
	StatusStagehand               // stagehand-owned (Marker present)
	StatusForeign                 // a hook file exists WITHOUT our Marker (never touch it)
)

// String renders the FR-H3 report tokens EXACTLY: "none" / "stagehand (v1)" / "foreign".
func (s Status) String() string {
	switch s {
	case StatusStagehand:
		return "stagehand (v1)"
	case StatusForeign:
		return "foreign"
	default:
		return "none"
	}
}

// Sentinels for the refusal paths (FR-H2 / FR-H3). Callers use errors.Is.
var (
	ErrForeignHook = errors.New("a foreign prepare-commit-msg hook exists")
	ErrNoHook      = errors.New("no stagehand prepare-commit-msg hook is installed")
)

// Detect examines the hooks directory and returns the current hook status.
// os.ErrNotExist → StatusNone; any other read error is returned.
// A file without Marker → StatusForeign; a file with Marker → StatusStagehand.
func Detect(hooksDir string) (Status, error) {
	data, err := os.ReadFile(filepath.Join(hooksDir, HookFilename))
	if errors.Is(err, os.ErrNotExist) {
		return StatusNone, nil
	}
	if err != nil {
		return StatusNone, err
	}
	if strings.Contains(string(data), Marker) {
		return StatusStagehand, nil
	}
	return StatusForeign, nil
}

// Install writes the stagehand prepare-commit-msg hook into hooksDir.
// For StatusNone or StatusStagehand (idempotent rewrite), it creates the dir if absent
// and writes the script with mode 0o755 (os.Chmod after WriteFile to defeat umask).
// For StatusForeign it returns ErrForeignHook WITHOUT touching the file.
// It returns the previous status so the caller can print "Installed" vs "Updated".
func Install(hooksDir string, strict bool) (Status, error) {
	prev, err := Detect(hooksDir)
	if err != nil {
		return prev, err
	}
	if prev == StatusForeign {
		return prev, ErrForeignHook
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return prev, err
	}
	p := filepath.Join(hooksDir, HookFilename)
	if err := os.WriteFile(p, []byte(hookScript(strict)), ScriptMode); err != nil {
		return prev, err
	}
	if err := os.Chmod(p, ScriptMode); err != nil {
		return prev, err
	}
	return prev, nil
}

// Uninstall removes the stagehand prepare-commit-msg hook.
// StatusStagehand → removes the file. StatusForeign → ErrForeignHook (untouched).
// StatusNone → ErrNoHook (idempotent — nothing to remove).
func Uninstall(hooksDir string) (Status, error) {
	st, err := Detect(hooksDir)
	if err != nil {
		return st, err
	}
	switch st {
	case StatusStagehand:
		return st, os.Remove(filepath.Join(hooksDir, HookFilename))
	case StatusForeign:
		return st, ErrForeignHook
	default:
		return st, ErrNoHook
	}
}

// Script returns the hook script content for the given strict mode.
// Exported so the cmd layer's `install --print` can access the unexported hookScript.
func Script(strict bool) string { return hookScript(strict) }

// InvocationLine returns the exec line baked into the hook script.
// Kept consistent with hookScript via drift-guard tests.
func InvocationLine(strict bool) string {
	if strict {
		return `exec stagehand hook exec --strict "$@"`
	}
	return `exec stagehand hook exec "$@"`
}
