# Research — P1.M4.T1.S2: internal/prompt/system.go — buildSystemPrompt

## Objective
`BuildSystemPrompt(examples []string, hasMultiline bool, newRepo bool, target int) string`
in the EXISTING `internal/prompt` package (created by S1's examples.go). Pure
string builder — no git, no config, no error return. Consumes the S1 outputs
(`examples`, `hasMultiline`) plus two scalar settings (`newRepo`, `target`).

## Canonical strings (verbatim from PRD §17.1 / §17.2 / Appendix A)

### §17.2 NEW-REPO path (newRepo == true) — the conventional-commit fallback
```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Target ~50 characters (~7 words). Format: type(scope): description
```
NOTE the new-repo RAW contract is SHORTER than the mature one: it is
`"Output ONLY the commit message. No preamble, no markdown, no code fences."`
— NO "no quoting", NO "If a body is warranted..." line. Capture this exactly.

### §17.1 MATURE path (newRepo == false) — style learning + anti-reuse
```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences,
no quoting. If a body is warranted, use a blank line between subject and body.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Match the tone and style of these recent commits from this repository:
---
<commit 1 full message>
---
<commit 2 full message>
...
(up to 20, ≤100 lines total)

CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.
They show the STYLE to match — format, tone, length, conventions. Producing
the same text you have seen is STRICTLY FORBIDDEN. Your output must be
entirely original wording describing THIS specific change. Reusing example
text is a critical failure.

<multi-line rule>
Target ~50 characters for the subject line.
```

### Multi-line rule variants (selected by hasMultiline)
- hasMultiline==true : `"Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."`
- hasMultiline==false: `"Only output a single-line subject (no body)."`

## Key design decisions

### D1 — target interpolation (the "~50" is really "~<target>")
Appendix A says commit the canonical strings VERBATIM with only
diff/examples/rejection-list interpolated. BUT the function signature takes
`target int` (default SubjectTargetChars=50, PRD §17 / config §[generation]).
Reconciliation: commit the STATIC prose as named Go constants; use
`fmt.Sprintf("Target ~%d characters for the subject line.", target)` (mature)
and `fmt.Sprintf("Target ~%d characters (~7 words). Format: type(scope): description", target)`
(new-repo). With target=50 the rendered text is byte-identical to the PRD.
"(~7 words)" stays static prose — the contract only parameterizes the char count.
TEST: target=72 ⇒ "Target ~72 characters…" appears (proves the param is used,
not a hardcoded 50).

### D2 — newRepo is the ONLY branch
Branch on `newRepo` first. newRepo==true ⇒ §17.2 (IGNORES examples, hasMultiline,
and renders NO examples block / NO anti-reuse). newRepo==false ⇒ §17.1.
The caller (M6 generate) sets newRepo from (CommitCount<=1), which is exactly the
S1 FetchExamples gate. Builder does NOT validate; it is a pure renderer.
Caller contract: when newRepo==false, examples is non-empty (FetchExamples yields
examples only when count>1). Empty examples + newRepo==false still renders the
mature prompt (header+footer, no example lines) — caller's responsibility.

### D3 — RAW contract, NO JSON anywhere (§17.4 / reference D1)
reference_impl.md D1: the Go port uses the RAW default ("Output ONLY the commit
message…"), NOT the reference's JSON `{"commit_message":…}` contract. So the
generated prompt MUST contain the raw-output line and MUST NOT contain the
substring "json" (case-insensitive) ANYWHERE. Verified: §17.1/§17.2 templates
contain NO "json"/"JSON" substring. TEST:
`!strings.Contains(strings.ToLower(p), "json")` on BOTH paths.

### D4 — examples rendering (each wrapped by "---" separator)
```
Match the tone and style of these recent commits from this repository:
---
<ex1>
---
<ex2>
(up to 20, ≤100 lines total)
```
Render: header + "\n", then per example "---\n" + example + "\n", then footer.
"---" is the per-message separator (same token splitExampleGroups splits on).
The ≤ in the footer is a UTF-8 U+2264 — copy VERBATIM from PRD, do NOT replace
with "<=". Slice order = render order (FetchExamples returns newest-first).

### D5 — committed as named Go constants (Appendix A)
Appendix A: "committed verbatim … as Go string constants". So define unexported
consts: systemRole, matureRawContract, newRepoRawContract, essenceInstruction,
examplesHeader, examplesFooter, antiReuseClause, multilineRuleMulti,
multilineRuleSingle, exampleSeparator. Only the two target lines use fmt.Sprintf.
Also export `DefaultSubjectTargetChars = 50` (mirrors S1's DefaultExampleCount=20
pattern — the canonical default the generate layer passes before M5 config wiring).

## Section ordering (mature path, joined by "\n\n")
1 role → 2 rawContract → 3 essence → 4 examplesBlock → 5 antiReuse →
6 multiLineRule → 7 targetLine

## Package-doc ownership (CRITICAL)
S1's examples.go OWNS `// Package prompt` (line 1, `package prompt` line 13).
system.go must use a FILE-level comment (mirror internal/git/log.go:
"// This file adds …" then `package git`) — NOT a package doc — and plain
`package prompt`. Do NOT duplicate the package doc.

## Testing approach (pure function, NO git, NO integration test)
White-box `package prompt` (house convention). stdlib `testing` + `strings` only.
No real-git test needed (pure string assembly). Use strings.Contains assertions
per the contract MOCKING (robust to exact-whitespace choices). Scenarios:
- mature (newRepo=false, hasMultiline=false): contains role, raw contract, each
  example, anti-reuse, the SINGLE-line variant, "Target ~50 characters for the
  subject line."; NOT the multi variant; NOT "json".
- mature + hasMultiline=true: contains the MULTI variant; NOT the single variant.
- new-repo (newRepo=true): contains role, new-repo raw contract, essence,
  "Target ~50 characters (~7 words). Format: type(scope): description"; NOT
  examples block / anti-reuse / "json".
- target interpolation: target=72 ⇒ "Target ~72 characters…" on both paths.
- raw contract present on both paths; no "json" on both paths.
- examples rendered newest-first in slice order, each wrapped by "---".

## Validation gates (verified project convention)
go build ./internal/prompt/ ; go vet ./internal/prompt/ ;
test -z "$(gofmt -l internal/prompt/)" ; go test ./internal/prompt/ ; go test ./...

## Scope boundaries
ONLY add internal/prompt/system.go + internal/prompt/system_test.go. Do NOT touch
examples.go/examples_test.go, internal/git, config (M5, not built), main.go,
Makefile, go.mod, go.sum. Do NOT create payload.go (S3). No new deps. No go mod tidy.
