# Research — go-toml/v2 tag behavior on the v2 Config fields (empirical probe)

> **Method:** extracted the modified `config.go` (RoleConfig + CurrentConfigVersion + the six v2 fields +
> updated Defaults()) and a probe test into an isolated temp module (`go 1.22` +
> `github.com/pelletier/go-toml/v2 v2.4.2`, matching the project), then `go test -v` observed the actual
> marshal output. Every claim below is from a real run (2026-07-01), not documentation inference. This backs
> `design-decisions.md §1/§4` and the `TestConfig_V2TOMLTags` test in the PRP.

## 1. The three tag classes behave as required

`Config` now mixes three toml-tag classes. The probe confirms each:

| Field | Tag | Marshal behavior (observed) | Verdict |
|---|---|---|---|
| `ConfigVersion` | `toml:"config_version"` | emits `config_version = 2` | ✅ file key, present |
| `MaxCommits` | `toml:"max_commits"` | emits `max_commits = 12` (or `= 9` when set) | ✅ file key, present |
| `BinaryExtensions` | `toml:"binary_extensions"` | nil → `binary_extensions = []`; set → `binary_extensions = ['foo', 'bar']` | ✅ file key, present (see §2) |
| `Roles` | `toml:"-"` | **never appears** — even when `Roles = {"planner": {…}}` | ✅ excluded (loader-populated) |
| `Commits` | `toml:"-"` | **never appears** — even when `Commits = 5` | ✅ excluded (CLI/runtime) |
| `Single` | `toml:"-"` | **never appears** — even when `Single = true` | ✅ excluded (CLI/runtime) |
| `NoColor` (existing) | `toml:"-"` | never appears (unchanged) | ✅ baseline confirmed |

**`toml:"-"` is a HARD exclusion** — go-toml/v2 drops the field entirely from marshal output regardless of
its value. This is exactly the `NoColor`/`Providers` precedent the new `toml:"-"` fields rely on.

## 2. nil `[]string` marshals as `[]` (NOT omitted) — the one surprise

A nil `BinaryExtensions` (`[]string`, tag `toml:"binary_extensions"`) marshals to a KEY LINE:
```
binary_extensions = []
```
It is **NOT omitted**. Contrast with nil pointers: a nil `*string`/`*bool` (`Output`, `StripCodeFence`) IS
omitted (which is why the existing `TestTOMLMarshalKeysAndNoColorExclusion` sets both before marshaling).

**Consequence for S1:** marshaling `Defaults()` now emits three new lines — `max_commits = 12`,
`binary_extensions = []`, `config_version = 2` — even though `BinaryExtensions` is nil. This is HARMLESS:
- It does NOT break `TestTOMLMarshalKeysAndNoColorExclusion` (that test checks PRESENCE of a fixed key list
  via `strings.Contains(s, key+" =")` + ABSENCE of `no_color`; the three new lines are additive — confirmed
  by reproducing that test against the modified struct, it still PASSes).
- It is semantically correct: `binary_extensions = []` in a written config means "no extras" (same as the
  built-in denylist only), which is what `nil` means at runtime. (S2's `file.go` decode will read `[]` back
  into a non-nil empty slice — equivalent to nil for the FR3a "merge with denylist" logic; that is S2's concern.)

## 3. THE substring pitfall — `commits` is a SUFFIX of `max_commits` (the trap that broke two test drafts)

The leak-check for the `toml:"-"` field `Commits` has a subtle collision: the marshaled output legitimately
contains `max_commits = 12`, and the string `"commits"` is a suffix of `"max_commits"`. Two naive checks
BOTH false-positive:

| Check | Matches `max_commits = 12`? | Verdict |
|---|---|---|
| `strings.Contains(out, "commits")` | YES (substring) | ❌ false positive |
| `strings.Contains(out, "commits =")` | YES (`max_commits =` contains `commits =`) | ❌ false positive |
| **`hasKeyLine(out, "commits")`** (a trimmed line starting with `commits =`) | NO (`max_commits = 12` trimmed starts with `max_`, not `commits`) | ✅ correct |

**Both drafts of the test FAILED in temp-module validation** before switching to the line-based check. The
fix is the `hasKeyLine` helper (a line whose trimmed form begins with `key =`):

```go
func hasKeyLine(tomlText, key string) bool {
	prefix := key + " ="
	for _, line := range strings.Split(tomlText, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return true
		}
	}
	return false
}
```

This is why the PRP's `TestConfig_V2TOMLTags` uses `hasKeyLine` for BOTH presence (file keys) and absence
(leak check) — it is robust against any suffix/prefix collision among the v2 keys, and it mirrors the
intent of the existing test's `key+" ="` check without the collision risk. (`single` and `roles` happen to
have no collisions, but `hasKeyLine` is uniform and future-proof.)

## 4. The existing marshal test is NOT broken (back-compat confirmed)

Reproduced `TestTOMLMarshalKeysAndNoColorExclusion` verbatim against the modified struct (with `Output` +
`StripCodeFence` set, as the real test does): PASS. The three new marshal lines (`config_version`,
`max_commits`, `binary_extensions`) are additive — the test's presence loop and no_color-absence check are
unaffected. **Do not modify the existing test**; ADD `TestConfig_V2TOMLTags` for the v2 behavior.

## 5. Minor observation — `time.Duration` marshals as nanoseconds

`timeout` marshals as `timeout = 120000000000` (nanoseconds), not `"120s"`. This is pre-existing go-toml/v2
behavior for `time.Duration` and is EXACTLY WHY `Config` is never directly decoded from the §16.2 file
(the struct doc comment's invariant): `file.go`'s `fileConfig` uses a string-duration field and
`materialize()` parses it into `time.Duration`. S1 does not change this; the new scalar fields
(`config_version`, `max_commits`, `binary_extensions`) are plain int/[]string and round-trip cleanly
through go-toml (no duration-style special-casing needed in S2).

## 6. Build/vet/gofmt on the modified config.go

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `gofmt -l` — clean after `gofmt -w` (the struct tag columns realign per section; the implementer runs
  `gofmt -w` as Level 1, so the PRP's code need only be structurally correct, not pre-aligned).
- No new import: `config.go` still imports ONLY `time` (RoleConfig is a same-package plain struct; the const
  is untyped; the new fields are plain types). `go mod tidy` is a no-op.
