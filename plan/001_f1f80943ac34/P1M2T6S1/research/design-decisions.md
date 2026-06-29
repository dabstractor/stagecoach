# P1.M2.T6.S1 — Design Decisions Synthesis

One-stop rationale for the non-obvious calls in `internal/provider/parse.go`. The two companion
briefs (`go-json-map-types.md`, `json-in-prose-brace-balanced.md`) hold the authoritative Go/RFC
backing; this file resolves the work-item ↔ PRD ↔ codebase reconciliation.

## 0. The signature: `ParseOutput` (exported, 3 returns) — NOT PRD's `parseOutput` (2 returns)

| Source | Signature |
|---|---|
| PRD §12.9 (narrative) | `parseOutput(raw string, m Manifest) (msg string, ok bool)` — lowercase, 2 returns |
| Work item (THIS contract) | `ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool)` — exported, 3 returns |

**Decision: implement the WORK-ITEM signature.** Reasons:
1. The work item is the binding contract for this subtask and explicitly names the third return
   `fellback`. PRD §12.9's 2-return form predates the decision to expose the fallback flag.
2. **Exported** because the consumer is the generate orchestrator in `internal/generate` (P1.M3.T4)
   and the dedupe loop (P1.M3.T2) — a different package. Unexported `parseOutput` would be
   unreachable cross-package. Cross-package ⇒ capital P. (Matches `Execute`, `Manifest.Render`,
   `CmdSpec` — all exported in this package for the same reason.)
3. `fellback` is the "parse-fallback flag for logging" the PRD itself names in step 3 ("set a
   parse-fallback flag for logging"). The work item surfaces it as a return so the orchestrator can
   log it (verbose mode, P1.M4.T3.S2) WITHOUT re-deriving it. `ok` alone can't carry "succeeded but
   via fallback" — that's exactly what `fellback` adds.

## 1. `fellback` truth table (the orchestrator's contract)

`fellback == true` means "json mode was requested but JSON parsing/extraction failed, so the cleaned
raw stdout was used as the message." It is a **logging signal only** — it does NOT change retry
behavior (that's `ok`). It is **always false in raw mode** (nothing failed; there was no parse).

| mode | outcome | msg | ok | fellback |
|---|---|---|---|---|
| raw  | non-empty | cleaned s | true  | false |
| raw  | empty     | ""        | false | false |
| json | valid JSON, field is a string | extracted string | true | false |
| json | valid JSON, field absent/null/non-string | cleaned s | true/false (by emptiness) | **true** |
| json | whole+balanced both fail to parse | cleaned s | true/false (by emptiness) | **true** |

Orchestrator (P1.M3.T4): `ok==false` ⇒ parse-retry with `retry_instruction` (FR29). `fellback==true`
⇒ emit a verbose log line ("provider %s: json parse failed, used raw output"). Dedupe (P1.M3.T2)
keys on `msg`; `fellback` is irrelevant to it.

## 2. ParseOutput calls `m.Resolve()` internally — but NOT `m.Validate()`

Mirrors `Manifest.Render` (render.go): Render does `Validate()` then `Resolve()`. ParseOutput needs
`*m.Output`, `*m.JsonField`, `*m.StripCodeFence` — if `m` is unresolved (nil pointers, e.g. a
manifest pulled straight from the registry, which stores merged-but-unresolved per P1.M2.T3), a bare
`*m.Output` **panics**. `Resolve()` guarantees every pointer is non-nil (Output→"raw",
StripCodeFence→true, JsonField→"") so deref is safe on a COPY (caller's manifest untouched).

ParseOutput does **not** call `Validate()`: parsing only consumes Output/JsonField/StripCodeFence, not
Name/Command. A manifest with a missing Command can still have its output parsed (the command already
ran). Validate's requiredness rules (Name/Command) are the orchestrator's concern, not the parser's.
This is a deliberate narrowening vs Render.

## 3. JSON field extraction: comma-ok type assertion; non-string ⇒ fallback (never stringify)

`json.Unmarshal` into `map[string]any` decodes a JSON string → Go `string`, number → `float64`,
bool → `bool`, null → `nil`, object → `map[string]any`, array → `[]any` (see go-json-map-types.md).
The commit-message field (e.g. `"result"`) must be a `string`. Extraction:

```go
v, ok := obj[field].(string)
if !ok { /* field absent / null / non-string → fallback: msg = s, fellback = true */ }
```

**Decision: on `!ok`, FALL BACK to raw — do NOT stringify** (no `fmt.Sprintf("%v", v)`). A non-string
field value is a schema mismatch from the agent (the message was never a string to begin with);
stringifying a `float64` `42`→`"42"` would silently mask the error and could produce a nonsense
commit subject. The raw cleaned stdout is the safer recovery (it's what the model actually printed).
This is the research-recommended policy (go-json-map-types.md §"Recommended fallback policy").

## 4. Brace-balanced extraction: inString + escaped flags, ASCII-safe (no rune decode)

Whole-string `json.Unmarshal` fails on trailing prose (e.g. `{"result":"x"} done` → trailing-content
error) — see go-json-map-types.md §3. The §12.9 retry: find first `{`, scan to the matching `}` at
depth 0. **MUST track `inString` AND `escaped`** or braces/quotes inside string values miscount
(json-in-prose-brace-balanced.md §1 gives the concrete failing example
`{"msg": "She said \"hello\" {world}"}`). Byte-by-byte is UTF-8-safe: `{`,`}`,`"`,`\` are all ASCII
(<0x80) and can never be UTF-8 continuation bytes (RFC 3629 §3) — no `utf8.DecodeRune` needed.

After extracting the balanced substring, **re-run `json.Unmarshal`** on it (the extraction only finds
a syntactically-plausible span; json.Unmarshal is the final validator). If THAT also fails → fallback.

## 5. Code-fence strip (step 2): opener-fence determines closer-fence

Detect the opener prefix (```` ``` ```` OR `~~~`) — that SAME token is the closer searched for via
`strings.LastIndex`. Remove: (a) the entire first line (opener + language tag, e.g. ` ```json`),
(b) from the LAST closer onward. Re-trim. **No closer found** (malformed, opener only) → keep the
body after the opener line (lenient; don't return empty). A fence char inside the message body is
irrelevant — only a *leading* fence triggers stripping (PRD: "starts with").

## 6. Normalization (step 4): literal PRD — \r\n→\n, then collapse 3+\n→2

Follow PRD §12.9 step 4 EXACTLY: `strings.ReplaceAll(msg, "\r\n", "\n")` then collapse runs of 3+
`\n` into `\n\n`. **Do NOT also handle lone `\r`** (old-Mac) — the PRD does not list it; adding it is
scope creep. Implemented with a manual `strings.Builder` pass (no `regexp` dependency — keeps the
stdlib import set to `encoding/json`+`strings`; the codebase has no regexp usage yet and a one-shot
CLI does not need the regex). Applies to `msg` AFTER the output-mode switch (step 3), whether raw or
json-extracted. Final `TrimSpace` (step 5) is last and sets `ok`.

## 7. Stdlib-only — go.mod/go.sum byte-unchanged

Imports: `encoding/json`, `strings` (parse.go); plus `testing`, `strings` (parse_test.go). NO new
module deps, NO `regexp`, NO third-party JSON lib. Matches the stdlib-only principle T5.S1/S2
established. `git diff --exit-code go.mod go.sum` MUST be empty.

## 8. Test placement: in-package, table-driven

`internal/provider/parse_test.go`, `package provider` (white-box — matches manifest_test.go,
executor_test.go, render_test.go, registry_test.go — ALL in-package). One table-driven
`TestParseOutput` over a `[]struct{ name, raw, manifest, wantMsg, wantOK, wantFallback }` slice
covering every work-item scenario (raw clean / raw+fence / json valid / json-in-prose / json
invalid→fallback / empty→ok=false / multi-newline) PLUS edge cases (~~~ tilde fence, fence+lang tag,
json_field missing, json_field non-string, strip_code_fence=false, nested braces in strings, \r\n,
escaped quotes, JSON with trailing prose, ok=false sets fellback correctly). Pure-function tests —
no subprocess, no `mustBin`, no mocking (ParseOutput has no I/O).
