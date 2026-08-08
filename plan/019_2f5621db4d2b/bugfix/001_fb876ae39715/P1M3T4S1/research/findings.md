# P1.M3.T4.S1 — Research Findings: Changeset-Level Documentation Sync (BUG-001/BUG-002)

> DOC-ONLY review task ([Mode B]). Goal: sweep cross-cutting docs for STALE claims about the file-disjoint
> fast-path that the BUG-001/BUG-002 fixes now contradict. Per the item's RESEARCH NOTE, the spec already
> stated the correct behavior (FR-E4 serialized editing; US7/FR32 no duplicate subjects); the fixes bring
> the CODE into conformance, so docs that describe the SPEC's intended behavior should already be correct.
> If they are, this is a no-op (confirm + close). This file records the per-file verdict + the ONE stale
> claim found.

## §0 The contract (what "corrected behavior" means — the fixes that LANDED)

- **BUG-001 fix (P1.M1.T1.S1 + P1.M1.T2.S1/S2)**: `generateMessage` was split into `generateMessageCore`
  (the concurrent-safe core: tree-to-tree diff via read-only tree reads + message-agent + per-concept
  dedupe vs pre-run history) + a thin `generateMessage` WRAPPER that additionally applies `EditMessage`.
  The fast-path launch goroutine now calls `generateMessageCore` ONLY (no editor); `EditMessage` is
  deferred to the SERIAL publish loop (one editor at a time, in CAS order) — restoring **FR-E4**
  ("`--edit` gates EACH commit's message before its ALREADY SERIALIZED publication").
- **BUG-002 fix (P1.M2.T1.S1)**: an incremental `seenSubjects` cross-concept dedupe check runs in the
  serial publish loop (each concept's subject judged against pre-run history + already-decided siblings,
  BEFORE publish), restoring **US7/FR32/FR33** (no duplicate subjects).
- Verified in code:
  - `internal/decompose/message.go:80`  `func generateMessageCore(...)` — the core (NO EditMessage).
  - `internal/decompose/message.go:249` `func generateMessage(...)` — wrapper: `generateMessageCore(nil)`
    (line 250) THEN `generate.EditMessage(...)` (line 259). ⇒ `generateMessage` INCLUDES EditMessage.
  - `internal/decompose/decompose.go:761`  fast-path goroutine calls `generateMessageCore` (NOT generateMessage).
  - `internal/decompose/decompose.go:886`  fast-path serial loop calls `generate.EditMessage` (BUG-001 fix).
  - `internal/decompose/decompose.go:371/415/515`  runLoop (tooled-stager fallback) still calls
    `generateMessage` (incl. EditMessage) — SAFE there because runLoop serializes via 1-deep overlap.

## §1 PRIMARY scope — SHIPPED docs (README.md + docs/*.md): ALL ACCURATE → NO-OP

Method: `grep -niE` for fast-path / disjoint / concurrent / edit / EDITMSG / EditMessage /
generateMessage / dedupe / duplicate / FR-E4 / FR-M13 / FR-M14 / US7 across README.md + docs/*.md;
read ±context for every hit. Verdict per file:

| File | Verdict | Notes |
|------|---------|-------|
| **README.md** | ✅ ACCURATE / no-op | No mention of `fast-path`/`disjoint`/`EDITMSG`/`EditMessage`/`generateMessage`/`FR-E4`/`FR-M13`/`FR-M14` (grep: 0 hits). Describes multi-commit decomposition GENERALLY (planner→stager→message→arbiter), the start-of-run freeze, the run lock, and the dedupe GUARANTEE (L428 "guarantees that no generated subject duplicates one of the last 50 subjects") — the fix RESTORED this guarantee, so the line is correct. L64/L209/L408 "concurrent edit" = an EXTERNAL concurrent edit during the run (the freeze invariant FR-M1b), NOT the BUG-001 editor race. Nothing stale. |
| **docs/README.md** | ✅ ACCURATE / no-op | Documentation INDEX only. Links to how-it-works.md; lists "stage-while-editing (`--edit`)" as a v2.1 addition. No fast-path/edit-concurrency claim. |
| **docs/how-it-works.md** | ✅ ACCURATE / no-op | The fast-path section (L105) says "the N message generations run **concurrently** and publish in CAS order (serialized `update-ref`, same as the loop)". This is STILL TRUE post-fix — message GENERATION (`generateMessageCore`) is still concurrent; the fix only moved EditMessage to the serial loop + added serial cross-concept dedupe. The doc does NOT claim editing is concurrent or that dedupe is per-concept-only, so it is accurate (silent on edit/dedupe mechanics, not stale). "Stage-while-editing (FR-E2)" (L112) and "Serialized publication" bullets are accurate. L246 "50-subject dedupe check ... ensures every generated subject is unique" — restored by the fix. L111/L117/L134/L172/L360 "concurrent" = external-during-run freeze invariant / run lock — unrelated to the bugs. |
| **docs/cli.md** | ✅ ACCURATE / no-op | `--edit` (L42): "In decompose mode each commit is gated" — matches FR-E4 verbatim ("gates EACH commit's message before its already serialized publication"). The fix makes the fast-path conform; the doc always stated the spec's intent. Other "duplicate" hits (L125/L339/L341/L448) = hook never-block / lazygit keybinding / exit-code docs — unrelated. |
| **docs/configuration.md** | ✅ ACCURATE / no-op | Only hit: `max_duplicate_retries = 3` (L146) — the FR32 dedupe retry knob. Unchanged by the fixes. |
| **docs/providers.md** | ✅ ACCURATE / no-op | `tooled_flags`/stager docs only (L31/L70/L88/L108/L112/L142). No fast-path/edit/dedupe-concurrency claim. |
| **docs/packaging.md** | ✅ ACCURATE / no-op | 0 keyword hits. |
| **docs/ci-validation.md** | ✅ ACCURATE / no-op | 0 keyword hits (CI validation procedure — unrelated). |
| **docs/windows-test-support.md** | ✅ ACCURATE / no-op | L18/L45: `cmd/stubeditor` / `fakeeditor.sh (--edit gate)` — the stub-editor TEST harness (used by the BUG-001 regression, P1.M3.T1.S1) + a note that concurrent git index ops use a throwaway index. Unrelated to the fast-path doc bug. |

**PRIMARY verdict: the shipped docs contain ZERO stale claims about the fast-path.** They describe the
spec's intended behavior (FR-E4 serialized; US7 no-duplicate), which the fixes restore — so they were
never wrong. The README/docs portion of this task is a **confirmed no-op** (record this review; no edits).

## §2 SECONDARY scope — "any architecture overviews": ONE STALE CLAIM found

The item also says to check "any architecture overviews". The repo has tracked planning architecture
overviews in `plan/019_2f5621db4d2b/architecture/` (1170 files under `plan/` are git-tracked). The
production code comment at `internal/decompose/decompose.go:743` POINTS TO `git_primitives.md` as the
authority for the fast-path's read-only-tree-reads concurrency invariant — so it is a living, code-
referenced architecture overview, not frozen history. Sweeping them:

| File | Verdict | Notes |
|------|---------|-------|
| **plan/019_2f5621db4d2b/architecture/git_primitives.md** | ❌ **ONE STALE CLAIM** | The fast-path CONCURRENCY-safety analysis (L49-52 + the "Net" L59-63) names `generateMessage` as the goroutine's function and concludes "concurrency is safe" / "does NOT touch the live index. ✅" reasoning ONLY about `.git/index`. Post-fix the goroutine calls `generateMessageCore` (no editor); `generateMessage` (the wrapper) now ALSO applies `EditMessage` (writes the shared `.git/STAGECOACH_EDITMSG` + opens `$EDITOR` — NOT concurrency-safe, BUG-001), deferred to the serial loop. The index-only test is necessary-but-NOT-sufficient — it is the EXACT blind spot that let BUG-001 (EDITMSG shared file) and BUG-002 (cross-concept dedupe) ship. See §3 for the surgical fix. |
| **plan/019_2f5621db4d2b/architecture/spec_requirements.md** | ✅ ACCURATE | Verbatim quotes of FR-M13/M14/§13.6.3 from the spec (which states correct behavior). No concurrency claim of its own. |
| **plan/019_2f5621db4d2b/architecture/system_context.md** | ✅ ACCURATE (for what it describes) | L42/L55 describe **runLoop** (the reference impl): its goroutine runs `generateMessage` + the function-signature table. runLoop was NOT changed (decompose.go:515 still calls `generateMessage` there, serialized via 1-deep overlap) — so this is correct about runLoop. The fast-path DIVERGENCE (generateMessageCore) is now captured in the tightened in-code comment (P1.M3.T3.S1). No stale claim; no edit required (would be redundant with the in-code comment). |
| **plan/019_2f5621db4d2b/architecture/findings_and_divergences.md** | ✅ ACCURATE | "fast-path" only in scope/selector context. No concurrency claim. |
| **plan/019_2f5621db4d2b/architecture/{provider_docs_state,role_defaults_drift}.md** | ✅ ACCURATE | 0 keyword hits. |
| **plan/.../bugfix/001_fb876ae39715/architecture/\*.md** | ✅ ACCURATE (out of scope) | These (bug_analysis/fix_design/system_context/test_strategy) ARE the bugfix's OWN design docs — accurate by construction. Not user docs; out of scope. |

## §3 The ONE stale claim — exact current text + the surgical fix (git_primitives.md)

**File**: `plan/019_2f5621db4d2b/architecture/git_primitives.md` (tracked; referenced by decompose.go:743).

**STALE current text** (the "message-generation phase" sub-bullet under "Implication for the fast-path",
plus the "Net" line):

```
  - The **message-generation phase** MUST NOT touch the live `.git/index`. As long as each goroutine
    reasons only over **tree-to-tree diffs** (`generateMessage` does `diff(tree[i-1], tree[i])`,
    which is read-only tree reads via `DiffTreeNames`/`TreeDiff`) — no `Add`/`ReadTree`/`WriteTree`(live)
    during message gen — concurrency is safe. `generateMessage`'s existing contract already uses
    tree-to-tree diffs (§13.6.3 invariant 2), so it does NOT touch the live index. ✅
```
… and later the "Net:" line:
```
- **Net:** the fast-path's design (serial staging sweep freezes all trees up front → concurrent
  message gen over frozen trees → serial CAS publish) is SOUND and needs NO new mutex, PROVIDED the
  implementer keeps the staging sweep strictly serial and confines message goroutines to tree reads.
```

**Why stale** (3 points):
1. Names `generateMessage` as the goroutine's function — but the fast-path goroutine now calls
   `generateMessageCore` (decompose.go:761, the BUG-001 fix P1.M1.T2.S1).
2. Concludes "concurrency is safe" for `generateMessage` — but `generateMessage` (message.go:249→259)
   now ALSO applies `EditMessage`, which writes the shared `.git/STAGECOACH_EDITMSG` + opens `$EDITOR`
   (NOT concurrency-safe — BUG-001). The fast-path deliberately calls `generateMessageCore` (no editor)
   and defers `EditMessage` to the serial publish loop (FR-E4).
3. The safety argument reasons ONLY about the `.git/index` ("does NOT touch the live index. ✅") — the
   exact blind spot that let BUG-001 (EDITMSG shared file) and BUG-002 (cross-concept dedupe) ship.
   The index-only test is necessary-but-NOT-sufficient.

**The fix (drop-in, doc-only)** — see PRP §Implementation Tasks for the verbatim replacement text:
rename `generateMessage` → `generateMessageCore` in the concurrency claim; add a one-line note that
`EditMessage` is serial-only (BUG-001/FR-E4) and that index-only reasoning missed the EDITMSG
shared-file hazard + the cross-concept dedupe gap (BUG-002); touch up the "Net" line's PROVIDED clause
to name `generateMessageCore` + "EditMessage is serial-only". Cross-ref the tightened in-code comment
(decompose.go ~741-757, P1.M3.T3.S1).

## §4 Scope fences / no-conflict

- **DOC-ONLY**: no code, no tests, no spec/ (human-owned; already correct), no tasks.json/prd_snapshot.
- **ONE file changed**: `plan/019_2f5621db4d2b/architecture/git_primitives.md` (the stale claim). Shipped
  docs (README.md + docs/*.md) get ZERO edits (confirmed accurate §1) — the no-op review is recorded
  here + in the PRP.
- **No conflict with parallel P1.M3.T3.S1**: that item edits the IN-CODE comment in
  `internal/decompose/decompose.go` (different file). This item edits a PLAN markdown doc. The two are
  COMPLEMENTARY: P1.M3.T3.S1 tightens the in-code comment; this item fixes the .md the comment points to.
  `git status --porcelain` will show two different files — no merge conflict.
- **system_context.md**: intentionally NOT edited (it correctly describes runLoop; the fast-path
  divergence is captured in-code by P1.M3.T3.S1). Editing it would be redundant + risk drift.

## §5 Validation

Doc-only → no build/test impact. Validation = (a) markdown lint clean (`make lint` includes markdownlint
via `.markdownlint.json` — verify the edited file passes; if the project's lint doesn't cover `plan/`,
run `npx markdownlint-cli2 plan/.../git_primitives.md` or confirm `make lint` is green); (b) grep guards:
`generateMessage` no longer appears in the concurrency-claim block (only `generateMessageCore`);
`BUG-001` + `EditMessage` + `FR-E4` appear in the corrected block; (c) `git status --porcelain` ==
`plan/019_2f5621db4d2b/architecture/git_primitives.md` ONLY; (d) re-run the §1 shipped-docs grep to
re-confirm 0 stale claims (sanity). Full `make test`/`make lint` should remain green (no code touched).