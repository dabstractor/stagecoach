# Research: FR3h index-line stripping — post-capture `^index ` filter across 3 diff functions

> Subtask P1.M2.T3.S1. Strip git's per-file `index <oid>..<oid> <mode>` header line from every captured
> diff body (PRD §9.1 FR3h). The blob OIDs are useless to the model and cost ~30 bytes/file. This is a
> POST-CAPTURE string transform — orthogonal to the parallel P1.M2.T2.S1 (which injects `-M`/`-U<ctx>` on
> the ARGV pre-capture). The two compose: T2.S1 shapes what git emits; T3.S1 shapes the captured string.

---

## 1. The index line — shape, why strip, and why post-capture

From `architecture/git_diff_semantics.md` §4 (verified): each file-pair's patch section is
```
diff --git a/path b/path
index 600d48a..62b056e 100644     ← STRIP THIS (blob OIDs + mode; useless to the model; ~30 bytes)
--- a/path                         ← KEEP (file identity)
+++ b/path                         ← KEEP (file identity)
@@ -l,s +l,s @@                    ← KEEP (hunk anchors)
 content/-removed/+added           ← KEEP (the actual change)
```
- **No git flag suppresses it.** `--no-index` is unrelated (compares files outside a repo); `--no-prefix`
  strips `a/`/`b/`; `--abbrev=<n>` only changes OID length. → **post-capture line stripping is the only way.**
- Special forms all carry the same `index <hex>..<hex> <mode>` shape: new file (`index 0000000..<oid>`),
  delete (`index <oid>..0000000`), mode+content change. A **pure rename** (§1, after -M) has NO index line
  (identical blob) → the strip is a no-op there. A **mode-only change** has no index line either.
- **Lines to KEEP** (FR3h + the contract): `diff --git`, `---`, `+++`, `@@`, content, `similarity index N%`,
  `rename from`/`rename to`. The strip targets ONLY `^index <hex>..<hex> `.

## 2. The regex — anchored, OID-form-disambiguated (the safety core)

Contract regex: **`^index [0-9a-f]+\.\.[0-9a-f]+ `** (anchored at start; one-or-more hex on each side of
`..`; trailing space before the mode). This is the disambiguator that makes the strip surgical:

- A content line that merely starts with the word "index" — e.g. a code comment `// index of items` or a
  bare `index of items` — does NOT match: it lacks the `<hex>..<hex> ` form (`of` is not hex). **Kept.**
- In a real diff body, EVERY content line carries a leading marker (`+`/`-`/space), so no content line can
  start with `index ` at all — the markers are a second layer of protection. The regex is belt-and-suspenders.
- `similarity index 100%` starts with `similarity `, not `index ` → kept. `rename from`/`rename to` start
  with `rename ` → kept.

(The `architecture/git_diff_semantics.md` §4 variant uses `[0-9a-f]{7,}` + `\d+$` — marginally stricter.
The contract's `[0-9a-f]+` is authoritative and equally safe in practice; a false positive would require a
content line literally shaped `index <hex>..<hex> <space>`, which cannot occur in a diff body. This PRP
uses the contract regex verbatim.)

**Implementation: package-level compiled `regexp` + a split/drop/join helper.** git.go does NOT currently
import `regexp` → add `"regexp"` to the import block (stdlib; no go.mod change). Compile once at package
scope: `var indexLineRe = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+ `)`.

```go
func stripIndexLines(s string) string {
	if !strings.Contains(s, "index ") {
		return s // fast path: no possible index line (common for pure-rename / mode-only / empty)
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if indexLineRe.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
```
The fast-path `Contains(s, "index ")` only short-circuits when the substring is absent entirely (→ no line
can start with "index "); when a content line contains "index ", it proceeds to the per-line regex check
which correctly keeps it. Safe + avoids a Split/Join alloc in the no-index case.

## 3. The 6 insertion sites — capture → strip → cap (order matters)

All 3 diff functions (`StagedDiff`/`TreeDiff`/`WorkingTreeDiff`) have the SAME two-part shape: a markdown
per-file loop (capture → line cap) and a non-markdown aggregate (capture → byte cap). `stripIndexLines` is
applied to each captured body **IMMEDIATELY after capture (after the exit-code check) and BEFORE the cap**,
so the cap measures the stripped size (the whole point of FR3h — save the ~30 bytes/file against the cap).

Pinned file:line (current working tree; the parallel T2.S1's `opts`/`-M`/`-U` edits land at the ARGV sites
just above each capture and do not shift these capture/cap lines materially — re-confirm line numbers at
edit time):

| Function | Markdown per-file `fileDiff` | Non-markdown `nmDiff` |
|---|---|---|
| StagedDiff | capture L726 → **strip** → line cap L734 | capture L805 → **strip** → byte cap L814 |
| TreeDiff | capture L1177 → **strip** → line cap L1184 | capture L1245 → **strip** → byte cap L1252 |
| WorkingTreeDiff | capture L1312 → **strip** → line cap L1319 | capture L1381 → **strip** → byte cap L1388 |

Each insertion is one line: `fileDiff = stripIndexLines(fileDiff)` (md loop) / `nmDiff = stripIndexLines(nmDiff)`
(nm section), placed between the `if fcode/nmcode != 0` check and the cap block. 6 insertions total.

**Placeholders are untouched by construction.** The `[binary]`/`[excluded]` placeholder lines are synthesized
by `binaryPlaceholderLine`/`excludedPlaceholderLine` and written directly to the `strings.Builder` — they
never pass through `stripIndexLines`, and they never contain an index line. So the contract's "Do NOT strip
inside placeholder lines" is satisfied automatically; no special-casing is needed.

## 4. Tests — unit (the helper logic) + integration (the wiring)

**`TestStripIndexLines`** — a table-driven UNIT test of the helper (no git; deterministic), covering:
1. index line removed (single file).
2. content line starting with "index" but no OID form kept (the contract's headline negative case — both
   a bare `index of items` line AND diff-marked content lines ` // index`/`-index`/`+index`).
3. no index line → byte-identical (incl. the fast path).
4. multiple files → all index lines gone, headers/content kept.
5. `similarity index N%` / `rename from` / `rename to` KEPT (the rename path, composes with T2.S1's -M).
6. empty string → empty string.

**`TestStagedDiff_IndexLineStripped`** — an INTEGRATION test through `StagedDiff` (real git; pins the
wiring): stage a one-line edit to `a.go`; assert NO line in the captured output matches `indexLineRe`
(reuse the package-level regex — white-box `package git`); assert `diff --git a/a.go b/a.go` and `+++ b/a.go`
are retained (the kept structural lines). This proves FR3h is wired into the capture path.

## 5. Golden-fixture reality — NO churn expected (verified)

Grepped every `*_test.go` in `internal/git/` for `index [0-9a-f]`: **zero matches** (the two hits are
comments about "the index", not the `index <oid>..<oid>` line). Every existing assertion is
substring/structural — `Contains("a.md")`, `Count("diff --git a/<file>")`, truncation sentinels,
`[binary]`/`[excluded]` placeholders, file presence. None assert on the index line. So **FR3h stripping
breaks zero existing fixtures** (matches the parallel T2.S1 PRP's "substring-based ⇒ ~zero churn" finding).
The contract's "update golden fixtures that currently expect an index line" is therefore a run-driven
no-op here: RUN the suites, fix only if something breaks (nothing should).

## 6. Scope fences (do NOT touch)

- `buildDiffArgs` and the 9 ARGV call sites — the parallel T2.S1 owns `-M`/`-U`. T3.S1 is purely a
  post-capture string transform; it does NOT touch argv or `buildDiffArgs`.
- `binary.go` (`detectBinaryFiles`/`fileStatuses`) — numstat/name-status; no patch body; no index line.
- The byte/line cap logic itself — unchanged; `stripIndexLines` is inserted BEFORE the cap, the cap code
  is untouched.
- Placeholder emitters (`binaryPlaceholderLine`/`excludedPlaceholderLine`) — untouched.
- `StagedDiffOptions` / config / the 6 production call sites — untouched (FR3h needs no option; it is
  always-on, like FR3e/FR3f per system_context §6).
- docs — DOCS: none (P1.M5 owns the diff-capture doc sync).

## 7. Files touched

- `internal/git/git.go` — ADD `regexp` import + `indexLineRe` var + `stripIndexLines` func; INSERT 6 calls
  (2 per function × 3 functions), each capture→strip→cap.
- `internal/git/stagediff_test.go` — ADD `TestStripIndexLines` (unit) + `TestStagedDiff_IndexLineStripped`
  (integration).
- Nothing else. go.mod/go.sum unchanged (regexp is stdlib).
