package git

// White-box tests for the staged-diff capture in diff.go. They are package git
// (NOT git_test) so they can call the unexported (g *Git).run seam, the
// unexported capLines/capBytes helpers, and compose the S2 harness helpers
// (newTempRepo/writeFileStage in gittestutil_test.go) which live as package-git
// _test.go files in this SAME directory. They drive the REAL host git binary
// (git 2.54.0, PRD §20.1 layer 2) — no mocks of git, no go-git — with one
// behavior per Test* function, mirroring plumbing_test.go's posture.

import (
	"fmt"
	"strings"
	"testing"
)

// TestStagedDiff_MarkdownCappedPerFile proves the markdown line cap is
// PER-FILE with head -n semantics: staging ONE .md with more than MaxMdLines
// changed lines, the per-file md diff is head -n capped to MaxMdLines and
// placed FIRST in the result. A small MaxMdLines (5) is used for determinism
// and a large MaxDiffBytes so the byte cap cannot bite (FR2).
func TestStagedDiff_MarkdownCappedPerFile(t *testing.T) {
	g := newTempRepo(t)

	// Build a .md with well over MaxMdLines changed content lines.
	var md strings.Builder
	md.WriteString("MDUNIQUE123\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&md, "line %d\n", i)
	}
	writeFileStage(t, g, "doc.md", md.String())

	cfg := DiffSettings{MaxMdLines: 5, MaxDiffBytes: 300000}
	got, err := g.StagedDiff(cfg)
	if err != nil {
		t.Fatalf("StagedDiff returned error %v; want nil", err)
	}

	// The raw per-file md diff must have MORE than 5 lines so the cap is
	// meaningful (it includes the diff preamble + the 60 added lines).
	rawMd, err := g.run("diff", "--cached", "--", "doc.md")
	if err != nil {
		t.Fatalf("raw diff doc.md: %v", err)
	}
	if got := len(strings.Split(rawMd, "\n")); got <= 5 {
		t.Fatalf("raw md diff has %d lines; want > 5 so the cap is meaningful", got)
	}

	// The capped per-file diff (head -n 5) must be a PREFIX of got (md-first).
	want := capLines(rawMd, 5)
	if !strings.HasPrefix(got, want) {
		t.Errorf("result does not start with the head -n 5 capped md diff:\nwant prefix=%q\ngot           =%q", want, got)
	}
}

// TestStagedDiff_OtherCappedTotalBytes proves the other-diff byte cap is TOTAL
// with head -c semantics: staging a LARGE non-md file, the single other-diff
// command output is head -c capped to MaxDiffBytes TOTAL. A small MaxDiffBytes
// (64) is used for determinism; md_files is empty so the result equals the
// capped other_diff only (FR3).
func TestStagedDiff_OtherCappedTotalBytes(t *testing.T) {
	g := newTempRepo(t)

	// A large non-md file (well over MaxDiffBytes).
	writeFileStage(t, g, "data.txt", strings.Repeat("x\n", 2000))

	cfg := DiffSettings{MaxMdLines: 100, MaxDiffBytes: 64}
	got, err := g.StagedDiff(cfg)
	if err != nil {
		t.Fatalf("StagedDiff returned error %v; want nil", err)
	}

	if len(got) == 0 {
		t.Fatal("result is empty; want a head -c capped other diff")
	}
	if len(got) > 64 {
		t.Errorf("len(result) = %d; want <= 64 (head -c TOTAL byte cap)", len(got))
	}

	// The result must equal head -c 64 of the raw other-diff command.
	rawOther, err := g.run("diff", "--cached", "--",
		":!*.lock", ":!package-lock.json", ":!pnpm-lock.yaml", ":!yarn.lock",
		":!*.snap", ":!*.map", ":!vendor/*", ":!.md", ":!.markdown")
	if err != nil {
		t.Fatalf("raw other diff: %v", err)
	}
	if want := capBytes(rawOther, 64); got != want {
		t.Errorf("result != head -c 64 of raw other diff:\nwant=%q\ngot =%q", want, got)
	}
}

// TestStagedDiff_ExcludesLockSnapMapVendor proves the contract pathspec
// exclusions remove .lock/.snap/.map/vendor/*/package-lock.json from the
// other_diff while keeping a normal source file. Distinctive content markers
// make the Contains assertions unambiguous (FR3; external_deps.md §D).
func TestStagedDiff_ExcludesLockSnapMapVendor(t *testing.T) {
	g := newTempRepo(t)

	writeFileStage(t, g, "a.lock", "LOCK-CONTENT-MARKER\n")
	writeFileStage(t, g, "b.snap", "SNAP-CONTENT-MARKER\n")
	writeFileStage(t, g, "c.map", "MAP-CONTENT-MARKER\n")
	writeFileStage(t, g, "vendor/v.go", "VENDOR-CONTENT-MARKER\n")
	writeFileStage(t, g, "package-lock.json", "PKGLOCK-CONTENT-MARKER\n")
	writeFileStage(t, g, "main.go", "func main() { /* MAIN-KEEP-MARKER */ }\n")

	cfg := DiffSettings{MaxMdLines: 100, MaxDiffBytes: 300000}
	got, err := g.StagedDiff(cfg)
	if err != nil {
		t.Fatalf("StagedDiff returned error %v; want nil", err)
	}

	if !strings.Contains(got, "MAIN-KEEP-MARKER") {
		t.Errorf("result does not contain main.go's MAIN-KEEP-MARKER; want it kept:\n%s", got)
	}

	for _, marker := range []string{
		"LOCK-CONTENT-MARKER",
		"SNAP-CONTENT-MARKER",
		"MAP-CONTENT-MARKER",
		"VENDOR-CONTENT-MARKER",
		"PKGLOCK-CONTENT-MARKER",
	} {
		if strings.Contains(got, marker) {
			t.Errorf("result contains excluded %s; want it absent (pathspec exclusion):\n%s", marker, got)
		}
	}
}

// TestStagedDiff_MdBeforeOther proves that when BOTH a markdown and a non-md
// file are staged, the markdown diff is placed FIRST and the other diff
// follows (plain concatenation, no separator). A small doc.md + main.go with
// distinctive markers, MaxMdLines=100 so the md diff is not truncated (FR4).
func TestStagedDiff_MdBeforeOther(t *testing.T) {
	g := newTempRepo(t)

	writeFileStage(t, g, "doc.md", "MD-FIRST-MARKER\n")
	writeFileStage(t, g, "main.go", "func main() { /* OTHER-MARKER */ }\n")

	cfg := DiffSettings{MaxMdLines: 100, MaxDiffBytes: 300000}
	got, err := g.StagedDiff(cfg)
	if err != nil {
		t.Fatalf("StagedDiff returned error %v; want nil", err)
	}

	// The uncapped md diff (doc.md is small, not truncated) must be a PREFIX
	// of got — markdown placed FIRST.
	mdDiff, err := g.run("diff", "--cached", "--", "doc.md")
	if err != nil {
		t.Fatalf("raw diff doc.md: %v", err)
	}
	if !strings.HasPrefix(got, mdDiff) {
		t.Errorf("result does not start with the markdown diff (md-first):\nwant prefix=%q\ngot           =%q", mdDiff, got)
	}
	if !strings.Contains(got, "OTHER-MARKER") {
		t.Errorf("result does not contain main.go's OTHER-MARKER; want the other diff present:\n%s", got)
	}
}

// TestStagedDiff_NothingStaged proves the ("", nil) non-error contract: on an
// unborn repo with nothing staged, StagedDiff returns an empty string and a
// NIL error (FR5 — the CLI/generate layer decides auto-stage/exit, not this
// primitive). DiffSettings{} also exercises the zero-value clamp to defaults.
func TestStagedDiff_NothingStaged(t *testing.T) {
	g := newTempRepo(t) // unborn, nothing staged

	got, err := g.StagedDiff(DiffSettings{})
	if err != nil {
		t.Fatalf("StagedDiff returned error %v on nothing staged; want nil (FR5)", err)
	}
	if got != "" {
		t.Errorf("StagedDiff = %q on nothing staged; want \"\" (FR5)", got)
	}
}
