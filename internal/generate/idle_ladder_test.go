// idle_ladder_test.go is the FR-R7 (§9.15) config-observability proof, closing P1: every one of the
// five precedence layers (flag > env > git > file > default — §9.8 FR34, plan/022_856008a8cbb5/
// architecture/config_ladder.md) observably binds the message role's TWO deadlines (idle + hard cap)
// END-TO-END through the REAL config.Load plumbing → config.ResolveRoleDeadlines("message", cfg)
// (the exact seam generate.go reads) → provider.Execute kill-or-survive behavior with stub agents.
//
// The S2/S4 distinction (keep it crisp): idle_parity_test.go (S2/S3) SET cfg fields directly and
// proved kill parity + false-kill discrimination; THIS file drives config.Load(ctx, LoadOpts{...})
// with REAL layer sources — a pflag set, t.Setenv env vars, `git -C repo config` keys, a
// .stagecoach.toml at the repo root, and nothing at all — and observes which deadline actually wins.
// A regression in any layer's plumbing (a renamed env var, a snake_case git key, an inverted
// per-role overlay guard) turns a kill row or a survive row red.
//
// This is an EXTERNAL test (package generate_test) — REQUIRED to import internal/exitcode (exitcode
// imports generate ⇒ an in-package test would be an import cycle) — and it REUSES (never redefines)
// the shared external-package helpers initTempRepo (hooks_freeze_test.go), stageChange and headSHA
// (idle_parity_test.go). Its own symbols are ladder*-prefixed + newLadderFlagSet.
//
// Platform note (executor D4): the idle watcher arms only when Idle>0 AND (GOOS==linux ||
// spec.ProgressiveStdout); print-mode stubs are disarmed on darwin/windows until P2. Every idle row
// therefore carries a `--timeout 4s` flag as HARNESS SCAFFOLDING (the flag layer's HARD field is
// orthogonal to the idle-field precedence under test) so non-Linux platforms terminate bounded via
// the cap; the cause-line assertion is GOOS-split exactly as landed in idle_parity_test.go. Rows
// whose expected winner IS the hard cap assert "hard cap" unconditionally. Rows are never skipped
// on any platform, and never run in parallel (t.Setenv + process-global chdir forbid it).
package generate_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/exitcode"
	"github.com/dabstractor/stagecoach/internal/generate"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/stubtest"
	"github.com/dabstractor/stagecoach/internal/ui"
)

// cfgV3 prefixes every written test TOML: config_version must equal CurrentConfigVersion (3) or the
// FR-B4/B9 version advisory prints (stderr noise on every row — never an error, but avoidable).
const cfgV3 = "config_version = 3\n"

// ladderRoleNames mirrors config's unexported roleNames (package-config test helpers are
// unimportable from generate_test): the four canonical roles drive newLadderFlagSet's per-role
// flag registration.
var ladderRoleNames = []string{"planner", "stager", "message", "arbiter"}

// newLadderFlagSet returns a fresh *pflag.FlagSet with the FULL Config-backed flag registration,
// copied verbatim from internal/config/load_test.go's newFlagSet (zero defaults — fs.Set marks
// Changed exactly as production cobra does). loadFlags probes fs.Changed for provider/model/
// timeout/idle-timeout/verbose/no-color/<role>-{provider,model,timeout,idle-timeout}/commits/
// max-commits/single/no-decompose/exclude/format/locale/template; a partial registration would
// silently change layer behavior, so this must stay COMPLETE.
func newLadderFlagSet(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("provider", "", "")
	fs.String("model", "", "")
	fs.String("timeout", "", "")
	fs.String("idle-timeout", "", "")
	fs.Bool("verbose", false, "")
	fs.Bool("no-color", false, "")
	// V2 per-role flags (PRD §9.15 FR-R3, §15.2)
	for _, role := range ladderRoleNames {
		fs.String(role+"-provider", "", "")
		fs.String(role+"-model", "", "")
		fs.String(role+"-timeout", "", "")
		fs.String(role+"-idle-timeout", "", "") // §9.30 FR-R7 per-role idle deadline
	}
	// V2 decompose flags (PRD §9.14 FR-M2, §15.2)
	fs.Int("commits", 0, "")
	fs.Bool("single", false, "")
	fs.Bool("no-decompose", false, "")
	fs.Int("max-commits", 0, "")
	// §9.18 FR-X1 — repeatable exclude flag (StringArray; each Set call appends one literal value).
	fs.StringArray("exclude", nil, "")
	// §9.19 FR-F1/FR-F6/FR-F8 — format/locale/template flags.
	fs.String("format", "", "")
	fs.String("locale", "", "")
	fs.String("template", "", "")
	return fs
}

// ladderScrubEnv neutralizes ambient STAGECOACH_* environment (the config package's loadEnvSetup
// scrubs nothing): every var with the STAGECOACH_ prefix is t.Setenv'd to "" — EMPTY is the
// documented no-op (present+non-empty overrides; unset/empty ignored). Must run BEFORE a row sets
// its own vars, or a developer-shell export would silently mutate every row.
func ladderScrubEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if name, _, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(name, "STAGECOACH_") {
			t.Setenv(name, "")
		}
	}
}

// ladderIsolateHome points HOME and XDG_CONFIG_HOME at a fresh temp dir so the GLOBAL config
// discovery layer finds nothing (XDG wins) — and DisableBootstrap keeps Load from writing one.
func ladderIsolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
}

// ladderChdir changes CWD to dir with a cleanup to restore (the os.Chdir+t.Cleanup idiom from
// internal/config/load_test.go — go 1.22 has NO t.Chdir). REQUIRED before Load: the repo-local
// .stagecoach.toml resolves from the process CWD (file.go returns the bare relative path), NOT
// from LoadOpts.RepoDir (which feeds only the git layer).
func ladderChdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("chdir restore failed: %v", err)
		}
	})
}

// ladderGitConfig writes a repo-local git config key=value (the setGitConfig pattern from
// internal/config/git_test.go). Keys are CAMELCASE in the final segment (stagecoach.idleTimeout) —
// git REJECTS underscores there ("invalid key"), the asymmetry the git rows exist to pin.
func ladderGitConfig(t *testing.T, repo, key, value string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", repo, "config", key, value).CombinedOutput(); err != nil {
		t.Fatalf("git config %s=%s failed: %v\n%s", key, value, err, out)
	}
}

// ladderWriteRepoFile writes .stagecoach.toml at the repo root. Callers must ladderChdir first
// (CWD-relative discovery). Duration values are STRINGS everywhere ("1s", "30s", "1500ms") —
// [defaults].idle_timeout parses with BARE time.ParseDuration and rejects bare ints.
func ladderWriteRepoFile(t *testing.T, repo, toml string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, ".stagecoach.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write .stagecoach.toml: %v", err)
	}
}

// ladderSetFlag sets one flag value on the row's pflag set (marks Changed as production cobra does).
func ladderSetFlag(t *testing.T, fs *pflag.FlagSet, name, value string) {
	t.Helper()
	if err := fs.Set(name, value); err != nil {
		t.Fatalf("flag --%s=%s: %v", name, value, err)
	}
}

// ladderCase is one precedence-layer→behavior row. setup stages the REAL layer sources (t.Setenv
// env vars, ladderGitConfig git keys, ladderWriteRepoFile TOML, ladderSetFlag flags) AFTER the
// runner's scrub+isolate+chdir. wantIdle/wantHard are the expected config.ResolveRoleDeadlines
// ("message", *cfg) pair — the seam the pipeline actually reads (generate.go) — asserted
// platform-uniform BEFORE the behavioral witness. want is "idle" (Linux idle-kill / D4-disarmed
// cap-kill elsewhere), "hard" (cap-kill everywhere), or "success" (commit lands).
type ladderCase struct {
	name       string
	setup      func(t *testing.T, repo string, fs *pflag.FlagSet)
	stub       stubtest.Options            // hung: {Out: "feat: never", SleepMS: 5000}; healthy: {Out: "feat: quick"}
	wantIdle   time.Duration               // expected message-role idle deadline
	wantHard   time.Duration               // expected message-role hard deadline
	wantRoles  map[string][2]time.Duration // extra roles to assert at the seam (planner isolation)
	want       string                      // "idle" | "hard" | "success"
	maxElapsed time.Duration               // hang-guard
}

// ladderHungStub is persona (a): sleeps SILENTLY past every deadline under test (5000ms > 4s bound
// > 1s idle > 1.5s cap), so every kill row self-bounds on every platform.
var ladderHungStub = stubtest.Options{Out: "feat: never", SleepMS: 5000}

// TestIdleLadder_EveryLayerBinds: 16 rows proving FR-R7's per-role two-deadline ladder binds
// observably at every layer. Each row: scrub+isolate+chdir → stage the layer source(s) →
// config.Load(DisableBootstrap: true) → seam assert (ResolveRoleDeadlines) → behavior assert
// (CommitStaged with a stub agent). If ANY row fails, the defect is in the LANDED config surface
// (P1.M1), engine (P1.M2), or harness (S1/S2/S3) — NOT here; this is a tests-only task.
func TestIdleLadder_EveryLayerBinds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow config-ladder behavioral tests in -short mode")
	}
	bin := stubtest.Build(t)
	for _, c := range []ladderCase{
		{
			// Row 1 — DEFAULT layer: nothing set ⇒ the LOADED cfg carries 60s idle / 600s hard
			// (Layer-1 defaults through Load — the claim), and a short healthy run is NOT killed.
			name:       "Defaults_BindAndSpare",
			stub:       stubtest.Options{Out: "feat: quick"},
			wantIdle:   60 * time.Second,
			wantHard:   600 * time.Second,
			want:       "success",
			maxElapsed: 10 * time.Second,
		},
		{
			// Row 2 — FLAG layer, global idle: `--idle-timeout 1s` kills a hung stub (Linux).
			name: "FlagGlobal_IdleKills",
			setup: func(t *testing.T, _ string, fs *pflag.FlagSet) {
				ladderSetFlag(t, fs, "idle-timeout", "1s")
				ladderSetFlag(t, fs, "timeout", "4s") // the bound: non-Linux D4 kill path
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 3 — FLAG layer, per-role: `--message-idle-timeout 1s` beats env 10s + git 20s +
			// file 30s — the full per-role ladder against the flag in one row.
			name: "FlagPerRoleIdle_BeatsEnvGitFile",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderSetFlag(t, fs, "message-idle-timeout", "1s")
				ladderSetFlag(t, fs, "timeout", "4s")
				t.Setenv("STAGECOACH_MESSAGE_IDLE_TIMEOUT", "10s")
				ladderGitConfig(t, repo, "stagecoach.role.message.idleTimeout", "20s")
				ladderWriteRepoFile(t, repo, cfgV3+"\n[role.message]\nidle_timeout = \"30s\"\n")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 4 — ENV layer, per-role: STAGECOACH_MESSAGE_IDLE_TIMEOUT=1s overrides the file's
			// [role.message].idle_timeout="30s".
			name: "EnvPerRoleIdle_BeatsFile",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				t.Setenv("STAGECOACH_MESSAGE_IDLE_TIMEOUT", "1s")
				ladderSetFlag(t, fs, "timeout", "4s")
				ladderWriteRepoFile(t, repo, cfgV3+"\n[role.message]\nidle_timeout = \"30s\"\n")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 5 — ENV layer, global idle: STAGECOACH_IDLE_TIMEOUT=1s binds.
			name: "EnvGlobal_IdleKills",
			setup: func(t *testing.T, _ string, fs *pflag.FlagSet) {
				t.Setenv("STAGECOACH_IDLE_TIMEOUT", "1s")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 6 — GIT layer, global idle: camelCase stagecoach.idleTimeout 1s binds (FINDING A:
			// git rejects the snake_case spelling, so camelCase IS the contract).
			name: "GitGlobal_IdleKills",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderGitConfig(t, repo, "stagecoach.idleTimeout", "1s")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 7 — GIT layer, per-role: stagecoach.role.message.idleTimeout 1s binds.
			name: "GitPerRole_IdleKills",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderGitConfig(t, repo, "stagecoach.role.message.idleTimeout", "1s")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 8 — GIT > FILE (FR34's unusual ordering): git 1s beats [defaults].idle_timeout="30s".
			name: "GitIdle_BeatsFile",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderGitConfig(t, repo, "stagecoach.idleTimeout", "1s")
				ladderWriteRepoFile(t, repo, cfgV3+"\n[defaults]\nidle_timeout = \"30s\"\n")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 9 — FILE layer, global idle: [defaults].idle_timeout="1s" (duration STRING — the
			// global file key rejects bare ints).
			name: "FileGlobal_IdleKills",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderWriteRepoFile(t, repo, cfgV3+"\n[defaults]\nidle_timeout = \"1s\"\n")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 10 — FILE layer, per-role: [role.message].idle_timeout="1s" binds.
			name: "FilePerRole_IdleKills",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderWriteRepoFile(t, repo, cfgV3+"\n[role.message]\nidle_timeout = \"1s\"\n")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 1 * time.Second, wantHard: 4 * time.Second,
			want: "idle", maxElapsed: 6 * time.Second,
		},
		{
			// Row 11 — per-role > global (INVERTED discriminator): [role.message] 30s beats
			// [defaults] 1s ⇒ the 4s bound is what kills (cause "hard cap" everywhere — if global
			// wrongly won, Linux would idle-kill at ~1s and the hard-cap assertion fails).
			name: "FilePerRoleIdle_BeatsGlobal",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderWriteRepoFile(t, repo, cfgV3+"\n[defaults]\nidle_timeout = \"1s\"\n\n[role.message]\nidle_timeout = \"30s\"\n")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub: ladderHungStub, wantIdle: 30 * time.Second, wantHard: 4 * time.Second,
			want: "hard", maxElapsed: 8 * time.Second,
		},
		{
			// Row 12 — HARD CAP via flag: `--timeout 1500ms` kills everywhere (idle default 60s
			// can never fire first).
			name: "FlagGlobalHardCap_Kills",
			setup: func(t *testing.T, _ string, fs *pflag.FlagSet) {
				ladderSetFlag(t, fs, "timeout", "1500ms")
			},
			stub: ladderHungStub, wantIdle: 60 * time.Second, wantHard: 1500 * time.Millisecond,
			want: "hard", maxElapsed: 6 * time.Second,
		},
		{
			// Row 13 — HARD CAP via file: [role.message].timeout="1500ms" kills everywhere.
			name: "FilePerRoleHardCap_Kills",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderWriteRepoFile(t, repo, cfgV3+"\n[role.message]\ntimeout = \"1500ms\"\n")
			},
			stub: ladderHungStub, wantIdle: 60 * time.Second, wantHard: 1500 * time.Millisecond,
			want: "hard", maxElapsed: 6 * time.Second,
		},
		{
			// Row 14 — HARD CAP via env: STAGECOACH_TIMEOUT=1500ms kills everywhere.
			name: "EnvGlobalHardCap_Kills",
			setup: func(t *testing.T, _ string, fs *pflag.FlagSet) {
				t.Setenv("STAGECOACH_TIMEOUT", "1500ms")
			},
			stub: ladderHungStub, wantIdle: 60 * time.Second, wantHard: 1500 * time.Millisecond,
			want: "hard", maxElapsed: 6 * time.Second,
		},
		{
			// Row 15 — HARD CAP via git: camelCase stagecoach.timeout 1500ms kills everywhere.
			name: "GitGlobalHardCap_Kills",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderGitConfig(t, repo, "stagecoach.timeout", "1500ms")
			},
			stub: ladderHungStub, wantIdle: 60 * time.Second, wantHard: 1500 * time.Millisecond,
			want: "hard", maxElapsed: 6 * time.Second,
		},
		{
			// Row 16 — per-ROLE independence: [role.planner].idle_timeout="1s" must NOT leak into
			// message (message inherits the 60s default ⇒ only the 4s bound kills). The seam assert
			// is the platform-uniform proof; the cap-kill corroborates the behavior on Linux.
			name: "PlannerIsolation_NoLeakToMessage",
			setup: func(t *testing.T, repo string, fs *pflag.FlagSet) {
				ladderWriteRepoFile(t, repo, cfgV3+"\n[role.planner]\nidle_timeout = \"1s\"\n")
				ladderSetFlag(t, fs, "timeout", "4s")
			},
			stub:     ladderHungStub,
			wantIdle: 60 * time.Second, wantHard: 4 * time.Second,
			wantRoles: map[string][2]time.Duration{
				"planner": {1 * time.Second, 4 * time.Second},
			},
			want: "hard", maxElapsed: 8 * time.Second,
		},
	} {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("skipping slow config-ladder row in -short mode")
			}
			repo := initTempRepo(t) // shared external-package helper (hooks_freeze_test.go)
			ladderScrubEnv(t)       // neutralize ambient STAGECOACH_* BEFORE the row's setup
			ladderIsolateHome(t)    // no global config; DisableBootstrap skips the write
			ladderChdir(t, repo)    // repo-local .stagecoach.toml resolves from CWD
			fs := newLadderFlagSet(t)
			if c.setup != nil {
				c.setup(t, repo, fs)
			}
			cfg, err := config.Load(context.Background(), config.LoadOpts{
				RepoDir:          repo, // feeds the git layer
				Flags:            fs,   // the flag layer (fs.Changed probes)
				DisableBootstrap: true, // documented TEST-ONLY seam (FR-B3): no write, no notice
			})
			if err != nil {
				t.Fatalf("config.Load: %v", err)
			}

			// (a) THE SEAM ASSERT — platform-uniform, the strict ladder proof: the exact resolver
			// the pipeline reads (generate.go's ResolveRoleDeadlines call). A wrong layer value
			// fails HERE on every platform.
			idle, hard := config.ResolveRoleDeadlines("message", *cfg)
			if idle != c.wantIdle || hard != c.wantHard {
				t.Errorf("message deadlines = (%v, %v), want (%v, %v)", idle, hard, c.wantIdle, c.wantHard)
			}
			for role, want := range c.wantRoles {
				i2, h2 := config.ResolveRoleDeadlines(role, *cfg)
				if i2 != want[0] || h2 != want[1] {
					t.Errorf("%s deadlines = (%v, %v), want (%v, %v)", role, i2, h2, want[0], want[1])
				}
			}

			// (b) THE BEHAVIOR ASSERT — kill-or-survive through the real pipeline.
			stageChange(t, repo) // shared helper: modify+stage fileA.txt (the untracked .stagecoach.toml never pollutes the staged diff)
			before := headSHA(t, repo)
			var vbuf bytes.Buffer
			start := time.Now()
			res, err := generate.CommitStaged(context.Background(), generate.Deps{
				Git:      git.New(repo),
				Manifest: stubtest.Manifest(bin, c.stub),
				Verbose:  ui.NewVerbose(&vbuf, true),
			}, *cfg)
			elapsed := time.Since(start)
			if elapsed > c.maxElapsed {
				t.Errorf("elapsed %v exceeds hang-guard %v; buf:\n%s", elapsed, c.maxElapsed, vbuf.String())
			}
			switch c.want {
			case "success":
				if err != nil {
					t.Fatalf("healthy run failed: %v (FR-R7 default must not kill); buf:\n%s", err, vbuf.String())
				}
				if res.Subject != c.stub.Out {
					t.Errorf("Subject = %q, want %q", res.Subject, c.stub.Out)
				}
				if got := headSHA(t, repo); got == before || got != res.CommitSHA {
					t.Errorf("HEAD = %s, want advanced to %s (before %s)", got, res.CommitSHA, before)
				}
				if strings.Contains(vbuf.String(), "DEBUG: cause:") {
					t.Errorf("a deadline fired on a healthy default run; buf:\n%s", vbuf.String())
				}
				if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
					t.Errorf("Execute count = %d, want 1", n)
				}
			default: // "idle" | "hard" — a bounded kill
				if err == nil {
					t.Fatalf("expected a deadline rescue, got nil (the layer did not bind); buf:\n%s", vbuf.String())
				}
				var re *generate.RescueError
				if !errors.As(err, &re) {
					t.Fatalf("err = %T %v, want *generate.RescueError", err, err)
				}
				if !errors.Is(err, generate.ErrTimeout) {
					t.Errorf("errors.Is(err, generate.ErrTimeout) = false: %v", err)
				}
				if code := exitcode.For(err); code != exitcode.Timeout {
					t.Errorf("exitcode.For(err) = %d, want %d (124)", code, exitcode.Timeout)
				}
				// NO-retry: exactly ONE Execute invocation — the DeadlineExceeded branch returns
				// from inside the retry loop on the first hit.
				if n := strings.Count(vbuf.String(), "DEBUG: command: "); n != 1 {
					t.Errorf("Execute count = %d, want 1 (immediate rescue, NO retry)", n)
				}
				if got := headSHA(t, repo); got != before {
					t.Errorf("HEAD moved on a killed run: %s → %s", before, got)
				}
				if c.want == "hard" {
					if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
						t.Errorf("missing hard-cap cause line; buf:\n%s", vbuf.String())
					}
				} else if runtime.GOOS == "linux" {
					// The landed S2 GOOS-split idiom (idle_parity_test.go): on Linux the idle
					// watcher is armed (D4) and the winning layer names itself.
					if !strings.Contains(vbuf.String(), "DEBUG: cause: idle deadline") {
						t.Errorf("linux: missing idle cause line (the layer bound the wrong deadline or lost precedence); buf:\n%s", vbuf.String())
					}
				} else if !strings.Contains(vbuf.String(), "DEBUG: cause: hard cap") {
					// D4-disarmed platform (darwin/windows): the 4s bound must terminate the run.
					t.Errorf("non-linux: missing hard-cap cause line (the 4s bound must fire); buf:\n%s", vbuf.String())
				}
			}
		})
	}
}
