# P1.M4.T4.S1 Research — Lock/Watchdog Documentation Accuracy Review

## 0. The task in one paragraph
A **documentation-sync / review** task. The lock/watchdog fixes (P1.M4.T1–T3) are **all code-only**
(one defense-in-depth re-read gate in `reapStaleLocks` + two comment-only rewrites). They deliberately
left README.md / docs/ untouched (T1/T2 committed; T3's PRP is Mode-A = code comment only). This task
reviews the user-facing docs for accuracy about lock status display, orphan detection, and stale-file
reaping, and refines any mismatch so docs match the fixed code. **Finding: the docs are ~99% accurate;
the single genuine mismatch is `docs/cli.md:400`'s explanation of the `orphaned:` hint.**

## 1. What T1/T2/T3 changed (code-only — NO user-visible behavior change)
| Task | Bug | File | Change | User-facing? |
|------|-----|------|--------|--------------|
| P1.M4.T1.S1 | BUG-009 | `internal/lock/lock.go` (+2 tests) | re-read-before-remove gate `staleLockUnchanged` in `reapStaleLocks` — dead-pid `*.lock` files STILL reaped (gate only skips when a concurrent acquirer rewrote the file). Defense-in-depth. | NO |
| P1.M4.T2.S1 | BUG-010 | `internal/lock/lock_windows.go` | `processAlive` doc-comment rewrite (body `return true` UNCHANGED). Corrects the false "files aren't created on Windows" claim **in the code comment**. | NO |
| P1.M4.T3.S1 | BUG-011 | `internal/lock/orphan_unix.go` | `appearsOrphaned` doc-comment rewrite (body `return ppid == 1` UNCHANGED). Documents the subreaper false-negative + cross-refs `arm_unix.go` watchdog (FR-K2). Mode A = code comment only. | NO |

Git state (verified): `7cfca67 fix(lock): narrow TOCTOU window in stale reaper` (T1) and
`6bef7f6 docs(lock): correct Windows processAlive no-op comment` (T2) are COMMITTED. T3's
`orphan_unix.go` strengthened comment is present in the working tree (uncommitted).

## 2. Doc-by-doc accuracy audit (grep `lock|orphan|watchdog|reap|stale|processAlive|flock|getppid|subreaper|parent.?pid`)

### ACCURATE — no edit needed
- **README.md:360** — `stagecoach lock status # inspect the per-repo run lock holder (path, pid, liveness, orphan status)` ✓
- **README.md:406** — contention / CAS description ✓
- **README.md:408** — "the orphaned run self-exits via a parent-death watchdog and releases the lock … `lock status` shows the holder's path and liveness … never force-breaks a live lock." ✓ (describes the watchdog self-exit + status; does NOT overstate the hint)
- **docs/cli.md:387** — `lock status` description ("whether the holder process is alive, and — on Unix — whether it appears orphaned (reparented)") ✓
- **docs/cli.md:388–398** — the `Lock:` output EXAMPLE matches `internal/cmd/lock.go` byte-for-byte (field names, the `orphaned: true (holder reparented — launcher has exited)` string, the snapshot "only shown once the snapshot is armed" comment = the `if contents.Snapshot != ""` guard) ✓
- **docs/how-it-works.md:211** — "Orphaned lock FILES … are reaped by pid-liveness on the next Acquire" ✓ (T1's gate still reaps dead-pid files)
- **docs/how-it-works.md:220** — "On Windows, `flock` is a no-op stub, reaping is a no-op too, and the CAS is the guarantee there." ✓ (does NOT repeat the false "files aren't created" claim — T2's code-comment correction needs NO doc fix)
- **docs/how-it-works.md:222** — "The child is reparented to init (or a subreaper)" ✓ (even names subreapers)
- **docs/how-it-works.md:224** — watchdog "Detection is by **parent-pid change** … not the brittle `getppid==1` test — subreaper-safe" ✓ (correctly attributes parent-pid-CHANGE to the WATCHDOG, FR-K2)
- **docs/how-it-works.md:228** — "`lock status` prints … whether it appears orphaned (reparented). It is read-only" ✓
- **docs/configuration.md:121,158,222,251,292–302** — `no_parent_watchdog` opt-out + lock-file location ✓
- **docs/windows-test-support.md:55** — "flock contention (`lock_windows.go` is a documented no-op; the CAS is the safety guarantee)" ✓
- **NO doc repeats the false "lock files aren't/never created on Windows" claim** (grep for `not created|aren't created|isn't created|never created` ⇒ only `cli.md:449` "commit not created" — a rescue-condition false positive, unrelated to lock files).

### THE ONE MISMATCH — `docs/cli.md:400` (the `orphaned:` outcomes paragraph)
**Current:** "… `true (holder reparented — launcher has exited)` (Unix; **the holder's parent pid changed** — its launcher closed without killing it), `false` (**alive and not reparented** — Windows always lands here) …"

**Why it's wrong:** it attributes "parent pid changed" to the `orphaned:` HINT, but the hint
(`internal/lock/orphan_unix.go:appearsOrphaned`) actually tests `ppid == 1` (reparented to **init**).
The parent-pid-**change** test is what the **watchdog** (`internal/watchdog/arm_unix.go:
osGetppid() != originalPpid`, FR-K2) uses. Under a subreaper (systemd/Docker/supervisord) a reparented
orphan's ppid is the subreaper's pid (≠ 1) ⇒ the hint reads `false` even though its parent pid *did*
change. So:
- "the holder's parent pid changed" **overstates** the hint (it only catches reparenting-to-init).
- "alive and not reparented" is **wrong** for the subreaper case (the holder IS reparented, just not to init).

This is exactly the imprecision the contract's "If docs reference the orphan hint … verify they match the
fixed code" targets. T3 documented the limitation in the code comment; the user-facing doc (cli.md:400)
still conflates the hint with the watchdog mechanism.

## 3. The fix (single file, docs/cli.md)
Rewrite the `orphaned:` outcomes paragraph to (a) describe the hint's actual mechanism (`ppid == 1` ⇒
reparented to init), (b) NOT conflate it with the watchdog's parent-pid-change, (c) add a brief
display-only subreaper false-negative note (consistent with how-it-works.md:222 which already names
subreapers), and (d) cross-ref the authoritative watchdog in how-it-works.md. The output EXAMPLE
(cli.md:388–398) and the command description (cli.md:387) are UNCHANGED — they already match the code.

## 4. Validation approach (doc-only task)
- `.markdownlint.json` exists: `{default:true, MD013:false, MD033:false, MD060:false}` → line-length OFF,
  but default rules ON (no trailing spaces, balanced emphasis, etc.). Run markdownlint on docs/cli.md.
- Grep guards: prove (a) no false "files aren't created" claim anywhere, (b) the watchdog prose still
  names parent-pid-change + subreaper-safe, (c) the Windows no-op prose is intact, (d) cli.md:400 now
  describes `ppid == 1`/init + subreaper + watchdog cross-ref.
- `go build ./...` sanity (docs can't break it, but confirm). `make test`/`make lint` untouched by a doc
  change; run to prove no collateral.
- Git scope guard: `docs/cli.md` ONLY (README.md + how-it-works.md verified accurate, NOT edited).

## 5. Scope fences
- Touches ONLY: `docs/cli.md` (the `orphaned:` outcomes paragraph at line ~400).
- Does NOT touch: README.md (accurate), docs/how-it-works.md (accurate), docs/configuration.md,
  docs/windows-test-support.md, any `*.go` file, any `*_test.go`, any `spec/*.md` (AGENTS.md rule #2),
  PRD.md, tasks.json, prd_snapshot.md, go.mod.
- Parallel-safe: P1.M4.T3.S1 (orphan_unix.go, code comment) is the working-tree change; its Mode-A
  contract forbids doc edits — zero overlap with this doc-only item.