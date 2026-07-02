package ui

import "testing"

// TestExitCodes is the golden, table-driven regression guard for the PRD §15.4
// exit-code contract. It asserts every exported Exit* constant equals its
// exact mandated integer so downstream error mapping (cmd/stagehand M7.T2.S1)
// and the rescue/timeout paths (M6) never drift from the documented values.
//
// This is the repo's first _test.go and establishes the repo-wide testing
// pattern: stdlib testing only, table-driven cases, t.Errorf on mismatch.
func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"ExitSuccess", int(ExitSuccess), 0},
		{"ExitError", int(ExitError), 1},
		{"ExitNothingToCommit", int(ExitNothingToCommit), 2},
		{"ExitRescue", int(ExitRescue), 3},
		{"ExitTimeout", int(ExitTimeout), 124},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
}
