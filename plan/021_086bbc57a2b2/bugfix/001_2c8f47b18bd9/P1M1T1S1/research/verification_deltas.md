# Research Notes — P1.M1.T1.S1 (containsReadVerb helper + fix len(paths)==0 branch in RunWorkDescription)

Verification against the CURRENT working tree. The task description + `architecture/bugfix_workdesc.md`
BUG-001 are accurate. These notes record the verified anchors + the loop-restructure design.

## VERIFIED — the bug (internal/generate/workdesc.go:99-102, inside the `for turn := 2; ; turn++` loop)
```go
paths := parseReadLines(out, skeleton)
if len(paths) == 0 {
    // FR-W7: a response with no valid READ line is the commit-message candidate.
    m, parseOK, _ := provider.ParseOutput(out, manifest)   // ← BUG: parses "READ typo.go" as the message
    return m, parseOK, nil
}
```
When the model emits `READ typo.go` (not in the staged skeleton), `parseReadLines` filters it →
`len(paths)==0` → the ENTIRE raw response is parsed as the commit message. Result: "READ typo.go"
becomes the commit subject. The forced-conclusion path (line ~117) correctly calls `stripReadLines`;
the natural path does NOT.

## VERIFIED — the READ-verb tokenization (parseReadLines L158-163, stripReadLines L196-200)
Both use the IDENTICAL verb detection:
```go
fields := strings.Fields(trimmed)
verb := strings.ToUpper(strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'"))
if verb == "READ" { ... }
```
`containsReadVerb` MUST use this exact tokenization (case-insensitive verb, same TrimLeft punctuation set)
so it agrees with parseReadLines/stripReadLines on what counts as a READ line.

## VERIFIED — the helpers available + their contracts
- `parseReadLines(response, skeleton string) []string` (L145): returns STAGED paths only (filters by
  `skeletonPaths(skeleton)`; non-staged silently dropped). Early-returns nil if skeleton is empty.
- `stripReadLines(response string) string` (L189): drops every READ line, returns the remainder.
- `splitPaths(s)` / `normalizePath(raw)` (L207/L215): the path tokenizers parseReadLines uses.
- `buildReadAnswer(ctx, g, cfg, excludes, paths, st)` (L263): builds the diff-chunk answer; its non-staged
  note wording (L277) is `"%s is not in the staged changes (or has no further diff).\n\n"` — but that path
  is for a STAGED path with empty diff, NOT for the genuinely-non-staged targets in the len(paths)==0
  branch. Use the cleaner FR-W3 wording "<p> is not in the staged changes." for the new notes.

## VERIFIED — test home + idiom (NO export needed)
`internal/generate/generate_workdesc_test.go` is `package generate` (INTERNAL test ⇒ calls unexported
helpers directly). Existing pattern: `TestParseReadLines_*` (Basic/CaseInsensitive/MultipleComma/
NonStagedIgnored/NoReadLineIsMessage/Deduplicates) + `TestStripReadLines` — table-driven, direct calls.
The task's "exported as needed for testing" is satisfied by the internal-test package — NO export needed.
New tests mirror this: `TestContainsReadVerb`, `TestReadTargets`, and a RunWorkDescription behavior test
(non-staged READ → noted, not committed).

## THE FIX DESIGN — the len(paths)==0 branch must route THROUGH the round-cap check

CURRENT loop body (L98-128):
```
paths := parseReadLines(out, skeleton)
if len(paths) == 0 { ParseOutput(out); return }            ← BUG (no stripReadLines; mis-handles non-staged)
if st.rounds >= st.N { ...forced conclusion (stripReadLines)... return }   ← round cap
st.rounds++
answer := buildReadAnswer(...)
render+execute → out
```
The round-cap check is AFTER the len(paths)==0 return. So a fix that handles the non-staged-READ case
INSIDE the len(paths)==0 branch (build notes, render, continue) would BYPASS the round cap ⇒ an infinite
loop if the model keeps emitting non-staged READs. THE FIX MUST route the non-staged-READ case THROUGH
the existing round-cap check.

RESTRUCTURED loop body (3-way len(paths)==0 split; minimal change):
```
paths := parseReadLines(out, skeleton)
var answer string
haveAnswer := false
if len(paths) > 0 {
    // normal: build via buildReadAnswer below (after the budget check, as today)
} else if containsReadVerb(out) {
    // BUG-001 fix: READ lines existed but all targets non-staged → FR-W3 notes, continue loop
    answer = buildNonStagedReadAnswer(readTargets(out))   // pre-build; the budget check still applies
    haveAnswer = true
} else {
    // FR-W7: genuinely no READ line → this IS the message (byte-identical to today's non-bug case)
    m, parseOK, _ := provider.ParseOutput(out, manifest)
    return m, parseOK, nil
}
// Budget check (FR-W6) — UNCHANGED, now bounds BOTH the normal and the non-staged-READ paths
if st.rounds >= st.N { ...forced conclusion (stripReadLines(out2))... return }
st.rounds++
if !haveAnswer {   // len(paths) > 0 path
    answer = buildReadAnswer(ctx, deps.Git, cfg, deps.Excludes, paths, &st)
}
// render answer, execute, set out  (UNCHANGED)
```
- `len(paths)>0`: byte-identical to today (buildReadAnswer after budget check).
- `len(paths)==0 && !containsReadVerb`: byte-identical to today's FR-W7 message return.
- `len(paths)==0 && containsReadVerb`: NEW — notes answer, flows through budget check (round cap fires
  when reached) + render. Bounded ⇒ eventually the forced-conclusion path strips READs and parses.

## THE NEW HELPERS

### containsReadVerb(response string) bool
Line-level: any line whose verb (first token, uppercased, TrimLeft-punctuation-stripped) == "READ".
Same tokenization as parseReadLines/stripReadLines.
```go
func containsReadVerb(response string) bool {
	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" { continue }
		fields := strings.Fields(trimmed)
		if len(fields) == 0 { continue }
		verb := strings.ToUpper(strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'"))
		if verb == "READ" { return true }
	}
	return false
}
```

### readTargets(response string) []string
The "variant of parseReadLines that returns raw targets" (NO skeleton filter). Reuses splitPaths +
normalizePath; dedupes; preserves order.
```go
func readTargets(response string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(response, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" { continue }
		fields := strings.Fields(trimmed)
		if len(fields) == 0 { continue }
		verb := strings.ToUpper(strings.TrimLeft(fields[0], " \t,.:;!?-*/`\"'"))
		if verb != "READ" { continue }
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
		if rest == "" { continue }
		for _, raw := range splitPaths(rest) {
			p := normalizePath(raw)
			if p == "" || seen[p] { continue }
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}
```
KEY INVARIANT: in the len(paths)==0 && containsReadBranch, ALL readTargets are non-staged (a staged
target would have made len(paths)>0). So the FR-W3 note "is not in the staged changes" is accurate for
every one of them.

### buildNonStagedReadAnswer(targets []string) string
FR-W3 note per target. If targets is empty (a bare "READ" with no path — degenerate), emit a minimal
generic note so the model isn't fed an empty turn; bounded by the round cap regardless.
```go
func buildNonStagedReadAnswer(targets []string) string {
	if len(targets) == 0 {
		return "(no staged file matches that read request)"   // bare READ; round cap will force conclusion
	}
	var b strings.Builder
	for _, p := range targets {
		fmt.Fprintf(&b, "%s is not in the staged changes.\n\n", p)
	}
	return strings.TrimRight(b.String(), "\n")
}
```

## BEHAVIOR-PRESERVATION PROOF (non-bug cases ⇒ byte-identical)
- A response with a STAGED READ (e.g. `READ a.txt` where a.txt is staged): parseReadLines returns ["a.txt"]
  ⇒ len(paths)>0 ⇒ normal buildReadAnswer path, byte-identical to today.
- A response with NO READ line at all (e.g. `feat: add thing`): parseReadLines returns nil ⇒
  len(paths)==0 && !containsReadVerb ⇒ ParseOutput(out) return, byte-identical to today's FR-W7 path.
- ONLY the previously-buggy case (READ of a non-staged path) changes behavior: was garbage commit
  subject, now an FR-W3 note + loop continuation.

## SCOPE BOUNDARIES (sibling subtasks — do NOT implement here)
- **P1.M1.T2.S1** (BUG-002): buildReadAnswer cursor-exhaustion "end of diff" note. DIFFERENT branch
  (the diff=="" / nextChunk path inside buildReadAnswer), different bug.
- **P1.M1.T3.S1** (BUG-005): chunk boundaries anchor to @@ hunk edges (nextChunk/chunkCount). Different.
- **P1.M1.T4.S1** (BUG-006): part-label byte/rune unit mismatch. Different.
- **P1.M1.T5.S1**: docs sync (README + how-it-works). S1's doc edit is ONLY the code comment on the
  len(paths)==0 branch.
- Do NOT: change parseReadLines/stripReadLines/buildReadAnswer signatures or behavior; change the
  forced-conclusion path (it's already correct); touch the FR-W6 round-cap logic; or change RunWorkDescription's
  signature. S1 adds 3 helpers + restructures the len(paths)==0 branch + adds tests.