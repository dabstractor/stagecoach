package upgrade

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRunner is the canned Runner used to exercise the tier-(b) cascade without any real package
// manager on PATH (CI has none of brew/scoop/choco/pacman/npm/mise/asdf). It records every
// invocation as "name args…" so the GOOS-gating and never-mutates tests can assert against what was
// actually run, and delegates the canned (stdout, exitCode, err) outcome to the caller's func.
type fakeRunner struct {
	canned func(name string, args []string) (string, int, error)
	calls  []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, int, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	return f.canned(name, args)
}

// called reports whether a recorded invocation starts with the given command prefix (e.g.
// called("choco") is true iff the choco probe was invoked at all — used by the GOOS-gating tests).
func (f *fakeRunner) called(prefix string) bool {
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix+" ") || c == prefix {
			return true
		}
	}
	return false
}

// silentRunner is a fakeRunner whose canned func never confirms (returns exit 1 for every probe),
// used as the Exec in path/default tests so tier (b) is exercised but never wins.
func silentRunner() *fakeRunner {
	return &fakeRunner{canned: func(string, []string) (string, int, error) {
		return "", 1, nil // exit 1 ⇒ "not installed" ⇒ every probe skips.
	}}
}

func TestValidChannel(t *testing.T) {
	for _, c := range []Channel{
		ChannelBrew, ChannelScoop, ChannelChocolatey, ChannelAUR, ChannelNpm,
		ChannelMise, ChannelAsdf, ChannelNix, ChannelGoInstall, ChannelDirect,
	} {
		if !validChannel(string(c)) {
			t.Errorf("validChannel(%q) = false, want true", c)
		}
	}
	for _, bad := range []string{"", "snap", "pacman", "homebrew", "apt", "GO-INSTALL", "Direct"} {
		if validChannel(bad) {
			t.Errorf("validChannel(%q) = true, want false", bad)
		}
	}
}

func TestDetect_OverrideFlagWins(t *testing.T) {
	// The flag overrides the env AND short-circuits before tier (b): Exec.Run must not be called.
	r := &fakeRunner{canned: func(string, []string) (string, int, error) {
		t.Fatal("Exec.Run must not be called when Override is set")
		return "", 0, nil
	}}
	d := &Detector{
		Override: "npm",
		Env:      func(string) string { return "brew" },
		Exec:     r,
		GOOS:     "linux",
	}
	ch, ev, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelNpm {
		t.Errorf("channel = %q, want npm", ch)
	}
	if !strings.Contains(ev, "override") {
		t.Errorf("evidence %q should mention the override", ev)
	}
	if len(r.calls) != 0 {
		t.Errorf("Exec called %d times, want 0 (override short-circuits): %v", len(r.calls), r.calls)
	}
}

func TestDetect_OverrideEnvWins(t *testing.T) {
	r := &fakeRunner{canned: func(string, []string) (string, int, error) {
		t.Fatal("Exec.Run must not be called when env override is set")
		return "", 0, nil
	}}
	d := &Detector{
		Override: "",
		Env: func(k string) string {
			if k == "STAGECOACH_INSTALL_METHOD" {
				return "mise"
			}
			return ""
		},
		Exec: r,
		GOOS: "linux",
	}
	ch, ev, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelMise {
		t.Errorf("channel = %q, want mise", ch)
	}
	if !strings.Contains(ev, "STAGECOACH_INSTALL_METHOD") {
		t.Errorf("evidence %q should name the env var", ev)
	}
}

func TestDetect_OverrideInvalidFlag_Error(t *testing.T) {
	d := &Detector{Override: "snap", GOOS: "linux"}
	_, _, err := d.Detect(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid override, got nil")
	}
	if !errors.Is(err, ErrUnknownChannel) {
		t.Errorf("err = %v, want errors.Is ErrUnknownChannel", err)
	}
	if !strings.Contains(err.Error(), "unknown --install-method") {
		t.Errorf("err %q should mention --install-method", err.Error())
	}
}

func TestDetect_OverrideInvalidEnv_Error(t *testing.T) {
	d := &Detector{
		Env: func(k string) string {
			if k == "STAGECOACH_INSTALL_METHOD" {
				return "apt"
			}
			return ""
		},
		GOOS: "linux",
	}
	_, _, err := d.Detect(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid env override, got nil")
	}
	if !errors.Is(err, ErrUnknownChannel) {
		t.Errorf("err = %v, want errors.Is ErrUnknownChannel", err)
	}
}

// TestDetect_PMProbesConfirm exercises each tier-(b) probe with a canned "installed" outcome and
// asserts the correct channel resolves. The table covers all 7 PM probes (nix + go-install are
// path-only and covered separately).
func TestDetect_PMProbesConfirm(t *testing.T) {
	cases := []struct {
		name    string
		goos    string
		exepath string
		canned  func(name string, args []string) (string, int, error)
		want    Channel
	}{
		{
			name: "brew installed (darwin)", goos: "darwin",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "brew" {
					return "", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelBrew,
		},
		{
			name: "brew installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "brew" {
					return "", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelBrew,
		},
		{
			name: "pacman/aur installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "pacman" {
					return "", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelAUR,
		},
		{
			name: "dpkg/deb installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "dpkg" {
					return "", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelDeb,
		},
		{
			name: "rpm installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "rpm" {
					return "", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelRpm,
		},
		{
			name: "scoop installed (windows)", goos: "windows",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "scoop" {
					return "", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelScoop,
		},
		{
			name: "chocolatey installed (windows)", goos: "windows",
			canned: func(n string, _ []string) (string, int, error) {
				// choco uses grepConfirm: `choco list` ALWAYS exits 0 (even on a zero-match listing —
				// chocolatey/choco#2118), so ownership is proven by finding "stagecoach" in the listing,
				// not by the exit code. A real `choco list --local-only stagecoach` prints the package line.
				if n == "choco" {
					return "stagecoach 1.0.0", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelChocolatey,
		},
		{
			name: "npm installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "npm" {
					return "/stagecoach@1.0.0", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelNpm,
		},
		{
			name: "mise installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "mise" {
					return "stagecoach", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelMise,
		},
		{
			name: "asdf installed (linux)", goos: "linux",
			canned: func(n string, _ []string) (string, int, error) {
				if n == "asdf" {
					return "stagecoach", 0, nil
				}
				return "", 1, nil
			},
			want: ChannelAsdf,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := &Detector{Exec: &fakeRunner{canned: tc.canned}, GOOS: tc.goos, ExePath: tc.exepath}
			ch, _, err := d.Detect(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ch != tc.want {
				t.Errorf("channel = %q, want %q", ch, tc.want)
			}
		})
	}
}

func TestDetect_PMNotInstalled_SkipsToDirect(t *testing.T) {
	// Every probe returns exit 1 (not installed) / absence ⇒ cascade falls through to direct.
	r := silentRunner()
	d := &Detector{Exec: r, GOOS: "darwin", ExePath: "/home/me/bin/stagecoach"}
	ch, ev, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelDirect {
		t.Errorf("channel = %q, want direct", ch)
	}
	if !strings.Contains(ev, "default") {
		t.Errorf("evidence %q should mention default", ev)
	}
}

func TestDetect_PMAbsent_StartErrorSkips(t *testing.T) {
	// The PM binary is absent (LookPath/start failure) ⇒ err != nil ⇒ every probe skips.
	r := &fakeRunner{canned: func(string, []string) (string, int, error) {
		return "", 0, errors.New("exec: not found")
	}}
	d := &Detector{Exec: r, GOOS: "darwin", ExePath: "/home/me/bin/stagecoach"}
	ch, _, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelDirect {
		t.Errorf("channel = %q, want direct (all PMs absent)", ch)
	}
}

// TestDetect_ChocoListZeroMatch_NoFalsePositive is the FR-U2 BUG guard: `choco list` exits 0 even
// when 0 packages match (chocolatey/choco#2118 — "0 packages found. … Exiting with 0."), so a probe
// that keys on the EXIT CODE (the old exit0Confirm) false-positives on every Windows box that merely
// HAS choco installed, misrouting a non-choco (e.g. PowerShell/direct) install to the chocolatey PRINT
// path. The fix keys on the listing CONTENT (grepConfirm("stagecoach")); this test simulates real
// choco v2 on a box WITHOUT stagecoach and asserts detection falls through to direct.
func TestDetect_ChocoListZeroMatch_NoFalsePositive(t *testing.T) {
	// Real choco v2 on a box without stagecoach: "0 packages found." + EXIT 0. scoop (which probes
	// first on windows) also misses → exit 1.
	canned := func(n string, _ []string) (string, int, error) {
		if n == "choco" {
			return "0 packages found.", 0, nil
		}
		return "", 1, nil
	}
	d := &Detector{
		Exec:    &fakeRunner{canned: canned},
		GOOS:    "windows",
		ExePath: `C:\Users\me\bin\stagecoach.exe`, // a direct/PowerShell install (NOT ProgramData\chocolatey)
	}
	ch, _, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelDirect {
		t.Errorf("zero-match choco list must NOT detect chocolatey (FR-U2 false-positive); got %q, want direct", ch)
	}
}

func TestDetect_GOOSGating(t *testing.T) {
	// On GOOS=linux, the choco + scoop probes must NEVER be invoked (Windows-only).
	r := &fakeRunner{canned: func(string, []string) (string, int, error) { return "", 1, nil }}
	d := &Detector{Exec: r, GOOS: "linux", ExePath: "/home/me/bin/stagecoach"}
	_, _, _ = d.Detect(context.Background())
	if r.called("choco") {
		t.Errorf("choco probe must not run on GOOS=linux; calls=%v", r.calls)
	}
	if r.called("scoop") {
		t.Errorf("scoop probe must not run on GOOS=linux; calls=%v", r.calls)
	}

	// On GOOS=darwin, pacman (AUR) + choco + scoop must never run (brew runs and probes).
	r2 := &fakeRunner{canned: func(string, []string) (string, int, error) { return "", 1, nil }}
	d2 := &Detector{Exec: r2, GOOS: "darwin", ExePath: "/home/me/bin/stagecoach"}
	_, _, _ = d2.Detect(context.Background())
	for _, banned := range []string{"choco", "scoop", "pacman", "dpkg", "rpm"} {
		if r2.called(banned) {
			t.Errorf("%s probe must not run on GOOS=darwin; calls=%v", banned, r2.calls)
		}
	}
	if !r2.called("brew") {
		t.Errorf("brew probe SHOULD run on GOOS=darwin; calls=%v", r2.calls)
	}

	// On GOOS=windows, brew + pacman must never run.
	r3 := &fakeRunner{canned: func(string, []string) (string, int, error) { return "", 1, nil }}
	d3 := &Detector{Exec: r3, GOOS: "windows", ExePath: `C:\Users\me\bin\stagecoach.exe`}
	_, _, _ = d3.Detect(context.Background())
	for _, banned := range []string{"brew", "pacman", "dpkg", "rpm"} {
		if r3.called(banned) {
			t.Errorf("%s probe must not run on GOOS=windows; calls=%v", banned, r3.calls)
		}
	}
}

func TestDetect_Path_BrewCellar(t *testing.T) {
	// Tier (c) with Exec=nil: the Homebrew Cellar prefix ⇒ brew. (Also covers /usr/local/Cellar/.)
	for _, p := range []string{
		"/opt/homebrew/Cellar/stagecoach/1.0/bin/stagecoach",
		"/usr/local/Cellar/stagecoach/1.0/bin/stagecoach",
	} {
		d := &Detector{ExePath: p, GOOS: "darwin"}
		ch, ev, ok := d.detectPath()
		if !ok || ch != ChannelBrew {
			t.Errorf("detectPath(%q) = %q,%q,%v, want brew,true", p, ch, ev, ok)
		}
	}
}

func TestDetect_Path_LinuxbrewCellar(t *testing.T) {
	// BUG-007: the Linuxbrew Cellar root (/home/linuxbrew/.linuxbrew/Cellar/) must detect as brew, not
	// fall through to direct (which would self-swap a brew-managed binary — FR-U1). detectPath is
	// GOOS-agnostic (cross-GOOS deterministic), so this matches under any host GOOS; GOOS="linux" is
	// set for semantic accuracy (Linuxbrew is a Linux install). The ExePath need not exist (detectPath
	// tolerates EvalSymlinks failure and falls back to the raw path).
	d := &Detector{ExePath: "/home/linuxbrew/.linuxbrew/Cellar/stagecoach/1.0/bin/stagecoach", GOOS: "linux"}
	ch, ev, ok := d.detectPath()
	if !ok || ch != ChannelBrew {
		t.Errorf("detectPath linuxbrew = %q,%q,%v, want brew,true", ch, ev, ok)
	}
}

func TestDetect_Path_NixStore(t *testing.T) {
	d := &Detector{ExePath: "/nix/store/abc123-stagecoach-1.0/bin/stagecoach", GOOS: "linux"}
	ch, _, ok := d.detectPath()
	if !ok || ch != ChannelNix {
		t.Errorf("detectPath nix = %q,%v, want nix,true", ch, ok)
	}
}

func TestDetect_Path_ScoopShims(t *testing.T) {
	// Windows-style backslash prefix matches case-insensitively even when the test GOOS is linux
	// (the matching is GOOS-agnostic so cross-GOOS tests are deterministic).
	d := &Detector{ExePath: `C:\Users\me\scoop\shims\stagecoach.exe`, GOOS: "linux"}
	ch, _, ok := d.detectPath()
	if !ok || ch != ChannelScoop {
		t.Errorf("detectPath scoop = %q,%v, want scoop,true", ch, ok)
	}
}

func TestDetect_Path_GoInstallViaGOPATH(t *testing.T) {
	d := &Detector{
		ExePath: "/home/me/go/bin/stagecoach",
		GOOS:    "linux",
		Env: func(k string) string {
			if k == "GOPATH" {
				return "/home/me/go"
			}
			return ""
		},
	}
	ch, _, ok := d.detectPath()
	if !ok || ch != ChannelGoInstall {
		t.Errorf("detectPath go-install = %q,%v, want go-install,true", ch, ok)
	}
}

// TestDetect_Path_GoInstallDefaultGOPATH is the BUG-002 regression: GOPATH unset (the common
// modern-Go case) but HOME set → a binary under $HOME/go/bin must be detected as go-install via
// the ~/go default (matches `go env GOPATH`).
func TestDetect_Path_GoInstallDefaultGOPATH(t *testing.T) {
	home := "/tmp/fakehome"
	d := &Detector{
		ExePath: filepath.Join(home, "go", "bin", "stagecoach"),
		GOOS:    "linux",
		Env: func(k string) string {
			if k == "HOME" {
				return home
			}
			return "" // GOPATH unset
		},
	}
	ch, _, ok := d.detectPath()
	if !ok || ch != ChannelGoInstall {
		t.Errorf("detectPath default-GOPATH = %q,%v, want go-install,true", ch, ok)
	}
}

// TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset is the BUG-002 no-false-positive guard:
// both GOPATH and HOME unset → even a ~/go/bin-style ExePath must NOT be detected as go-install
// (no HOME ⇒ no default GOPATH to resolve ⇒ falls through to direct).
func TestDetect_Path_GoInstallNoFalsePositiveWhenHOMEUnset(t *testing.T) {
	d := &Detector{
		ExePath: "/home/me/go/bin/stagecoach", // looks like a go-install path
		GOOS:    "linux",
		Env:     func(k string) string { return "" }, // GOPATH and HOME both unset
	}
	ch, _, ok := d.detectPath()
	if ok && ch == ChannelGoInstall {
		t.Errorf("detectPath false-positive go-install = %q,%v, want no go-install match (HOME unknown)", ch, ok)
	}
}

func TestDetect_Path_NpmNodeModules(t *testing.T) {
	d := &Detector{
		ExePath: "/usr/local/lib/node_modules/@scope/stagecoach/bin/stagecoach",
		GOOS:    "linux",
	}
	ch, _, ok := d.detectPath()
	if !ok || ch != ChannelNpm {
		t.Errorf("detectPath npm = %q,%v, want npm,true", ch, ok)
	}
}

func TestDetect_DefaultDirect(t *testing.T) {
	// Nothing confirms: no override, PMs absent (Exec=nil), and a path that matches no heuristic.
	d := &Detector{ExePath: "/home/me/bin/stagecoach", GOOS: "linux"}
	ch, ev, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelDirect {
		t.Errorf("channel = %q, want direct", ch)
	}
	if ev != "default (ambiguous)" {
		t.Errorf("evidence = %q, want %q", ev, "default (ambiguous)")
	}
}

func TestDetect_ZeroValueDetector_Direct(t *testing.T) {
	// A bare Detector{} with no override and no seams must return direct without panicking.
	var d Detector
	ch, _, err := d.Detect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch != ChannelDirect {
		t.Errorf("channel = %q, want direct", ch)
	}
}

func TestDetect_NilLogSafe(t *testing.T) {
	// Log is nil; the default-direct path calls d.log and must not panic.
	var d Detector
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil Log panicked: %v", r)
		}
	}()
	_, _, _ = d.Detect(context.Background())
}

func TestDetect_NeverMutates(t *testing.T) {
	// Property: every recorded probe invocation is a read-only query (no install/upgrade/remove/
	// uninstall verbs) regardless of which probes run. Run on every GOOS so all probes are seen.
	mutating := []string{"install", "upgrade", "remove", "uninstall"}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		r := &fakeRunner{canned: func(string, []string) (string, int, error) { return "", 1, nil }}
		d := &Detector{Exec: r, GOOS: goos, ExePath: "/home/me/bin/stagecoach"}
		_, _, _ = d.Detect(context.Background())
		for _, call := range r.calls {
			low := strings.ToLower(call)
			for _, verb := range mutating {
				// Exclude the PM name "stagecoach" only; guard against `upgrade`/`install` as args.
				// Match against the args portion (after the PM binary name) to avoid flagging a
				// hypothetical PM literally named "upgrade".
				parts := strings.SplitN(low, " ", 2)
				args := ""
				if len(parts) == 2 {
					args = parts[1]
				}
				if strings.Contains(args, verb) {
					t.Errorf("GOOS=%s: probe %q contains mutating verb %q", goos, call, verb)
				}
			}
		}
	}
}

func TestOsRunner_NonZeroExitIsNotError(t *testing.T) {
	// The load-bearing contract: a non-zero process exit returns (stdout, code, nil) so the probe
	// treats "not installed" as a skip. We exercise this with /bin/sh -c 'exit 1' (portable).
	r := &osRunner{timeout: 0} // 0 ⇒ 3s default.
	stdout, code, err := r.Run(context.Background(), "sh", "-c", "echo hi; exit 1")
	if err != nil {
		t.Fatalf("non-zero exit must NOT return an error: got %v (stdout=%q, code=%d)", err, stdout, code)
	}
	if code != 1 {
		t.Errorf("exitCode = %d, want 1", code)
	}
	if stdout != "hi\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hi\n")
	}
}

func TestOsRunner_AbsentBinary_StartError(t *testing.T) {
	// A binary that is absent on PATH returns ("", 0, err) so the probe skips (PM unavailable).
	r := &osRunner{timeout: 0}
	_, _, err := r.Run(context.Background(), "this-binary-does-not-exist-xyz", "--version")
	if err == nil {
		t.Fatal("absent binary must return an error so the probe skips")
	}
}

func TestOsRunner_ExitZero(t *testing.T) {
	r := &osRunner{timeout: 0}
	stdout, code, err := r.Run(context.Background(), "sh", "-c", "echo ok")
	if err != nil {
		t.Fatalf("exit 0: unexpected error %v", err)
	}
	if code != 0 {
		t.Errorf("exitCode = %d, want 0", code)
	}
	if stdout != "ok\n" {
		t.Errorf("stdout = %q, want %q", stdout, "ok\n")
	}
}

func TestOsRunner_PerQueryTimeout(t *testing.T) {
	// A hung subprocess is bounded by the per-query timeout (default 3s; here 200ms for speed) and
	// returns a non-nil err (context deadline) so the probe skips.
	r := &osRunner{timeout: 200 * time.Millisecond}
	start := time.Now()
	_, _, err := r.Run(context.Background(), "sh", "-c", "sleep 5")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("hung subprocess must return a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Errorf("per-query timeout not honored: elapsed=%v (want ~200ms)", elapsed)
	}
}
