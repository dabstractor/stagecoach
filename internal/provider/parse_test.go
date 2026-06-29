package provider

import (
	"strings"
	"testing"
)

// TestParseOutput is the table-driven suite for the §12.9 pipeline. Pure-function tests — no subprocess,
// no mocking. Covers every work-item scenario (raw clean / raw+fence / json valid / json-in-prose / json
// invalid→fallback / empty→ok=false / multi-newline) plus edge cases. Manifests are built with Resolve()
// applied (the documented call path) OR via literals with strPtr/boolPtr (same-package helpers).
func TestParseOutput(t *testing.T) {
	// Helper: a raw-mode manifest (strip_code_fence on/off configurable).
	rawManifest := func(strip bool) Manifest {
		return Manifest{Name: "t", Command: strPtr("t"), Output: strPtr("raw"), StripCodeFence: boolPtr(strip)}.
			Resolve()
	}
	// Helper: a json-mode manifest with a given json_field.
	jsonManifest := func(field string) Manifest {
		return Manifest{Name: "t", Command: strPtr("t"), Output: strPtr("json"), JsonField: strPtr(field),
			StripCodeFence: boolPtr(true)}.Resolve()
	}

	cases := []struct {
		name         string
		raw          string
		manifest     Manifest
		wantMsg      string
		wantOK       bool
		wantFallback bool
	}{
		// --- raw mode (FR26) ---
		{"raw clean", "fix: handle nil deref in parser", rawManifest(true), "fix: handle nil deref in parser", true, false},
		{"raw trims surrounding whitespace", "  \n fix: x \n ", rawManifest(true), "fix: x", true, false},
		{"raw with ``` fence", "```json\nfix: x\n```", rawManifest(true), "fix: x", true, false},
		{"raw with ~~~ fence", "~~~\nfix: x\n~~~", rawManifest(true), "fix: x", true, false},
		{"raw fence with language tag", "```text\nfix: x\n```", rawManifest(true), "fix: x", true, false},
		{"raw fence multi-line body", "```\nfix: x\n\nbody line 2\n```", rawManifest(true), "fix: x\n\nbody line 2", true, false},
		{"raw strip_code_fence=false preserves fence", "```fix: x```", rawManifest(false), "```fix: x```", true, false},

		// --- empty / whitespace-only (FR29 trigger: ok=false) ---
		{"empty raw → ok=false fellback=false", "", rawManifest(true), "", false, false},
		{"whitespace-only raw → ok=false", "   \n\t  \n", rawManifest(true), "", false, false},
		{"fence-only (empty body) → ok=false", "```\n```", rawManifest(true), "", false, false},

		// --- newline normalization (§12.9 step 4) ---
		{"collapse 3+ newlines to 2", "fix: x\n\n\n\nbody", rawManifest(true), "fix: x\n\nbody", true, false},
		{"crlf normalized", "fix: x\r\n\r\nbody", rawManifest(true), "fix: x\n\nbody", true, false},
		{"keep exactly 2 newlines", "fix: x\n\nbody", rawManifest(true), "fix: x\n\nbody", true, false},

		// --- json mode (FR27) ---
		{"json valid field extracted", `{"result":"fix: x"}`, jsonManifest("result"), "fix: x", true, false},
		{"json with surrounding whitespace", "\n  {\"result\":\"fix: x\"}  \n", jsonManifest("result"), "fix: x", true, false},
		{"json-in-prose (brace-balanced retry)", `Here is the message: {"result":"fix: x"} hope it helps!`,
			jsonManifest("result"), "fix: x", true, false},
		{"json nested object, field is string", `{"meta":{"k":1},"result":"fix: x"}`, jsonManifest("result"), "fix: x", true, false},
		{"json message with newlines is normalized", `{"result":"fix: x\n\n\nbody"}`, jsonManifest("result"), "fix: x\n\nbody", true, false},

		// --- json fallback (fellback=true) ---
		{"json invalid → fallback to raw", "this is not json at all", jsonManifest("result"), "this is not json at all", true, true},
		{"json field missing → fallback", `{"other":"x"}`, jsonManifest("result"), `{"other":"x"}`, true, true},
		{"json field null → fallback", `{"result":null}`, jsonManifest("result"), `{"result":null}`, true, true},
		{"json field non-string (number) → fallback", `{"result":42}`, jsonManifest("result"), `{"result":42}`, true, true},
		{"json field non-string (object) → fallback", `{"result":{"a":1}}`, jsonManifest("result"), `{"result":{"a":1}}`, true, true},
		{"json empty → ok=false fellback=true", "", jsonManifest("result"), "", false, true},

		// --- brace-balanced correctness (strings containing braces/quotes) ---
		{"braces inside json string not miscounted",
			`{"result":"fix: handle { edge } case"}`, jsonManifest("result"),
			"fix: handle { edge } case", true, false},
		{"escaped quotes inside json string",
			`{"result":"She said \"hello\""}`, jsonManifest("result"),
			`She said "hello"`, true, false},
		{"escaped quotes + braces inside json string (in prose)",
			`Here: {"result":"a { b } \"c\""}`, jsonManifest("result"),
			`a { b } "c"`, true, false},
		{"json-in-prose with trailing semicolons/commentary",
			`Sure! {"result":"feat: add parser"} Let me know.`, jsonManifest("result"),
			"feat: add parser", true, false},

		// --- unresolved-manifest safety (Resolve() called internally) ---
		{"unresolved raw manifest does not panic (Output nil→raw default)",
			"fix: x", Manifest{Name: "t", Command: strPtr("t")}, "fix: x", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, ok, fb := ParseOutput(tc.raw, tc.manifest)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if fb != tc.wantFallback {
				t.Errorf("fellback = %v, want %v", fb, tc.wantFallback)
			}
			if msg != tc.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tc.wantMsg)
			}
			// INVARIANT: ok == false ⇒ msg == "" (the FR29 contract).
			if !ok && msg != "" {
				t.Errorf("ok=false but msg=%q (must be empty)", msg)
			}
		})
	}
}

// TestExtractJSONObject_BalancedCorrectness targets the brace-balanced helper directly for the cases
// most likely to be miscounted by a naive depth-only counter (string state + escape state).
func TestExtractJSONObject_BalancedCorrectness(t *testing.T) {
	cases := []struct {
		in        string
		want      string
		wantFound bool
	}{
		{`{"a":1}`, `{"a":1}`, true},
		{`pre {"a":1} post`, `{"a":1}`, true},
		{`{"a":{"b":2}}`, `{"a":{"b":2}}`, true}, // nested
		{`{"a":"}"}`, `{"a":"}"}`, true},         // '}' inside string
		{`{"a":"{"}`, `{"a":"{"}`, true},         // '{' inside string
		{`{"a":"\"{"}`, `{"a":"\"{"}`, true},     // escaped quote then '{' in string
		{`no braces here`, ``, false},
		{`{ "unbalanced`, ``, false}, // depth never returns to 0
	}
	for _, c := range cases {
		got, found := extractJSONObject(c.in)
		if found != c.wantFound {
			t.Errorf("extractJSONObject(%q) found=%v want %v", c.in, found, c.wantFound)
			continue
		}
		if found && got != c.want {
			t.Errorf("extractJSONObject(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestStripCodeFence_FenceVariants targets the fence helper directly.
func TestStripCodeFence_FenceVariants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"```json\nfix\n```", "fix"},
		{"```\nfix\n```", "fix"},
		{"~~~\nfix\n~~~", "fix"},
		{"no fence", "no fence"},
		{"```", ""},         // opener only
		{"```\nfix", "fix"}, // no closer — keep body
	}
	for _, c := range cases {
		if got := strings.TrimSpace(stripCodeFence(c.in)); got != c.want {
			t.Errorf("stripCodeFence(%q) (trimmed) = %q, want %q", c.in, got, c.want)
		}
	}
}
