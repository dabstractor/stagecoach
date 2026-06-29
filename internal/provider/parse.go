package provider

import (
	"encoding/json"
	"strings"
)

// ParseOutput is the fourth and final stage of the provider pipeline (PRD §12.9 / §9.6 FR26–FR29):
// manifests (T1–T3) describe the agent; Render (T4) composes the CmdSpec; Execute (T5) runs it and
// captures stdout; ParseOutput (T6) turns that stdout into the commit message. It is a pure function
// over the captured stdout string and the manifest's three output fields.
//
// THE 5-STEP PIPELINE (PRD §12.9, AUTHORITATIVE — implement in EXACTLY this order):
//
//  1. s = strings.TrimSpace(raw).
//  2. If m.strip_code_fence and s starts with ``` or ~~~: remove the first line (fence opener +
//     language tag) and everything from the LAST fence closer onward. Re-trim.
//  3. Switch on m.output:
//     raw  → msg = s.
//     json → json.Unmarshal whole into map[string]any; on failure find the first '{' and the matching
//     '}' (brace-balanced substring) and retry; extract obj[m.json_field] as a string. ANY
//     failure ⇒ fall through to raw (msg = s) and set fellback = true.
//  4. Normalize newlines: \r\n → \n; collapse 3+ consecutive \n to 2.
//  5. msg = strings.TrimSpace(msg); ok = msg != "".
//
// RETURN CONTRACT (the orchestrator's, P1.M3.T4 + dedupe P1.M3.T2):
//   - msg:    the cleaned commit message ("" if the output was empty after cleanup).
//   - ok:     msg != "". ok==false ⇒ the orchestrator retries once with m.retry_instruction (FR29).
//   - fellback: true ONLY in json mode when JSON parsing/extraction failed and the cleaned raw stdout
//     was used instead (the "parse-fallback flag for logging" of §12.9 step 3). ALWAYS false
//     in raw mode. A pure logging signal — it does NOT change retry behavior (that's `ok`).
//
// WHY EXPORTED + 3 RETURNS (not PRD's lowercase 2-return parseOutput): the consumer is internal/generate
// (P1.M3.T4/T2), a different package ⇒ capital P. The third return surfaces the fallback flag so the
// orchestrator/verbose-UI can log it without re-deriving it. See research design-decisions.md §0/§1.
//
// WHY Resolve() NOT Validate(): the Output/JsonField/StripCodeFence fields are *string/*bool POINTERS;
// dereferencing a nil one panics. A manifest from the registry may be unresolved (P1.M2.T3). Resolve()
// (P1.M2.T1.S1) guarantees every pointer non-nil on a COPY (caller's m untouched) — mirrors render.go.
// Validate()'s Name/Command requiredness is the orchestrator's concern, not the parser's.
func ParseOutput(raw string, m Manifest) (msg string, ok bool, fellback bool) {
	r := m.Resolve() // nil-pointer-safe deref; copy — caller's m untouched (mirrors render.go)

	// Step 1: trim leading/trailing whitespace.
	s := strings.TrimSpace(raw)

	// Step 2: optional single-layer code-fence unwrap (``` or ~~~). PREFIX check only.
	if *r.StripCodeFence {
		s = strings.TrimSpace(stripCodeFence(s))
	}

	// Step 3: output-mode switch.
	switch *r.Output {
	case "json":
		msg, fellback = parseJSON(s, *r.JsonField)
	case "raw":
		msg = s
		fellback = false // raw mode never falls back (nothing failed)
	default:
		// Validate() rejects invalid Output; if we get here the caller skipped Validate. Treat as raw
		// (the PRD default) rather than panic — robustness over strictness for an internal helper.
		msg = s
		fellback = false
	}

	// Step 4: normalize newlines (\r\n→\n, then collapse 3+ \n to 2). Literal PRD; no lone-\r handling.
	msg = normalizeNewlines(msg)

	// Step 5: final trim + ok. ok==false ⇒ orchestrator retries with retry_instruction (FR29).
	msg = strings.TrimSpace(msg)
	ok = msg != ""
	return msg, ok, fellback
}

// parseJSON implements §12.9 step 3's json branch: try json.Unmarshal on the whole string; on failure
// extract the first brace-balanced {...} substring and retry; then pull obj[field] as a string. On ANY
// failure (both Unmarshals fail, field absent/null/non-string) return (s, true) — the cleaned raw string
// with the fallback flag set. The caller (ParseOutput) then normalizes s as if it were raw.
//
// Returns (message, fellback). fellback==true ⇔ JSON extraction failed and message == the raw input s.
func parseJSON(s, field string) (string, bool) {
	// Attempt 1: whole-string Unmarshal.
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		// Attempt 2: brace-balanced substring (handles JSON embedded in prose / trailing commentary).
		sub, found := extractJSONObject(s)
		if !found {
			return s, true // no '{' at all, or unbalanced → fallback to raw
		}
		if err := json.Unmarshal([]byte(sub), &obj); err != nil {
			return s, true // balanced span still isn't valid JSON → fallback
		}
	}
	// Extract the configured field as a string. Absent / null / non-string ⇒ fallback (never stringify).
	v, strOK := obj[field].(string)
	if !strOK {
		return s, true // field missing, JSON null, or a non-string type (number/bool/object/array)
	}
	return v, false
}

// extractJSONObject finds the first '{' in s and scans to the matching '}' that returns brace depth to
// zero, correctly ignoring braces and quotes that appear INSIDE JSON string values (RFC 8259 §7 allows
// '{' and '}' unescaped in strings). Returns the balanced substring (inclusive of the braces) and true,
// or "" and false if there is no '{' or the braces never balance.
//
// State machine: `inString` suppresses brace counting inside "..."; `escaped` (one-byte lookahead)
// consumes the byte after a backslash inside a string so an escaped quote `\"` does NOT toggle inString.
// Byte scanning is UTF-8-safe: '{' '}' '"' '\\' are all ASCII (<0x80) and RFC 3629 §3 guarantees ASCII
// bytes never appear as UTF-8 continuation bytes — no utf8.DecodeRune needed.
func extractJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false // consume this byte literally (it was preceded by '\' inside a string)
			continue
		}
		if inString {
			switch c {
			case '\\':
				escaped = true // next byte is escaped
			case '"':
				inString = false
			}
			continue // inside a string — braces/quotes-except-closer don't affect depth
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true // balanced — inclusive slice
			}
		}
	}
	return "", false // ran off the end with depth > 0 — unbalanced
}

// stripCodeFence removes a single layer of markdown code fence if s STARTS with ``` or ~~~ (PRD §12.9
// step 2): it drops the entire first line (the opener plus any language tag, e.g. ```json) and everything
// from the LAST occurrence of the SAME fence token onward, then the caller re-trims. If there is no
// newline after the opener (opener-only line) the result is "". If no matching closer is found the body
// after the opener line is returned as-is (lenient). A fence token that appears mid-string (not at the
// very start) is left untouched — this is a prefix check only.
func stripCodeFence(s string) string {
	var fence string
	switch {
	case strings.HasPrefix(s, "```"):
		fence = "```"
	case strings.HasPrefix(s, "~~~"):
		fence = "~~~"
	default:
		return s // no leading fence
	}
	// Drop the first line (opener + language tag).
	nl := strings.IndexByte(s, '\n')
	if nl < 0 {
		return "" // opener only, nothing follows
	}
	body := s[nl+1:]
	// Drop everything from the LAST matching closer onward.
	if last := strings.LastIndex(body, fence); last >= 0 {
		body = body[:last]
	}
	return body
}

// normalizeNewlines implements §12.9 step 4: convert "\r\n" → "\n", then collapse runs of 3+ consecutive
// "\n" into exactly "\n\n". It does NOT touch a lone "\r" (old-Mac) — the PRD lists only "\r\n". Built
// with a manual pass (no regexp import — keeps the package's import set to encoding/json + strings).
func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	b.Grow(len(s))
	run := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' {
			run++
			if run <= 2 {
				b.WriteByte(c) // keep at most two consecutive newlines
			}
			// 3rd+ consecutive newline: skip (collapse)
			continue
		}
		run = 0
		b.WriteByte(c)
	}
	return b.String()
}
