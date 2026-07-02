package config

import (
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// stripComments removes a single leading '#' plus an optional following space
// from every non-blank line of s. It leaves blank lines blank and leaves
// INLINE trailing comments intact (go-toml/v2 ignores them). Its purpose is to
// turn ExampleConfig() — a fully-commented template — back into the TOML body
// the loader would parse, so the "parses back to defaults" white-box test can
// assert the documented scalar values equal the Default* constants.
//
// Note the convention it relies on: in ExampleConfig(), uncomment-to-activate
// settings are written as `# key = value` (one '#'), while permanent notes are
// written as `## note` (two '#'); stripping one '#' from a `## note` line
// leaves a `# note` comment that go-toml ignores, so prose never breaks the
// parse.
func stripComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteByte('\n')
			continue
		}
		rest := line
		if strings.HasPrefix(rest, "#") {
			rest = rest[1:]
			// Remove at most one optional space so `# key` → `key` and
			// `## note` → `# note`.
			if strings.HasPrefix(rest, " ") {
				rest = rest[1:]
			}
		}
		b.WriteString(rest)
		b.WriteByte('\n')
	}
	return b.String()
}

// TestExampleConfig_ParsesToDefaults is the MOCKING contract for the example
// template: stripping the leading '# ' per line and toml.Unmarshal-ing the
// result into fileDTO (the in-package TOML shape) yields [defaults] and
// [generation] pointer fields that are non-nil and equal the Default*
// constants. This proves the documented template cannot drift from the code
// that reads it (Mode A). It is white-box package config because fileDTO and
// parseDuration are UNEXPORTED.
func TestExampleConfig_ParsesToDefaults(t *testing.T) {
	uncommented := stripComments(ExampleConfig())

	var d fileDTO
	if err := toml.Unmarshal([]byte(uncommented), &d); err != nil {
		t.Fatalf("stripped ExampleConfig() failed to parse as TOML: %v\nbody:\n%s", err, uncommented)
	}

	if d.Defaults == nil {
		t.Fatal("parsed [defaults] table is nil, want non-nil")
	}
	if d.Defaults.Provider == nil {
		t.Fatal("d.Defaults.Provider is nil, want non-nil")
	}
	if got := *d.Defaults.Provider; got != "" {
		t.Errorf("defaults.provider = %q, want %q (empty = auto-resolve)", got, "")
	}
	if d.Defaults.Model == nil {
		t.Fatal("d.Defaults.Model is nil, want non-nil")
	}
	if got := *d.Defaults.Model; got != "" {
		t.Errorf("defaults.model = %q, want %q (empty = manifest default_model)", got, "")
	}
	if d.Defaults.Timeout == nil {
		t.Fatal("d.Defaults.Timeout is nil, want non-nil")
	}
	gotTimeout, err := parseDuration(*d.Defaults.Timeout)
	if err != nil {
		t.Fatalf("parseDuration(%q) error: %v", *d.Defaults.Timeout, err)
	}
	if gotTimeout != DefaultTimeout {
		t.Errorf("defaults.timeout = %v, want %v", gotTimeout, DefaultTimeout)
	}
	if d.Defaults.AutoStageAll == nil {
		t.Fatal("d.Defaults.AutoStageAll is nil, want non-nil")
	}
	if got := *d.Defaults.AutoStageAll; got != DefaultAutoStageAll {
		t.Errorf("defaults.auto_stage_all = %v, want %v", got, DefaultAutoStageAll)
	}

	if d.Generation == nil {
		t.Fatal("parsed [generation] table is nil, want non-nil")
	}
	if d.Generation.MaxDiffBytes == nil {
		t.Fatal("d.Generation.MaxDiffBytes is nil, want non-nil")
	}
	if got := *d.Generation.MaxDiffBytes; got != DefaultMaxDiffBytes {
		t.Errorf("generation.max_diff_bytes = %d, want %d", got, DefaultMaxDiffBytes)
	}
	if d.Generation.MaxMdLines == nil {
		t.Fatal("d.Generation.MaxMdLines is nil, want non-nil")
	}
	if got := *d.Generation.MaxMdLines; got != DefaultMaxMdLines {
		t.Errorf("generation.max_md_lines = %d, want %d", got, DefaultMaxMdLines)
	}
	if d.Generation.MaxDuplicateRetries == nil {
		t.Fatal("d.Generation.MaxDuplicateRetries is nil, want non-nil")
	}
	if got := *d.Generation.MaxDuplicateRetries; got != DefaultMaxDuplicateRetries {
		t.Errorf("generation.max_duplicate_retries = %d, want %d", got, DefaultMaxDuplicateRetries)
	}
	if d.Generation.Output == nil {
		t.Fatal("d.Generation.Output is nil, want non-nil")
	}
	if got := *d.Generation.Output; got != string(DefaultOutput) {
		t.Errorf("generation.output = %q, want %q", got, DefaultOutput)
	}
	if d.Generation.StripCodeFence == nil {
		t.Fatal("d.Generation.StripCodeFence is nil, want non-nil")
	}
	if got := *d.Generation.StripCodeFence; got != DefaultStripCodeFence {
		t.Errorf("generation.strip_code_fence = %v, want %v", got, DefaultStripCodeFence)
	}
	if d.Generation.SubjectTargetChars == nil {
		t.Fatal("d.Generation.SubjectTargetChars is nil, want non-nil")
	}
	if got := *d.Generation.SubjectTargetChars; got != DefaultSubjectTargetChars {
		t.Errorf("generation.subject_target_chars = %d, want %d", got, DefaultSubjectTargetChars)
	}
}

// TestExampleConfig_DocumentsEveryKey asserts every §16.2 key and the
// [provider.pi]/[provider.myagent] table headers appear in ExampleConfig(), so
// the generated file documents the full key surface (Mode A reference).
func TestExampleConfig_DocumentsEveryKey(t *testing.T) {
	s := ExampleConfig()
	if s == "" {
		t.Fatal("ExampleConfig() returned empty string, want the §16.2 template")
	}
	for _, key := range []string{
		"provider",
		"model",
		"timeout",
		"auto_stage_all",
		"verbose",
		"max_diff_bytes",
		"max_md_lines",
		"max_duplicate_retries",
		"output",
		"strip_code_fence",
		"subject_target_chars",
		"[provider.pi]",
		"[provider.myagent]",
	} {
		if !strings.Contains(s, key) {
			t.Errorf("ExampleConfig() missing key %q", key)
		}
	}
}
