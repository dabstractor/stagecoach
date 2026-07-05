package prompt

import (
	"strings"
	"testing"
)

// TestBuildStagerTask_CanonicalExact asserts the FULL assembled stager task prompt for a known
// (title, description) is byte-for-byte the §17.6 rendering, in TWO cases: with a files list
// (block PRESENT) and with nil files (block ABSENT — byte-identity minus the block segment).
// Independently derived from PRD §17.6 (not from the implementation) so a match is meaningful.
// Pins the instruction, the blank-line topology, the files block, and the guardrails block.
func TestBuildStagerTask_CanonicalExact(t *testing.T) {
	const title = "Refactor auth middleware"
	const description = "Stage internal/auth/middleware.go and its callers in internal/api/."

	// The shared guardrails tail (verbatim §17.6 wording; em-dash U+2014 preserved).
	const guardrails = "Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply\n" +
		"only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description\n" +
		"assigns to this concept (the files above are where they live); leave everything else unstaged.\n" +
		"Do not commit, do not amend, do not push, do not modify file contents — only update the index.\n" +
		"When done, reply with the list of paths you staged and stop."

	// Case (a): files = []string{"a.go", "b.go"} — the files block is PRESENT.
	t.Run("files present", func(t *testing.T) {
		const want = "Stage, but do NOT commit, all changes in this repository that match this concept:\n" +
			"\n" +
			"Refactor auth middleware\n" +
			"Stage internal/auth/middleware.go and its callers in internal/api/.\n" +
			"\n" +
			"Files for this concept (where these changes live):\n" +
			"a.go\n" +
			"b.go\n" +
			"\n" +
			guardrails

		got := BuildStagerTask(title, description, []string{"a.go", "b.go"})
		if got != want {
			t.Errorf("BuildStagerTask with files mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
		}
	})

	// Case (b): files = nil — the files block is ABSENT (byte-identity to case (a) MINUS the block segment).
	t.Run("files nil", func(t *testing.T) {
		const want = "Stage, but do NOT commit, all changes in this repository that match this concept:\n" +
			"\n" +
			"Refactor auth middleware\n" +
			"Stage internal/auth/middleware.go and its callers in internal/api/.\n" +
			"\n" +
			guardrails

		got := BuildStagerTask(title, description, nil)
		if got != want {
			t.Errorf("BuildStagerTask nil-files mismatch:\n--- got ---\n%q\n--- want ---\n%q", got, want)
		}
	})
}

// TestBuildStagerTask_FilesBlock_OmittedWhenEmpty is the ⚠️§2 regression net: when files is nil OR an
// empty slice, the files block AND its leading "\n\n" are BOTH omitted (no blank-line artifact), and
// the two empty variants render byte-identically to each other AND to the legacy no-block shape.
func TestBuildStagerTask_FilesBlock_OmittedWhenEmpty(t *testing.T) {
	pNil := BuildStagerTask("T", "D", nil)
	pEmpty := BuildStagerTask("T", "D", []string{})

	if strings.Contains(pNil, "Files for this concept") {
		t.Errorf("nil files must NOT contain the files block; got %q", pNil)
	}
	if strings.Contains(pEmpty, "Files for this concept") {
		t.Errorf("empty-slice files must NOT contain the files block; got %q", pEmpty)
	}
	if pNil != pEmpty {
		t.Errorf("nil and []string{} must render byte-identically;\nnil=%q\nempty=%q", pNil, pEmpty)
	}

	// Legacy no-block shape: NO stray "\n\n" before the guardrails.
	legacy := stagerInstruction + "\n\n" + "T" + "\n" + "D" + "\n\n" + stagerGuardrails
	if pNil != legacy {
		t.Errorf("nil-files output must equal the legacy (no-block) shape;\ngot=%q\nwant=%q", pNil, legacy)
	}
}

// TestBuildStagerTask_Properties is a table of structural invariants guarding the load-bearing decisions:
// instruction/guardrails presence, verbatim tokens, em-dash fidelity, title+description interpolation
// in order, multi-line description preservation, and anti-copy-paste guards pinning §17.1/§17.5
// elements ABSENT.
func TestBuildStagerTask_Properties(t *testing.T) {
	const title = "TTT"
	const desc = "DDD"
	p := BuildStagerTask(title, desc, nil)
	pFiles := BuildStagerTask(title, desc, []string{"a.go", "b.go"})

	cases := []struct {
		name      string
		needle    string
		mustExist bool
	}{
		// Instruction line.
		{"instruction line present", "Stage, but do NOT commit, all changes in this repository that match this concept:", true},

		// Guardrails block present.
		{"guardrails: first line present", "Use git to stage the relevant files and hunks", true},
		{"guardrails: `git add <path>` backtick command present", "`git add <path>`", true},
		{"guardrails: `git apply --cached` backtick command present", "`git apply --cached`", true},
		{"guardrails: literal <path> token present", "<path>", true},
		{"guardrails: Stage ONLY clause present (new §17.6 wording)", "Stage ONLY the changes the description\nassigns to this concept (the files above are where they live)", true},
		{"guardrails: hard-guardrails clause present", "Do not commit, do not amend, do not push", true},
		{"guardrails: 'only update the index' present", "only update the index", true},
		{"guardrails: reply-with-paths instruction present", "reply with the list of paths you staged and stop", true},

		// Anti-copy-paste: §17.1 mature elements ABSENT.
		{"anti-copy-paste: §17.1 'commit message generator' ABSENT", "You are a commit message generator", false},
		{"anti-copy-paste: §17.1 'Output ONLY the commit message' ABSENT", "Output ONLY the commit message", false},
		{"anti-copy-paste: §17.1 anti-reuse block ABSENT", "CRITICAL: You MUST NOT copy", false},
		{"anti-copy-paste: §17.1 'Target ~' ABSENT", "Target ~", false},

		// Anti-copy-paste: §17.5 planner elements ABSENT.
		{"anti-copy-paste: §17.5 'commit-planning assistant' ABSENT", "You are a commit-planning assistant", false},
		{"anti-copy-paste: §17.5 JSON contract ABSENT", "Respond with ONLY JSON", false},
		{"anti-copy-paste: §17.5 planner user-instruction ABSENT", "Decompose these un-staged changes", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			has := strings.Contains(p, tc.needle)
			if tc.mustExist && !has {
				t.Errorf("expected %q in stager prompt; not found", tc.needle)
			}
			if !tc.mustExist && has {
				t.Errorf("stager prompt must NOT contain element %q (copy-paste leak)", tc.needle)
			}
		})
	}

	// Em-dash present (NOT ascii hyphen).
	t.Run("em-dash present (NOT ascii hyphen)", func(t *testing.T) {
		if !strings.Contains(p, "contents — only") {
			t.Errorf("em-dash (U+2014) missing near 'contents'; got %q", near(p, "contents"))
		}
		if strings.Contains(p, "contents - only") { // ASCII hyphen variant
			t.Errorf("expected em-dash '—', found ASCII hyphen '-'")
		}
	})

	// Files block: header PRESENT (with files).
	t.Run("files-header PRESENT (with files)", func(t *testing.T) {
		if !strings.Contains(pFiles, "Files for this concept (where these changes live):") {
			t.Errorf("files-header missing in files-present variant; got %q", pFiles)
		}
	})

	// Files block: per-path rendering PRESENT (with files).
	t.Run("per-path rendering PRESENT (with files)", func(t *testing.T) {
		if !strings.Contains(pFiles, "a.go\nb.go") {
			t.Errorf("per-path rendering missing in files-present variant; got %q", pFiles)
		}
	})

	// Title interpolated, in order before description.
	t.Run("title interpolated before description", func(t *testing.T) {
		i := strings.Index(p, title)
		j := strings.Index(p, desc)
		if i < 0 {
			t.Fatalf("title %q not found in prompt", title)
		}
		if j < 0 {
			t.Fatalf("description %q not found in prompt", desc)
		}
		if i >= j {
			t.Errorf("title must appear BEFORE description; title@%d desc@%d", i, j)
		}
	})

	// Description interpolated, in order after title and before guardrails.
	t.Run("description interpolated after title and before guardrails", func(t *testing.T) {
		i := strings.Index(p, title)
		j := strings.Index(p, desc)
		k := strings.Index(p, "Use git to stage")
		if !(j > i && j < k) {
			t.Errorf("description must be between title and guardrails; title@%d desc@%d guardrails@%d", i, j, k)
		}
	})

	// Title is verbatim (symbols/spaces survive).
	t.Run("title verbatim (symbols/spaces survive)", func(t *testing.T) {
		weirdTitle := "feat(api): add [x] & y"
		p := BuildStagerTask(weirdTitle, "desc", nil)
		if !strings.Contains(p, weirdTitle) {
			t.Errorf("weird title %q not found verbatim in prompt", weirdTitle)
		}
	})

	// Multi-line description: internal newlines survive.
	t.Run("multi-line description: internal newlines survive", func(t *testing.T) {
		multiDesc := "line1\nline2"
		p := BuildStagerTask("title", multiDesc, nil)
		if !strings.Contains(p, "line1\nline2") {
			t.Errorf("multi-line description internal newlines not preserved; got %q", near(p, "line1"))
		}
	})

	// Blank-line topology: exactly one blank line before title.
	t.Run("blank-line topology: one blank line before title", func(t *testing.T) {
		if !strings.HasPrefix(p, stagerInstruction+"\n\n"+title) {
			t.Errorf("expected instruction + blank line + title at start; got %q", near(p, "Stage"))
		}
	})

	// Blank-line topology: exactly one blank line after description (before guardrails).
	t.Run("blank-line topology: one blank line after description", func(t *testing.T) {
		if !strings.Contains(p, desc+"\n\n"+stagerGuardrails) {
			t.Errorf("expected description + blank line + guardrails; got %q", near(p, desc))
		}
	})
}

// TestBuildStagerTask_EdgeCases covers the defensive paths: empty title, empty description, both empty
// — no panic, and the assembly still produces a well-formed string with instruction and guardrails.
func TestBuildStagerTask_EdgeCases(t *testing.T) {
	t.Run("empty title does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildStagerTask(\"\", \"desc\") panicked: %v", r)
			}
		}()
		p := BuildStagerTask("", "desc", nil)
		if !strings.HasPrefix(p, stagerInstruction) {
			t.Error("empty-title output must still start with the instruction")
		}
		if !strings.Contains(p, stagerGuardrails) {
			t.Error("empty-title output must still contain the guardrails")
		}
	})

	t.Run("empty description does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildStagerTask(\"title\", \"\") panicked: %v", r)
			}
		}()
		p := BuildStagerTask("title", "", nil)
		if !strings.HasPrefix(p, stagerInstruction) {
			t.Error("empty-description output must still start with the instruction")
		}
		if !strings.Contains(p, stagerGuardrails) {
			t.Error("empty-description output must still contain the guardrails")
		}
	})

	t.Run("both empty does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildStagerTask(\"\", \"\") panicked: %v", r)
			}
		}()
		p := BuildStagerTask("", "", nil)
		if !strings.HasPrefix(p, stagerInstruction) {
			t.Error("both-empty output must still start with the instruction")
		}
		if !strings.Contains(p, stagerGuardrails) {
			t.Error("both-empty output must still contain the guardrails")
		}
	})
}
