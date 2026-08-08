package upgrade

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// fakeExecRunner is the canned ExecRunner for Delegate tests. It records every Run call (name +
// args) and delegates to a canned func that returns (code, err) and may write to the passed stdout
// (so the streaming test can prove the output reaches opts.Out). Mirrors the canned-fake idiom of
// releases_test.go (newFakeClient) and detect_test.go (fakeRunner).
type fakeExecRunner struct {
	// canned returns the canned (exitCode, err) for a Run call and may write streaming output to
	// stdout (the writer Delegate passes). It is called with the name + args so a test can vary its
	// response per command (e.g. asdf install vs global).
	canned func(name string, args []string, stdout io.Writer) (int, error)
	// calls records every Run call as the full argv (name + args), in order.
	calls [][]string
}

// Run implements ExecRunner: records the call and delegates to canned. The stdout/stderr writers are
// passed through to canned so a test can assert streaming.
func (f *fakeExecRunner) Run(_ context.Context, stdout, stderr io.Writer, name string, args ...string) (int, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return f.canned(name, args, stdout)
}

// okRunner returns a fakeExecRunner that exits 0 with no error and writes nothing.
func okRunner() *fakeExecRunner {
	return &fakeExecRunner{canned: func(string, []string, io.Writer) (int, error) { return 0, nil }}
}

// argvEquals reports whether recorded equals want, element-wise.
func argvEquals(recorded, want []string) bool {
	if len(recorded) != len(want) {
		return false
	}
	for i := range recorded {
		if recorded[i] != want[i] {
			return false
		}
	}
	return true
}

// TestDelegate_RunArgvPerChannel asserts each RUN channel execs its exact FR-U3 argv via the injected
// runner (no real exec). The table covers the 6 non-npm RUN channels; npm is covered by
// TestDelegate_NpmVariant (its argv depends on the injected Env), and asdf's 2-step is covered by
// TestDelegate_AsdfTwoStep.
func TestDelegate_RunArgvPerChannel(t *testing.T) {
	cases := []struct {
		name string
		ch   Channel
		want [][]string
	}{
		{"brew", ChannelBrew, [][]string{{"brew", "upgrade", "stagecoach"}}},
		{"scoop", ChannelScoop, [][]string{{"scoop", "update", "stagecoach"}}},
		{"choco", ChannelChocolatey, [][]string{{"choco", "upgrade", "stagecoach"}}},
		{"mise", ChannelMise, [][]string{{"mise", "upgrade", "stagecoach"}}},
		{"go-install", ChannelGoInstall, [][]string{{"go", "install", "github.com/dabstractor/stagecoach/cmd/stagecoach@latest"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := okRunner()
			res, err := Delegate(context.Background(), tc.ch, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
			if err != nil {
				t.Fatalf("Delegate(%s): unexpected error: %v", tc.name, err)
			}
			if !res.Ran {
				t.Errorf("Ran = false, want true")
			}
			if res.ExitCode != 0 {
				t.Errorf("ExitCode = %d, want 0", res.ExitCode)
			}
			if len(f.calls) != len(tc.want) {
				t.Fatalf("recorded %d calls, want %d: %+v", len(f.calls), len(tc.want), f.calls)
			}
			for i, want := range tc.want {
				if !argvEquals(f.calls[i], want) {
					t.Errorf("call[%d] = %v, want %v", i, f.calls[i], want)
				}
			}
			// Command is the space-joined argv (single step ⇒ no " && " separator).
			wantCmd := joinArgv(tc.want)
			if res.Command != wantCmd {
				t.Errorf("Command = %q, want %q", res.Command, wantCmd)
			}
		})
	}
}

// TestDelegate_RunReturnsExitCode asserts a run-channel non-zero exit is returned as Ran:true +
// ExitCode:code with err==nil (an exit is NOT a Delegate error; the command layer maps the code).
func TestDelegate_RunReturnsExitCode(t *testing.T) {
	t.Run("exit0", func(t *testing.T) {
		f := okRunner()
		res, err := Delegate(context.Background(), ChannelBrew, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Ran || res.ExitCode != 0 {
			t.Errorf("got %+v, want Ran=true ExitCode=0", res)
		}
	})
	t.Run("exit1", func(t *testing.T) {
		f := &fakeExecRunner{canned: func(string, []string, io.Writer) (int, error) { return 1, nil }}
		res, err := Delegate(context.Background(), ChannelBrew, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("a non-zero exit must NOT be a Delegate error; got err=%v", err)
		}
		if !res.Ran {
			t.Errorf("Ran = false, want true (the updater ran)")
		}
		if res.ExitCode != 1 {
			t.Errorf("ExitCode = %d, want 1", res.ExitCode)
		}
	})
}

// TestDelegate_RunStartFailure asserts a start/LookPath failure (the PM binary absent) IS a Delegate
// error: Ran:true (we attempted the run) + err != nil.
func TestDelegate_RunStartFailure(t *testing.T) {
	startErr := errors.New("exec: not found")
	f := &fakeExecRunner{canned: func(string, []string, io.Writer) (int, error) { return 0, startErr }}
	res, err := Delegate(context.Background(), ChannelBrew, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
	if err == nil {
		t.Fatalf("expected a start-failure error, got nil")
	}
	if !errors.Is(err, startErr) {
		t.Errorf("err = %v, want to wrap %v", err, startErr)
	}
	if !res.Ran {
		t.Errorf("Ran = false, want true (we attempted the run)")
	}
}

// TestDelegate_PrintChannels asserts AUR/Nix WRITE the exact command to opts.Out and return
// Ran:false, ExitCode:0, Command=primary. The runner must NOT be called (no exec for print channels).
func TestDelegate_PrintChannels(t *testing.T) {
	cases := []struct {
		name        string
		ch          Channel
		primary     string
		containsAll []string // every one of these substrings must appear in Out
	}{
		{
			name:        "aur",
			ch:          ChannelAUR,
			primary:     "sudo pacman -Syu stagecoach-bin",
			containsAll: []string{"sudo pacman -Syu stagecoach-bin", "yay -Syu stagecoach-bin"},
		},
		{
			name:        "nix",
			ch:          ChannelNix,
			primary:     "nix profile upgrade stagecoach",
			containsAll: []string{"nix profile upgrade stagecoach", "nix flake update"},
		},
		{
			name:        "deb",
			ch:          ChannelDeb,
			primary:     "sudo apt install --only-upgrade stagecoach",
			containsAll: []string{"sudo apt install --only-upgrade stagecoach", "apt repo", "releases/latest"},
		},
		{
			name:        "rpm",
			ch:          ChannelRpm,
			primary:     "sudo dnf upgrade stagecoach",
			containsAll: []string{"sudo dnf upgrade stagecoach", "dnf repo", "releases/latest"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := okRunner() // must NOT be called for print channels.
			var out bytes.Buffer
			res, err := Delegate(context.Background(), tc.ch, DelegateOptions{Exec: f, Out: &out})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Ran {
				t.Errorf("Ran = true, want false (print channel)")
			}
			if res.ExitCode != 0 {
				t.Errorf("ExitCode = %d, want 0 (FR-U4: a printed command exits 0)", res.ExitCode)
			}
			if res.Command != tc.primary {
				t.Errorf("Command = %q, want primary %q", res.Command, tc.primary)
			}
			if len(f.calls) != 0 {
				t.Errorf("Exec.Run was called %d time(s) for a print channel; want 0: %+v", len(f.calls), f.calls)
			}
			got := out.String()
			for _, want := range tc.containsAll {
				if !strings.Contains(got, want) {
					t.Errorf("Out = %q, missing substring %q", got, want)
				}
			}
		})
	}
}

// TestDelegate_NpmVariant asserts the npm-family command syntax follows the injected Env:
// PNPM_HOME ⇒ pnpm add -g; BUN_INSTALL ⇒ bun add -g; nil/empty Env ⇒ npm install -g.
func TestDelegate_NpmVariant(t *testing.T) {
	pkg := "stagecoach-ai@latest"
	cases := []struct {
		name string
		env  func(string) string
		want []string
	}{
		{"pnpm", func(k string) string {
			if k == "PNPM_HOME" {
				return "/home/me/.local/share/pnpm"
			}
			return ""
		}, []string{"pnpm", "add", "-g", pkg}},
		{"bun", func(k string) string {
			if k == "BUN_INSTALL" {
				return "/home/me/.bun"
			}
			return ""
		}, []string{"bun", "add", "-g", pkg}},
		{"npm-default-nil-env", nil, []string{"npm", "install", "-g", pkg}},
		{"npm-default-empty-env", func(string) string { return "" }, []string{"npm", "install", "-g", pkg}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := okRunner()
			_, err := Delegate(context.Background(), ChannelNpm, DelegateOptions{Exec: f, Out: &bytes.Buffer{}, Env: tc.env})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(f.calls) != 1 {
				t.Fatalf("recorded %d calls, want 1", len(f.calls))
			}
			if !argvEquals(f.calls[0], tc.want) {
				t.Errorf("argv = %v, want %v", f.calls[0], tc.want)
			}
		})
	}
}

// TestDelegate_AsdfTwoStep asserts asdf runs install THEN global (2 sequential calls, in order) and
// stops on the first non-zero exit (global is NOT called when install fails).
func TestDelegate_AsdfTwoStep(t *testing.T) {
	t.Run("both-success", func(t *testing.T) {
		f := okRunner()
		res, err := Delegate(context.Background(), ChannelAsdf, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.Ran || res.ExitCode != 0 {
			t.Errorf("got %+v, want Ran=true ExitCode=0", res)
		}
		want := [][]string{
			{"asdf", "install", "stagecoach", "latest"},
			{"asdf", "global", "stagecoach", "latest"},
		}
		if len(f.calls) != 2 {
			t.Fatalf("recorded %d calls, want 2: %+v", len(f.calls), f.calls)
		}
		for i, w := range want {
			if !argvEquals(f.calls[i], w) {
				t.Errorf("call[%d] = %v, want %v", i, f.calls[i], w)
			}
		}
		// Command is the " && "-joined 2-step.
		wantCmd := "asdf install stagecoach latest && asdf global stagecoach latest"
		if res.Command != wantCmd {
			t.Errorf("Command = %q, want %q", res.Command, wantCmd)
		}
	})
	t.Run("install-fails-stops-before-global", func(t *testing.T) {
		f := &fakeExecRunner{canned: func(name string, _ []string, _ io.Writer) (int, error) {
			if name == "asdf" {
				// First call (install) fails; global must never run.
				return 2, nil
			}
			return 0, nil
		}}
		res, err := Delegate(context.Background(), ChannelAsdf, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("a non-zero exit must NOT be a Delegate error; got err=%v", err)
		}
		if !res.Ran || res.ExitCode != 2 {
			t.Errorf("got %+v, want Ran=true ExitCode=2", res)
		}
		if len(f.calls) != 1 {
			t.Fatalf("expected exactly 1 call (global must not run on install failure); got %d: %+v", len(f.calls), f.calls)
		}
		if !argvEquals(f.calls[0], []string{"asdf", "install", "stagecoach", "latest"}) {
			t.Errorf("call[0] = %v, want the install step", f.calls[0])
		}
	})
}

// TestDelegate_DirectErrDirectSwap asserts ChannelDirect returns ErrDirectSwap and Exec.Run is never
// called.
func TestDelegate_DirectErrDirectSwap(t *testing.T) {
	f := okRunner()
	res, err := Delegate(context.Background(), ChannelDirect, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
	if !errors.Is(err, ErrDirectSwap) {
		t.Fatalf("err = %v, want ErrDirectSwap", err)
	}
	if res.Ran {
		t.Errorf("Ran = true for direct, want false")
	}
	if len(f.calls) != 0 {
		t.Errorf("Exec.Run called %d time(s) for direct; want 0", len(f.calls))
	}
}

// TestDelegate_StreamsToWriter asserts the updater's output is STREAMED to opts.Out (not captured):
// the fake writes "BREW-OUT" to the stdout writer Delegate passes, and opts.Out must receive it.
func TestDelegate_StreamsToWriter(t *testing.T) {
	f := &fakeExecRunner{canned: func(_ string, _ []string, stdout io.Writer) (int, error) {
		// Write to the streaming stdout Delegate passed so opts.Out receives the bytes (streaming proof).
		_, _ = io.WriteString(stdout, "BREW-OUT")
		return 0, nil
	}}
	var out bytes.Buffer
	_, err := Delegate(context.Background(), ChannelBrew, DelegateOptions{Exec: f, Out: &out})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "BREW-OUT") {
		t.Errorf("opts.Out = %q, want it to contain the streamed %q (streaming, not capture)", got, "BREW-OUT")
	}
}

// TestDelegate_NeverSudo asserts no RUN-channel argv begins with "sudo" (FR-U4: never auto-sudo).
// sudo may appear ONLY in the AUR print string (which the USER runs).
func TestDelegate_NeverSudo(t *testing.T) {
	runChannels := []Channel{
		ChannelBrew, ChannelScoop, ChannelChocolatey, ChannelNpm, ChannelMise, ChannelAsdf, ChannelGoInstall,
	}
	for _, ch := range runChannels {
		f := okRunner()
		_, _ = Delegate(context.Background(), ch, DelegateOptions{Exec: f, Out: &bytes.Buffer{}})
		for i, call := range f.calls {
			if len(call) == 0 {
				t.Errorf("%s call[%d] is empty", ch, i)
				continue
			}
			if call[0] == "sudo" {
				t.Errorf("%s call[%d] argv begins with %q — FR-U4 forbids auto-sudo on RUN channels: %v",
					ch, i, "sudo", call)
			}
		}
	}
}

// TestDelegate_VerboseHook asserts the Verbose func is invoked with a "running:" message for RUN
// channels and a "printed" message for PRINT channels (and never for the direct/ErrDirectSwap case).
func TestDelegate_VerboseHook(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		var logged []string
		vb := func(msg string) { logged = append(logged, msg) }
		_, _ = Delegate(context.Background(), ChannelBrew, DelegateOptions{Exec: okRunner(), Out: &bytes.Buffer{}, Verbose: vb})
		if len(logged) == 0 {
			t.Fatalf("Verbose not called for a RUN channel")
		}
		if !strings.Contains(logged[0], "running:") {
			t.Errorf("Verbose[0] = %q, want a 'running:' message", logged[0])
		}
	})
	t.Run("print", func(t *testing.T) {
		var logged []string
		vb := func(msg string) { logged = append(logged, msg) }
		_, _ = Delegate(context.Background(), ChannelNix, DelegateOptions{Out: &bytes.Buffer{}, Verbose: vb})
		if len(logged) == 0 {
			t.Fatalf("Verbose not called for a PRINT channel")
		}
		if !strings.Contains(logged[0], "printed") {
			t.Errorf("Verbose[0] = %q, want a 'printed' message", logged[0])
		}
	})
	t.Run("direct-no-verbose", func(t *testing.T) {
		var logged []string
		vb := func(msg string) { logged = append(logged, msg) }
		_, _ = Delegate(context.Background(), ChannelDirect, DelegateOptions{Out: &bytes.Buffer{}, Verbose: vb})
		if len(logged) != 0 {
			t.Errorf("Verbose called for the direct/ErrDirectSwap case; want no calls: %v", logged)
		}
	})
}

// TestDelegate_NilOutDoesNotPanic asserts a library caller that forgets Out gets io.Discard rather
// than a nil-pointer write panic (the run still executes via the fake runner).
func TestDelegate_NilOutDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Delegate panicked on nil Out: %v", r)
		}
	}()
	f := okRunner()
	_, err := Delegate(context.Background(), ChannelBrew, DelegateOptions{Exec: f}) // Out nil.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("expected the run to still execute (1 call); got %d", len(f.calls))
	}
}
