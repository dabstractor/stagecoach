---
name: "P1.M2.T2.S1 — Rename config file discovery paths and lock directory paths (stagehand→stagecoach)"
description: |
  The FILESYSTEM-PATH surface of the stagehand→stagecoach project rename (PRD h2.30 + §16.1 + §18.5). Two path
  families become `stagecoach`: (A) **config discovery** — global `~/.config/stagecoach/config.toml`
  (`$XDG_CONFIG_HOME/stagecoach/config.toml`) + repo-local `.stagecoach.toml` (`internal/config/file.go`); (B)
  **lock directory** — `…/stagecoach/locks` (`internal/lock/lock.go`). Plus the doc comments that describe those
  paths, the user-facing strings that NAME them (the lock contention error, the bootstrap-config-written notice,
  the repo-local-provider notice), and the test assertions that pin them. PRD §16.1 fixes the config paths;
  §18.5 fixes the lock paths; h2.30 mandates the rename.

  ⚠️ **THE central design call — rename ONLY path/filename literals + the doc/user-strings that NAME them;
  leave every OTHER `stagehand` ref to its owner.** This task's targets are structurally specific: the quoted
  directory literal `"stagehand"` (in `filepath.Join(…, "stagehand", …)`), the filename `.stagehand.toml`, the
  path doc comments (`stagehand/config.toml`, `stagehand/locks`), and the path-naming user strings
  (`stagehand run lock held`, `stagehand: wrote bootstrap`, `stagehand: repo-local config`). It does NOT touch:
  git-config keys `stagecoach.*` (S2), other `"stagehand:"` error prefixes (P1.M2.T3.S1), the bootstrap header
  prose `# Stagehand configuration file` (P1.M2.T3.S2), project-name prose (P1.M2.T3/P1.M5), or command-name
  strings (P1.M2.T3/P1.M3). See research §2 for the precise boundary.

  ⚠️ **THE second design call — NO overlap with the parallel S2 (git-config keys); the patterns are disjoint.**
  S2's perl pass `(?<![.\/])stagehand\.` matches `stagehand` + DOT. This task's targets are `"stagehand"` (quote
  after, not dot), `.stagehand.toml` (S2's lookbehind preserves it — preceded by `.`), `stagehand/…` (slash
  after), `stagehand:`/`stagehand ` (colon/space after). S2 provably does NOT touch any of them; this task
  provably does NOT touch `stagecoach.*` (already renamed by S2). The two passes are disjoint by construction
  (research §3). INPUT: the project with env vars (S1) + git-config keys (S2) already renamed.

  ⚠️ **THE third design call — the codebase is at `/home/dustin/projects/stagehand` (NOT `/stagecoach`).** The
  module path is already `github.com/dustin/stagecoach` (M1), but the on-disk directory keeps its name. Run ALL
  commands from `/home/dustin/projects/stagehand`. Verify: `head -1 go.mod` → `module github.com/dustin/stagecoach`.

  ⚠️ **THE fourth design call — two targeted seds (scoped to 5 files) for the ~25 mechanical path/filename
  repeats + ~10 manual edits for the unique prose/error/path-doc sites.** sed 1: `s/"stagehand"/"stagecoach"/g`
  (the directory literal — only in filepath.Join path contexts, verified). sed 2: `s/\.stagehand\.toml/.stagecoach.toml/g`
  (the filename — specific string; does NOT match `.stagehand/config.toml`). Then manual `edit` for the doc
  comments + error/notice strings (file.go:92/94/95/125/126/491, lock.go:51/60/219, load.go:109, + 2 test
  comments). See research §4.

  Deliverable (edits to 7 existing files — NO new files): `internal/config/{file.go, load.go, bootstrap.go,
  file_test.go, load_test.go}` + `internal/lock/{lock.go, lock_test.go}`. OUTPUT: config discovered at
  `~/.config/stagecoach/config.toml` + `.stagecoach.toml`; locks at `stagecoach/locks`; `go test ./internal/config/...
  ./internal/lock/... -count=1` passes. DOCS: Mode A — docs/configuration.md path refs are P1.M4.T1.S2 (NOT this
  task). SCOPE: internal/config/ + internal/lock/ ONLY.
---

## Goal

**Feature Goal**: Rename every filesystem-PATH `stagehand` reference to `stagecoach` in the config-discovery
and lock-directory code paths, so config files are discovered at `~/.config/stagecoach/config.toml` +
`.stagecoach.toml`, and locks live at `…/stagecoach/locks` — matching the stagecoach identity (PRD §16.1, §18.5,
h2.30). Includes the doc comments describing those paths + the user-facing strings that name them + the test
assertions.

**Deliverable** (edits to 7 existing files):
1. **`internal/config/file.go`** — globalConfigPath literals (L102/108 `"stagehand"`→`"stagecoach"`) + doc
   (L92/94/95); repoLocalConfigPath filename (L127 `.stagehand.toml`→`.stagecoach.toml`) + doc (L125/126);
   loadRepoLocalConfig doc (L470) + repoProviderNotice (L491).
2. **`internal/lock/lock.go`** — lockDir literals (L222/225/231 `"stagehand"`→`"stagecoach"`) + doc (L219);
   HeldError doc (L51) + Error() (L60).
3. **`internal/config/load.go`** — bootstrap notice (L109) + Layer-3 comment (L118).
4. **`internal/config/bootstrap.go`** — template path comments (L244/246 `.stagehand.toml`→`.stagecoach.toml`).
5. **`internal/config/file_test.go` + `load_test.go` + `internal/lock/lock_test.go`** — all path/filename
   assertions (the `"stagehand"` directory + `.stagehand.toml` filename + the path doc comments).

**Success Definition**: `go build ./...`, `go vet ./...`, `gofmt -l` clean; `go test ./internal/config/...
./internal/lock/... -count=1` + `go test ./... -count=1` green; config discovered at `stagecoach/` paths;
locks at `stagecoach/locks`; zero residual path/filename `stagehand` refs in the 7 files; S2's git-config
keys (`stagecoach.*`) intact; P1.M2.T3's prose/error-prefix scope untouched (bootstrap header still says
"Stagehand"); go.mod/go.sum unchanged; only the 7 files touched.

## User Persona

**Target User**: Users whose config/lock files move to the stagecoach-named locations. After this, `stagecoach
config path` prints `~/.config/stagecoach/config.toml`; a repo-local override is `.stagecoach.toml`; the lock
dir is `$XDG_RUNTIME_DIR/stagecoach/locks`. Transitively PRD §16.1 (config resolution) + §18.5 (the run lock).

**Use Case**: A user creates `~/.config/stagecoach/config.toml` (or `./.stagecoach.toml`) and stagecoach
discovers it; two terminals in one repo contend on `stagecoach/locks/<hash>.lock`.

**User Journey**: `stagecoach` → `globalConfigPath()` → `~/.config/stagecoach/config.toml` (renamed); repo
override → `./.stagecoach.toml` (renamed); commit → `lockDir()` → `stagecoach/locks` (renamed).

**Pain Points Addressed**: the on-disk paths match the new project name; no stale `stagehand/` config/lock
dirs remain to confuse users or fragment their config.

## Why

- **Completes the path surface rename.** §16.1 documents `stagecoach/config.toml` + `.stagecoach.toml`; §18.5
  documents `stagecoach/locks`. The code must match.
- **Disjoint from S2 (no merge risk).** S2's git-config perl pass provably doesn't touch these patterns
  (research §3); this task provably doesn't touch `stagecoach.*`. The two land independently.
- **Atomic with its tests.** The path literals + the test assertions rename together → the config/lock test
  suites pass with the new paths (the tests construct the expected `stagecoach/…` paths).
- **Cheap + mechanical.** ~25 sed-able repeats + ~10 manual prose edits; no logic change, no new types.
- **No API/config-schema change.** The path strings are data; the discovery LOGIC is unchanged. go.mod unchanged.

## What

The `"stagehand"` directory literal, the `.stagehand.toml` filename, their doc comments, and the path-naming
user strings become `stagecoach` across `internal/config/` + `internal/lock/`. No git-config keys, no other
error prefixes, no bootstrap header, no project-name prose, no docs/*.md (those are sibling tasks).

### Success Criteria

- [ ] `internal/config/file.go`: globalConfigPath returns `…/stagecoach/config.toml` (L102/108); doc L92/94/95
      updated; repoLocalConfigPath returns `.stagecoach.toml` (L127); doc L125/126 updated; L470 doc + L491
      notice updated (`.stagecoach.toml` + `stagecoach:` prefix).
- [ ] `internal/lock/lock.go`: lockDir returns `…/stagecoach/locks` (L222/225/231); doc L219 updated;
      HeldError doc L51 + Error() L60 say `stagecoach`.
- [ ] `internal/config/load.go`: bootstrap notice L109 `stagecoach: wrote bootstrap config…`; comment L118
      `.stagecoach.toml`.
- [ ] `internal/config/bootstrap.go`: template comments L244/246 `.stagecoach.toml` (HEADER L236/238 UNCHANGED —
      P1.M2.T3.S2's scope).
- [ ] `file_test.go` + `load_test.go` + `lock_test.go`: all `"stagehand"` dir-literal + `.stagehand.toml`
      filename + path-doc-comment assertions updated to `stagecoach`/`.stagecoach.toml`.
- [ ] `go build ./...`, `go vet ./...`, `gofmt -l`, `go test ./internal/config/... ./internal/lock/... -count=1`,
      `go test ./... -count=1` all clean/green; go.mod/go.sum unchanged; only the 7 files touched.
- [ ] ZERO residual path/filename `stagehand` refs in the 7 files (GATE 1); S2's git-config keys intact;
      P1.M2.T3's prose scope untouched (bootstrap header still "Stagehand configuration file").

## All Needed Context

### Context Completeness Check

_Pass._ A developer with no prior knowledge can implement this from: the complete site map (research §1 — every
file:line + old→new), the two scoped seds + the manual-edit list (§4), the scope boundary with P1.M2.T3 (§2),
the disjoint-from-S2 proof (§3), and the five verification gates (§5). No feature knowledge beyond "rename these path strings."

### Documentation & References

```yaml
# MUST READ — the authoritative site map + mechanism
- docfile: plan/012_963e3918ec08/P1M2T2S1/research/path-and-lock-dir-rename.md
  why: the COMPLETE verified site map (§1 — every file:line + old→new, source + tests), the scope boundary with
       P1.M2.T3 (§2 — what NOT to touch), the disjoint-from-S2 proof (§3), the two-sed + manual-edit mechanism
       (§4), the five verification gates (§5), scope fences (§6).
  critical: §1 (the exact edits) + §2 (the boundary — leave error-prefixes/bootstrap-header/prose to P1.M2.T3)
       + §3 (no S2 overlap — the patterns are disjoint).

# The contract — what the parallel S2 (git-config keys) does + the disjointness proof
- docfile: plan/012_963e3918ec08/P1M2T1S2/PRP.md
  why: S2 renames `stagehand.*` git-config keys via perl `(?<![.\/])stagehand\.`. Its lookbehind PRESERVES
       `.stagehand.toml` (category B — this task's scope). The PRP explicitly says ".stagehand.toml refs survive
       (P1.M2.T2.S1's scope, untouched)." Confirms the codebase location (.../stagehand, module .../stagecoach).
  critical: S2 is a HARD prerequisite (assume LANDED). This task's targets are intact after S2 (research §3).

# The files EDITED (read the exact current text before editing)
- file: internal/config/file.go
  section: globalConfigPath (L92-108 — the doc + the two filepath.Join literals); repoLocalConfigPath (L125-127);
           loadRepoLocalConfig (L470 doc); repoProviderNotice (L491 — the notice string).
  why: the config PATH literals + filename + the path-naming notice. The two `"stagehand"` dir literals (L102/108)
       + the `.stagehand.toml` filename (L127) are the sed targets; L92/94/95/125/126/470/491 are manual.
  pattern: `"stagehand"` → `"stagecoach"` (filepath.Join arg); `.stagehand.toml` → `.stagecoach.toml`.
  gotcha: L126 has `.stagehand/config.toml` (a REJECTED alt-design dir ref) — rename to `.stagecoach/config.toml`
           for consistency (it's NOT matched by the `.toml` sed — edit manually). L95 "Stagehand config path" is
           project-name prose IN the path doc — update to "Stagecoach" (keeps the doc coherent with the renamed path).

- file: internal/lock/lock.go
  section: HeldError doc (L51) + Error() (L60); lockDir (L219 doc + L222/225/231 literals).
  why: the lock PATH literals + the contention error that names the lock. The three `"stagehand"` dir literals
       (L222/225/231) are sed targets; L51/60/219 are manual.
  pattern: `"stagehand"` → `"stagecoach"` (filepath.Join); `stagehand run lock held` → `stagecoach run lock held`.
  gotcha: lock.go:10 (`no stagehand imports`) is project-name PROSE, not a path — LEAVE it (P1.M5 audit).

- file: internal/config/load.go
  section: L109 (the bootstrap-written notice) + L118 (the Layer-3 comment).
  why: the notice names the config PATH; the comment names the filename. Both manual edits.
  gotcha: load.go:520-527 + migrate.go:108-114 are `"stagehand:"` schema/migration error prefixes — NOT path-related
           → P1.M2.T3.S1. LEAVE them. load.go:109 is path-related (names the config file path) → MINE.

- file: internal/config/bootstrap.go
  section: L244/246 (template path comments referencing `.stagehand.toml`).
  why: the filename in the generated config's precedence comment. sed target (`.stagehand.toml`).
  gotcha: L236 (`# Stagehand configuration file`) + L238 (`Generated by \`stagehand config init\``) + L153 are the
           bootstrap HEADER / project-name prose → P1.M2.T3.S2. LEAVE them. ONLY L244/246 are this task.

# The PRD (the path spec)
- file: PRD.md   §16.1 (h3.76 — `$XDG_CONFIG_HOME/stagecoach/config.toml`, `./.stagecoach.toml`),
                    §18.5 (h3.92 — `$XDG_RUNTIME_DIR/stagecoach/locks`), h2.30 (the rename mandate).
  why: the canonical path names this rename targets. FR34 names `.stagecoach.toml`; §18.5 names `stagecoach/locks`.
```

### Current Codebase tree (relevant slice)

```bash
# Codebase root: /home/dustin/projects/stagehand   (module github.com/dustin/stagecoach; on-disk name unchanged)
internal/config/
  file.go             # globalConfigPath (L92-108) + repoLocalConfigPath (L125-127) + repoProviderNotice (L491) + doc L470  ← EDIT
  load.go             # bootstrap notice (L109) + Layer-3 comment (L118)                                                                            ← EDIT
  bootstrap.go        # template path comments (L244/246) — HEADER L236/238 UNCHANGED (P1.M2.T3.S2)                                                 ← EDIT (L244/246 only)
  file_test.go        # path/filename assertions (L36/157/162/169/236-302/331/381)                                                                  ← EDIT
  load_test.go        # path/filename assertions (L90/551-1497)                                                                                     ← EDIT
internal/lock/
  lock.go             # lockDir (L219/222/225/231) + HeldError (L51/60)                                                                            ← EDIT
  lock_test.go        # path assertions (L29/45/51/62/80)                                                                                          ← EDIT
go.mod / go.sum       # unchanged (module already stagecoach; this is source-content only)
```

### Desired Codebase tree with files to added

```bash
# NO new files. Edits to the 7 listed files IN PLACE. No structural change.
```

### Known Gotchas of our codebase & Library Quirks

```bash
# CRITICAL (scope boundary): rename ONLY path/filename literals + their doc/user-strings. LEAVE every OTHER
# stagehand ref to its owner: git-config keys stagecoach.* (S2); error prefixes load.go:520-527 + migrate.go
# (P1.M2.T3.S1); the bootstrap header bootstrap.go:236/238/153 (P1.M2.T3.S2); project-name prose config.go:14/27/42,
# load.go:66, lock.go:10, role_defaults.go:4 (P1.M2.T3/P1.M5); command-name strings load.go:201/216 (P1.M2.T3/P1.M3).
# Research §2 is the precise boundary. When a line has BOTH a path-ref AND a prefix (file.go:491, load.go:109),
# rename the WHOLE line (it's path-related); P1.M2.T3 no-ops it later.

# CRITICAL (no S2 overlap): S2's perl `(?<![.\/])stagehand\.` matches stagehand+DOT. This task's targets are
# `"stagehand"` (quote after), `.stagehand.toml` (S2's lookbehind preserves — preceded by `.`), `stagehand/…`
# (slash after), `stagehand:`/`stagehand ` (colon/space after). S2 provably doesn't touch them; this task
# provably doesn't touch stagecoach.* (already renamed). Disjoint by construction (research §3).

# CRITICAL (codebase location): the codebase is at /home/dustin/projects/stagehand (NOT /stagecoach). The plan
# cwd .../stagecoach has ONLY plan/. Verify: head -1 /home/dustin/projects/stagehand/go.mod → module github.com/dustin/stagecoach.
# Run ALL commands from /home/dustin/projects/stagehand.

# CRITICAL (sed scoping): scope sed 1 (`s/"stagehand"/"stagecoach"/g`) + sed 2 (`s/\.stagehand\.toml/.stagecoach.toml/g`)
# to the 5-7 TARGET files ONLY (file.go, lock.go, load.go, bootstrap.go, + file_test.go/load_test.go/lock_test.go).
# Do NOT run a repo-wide sed — it would touch P1.M2.T3/P1.M3/P1.M4 scope (docs, Makefile, .toml). The `"stagehand"`
# quoted literal appears ONLY in filepath.Join path contexts in these files (verified by grep).

# GOTCHA: sed 2 (`\.stagehand\.toml`) does NOT match `.stagehand/config.toml` (file.go:126 — has `/`, not `.toml`).
# Edit L126 manually (`.stagecoach/config.toml`). Also manually: the path DOC COMMENTS with `stagehand/config.toml`
# (file.go:92/94) and `stagehand/locks` (lock.go:219) — sed 1 catches only the QUOTED `"stagehand"`, not prose `stagehand/`.

# GOTCHA: file.go:95 "Stagehand config path" is project-name PROSE in the path doc. Update to "Stagecoach" to keep
# the doc coherent with the renamed path (it's IN the globalConfigPath doc block). This is the ONE prose site that
# rides with the path rename (it describes the renamed path); all OTHER prose (config.go, load.go:66, etc.) is P1.M2.T3.

# GOTCHA: bootstrap.go — edit ONLY L244/246 (the `.stagehand.toml` filename in the precedence comment). The HEADER
# (L236 `# Stagehand configuration file`, L238 `Generated by stagehand config init`, L153 `Stagehand behavior`) is
# P1.M2.T3.S2's "bootstrap config template" scope — LEAVE it. This is the keystone boundary proof: the SAME file has
# a path-ref (L244, mine) and header-prose (L236, P1.M2.T3.S2) on nearby lines.

# GOTCHA: the test files construct EXPECTED paths (`filepath.Join(…, "stagehand", …)`) + write `.stagehand.toml`
# fixtures. These MUST rename in lockstep with the source literals — else the tests fail (expected `stagecoach/…`,
# got `stagehand/…`). The seds handle both source + test mechanically.
```

## Implementation Blueprint

### Data models and structure

N/A — no types, no data models. A scope-disciplined path/filename string rename + test-assertion sync.

### Implementation Tasks (ordered by dependencies)

```yaml
Task 1: sed pass 1 — the "stagehand" directory literal (filepath.Join arg)
  - COMMAND (from /home/dustin/projects/stagehand):
    sed -i 's/"stagehand"/"stagecoach"/g' \
      internal/config/file.go internal/config/file_test.go internal/config/load_test.go \
      internal/lock/lock.go internal/lock/lock_test.go
  - CATCHES: file.go:102/108, lock.go:222/225/231, file_test.go:157/169/331/381, lock_test.go:29/45/62/80,
    load_test.go:90 (all filepath.Join `"stagehand"` args — verified the quoted literal is path-context-only).
  - WHY a sed: `"stagehand"` (quoted standalone) is unambiguous in these files; ~13 mechanical repeats.

Task 2: sed pass 2 — the .stagehand.toml filename
  - COMMAND:
    sed -i 's/\.stagehand\.toml/.stagecoach.toml/g' \
      internal/config/file.go internal/config/file_test.go internal/config/load.go internal/config/load_test.go \
      internal/config/bootstrap.go internal/lock/lock.go
  - CATCHES: file.go:127/470/491, file_test.go:36/237/238/280/302 (+comments), load.go:118, load_test.go:552/572/
    778/798/1042/1497, bootstrap.go:244/246. (Does NOT catch file.go:126 `.stagehand/config.toml` — manual.)
  - GOTCHA: scoped to the 6 files; does NOT touch docs/*.toml or providers/*.toml (P1.M4 scope).

Task 3: manual edits — the doc comments + error/notice/path-doc strings (unique sites)
  - file.go L92: `$XDG_CONFIG_HOME/stagehand/config.toml` → `stagecoach/config.toml` (doc).
  - file.go L94: `~/.config/stagehand/config.toml` → `stagecoach/config.toml` (doc).
  - file.go L95: `GLOBAL Stagehand config path` → `GLOBAL Stagecoach config path` (prose in path doc).
  - file.go L125: `the file ./.stagecoach.toml.` (already done by sed 2? L125 is a COMMENT `./.stagehand.toml` —
    sed 2 matches `.stagehand.toml` → `.stagecoach.toml`; verify it caught the comment too. If not, manual.)
  - file.go L126: `NOT arch §2.8's .stagehand/config.toml directory.` → `.stagecoach/config.toml` (manual — sed 2
    doesn't match `.stagehand/config`).
  - file.go L491: ensure the `stagehand:` prefix → `stagecoach:` (sed 2 did the `.stagehand.toml` part; the prefix
    is manual — the WHOLE string becomes `"stagecoach: repo-local config (.stagecoach.toml) sets provider…"`).
  - lock.go L51: `another stagehand process holds the lock` → `stagecoach process` (doc).
  - lock.go L60: `"stagehand run lock held by pid %s on %s"` → `"stagecoach run lock held…"` (error msg).
  - lock.go L219: `~/.cache/stagehand/locks` → `stagecoach/locks` (doc).
  - load.go L109: `"stagehand: wrote bootstrap config to %s\n"` → `"stagecoach: wrote bootstrap config to %s\n"`.
  - file_test.go L162: comment `home/.config/stagehand/config.toml` → `stagecoach` (manual comment).
  - lock_test.go L51: comment `~/.cache/stagehand/locks` → `stagecoach` (manual comment).
  - USE the `edit` tool with exact oldText for each (the strings are unique).

Task 4: VERIFY (gates 1-5 from research §5)
  - GATE 1: grep the 7 files for residual path/filename stagehand refs → EMPTY.
  - GATE 2: grep for the new stagecoach paths → non-empty.
  - GATE 3: S2's git-config keys intact (grep stagecoach. in git.go — not regressed).
  - GATE 4: go test ./internal/config/... ./internal/lock/... -count=1 → PASS.
  - GATE 5: go build ./... + go test ./... -count=1; bootstrap header STILL "Stagehand configuration file"
    (P1.M2.T3.S2's scope, untouched).

Task 5: FINAL scope audit
  - git diff --stat → only the 7 files (no docs/.toml/Makefile/.yml).
  - git diff --exit-code go.mod go.sum → unchanged.
  - Confirm the bootstrap.go boundary: L244 says `.stagecoach.toml` (renamed) AND L236 says `Stagehand
    configuration file` (UNCHANGED — P1.M2.T3.S2) — the keystone scope-split proof.
```

### Implementation Patterns & Key Details

```bash
# The two scoped seds (run from /home/dustin/projects/stagehand):
sed -i 's/"stagehand"/"stagecoach"/g' \
  internal/config/file.go internal/config/file_test.go internal/config/load_test.go \
  internal/lock/lock.go internal/lock/lock_test.go
sed -i 's/\.stagehand\.toml/.stagecoach.toml/g' \
  internal/config/file.go internal/config/file_test.go internal/config/load.go internal/config/load_test.go \
  internal/config/bootstrap.go internal/lock/lock.go

# GATE 1 (zero residual path/filename refs in the 7 files):
grep -rn '"stagehand"\|\.stagehand\.toml\|stagehand/config\|stagehand/locks\|stagehand run lock\|stagehand: wrote bootstrap\|stagehand: repo-local' \
  internal/config/file.go internal/config/load.go internal/config/bootstrap.go \
  internal/lock/lock.go internal/config/file_test.go internal/config/load_test.go internal/lock/lock_test.go
# EXPECT: empty.

# The keystone scope-split proof (bootstrap.go — path-ref renamed, header-prose untouched):
grep -n '\.stagecoach\.toml\|Stagehand configuration file' internal/config/bootstrap.go
# EXPECT BOTH: `.stagecoach.toml` (L244, renamed — mine) AND `Stagehand configuration file` (L236, UNCHANGED — P1.M2.T3.S2).
```

```bash
# BEFORE/AFTER (lock.go lockDir — the path literal + doc):
#   BEFORE:
#     // Resolution: XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagehand/locks.
#     return filepath.Join(xdg, "stagehand", "locks"), nil
#   AFTER:
#     // Resolution: XDG_RUNTIME_DIR → XDG_CACHE_HOME → ~/.cache/stagecoach/locks.
#     return filepath.Join(xdg, "stagecoach", "locks"), nil
# (sed 1 does the literal; the doc comment `stagehand/locks` is a manual edit — sed 1 catches only "stagehand".)
```

### Integration Points

```yaml
GO MODULE (go.mod / go.sum): NONE — pure source-content rename; module already stagecoach (M1). go mod tidy no-op.

PACKAGE EDGES: NONE — no import changes (M1 owned imports; Complete). The rename is string-literal/comment content only.

FROZEN / NOT-EDITED:
  - Git-config keys (stagecoach.*) — S2 (parallel, assume landed). This task's patterns don't match them.
  - Error-prefix strings (load.go:520-527, migrate.go:108-114) — P1.M2.T3.S1 ("Rename error message prefixes").
  - The bootstrap header (bootstrap.go:236/238/153) — P1.M2.T3.S2 ("bootstrap config template").
  - Project-name prose (config.go:14/27/42, load.go:66, lock.go:10, role_defaults.go:4) — P1.M2.T3 / P1.M5 audit.
  - Command-name strings (load.go:201/216 `commit-pi/stagehand`) — P1.M2.T3 / P1.M3 (binary name).
  - docs/configuration.md path refs — P1.M4.T1.S2 (Mode A docs ride with M4, NOT this task).
  - Non-config/lock files (Makefile, .goreleaser.yaml, providers/*.toml, .github/workflows) — P1.M3/P1.M4.
  - .stagecoachignore (the exclusion file) — P1.M2.T2.S2 (a SIBLING task; this task is paths + lock dirs only).

DOWNSTREAM / RELATED:
  - P1.M2.T2.S2: .stagehandignore → .stagecoachignore (the exclusion filename). SIBLING — different file/constant.
  - P1.M2.T3.S1/S2: the remaining user-facing strings (error prefixes, bootstrap header, session/temp prefixes).
  - P1.M4.T1.S2: docs/configuration.md path refs (the .md twin of this task).
  - P1.M5.T2.S1: final grep audit ("zero stagehand references in tracked files") — catches ALL residue (incl. the
    prose this task deliberately left for P1.M2.T3).

NO DATABASE / NO ROUTES / NO CONFIG LOGIC CHANGE (the path strings are data; discovery logic is unchanged).
```

## Validation Loop

### Level 1: Syntax & Style (Immediate Feedback)

```bash
cd /home/dustin/projects/stagehand
gofmt -l internal/config/ internal/lock/   # expect: empty (content-only rename; no structural change)
go vet ./...                                # expect: clean (string literals; no broken identifiers)
go build ./...                              # expect: success
git diff --exit-code go.mod go.sum && echo "go.mod/go.sum UNCHANGED (expected)"
# Confirm only the 7 files changed (no docs/.toml/Makefile/.yml):
git diff --name-only | grep -vE '^internal/(config|lock)/' && echo "BAD: file outside scope changed" || echo "only internal/config + internal/lock changed (good)"
```

### Level 2: Unit Tests (Component Validation) — the path-discovery gate

```bash
cd /home/dustin/projects/stagehand
go test ./internal/config/... ./internal/lock/... -count=1 -v
# Expected: PASS. globalConfigPath/lockDir return stagecoach/ paths; repoLocalConfigPath returns .stagecoach.toml;
# the test assertions construct the SAME stagecoach/ paths → match. -count=1 disables caching (forces the new paths).
go test ./... -count=1
# Expected: ALL PASS. A failure means a path literal was missed on one side (source vs test) — re-check GATE 1.
```

### Level 3: Integration Testing (System Validation) — the five scope gates

```bash
cd /home/dustin/projects/stagehand
go build -o /tmp/stagecoach ./cmd/stagecoach && echo "binary builds"
git diff --exit-code go.mod go.sum && echo "deps unchanged"
# GATE 1: zero residual path/filename stagehand refs in the 7 files:
grep -rn '"stagehand"\|\.stagehand\.toml\|stagehand/config\|stagehand/locks\|stagehand run lock\|stagehand: wrote bootstrap\|stagehand: repo-local' \
  internal/config/file.go internal/config/load.go internal/config/bootstrap.go \
  internal/lock/lock.go internal/config/file_test.go internal/config/load_test.go internal/lock/lock_test.go && echo "BAD: residual path ref" || echo "zero path/filename stagehand refs (good)"
# GATE 2: the new stagecoach paths are present:
grep -rn '"stagecoach"\|\.stagecoach\.toml' internal/config/ internal/lock/ | wc -l   # EXPECT: > 0
# GATE 3: S2's git-config keys intact (not regressed):
grep -c 'stagecoach\.' internal/config/git.go   # EXPECT: > 0 (S2's readers, untouched)
# GATE 4: tests pass (above, Level 2).
# GATE 5 / KEYSTONE: bootstrap.go — path-ref renamed, header-prose UNTOUCHED (the scope-split proof):
grep -n '\.stagecoach\.toml\|Stagehand configuration file' internal/config/bootstrap.go
# EXPECT BOTH: `.stagecoach.toml` (L244, renamed — mine) AND `Stagehand configuration file` (L236, UNCHANGED — P1.M2.T3.S2).
```

### Level 4: Creative & Domain-Specific Validation

```bash
cd /home/dustin/projects/stagehand
# Functional smoke (optional): confirm config discovery + lock dir resolve to stagecoach/ paths:
#   STAGECOACH_CONFIG=/tmp/x.toml ... (env already renamed by S1); or inspect GlobalConfigPath() output.
# The unit tests ARE the proof (they assert the exact stagecoach/ paths); this is belt-and-suspenders.
# Scope-boundary audit: confirm P1.M2.T3's prose scope is UNTOUCHED in the files I edited:
grep -n 'stagehand: config file has no config_version\|stagehand: config schema' internal/config/load.go internal/config/migrate.go   # EXPECT: still "stagehand:" (P1.M2.T3.S1's scope)
grep -n 'Stagehand configuration file\|Generated by .stagehand config init' internal/config/bootstrap.go   # EXPECT: still there (P1.M2.T3.S2)
# golangci-lint: make lint (project-wide — content-only rename; no lint drift).
```

## Final Validation Checklist

### Technical Validation

- [ ] Level 1 clean: `gofmt -l`, `go vet ./...`, `go build ./...`, `go mod tidy` no-op; only the 7 files changed.
- [ ] Level 2 green: `go test ./internal/config/... ./internal/lock/... -count=1` + `go test ./... -count=1`.
- [ ] Level 3: GATE 1 zero residual path refs; GATE 2 new paths present; GATE 3 S2 keys intact; GATE 5 keystone
      (bootstrap.go path-ref renamed + header-prose untouched).

### Feature Validation

- [ ] globalConfigPath returns `…/stagecoach/config.toml`; repoLocalConfigPath returns `.stagecoach.toml`.
- [ ] lockDir returns `…/stagecoach/locks`; HeldError.Error() says `stagecoach run lock held`.
- [ ] load.go bootstrap notice says `stagecoach: wrote bootstrap config…`.
- [ ] bootstrap.go:244/246 say `.stagecoach.toml` (and L236 header UNCHANGED).
- [ ] All test assertions updated; `go test ./internal/config/... ./internal/lock/...` green.

### Code Quality Validation

- [ ] Scope-disciplined: ONLY path/filename literals + their doc/user-strings renamed; P1.M2.T3's error-prefixes/
      bootstrap-header/prose UNTOUCHED; S2's git-config keys intact.
- [ ] Only internal/config/ + internal/lock/ files changed (no docs/.toml/Makefile/.yml).
- [ ] No overlap with S2 (disjoint patterns — research §3).
- [ ] Anti-patterns avoided (see below).

### Documentation & Deployment

- [ ] No docs edited here (Mode A docs/configuration.md path refs are P1.M4.T1.S2's scope).
- [ ] go.mod/go.sum byte-unchanged; no new files.

---

## Anti-Patterns to Avoid

- ❌ Don't run a REPO-WIDE sed. Scope sed 1 + sed 2 to the 5-7 TARGET files (internal/config/ + internal/lock/).
  A repo-wide `s/stagehand/stagecoach/g` would clobber P1.M2.T3 (error prefixes, prose), P1.M3 (Makefile,
  .goreleaser), P1.M4 (docs, providers/*.toml), AND S2's already-done git-config keys (no-op but risky).
- ❌ Don't rename git-config keys (`stagecoach.*`). Those are S2 (parallel, assume landed). This task's sed
  patterns (`"stagehand"` quoted, `.stagehand.toml`) do NOT match `stagecoach.*` — but a careless broad sed would.
  Verify GATE 3 (git.go still reads `stagecoach.*`).
- ❌ Don't rename the OTHER `"stagehand:"` error prefixes (load.go:520-527, migrate.go:108-114). Those are
  P1.M2.T3.S1 ("Rename error message prefixes"). This task owns ONLY the path-naming strings (lock.go:60,
  load.go:109, file.go:491). The line: path-related error/notice → mine; schema/migration/version notices → P1.M2.T3.S1.
- ❌ Don't rename the bootstrap HEADER (bootstrap.go:236 `# Stagehand configuration file`, L238 `Generated by
  \`stagehand config init\``, L153). Those are P1.M2.T3.S2 ("bootstrap config template"). This task touches ONLY
  L244/246 (the `.stagehand.toml` filename in the precedence comment). The keystone proof: L244 renamed, L236 untouched.
- ❌ Don't rename project-name PROSE (config.go:14/27/42 "Stagehand configuration", load.go:66, lock.go:10
  "no stagehand imports", role_defaults.go:4). Those are P1.M2.T3 / the P1.M5 final audit. The ONE exception is
  file.go:95 ("Stagehand config path") — it's IN the globalConfigPath doc block describing the renamed path, so
  update it to keep the doc coherent.
- ❌ Don't work in `/home/dustin/projects/stagecoach` — that's the plan-staging dir (only `plan/`). The codebase
  is at `/home/dustin/projects/stagehand` (module already `github.com/dustin/stagecoach`). Verify with `head -1 go.mod`.
- ❌ Don't forget the TEST files. The path literals + the test assertions MUST rename in lockstep (the tests
  construct expected `stagecoach/…` paths + write `.stagecoach.toml` fixtures). Missing a test assertion → the
  test fails (expected `stagecoach`, got `stagehand`). The seds handle source + tests mechanically.
- ❌ Don't miss file.go:126 (`.stagehand/config.toml` — a rejected-alt-design dir ref). sed 2 (`.stagehand.toml`)
  does NOT match it (it has `/`, not `.toml`). Edit it manually → `.stagecoach/config.toml`.
- ❌ Don't miss the path DOC COMMENTS (file.go:92/94 `stagehand/config.toml`, lock.go:219 `stagehand/locks`).
  sed 1 catches only the QUOTED `"stagehand"` literal, not prose `stagehand/` in a comment. Edit those manually.
- ❌ Don't touch docs/configuration.md or any non-.go file. docs/configuration.md path refs are P1.M4.T1.S2
  (Mode A docs ride with M4). The `--include` scoping (or the explicit 7-file sed list) handles this.
- ❌ Don't change go.mod/go.sum or add files. This is a source-content rename (7 existing files); no dep change.
- ❌ Don't skip the five verification gates. GATE 1 (zero residual) + GATE 5 (keystone: bootstrap.go path-ref
  renamed + header untouched) together PROVE the rename was both complete AND scope-disciplined. Skipping leaves
  a silent overlap with P1.M2.T3 or a missed test assertion.
- ❌ Don't conflate this with P1.M2.T2.S2 (`.stagehandignore` → `.stagecoachignore`). That's a SIBLING task (the
  exclusion filename + its constant); THIS task is config/lock PATHS + the `.stagecoach.toml` config filename.

---

## Confidence Score

**9/10** — a mechanical, well-mapped path/filename rename with a verified complete site map (research §1 — every
file:line + old→new, source + tests), a provably-disjoint-from-S2 boundary (the patterns can't collide), a
precise P1.M2.T3 scope split (path-naming strings mine; error-prefixes/bootstrap-header/prose theirs), and five
verification gates that prove both completeness and scope discipline. The two scoped seds handle the ~25
mechanical repeats; ~10 manual edits handle the unique prose/error/path-doc sites. The -1 reserves for the
manual-edit diligence (a missed doc comment like file.go:92/94 or lock.go:219 would leave a stale `stagehand/`
in a comment — GATE 1's grep catches it, but the implementer must act on it) and the slim chance a test asserts
a path via a mechanism the seds don't catch (e.g. a helper-built path).
