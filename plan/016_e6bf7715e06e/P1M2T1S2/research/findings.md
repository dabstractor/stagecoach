# Research Findings — P1.M2.T1.S2 (cross-cutting chrome-disable docs: how-it-works / README index / README)

## 0. Task shape (one sentence)
A pure **Mode-B documentation** task: make three cross-cutting overview docs honestly surface the
chrome-disable story (FR-C1–C5, §9.28) that the manifests already implement and that the sibling
P1.M2.T1.S1 just added to docs/providers.md. NO code, NO tests, NO schema.

The three edits:
- (a) **docs/how-it-works.md** — extend the `### Safety invariant` paragraph with ONE verbatim sentence.
- (b) **docs/README.md** — add ONE capability-index line (verbatim, from the contract).
- (c) **README.md** — add a BRIEF chrome-less mention to the existing `### Will it corrupt my repo?` FAQ
  answer (no new top-level section).

## 1. ⭐ S1 (P1.M2.T1.S1) is ALREADY APPLIED in the working tree
Verified live (2026-07-13): docs/providers.md already has BOTH S1 edits landed:
- **Line 78**: the 9-column table header incl. `Chrome-disable` (between "Tool-disable approach" and "Stager?").
- **Line 100**: the new asymmetry bullet `- **Chrome is a separate axis** (all providers): …`.

This means:
1. The anchor target `providers.md#tools-disable-asymmetry` (from the existing `## Tools-disable
   asymmetry` heading at line 90) **already exists** — the how-it-works.md + docs/README.md + README.md
   links resolve. (GitHub anchor for "Tools-disable asymmetry" = `#tools-disable-asymmetry`.)
2. S1 is NOT a moving target here — its output is stable in the working tree. This task builds on it.
3. Do NOT re-edit docs/providers.md (S1's exclusive domain).

## 2. The authoritative chrome surface facts (from builtin.go CHROME-DISABLE notes, P1.M1.T1.S1)
The verbatim content this task records must agree with these verified manifest notes:
- **pi** (builtin.go:43–47): extensions (`--no-extensions`), skills (`--no-skills`),
  prompt-templates (`--no-prompt-templates`), context files (`--no-context-files`) ALL disabled.
  **MCP is NOT disabled** — pi has no `--no-mcp`; `--no-tools` suppresses MCP tool *use* but servers
  configured in settings may still connect at startup (tracked limitation).
- **claude** (builtin.go:120–122): `--tools ""` disables ALL built-in tools (MCP surfaces as tools);
  `--setting-sources ""` blocks the settings files where MCP servers, skills, and extensions are
  configured. (So claude covers chrome via two mechanisms.)
- **agy** (builtin.go:212–213): exposes NO per-surface chrome-disable switch for
  skills/extensions/context-files/MCP. (read-only constraint only → documented limitation)
- **codex** (builtin.go:363–365): `--sandbox read-only` + `--ephemeral`; no per-surface chrome switch
  (notes MCP/AGENTS.md/skills as surfaces it cannot switch off). (documented limitation)
- **cursor** (builtin.go:407), **opencode** (builtin.go:310): same pattern — no per-surface chrome
  switch; read-only. (documented limitation)

So: **chrome = {skills, extensions/prompt-templates, context files (AGENTS.md/CLAUDE.md), MCP servers}**.
Two providers switch it off (pi, claude); five document the gap (codex, cursor, opencode, agy, qwen-code).
The contract's how-it-works.md sentence ("skills, extensions, context files, and MCP servers … pi and
claude today … tracked limitations") is ACCURATE against these notes.

## 3. Edit (a) — docs/how-it-works.md `### Safety invariant`
- **Location**: `### Safety invariant` heading at **line 195**; the paragraph at **line 197** (single
  long line): *"No provider mutates the repository (PRD §18.1). … it never runs `git add`, `git commit`,
  or any write command."*
- **No existing chrome mention** in how-it-works.md (grep confirmed: only "bare mode on every provider"
  at line 326, in the unrelated work-description-mode section — not safety). So this is a clean ADD.
- **Edit**: APPEND the contract's verbatim sentence to the END of the line-197 paragraph (after the
  final period). The sentence (exact, with the providers.md relative link — how-it-works.md lives in
  docs/, so `providers.md#…` is the correct relative path):
  > "Every provider also renders chrome-less — skills, extensions, context files, and MCP servers are
  > disabled wherever the agent CLI exposes a switch for them (pi and claude today); surfaces a provider
  > cannot switch off are documented as tracked limitations rather than hidden assumptions. See
  > [providers.md](providers.md#tools-disable-asymmetry) for the per-provider chrome-disable details."
- **Why this wording is accurate**: "wherever the agent CLI exposes a switch" is the load-bearing phrase
  — it does NOT claim MCP is disabled on pi (no switch exists); the next clause ("surfaces a provider
  cannot switch off are documented as tracked limitations") covers exactly that gap.

## 4. Edit (b) — docs/README.md `## Capability index`
- **Location**: the `## Capability index` section (after the `## Documentation index` table). Existing
  entries use the format: `- **Name** → [doc.md#anchor](doc.md#anchor) — description` (relative paths,
  no `docs/` prefix, because docs/README.md itself lives in docs/). The intro line reads "Each v2.1
  capability maps to a specific doc anchor:".
- **Existing entries** (do NOT touch): Payload exclusions, Message shaping, Git hook mode, Tool
  integrations, `--edit` / `--push`, Discovery, Concurrency & lock reclamation.
- **Edit**: APPEND one new bullet (contract-verbatim; the relative path `providers.md#…` is correct):
  > `- **Chrome-disable (v2.9)** → [providers.md#tools-disable-asymmetry](providers.md#tools-disable-asymmetry) — every provider renders chrome-less where the agent CLI allows it.`
- **Version-label note (see §6)**: the contract specifies "(v2.9)"; the existing capability-index
  entries carry NO version labels, so this is a slight style deviation the contract explicitly requests.

## 5. Edit (c) — README.md `### Will it corrupt my repo?` FAQ
- **Location**: the `## FAQ` → `### Will it corrupt my repo?` heading at **line 337**. Its answer is
  PURELY mutation-safety (atomic snapshot, never touches live index, per-repo run lock, orphan
  self-exit). NO chrome mention anywhere in README.md (grep confirmed: only "can never corrupt your
  repo" hero pitch at line 4 + "bare model" config notes at 232/242, unrelated).
- **The Features table** (line ~66) has NO dedicated safety/chrome row (rows: decomposition, exclusions,
  payload optimization, multi-turn, message shaping, hook mode, commit hooks, integrations, edit/push,
  discovery). So per the contract ("check the feature table at ~line 66 OR the FAQ"), the FAQ is the
  home — NOT a new top-level section, NOT a new Features row.
- **Edit**: add a BRIEF sentence/short paragraph at the END of the `### Will it corrupt my repo?`
  answer (after the orphan-self-exit paragraph). The contract does NOT give verbatim text for README.md
  (only "a concise mention"); craft it brief, as a refinement of the existing safety answer, linking to
  the detail. Proposed wording (links use `docs/providers.md#…` because README.md is at repo root):
  > "And the commit-message call itself is **chrome-less** where the agent allows it — skills,
  > extensions, context files, and MCP servers are switched off (pi, claude), so nothing loads, spawns,
  > or injects around the call; providers that expose no such switch document the gap instead
  > ([docs/providers.md](docs/providers.md#tools-disable-asymmetry))."
  Keep it to ~2–3 sentences max. Do NOT pitch a new feature; it refines "is it safe to run".

## 6. Version-label observation (v2.9)
The contract's docs/README.md entry carries "(v2.9)". A repo-wide grep confirms **"v2.9" appears NOWHERE
else** in docs/README.md/README.md/internal (the only "2.9" hits are PRD "§12.9" section refs and
"git ≥2.9" — unrelated). The docs' established progression is v2.1 (this revision, PRD §10.4) → v2.4
(commit hooks, README/hot-it-works). So "(v2.9)" is a NEW forward-looking label the contract is
introducing for this P2/G25 (§9.28) chrome-disable batch. **Decision: use the contract's "(v2.9)"
verbatim** — it is monotonic (v2.9 > v2.4), the contract explicitly specifies it, and introducing the
label alongside the capability is the natural place. (Noted here so the implementer is not surprised
that no other doc says v2.9 yet; it is intentional, not a typo.)

## 7. Scope fences (what NOT to do)
- NO code, NO tests, NO schema, NO builtin.go (source of truth, P1.M1.T1 done), NO providers/*.toml.
- NO docs/providers.md edit (S1's domain — already landed; do not touch).
- NO new README.md top-level section and NO new Features-table row (the contract forbids both; refine
  the existing FAQ answer only).
- NO rewrite of the existing `### Safety invariant` mutation-safety text — APPEND one sentence only.
- NO edit to PRD.md / tasks.json / prd_snapshot.md (read-only).
- The only files touched: docs/how-it-works.md, docs/README.md, README.md (exactly these 3).

## 8. Validation approach (docs are NOT in `make`)
`make lint` is golangci-lint (Go only); there is NO markdownlint make target (`.markdownlint.json`
exists with MD013/MD033/MD060 OFF but is not wired to make). Validation = grep guards + manual render +
scope guard (`git diff --name-only` == exactly the 3 files). `make test` is run only to prove the
working tree is otherwise clean (a docs edit cannot break Go tests).
