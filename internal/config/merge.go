package config

import (
	"fmt"
	"sort"
	"strings"
)

// MergeActiveSettings carries the active (uncommented) settings from `existing` into `fresh`,
// returning fresh with every active old setting preserved verbatim under its [table] heading
// (PRD §9.17 FR-B8 — "config writes never clobber active settings"). Used by `config init --force`
// (Issue 3) so a user who hand-tunes their config and then runs `config init --force` to refresh the
// template does NOT lose their customizations.
//
// Algorithm:
//  1. Scan `existing` for active settings grouped by [table] heading (ActiveSettings).
//  2. Walk `fresh` line by line, tracking the current [table] section. For each active `key = value`
//     line in fresh that the old file also sets actively, REPLACE the value with the old one (verbatim).
//     Record each carried key so step 3 knows what is still owed.
//  3. Any old active setting NOT present in fresh is carried into fresh:
//     - If its section ALREADY exists in fresh, the owed key is INSERTED into that section (right
//       after the section header — ahead of any commented keys, so it lands as active TOML under the
//       right heading; appending a second `[section]` block would be invalid TOML).
//     - If its section does NOT exist in fresh, a new `[section]` block is appended at the end
//       (FR-B8: "An active setting whose section the new template lacks is appended in its own
//       [section] block").
//
// PURE (no I/O). The result is always valid TOML when both inputs are. The `config_version` key is
// intentionally NOT carried from existing: the fresh template carries the current schema version, and
// FR-B8 scopes preservation to USER settings (config_version is metadata, not a user value — carrying
// a stale v2 version would re-introduce the very legacy notice the template refresh is meant to clear).
func MergeActiveSettings(fresh, existing string) string {
	old := ActiveSettings(existing)
	if len(old) == 0 {
		return fresh // nothing to preserve (existing is inert)
	}
	delete(old[""], "config_version") // metadata, not a user setting — fresh carries the current version

	// carried[section] is the set of keys already placed into fresh by the in-place replace pass.
	carried := map[string]map[string]bool{}
	for sec := range old {
		carried[sec] = map[string]bool{}
	}

	// Pass 1: in-place value replacement on lines fresh already has. Also record which sections
	// appear in fresh (freshSections) so Pass 2 can distinguish "insert into existing section" from
	// "append a new section block".
	freshLines := strings.Split(fresh, "\n")
	freshSections := map[string]bool{}
	section := ""
	for i, line := range freshLines {
		if m := activeTableHeaderRe.FindStringSubmatch(line); m != nil {
			section = m[1]
			freshSections[section] = true
			continue
		}
		if m := activeKVRe.FindStringSubmatch(line); m == nil {
			continue
		} else {
			key := m[1]
			if sec, ok := old[section]; ok {
				if val, ok := sec[key]; ok {
					freshLines[i] = formatKV(key, val)
					carried[section][key] = true
				}
			}
		}
	}

	// Collect owed keys (old active keys NOT in-place replaced) per section.
	owed := map[string][]string{} // section → sorted keys
	for sec, keys := range old {
		for key := range keys {
			if !carried[sec][key] {
				owed[sec] = append(owed[sec], key)
			}
		}
		sort.Strings(owed[sec])
	}
	if len(owed) == 0 {
		return strings.Join(freshLines, "\n") // every old setting was in-place replaced
	}

	// Pass 2a: insert owed keys into sections that ALREADY exist in fresh. Walk fresh again; right
	// after each existing section's header, insert that section's owed keys (with a preservation note).
	inserted := map[string]bool{} // sections whose owed keys were inserted in-place
	out := make([]string, 0, len(freshLines)+32)
	section = ""
	for _, line := range freshLines {
		out = append(out, line)
		if m := activeTableHeaderRe.FindStringSubmatch(line); m != nil {
			section = m[1]
			// If this section has owed keys, emit them immediately after the header.
			if keys, ok := owed[section]; ok && freshSections[section] {
				out = append(out, "# preserved from your previous config (FR-B8: config writes never clobber active settings)")
				for _, k := range keys {
					out = append(out, formatKV(k, old[section][k]))
				}
				inserted[section] = true
			}
		}
	}

	// Pass 2b: append owed keys for sections that DON'T exist in fresh as new [section] blocks.
	// Deterministic order: "" (top-level) first, then alphabetical. Top-level keys go before the
	// first [table]; the rest append at the end.
	var newSections []string
	var toplevelOwed []string
	for sec, keys := range owed {
		if sec == "" {
			for _, k := range keys {
				toplevelOwed = append(toplevelOwed, formatKV(k, old[sec][k]))
			}
			continue
		}
		if !freshSections[sec] {
			newSections = append(newSections, sec)
		}
	}
	sort.Strings(newSections)

	result := strings.Join(out, "\n")

	// Insert top-level owed keys just before the first [table] header.
	if len(toplevelOwed) > 0 {
		result = insertBeforeFirstTable(result, strings.Join(toplevelOwed, ""))
	}

	// Append new [section] blocks for sections fresh lacks.
	if len(newSections) > 0 {
		var b strings.Builder
		for _, sec := range newSections {
			b.WriteString("\n[")
			b.WriteString(sec)
			b.WriteString("]\n")
			b.WriteString("# preserved from your previous config (FR-B8: config writes never clobber active settings)\n")
			for _, k := range owed[sec] {
				b.WriteString(formatKV(k, old[sec][k]))
				b.WriteString("\n")
			}
		}
		if !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		result += b.String()
	}
	return result
}

// formatKV renders an active key=value line. `val` is the raw value string exactly as captured by
// ActiveSettings (already trimmed, NOT unquoted — carried verbatim per FR-B8). One trailing newline
// is NOT included (callers add it). A bare key with an empty value renders as `key = ""` so the line
// remains valid TOML (ActiveSettings trims the value, so an original `key = ""` is captured as `` and
// must be re-quoted here to stay valid).
func formatKV(key, val string) string {
	if val == "" {
		return fmt.Sprintf("%s = \"\"", key)
	}
	return fmt.Sprintf("%s = %s", key, val)
}

// insertBeforeFirstTable returns content with the given block of lines inserted immediately before
// the first [table] header line. If content has no table header, the block is appended at the end.
// Used to place owed top-level keys with the other root keys, before any [section].
func insertBeforeFirstTable(content, block string) string {
	lines := strings.Split(content, "\n")
	insertAt := len(lines)
	for i, line := range lines {
		if activeTableHeaderRe.MatchString(line) {
			insertAt = i
			break
		}
	}
	blockLines := strings.Split(strings.TrimRight(block, "\n"), "\n")
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, blockLines...)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}
