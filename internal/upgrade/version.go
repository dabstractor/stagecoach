// Package upgrade provides a comparable (semver) representation of the running binary's
// version, distinct from cmd/stagecoach/main.go's resolveVersion() which produces a DISPLAY
// string such as "dev (19f4df7-dirty)" that feeds --version and is NOT comparable.
//
// This package's primitives — ParseAndClean, Compare, CurrentSemver — are used by
// stagecoach upgrade --check (FR-U6) and the direct-swap path (FR-U5 step 1, §21.1)
// to compare the running binary's version against the latest release tag.
// A dev build (version == "dev") yields ok=false so --check stays informational
// and does not falsely claim an update is available.
//
// All implementation is stdlib-only (errors, strconv, strings) — no external semver
// dependency is added, matching the repo's minimal-deps posture.
//
// releases.go adds the package's network surface — the GitHub Releases metadata client used by
// stagecoach upgrade to resolve a target release (FR-U5 step 2, FR-U6). It is the SOLE network
// call in stagecoach: §19 scopes "no network calls" to the commit path (§9.1–§9.28) and names
// stagecoach upgrade as the explicit exception, which fetches ONLY the project's own release
// tags, asset URLs, and asset sizes — never credentials, never a diff, never repo data. Like the
// rest of the package it is stdlib-only (net/http + encoding/json).
package upgrade

import (
	"strconv"
	"strings"
)

// currentVersion holds the raw ldflags-injected version string. Default "dev" (unparseable).
// Set once at startup by main.go via SetCurrentVersion; injectable for tests (restore via t.Cleanup).
var currentVersion = "dev"

// SetCurrentVersion stores the raw ldflags version for later comparison. Called once at startup
// from cmd/stagecoach/main.go with the raw version var (NOT the resolveVersion display string).
// Injectable for tests; restore the previous value via t.Cleanup.
func SetCurrentVersion(v string) {
	if v != "" {
		currentVersion = v
	}
}

// CurrentSemver returns the normalized v-prefixed semver of the running binary and whether it
// is a valid release version. A dev build (or any unparseable value) returns ("", false).
func CurrentSemver() (string, bool) {
	return ParseAndClean(currentVersion)
}

// ParseAndClean normalizes a raw version string into v-prefixed canonical semver form.
// Accepted forms: "1.0.0", "v1.0.0", "1.0.0-rc1", "v1.0.0+build".
// Rejected: empty, "dev", "dev ...", malformed core (wrong parts, leading zeros, non-numeric),
// malformed prerelease (empty, invalid characters).
// Build metadata (after "+") is stripped per semver §10.
// Returns v-prefixed canonical ("vMAJOR.MINOR.PATCH[-prerelease]") and true, or ("", false).
func ParseAndClean(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "dev" || strings.HasPrefix(raw, "dev ") {
		return "", false
	}
	raw = strings.TrimPrefix(raw, "v")
	raw = strings.TrimPrefix(raw, "V")

	// Strip build metadata (semver §10: ignored in precedence).
	if i := strings.Index(raw, "+"); i >= 0 {
		raw = raw[:i]
	}

	core, pre, hasPre := raw, "", false
	if i := strings.Index(raw, "-"); i >= 0 {
		core, pre, hasPre = raw[:i], raw[i+1:], true
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return "", false
	}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return "", false
		}
		if len(p) > 1 && p[0] == '0' {
			return "", false // no leading zeros unless exactly "0"
		}
	}

	if hasPre {
		if pre == "" {
			return "", false
		}
		for _, id := range strings.Split(pre, ".") {
			if id == "" || !validPreID(id) {
				return "", false
			}
		}
		return "v" + core + "-" + pre, true
	}
	return "v" + core, true
}

// validPreID returns true if s is a non-empty string of [0-9A-Za-z-] characters (semver §9).
func validPreID(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-') {
			return false
		}
	}
	return s != ""
}

// Compare compares two semver strings by precedence (semver §11).
// Returns -1 if a < b, 0 if a == b (or either is unparseable), +1 if a > b.
// An unparseable operand yields 0 (equal) so a dev build can never falsely signal
// an update is available (defense-in-depth for FR-U6's dev-build rule).
func Compare(a, b string) int {
	ca, oka := ParseAndClean(a)
	cb, okb := ParseAndClean(b)
	if !oka || !okb {
		return 0
	}

	// Strip leading "v" from both normalized forms.
	ca = strings.TrimPrefix(ca, "v")
	cb = strings.TrimPrefix(cb, "v")

	aCore, aPre, aHasPre := splitPre(ca)
	bCore, bPre, bHasPre := splitPre(cb)

	// Compare major.minor.patch numerically.
	aParts := strings.Split(aCore, ".")
	bParts := strings.Split(bCore, ".")
	for i := 0; i < 3; i++ {
		aN, _ := strconv.Atoi(aParts[i])
		bN, _ := strconv.Atoi(bParts[i])
		if aN < bN {
			return -1
		}
		if aN > bN {
			return 1
		}
	}

	// Core equal: prerelease precedence.
	// §11.4: no prerelease > has prerelease (release is higher).
	if !aHasPre && bHasPre {
		return 1
	}
	if aHasPre && !bHasPre {
		return -1
	}
	if !aHasPre && !bHasPre {
		return 0
	}

	// Both have prerelease: compare dot-identifier lists left-to-right (§11.4).
	aIDs := strings.Split(aPre, ".")
	bIDs := strings.Split(bPre, ".")
	for i := 0; i < len(aIDs) && i < len(bIDs); i++ {
		cmp := cmpPreID(aIDs[i], bIDs[i])
		if cmp != 0 {
			return cmp
		}
	}
	// All preceding equal: longer set of identifiers is greater.
	if len(aIDs) < len(bIDs) {
		return -1
	}
	if len(aIDs) > len(bIDs) {
		return 1
	}
	return 0
}

// splitPre splits a core-prerelease string (no leading "v") into core, prerelease, and hasPre.
func splitPre(s string) (core, pre string, hasPre bool) {
	if i := strings.Index(s, "-"); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
}

// cmpPreID compares two prerelease identifiers per semver §11:
// both numeric → numeric compare; both non-numeric → ASCII lexical; mixed → numeric is lower.
func cmpPreID(a, b string) int {
	aNum := isAllDigits(a)
	bNum := isAllDigits(b)
	switch {
	case aNum && bNum:
		aN, _ := strconv.Atoi(a)
		bN, _ := strconv.Atoi(b)
		if aN < bN {
			return -1
		}
		if aN > bN {
			return 1
		}
		return 0
	case !aNum && !bNum:
		return strings.Compare(a, b)
	case aNum: // a numeric, b not → numeric is lower
		return -1
	default: // b numeric, a not → numeric is lower
		return 1
	}
}

// isAllDigits returns true if s is non-empty and every rune is a digit.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
