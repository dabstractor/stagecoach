package config

import (
	"regexp"
	"strings"
)

// activeKVRe matches an UNCOMMENTED `key = value` assignment at column 0 (a leading '#' fails the
// [A-Za-z_] anchor, so comment lines — including commented-out keys — are NOT matched). The key is
// captured; the value is whatever follows the '='. Leading whitespace is NOT allowed (column-0
// anchored) so an indented continuation or a commented line is excluded. Used by ScanActiveSettings
// to detect active (uncommented) settings for FR-B8/B9.
//
// NOTE: this deliberately matches the WHOLE line as a key=value pair rather than only string values
// (cf. cmd/config.go's kvStringRe). FR-B8/B9 care about ANY active key=value — booleans, ints,
// arrays, strings — because every one is a user setting that must be preserved / must suppress the
// inert classification. Restricting to strings would mis-classify `auto_stage_all = true` as inert.
var activeKVRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\s*=\s*(.*)$`)

// activeTableHeaderRe matches an UNCOMMENTED TOML [table] / [[array-of-tables]] header at column 0,
// capturing the dotted path inside the brackets. A leading '#' fails the column-0 anchor. Used by
// ScanActiveSettings to track the current section while scanning (so an active key is attributed to
// the right heading). Matches both [table] and [[array]] shapes (config files use [table] for all
// our sections, but [[array]] is valid TOML and treated as its own section key here).
var activeTableHeaderRe = regexp.MustCompile(`^\[+([A-Za-z0-9_.-]+)\]+\s*$`)

// ActiveSettings returns the active (uncommented) `key = value` lines grouped by their [table]
// heading. Top-level keys (before any [table]) are grouped under the "" (empty) section. The result
// maps section → key → raw value string (everything after the first '=', trimmed of surrounding
// whitespace, but NOT unquoted — FR-B8/B9 carry values VERBATIM). Commented lines and blank lines
// are ignored. PURE (no I/O) so it is fully unit-testable; the load path (FR-B9) and the config
// init/upgrade commands (FR-B8) share this single source of truth for "what is an active setting".
//
// This is the helper the v2.7 validation report identified as the common root-cause fix for all
// three FR-B8/B9 issues: a single scan of active key=value lines grouped by [table] heading closes
// the load-time false alarm (Issue 2), the upgrade stray-line bug (Issue 1), and the init --force
// clobber (Issue 3).
func ActiveSettings(content string) map[string]map[string]string {
	out := map[string]map[string]string{}
	section := "" // top-level keys live under ""
	for _, line := range strings.Split(content, "\n") {
		// Update the current section on an UNCOMMENTED table header.
		if m := activeTableHeaderRe.FindStringSubmatch(line); m != nil {
			section = m[1]
			continue
		}
		// An active key=value at column 0?
		m := activeKVRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := m[1]
		val := strings.TrimSpace(m[2])
		if out[section] == nil {
			out[section] = map[string]string{}
		}
		out[section][key] = val
	}
	return out
}

// IsInert reports whether content has ZERO active (uncommented) `key = value` settings — i.e. the
// file is all comments, blanks, and/or (commented) table headers. The all-commented reference
// template written by `config init --template` is inert; a freshly-bootstrapped populated config is
// NOT (it has uncommented provider/model/config_version lines). PURE (no I/O).
//
// FR-B9: an inert file must not trigger the load-time "legacy config" notice (there is nothing to
// migrate) and `config upgrade` on an inert file is a no-op. FR-B8: `config init --force` preserves
// active settings — an inert existing file has none to preserve, so the fresh template is written
// verbatim.
func IsInert(content string) bool {
	return len(ActiveSettings(content)) == 0
}
