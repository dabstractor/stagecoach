# README.md Cross-Cutting Coherence Audit — P1.M3.T1.S1

> The full grep audit underpinning the P1.M3.T1.S1 sweep (Issue 1, FR52 run-lock qualification). Every
> contract term was grepped against `README.md` (2026-07-04, post P1.M1.T1.S1). Verdict: **README.md is
> coherent — no edit required (no-op confirmation).** The run-lock / no-op-fast-path / exit-0-or-5 /
> "Safe to run twice" discussion exists in exactly ONE place (L330), already correctly qualified.

## §1 — The authoritative qualified claim (L330, the reference the audit checks against)

P1.M1.T1.S1 already scoped the "Safe to run twice" FAQ paragraph (README.md:330) to:

- **single-commit path** (changes staged): accidental double-invoke exits **0** (nothing new staged) or
  **5 (Busy)** (genuinely new work staged).
- **decompose path** (nothing staged, dirty tree): accidental double-run exits **5 (Busy)** rather than 0
  (the in-progress run publishes a working-tree snapshot a contender can't reproduce without the lock).
- shared-filesystem caveat: the lock can't help across hosts; the `update-ref` CAS is the never-clobber
  guarantee there.

This is the SINGLE source of truth for lock/contention behavior in README.md. Any other section that
mentioned exit 0/5, "safe to run twice", the no-op fast path, or implied decompose-is-safe-to-double-run
would contradict it. The audit below confirms none does.

## §2 — Grep audit (every contract term → every hit → verdict)

Terms grepped (case-insensitive): `run.?lock`, `FR52`, `safe to run`, `nothing to do`, `exit (0|5)`,
`\bBusy\b`, `concurrent`, `double.?run`, `decompos`, `no-op fast path`, `accidental`, plus the broader
safety set `atomic|never corrupt|byte-for-byte|clobber|race` and all `exit`/status mentions.

| Line(s) | Term hit | Section / context | Discusses the lock / no-op / exit-0-5? | Verdict |
|---|---|---|---|---|
| **330** | run lock, Safe to run, nothing to do, exit 0, exit 5, Busy, concurrent, double-run, decompose, no-op fast path (implicit), accidental | FAQ "Will it corrupt my repo?" → **Safe to run twice.** | **YES — this IS the qualified claim** | ✅ Coherent — the reference (P1.M1.T1.S1). No edit. |
| 4 | decompose, atomic, never corrupt | Tagline / overview | No — general "what decompose does" + snapshot/CAS safety (never clobber HEAD). No lock/double-run claim. | ✅ Coherent. No edit. |
| 31 | decompose | Features comparison table row | No — feature-list ("Yes (auto-decompose dirty tree into N logical commits)"). | ✅ Coherent. No edit. |
| 139, 141, 144, 146, 168 | decompose | "Multi-commit decomposition" section | No — pipeline mechanics (planner→stager→message→arbiter, T_start freeze, stager constraints, --reasoning/--commits/--single). No contention/lock/double-run mention. | ✅ Coherent. No edit (see §3). |
| 338 | decompose | FAQ "Can it write multiple commits?" | No — points at the decompose section + docs. No lock mention. | ✅ Coherent. No edit. |
| 113, 132 | atomic | Quick-start / examples | No — "commits atomically" (snapshot mechanism). No lock/double-run claim. | ✅ Coherent. No edit. |
| 126 | exit (1) | `--dry-run` note | No — dry-run failure exit 1. Unrelated to the lock (no contention context). | ✅ Coherent. No edit. |
| 216 | exit (1) | Provider-not-on-$PATH fast-fail | No — exit 1 provider missing. Unrelated. | ✅ Coherent. No edit. |
| 253 | exit (1) | `--config` missing-file fast-fail | No — exit 1 config error. Unrelated. | ✅ Coherent. No edit. |
| 274 | byte-for-byte unchanged | Stage-while-generating | No — failed-generation safety. No lock/double-run claim. | ✅ Coherent. No edit. |
| 328 | atomic, byte-for-byte | FAQ "Will it corrupt my repo?" (lead) | No — snapshot/commit-tree/update-ref mechanism. No contention mention (the lock para L330 follows). | ✅ Coherent. No edit. |
| 358, 366 | atomic | Hook mode / plumbing | No — pre-commit-hook bypass + atomicity. No lock/double-run claim. | ✅ Coherent. No edit. |
| 165 | (none — `--reasoning` note) | decompose section NOTE | No — reasoning-level provider support. | ✅ Coherent. No edit. |

**No hit outside L330 discusses the run lock, the no-op fast path, exit 0/5 in a contention context,
"safe to run twice", or double-run behavior.** No contradiction exists.

## §3 — "Does the decompose section need the Busy(5) caveat?" → No

The contract asks whether the features list / overview / decompose section mention decompose in a way that
now needs the Busy(5) caveat. They do not:

- The decompose section (L139-168) describes **how decompose works** (the four-role pipeline, T_start
  freeze, stager git-scope). It makes NO claim about concurrent/double runs. The contention behavior is a
  separate concern that correctly lives in the FAQ (L330) — adding a Busy(5) caveat to the pipeline section
  would be **redundant with L330** and **out of character** for a pipeline-architecture section.
- The FAQ headline "Safe to run twice." (L330) is **immediately qualified in the same paragraph** (single-
  commit→0/5; decompose→5). A reader seeing the headline gets the qualification inline — no distant section
  needs to repeat it.
- Adding the caveat anywhere else would re-introduce the very kind of scattered, possibly-stale claim this
  sweep exists to prevent. The right design is ONE authoritative place (L330), which already exists.

## §4 — No unconditional "exits 0" claim anywhere

A targeted check for any UNQUALIFIED "exits 0" / "exit 0" / "will exit 0" claim (the bug P1.M1.T1.S1 fixed):
the ONLY "exit 0" in README.md is inside L330's qualified single-commit clause ("an accidental double-invoke
exits `0` if nothing new has been staged"). No standalone/unconditional "exits 0" remains. The fix is not
re-introduced anywhere.

## §5 — No conflict with the parallel work item

P1.M2.T4.S1 (Issue 4b, contention-message empty-field guard) touches ONLY Go code:
`internal/cmd/default_action.go`, `internal/cmd/lock_contention_test.go`, `internal/exitcode/exitcode.go`,
`internal/lock/lock.go`, `internal/lock/lock_unix.go`. It edits **no markdown** (`.md` grep of its PRP →
empty). This sweep touches ONLY `README.md` (read + possible edit). Zero file overlap ⇒ independent.

## §6 — Conclusion

**README.md is coherent with the qualified claims across all sections. No edit is required.** This
subtask is a **no-op confirmation**: the run-lock/contention discussion is singleton at L330 and already
correct; nothing else contradicts it or needs the Busy(5) caveat. The PRP instructs the implementer to
re-run the §2 grep to confirm (README could drift) and, if still clean, to record the no-op in
`research/readme-sweep-audit.md` (this file) and make NO README edit. If a contradiction IS found on
re-verify, edit the offending line to match L330's qualified claim (single-commit→0/5; decompose→5).

## §7 — Re-verification (implementation-time, 2026-07-04)

**Re-verified 2026-07-04:** re-ran the §2 audit grep (contention-term set, broader safety set, and the
bare-"exits 0" check) against the live `README.md` at the current HEAD. The run-lock / no-op-fast-path /
exit-0-or-5 / "Safe to run twice" discussion **remains singleton at L330** (already correctly qualified by
P1.M1.T1.S1). Every non-L330 hit classifies as expected and **no non-L330 hit classifies as
CONTENTION_DISCUSSION**:

- `decompos` → L4 (tagline), L31 (features table), L139/141/144/146/168 (decompose pipeline section),
  L330 (the reference), L338 (FAQ "multiple commits") — all FEATURE_DESCRIPTION; none makes a lock /
  double-run / exit-0-or-5 claim.
- `atomic` / `never corrupt` / `byte-for-byte` → L4, L69, L113, L132, L274, L328, L358, L366 — all
  SAFETY_MECHANISM (snapshot / `commit-tree` / `update-ref` / failed-generation safety); no double-run claim.
- `no-op` → L165 only, in the `--reasoning` note ("graceful no-op (no error) per FR-R6") — UNRELATED to
  the run-lock no-op fast path (it concerns reasoning-level provider support).
- The only `exit 0` / `exits 0` hit in README.md is inside L330's qualified single-commit clause; the
  bare-"exits 0" grep (`exits? `?0`?|will exit 0|it exits 0`) returns ONLY L330. No unconditional /
  bare "exits 0" exists anywhere.
- The unrelated `exit 1` mentions (dry-run / provider-missing / config-missing) are unchanged and
  out-of-scope (not contention exits).

**No contradiction found. NO README.md edit required (no-op confirmation).**

Guards confirmed at re-verify: `go build ./...` clean; `go test ./...` all PASS (doc-only task, no code
touched); `PRD.md`, `docs/cli.md`, `docs/how-it-works.md` byte-unchanged; no `.go` file changed;
`README.md` unchanged (no-op).
