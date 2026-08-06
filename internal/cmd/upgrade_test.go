package cmd

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/dabstractor/stagecoach/internal/config"
)

// TestUpgradeCommand_Registered verifies upgradeCmd is registered on rootCmd via init()
// (ZERO edits to root.go).
func TestUpgradeCommand_Registered(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"upgrade"})
	if err != nil {
		t.Fatalf("rootCmd.Find([\"upgrade\"]): %v", err)
	}
	if c == nil || c.Name() != "upgrade" {
		t.Errorf("upgrade not registered: got %v", c)
	}
}

// TestUpgradeCommand_Flags verifies all 9 flags + the -c/-y shorthands are registered on
// upgradeCmd.Flags() (LOCAL), and that NONE leaked onto root's persistent flag set (the
// commit-path must not see --check/--rollback/etc.).
func TestUpgradeCommand_Flags(t *testing.T) {
	fs := upgradeCmd.Flags()
	for _, name := range []string{
		"check", "version", "prerelease", "force", "rollback",
		"install-method", "yes", "channel", "source-repo",
	} {
		if fs.Lookup(name) == nil {
			t.Errorf("flag --%s not registered on upgrade", name)
		}
	}
	if fs.ShorthandLookup("c") == nil {
		t.Errorf("-c shorthand missing (check)")
	}
	if fs.ShorthandLookup("y") == nil {
		t.Errorf("-y shorthand missing (yes)")
	}

	// NEGATIVE: the flags are LOCAL — they must NOT appear on root's persistent flag set.
	for _, name := range []string{"check", "rollback", "channel", "source-repo", "install-method"} {
		if rootCmd.PersistentFlags().Lookup(name) != nil {
			t.Errorf("--%s leaked onto root persistent flags (must be LOCAL to upgrade)", name)
		}
	}
}

// TestUpgradeCommand_NoBootstrapOutsideRepo is the FR-B3/FR-U12 proof: running `upgrade`
// outside a git repo with NO global config must NOT bootstrap-write a config file. The
// no-op PersistentPreRunE prevents config.Load from running (the structural guarantee).
func TestUpgradeCommand_NoBootstrapOutsideRepo(t *testing.T) {
	// Isolate HOME + XDG so GlobalConfigPath() lands in a temp dir; assert no bootstrap write.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", "") // force the ~/.config fallback inside tmpHome
	cfgPath := config.GlobalConfigPath()
	if _, err := os.Stat(cfgPath); err == nil {
		t.Fatalf("precondition: global config already exists at %s", cfgPath)
	}

	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)
	defer resetFlags(upgradeCmd.Flags())

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs([]string{"upgrade"}) // runs runUpgrade (placeholder) → exercises the no-op PersistentPreRunE
	_ = Execute(context.Background())    // the placeholder returns nil; tolerate any non-config err

	// THE INVARIANT: config.Load never ran ⇒ no bootstrap file was written.
	if _, err := os.Stat(cfgPath); err == nil {
		t.Errorf("upgrade bootstrapped a config at %s — the no-op PersistentPreRunE must prevent config.Load (FR-B3/FR-U12)", cfgPath)
	}
	// And neither stream must carry a config-load/bootstrap error.
	if strings.Contains(errBuf.String(), "config:") || strings.Contains(outBuf.String(), "wrote bootstrap config") {
		t.Errorf("upgrade triggered config.Load (stderr=%q stdout=%q) — the no-op PersistentPreRunE must skip it",
			errBuf.String(), outBuf.String())
	}
}

// TestUpgradeCommand_HelpDisambiguates is the naming-collision guard: upgradeCmd.Long MUST
// distinguish binary self-update from `config upgrade` (the config-schema migration). It also
// verifies --help short-circuits before PersistentPreRunE (so --help never bootstraps either)
// and lists --check.
func TestUpgradeCommand_HelpDisambiguates(t *testing.T) {
	long := upgradeCmd.Long
	if !strings.Contains(long, "upgrade") {
		t.Errorf("Long missing 'upgrade'")
	}
	// It must reference the OTHER command by name so a user reading --help can tell them apart.
	if !strings.Contains(long, "config upgrade") && !strings.Contains(long, "config-upgrade") {
		t.Errorf("Long must disambiguate from 'config upgrade' (the config-schema migration); got:\n%s", long)
	}
	// And it must NOT describe itself as a schema migration.
	if strings.Contains(long, "schema") && strings.Contains(long, "migration") {
		t.Errorf("Long must not describe itself as a schema migration (that's config upgrade)")
	}

	// --help short-circuits before PersistentPreRunE (cobra) — verify it does NOT bootstrap either.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CONFIG_HOME", "")
	cfgPath := config.GlobalConfigPath()

	_, origOut, origErr, origRunE := saveRootState(t)
	defer restoreRootState(t, nil, origOut, origErr, origRunE)

	var b bytes.Buffer
	rootCmd.SetOut(&b)
	rootCmd.SetErr(&b)
	rootCmd.SetArgs([]string{"upgrade", "--help"})
	_ = Execute(context.Background())

	if _, err := os.Stat(cfgPath); err == nil {
		t.Errorf("upgrade --help bootstrapped a config")
	}
	if !strings.Contains(b.String(), "check") {
		t.Errorf("upgrade --help must list --check; got:\n%s", b.String())
	}
}

// TestUpgradeCommand_FlagValidation exercises the 3 flag-contract rules
// (--version+--prerelease mutex; --rollback vs --check/--version mutex; --channel enum),
// plus valid-combo negatives. validateUpgradeFlags is a pure helper taking *pflag.FlagSet,
// so each case builds a fresh FlagSet (no reliance on the package singleton).
func TestUpgradeCommand_FlagValidation(t *testing.T) {
	// newFS mirrors upgradeCmd's relevant flags for the bool-mutex rules (which use fs.Changed).
	newFS := func() *pflag.FlagSet {
		fs := pflag.NewFlagSet("upgrade", pflag.ContinueOnError)
		fs.Bool("check", false, "")
		fs.String("version", "", "")
		fs.Bool("prerelease", false, "")
		fs.Bool("rollback", false, "")
		fs.String("channel", "", "")
		return fs
	}

	// (1) --version + --prerelease mutex.
	fs := newFS()
	_ = fs.Set("version", "1.2.3")
	_ = fs.Set("prerelease", "true")
	if err := validateUpgradeFlags(fs); err == nil {
		t.Errorf("--version+--prerelease must error")
	}

	// (2a) --rollback + --check mutex.
	fs = newFS()
	_ = fs.Set("rollback", "true")
	_ = fs.Set("check", "true")
	if err := validateUpgradeFlags(fs); err == nil {
		t.Errorf("--rollback+--check must error")
	}

	// (2b) --rollback + --version mutex.
	fs = newFS()
	_ = fs.Set("rollback", "true")
	_ = fs.Set("version", "1.2.3")
	if err := validateUpgradeFlags(fs); err == nil {
		t.Errorf("--rollback+--version must error")
	}

	// (3) --channel unknown value. Rule (3) reads the flagChannel PACKAGE VAR (not fs):
	// set it directly + restore.
	orig := flagChannel
	defer func() { flagChannel = orig }()
	flagChannel = "bogus"
	if err := validateUpgradeFlags(newFS()); err == nil {
		t.Errorf("--channel bogus must error")
	}

	// (negative) valid combos pass.
	flagChannel = ""
	fs = newFS()
	_ = fs.Set("check", "true")
	if err := validateUpgradeFlags(fs); err != nil {
		t.Errorf("--check alone must pass; got %v", err)
	}
	flagChannel = "prerelease"
	if err := validateUpgradeFlags(newFS()); err != nil {
		t.Errorf("--channel prerelease must pass; got %v", err)
	}
}
