# Research: Rename .stagehandignore → .stagecoachignore residue (P1.M2.T2.S2)

> The `.stagehandignore` exclusion filename is the LAST rename residue in the **Go source**. A prior bulk
> rename (M1.T2.S1's `stagehand → stagecoach` identifier pass) already converted the constant value +
> `internal/exclude/exclude.go` + `exclude_test.go` in full. **The item description's premise is STALE**: it
> says "edit `exclude.go:28` `.stagehandignore` → `.stagecoachignore`" — but `exclude.go:28` ALREADY reads
> `const StagecoachIgnoreFile = ".stagecoachignore"` and the whole package doc + every comment say
> `.stagecoachignore`. The REAL remaining work is **exactly 2 `.go` sites** the bulk pass missed.

## §0 — REALITY CHECK: the item's premise is stale (exclude.go is already done)

Verified live (`cd /home/dustin/projects/stagehand; grep -n ... internal/exclude/exclude.go`):

```
27: // StagecoachIgnoreFile is the fixed repo-root filename (PRD §9.18 FR-X1b/FR-X2).
28: const StagecoachIgnoreFile = ".stagecoachignore"   ← ALREADY .stagecoachignore (NOT .stagehandignore)
```

- `grep -c 'stagehandignore' internal/exclude/exclude.go` → **0** (zero residue; fully renamed).
- `exclude_test.go` uses the `StagecoachIgnoreFile` const + asserts `.stagecoachignore` strings (lines 99, 151,
  161, 162, 193, 223) → **already renamed; tests pass as-is**.

⇒ Do NOT edit `exclude.go` (it's done). The item's "INPUT: the const NAME already renamed; now its VALUE must
change" describes a state that no longer exists — the value changed too (M1.T2.S1's broad `s/stagehand/stagecoach/g`
caught string literals + comments, not just identifiers). The only thing M1 missed is the 2 sites below.

## §1 — The ACTUAL residue: exactly 2 `.go` sites

`grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'` → exactly:

### Site 1 — `internal/cmd/root.go:164-165` (the `--exclude` flag help text)

```go
pf.StringArrayVarP(&flagExclude, "exclude", "x", nil,
    "Exclude matching files from the agent payload (unions with .stagehandignore and "+
        "[generation].exclude; never excluded from the commit)")
```

`stagehandignore` → `stagecoachignore` in the user-facing `--exclude` help string (printed by `--help`).

### Site 2 — `internal/ui/verbose.go:101` (the `VerboseWarn` doc comment)

```go
// VerboseWarn prints a general warning for diagnostics such as unsupported .stagehandignore
// negation patterns (PRD §9.18 FR-X2). Format: "DEBUG: <msg>\n". No-op when v==nil, v.w==nil, or !v.on.
```

`stagehandignore` → `stagecoachignore` in the doc comment (the runtime string it prints comes from
`exclude.go:85`'s `VerboseWarn("ignoring unsupported negation pattern in .stagecoachignore: " + line)` —
already renamed; only THIS comment is stale).

**That is the entire `.go` surface.** No other `.go` file references `stagehandignore`.

## §2 — Scope: `.go`-ONLY. Docs are deferred to M4 (P1.M4.T1)

The item's LOGIC (b) sed is scoped `--include='*.go'`. Its DOCS line: "Mode A — update README.md and docs/cli.md
`.stagecoachignore` references **in M4**." ⇒ docs are **P1.M4.T1's scope, NOT this task's.** Do NOT touch:

- `README.md:66` — `.stagehandignore` in the feature table.
- `docs/cli.md:37`, `docs/README.md:34/42`, `docs/how-it-works.md:156/158/162`,
  `docs/configuration.md:253/255/260/277` — all `.stagehandignore` doc references.

(The `bin/stagehand`, `bin/stagehand-test`, root `stagehand`/`stagecoach` "matches" are compiled build
artifacts — not source; ignore them. They rebuild clean from the renamed source.)

## §3 — No test impact (the rename is safe)

`grep` confirms **no test asserts on either residue string**:
- No cmd test pins the `--exclude` help text containing `.stagehandignore` (the only `.go` hit is `root.go:164`
  itself — the source, not a test).
- No test pins the `VerboseWarn` doc comment.
- `exclude_test.go` already uses `.stagecoachignore` → `go test ./internal/exclude/... -count=1` passes BEFORE
  this task (and after — the 2 edits don't touch exclude at all).

So the 2 edits are comment/help-text-only → zero behavioral change → zero test regression.

## §4 — Disjoint from the parallel S1 (P1.M2.T2.S1, config/lock paths)

S1 renames config-discovery PATHS (`internal/config/file.go`, `load.go`, `bootstrap.go`, `*_test.go`) + lock
PATHS (`internal/lock/lock.go`, `lock_test.go`). This task (S2) renames the EXCLUSION filename in
`internal/cmd/root.go` + `internal/ui/verbose.go` (and would have touched `internal/exclude/` if it weren't
already done). **Zero file overlap** with S1 — the two land independently. S1's PRP explicitly listed
`.stagecoachignore` as "P1.M2.T2.S2 (a SIBLING task)" — confirming the split.

## §5 — Mechanism + verification

- **Mechanism (either works):**
  - Scoped sed on the 2 files: `sed -i 's/stagehandignore/stagecoachignore/g' internal/cmd/root.go internal/ui/verbose.go`
    (catches both sites; `stagehandignore` is unambiguous — it appears NOWHERE else in `.go`).
  - OR the item's grep|xargs form (auto-discovers exactly these 2 files):
    `grep -rl 'stagehandignore' --include='*.go' . | grep -v '.git/' | xargs sed -i 's/stagehandignore/stagecoachignore/g'`
    (verified: the grep yields exactly `internal/cmd/root.go` + `internal/ui/verbose.go`).
  - OR two precise `edit` calls (one per site) — safest for a 2-site change.
- **Verification:**
  - `grep -rn 'stagehandignore' --include='*.go' . | grep -v '.git/' | grep -v '.pi-subagents'` → **EMPTY** (zero `.go` residue).
  - `gofmt -l internal/cmd/root.go internal/ui/verbose.go` → clean (content-only; no structural change).
  - `go build ./...` + `go vet ./...` → clean.
  - `go test ./internal/exclude/... -count=1` → PASS (already passes; the edits don't touch exclude).
  - `go test ./... -count=1` → green (no regression; comment/help-text only).
  - go.mod/go.sum byte-unchanged; only `internal/cmd/root.go` + `internal/ui/verbose.go` touched.

## §6 — Codebase location note

The codebase is at **`/home/dustin/projects/stagehand`** (on-disk dir name unchanged from the original; the
Go MODULE path is already `github.com/dustin/stagecoach` — `head -1 go.mod` confirms). The plan-staging cwd
`/home/dustin/projects/stagecoach` holds only `plan/`. **Run ALL commands from `/home/dustin/projects/stagehand`.**
(This matches S1's note; the rename hasn't moved the on-disk repo.)
