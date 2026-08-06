package exitcode

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dabstractor/stagecoach/internal/generate"
)

func TestFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil → Success", nil, Success},
		{"explicit ExitError NothingToCommit", New(NothingToCommit, errors.New("x")), NothingToCommit},
		{"explicit ExitError custom code", New(7, errors.New("custom")), 7},
		{"explicit ExitError Busy (silent, nil err) → 5", New(Busy, nil), Busy},
		{"wrapped ExitError → Code (errors.As traverses %w)", fmt.Errorf("wrap: %w", New(7, errors.New("x"))), 7},
		{"wrapped ExitError NothingToCommit beats sentinel branch", fmt.Errorf("%w", New(NothingToCommit, errors.New("y"))), NothingToCommit},
		{"ErrNothingToCommit", generate.ErrNothingToCommit, NothingToCommit},
		{"ErrEmptyMessage → 1", generate.ErrEmptyMessage, Error},
		{"RescueError(ErrRescue) → 3", &generate.RescueError{Kind: generate.ErrRescue}, Rescue},
		{"RescueError(ErrTimeout) → 124 (timeout before rescue)", &generate.RescueError{Kind: generate.ErrTimeout}, Timeout},
		{"wrapped ErrTimeout → 124", fmt.Errorf("wrap: %w", generate.ErrTimeout), Timeout},
		{"context.DeadlineExceeded → 124", context.DeadlineExceeded, Timeout},
		{"ErrCASFailed → 1", generate.ErrCASFailed, Error},
		{"CASError → 1", &generate.CASError{Expected: "a", Actual: "b"}, Error},
		{"generic error → 1", errors.New("anything else"), Error},
		{"ErrUpdateAvailable → 6", ErrUpdateAvailable, UpdateAvailable},
		{"wrapped ErrUpdateAvailable → 6 (errors.Is traverses %w)", fmt.Errorf("wrap: %w", ErrUpdateAvailable), UpdateAvailable},
		{"explicit ExitError UpdateAvailable (nil err) → 6", New(UpdateAvailable, nil), UpdateAvailable},
		{"explicit ExitError UpdateAvailable (with err) → 6", New(UpdateAvailable, errors.New("newer")), UpdateAvailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := For(tc.err)
			if got != tc.want {
				t.Errorf("For(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestBusyCodeValue(t *testing.T) {
	// Busy is 5 — distinct from 0/1/2/3/124; 4 is reserved (integration_seams §2).
	if Busy != 5 {
		t.Errorf("Busy = %d, want 5", Busy)
	}
}

func TestFor_NoCommitPathYieldsUpdateAvailable(t *testing.T) {
	commitPathErrors := []error{
		generate.ErrNothingToCommit,
		generate.ErrEmptyMessage,
		&generate.RescueError{Kind: generate.ErrRescue},
		&generate.RescueError{Kind: generate.ErrTimeout},
		generate.ErrTimeout,
		context.DeadlineExceeded,
		generate.ErrCASFailed,
		&generate.CASError{Expected: "a", Actual: "b"},
		errors.New("generic commit-path failure"),
	}
	for _, err := range commitPathErrors {
		if got := For(err); got == UpdateAvailable {
			t.Errorf("commit-path error %v mapped to UpdateAvailable(6) -- 6 must be upgrade-path only", err)
		}
	}
}

func TestUpdateAvailableCodeValue(t *testing.T) {
	if UpdateAvailable != 6 {
		t.Errorf("UpdateAvailable = %d, want 6", UpdateAvailable)
	}
}

func TestExitError_NilErr(t *testing.T) {
	ee := New(Error, nil)
	if ee.Error() != "" {
		t.Errorf("ExitError{Err:nil}.Error() = %q, want empty", ee.Error())
	}
	if code := For(ee); code != Error {
		t.Errorf("For(ExitError{Code:1,Err:nil}) = %d, want 1", code)
	}
}
