# Test Recipe: P1.M2.T1.S2

## TestRunLoop_NoStagerAvailable_Errors (the guard test — restore if absent)

Location: `internal/decompose/decompose_test.go`, currently ~line 1068 (added by the previous
attempt). If absent in the checkout, restore verbatim:

```go
// TestRunLoop_NoStagerAvailable_Errors is the BUG-003/FR-M13 guard test: a shared-file (non-disjoint)
// partition routes to runLoop, which genuinely requires a tooled stager. When ResolveRoles (S1) reports
// StagerAvailable=false (no tooled provider + no fallback), runLoop must fail fast with a clear,
// actionable error BEFORE any freeze/generation work — so a zero-Git Deps returns the error without
// dereferencing Git (the guard fires first). The disjoint fast-path (runLoopFastPath) is unaffected.
func TestRunLoop_NoStagerAvailable_Errors(t *testing.T) {
	deps := Deps{Roles: RoleManifests{StagerAvailable: false}} // Git/Planner zero — guard fires first
	concepts := []prompt.PlannerCommit{
		{Title: "a", Files: []string{"shared.go"}},
		{Title: "b", Files: []string{"shared.go"}}, // shared file → non-disjoint (the runLoop case)
	}
	_, _, err := runLoop(context.Background(), deps, concepts, "base", "tstart", "head", false)
	if err == nil {
		t.Fatal("runLoop StagerAvailable=false: want error, got nil")
	}
	if !errors.Is(err, ErrDecomposeFailed) {
		t.Errorf("error not ErrDecomposeFailed-wrapped: %v", err)
	}
	for _, want := range []string{"tooled stager", "tooled_flags", "disjoint", "pi or claude"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}
```

Notes:
- `errors` and `strings` are already imported in `decompose_test.go`.
- `prompt.PlannerCommit`, `runLoop`, `Deps`, `RoleManifests`, `ErrDecomposeFailed` are all in-package
  (`package decompose`) — no import needed for them.

---

## The re-scope: shared_cannot_serve_as_stager (the conflict-resolution edit)

Location: `internal/decompose/decompose_test.go`, the `shared_cannot_serve_as_stager` `t.Run` block
inside `TestDecompose_FastPath_TooledFlagsLessProvider`, ~line 4522.

### KEEP (do not touch) — the realistic setup

```go
repo := t.TempDir()
dcmInitRepo(t, repo)

base := "def get_links():\n    return []\n\ndef sort_items():\n    return []\n"
tStartContent := "def get_links():\n    return fetch_all_links()\n\ndef sort_items():\n    return sorted(links, key=lambda c: c.code)\n"
dcmWriteFile(t, repo, "store.py", base)
dcmStageFile(t, repo, "store.py")
dcmCommitRaw(t, repo, "initial")
dcmWriteFile(t, repo, "store.py", tStartContent)

plannerJSON := `{"count":2,"single":false,"commits":[` +
    `{"title":"feat: add link fetching","description":"the get_links change","files":["store.py"]},` +
    `{"title":"feat: sort listed links by code","description":"the sort_items change","files":["store.py"]}` +
    `]}`
plannerM := dcmPlannerManifest(t, bin, plannerJSON)
messageM := dcmMessageScriptManifest(t, bin, []string{"feat: add link fetching", "feat: sort listed links by code"})
roles := RoleManifests{
    Planner: plannerM,
    Stager:  stubtest.Manifest(bin, stubtest.Options{Out: ""}), // BARE — nil TooledFlags (KEEP BARE)
    Message: messageM,
    Arbiter: stubtest.Manifest(bin, stubtest.Options{Out: ""}),
}
// NOTE: StagerAvailable is the zero-value false here — CORRECT. The stager is genuinely bare; do NOT
// add `StagerAvailable: true` (that would defeat the test + misrepresent S1's sentinel).
deps := Deps{
    Git:     git.New(repo),
    Config:  config.Defaults(),
    Roles:   roles,
    Verbose: ui.NewVerbose(io.Discard, true), // was NewVerbose(&logBuf, true); logBuf no longer asserted
}
deps.Config.Commits = 2
// deps.stager stays NIL — irrelevant now (guard fires before stageConcept), but harmless to leave.
```

Why the setup stays: Decompose must reach `runLoop` (freeze the tree, run the planner, compute
`isFileDisjoint==false`). The guard fires at `runLoop` entry, AFTER that flow but BEFORE any
per-concept staging.

### REPLACE — assertions block

OLD (delete):
```go
result, err := Decompose(context.Background(), deps)
if err != nil {
    t.Fatalf("FR-M12d swallows the stager error into empty-skip; got unexpected err: %v", err)
}
if len(result.Commits) != 0 {
    t.Errorf("Commits len = %d, want 0 (TooledFlags-less stager cannot stage the shared file; both concepts FR-M8-skipped)", len(result.Commits))
}
logStr := logBuf.String()
if !strings.Contains(logStr, "stager failed twice") || !strings.Contains(logStr, "treating concept as empty") {
    t.Errorf("expected FR-M12d 'stager failed twice … treating concept as empty' log ...; got: %s", logStr)
}
```

NEW (the BUG-003 contract):
```go
result, err := Decompose(context.Background(), deps)
if err == nil {
    t.Fatal("BUG-003: a bare/tooled-less stager on a shared partition must hard-error; got nil")
}
if !errors.Is(err, ErrDecomposeFailed) {
    t.Errorf("error not ErrDecomposeFailed-wrapped: %v", err)
}
for _, want := range []string{"tooled stager", "tooled_flags", "disjoint", "pi or claude"} {
    if !strings.Contains(err.Error(), want) {
        t.Errorf("error %q missing %q", err.Error(), want)
    }
}
if len(result.Commits) != 0 {
    t.Errorf("Commits len = %d, want 0 (guard fires before any staging)", len(result.Commits))
}
```

### UPDATE — doc comments

Sub-test header (replace the FR-M12d-swallow framing):
```go
// --- Sub-case: shared partition + BARE stager → BUG-003 hard error (capability gap). ---
//
// A TooledFlags-less provider "simply cannot serve as a stager" (spec §12.7). BUG-003 turns the old
// silent empty-skip into a clear, actionable error: runLoop's StagerAvailable guard fires BEFORE any
// staging (it checks the S1 sentinel, not exec output), so Decompose returns ErrDecomposeFailed naming
// the remedy. The guard is ORTHOGONAL to FR-M12d: FR-M12d's retry-then-empty governs RUNTIME failures
// of a TOOLED stager (StagerAvailable=true), covered separately by TestDecompose_StagerRetryThenEmpty.
// The disjoint sub-case above proves the fast-path bypass; this proves the capability-gap hard error.
```

Case-8 header (~line 4454): reword the claim that the shared case is "swallowed into an empty-skip
for BOTH concepts" → "produces a BUG-003 hard error (capability gap: a tooled-less provider cannot
serve as a stager, spec §12.7)." Keep the `disjoint_succeeds` description intact.

### Imports

- `io` (for `io.Discard`) — check it's imported; if you keep `logBuf` instead, you don't need `io`.
- `errors`, `strings` — already imported.