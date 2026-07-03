package main

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/provider"
)

// These white-box tests cover the MOCKING contract for the providers command
// tree (PRD FR46–FR48, US10). They mirror the repo's testing conventions:
// white-box package main, no testify, reflect.DeepEqual, and the pure render
// helpers (renderProvidersList / showProviderManifest) as the hermetic targets.
//
// They deliberately avoid config.Load (which does file/git-config I/O): the
// pure helpers take a *provider.Registry built directly from provider.Builtins
// plus fabricated overrides, so the assertions are hermetic and deterministic.

// manifestEqual reports whether two manifests are equal after normalizing nil
// vs empty for the slice/map fields. go-toml/v2 encodes a nil slice as
// `key = []` and a nil map as `key = {}`, which both decode back to NON-nil
// empty allocations — TOML cannot distinguish them — so a naive
// reflect.DeepEqual would report pi (BareFlags set, Env/Subcommand nil) and
// opencode (BareFlags nil) as having changed across a round-trip. Normalizing
// both sides to nil when a field is empty makes the round-trip lossless for
// the scalar+bool fields, which ARE exactly comparable. (Subcommand, BareFlags,
// and Env are the only reference-typed fields on Manifest.)
func manifestEqual(a, b provider.Manifest) bool {
	if len(a.Subcommand) == 0 && len(b.Subcommand) == 0 {
		a.Subcommand, b.Subcommand = nil, nil
	}
	if len(a.BareFlags) == 0 && len(b.BareFlags) == 0 {
		a.BareFlags, b.BareFlags = nil, nil
	}
	if len(a.Env) == 0 && len(b.Env) == 0 {
		a.Env, b.Env = nil, nil
	}
	return reflect.DeepEqual(a, b)
}

// TestProvidersList_DetectedAndDefault asserts the MOCKING contract for the
// list render: every built-in name appears, detected/not-detected statuses are
// emitted, a fabricated provider is marked "not detected", and the trailing
// default line carries the resolved default name and model.
func TestProvidersList_DetectedAndDefault(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), map[string]provider.Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})
	detected := reg.Detect()

	var buf bytes.Buffer
	if err := renderProvidersList(&buf, reg, detected, "pi", "glm-5-turbo"); err != nil {
		t.Fatalf("renderProvidersList returned error: %v", err)
	}
	out := buf.String()

	// Every built-in provider name is present.
	for _, name := range []string{"pi", "claude", "gemini", "opencode", "codex", "cursor"} {
		if !strings.Contains(out, name) {
			t.Errorf("output missing built-in provider %q\noutput:\n%s", name, out)
		}
	}

	// "detected" status is emitted, and pi (installed in this env) is detected.
	if !strings.Contains(out, "detected") {
		t.Errorf("output missing a detected/not-detected status\noutput:\n%s", out)
	}
	if detected["pi"] && !strings.Contains(out, "pi") {
		t.Errorf("pi is detected but absent from output\noutput:\n%s", out)
	}

	// The fabricated provider is present and marked not detected.
	if !strings.Contains(out, "definitely-not-an-agent-xyz") {
		t.Errorf("output missing fabricated provider name\noutput:\n%s", out)
	}
	// The fabricated row must carry "not detected" somewhere after its name.
	if !strings.Contains(out, "not detected") {
		t.Errorf("output missing \"not detected\" status\noutput:\n%s", out)
	}

	// The trailing default line names the resolved default and model.
	if !strings.Contains(out, "default provider: pi") {
		t.Errorf("output missing \"default provider: pi\" line\noutput:\n%s", out)
	}
	if !strings.Contains(out, "glm-5-turbo") {
		t.Errorf("output missing resolved default model \"glm-5-turbo\"\noutput:\n%s", out)
	}
}

// TestProvidersList_DefaultMarkerOnDefault asserts the "(default)" marker is
// placed on the resolved default provider's row (FR46).
func TestProvidersList_DefaultMarkerOnDefault(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), nil)
	detected := reg.Detect()

	var buf bytes.Buffer
	if err := renderProvidersList(&buf, reg, detected, "claude", "sonnet"); err != nil {
		t.Fatalf("renderProvidersList returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "claude (default)") && !strings.Contains(out, "sonnet (default)") {
		t.Errorf("output missing a \"(default)\" marker on the default provider's row\noutput:\n%s", out)
	}
	// Ensure a non-default provider is NOT mis-marked. pi is a builtin distinct
	// from claude; its row must not carry the default marker.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "pi ") || strings.Contains(line, " pi ") {
			if strings.Contains(line, "(default)") {
				t.Errorf("non-default provider row carries (default) marker: %q", line)
			}
		}
	}
}

// TestProvidersList_NoneDetected asserts that when no provider is detected, the
// default line reads "(none detected)" and the model falls back to "(unset)".
func TestProvidersList_NoneDetected(t *testing.T) {
	// A registry whose single provider is not on $PATH.
	reg := provider.NewRegistry(nil, map[string]provider.Manifest{
		"definitely-not-an-agent-xyz": {Command: "definitely-not-an-agent-xyz"},
	})
	detected := reg.Detect()

	var buf bytes.Buffer
	if err := renderProvidersList(&buf, reg, detected, "", ""); err != nil {
		t.Fatalf("renderProvidersList returned error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "default provider: (none detected)") {
		t.Errorf("output missing \"(none detected)\" default line\noutput:\n%s", out)
	}
	if !strings.Contains(out, "model: (unset)") {
		t.Errorf("output missing \"(unset)\" model fallback\noutput:\n%s", out)
	}
}

// TestProvidersShow_RoundTripsToSameManifest asserts every built-in manifest
// survives a TOML encode→decode round-trip under the nil-vs-empty-normalizing
// manifestEqual helper. This is the gotcha surface: go-toml/v2 encodes nil
// slices/maps as `[]`/`{}` which decode to non-nil empties, so pi (Env nil),
// opencode (BareFlags nil), and codex/gemini/cursor/pi must all compare equal
// after normalization.
func TestProvidersShow_RoundTripsToSameManifest(t *testing.T) {
	for name, orig := range provider.Builtins() {
		var buf bytes.Buffer
		if err := toml.NewEncoder(&buf).Encode(orig); err != nil {
			t.Errorf("encode %q failed: %v", name, err)
			continue
		}
		var decoded provider.Manifest
		if err := toml.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Errorf("decode %q failed: %v\ninput:\n%s", name, err, buf.String())
			continue
		}
		if !manifestEqual(orig, decoded) {
			t.Errorf("manifest %q did not round-trip\norig:   %#v\ndecoded: %#v", name, orig, decoded)
		}
	}
}

// TestProvidersShow_OverrideReflected asserts a user override (FR48) is visible
// in the show output while the rest of the built-in manifest survives the
// field-merge (decisions.md §6): setting only default_model leaves bare_flags
// intact. Routed through showProviderManifest to exercise the real show path.
func TestProvidersShow_OverrideReflected(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), map[string]provider.Manifest{
		"pi": {DefaultModel: "overridden-model"},
	})

	m, ok := reg.Get("pi")
	if !ok {
		t.Fatal(`reg.Get("pi") ok = false, want true`)
	}
	var buf bytes.Buffer
	if err := showProviderManifest(&buf, reg, "pi"); err != nil {
		t.Fatalf("showProviderManifest returned error: %v", err)
	}
	out := buf.String()

	// The override took effect.
	if !strings.Contains(out, "overridden-model") {
		t.Errorf("override default_model not reflected in show output\noutput:\n%s", out)
	}
	// A built-in field (pi's first bare flag) survived the merge.
	if !strings.Contains(out, "--no-tools") {
		t.Errorf("built-in bare_flags did not survive the field-merge\noutput:\n%s", out)
	}
	// Sanity: the merged manifest in-memory also carries both (proves the
	// field-merge happened before encoding, not via the TOML).
	if m.DefaultModel != "overridden-model" {
		t.Errorf("merged DefaultModel = %q, want %q", m.DefaultModel, "overridden-model")
	}
	found := false
	for _, f := range m.BareFlags {
		if f == "--no-tools" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("merged BareFlags lost --no-tools: %#v", m.BareFlags)
	}
}

// TestProvidersShow_UnknownErrors asserts showProviderManifest returns a non-nil
// error whose message mentions "unknown provider" for a name absent from the
// registry (FR47 exit-1 path). This drives the error path directly with a
// hermetic registry, avoiding config.Load file/git-config I/O.
func TestProvidersShow_UnknownErrors(t *testing.T) {
	reg := provider.NewRegistry(provider.Builtins(), nil)

	var buf bytes.Buffer
	err := showProviderManifest(&buf, reg, "no-such-agent")
	if err == nil {
		t.Fatal("showProviderManifest(unknown) err = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error message = %q, want it to mention \"unknown provider\"", err.Error())
	}
}

// TestProvidersCmd_Registered asserts the providers tree is wired onto rootCmd
// (via the package init) without main.go being edited, and that list/show are
// its children. This guards the registration contract with P1.M7.T2.S1.
func TestProvidersCmd_Registered(t *testing.T) {
	providersCmd, _, err := rootCmd.Find([]string{"providers"})
	if err != nil {
		t.Fatalf("rootCmd.Find([providers]) error: %v", err)
	}
	if providersCmd == nil {
		t.Fatal("providers command not registered on rootCmd")
	}
	if providersCmd.Name() != "providers" {
		t.Errorf("providers command Name = %q, want %q", providersCmd.Name(), "providers")
	}
	// Both children are present.
	for _, sub := range []string{"list", "show"} {
		c, _, err := providersCmd.Find([]string{sub})
		if err != nil || c == nil {
			t.Errorf("providers subcommand %q not found: err=%v", sub, err)
		}
	}
}

// The regression tests below drive the REAL list/show RunE closures end-to-end
// (BUG-003). They do NOT target the pure render helpers: the bug was that the
// RunE closures called config.Load(config.Flags{}, "") with an EMPTY Flags
// struct, so the persistent --config / --provider / --model flags and every
// STAGEHAND_* env var (FR34 layers 6-7) were silently ignored. The fix rewires
// both closures to buildFlags(cmd) -> config.Load(flags, ".") (the runDefault
// pattern), and these tests prove the --config flag, the STAGEHAND_CONFIG env
// var, and `providers show` now honor the explicit config file. They mirror
// the newTestCmd/registerPersistentFlags + cmd.Execute() pattern from
// run_test.go (TestVersionShortCircuit) rather than the pure-helper style.

// newProvidersRoot builds a fresh stagehand root *cobra.Command carrying the
// real PRD §15.2 persistent flag set (via registerPersistentFlags) and the
// providers subcommand tree, so a regression test can drive the REAL list/show
// RunE closures via SetArgs + Execute without touching the package-global
// rootCmd (whose flag-parse state would leak across tests). Mirrors newTestCmd
// in run_test.go.
func newProvidersRoot(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "stagehand"}
	registerPersistentFlags(root)
	root.AddCommand(newProvidersCmd())
	return root
}

// writeCustomProviderConfig writes a temp TOML config file defining a single
// custom provider [provider.customagent] and returns its path. The command and
// detect tokens are deliberately a non-existent binary so the provider shows
// "not detected" but is still LISTED — the regression assertion is that the
// name APPEARS at all (BUG-003: it was absent because the RunE ignored
// --config / STAGEHAND_CONFIG). default_model gives `providers show` a token
// to assert on.
func writeCustomProviderConfig(t *testing.T) string {
	t.Helper()
	const conf = `[provider.customagent]
command = "customagent-bin"
detect = "customagent-bin"
default_model = "custom-model-x"
`
	dir := t.TempDir()
	path := dir + "/myconf.toml"
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

// TestProvidersList_HonorsConfigFlag asserts BUG-003 is fixed for the --config
// flag (PRD §15.2, FR34 layer 7): `providers list --config <path>` where <path>
// defines [provider.customagent] now LISTS customagent — it was MISSING before
// the fix (the RunE called config.Load(config.Flags{}, "") with an empty Flags
// struct, so the persistent --config flag was ignored). Drives the REAL list
// RunE via cmd.Execute. Hermetic: an explicit --config path REPLACES the
// global+repo file layers in config.Load, so the fabricated provider is the
// only [provider.*] source regardless of the host's ~/.config/stagehand file.
func TestProvidersList_HonorsConfigFlag(t *testing.T) {
	path := writeCustomProviderConfig(t)

	root := newProvidersRoot(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"providers", "list", "--config", path})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "customagent") {
		t.Errorf("output missing %q (BUG-003 regressed via --config)\noutput:\n%s", "customagent", out)
	}
}

// TestProvidersList_HonorsConfigEnv asserts BUG-003 is fixed for the
// STAGEHAND_CONFIG env var (FR34 layer 6): `STAGEHAND_CONFIG=<path> providers
// list` (with NO --config flag) now LISTS customagent. Before the fix the env
// layer was ignored for the same reason (empty Flags struct). t.Setenv scopes
// the var to this test only; buildFlags reads it during Execute.
func TestProvidersList_HonorsConfigEnv(t *testing.T) {
	path := writeCustomProviderConfig(t)
	t.Setenv("STAGEHAND_CONFIG", path)

	root := newProvidersRoot(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"providers", "list"}) // NO --config flag

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "customagent") {
		t.Errorf("output missing %q (BUG-003 regressed via STAGEHAND_CONFIG)\noutput:\n%s", "customagent", out)
	}
}

// TestProvidersShow_HonorsConfigFlag asserts BUG-003 is fixed for `providers
// show` (FR47): `providers show <name> --config <path>` loads the override
// manifest from the explicit config file and prints it as TOML. Before the
// fix the show RunE ignored --config (same empty-Flags-struct bug), so a
// provider defined ONLY in the --config file was "unknown". The assertion
// checks the override command token ("customagent-bin") reaches the output.
func TestProvidersShow_HonorsConfigFlag(t *testing.T) {
	path := writeCustomProviderConfig(t)

	root := newProvidersRoot(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"providers", "show", "customagent", "--config", path})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "customagent-bin") {
		t.Errorf("output missing override command %q (BUG-003 regressed for show)\noutput:\n%s", "customagent-bin", out)
	}
}
