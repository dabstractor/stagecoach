# Docs Sweep Audit — P1.M3.T1.S1 (FR3i sentinel newline + diff_context hardening, Mode B)

> The full audit underpinning the P1.M3.T1.S1 changeset-level docs sweep (bugfix `001_7e79f5773da8`,
> Issues 1 & 2). Three docs × two concerns. Verdict: **no inaccuracies and no stale references — a no-op
> confirmation, with ONE optional clarification** (how-it-works.md L144). P1.M2.T1.S1 owns the
> docs/configuration.md diff_context update; this sweep does NOT duplicate it.

## §1 — Scope boundary: who owns what

- **P1.M2.T1.S1** (parallel, implementing) owns the **diff_context range/rejection** Mode-A docs in
  `docs/configuration.md` (3 spots: ~107 comment, ~131 table row, ~147 prose — adds "valid range 0–3;
  out-of-range rejected at config load") + `internal/config/bootstrap.go:291` template comment. It touches
  **neither** `docs/how-it-works.md` **nor** `README.md`. ⇒ This sweep must NOT restate the diff_context
  range/rejection in configuration.md (duplicate). The only reason to touch configuration.md here would be
  an inconsistency P1.M2.T1.S1 left — none exists (its spec is coherent).
- **P1.M1.T1.S1** (complete) = the sentinel-newline fix in `internal/git/truncatediff.go` (INTERNAL output
  line-shape; a trailing `\n` after each `... [truncated]` sentinel). No config/API surface change.
- **This task (P1.M3.T1.S1)** = sweep README.md + docs/how-it-works.md + docs/configuration.md for two
  accuracy concerns, update ONLY inaccurate/materially-improving statements.

## §2 — Concern (a): per-file truncation format / sentinel line-shape (Issue 1's internal fix)

Issue 1's fix changed INTERNAL output line-shape only (each truncated file's section now ends with
`... [truncated]\n` so the next `diff --git` begins at a line start). No config/API surface change. The
question: does any doc state a line-shape that is now wrong, or imply sentinels can run together?

| Doc | Line(s) | What it says about truncation/sentinel | Line-shape claim? | Inaccurate? | Verdict |
|---|---|---|---|---|---|
| README.md | 66 | features row: "optionally capped to your model's context window via `token_limit`" | NONE (no sentinel/format/line-shape) | No | ✅ No edit |
| docs/how-it-works.md | 143 | legacy caps: `... [diff truncated at N bytes]` / `... [diff truncated at N lines]` sentinels | NONE (no line-shape; no concatenation claim) | No | ✅ No edit |
| docs/how-it-works.md | 144 | water-fill: "every file *larger* than `L` is truncated to `L` (with a `... [truncated]` marker)" | NONE — silent on line-shape; does NOT imply sentinels run together | No (not inaccurate) | ⚠️ OPTIONAL clarification (see §4) |
| docs/configuration.md | 146 | token_limit prose: budget, ≈4 chars/token, supersedes legacy caps | NONE (no sentinel/format/line-shape) | No | ✅ No edit |

**No doc implies sentinels can run together** (the contract's "ensure" clause is already satisfied). No doc
is inaccurate on truncation line-shape. The only action is the OPTIONAL note in how-it-works.md L144 (§4).

## §3 — Concern (b): diff_context valid range / out-of-range behavior (Issue 2's fix)

Issue 2's fix (P1.M2.T1.S1) makes out-of-range `diff_context` fail config load with a clear error (was:
silently clamped to 1). The question: does any doc state a range/clamp behavior that is now wrong?

| Doc | Line(s) | What it says about diff_context | Range/out-of-range claim? | Inaccurate? | Verdict |
|---|---|---|---|---|---|
| README.md | 66 | (no diff_context mention) | NONE | No | ✅ No edit |
| docs/how-it-works.md | 138 | "Tune it with `diff_context`: `0` = changed lines only, `1` = one anchor line (the default), `3` = git's default" | Lists 0/1/3 (all valid); NO range/out-of-range statement; NO clamp claim | No (0/1/3 are valid; not inaccurate) | ✅ No edit (the authoritative range/rejection detail is configuration.md's job — P1.M2.T1.S1; restating it here would DUPLICATE) |
| docs/configuration.md | 107/131/147 | "0 = changed-lines-only, 1 = one anchor (default), 3 = git default" (pre-P1.M2.T1.S1) | (P1.M2.T1.S1 is ADDING "valid range 0–3; out-of-range rejected at config load" here) | No — P1.M2.T1.S1 owns this surface | ✅ No edit (DO NOT duplicate P1.M2.T1.S1) |

**No doc references "silent clamping"** — there is no stale clamp claim to correct. how-it-works.md L138's
0/1/3 listing is accurate (all three are valid values); the full range/out-of-range contract is P1.M2.T1.S1's
configuration.md update. how-it-works.md is the explainer, configuration.md is the knob reference — keeping
the range detail in one place (configuration.md) is the right design (avoids the scattered-claim anti-
pattern). No edit for concern (b) in any of the three docs.

## §4 — The ONE optional clarification (how-it-works.md L144)

The contract: *"optionally note each truncated file's section is newline-separated."* how-it-works.md L144
describes the water-fill and the `... [truncated]` marker but is silent on line-shape. A short clarifying
clause improves the reader's mental model (the marker ends the file's section on its own line; the next
file's `diff --git` begins fresh) and aligns the doc with the fixed shipped behavior. It is OPTIONAL (the
doc is not inaccurate without it) and NOT a duplicate (no other doc states this).

Suggested wording (append into the existing marker parenthetical on L144, house-style-safe — markdownlint
only disables MD013/MD033/MD060):

- current: `…every file larger than L is truncated to L (with a … [truncated] marker). Small files are never…`
- optional: `…every file larger than L is truncated to L (with a … [truncated] marker that ends the file's
  section on its own line, so the next file's diff --git begins fresh). Small files are never…`

If the implementer judges this adds clutter to an already-dense paragraph, skipping it is acceptable — the
doc remains accurate. The choice is documented either way in this audit note.

## §5 — Stale-reference check (contract output requirement)

The contract requires "no stale references to the pre-fix sentinel gluing or to silent diff_context
clamping." Verified by grep across all three docs:
- No doc describes sentinels abutting the next section (none did even before the fix) ⇒ no "sentinel
  gluing" reference to remove.
- No doc says diff_context is "clamped"/"silently adjusted" ⇒ no "silent clamping" reference to remove.
  (configuration.md's pre-P1.M2.T1.S1 text lists 0/1/3 without any clamp claim; P1.M2.T1.S1 adds the
  explicit "out-of-range rejected" wording.)

⇒ The stale-reference requirement is already satisfied; no removals needed.

## §6 — House style

`.markdownlint.json`: `default: true`, with MD013 (line length), MD033 (inline HTML), MD060 (no-punctuation-
heading) DISABLED. So: any edit may use long lines (no wrap constraint), may use inline HTML if absolutely
needed (not expected here), and headings ending in punctuation are tolerated. The recommended L144 edit uses
plain prose + inline code (`… [truncated]`, `diff --git`) — fully compliant. No other lint risk.

## §7 — Conclusion

**No inaccuracies and no stale references across README.md, docs/how-it-works.md, docs/configuration.md.**
The changeset-level docs are consistent with the shipped behavior (sentinel newline fix + diff_context
validation). The only recommended action is the OPTIONAL how-it-works.md L144 newline-separation clause
(§4). configuration.md's diff_context range/rejection docs are P1.M2.T1.S1's deliverable (do not duplicate).
README.md needs no edit (no relevant claim). This is a **no-op confirmation (+ one optional clarification)**.
The PRP instructs the implementer to re-verify the audit at implementation time (docs drift) and record the
outcome here.

## §8 — Implementation-time re-verification (2026-07-04)

Re-verified **2026-07-04** at HEAD `b7f723d`: re-ran the audit grep against `README.md` /
`docs/how-it-works.md` / `docs/configuration.md`. P1.M2.T1.S1 has landed its `docs/configuration.md`
diff_context edits (L107 comment, L131 table row, L147 prose — each now carries "valid range 0–3;
out-of-range rejected at config load"). Classification of every truncation/diff_context hit:

- README.md L66 — `token_limit` feature row; no truncation-format/diff_context/line-shape claim ⇒ accurate, no edit.
- docs/how-it-works.md L134 — skeleton bullet (FR3g); accurate, no edit.
- docs/how-it-works.md L138 — diff_context lists 0/1/3 (all valid values); accurate explainer, no edit
  (range/out-of-range contract is configuration.md's single source of truth — not duplicated).
- docs/how-it-works.md L143 — legacy caps sentinels; no line-shape/concatenation claim ⇒ accurate, no edit.
- docs/how-it-works.md L144 — water-fill + `... [truncated]` marker; was silent on line-shape (not
  inaccurate) ⇒ **optional L144 clause APPLIED** (the marker "ends the file's section on its own line, so
  the next file's `diff --git` begins fresh"). markdownlint clean (0 errors).
- docs/how-it-works.md L146 — knob pointer to configuration.md; accurate, no edit.
- docs/configuration.md L104–107 / L130–131 / L146–147 — token_limit + diff_context knob docs;
  P1.M2.T1.S1's surface, coherent; **NOT touched/duplicated** here.

Stale-reference grep (`sentinel.*glue|glued|silent.*clamp|silently clamp`) across the three docs ⇒ ZERO
hits. Range/rejection-outside-configuration.md grep (`range 0.3|out-of-range|rejected at config` against
how-it-works.md + README.md) ⇒ ZERO hits (the scatter anti-pattern is absent).

Guards: `PRD.md` / `docs/cli.md` / `docs/providers.md` byte-unchanged; no `.go` file changed
(`go build ./...` clean; `go test ./...` all PASS — doc-only). Diff stat: only `docs/how-it-works.md`
(1 insertion, 1 deletion, the L144 clause). README.md and docs/configuration.md unchanged.

Outcome: changeset-level docs consistent with the shipped behavior; the ONE optional L144 newline-separation
clause applied; no other edit; no stale sentinel-gluing / silent-clamp reference remains.
