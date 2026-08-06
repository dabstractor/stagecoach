// detect.go implements the FR-U2 install-method detection cascade — the resolver the
// `stagecoach upgrade` dispatcher (P1.M2.T2.S1 delegate()) and the upgrade command (P1.M4) use to
// decide how the running binary was installed so upgrade never overwrites a package-manager-owned
// file (FR-U1) and routes to the native updater instead (FR-U3). Detect runs a 4-tier cascade:
// (a) explicit override (--install-method flag > STAGECOACH_INSTALL_METHOD env, validated against
// the known channels — unknown = error, not silent direct); (b) GOOS-gated package-manager DB
// queries (brew/scoop/winget/pacman/npm/mise/asdf — first confirming query wins; nix + go-install
// have no ownership query and are path-only); (c) path heuristics on realpath(ExePath) (Homebrew
// Cellar, Scoop shims, Nix store, $GOPATH/bin, npm node_modules); (d) default direct (the only
// self-swap-eligible channel). Detection is best-effort, read-only (queries never mutate), and
// --verbose-logged; ambiguous → direct. Every environment-touching seam (subprocess exec,
// os.Executable path, GOOS, env getter, verbose logger) is an injectable Detector field so CI —
// which has none of these package managers installed — exercises every branch against canned outputs.
// Walled off (FR-U12: stdlib-only, no internal/* imports); the verbose logger is injected as a
// func(string) field, not imported from internal/ui. File comment only — releases.go owns the
// package doc.
package upgrade

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Channel is the detected install method (FR-U2). The string values are STABLE contract IDs that
// the future delegation dispatcher (P1.M2.T2.S1 delegate()) switches on to route to each channel's
// native updater (FR-U3) — they must never be renamed. Note the AUR channel uses the identifier
// "aur" (NOT "pacman"): FR-U3's delegation row is "AUR" and the pacman COMMAND only appears in the
// tier-(b) probe args. ChannelDirect is the ONLY self-swap-eligible channel (FR-U1/U5).
type Channel string

const (
	ChannelBrew      Channel = "brew"
	ChannelScoop     Channel = "scoop"
	ChannelWinget    Channel = "winget"
	ChannelAUR       Channel = "aur" // Arch/AUR (pacman-owned); FR-U3 "AUR".
	ChannelNpm       Channel = "npm"
	ChannelMise      Channel = "mise"
	ChannelAsdf      Channel = "asdf"
	ChannelNix       Channel = "nix"
	ChannelGoInstall Channel = "go-install"
	ChannelDirect    Channel = "direct" // the ONLY self-swap-eligible channel (FR-U1/U5).
)

// ErrUnknownChannel is returned when an explicit --install-method / STAGECOACH_INSTALL_METHOD
// override names a channel that is not one of the 10 known Channel values. An invalid override is a
// hard error (the user explicitly asked for something wrong), NOT a silent fallback to direct — a
// typo could otherwise trigger a wrong-channel delegation or an unwanted self-swap. Only tiers (b)
// and (c) AMBIGUITY falls through to direct.
var ErrUnknownChannel = errors.New("upgrade: unknown --install-method")

// validChannel reports whether s names one of the 10 known Channel values. It validates the explicit
// override (tier a) so a typo surfaces immediately instead of silently defaulting to direct.
func validChannel(s string) bool {
	switch Channel(s) {
	case ChannelBrew, ChannelScoop, ChannelWinget, ChannelAUR, ChannelNpm,
		ChannelMise, ChannelAsdf, ChannelNix, ChannelGoInstall, ChannelDirect:
		return true
	}
	return false
}

// Runner is the injectable subprocess seam for the tier-(b) package-manager DB queries. Production
// code uses an *osRunner (exec.CommandContext + per-query timeout); tests inject a canned fakeRunner.
//
// The (stdout, exitCode, err) contract is load-bearing for the cascade and mirrors the house git
// subprocess pattern (internal/git/git.go run): a NON-ZERO process exit — e.g. `brew list
// stagecoach` exits 1 when the package is not installed — is returned as (stdout, code, nil) so the
// probe treats "not installed" as a SKIP, not an error. Only infrastructural failures (the PM binary
// absent on PATH / a start failure / a context deadline) return err != nil, which the probe likewise
// treats as a skip (the PM is unavailable or hung). Conflating a non-zero exit with an error would
// abort the whole cascade on the first "not installed" PM.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, exitCode int, err error)
}

// osRunner is the production Runner. Each Run wraps the command in a per-query context timeout
// (default 3s when timeout is zero) per external_deps §7: "these queries must not hang — apply a
// short timeout." The timeout is per-QUERY, not per-cascade, so a single hung brew/scoop/winget
// cannot stall `stagecoach upgrade`.
type osRunner struct {
	timeout time.Duration // 0 ⇒ 3s default (defaultQueryTimeout).
}

// defaultQueryTimeout caps each package-manager DB query so a hung PM cannot stall the cascade
// (external_deps §7). 3s is generous for a local `brew list` / `scoop prefix` / `pacman -Q` while
// still bounding the worst case.
const defaultQueryTimeout = 3 * time.Second

// Run executes name args... and maps the outcome to the Runner contract:
//   - exit 0 ⇒ (stdout, 0, nil) — the probe's confirm predicate runs.
//   - non-zero exit (*exec.ExitError) ⇒ (stdout, code, nil) — "not installed", the probe skips.
//   - start/LookPath failure (binary absent) or context deadline ⇒ ("", 0, err) — the probe skips.
//
// stdout is whatever the process wrote before exiting (or failing), even on a non-zero exit, so a
// confirm predicate that greps stdout still works when a PM prints a partial listing then exits 1.
func (r *osRunner) Run(ctx context.Context, name string, args ...string) (string, int, error) {
	timeout := r.timeout
	if timeout == 0 {
		timeout = defaultQueryTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		// A context cancellation/timeout kills the child with a signal, which surfaces as a
		// *exec.ExitError (signal: killed) — NOT a plain context error. Check ctx.Err() FIRST so the
		// per-query timeout is treated as a skip (PM hung) rather than a non-zero "not installed" exit.
		// This mirrors internal/git/git.go run()'s ordering (cerr check before errors.As ExitError).
		if cerr := ctx.Err(); cerr != nil {
			return out.String(), 0, cerr // per-query timeout / cancelled context ⇒ PM hung ⇒ skip.
		}
		// A non-zero process exit is *exec.ExitError — extract the code, return err==nil so the
		// probe treats "not installed" as a skip, not a cascade-aborting error (the load-bearing
		// distinction; mirror internal/git/git.go run()).
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return out.String(), ee.ExitCode(), nil
		}
		// Start failure (binary absent on PATH) or I/O failure — the PM is unavailable right now ⇒ skip.
		return "", 0, err
	}
	return out.String(), 0, nil
}

// Detector resolves how the running binary was installed (FR-U2). Every environment-touching seam is
// an injectable field so the resolver is fully unit-testable against canned outputs without any real
// package manager on PATH: Exec is the tier-(b) subprocess seam (nil ⇒ skip PM probes), ExePath is
// os.Executable() ("" ⇒ skip path heuristics), GOOS gates which PM probes run (winget⇒windows,
// brew⇒darwin/linux, …; "" ⇒ runtime.GOOS), Override is the --install-method flag value ("" ⇒ unset),
// Env reads the environment (nil ⇒ skip the env override), Log is the --verbose logger (nil ⇒ no-op).
// A zero-value Detector{} with no override returns ChannelDirect (the safe default).
type Detector struct {
	Exec     Runner              // tier (b); nil ⇒ skip PM probes. Production: &osRunner{}.
	ExePath  string              // os.Executable() result; "" ⇒ skip tier (c). Tests inject a path.
	GOOS     string              // runtime.GOOS; gates PM probes. "" ⇒ treated as runtime.GOOS.
	Override string              // --install-method flag value ("" ⇒ unset); cmd layer (P1.M4) sets it.
	Env      func(string) string // os.Getenv; reads STAGECOACH_INSTALL_METHOD. nil ⇒ skip env override.
	Log      func(string)        // --verbose logger (cmd layer wires ui.Verbose); nil ⇒ no-op.
}

// log emits msg to the verbose logger when one is configured; it is a no-op when Log is nil so a
// zero-value Detector never calls a nil func.
func (d *Detector) log(msg string) {
	if d.Log != nil {
		d.Log(msg)
	}
}

// goos returns the effective GOOS for gating PM probes: the injected d.GOOS, or runtime.GOOS when
// the caller left it empty (production wiring passes runtime.GOOS explicitly; this fallback keeps a
// bare Detector{} sensible). Probes are gated at RUNTIME via this field — NOT via //go:build tags —
// so one binary runs on every platform and tests inject GOOS freely.
func (d *Detector) goos() string {
	if d.GOOS != "" {
		return d.GOOS
	}
	return runtime.GOOS
}

// Detect resolves the install channel via the FR-U2 cascade and returns it with a short,
// human-readable evidence string naming the confirming tier/signal (e.g. "--install-method
// override", "brew list stagecoach (exit 0)", "path: /opt/homebrew/Cellar/"). The cascade is
// best-effort and read-only (queries never mutate); ambiguous input falls through to ChannelDirect
// with evidence "default (ambiguous)". An INVALID explicit override is a hard error (ErrUnknownChannel),
// never a silent fallback to direct — only tier (b)/(c) ambiguity defaults to direct.
func (d *Detector) Detect(ctx context.Context) (Channel, string, error) {
	// (a) explicit override — flag beats env; validate it is a known channel (else error).
	if ch, ev, ok, err := d.detectOverride(); ok || err != nil {
		return ch, ev, err // known override ⇒ return; invalid ⇒ err; absent ⇒ fall through (ok=false, err=nil).
	}

	// (b) package-manager DB queries — GOOS-gated table; first confirming probe wins.
	if d.Exec != nil {
		if ch, ev, ok := d.detectPackageManager(ctx); ok {
			return ch, ev, nil
		}
	}

	// (c) path heuristics on realpath(ExePath).
	if ch, ev, ok := d.detectPath(); ok {
		return ch, ev, nil
	}

	// (d) default direct + ambiguous hint (FR-U2: ambiguous → direct; suggest pinning if wrong).
	d.log("install method ambiguous; defaulting to direct (pin --install-method if wrong)")
	return ChannelDirect, "default (ambiguous)", nil
}

// detectOverride resolves the tier-(a) explicit override: the --install-method flag (d.Override)
// takes precedence over the STAGECOACH_INSTALL_METHOD env var (the npm wrapper sets it; see
// external_deps §3). Returns (channel, evidence, true, nil) for a known override, (zero, "", false,
// nil) when no override is set, and (zero, "", false, err) for an unknown override (a hard error —
// a typo should surface, not silently default to direct).
func (d *Detector) detectOverride() (Channel, string, bool, error) {
	// Flag first.
	if d.Override != "" {
		if !validChannel(d.Override) {
			return "", "", false, fmt.Errorf("%s %q (want one of %s): %w",
				"unknown --install-method", d.Override, knownChannelList(), ErrUnknownChannel)
		}
		return Channel(d.Override), "--install-method override", true, nil
	}
	// Then env.
	if d.Env != nil {
		if v := d.Env("STAGECOACH_INSTALL_METHOD"); v != "" {
			if !validChannel(v) {
				return "", "", false, fmt.Errorf("unknown STAGECOACH_INSTALL_METHOD=%q (want one of %s): %w",
					v, knownChannelList(), ErrUnknownChannel)
			}
			return Channel(v), "STAGECOACH_INSTALL_METHOD=" + v, true, nil
		}
	}
	return "", "", false, nil
}

// knownChannelList returns the 10 channel identifiers as a comma-separated hint for the invalid-
// override error message, in the canonical const-declaration order.
func knownChannelList() string {
	return strings.Join([]string{
		string(ChannelBrew), string(ChannelScoop), string(ChannelWinget), string(ChannelAUR),
		string(ChannelNpm), string(ChannelMise), string(ChannelAsdf), string(ChannelNix),
		string(ChannelGoInstall), string(ChannelDirect),
	}, ", ")
}

// pmProbe is one row of the tier-(b) package-manager DB query table. goos is the GOOS gate (empty ⇒
// all platforms); name/args is the read-only ownership query; confirm decides whether the query's
// (stdout, exitCode) proves stagecoach is installed via that PM. Every probe is strictly read-only —
// no install/upgrade/remove verbs (queries only) — so Detect never mutates the user's system.
type pmProbe struct {
	channel Channel
	goos    []string // empty ⇒ admit on every GOOS.
	name    string
	args    []string
	confirm func(stdout string, exitCode int) bool
}

// pmProbes is the tier-(b) query table (external_deps §7), ordered so the most specific / reliable
// probes run first. nix and go-install are deliberately ABSENT: neither has an ownership query (a
// nix profile / a `go install`ed binary cannot be distinguished from a manual copy via a DB lookup),
// so they are detected by the tier-(c) path heuristics instead. Each confirm predicate is tuned per
// PM exit/listing semantics: brew/scoop/pacman exit 0 iff the package is installed (exit0Confirm);
// winget/npm/mise/asdf list everything and we grep the listing for "stagecoach" (grepConfirm).
var pmProbes = []pmProbe{
	{channel: ChannelBrew, goos: []string{"darwin", "linux"}, name: "brew", args: []string{"list", "stagecoach"}, confirm: exit0Confirm},
	{channel: ChannelAUR, goos: []string{"linux"}, name: "pacman", args: []string{"-Q", "stagecoach-bin"}, confirm: exit0Confirm},
	{channel: ChannelScoop, goos: []string{"windows"}, name: "scoop", args: []string{"prefix", "stagecoach"}, confirm: exit0Confirm},
	{channel: ChannelWinget, goos: []string{"windows"}, name: "winget", args: []string{"list"}, confirm: grepConfirm("stagecoach")},
	{channel: ChannelNpm, goos: nil, name: "npm", args: []string{"ls", "-g", "--depth=0"}, confirm: grepConfirm("stagecoach")},
	{channel: ChannelMise, goos: nil, name: "mise", args: []string{"ls"}, confirm: grepConfirm("stagecoach")},
	{channel: ChannelAsdf, goos: nil, name: "asdf", args: []string{"list"}, confirm: grepConfirm("stagecoach")},
}

// exit0Confirm reports a PM-owned install when the query exited 0 (brew list / scoop prefix /
// pacman -Q all exit 0 iff the named package is installed).
func exit0Confirm(_ string, exitCode int) bool { return exitCode == 0 }

// grepConfirm returns a confirm predicate that reports a PM-owned install when the query's stdout
// contains needle. winget/npm/mise/asdf list every package and cannot exit-nonzero on a single
// missing one, so ownership is proven by finding the stagecoach entry in the listing.
func grepConfirm(needle string) func(string, int) bool {
	return func(stdout string, _ int) bool { return strings.Contains(stdout, needle) }
}

// goosAdmits reports whether probe should run on the given GOOS: an empty goos slice admits every
// platform; otherwise the GOOS must be listed.
func goosAdmits(goos []string, target string) bool {
	if len(goos) == 0 {
		return true
	}
	for _, g := range goos {
		if g == target {
			return true
		}
	}
	return false
}

// detectPackageManager runs the tier-(b) PM DB queries in table order, gated by the effective GOOS.
// For each admitting probe it runs the query via d.Exec: a start/LookPath failure (err != nil — the
// PM binary is absent) or a non-zero exit (the package is not installed) ⇒ log + skip; a confirming
// query ⇒ return the channel + evidence. First confirming probe wins.
func (d *Detector) detectPackageManager(ctx context.Context) (Channel, string, bool) {
	goos := d.goos()
	for _, p := range pmProbes {
		if !goosAdmits(p.goos, goos) {
			continue // GOOS-gated: winget/scoop are Windows-only, brew is darwin/linux, pacman is linux.
		}
		stdout, code, err := d.Exec.Run(ctx, p.name, p.args...)
		switch {
		case err != nil:
			// PM binary absent / start failure / per-query timeout fired — skip this probe (not a
			// Detect error; the PM is simply unavailable right now).
			d.log(fmt.Sprintf("install-method: %s probe skipped (unavailable: %v)", p.name, err))
			continue
		case code != 0:
			// Non-zero exit = the package is not installed via this PM — skip.
			d.log(fmt.Sprintf("install-method: %s probe not-installed (exit %d)", p.name, code))
			continue
		case p.confirm(stdout, code):
			ev := fmt.Sprintf("%s %s (exit 0)", p.name, strings.Join(p.args, " "))
			return p.channel, ev, true
		default:
			// Exit 0 but the confirm predicate (grep) did not match — skip.
			d.log(fmt.Sprintf("install-method: %s probe exit 0 but not confirmed", p.name))
		}
	}
	return "", "", false
}

// pathHeuristic is one row of the tier-(c) path-prefix table: if realpath(ExePath) begins with
// prefix, the binary is installed via channel.
type pathHeuristic struct {
	prefix  string
	channel Channel
}

// pathHeuristics maps well-known install roots to channels (external_deps §7). It is GOOS-aware via
// the prefix itself — Unix roots use forward slashes (/opt/homebrew/Cellar/), the Scoop root uses
// Windows backslashes (\scoop\shims\). /usr/bin is DELIBERATELY ABSENT: it is ambiguous (a manual
// `cp stagecoach /usr/bin/` would false-positive as AUR and the dispatcher would print `sudo
// pacman…` for a non-pacman install). AUR detection is owned by the tier-(b) pacman QUERY instead,
// which only confirms when pacman actually manages the package. FR-U2: ambiguous → direct.
var pathHeuristics = []pathHeuristic{
	{prefix: "/opt/homebrew/Cellar/", channel: ChannelBrew},
	{prefix: "/usr/local/Cellar/", channel: ChannelBrew},
	{prefix: `\scoop\shims\`, channel: ChannelScoop},
	{prefix: "/nix/store/", channel: ChannelNix},
}

// detectPath resolves the tier-(c) path heuristics on realpath(ExePath). It first EvalSymlinks the
// path (external_deps §7: the Homebrew install is a symlink into the Cellar; Scoop shims are
// symlinks/junctions) and tolerates an error by falling back to the raw ExePath. It then matches the
// cleaned path against pathHeuristics (case-insensitively, so a Windows Scoop path matches on a
// case-insensitive FS and in a cross-GOOS test), and finally checks the env-derived $GOPATH/bin
// (go-install) and a conventional npm node_modules path (best-effort). Returns false when nothing
// matches so the cascade falls through to default direct.
func (d *Detector) detectPath() (Channel, string, bool) {
	if d.ExePath == "" {
		return "", "", false
	}
	real, err := filepath.EvalSymlinks(d.ExePath)
	if err != nil {
		real = d.ExePath // tolerate — fall back to the raw path (e.g. a path that does not exist in a test).
	}
	real = filepath.Clean(real)
	lower := strings.ToLower(real)

	// Static prefix/segment table (Homebrew Cellar, Scoop shims, Nix store). Matched via Contains on
	// the lowercased path so (a) a true prefix like /opt/homebrew/Cellar/ matches whether or not the
	// ExePath is the realpath into the Cellar, and (b) a mid-path Windows marker like \scoop\shims\
	// matches inside a full C:\Users\me\scoop\shims\... path on a case-insensitive FS and in a
	// cross-GOOS test (filepath.Clean keeps backslashes on Linux, so HasPrefix would miss the drive).
	for _, h := range pathHeuristics {
		if strings.Contains(lower, strings.ToLower(h.prefix)) {
			return h.channel, "path: " + h.prefix, true
		}
	}

	// go-install via $GOPATH/bin. When GOPATH is explicitly set, use it; when GOPATH is UNSET (the
	// overwhelmingly common modern-Go case — `go env GOPATH` defaults to ~/go), fall back to $HOME/go
	// so a binary installed via `go install ...@latest` under the default ~/go/bin is detected as
	// go-install (FR-U2/FR-U3) instead of misrouted to direct (BUG-002). Resolved from the injected
	// env getter so a test can set GOPATH/HOME without touching the real environment. An explicit
	// GOPATH always wins; the HOME-based default applies only when GOPATH is empty.
	gopath := d.envOr("GOPATH", "")
	evidence := "path: $GOPATH/bin"
	if gopath == "" {
		if home := d.envOr("HOME", ""); home != "" {
			gopath = filepath.Join(home, "go")
			evidence = "path: ~/go/bin (default GOPATH)"
		}
	}
	if gopath != "" {
		goBin := filepath.Clean(filepath.Join(gopath, "bin")) + string(filepath.Separator)
		if strings.HasPrefix(real, goBin) || strings.HasPrefix(lower, strings.ToLower(goBin)) {
			return ChannelGoInstall, evidence, true
		}
	}

	// npm global: best-effort conventional node_modules path. npm installs global bin shims that
	// point into a node_modules/...stagecoach... tree; we look for that marker in the realpath. This
	// is intentionally weak (node_modules can appear anywhere) — the canonical npm signal is the
	// tier-(b) npm probe / the STAGECOACH_INSTALL_METHOD env the npm wrapper sets (external_deps §3).
	if idx := strings.Index(lower, "node_modules"); idx >= 0 {
		if strings.Contains(lower[idx:], "stagecoach") {
			return ChannelNpm, "path: node_modules/.../stagecoach", true
		}
	}

	return "", "", false
}

// envOr returns d.Env(key) when an env getter is configured, else fallback. Centralizes the nil-env
// guard so detectPath stays readable.
func (d *Detector) envOr(key, fallback string) string {
	if d.Env == nil {
		return fallback
	}
	if v := d.Env(key); v != "" {
		return v
	}
	return fallback
}
