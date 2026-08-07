## 17. Prompt engineering

### 17.1 The system prompt (mature repo, >1 commit)

Ported and refined from `commit-pi`. The structure:

```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences,
no quoting. If a body is warranted, use a blank line between subject and body.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Match the tone and style of these recent commits from this repository:
---
<commit 1 full message>
---
<commit 2 full message>
...
(up to 20, ≤100 lines total)

CRITICAL: You MUST NOT copy or reuse ANY phrasing from the examples above.
They show the STYLE to match — format, tone, length, conventions. Producing
the same text you have seen is STRICTLY FORBIDDEN. Your output must be
entirely original wording describing THIS specific change. Reusing example
text is a critical failure.

<multi-line rule>
Target ~50 characters for the subject line.
```

Where `<multi-line rule>` is one of:

- If history has multi-line commits: _"Only add a body (blank line + description) if the history shows multi-line commits AND these changes truly warrant detailed explanation. Otherwise, use a single-line subject only."_
- Else: _"Only output a single-line subject (no body)."_

### 17.2 The system prompt (new repo, ≤1 commit)

```
You are a commit message generator.

Output ONLY the commit message. No preamble, no markdown, no code fences.

Focus on the ESSENCE of the change (the intent/purpose), not implementation
details like filenames or function names.

Target ~50 characters (~7 words). Format: type(scope): description
```

### 17.3 The user payload

Delivered via stdin (or positional/flag per manifest). Structure:

```
Generate a commit message for these changes:

<diff payload (markdown section, then other-files section)>
```

On a duplicate-rejection retry, a rejection block is inserted after the instruction:

```
Generate a commit message for these changes.

IMPORTANT: The following messages were REJECTED because they already exist
in git history. You MUST generate something COMPLETELY DIFFERENT:
- <rejected subject 1>
- <rejected subject 2>

Create an entirely new message with different wording.

<diff payload>
```

### 17.4 Why raw output, not JSON (the v1 design call)

`commit-pi` used `{"commit_message": "..."}` and parsed it with `sed`. This required (a) telling the model never to use double quotes inside the message (a real constraint that produced awkward messages), and (b) a fragile regex. Go's `json.Unmarshal` removes (b), but (a) remains — JSON string escaping is a footgun for free-form prose, and models frequently emit invalid JSON when the message contains quotes or newlines.

Raw output ("output only the message") is more robust for this use case: there is nothing to escape, nothing to parse structurally, and the robust cleanup pipeline (§12.9) handles the rare case of a model wrapping output in a code fence. The only failure mode raw introduces is "the model added a preamble sentence," which the retry instruction corrects, and which is strictly less common than JSON-parse failures.

JSON mode remains available (`output = "json"`, `json_field = "result"`) for agents like Claude Code whose `--output-format json` is specifically designed to be machine-parsed and may be more reliable than raw for certain models. The default is raw; the option exists.

### 17.5 Planner prompt (v2; §13.6.2, FR-M3)

The planner is **bare** and receives the full working-tree diff (with binary placeholders) plus the §17.1 style examples. Its job: decide whether this changeset is one commit or many, partition accordingly, and — only if one — produce the message. Because the output is structured (a list), a **JSON contract** is justified here (unlike free-form commit messages, §17.4), with a robust parse + one retry. The planner does **not** emit hunks or line numbers — it produces the _semantic_ partition (which concept is which, and which files each touches); the stager resolves the exact hunks mechanically (FR-M5, §17.6).

The system prompt's **rules block is mode-conditional** (FR-M2): the opener, the "UNSTAGED" framing line, and the JSON contract are shared; only the `Rules:` block changes. **Auto-decompose** leans toward splitting unrelated changes (the planner runs only when nothing was staged and the tree is dirty — that precondition is itself the user's signal that they want the changes organized into commits for them, so the prompt names it explicitly). **Forced-count** (`--commits N`) treats the count as fixed. The counterweight to "lean toward SEVERAL" is a _soft_ count target of `max_commits / 2` (FR-M4): split when warranted, but don't fan a tree out into a dozen micro-commits.

System prompt — auto-decompose (sketch):

```
You are a commit-planning assistant. Given a diff of un-staged changes, decide whether they
form ONE coherent commit or SEVERAL, and partition them into logical units.

These changes were left UNSTAGED on purpose and handed to you to organize — finding the real
commit boundaries is the job you were asked to do, not a fallback to resist.

Rules:
- Split changes that serve DIFFERENT purposes into separate commits. Two changes you would
  describe with different verbs, or explain to a reviewer in separate sentences, almost always
  belong in separate commits. When torn between one commit and several, lean toward SEVERAL.
- Do not manufacture tiny commits. Group changes that only make sense together (a function plus
  its test, a refactor plus the callers it updates). A single commit is correct only when the
  whole changeset pursues ONE purpose.
- Keep the count modest: in ordinary cases at or below 6 (half the max of 12). Only exceed that
  when the changes genuinely span many unrelated concerns; do not approach the max casually.
- Account for every changed path: each file in the diff should appear in some commit's "files".
  A single file may be split across two concepts — name it in both and say, per file, WHICH
  part belongs here.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.

Respond with ONLY JSON, no prose, no code fences:
{"count": <int>, "single": <bool>, "commits": [{"title": "<short concept>", "description": "<which change belongs here, per file>", "files": ["<path>", ...]}, ...]}
- If single is true, set count=1 and ALSO include "message": "<the full commit message>".
- "files" must list every path this commit touches; "description" must say, per file, WHICH
  change belongs to this commit so a stager can find the exact hunks. Do NOT emit hunks or
  line numbers.

<style examples>
```

Forced-count mode (`--commits N`) swaps ONLY the `Rules:` block above for this one (the opener, framing line, and JSON contract are unchanged):

```
Rules:
- You MUST partition into EXACTLY the requested number of commits. Do not return more or fewer,
  and do not reconsider the count.
- Split changes that serve DIFFERENT purposes into separate commits; group changes that only
  make sense together (a function plus its test, a refactor plus its callers).
- Account for every changed path (each file in the diff in some commit's "files"); name it in
  both if a single file is split across two concepts, and say WHICH part per file.
- Each commit must be independently meaningful and reviewable.
- Respect dependencies: if change B depends on change A, A comes first.
- Match the repository's commit style shown below (format/tone), but NEVER reuse wording.
```

The `<6>` and `<12>` in the soft-target line are interpolated from `max_commits` at build time (default 12 → "6"), mirroring §17.1's `~50` subject-target interpolation. The builder emits exactly one rules block — auto-decompose unless `--commits N` — then appends the style examples (FR-F5 / §17.8) or the format scaffold.

User payload: `"Decompose these un-staged changes into commits:\n\n<diff>"`. Forced-count mode prepends: `"Produce EXACTLY N commits from these changes (do not reconsider the count):"`. Retry instruction (unparseable JSON): `"Respond with ONLY the JSON object described, no other text."`

### 17.6 Stager task prompt (v2; §13.6.2, FR-M5)

The stager is **tooled** (git access, repo-scoped). It receives one concept's title + description + files (from the planner, §17.5) as a _task_, not a system-prompt-and-diff. It must stage exactly that concept's changes and stop. The `files` list is guidance (where the concept's changes live), not a hard constraint — FR-M1c (content ⊆ `T_start`) remains the sole content guarantee; an empty list simply omits the files block.

Task prompt (delivered as the user payload; system prompt minimal/empty):

```
Stage, but do NOT commit, all changes in this repository that match this concept:

<title>
<description>

Files for this concept (where these changes live):
<files>

Use git to stage the relevant files and hunks (`git add <path>`, and for partial files apply
only the relevant hunks via `git apply --cached`). Stage ONLY the changes the description
assigns to this concept (the files above are where they live); leave everything else unstaged.
Do not commit, do not amend, do not push, do not modify file contents — only update the index.
When done, reply with the list of paths you staged and stop.
```

The hard guardrails (no commit/amend/push/ref-mutation) are restated in the prompt AND enforced structurally: the stager runs with a git-scoped tool profile (`tooled_flags`, §12.1) and stagecoach performs every ref operation itself. A stager that nevertheless attempts a commit is a best-effort concern — it cannot move stagecoach's refs (stagecoach owns those via `update-ref`), and the user-visible HEAD only advances through stagecoach's CAS.

### 17.7 Arbiter prompt (v2; §13.6.5, FR-M9)

The arbiter is **bare** and runs only if the **frozen leftover** `diff(tipTree, T_start)` is non-empty after the loop (FR-M1d) — i.e. some `T_start` content was not claimed by any stager. It receives the commits made this run (SHA + subject + file list each) and `TreeDiff(tipTree, T_start)` as the diff of the remaining changes; a concurrent working-tree change (not in `T_start`) is invisible to it. It returns a target SHA or null.

System prompt (sketch):

```
You reconcile leftover changes into commits that were just made. You are given the commits
created this run (with their messages and changed files) and a diff of changes that were not
included in any of them.

Decide: do these leftovers logically belong WITH one of those commits, or do they warrant a
NEW commit?
- Choose an existing commit only if the leftovers are part of the SAME logical change.
- When in doubt, prefer a NEW commit (return null) — never force a fit.
- You may only target a commit from the provided list.

Respond with ONLY JSON: {"target": "<sha from the list>"} or {"target": null}.
```

User payload: the commit list + the leftover diff. Stagecoach performs all resulting git (FR-M10); the arbiter only returns the decision.

### 17.8 Format modes, locale, and context (v2.1; §9.19)

Three orthogonal deltas to the prompts above. All default to off; `auto` format means §17.1/§17.2 verbatim.

**Format modes (FR-F1–F5).** A non-`auto` format **replaces** the style-examples block (the `Match the tone and style…` section plus the anti-reuse warning — there are no examples to protect) with an explicit contract; the output rules, essence-not-filenames instruction, and multi-line rule are retained:

- `conventional`: _"Format: `type(scope): description`. type ∈ feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert; scope optional. Target ~50 characters for the subject."_
- `gitmoji`: _"Begin the subject with exactly ONE emoji from the gitmoji list below (the emoji character itself, not a `:shortcode:`), followed by a space and the description."_ — followed by the compiled-in gitmoji reference table (emoji + meaning).
- `plain`: no format contract and no examples; output rules + essence + subject-length target only.

The planner's partitioning prompt (§17.5) is unchanged by format modes; when the planner emits a message (FR-M11), its style-examples block undergoes the same substitution.

**Locale (FR-F6).** When set, appended to the system prompt (any format, both repo-age variants): `Write the commit message in <lang>.` Nothing else changes — the diff, examples, and rules stay in their original language; models handle the mix natively, which is why stagecoach ships zero i18n prompt files.

**Context (FR-F7).** When `--context` is given, inserted into the **user payload** (message and planner roles), after the instruction line and before the diff — the same slot the duplicate-rejection block occupies (§17.3), and before it when both are present:

```
Additional context from the user (treat as authoritative):
<text>
```

**Template (FR-F8) is not a prompt feature.** It is a post-generation string substitution (parse → cleanup → template → duplicate check); the model never sees it, so it can never leak into the generated prose.

---

