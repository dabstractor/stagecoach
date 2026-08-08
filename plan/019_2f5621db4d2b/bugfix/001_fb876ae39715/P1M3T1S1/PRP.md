name: "P1.M3.T1.S1 — Regression test: --edit + disjoint fast-path, no cross-contamination (BUG-001)"
description: >
  TEST-ONLY task. Adds the BUG-001 regression test that verifies the P1.M1 fix (ALREADY COMPLETE:
  generateMessageCore in the concurrent goroutine + EditMessage moved to the serial publish loop,
  decompose.go:746 + :806). The test mirrors the proven TestRunLoopFastPath_ConcurrentPublish skeleton
  (decompose_test.go:3150) with 3 deltas: cfg.Edit=true, t.Setenv("GIT_EDITOR","true") (a no-op editor
  that preserves each concept's generated message unchanged), and 2 disjoint concepts (a.go/b.go). The
  concurrency-safe dcmMessageMatchManifest emits DISTINCT input-derived messages per concept
  (a.go→"feat: add a", b.go→"feat: add b") so each concept's tree-to-tree diff deterministically selects
  its OWN message. The load-bearing assertions: len(commits)==2, commits[0].Subject=="feat: add a",
  commits[1].Subject=="feat: add b", commits[0].Subject != commits[1].Subject (no cross-contamination).
  On the FIXED code EditMessage runs SERIALLY (one at a time in the publish loop) so each concept reads
  back its own STAGECOACH_EDITMSG content; on the OLD (buggy) code the shared file race made one concept
  read the other's content. GIT_EDITOR=true keeps the editor a genuine no-op (no stubeditor needed —
  test_strategy.md confirms). NO production-code change, NO docs (contract: "none — test-only file"). Does
  NOT collide with the in-flight BUG-002 dedupe fix (P1.M2.T1.S1) — that adds seenSubjects INSIDE the
  serial loop with NO signature change, and this test uses DISTINCT subjects so the dedupe path never
  triggers; the test passes with OR without the BUG-002 fix. Placement: append to
  internal/decompose/decompose_test.go (package decompose — where the skeleton + all dcm* helpers live) so
  it reuses them without re-declaration. go test + go test -race + make test green.

---

## Goal

**Feature Goal**: Lock in the BUG-001 fix with a deterministic regression test so a future change that
re-introduces concurrent EditMessage on the fast-path (or otherwise breaks the serialization) is caught at
test time. The test proves that `stagecoach --edit` (cfg.Edit=true) on a pairwise-file-disjoint fast-path
decompose run publishes each concept with its OWN commit message — no concept silently receives a
sibling's message via the shared STAGECOACH_EDITMSG race.

**Deliverable** (1 new test function, appended to an existing test file; no new files needed):
- **`internal/decompose/decompose_test.go`** — `TestRunLoopFastPath_EditGate_NoCrossContamination`:
  mirrors `TestRunLoopFastPath_ConcurrentPublish` (the proven fast-path test skeleton) with cfg.Edit=true,
  GIT_EDITOR=true, and 2 disjoint concepts; asserts each commit's subject matches its own concept's
  generated message and the two subjects differ.

**Success Definition**:
- The test PASSES against current HEAD (the P1.M1 fix is complete: EditMessage serial in the publish loop).
- `len(commits) == 2`; `commits[0].Subject == "feat: add a"`; `commits[1].Subject == "feat: add b"`;
  `commits[0].Subject != commits[1].Subject`.
- `runLoopFastPath` returns `err == nil`; HEAD advanced to `commits[1].SHA`; each commit parents in CAS
  order (commit[0]→preRunHEAD, commit[1]→commits[0].SHA) — the standard fast-path invariants, mirroring the
  skeleton.
- `go test ./internal/decompose/ -run 'TestRunLoopFastPath_EditGate_NoCrossContamination' -v` GREEN.
- `go test -race ./internal/decompose/...` GREEN (the test exercises concurrent message generation).
- `make test` + `make lint` GREEN; `gofmt -l` clean.
- NO production-code change; NO docs; NO new file (appended to decompose_test.go); NO collision with the
  in-flight BUG-002 dedupe fix (P1.M2.T1.S1).

## User Persona (if applicable)

**Target User**: The maintainer/contributor who needs confidence the BUG-001 fix (concurrent editor race on
the fast-path) won't silently regress.

**Use Case**: A future refactor of `runLoopFastPath` that accidentally moves EditMessage back into the
concurrent generateMessage goroutine (or otherwise re-opens the shared-file race) trips this test in CI
before shipping — instead of a user silently getting a commit whose message describes a different commit's diff.

**User Journey**: contributor edits `runLoopFastPath` → `go test ./internal/decompose/ -run EditGate` → the
"NoCrossContamination" test fails ("commits[1].Subject = \"feat: add a\", want \"feat: add b\"") → the
regression is caught at test time, not in a user's mangled commit history.

**Pain Points Addressed**: BUG-001 had NO regression test — the P1.M1 fix could have been silently
regressed. This test closes that gap with a deterministic (on the fixed code) correctness assertion.

## Why

- **BUG-001 (Critical) / FR-E4**: the fix (P1.M1) moved EditMessage out of the concurrent goroutine and
  into the serial publish loop (decompose.go:806), so editing is serialized (FR-E4's "serialized" word is
  load-bearing). Without a test, a future change could re-introduce the shared-STAGECOACH_EDITMSG race and
  silently attach the wrong message to a commit — exactly the silent correctness violation BUG-001 was.
  This test is the regression net.
- **Mirrors a proven skeleton**: `TestRunLoopFastPath_ConcurrentPublish` already exercises the fast-path's
  repo setup + freeze + concurrent generation + CAS-order assertions. This test reuses that exact shape
  (and the dcm* helpers), adding only the --edit dimension. No new test infrastructure.
- **Bounded, no-conflict scope**: one test function appended to one test file. The fix is COMPLETE; the
  in-flight BUG-002 sibling (P1.M2.T1.S1) adds dedupe with no signature change and this test's distinct
  subjects never trigger it — zero overlap.

## What

**User-visible behavior**: None (test-only).

**Technical change**: one test function appended to `internal/decompose/decompose_test.go`. See the
Implementation Blueprint for the verbatim test body + exact anchors.

### Success Criteria
- [ ] `TestRunLoopFastPath_EditGate_NoCrossContamination` exists in `internal/decompose/decompose_test.go`.
- [ ] It sets `cfg.Edit = true` and `t.Setenv("GIT_EDITOR", "true")` (no-op editor).
- [ ] It builds a 2-concept disjoint partition (a.go, b.go) over a BORN repo with distinct dirty changes.
- [ ] It uses `dcmMessageMatchManifest` with `{substr:"a.go", msg:"feat: add a"}` + `{substr:"b.go", msg:"feat: add b"}`.
- [ ] It calls `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly.
- [ ] It asserts: `err == nil`; `len(commits) == 2`; `commits[0].Subject == "feat: add a"`;
      `commits[1].Subject == "feat: add b"`; `commits[0].Subject != commits[1].Subject`.
- [ ] `go test ./internal/decompose/ -run 'TestRunLoopFastPath_EditGate_NoCrossContamination' -v` GREEN.
- [ ] `go test -race ./internal/decompose/...` + `make test` + `make lint` GREEN; `gofmt -l` clean.
- [ ] NO production-code change; NO docs; NO new file (appended to decompose_test.go).

## All Needed Context

### Context Completeness Check

_If someone knew nothing about this codebase, would they have everything needed to implement this successfully?_
**Yes** — the verbatim skeleton to mirror (TestRunLoopFastPath_ConcurrentPublish, with line-precise repo
setup + freeze + runLoopFastPath call), the exact 3 deltas (cfg.Edit=true, GIT_EDITOR=true, 2 concepts),
the full dcm* helper inventory (dcmInitRepo/dcmWriteFile/dcmRunGit/dcmCommitRaw/dcmGitOut/dcmHeadSHA/
dcmMessageMatchManifest/messageMatchRule — all in-package, reuse don't redeclare), the GIT_EDITOR=true
mechanism (git var GIT_EDITOR → /bin/true no-op → EditMessage reads back what it wrote) + why it catches
the bug, the architect's exact assertions (test_strategy.md "BUG-001 Regression Test"), the CommitResult
fields (.Subject/.SHA), the FROZEN runLoopFastPath signature (BUG-002 in-flight does NOT change it), and
the scope fence (no production change, no BUG-002 collision).

### Documentation & References

```yaml
# MUST READ — the authoritative findings (skeleton + helpers + GIT_EDITOR recipe + scope)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M3T1S1/research/findings.md
  why: "§1 the fix being guarded (P1.M1 COMPLETE — EditMessage serial at decompose.go:806); §2 the verbatim
        skeleton (TestRunLoopFastPath_ConcurrentPublish); §3 the full dcm* helper inventory; §4 the cfg.Edit
        + GIT_EDITOR=true recipe + WHY it catches the bug + the determinism note; §5 the FROZEN
        runLoopFastPath signature + CommitResult fields + BUG-002 non-collision; §6 the architect's exact
        assertions; §7 name/placement/scope."
  critical: "§4: t.Setenv(\"GIT_EDITOR\", \"true\") is a NO-OP editor (NOT the stubeditor) — it preserves each
             concept's message unchanged BECAUSE the fix serializes EditMessage. §5: the test is compatible
             with OR without the in-flight BUG-002 fix (distinct subjects ⇒ no dedupe trigger). §6: the
             cross-contamination assertion (commits[0].Subject != commits[1].Subject) is the load-bearing one."

# MUST READ — the architect's exact BUG-001 regression-test recipe
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/architecture/test_strategy.md
  section: "## BUG-001 Regression Test"
  why: "Specifies the Goal, Setup (2+ disjoint concepts, dcmMessageMatchManifest distinct msgs, GIT_EDITOR=true,
        cfg.Edit=true), Assertions (len>=2, commits[0].Subject=='feat: add a', commits[1].Subject=='feat: add b',
        !=), 'Why this catches the bug', and the determinism note (verifies FIXED correctness, not bug repro).
        Also documents the cfg.Edit recipe + the GIT_EDITOR=true no-op-editor choice (vs the stubeditor)."
  critical: "Follow this recipe EXACTLY. The 'Why this catches the bug' paragraph is the rationale for
             GIT_EDITOR=true (last writer's content is what both read on old code; serial on fixed code)."

# MUST EDIT — the test file (the skeleton + all dcm* helpers live here)
- file: internal/decompose/decompose_test.go   # append the new test at the end (or near the other RunLoopFastPath tests)
  why: "TestRunLoopFastPath_ConcurrentPublish (:3150) is the verbatim skeleton; the dcm* helpers (:30-190) are
        all in this file (package decompose). CommitResult.Subject/SHA are the assertion fields. Appending here
        means the new test reuses the helpers WITHOUT re-declaring them."
  pattern: "Mirror TestRunLoopFastPath_ConcurrentPublish's repo setup (dcmInitRepo + dcmWriteFile + dcmRunGit
            add + dcmCommitRaw 'initial' + disjoint dirty re-writes), the g.FreezeWorkingTree(ctx, baseTree)
            + preRunHEAD capture, the Deps{Git, Config, Roles, Verbose} construction, and the runLoopFastPath
            direct call — then assert subjects."
  gotcha: "The skeleton constructs Deps directly (Deps{Git: g, Config: ..., Roles: roles, Verbose: ...}) reusing
           the single g := git.New(repo) for BOTH FreezeWorkingTree and Deps. Mirror that (don't call
           dcmDepsWithConfig, which builds a separate git.New — harmless but diverges from the skeleton). Set
           Config: cfg (the cfg.Edit=true one), NOT config.Defaults()."

# MUST READ — the skeleton itself (clone its shape precisely)
- file: internal/decompose/decompose_test.go   # TestRunLoopFastPath_ConcurrentPublish :3150-3268
  why: "The exact repo-freeze-call-assert flow to copy. Note: it seeds 3 files (a.go/b.go/c.go); this test uses
        2 (a.go/b.go) per the contract. It sets STAGECOACH_STUB_SLEEP_MS=150 to make concurrency observable —
        OMIT that here (this test is about correctness, not concurrency timing)."

# CONTEXT — the fix being guarded (READ-ONLY; P1.M1 COMPLETE)
- file: internal/decompose/decompose.go   # runLoopFastPath :680; generateMessageCore call :746; serial EditMessage :806
  why: "Confirms the fix LANDED: the goroutine calls generateMessageCore (no editor) at :746, and EditMessage is
        applied in the serial publish loop at :806 before publishCommit at :821. This serialization is what the
        test verifies. runLoopFastPath signature (:680) is FROZEN."
  critical: "Do NOT edit decompose.go — the fix is complete; this task is test-only."

# CONTEXT — EditMessage + GIT_EDITOR resolution (confirms the no-op-editor mechanism)
- file: internal/generate/finalize.go   # EditMessage :67; editor resolution :95 (git var GIT_EDITOR → VISUAL → EDITOR → vi)
  why: "EditMessage writes <gitDir>/STAGECOACH_EDITMSG, resolves the editor via `git var GIT_EDITOR`, runs it,
        reads the file back. With GIT_EDITOR=true, git resolves the editor to `true` ⇒ sh -c 'true <editmsg>'
        ⇒ /bin/true exits 0 without touching the file ⇒ EditMessage reads back what it wrote. NO stubeditor needed."
  critical: "READ-ONLY. Do NOT edit finalize.go. The mechanism is why t.Setenv(\"GIT_EDITOR\", \"true\") works."

# CONTEXT — the in-flight sibling (NO collision; READ-ONLY)
- docfile: plan/019_2f5621db4d2b/bugfix/001_fb876ae39715/P1M2T1S1/PRP.md
  why: "BUG-002 (cross-concept duplicate-subject) fix adds seenSubjects dedupe INSIDE runLoopFastPath's serial
        publish loop, AFTER the res.err block and BEFORE EditMessage, with NO signature change ('FROZEN
        signatures'). This test's DISTINCT subjects (a.go/b.go) never trigger the dedupe/re-generation path, so
        the test passes with OR without that fix. Confirms zero overlap."
  critical: "Do NOT add a same-subject collision case to THIS test — that's the BUG-002 regression test
             (P1.M3.T2.S1), a separate sibling. This test is purely BUG-001 (concurrent editor race)."
```

### Current Codebase tree (relevant slice)

```bash
internal/decompose/
  decompose.go        # READ-ONLY — runLoopFastPath (:680) + the LANDED fix (generateMessageCore :746, serial EditMessage :806)
  decompose_test.go   # EDIT (append) — +TestRunLoopFastPath_EditGate_NoCrossContamination; skeleton (:3150) + dcm* helpers (:30-190) live here
  (other *_test.go)   # READ-ONLY — regression net (make test)
internal/generate/
  finalize.go         # READ-ONLY — EditMessage (:67) + GIT_EDITOR resolution (:95); confirms the no-op-editor mechanism
# go.mod / Makefile — READ-ONLY (no new dep; test=line70 -race; lint=line103)
```

### Desired Codebase tree with files to be added/modified

```bash
# MODIFIED (the ONLY file this task touches):
internal/decompose/decompose_test.go   # +TestRunLoopFastPath_EditGate_NoCrossContamination (appended; reuses in-package dcm* helpers)
# (NO new files. NO production-code change. NO docs.)
```

### Known Gotchas of our codebase & Library Quirks

```go
// CRITICAL (GIT_EDITOR=true is a NO-OP editor, NOT the stubeditor): the contract + test_strategy.md specify
// t.Setenv("GIT_EDITOR", "true"). `git var GIT_EDITOR` resolves the env var → outputs "true" → EditMessage runs
// sh -c "true <editmsg>" → /bin/true exits 0 WITHOUT modifying the file → EditMessage reads back what it wrote
// (the concept's own generated message). Do NOT use stubtest.BuildEditor / SetEditorEnv here — that's the
// generate_test.go:836 recipe for CHANGING the message; this test wants the message PRESERVED (to detect
// cross-contamination). The no-op editor + the serialized (fixed) EditMessage ⇒ each concept reads its own msg.

// CRITICAL (the test verifies FIXED correctness, not bug reproduction): on the OLD code the shared-file race
// was non-deterministic (last writer wins), so the test might or might not fail without the fix. On the FIXED
// code there IS no race, so the test is deterministic. Do NOT add sleeps/timing tricks to "force" the bug — the
// architect's determinism note (test_strategy.md) explicitly scopes this as a correctness test of the fixed
// behavior. (The STAGECOACH_STUB_SLEEP_MS=150 from the ConcurrentPublish skeleton is OMITTED — it's for timing,
// not correctness.)

// CRITICAL (construct Deps directly reusing one g := git.New(repo), mirroring the skeleton): the skeleton does
// g := git.New(repo); ...; g.FreezeWorkingTree(ctx, baseTree); ...; Deps{Git: g, Config: ..., Roles: roles, ...}.
// Do NOT call dcmDepsWithConfig (it builds a separate git.New and sets Verbose: nil). Reuse g for both the freeze
// AND Deps; set Config: cfg (the cfg.Edit=true variant), Verbose: ui.NewVerbose(&logBuf, true) for debug output.

// CRITICAL (2 concepts, NOT 3): the contract + test_strategy.md say "2+ disjoint concepts". Use 2 (a.go, b.go).
// The skeleton uses 3 — do not copy the third. The assertions index commits[0] and commits[1] only.

// GOTCHA (messageMatchRule fields are unexported but the test is package decompose): messageMatchRule{substr, msg
// string; sleepMs int} has unexported fields, but decompose_test.go is `package decompose` ⇒ the literal
// messageMatchRule{substr: "a.go", msg: "feat: add a"} compiles. Use the 2-field form (omit sleepMs).

// GOTCHA (dcmCommitRaw uses --allow-empty but commits staged content too): dcmRunGit(t,repo,"add",...) THEN
// dcmCommitRaw(t,repo,"initial") commits the staged base files (the --allow-empty just also permits empties).
// So the BORN repo's base commit holds a.go+b.go v1 — exactly what the disjoint dirty re-writes (v2) build on.

// GOTCHA (no BUG-002 collision): this test's distinct subjects (a.go→"feat: add a", b.go→"feat: add b") never
// trigger the in-flight BUG-002 seenSubjects dedupe (P1.M2.T1.S1). Do NOT add a same-subject case here — that is
// the separate BUG-002 regression test (P1.M3.T2.S1). Mixing them would couple two independent bug fixes.

// GOTCHA (FreezeWorkingTree resets the index to baseTree): after g.FreezeWorkingTree(ctx, baseTree), the index is
// back at baseTree (clean) and the working-tree changes are "in T_start". Each concept's `git add <its file>`
// (done INSIDE runLoopFastPath's sweep) re-stages its slice of T_start. The test does NOT stage anything itself
// after the freeze — runLoopFastPath owns the per-concept staging. (Mirror the skeleton exactly.)
```

## Implementation Blueprint

### Data models and structure

None. No new types, helpers, or fixtures. One new test function that reuses the in-package `dcm*` helpers +
the `messageMatchRule` type + `prompt.PlannerCommit` + `CommitResult` (all existing).

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: EDIT internal/decompose/decompose_test.go — APPEND TestRunLoopFastPath_EditGate_NoCrossContamination
  - PLACE: append at the end of the file (or immediately after TestRunLoopFastPath_ConcurrentPublish for
    cohesion). Package decompose (internal test) — reuses the dcm* helpers + messageMatchRule without import.
  - IMPORTS: none new (bytes, context, testing, time, config, git, prompt, stubtest, ui are already imported
    at the top of decompose_test.go — verify before relying; the skeleton uses all of them).
  - BODY (mirror TestRunLoopFastPath_ConcurrentPublish with the 3 deltas; verbatim shape):
      func TestRunLoopFastPath_EditGate_NoCrossContamination(t *testing.T) {
          bin := stubtest.Build(t)
          repo := t.TempDir()
          dcmInitRepo(t, repo)
          // Base commit with 2 files (BORN repo → baseTree = HEAD^{tree}).
          dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 1\n")
          dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 1\n")
          dcmRunGit(t, repo, "add", "a.go", "b.go")
          dcmCommitRaw(t, repo, "initial")
          // Disjoint dirty change set: modify each file independently.
          dcmWriteFile(t, repo, "a.go", "package a\n\nvar A = 2\n")
          dcmWriteFile(t, repo, "b.go", "package b\n\nvar B = 2\n")

          // BUG-001 delta 1: --edit on. GIT_EDITOR=true = NO-OP editor (preserves each concept's msg unchanged).
          t.Setenv("GIT_EDITOR", "true")
          cfg := config.Defaults()
          cfg.Edit = true   // BUG-001 delta 2

          g := git.New(repo)
          ctx := context.Background()
          baseTree := dcmGitOut(t, repo, "rev-parse", "HEAD^{tree}")
          tStart, err := g.FreezeWorkingTree(ctx, baseTree)
          if err != nil { t.Fatalf("FreezeWorkingTree: %v", err) }
          preRunHEAD := dcmHeadSHA(t, repo)

          // Concurrency-safe, input-derived message stub: each concept's diff names a distinct file ⇒ its OWN msg.
          messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
              {substr: "a.go", msg: "feat: add a"},
              {substr: "b.go", msg: "feat: add b"},
          })
          roles := RoleManifests{Message: messageM}
          var logBuf bytes.Buffer
          deps := Deps{Git: g, Config: cfg, Roles: roles, Verbose: ui.NewVerbose(&logBuf, true)}

          concepts := []prompt.PlannerCommit{   // BUG-001 delta 3: 2 disjoint concepts
              {Title: "c1", Files: []string{"a.go"}},
              {Title: "c2", Files: []string{"b.go"}},
          }

          commits, _, err := runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)
          if err != nil {
              t.Fatalf("runLoopFastPath: %v\nverbose:\n%s", err, logBuf.String())
          }

          // BUG-001 regression: each concept gets its OWN message (no shared-STAGECOACH_EDITMSG contamination).
          if len(commits) != 2 {
              t.Fatalf("Commits len = %d, want 2\nverbose:\n%s", len(commits), logBuf.String())
          }
          if commits[0].Subject != "feat: add a" {
              t.Errorf("commits[0].Subject = %q, want %q (concept 0's own message — BUG-001 cross-contamination?)", commits[0].Subject, "feat: add a")
          }
          if commits[1].Subject != "feat: add b" {
              t.Errorf("commits[1].Subject = %q, want %q (concept 1's own message — BUG-001 cross-contamination?)", commits[1].Subject, "feat: add b")
          }
          if commits[0].Subject == commits[1].Subject {
              t.Errorf("cross-contamination: both commits share subject %q (BUG-001 — EditMessage not serialized)", commits[0].Subject)
          }

          // CAS-order sanity (mirrors the skeleton's hard gate; cheap + catches structural breakage too).
          if parent := dcmGitOut(t, repo, "rev-parse", commits[0].SHA+"^"); parent != preRunHEAD {
              t.Errorf("commit[0] parent = %s, want %s (preRunHEAD)", parent, preRunHEAD)
          }
          if parent := dcmGitOut(t, repo, "rev-parse", commits[1].SHA+"^"); parent != commits[0].SHA {
              t.Errorf("commit[1] parent = %s, want %s (commit[0].SHA — CAS order)", parent, commits[0].SHA)
          }
          if head := dcmHeadSHA(t, repo); head != commits[1].SHA {
              t.Errorf("HEAD = %s, want %s (last commit)", head, commits[1].SHA)
          }
      }
  - FOLLOW pattern: TestRunLoopFastPath_ConcurrentPublish (:3150) — same setup/freeze/call/assert shape.
  - NAMING: TestRunLoopFastPath_EditGate_NoCrossContamination (descriptive; the contract's "or similar").
  - GOTCHA: reuse the single `g := git.New(repo)` for FreezeWorkingTree AND Deps (mirror the skeleton; do NOT
    use dcmDepsWithConfig). Config: cfg (the cfg.Edit=true one). Verbose: ui.NewVerbose(&logBuf, true) for debug.
  - GOTCHA: OMIT the skeleton's `messageM.Env["STAGECOACH_STUB_SLEEP_MS"] = "150"` — this test is about
    correctness, not concurrency timing.

Task 2: VERIFY — focused + race + full tests, format, lint, grep guards
  - go test ./internal/decompose/ -run 'TestRunLoopFastPath_EditGate_NoCrossContamination' -v
  - go test -race ./internal/decompose/...   # race detector — concurrent generation is exercised
  - make test                                # full suite; the new test is additive
  - gofmt -l internal/decompose/decompose_test.go   # empty
  - make lint
  - grep guards (see Validation Loop Level 4)
```

### Implementation Patterns & Key Details

```go
// PATTERN: the BUG-001 regression assertion (the load-bearing check — each concept gets its OWN subject).
if commits[0].Subject != "feat: add a" { t.Errorf("... concept 0's own message — BUG-001 ...") }
if commits[1].Subject != "feat: add b" { t.Errorf("... concept 1's own message — BUG-001 ...") }
if commits[0].Subject == commits[1].Subject {
    t.Errorf("cross-contamination: both commits share subject %q (BUG-001 — EditMessage not serialized)", commits[0].Subject)
}

// PATTERN: the no-op editor (GIT_EDITOR=true) — preserves each concept's message; the FIXED serial EditMessage
// then guarantees each concept reads back its own STAGECOACH_EDITMSG content.
t.Setenv("GIT_EDITOR", "true")
cfg := config.Defaults()
cfg.Edit = true

// PATTERN: the concurrency-safe, input-derived message stub (each concept's diff names a distinct file).
messageM := dcmMessageMatchManifest(t, bin, []messageMatchRule{
    {substr: "a.go", msg: "feat: add a"},
    {substr: "b.go", msg: "feat: add b"},
})
```

### Integration Points

```yaml
TEST FILE (internal/decompose/decompose_test.go):
  - APPEND TestRunLoopFastPath_EditGate_NoCrossContamination (reuses in-package dcm* helpers + messageMatchRule).

NO database / migration / routes / new types / new imports / production-code change / docs change.

DEPENDENCY (the fix this test guards — TREAT AS LANDED; P1.M1 COMPLETE):
  - runLoopFastPath must call generateMessageCore in the goroutine (decompose.go:746, NO editor) and apply
    EditMessage in the SERIAL publish loop (decompose.go:806) before publishCommit. WITHOUT that serialization,
    the shared STAGECOACH_EDITMSG race could cross-contaminate (non-deterministically on old code). WITH it,
    the test is deterministic and passes.

SCOPE FENCES: NO production-code change (decompose.go/finalize.go/message.go are READ-ONLY); NO new file
  (appended to decompose_test.go); NO docs (contract: "none — test-only file"); NO BUG-002 logic (distinct
  subjects ⇒ no dedupe trigger; the BUG-002 regression test is the separate P1.M3.T2.S1 sibling); NO same-subject
  collision case (that's BUG-002); NO stubeditor (GIT_EDITOR=true is the no-op editor per the contract).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
# Build + vet (the test must compile — it reuses in-package helpers; no new import expected).
go build ./...
go vet ./internal/decompose/...
# Expected: clean. A vet/build error means a typo, a wrong field name (CommitResult.Subject vs Message), or a
#           helper misuse — there shouldn't be one (the skeleton uses the same helpers).

# Format.
gofmt -l internal/decompose/decompose_test.go
# Expected: empty. If listed: gofmt -w the file.

# Lint.
make lint      # golangci-lint
# Expected: zero errors. The new test reads all its locals; no unused.

# Scope guard: ONLY decompose_test.go changed (test-only; no production change).
git status --short
# Expected: M internal/decompose/decompose_test.go  (only ONE file modified). NO decompose.go / finalize.go.
```

### Level 2: Unit Tests (Component Validation) — THE PRIMARY GATE

```bash
# The new regression test (focused).
go test ./internal/decompose/ -run 'TestRunLoopFastPath_EditGate_NoCrossContamination' -v
# Expected: PASS — len(commits)==2; commits[0].Subject=="feat: add a"; commits[1].Subject=="feat: add b";
#           subjects differ; CAS order; HEAD == commits[1].SHA. (The P1.M1 fix serializes EditMessage ⇒ each
#           concept reads its own STAGECOACH_EDITMSG content.)

# Race detector (the test exercises concurrent message generation via runLoopFastPath's goroutine fan-out).
go test -race ./internal/decompose/...
# Expected: green. -race must not report a data race (the fixed EditMessage is serial; generateMessageCore is
#           concurrency-safe by design — read-only tree reads).

# Full race suite.
make test
# Expected: green. The new test is additive; the existing TestRunLoopFastPath_ConcurrentPublish + all other
#           decompose tests pass unchanged.
```

### Level 3: Integration Testing (System Validation)

```bash
# Not applicable — this is a unit-test-only task. The "integration" is `go test ./internal/decompose/ -run
# EditGate -v` (Level 2), which exercises the REAL runLoopFastPath → generateMessageCore → serial EditMessage
# → publishCommit path end-to-end against a real temp git repo + a compiled stub agent binary. A full CLI e2e
# (`stagecoach --edit` on a real dirty disjoint tree) is covered by the §20.5 harness / manual smoke, not this unit test.
```

### Level 4: Creative & Domain-Specific Validation (grep guards)

```bash
# Guard 1: the test exists + is named per the contract.
grep -n 'func TestRunLoopFastPath_EditGate_NoCrossContamination' internal/decompose/decompose_test.go
# Expected: 1 hit.

# Guard 2: the test sets cfg.Edit=true AND GIT_EDITOR=true (the 2 BUG-001 deltas).
grep -n 'cfg.Edit = true' internal/decompose/decompose_test.go | grep -A0 'EditGate\|true' ; \
grep -c 't.Setenv("GIT_EDITOR", "true")' internal/decompose/decompose_test.go
# Expected: cfg.Edit=true present in the new test; GIT_EDITOR=true ≥1 hit. (Confirm both are INSIDE the new test.)

# Guard 3: the no-cross-contamination assertions are present (the load-bearing checks).
grep -cE 'commits\[0\]\.Subject != "feat: add a"|commits\[1\]\.Subject != "feat: add b"|commits\[0\]\.Subject == commits\[1\]\.Subject' internal/decompose/decompose_test.go
# Expected: ≥3 hits in the new test (the two want-subject checks + the cross-contamination guard).

# Guard 4: NO stubeditor used (the contract specifies GIT_EDITOR=true, not stubtest.BuildEditor).
grep -n 'BuildEditor\|SetEditorEnv' internal/decompose/decompose_test.go
# Expected: ZERO hits in the new test (the stubeditor is the generate_test.go:836 recipe for CHANGING the msg;
#           this test PRESERVES the msg via the no-op editor). [If pre-existing BuildEditor uses exist elsewhere
#           in the file, confirm the new test has none.]

# Guard 5: NO production-code change (the fix P1.M1 is COMPLETE; this task is test-only).
git diff --name-only | grep -v '_test.go'
# Expected: EMPTY (no production file modified). decompose.go / finalize.go / message.go must NOT appear.

# Guard 6: the test uses the concurrency-safe message stub (dcmMessageMatchManifest), not the racing script stub.
grep -c 'dcmMessageMatchManifest' internal/decompose/decompose_test.go
# Expected: ≥2 hits (the pre-existing ConcurrentPublish test + the new one). Confirms the new test doesn't use
#           dcmMessageScriptManifest (which races on its file-backed counter under concurrency).

# Guard 7: scope — only one file changed.
git status --porcelain
# Expected: M internal/decompose/decompose_test.go (only). No new files, no other modifications.

# Regression: the existing fast-path + skeleton tests still pass (the new test is additive).
go test ./internal/decompose/ -run 'TestRunLoopFastPath' -v
# Expected: all PASS (ConcurrentPublish + the new EditGate test + any other RunLoopFastPath tests).
```

## Final Validation Checklist

### Technical Validation
- [ ] `go build ./...` + `go vet ./internal/decompose/...` clean
- [ ] `gofmt -l internal/decompose/decompose_test.go` empty
- [ ] `make lint` zero errors
- [ ] `go test -race ./internal/decompose/...` green (race detector; concurrent generation exercised)
- [ ] `make test` (full suite) green

### Feature Validation
- [ ] `TestRunLoopFastPath_EditGate_NoCrossContamination` exists in decompose_test.go
- [ ] Sets `cfg.Edit = true` + `t.Setenv("GIT_EDITOR", "true")` (no-op editor)
- [ ] 2 disjoint concepts (a.go, b.go) over a BORN repo; dcmMessageMatchManifest distinct msgs
- [ ] Calls `runLoopFastPath(ctx, deps, concepts, baseTree, tStart, preRunHEAD, false)` directly
- [ ] Asserts `len(commits)==2`, `commits[0].Subject=="feat: add a"`, `commits[1].Subject=="feat: add b"`,
      `commits[0].Subject != commits[1].Subject` (the no-cross-contamination gate)
- [ ] CAS-order + HEAD sanity (mirrors the skeleton's hard gates)

### Scope-Boundary Validation
- [ ] `git status` shows ONLY `internal/decompose/decompose_test.go` modified (1 file; test-only)
- [ ] NO production-code change (decompose.go / finalize.go / message.go untouched — the P1.M1 fix is complete)
- [ ] NO new file (appended to decompose_test.go); NO new import; NO docs
- [ ] NO BUG-002 collision (distinct subjects ⇒ no dedupe trigger; no same-subject case — that's P1.M3.T2.S1)
- [ ] NO stubeditor (GIT_EDITOR=true is the no-op editor per the contract); NO STAGECOACH_STUB_SLEEP_MS timing trick
- [ ] Grep guards 1–7 (Level 4) all pass

### Code Quality & Docs
- [ ] Mirrors the TestRunLoopFastPath_ConcurrentPublish skeleton (repo setup + freeze + Deps + call + assert)
- [ ] Reuses the in-package dcm* helpers + messageMatchRule (no re-declaration)
- [ ] Error messages cite BUG-001 (so a failure points at the right bug)
- [ ] Verbose buffer wired into Deps for failure diagnostics

---

## Anti-Patterns to Avoid

- ❌ Don't use the stubeditor (`stubtest.BuildEditor` / `SetEditorEnv`). That's the generate_test.go:836 recipe
  for CHANGING the message (it writes STAGECOACH_EDITOR_MSG over the file). This test wants the message
  PRESERVED so it can detect cross-contamination. Use `t.Setenv("GIT_EDITOR", "true")` — the no-op editor
  (test_strategy.md: "For a NO-OP editor (preserve input): t.Setenv(\"GIT_EDITOR\", \"true\")"). Grep guard 4.
- ❌ Don't use `dcmMessageScriptManifest` (the script stub) for the messages. Its file-backed counter RACES
  across N concurrent stub processes (exactly the bug P1.M1.T1.S3's dcmMessageMatchManifest was built to fix).
  Use `dcmMessageMatchManifest` — it's INPUT-DERIVED (each process inspects its own stdin), so a concept's
  message is deterministic regardless of goroutine scheduling. Grep guard 6.
- ❌ Don't add a same-subject collision case to this test. That's the BUG-002 regression test (P1.M3.T2.S1), a
  SEPARATE sibling. Mixing the two couples independent bug fixes and muddies which failure means which bug.
  This test uses DISTINCT subjects (a.go/b.go) so it isolates BUG-001 (the editor race) cleanly.
- ❌ Don't edit any production file (decompose.go / finalize.go / message.go). The BUG-001 fix (P1.M1) is
  COMPLETE. This task is TEST-ONLY. Grep guard 5 enforces it (no non-_test.go file in the diff).
- ❌ Don't add `STAGECOACH_STUB_SLEEP_MS` / timing tricks to "force" the bug to reproduce. The architect's
  determinism note (test_strategy.md) explicitly scopes this as a FIXED-correctness test: on the fixed code
  there IS no race, so the test is naturally deterministic; on old code the race was non-deterministic anyway.
  The sleep is for the ConcurrentPublish skeleton's concurrency-timing SOFT gate, which this test omits.
- ❌ Don't use 3 concepts. The contract + test_strategy.md say "2+". Use exactly 2 (a.go, b.go) so the
  assertions index commits[0]/commits[1] only and the test stays focused on the cross-contamination case.
- ❌ Don't call `dcmDepsWithConfig` — it builds a separate `git.New(repo)` and sets Verbose: nil. Mirror the
  skeleton: construct `Deps{Git: g, Config: cfg, Roles: roles, Verbose: ui.NewVerbose(&logBuf, true)}`
  directly, reusing the single `g := git.New(repo)` (used for FreezeWorkingTree too) and wiring a Verbose
  buffer for failure diagnostics.
- ❌ Don't set `Config: config.Defaults()` — that has Edit=false, so EditMessage is never called and the test
  would pass trivially WITHOUT exercising the BUG-001 path. Set `Config: cfg` where `cfg.Edit = true`.
- ❌ Don't forget `t.Setenv` (not `os.Setenv`). `t.Setenv` restores the env on test cleanup (parallel-test
  safety); `os.Setenv` leaks GIT_EDITOR=true into sibling tests in the same process. The skeleton/contract use
  t.Setenv.
- ❌ Don't stage anything after `g.FreezeWorkingTree`. FreezeWorkingTree captures T_start AND resets the index
  to baseTree; runLoopFastPath's own sweep does the per-concept `git add`. Staging in the test would corrupt
  the freeze baseline. (Mirror the skeleton exactly — no `dcmRunGit add` after the freeze.)
- ❌ Don't assert on `commits[i].Message` (the full message) when `commits[i].Subject` is the contract's
  specified field. The cross-contamination symptom is a wrong SUBJECT (the skeleton + test_strategy.md both
  assert `.Subject`). `.Message` would also work but the contract is explicit about Subject.
- ❌ Don't redeclare the dcm* helpers or messageMatchRule. They're in-package (decompose_test.go, package
  decompose). Reusing them is the whole point of appending to this file. Re-declaring causes a compile error
  (duplicate declaration) or, if names differ, needless duplication.

---

## Confidence Score: 9/10

This is a test-only task with a proven skeleton to mirror (TestRunLoopFastPath_ConcurrentPublish, line-precise),
the full in-package dcm* helper inventory, the architect's exact recipe + assertions (test_strategy.md "BUG-001
Regression Test"), the confirmed GIT_EDITOR=true no-op-editor mechanism (and why it catches the bug), the
FROZEN runLoopFastPath signature + CommitResult fields, the confirmed P1.M1 fix landing (generateMessageCore at
:746, serial EditMessage at :806), and the explicit scope fences (no production change, no BUG-002 collision, no
stubeditor, no timing tricks). The verbatim test body is spelled out. The -1 from 10/10 reflects the one residual
non-determinism the architect flagged: on the OLD (pre-fix) code the shared-file race was non-deterministic, so
this test is a FIXED-correctness test, not a guaranteed-fails-without-the-fix reproduction — but that is by design
(test_strategy.md's determinism note), and the test still serves its purpose (catch a regression of the
serialization). No new dep, no new file, no production change, no docs.