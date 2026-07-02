# Reference Implementation Analysis — `commit-pi` / `commit-claude`

Source of truth: `/home/dustin/projects/git-scripts/commit-pi` and `commit-claude` (zsh, proven, daily-driver). This document captures their **exact runtime behavior** and the **discrepancies vs the PRD** that the Go port must reconcile. When the PRD and the reference disagree, the **PRD governs the product** but the **reference is the proven behavioral baseline** — each discrepancy is flagged with a recommendation.

## 1. The pipeline (faithful port — preserve these mechanics)

```
1. DIFF CAPTURE
   - md_files   = git diff --cached --name-only -- '*.md' '*.markdown'
   - markdown_diff = for each md file: git diff --cached -- "$file" | head -n 100   (PER-FILE 100-line cap)
   - other_diff = git diff --cached \
                   ':!*.lock' ':!package-lock.json' ':!pnpm-lock.yaml' \
                   ':!*.snap' ':!*.map' ':!vendor/*' \
                   ':!.md' ':!.markdown' \
                 | head -c 300000                                           (TOTAL 300KB cap)
   - diff = markdown_diff + other_diff
   - if diff empty → "No staged changes found." exit 1          [PRD changes this → auto-stage / exit 2; see §4]

2. SNAPSHOT (plumbing)
   - PARENT_SHA = git rev-parse HEAD            (captured BEFORE write-tree; may be empty = root repo)
   - TREE_SHA   = git write-tree                (fails on unresolved merge conflicts → abort)
   - trap 'handle_error' INT TERM               (installed HERE, before generation)

3. PROMPT
   - commit_count = git rev-list --count HEAD   (|| 0)
   - if commit_count > 1:
       examples = git log --format='---%n%B' -20 | sed '/^$/d' | head -100
       has_multiline = awk scan: between '---' separators, any group with >1 line ⇒ multiline
       multiline_rule = conditional (see PRD §17.1)
       system_prompt = JSON-contract + essence + examples + CRITICAL-anti-reuse + multiline_rule + ~50char target
   - else (≤1 commit): conventional-commit fallback prompt
   - prompt (user) = "Generate a commit message for these changes:"

4. GENERATION LOOP  ★ TWO NESTED LOOPS — see §2 ★
   - stdin payload = printf '%s\n\n%s' "$diff" "$user_prompt"      [diff FIRST, then instruction — see §3]
   - run agent (pi: --provider zai --model $model --system-prompt "$system_prompt" --no-tools ... --no-session -p)
   - parse → commit_msg
   - dedupe check: subject = head -1 of msg; reject if in `git log --format=%s -50`

5. COMMIT (plumbing)
   - if PARENT_SHA: NEW = git commit-tree -p PARENT -m MSG TREE ; git update-ref HEAD NEW PARENT  (CAS form)
   - else (root):   NEW = git commit-tree -m MSG TREE ; git update-ref HEAD NEW                   (no expected-old)
   - echo "[$NEW] $subject"
   - git --no-pager diff-tree --no-commit-id --name-status -r "$NEW"
   - trap - INT TERM                                                            (restore handler before final commit)
```

## 2. The generation loop is TWO NESTED LOOPS (★ most important behavioral detail ★)

The reference does NOT have a single retry loop. It has an **outer duplicate-subject loop** wrapping an **inner parse-correction loop**:

```
max_duplicate_retries = 3
duplicate_retry = 0
while duplicate_retry < max_duplicate_retries:        # OUTER: duplicate rejection (PRD FR30–FR33)
    attempt = 1; max_attempts = 2
    while attempt <= max_attempts:                    # INNER: parse/empty-output correction (PRD FR29)
        response = run_agent(stdin = diff + user_prompt)
        commit_msg = parse(response)
        if commit_msg != "": break                    # parse OK → leave inner loop
        else: user_prompt = "Output STRICT valid JSON only..." ; attempt++   # 1 corrective retry
    if commit_msg == "": handle_error()               # parse failed twice → RESCUE
    subject = head -1 commit_msg
    if subject in recent_50_subjects:
        rejected_messages += [subject]; duplicate_retry++; commit_msg=""   # dup → outer retry
    else: break                                       # unique → leave outer loop
if commit_msg == "": handle_error()                   # exhausted duplicates → RESCUE
```

### Counting semantics (reconcile with PRD FR29/FR32)
- **Inner loop:** `max_attempts=2` ⇒ **1 corrective retry** on empty/invalid output. Matches PRD FR29 ("retry generation once"). ✅
- **Outer loop:** the reference runs the body for `duplicate_retry ∈ {0,1,2}` = **3 generations total**, then fails. PRD FR32 phrases this as "up to `max_duplicate_retries` (default 3) *retries*" — i.e. 1 initial + 3 retries = 4 generations. **This is an off-by-one between the reference (3 total) and the PRD's wording (4 total).**
  - **Recommendation:** implement the loop as the PRD describes (initial attempt + up to `max_duplicate_retries` retries, default 3 → up to 4 generations), because the PRD is the product spec and "3 retries" is the more intuitive reading. Document the reference's exact counting in a code comment so a maintainer can dial it back to match the proven script if desired. Either is defensible; pick the PRD wording.
- **Each outer iteration gets its OWN fresh inner budget** (the reference resets `attempt=1` at the top of the outer loop). The Go `generate` orchestrator must replicate this nesting — a flat retry counter is wrong.

## 3. stdin payload ORDERING — diff first, instruction last (★ behavioral nuance ★)

The reference pipes:
```
printf '%s\n\n%s' "$diff" "$user_prompt"
```
i.e. **stdin = `<diff>\n\n<user instruction>`, system prompt via `--system-prompt` flag.**

PRD §17.3 prose describes the user payload as "instruction, then diff." This is the **reverse** of the proven script.

- **Recommendation:** assemble the stdin payload as **`<diff>\n\n<instruction>`** (reference ordering). Rationale: (a) it is the proven daily-driver behavior; (b) placing the imperative ("Generate a commit message…") closest to where the model begins generation leverages recency. The `internal/prompt/payload.go` task must define the exact byte layout and the PRP implementing it should follow the reference ordering, treating PRD §17.3's prose as illustrative. **Flag this as an explicit decision** so it is reviewed, not silently flipped.

For agents WITHOUT a system-prompt flag (gemini/opencode/codex/cursor), the system prompt is **prepended** to this stdin/positional payload per PRD §12.2, yielding: `<system>\n\n<diff>\n\n<instruction>`.

## 4. Discrepancies: PRD vs reference (the implementation must reconcile each)

| # | Topic | Reference (proven) | PRD (governs) | Reconciliation |
|---|---|---|---|---|
| D1 | **Output contract** | JSON `{"commit_message":"..."}` parsed by `sed`/`tr` | **Raw** default ("output only the message"); JSON optional (§17.4, §12.9) | Implement PRD: raw default + robust cleanup pipeline (`parseOutput`, §12.9). JSON remains available (`output="json"`, `json_field`). The raw contract removes the reference's "no double quotes inside message" constraint. |
| D2 | **Auto-stage-all** | none — empty diff ⇒ exit 1 | **New**: `auto_stage_all=true` default; `git add -A`; exit **2** if still clean; `--no-auto-stage`, `--all` (FR16–FR20, G5) | Implement PRD (new feature). Reference's exit-1-on-empty is superseded. |
| D3 | **Exit codes** | ad-hoc (1 for empty, dup-exhaust, CAS fail) | canonical: 0/1/2/3/124 (§15.4) | Implement PRD §15.4 exit codes. |
| D4 | **claude bare_flags** | `--setting-sources "" --tools "" --disable-slash-commands --no-chrome --no-session-persistence -p` | §12.4 lists only `--tools "" --setting-sources "" --no-session-persistence` (subset) | **Use the reference's fuller set** — `--disable-slash-commands` and `--no-chrome` are CONFIRMED present in current `claude --help` (see `external_deps.md`). PRD §12.4 omitted them; add them. |
| D5 | **stdin payload order** | `<diff>\n\n<instruction>` | §17.3 prose: instruction-then-diff | Follow reference ordering (§3 above). |
| D6 | **Duplicate-retry count** | 3 generations total | "3 retries" (≈4 total) | Follow PRD wording; document reference counting. |
| D7 | **Provider hard-coding** | `pi --provider zai ...` welded in | provider manifest (§12) | Implement the manifest system (the whole point of Stagehand). |
| D8 | **Verbosity** | `VERBOSE=1` env prints DEBUG lines | `--verbose`/`-v`/`STAGEHAND_VERBOSE` (FR50) | Implement PRD flag model; reference's DEBUG lines map to `--verbose` stderr output. |

## 5. The rescue message (port verbatim, enrich per PRD §18.3)

Reference `handle_error()` fires when `TREE_SHA != "" && NEW_COMMIT_SHA == ""`:
```
❌ Commit generation failed.
------------------------------------------------------------
Your files were safely snapshotted before generation.
Tree ID: $TREE_SHA

To commit the originally staged files manually, run:
  git commit-tree -p $PARENT_SHA -m "Your manual message" $TREE_SHA | xargs git update-ref HEAD
   (omit -p if root commit)
------------------------------------------------------------
```
PRD §18.3 enriches this: on duplicate-exhaustion/parse-fail WITH a candidate message, also print *"A candidate message was produced but rejected: \"<msg>\". You can use it manually."* Implement the PRD enrichment. Rescue fires on: SIGINT/SIGTERM post-snapshot, timeout, parse-fail-after-retries, duplicate-exhaustion, (NOT on CAS failure — that prints its own message and exits 1, PRD §18.2).

## 6. Multi-line detection algorithm (port the awk heuristic)

```
examples = git log --format='---%n%B' -20 | sed '/^$/d' | head -100
has_multiline = awk '/^---$/{ if(lines>1) found=1; lines=0; next } { lines++ } END { print found+0 }'
```
Logic: split examples on `---` separators; if ANY group has more than 1 non-empty line, the repo uses multi-line (subject+body) commits. Port this to Go (a small scanner over the `git log` output). PRD FR12 ("detect by scanning the examples") → this is the concrete algorithm.

## 7. What is NOT in the reference (new PRD work, not a port)

- Provider manifest system + registry + override merge (PRD §12).
- Config precedence: flag > env > git-config > repo file > global file > builtin (PRD §16, FR34).
- `providers list/show`, `config init/path` subcommands (FR46–FR48, FR38).
- `--dry-run`, `--all`, `--no-auto-stage`, `--timeout`, `--no-color`, `--version` (FR19, FR20, FR25, FR49, FR51).
- Robust parse pipeline / code-fence stripping / JSON fallback (§12.9) — reference used brittle `sed`.
- Public library `pkg/stagehand.GenerateCommit` (§14.1).
- goreleaser / Homebrew / Scoop / AUR distribution (§21).
- Process-group signal handling with grace-period SIGKILL (§18.4 — reference just `trap`'d).

These are the genuinely new engineering surfaces; everything else is a faithful Go port of the proven pipeline.
