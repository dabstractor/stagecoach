// Package ui holds stagehand's user-facing process artifacts: the canonical
// exit-code constants consumed by cmd/stagehand (and every error/timeout/rescue
// path) plus the output/rendering helpers.
//
// This file defines the single, frozen set of process exit-code constants
// mandated by PRD §15.4. Every code path across the binary maps to one of
// these values; the sibling exitcode_test.go asserts the exact integers so
// downstream error mapping (M7.T2.S1) and rescue/timeout paths (M6) never
// drift from the documented contract.
package ui

// ExitSuccess is the process exit code for a successful run: a commit was
// created, or a dry-run message was printed (PRD §15.4).
const ExitSuccess int = 0

// ExitError is the process exit code for a general error: generation failed,
// parse failed after retries, agent missing, or a CAS conflict per
// decisions.md §7 (PRD §15.4).
const ExitError int = 1

// ExitNothingToCommit is the process exit code when there is nothing to
// commit: a clean tree after auto-stage, or nothing staged with
// --no-auto-stage (FR17; PRD §15.4).
const ExitNothingToCommit int = 2

// ExitRescue is the process exit code for a rescue condition: a snapshot was
// taken, no commit was created, and manual recovery instructions were printed
// (§9.10 / Appendix B.5; PRD §15.4).
const ExitRescue int = 3

// ExitTimeout is the process exit code when generation exceeded --timeout.
// The value 124 deliberately mirrors the GNU timeout utility's own
// exit-124-on-timeout convention so a stagehand invocation wrapped under
// timeout reports a timeout consistently (PRD §15.4).
const ExitTimeout int = 124
