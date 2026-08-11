// generate_workdesc_test.go — work-description mode tests (PRD §9.26 FR-W1–W8).
//
// Covers three layers:
//  1. prompt.BuildWorkDescSystemPrompt / BuildWorkDescPayload — pure unit tests (the description-first
//     payload + the round-budget system prompt).
//  2. parseReadLines / skeletonPaths / nextChunk — pure unit tests for the loose READ <path> protocol
//     parser (FR-W3) and the chunk cursor (FR-W5).
//  3. CommitStaged end-to-end with the stub agent (the full description-first read/answer loop):
//     - happy path (model READs a file, then emits the message) → committed;
//     - round-budget forced conclusion (FR-W6);
//     - the mode does NOT cascade into multi-turn fallback (FR-W7);
//     - non-append provider → rescue (FR-W4, session_mode gate).
package generate

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dabstractor/stagecoach/internal/config"
	"github.com/dabstractor/stagecoach/internal/git"
	"github.com/dabstractor/stagecoach/internal/prompt"
	"github.com/dabstractor/stagecoach/internal/stubtest"
)

// ---- 1. Prompt builder unit tests ----

func TestBuildWorkDescSystemPrompt_IncludesBudget(t *testing.T) {
	s := prompt.BuildWorkDescSystemPrompt("BASE", 5)
	// The protocol intro and the budget line are both present.
	if !strings.Contains(s, "READ <path>") {
		t.Errorf("system prompt missing READ protocol: %q", s)
	}
	if !strings.Contains(s, "at most 5 responses") {
		t.Errorf("system prompt missing round budget N=5: %q", s)
	}
	if !strings.HasPrefix(s, "BASE\n\n") {
		t.Errorf("system prompt must START with the base prompt: %q", s)
	}
}

func TestBuildWorkDescSystemPrompt_ClampsNonPositive(t *testing.T) {
	// A non-positive budget collapses to 1 (defensive; guarantees termination).
	s := prompt.BuildWorkDescSystemPrompt("BASE", 0)
	if !strings.Contains(s, "at most 1 responses") {
		t.Errorf("non-positive budget should clamp to 1: %q", s)
	}
}

func TestBuildWorkDescPayload_DescriptionFirst(t *testing.T) {
	got := prompt.BuildWorkDescPayload("add login flow", "phrase it casually", "10\t2\tsrc/login.go\n")
	// Description leads (FR-W2: description is content-authoritative, at the top).
	if !strings.HasPrefix(got, "Work description (what this commit does") {
		t.Errorf("payload must lead with the work description: %q", got)
	}
	if !strings.Contains(got, "add login flow") {
		t.Errorf("payload missing the work description text: %q", got)
	}
	// Context block present when set (FR-W1: --context is the _how_).
	if !strings.Contains(got, "phrase it casually") {
		t.Errorf("payload missing the context block: %q", got)
	}
	// Skeleton (the file menu) present (FR-W2).
	if !strings.Contains(got, "src/login.go") {
		t.Errorf("payload missing the skeleton/file menu: %q", got)
	}
	// NO diff bodies (FR-W2: "no diff bodies" — the model READs them on demand).
	if strings.Contains(got, "diff --git") {
		t.Errorf("payload must NOT contain diff bodies: %q", got)
	}
}

func TestBuildWorkDescPayload_NoContext(t *testing.T) {
	got := prompt.BuildWorkDescPayload("fix bug", "", "1\t0\tmain.go\n")
	if strings.Contains(got, "Directing guidance") {
		t.Errorf("empty context must omit the context block: %q", got)
	}
}

// ---- 2. READ-protocol parser unit tests (FR-W3) ----

func TestParseReadLines_Basic(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n5\t0\thelper.go\n"
	got := parseReadLines("Let me check.\nREAD main.go\nThanks", skeleton)
	want := []string{"main.go"}
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("parseReadLines = %v, want %v", got, want)
	}
}

func TestParseReadLines_CaseInsensitiveAndPunctuationForgiving(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	// Backticks, commas, mixed case verb, trailing punctuation — all forgiving (FR-W3).
	got := parseReadLines("read `main.go`, please", skeleton)
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("forgiving parse = %v, want [main.go]", got)
	}
}

func TestParseReadLines_MultipleCommaSeparated(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\ta.go\n5\t0\tb.go\n1\t0\tc.go\n"
	got := parseReadLines("READ a.go, b.go\nREAD c.go", skeleton)
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != 3 {
		t.Fatalf("parseReadLines = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("parseReadLines[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestParseReadLines_NonStagedIgnored(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	// A path NOT in the skeleton is ignored (FR-W3: non-staged/unrecognized ignored).
	got := parseReadLines("READ other.go\nREAD main.go", skeleton)
	if len(got) != 1 || got[0] != "main.go" {
		t.Errorf("non-staged READ should be ignored: %v, want [main.go]", got)
	}
}

func TestParseReadLines_NoReadLineIsMessage(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	// A response with no valid READ line yields no paths → the caller treats it as the message (FR-W7).
	got := parseReadLines("feat: add login\n\nThis is the commit message.", skeleton)
	if len(got) != 0 {
		t.Errorf("a no-READ response must yield no paths: %v", got)
	}
}

func TestParseReadLines_Deduplicates(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n"
	got := parseReadLines("READ main.go\nREAD main.go", skeleton)
	if len(got) != 1 {
		t.Errorf("duplicate READs must dedupe: %v", got)
	}
}

func TestStripReadLines(t *testing.T) {
	// READ lines are removed; the message remains.
	got := stripReadLines("READ a.go\nfeat: my change\n\nbody")
	if got != "feat: my change\n\nbody" {
		t.Errorf("stripReadLines = %q, want the message sans READ lines", got)
	}
	// An all-READ response yields "" (no message → ParseOutput ok=false → rescue).
	if got := stripReadLines("READ a.go\nREAD b.go"); got != "" {
		t.Errorf("all-READ strip = %q, want empty", got)
	}
}

func TestSkeletonPaths_Parses(t *testing.T) {
	skeleton := "Change summary (numstat: added\tdeleted\tpath):\n3\t1\tmain.go\n5\t0\tsub/util.go\n-\t-\tbin.dat\n"
	set := skeletonPaths(skeleton)
	if !set["main.go"] || !set["sub/util.go"] || !set["bin.dat"] {
		t.Errorf("skeletonPaths = %v, want main.go+sub/util.go+bin.dat", set)
	}
}

func TestSkeletonPaths_Empty(t *testing.T) {
	if skeletonPaths("") != nil {
		t.Error("empty skeleton must yield nil")
	}
	if skeletonPaths("Change summary (numstat: added\tdeleted\tpath):\n") != nil {
		t.Error("header-only skeleton must yield nil")
	}
}

func TestNextChunk_SmallDiffIsOneChunk(t *testing.T) {
	diff := "diff --git a/x b/x\n+hello\n"
	chunk, total, advance := nextChunk(diff, 0)
	if total != 1 {
		t.Errorf("small diff total = %d, want 1", total)
	}
	if chunk != diff {
		t.Errorf("small diff chunk = %q, want the whole diff", chunk)
	}
	if advance != len(diff) {
		t.Errorf("advance = %d, want %d", advance, len(diff))
	}
	// Cursor exhausted → empty chunk (FR-W5 end-of-diff).
	c2, _, a2 := nextChunk(diff, len(diff))
	if c2 != "" || a2 != 0 {
		t.Errorf("exhausted cursor chunk = %q advance = %d, want empty/0", c2, a2)
	}
}

// ---- 3. CommitStaged end-to-end (work-description mode) ----

// TestCommitStaged_WorkDescription_HappyPath: the model READs a staged file, then emits a unique
// commit message. The commit lands with that message; HEAD advances; the staged file is committed.
func TestCommitStaged_WorkDescription_HappyPath(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc NewFeature() string { return \"x\" }\n")
	stageFile(t, repo, "feature.go")

	beforeHEAD := headSHA(t, repo)

	// Script: turn 1 = "READ feature.go" (a READ request); turn 2 = the commit message (no READ).
	// SessionMode="append" so RenderMultiTurn's gate passes (FR-W4).
	m := appendScriptManifest(t, bin, []string{"READ feature.go", "feat: add NewFeature function"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add the NewFeature function"

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.CommitSHA == "" {
		t.Fatal("CommitSHA empty — nothing committed")
	}
	if res.Subject != "feat: add NewFeature function" {
		t.Errorf("Subject = %q, want the work-description message", res.Subject)
	}
	if headSHA(t, repo) == beforeHEAD {
		t.Error("HEAD did not advance")
	}
}

// TestCommitStaged_WorkDescription_MessageRoleTimeout verifies FR-R7 on the work-description path:
// the message role's resolved timeout (ResolveRoleTimeout("message", cfg)) — NOT the flat cfg.Timeout —
// bounds RunWorkDescription's per-turn Execute. Sets the GLOBAL large (30s, which would NOT time out
// against a 2000ms stub sleep) and the MESSAGE-ROLE small (150ms → times out). Asserting ErrTimeout
// here proves msgTimeout (150ms), not cfg.Timeout (30s), reached turn-1's Execute. The workdesc path is
// cleanly isolated via CommitStaged's workDescActive branch (it runs RunWorkDescription and SKIPS the
// one-shot/multi-turn default loop, so S1's one-shot msgTimeout path is not exercised). On a turn-1
// timeout RunWorkDescription returns cause=DeadlineExceeded → CommitStaged returns *RescueError{ErrTimeout}.
func TestCommitStaged_WorkDescription_MessageRoleTimeout(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc F() {}\n")
	stageFile(t, repo, "feature.go")

	// turn-1 Execute sleeps 2000ms; the 150ms message-role timeout kills it before any READ/message.
	m := stubtest.Manifest(bin, stubtest.Options{Out: "feat: never reached", SleepMS: 2000})
	appendMode := "append"
	m.SessionMode = &appendMode // REQUIRED: RunWorkDescription's turn-1 RenderMultiTurn gate

	cfg := config.Defaults()
	cfg.WorkDescription = "add F"                                                          // activates the workdesc branch (skips one-shot)
	cfg.Timeout = 30 * time.Second                                                         // LARGE global (would NOT time out)
	cfg.Roles = map[string]config.RoleConfig{"message": {Timeout: 150 * time.Millisecond}} // SMALL role → times out

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected *RescueError on message-role timeout, got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T, want *RescueError (workdesc turn-1 timeout → rescue)", err)
	}
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("errors.Is(err, ErrTimeout) = false, want true (the 150ms message-role timeout bounded turn-1 Execute)")
	}
}

// TestCommitStaged_WorkDescription_RoundBudgetForcesConclusion (FR-W6): with a tiny round budget,
// the model keeps requesting READs past the cap; the forced-conclusion turn demands the message and
// the run commits the message from that turn (the cap guarantees termination).
func TestCommitStaged_WorkDescription_RoundBudgetForcesConclusion(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc F() {}\n")
	stageFile(t, repo, "feature.go")

	// Script: turn 1 = READ; turn 2 = READ (now over the cap of 1); turn 3 = the message.
	m := appendScriptManifest(t, bin, []string{"READ feature.go", "READ feature.go", "feat: forced conclusion"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add F"
	cfg.WorkDescReadRounds = 1 // cap = 1 round; the 2nd READ triggers the forced-conclusion turn

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "feat: forced conclusion" {
		t.Errorf("Subject = %q, want the forced-conclusion message", res.Subject)
	}
}

// TestCommitStaged_WorkDescription_NoCascadeToMultiTurn (FR-W7): work-description mode does NOT
// cascade into §9.24 multi-turn fallback even when the final message is empty/duplicate. A no-valid-
// message run rescues (the existing §9.10 protocol), never multi-turn.
func TestCommitStaged_WorkDescription_NoCascadeToMultiTurn(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "big.txt", strings.Repeat("line\n", 2000)) // huge payload ⇒ multi-turn WOULD trigger if it cascaded
	stageFile(t, repo, "big.txt")

	// Script: every response is a READ that never resolves to a message → after the cap, forced-
	// conclusion turn yields "" (empty) → no valid message → rescue (FR-W7). Multi-turn would have
	// consumed many script entries delivering chunks; here the rescue fires without that.
	m := appendScriptManifest(t, bin, []string{"READ big.txt", "READ big.txt", "READ big.txt", "READ big.txt", "READ big.txt", "READ big.txt", ""})
	cfg := config.Defaults()
	cfg.WorkDescription = "huge change"
	cfg.WorkDescReadRounds = 3
	cfg.MultiTurnFallback = boolPtr(true) // enabled — but FR-W7 says it must NOT trigger
	cfg.MultiTurnChunkTokens = 4          // tiny ⇒ multi-turn WOULD fire on the default path
	cfg.MaxDuplicateRetries = 0

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected a rescue error (no valid message), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-W7 rescue, not a multi-turn cascade)", err)
	}
	if re.Kind != ErrRescue {
		t.Errorf("Kind = %v, want ErrRescue", re.Kind)
	}
}

// TestCommitStaged_WorkDescription_NonAppendProviderRescues (FR-W4): a provider without
// session_mode="append" yields a turn-1 RenderMultiTurn error → rescue (provider support identical
// to §9.24). The run must NOT commit.
func TestCommitStaged_WorkDescription_NonAppendProviderRescues(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")

	beforeHEAD := headSHA(t, repo)

	// RAW NewScript ⇒ SessionMode unset (⇒ "" after Resolve) ⇒ RenderMultiTurn's gate fails.
	m := stubtest.NewScript(t, bin, []string{"feat: x"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add x"

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected a rescue (non-append provider), got nil")
	}
	var re *RescueError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *RescueError (FR-W4 non-append rescue)", err)
	}
	// HEAD must be unchanged (never committed).
	if headSHA(t, repo) != beforeHEAD {
		t.Errorf("HEAD moved on a non-append-provider rescue (repo must be unchanged)")
	}
}

// TestCommitStaged_WorkDescription_OffByDefault (FR-W1): an empty WorkDescription runs the default
// diff-first path unchanged. Sanity: the feature is opt-in and never the default.
func TestCommitStaged_WorkDescription_OffByDefault(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n")
	stageFile(t, repo, "feature.go")

	// Default path: one-shot, single response, no READ protocol. SessionMode unset is fine (Render, not
	// RenderMultiTurn). WorkDescription == "" ⇒ the default loop runs.
	m := stubtest.NewScript(t, bin, []string{"feat: default path"})
	cfg := config.Defaults() // WorkDescription == ""

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.Subject != "feat: default path" {
		t.Errorf("Subject = %q, want the default-path message", res.Subject)
	}
}

// TestStagedNumstatSkeleton_MirrorsStagedSet: the skeleton returned by git.StagedNumstatSkeleton is
// the same file menu StagedDiff prepends — it is the READ-able path set (FR-W2).
func TestStagedNumstatSkeleton_MirrorsStagedSet(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.go", "package main\n")
	writeFile(t, repo, "b.go", "package main\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")

	g := git.New(repo)
	skeleton, err := g.StagedNumstatSkeleton(context.Background(), git.StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedNumstatSkeleton: %v", err)
	}
	if !strings.Contains(skeleton, "a.go") || !strings.Contains(skeleton, "b.go") {
		t.Errorf("skeleton = %q, must list a.go and b.go", skeleton)
	}
	if strings.Contains(skeleton, "diff --git") {
		t.Errorf("skeleton must NOT contain diff bodies: %q", skeleton)
	}
}

// TestStagedFileDiff_SinglePath: StagedFileDiff returns ONE staged file's diff body, no skeleton.
func TestStagedFileDiff_SinglePath(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.go", "package main\n")
	writeFile(t, repo, "b.go", "package main\n")
	stageFile(t, repo, "a.go")
	stageFile(t, repo, "b.go")

	g := git.New(repo)
	diff, err := g.StagedFileDiff(context.Background(), "a.go", git.StagedDiffOptions{DiffContext: 1})
	if err != nil {
		t.Fatalf("StagedFileDiff: %v", err)
	}
	if !strings.Contains(diff, "a.go") {
		t.Errorf("StagedFileDiff(a.go) = %q, must mention a.go", diff)
	}
	if strings.Contains(diff, "b.go") {
		t.Errorf("StagedFileDiff(a.go) must NOT mention b.go: %q", diff)
	}
	// A non-staged path yields "" (the caller notes "not in the staged changes").
	missing, _ := g.StagedFileDiff(context.Background(), "nonexistent.go", git.StagedDiffOptions{DiffContext: 1})
	if missing != "" {
		t.Errorf("StagedFileDiff(nonexistent) = %q, want empty", missing)
	}
}

// TestBuildReadAnswer_EndOfDiff is the BUG-002 regression: a re-READ of a staged file whose diff was
// already fully delivered (cursor exhausted) MUST emit the FR-W5 "end of diff" note — not the empty
// "a.go:\n\n" body the pre-fix code produced. Mirrors TestStagedFileDiff_SinglePath (real git.New(repo),
// a staged file) and sets the cursor deterministically via len(diff)+1 (no hardcoded length). Includes
// a non-exhaustion control (offset=0 ⇒ chunk delivered, NOT "end of diff") proving the branch fires
// ONLY when exhausted.
func TestBuildReadAnswer_EndOfDiff(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "a.go", "package main\n")
	stageFile(t, repo, "a.go")

	g := git.New(repo)
	ctx := context.Background()
	opts := git.StagedDiffOptions{DiffContext: 1}
	diff, err := g.StagedFileDiff(ctx, "a.go", opts)
	if err != nil {
		t.Fatalf("StagedFileDiff: %v", err)
	}
	if diff == "" {
		t.Fatalf("setup: staged a.go diff is empty — test fixture broken")
	}
	cfg := config.Defaults()

	t.Run("exhausted_cursor_emits_end_of_diff", func(t *testing.T) {
		st := &readState{N: 5, offsets: map[string]int{"a.go": len(diff) + 1}} // cursor EXHAUSTED
		got := buildReadAnswer(ctx, g, cfg, nil, []string{"a.go"}, st)
		if !strings.Contains(got, "a.go — end of diff (all parts shown).") {
			t.Errorf("exhausted cursor: got %q, want the 'end of diff' note", got)
		}
		// The BUG-002 hazard: an empty-body "a.go:\n\n" (the pre-fix output) must NOT appear.
		if strings.Contains(got, "a.go:\n\n") {
			t.Errorf("exhausted cursor: got empty-body form %q, want the 'end of diff' note", got)
		}
	})

	t.Run("control_offset_zero_delivers_chunk", func(t *testing.T) {
		st := &readState{N: 5, offsets: map[string]int{"a.go": 0}} // cursor at start
		got := buildReadAnswer(ctx, g, cfg, nil, []string{"a.go"}, st)
		if strings.Contains(got, "end of diff") {
			t.Errorf("control (offset=0): got %q, must NOT contain 'end of diff' (the branch fired prematurely)", got)
		}
		// The diff body must be delivered (the chunk is present).
		if !strings.Contains(got, "a.go") {
			t.Errorf("control (offset=0): got %q, must contain the diff body for a.go", got)
		}
	})
}

// ---- BUG-001 helpers: containsReadVerb / readTargets / buildNonStagedReadAnswer ----

// TestContainsReadVerb covers the BUG-001 gate: ANY line whose verb == "READ" (exact
// parseReadLines/stripReadLines tokenization). Line-level, case-insensitive, punctuation-forgiving.
func TestContainsReadVerb(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"read one path", "READ a.go", true},
		{"case-insensitive verb", "read a.go", true},
		{"message only", "feat: add thing", false},
		{"empty", "", false},
		{"bare READ (no path)", "READ", true},
		{"any line is enough", "READ a.go\nfeat: x", true},
		{"leading punctuation + ws on verb", "  read  a.go", true},
		{"READ-like but not the verb (readfile)", "readfile a.go", false},
		{"quoted verb is NOT recognized (matches parseReadLines tokenization)", "\"READ\" a.go", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := containsReadVerb(tc.in); got != tc.want {
				t.Errorf("containsReadVerb(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestReadTargets covers the raw non-staged variant of parseReadLines: normalized, de-duplicated,
// NO skeleton filter (every READ target returned as-is).
func TestReadTargets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"comma-separated", "READ a.go, b.go", []string{"a.go", "b.go"}},
		{"backtick-stripped", "READ `typo.go`", []string{"typo.go"}},
		{"dedupes", "READ a.go\nREAD a.go", []string{"a.go"}},
		{"no READ yields empty", "feat: add thing", nil},
		{"bare READ yields empty", "READ", nil},
		{"multiple lines preserved in order", "READ a.go\nREAD b.go, c.go", []string{"a.go", "b.go", "c.go"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := readTargets(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("readTargets(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i, w := range tc.want {
				if got[i] != w {
					t.Errorf("readTargets(%q)[%d] = %q, want %q", tc.in, i, got[i], w)
				}
			}
		})
	}
}

// TestBuildNonStagedReadAnswer covers the FR-W3 note builder.
func TestBuildNonStagedReadAnswer(t *testing.T) {
	if got := buildNonStagedReadAnswer([]string{"typo.go"}); got != "typo.go is not in the staged changes." {
		t.Errorf("single target = %q", got)
	}
	if got := buildNonStagedReadAnswer(nil); got != "(no staged file matches that read request.)" {
		t.Errorf("empty targets = %q, want the generic note", got)
	}
	// Each target gets its own note line.
	got := buildNonStagedReadAnswer([]string{"a.go", "b.go"})
	if !strings.Contains(got, "a.go is not in the staged changes.") || !strings.Contains(got, "b.go is not in the staged changes.") {
		t.Errorf("multi-target note missing a target: %q", got)
	}
}

// TestCommitStaged_WorkDescription_NonStagedReadNotCommitted (BUG-001 regression): a model whose
// first response is ONLY a READ of a NON-staged path ("READ typo.go", typo.go not staged) must NOT
// have that READ line parsed as the commit subject. Before the fix the buggy len(paths)==0 branch did
// exactly that (subject == "READ typo.go"). After the fix the run notes typo.go as non-staged (FR-W3)
// and continues; the scripted second response is the real message, which is what commits.
func TestCommitStaged_WorkDescription_NonStagedReadNotCommitted(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc F() {}\n")
	stageFile(t, repo, "feature.go")

	// Script: turn 1 = "READ typo.go" (typo.go is NOT staged → BUG-001 trigger); turn 2 = the message.
	m := appendScriptManifest(t, bin, []string{"READ typo.go", "feat: add F"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add F"

	res, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err != nil {
		t.Fatalf("CommitStaged: %v", err)
	}
	if res.CommitSHA == "" {
		t.Fatal("CommitSHA empty — nothing committed")
	}
	if res.Subject != "feat: add F" {
		t.Errorf("BUG-001 regression: Subject = %q; the non-staged READ line must NOT be the subject (want %q)", res.Subject, "feat: add F")
	}
}

// TestCommitStaged_WorkDescription_NonStagedReadRoundCapBounds (BUG-001 + FR-W6): a model that keeps
// emitting ONLY non-staged READs must hit the round cap and force a conclusion — it must NOT loop
// forever (the critical round-cap-routing requirement of the restructure) and must NOT commit the READ
// line. With a tiny round budget and a script of all-non-staged READs, the cap fires and the
// forced-conclusion turn (which itself strips READ lines) yields no valid message → RescueError.
func TestCommitStaged_WorkDescription_NonStagedReadRoundCapBounds(t *testing.T) {
	bin := stubtest.Build(t)
	repo := t.TempDir()
	initRepo(t, repo)
	commitRaw(t, repo, "initial")
	writeFile(t, repo, "feature.go", "package main\n\nfunc F() {}\n")
	stageFile(t, repo, "feature.go")

	// Script: every turn is a non-staged READ (typo.go never staged). The round cap (WorkDescReadRounds=1)
	// must fire after the first non-staged READ is noted, forcing conclusion; the forced-conclusion turn
	// also gets a non-staged READ → stripped → empty → ok=false → RescueError (NOT an infinite loop).
	m := appendScriptManifest(t, bin, []string{"READ typo.go", "READ typo.go", "READ typo.go"})
	cfg := config.Defaults()
	cfg.WorkDescription = "add F"
	cfg.WorkDescReadRounds = 1 // tiny cap so the bounded-termination path is exercised quickly

	_, err := CommitStaged(context.Background(), Deps{Git: git.New(repo), Manifest: m}, cfg)
	if err == nil {
		t.Fatal("expected a rescue once the round cap forces conclusion on an all-non-staged-READ run; got nil")
	}
}
