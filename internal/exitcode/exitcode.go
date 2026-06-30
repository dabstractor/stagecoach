// Package exitcode maps Stagehand errors to PRD §15.4 process exit codes (0/1/2/3/124).
// For() is the single source of truth used by the CLI's main(); it covers explicit *ExitError
// overrides, the generate-domain mapping (nothing-to-commit/rescue/timeout/CAS), and a default
// of 1. §15.4 overrides arch/go_ecosystem_patterns.md §1.2's generic table (2=nothing-to-commit,
// not usage; 3=rescue, not config).
package exitcode

import (
	"context"
	"errors"

	"github.com/dustin/stagehand/internal/generate"
)

// PRD §15.4 exit codes (AUTHORITATIVE — overrides arch/go_ecosystem_patterns.md §1.2's generic table,
// which says 2=usage/3=config; PRD says 2=nothing-to-commit/3=rescue).
const (
	Success         = 0   // commit created, or dry-run message printed
	Error           = 1   // general error (generation failed, parse failed, agent missing, CAS, usage/flag)
	NothingToCommit = 2   // clean tree after auto-stage, or nothing staged with --no-auto-stage
	Rescue          = 3   // snapshot taken, commit not created — manual recovery printed
	Timeout         = 124 // generation exceeded --timeout (mirrors GNU `timeout`)
)

// ExitError lets a command force a specific exit code for an error that For()'s domain mapping
// would otherwise default. Return from any RunE: `return exitcode.New(exitcode.Error, err)`.
// errors.As(err, &ee) recovers Code; Unwrap() returns Err (errors.Is chains through).
type ExitError struct {
	Code int   // the exit code to use
	Err  error // underlying cause; may be nil for a clean non-zero exit
}

func (e *ExitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}

func (e *ExitError) Unwrap() error { return e.Err }

// New wraps err with a forced exit code.
func New(code int, err error) *ExitError { return &ExitError{Code: code, Err: err} }

// For returns the PRD §15.4 exit code for err. Order: nil→0; explicit *ExitError→its Code; then the
// generate-domain mapping (NothingToCommit→2, Rescue→3, Timeout/Deadline→124, CAS→1); else 1.
// A *generate.RescueError whose Kind is ErrTimeout maps to 124 (checked BEFORE the generic rescue→3).
func For(err error) int {
	if err == nil {
		return Success
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	if errors.Is(err, generate.ErrNothingToCommit) {
		return NothingToCommit
	}
	// *RescueError.Unwrap()==Kind; check timeout BEFORE rescue (a timeout IS a rescue with Kind=ErrTimeout).
	if errors.Is(err, generate.ErrTimeout) {
		return Timeout
	}
	if errors.Is(err, generate.ErrRescue) {
		return Rescue
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}
	if errors.Is(err, generate.ErrCASFailed) {
		return Error // CAS is a general (non-rescue) failure per PRD §13.5/§15.4
	}
	return Error
}
