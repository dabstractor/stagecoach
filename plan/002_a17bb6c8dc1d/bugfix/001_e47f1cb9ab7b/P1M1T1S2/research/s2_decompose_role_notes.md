# S2 Research Notes — Fix all four decompose role Render calls

Scope: apply the same provider/sub-provider deconflation (Issue 1, S1 = generate.go, commit `9ff53e6`)
to the four decompose role files. S2 is the **decompose half** of P1.M1.T1.

## 1. The four exact call sites (grep-confirmed line numbers)

Each role file derives `(prov, mdl)` via `config.ResolveRoleModel` then passes `prov` to Render.
`prov` is the **manifest name** (registry key, e.g. `"pi"`), NOT the sub-provider — same bug as
generate.go. After the fix `prov` is unused at the Render call.

| File | Decl (prov,mdl) | Render call | Render mode |
|---|---|---|---|
| `internal/decompose/planner.go` | L61 | L91 `deps.Roles.Planner.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)` | Bare |
| `internal/decompose/stager.go`  | L60 | L66 `deps.Roles.Stager.Render(mdl, prov, "", task, provider.RenderTooled)` | Tooled |
| `internal/decompose/message.go` | L102 | L122 `deps.Roles.Message.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)` | Bare |
| `internal/decompose/arbiter.go` | L81 | L91 `deps.Roles.Arbiter.Render(mdl, prov, sysPrompt, payload, provider.RenderBare)` | Bare |

## 2. `prov` is used ONLY at the Render call in every file (grep-confirmed)

`grep -n "prov"` per file shows: the declaration line + exactly ONE Render use. No other reference.
⇒ After changing Render's provider arg to `""`, `prov` is unused → Go compile error
("declared but not used"). **Fix: change `prov, mdl :=` → `_, mdl :=`** at each of the 4 declaration
lines. This is the contract-sanctioned option (vs `_ = prov`). `mdl` is still used by Render.

`ResolveRoleModel`'s `prov` return value is STILL correct and required for `reg.Get(prov)` inside
`ResolveRoles` (`internal/decompose/roles.go:113`) — that usage is UNTOUCHED (it's a different call
site; S2 does not edit roles.go). Only the 4 Render-call files change.

## 3. S1 convention to mirror (generate.go, commit 9ff53e6)

```go
// Pass "" for the sub-provider: cfg.Provider is the manifest/agent NAME (the registry key,
// e.g. "pi"), NOT the upstream backend. Render resolves the real sub-provider from the
// manifest's merged DefaultProvider (FR37a) — emitting "--provider <DefaultProvider>", or
// omitting --provider when DefaultProvider is unset (pi's shipped default, §12.3).
spec, rerr := deps.Manifest.Render(cfg.Model, "", sysPrompt, payload)
```
Difference for S2: the value comes from `ResolveRoleModel` (not `cfg.Provider`), so the comment says
"ResolveRoleModel returns the manifest name". Same `""` token, same Render fallback reliance.

## 4. Render fallback that makes `""` correct (render.go — NOT edited)

```go
providerToUse := provider
if providerToUse == "" { providerToUse = *r.DefaultProvider }   // fires now that we pass ""
if *r.ProviderFlag != "" && providerToUse != "" {
    args = append(args, *r.ProviderFlag, providerToUse)          // → "--provider openrouter"
}
```
Render internally calls `m.Resolve()` (reads the manifest's merged DefaultProvider/ProviderFlag). So
the merged manifest on `deps.Roles.X` supplies the sub-provider. Confirmed in
`issue1_provider_conflation.md` (render.go/merge.go/roles.go need NO changes).

## 5. Test infrastructure (per role + full loop)

Per-role `Deps` helpers (each sets `Roles: RoleManifests{<Role>: m}`, `Config: config.Defaults()`,
`Verbose: nil`):
- `plannerDeps(t, repo, m)` — planner_test.go:55  → call `callPlanner(ctx, deps, forcedCount, isUnborn)`.
- `messageDeps(t, repo, m)` — message_test.go:71  → call `generateMessage(ctx, deps, treeA, treeB)` (needs 2 tree SHAs).
- `arbDeps(t, repo, m)`     — arbiter_test.go:66  → call `runArbiter(ctx, deps, commits, leftoverDiff)`.
- **No `stagerDeps`** — stager.go:stageConcept is exercised either directly (call `stageConcept(ctx, deps,
  concept)`) or via the full `Decompose` loop with the `deps.stager` test seam (decompose.go:invokeStager).

Full-loop helpers (decompose_test.go):
- `dcmDeps(t, repo, roles)` / `dcmDepsWithConfig(t, repo, roles, cfg)` — populate ALL four roles.
- `dcmStagerSeam(t, repo, conceptFiles)` — injects a stager that actually runs `git add` (the stubtest
  agent can't run git) — used by `TestDecompose_AutoMultiCommit_HappyPath` / `_Overlap` / etc.
- `dcmOutBuffer(t, repo, roles)` — returns `(Deps, *bytes.Buffer)` wiring `deps.Out`.

stubtest builders (internal/stubtest/stubtest.go): `Build(t) string`, `Manifest(bin, Options) Manifest`,
`NewScript(t, bin, []string) Manifest`. `Options{Out, Script, Counter, Exit, SleepMS}`. The returned
`provider.Manifest` has EXPORTED `*string`/`*bool` fields — set pi-shaped via `&localVar` (provider.strPtr
is unexported; cross-package can't call it). **Same trick S1 used.**

## 6. Back-compat (existing tests must stay byte-identical)

stubtest default manifest sets NO `ProviderFlag`/`DefaultProvider`. So today (pre-fix) the 4 role files
call `Render(mdl, prov, ...)` with `prov` from `config.Defaults()` (Provider="") → ResolveRoleModel
returns ("","") → `Render(mdl, "", ...)` → Render omits `--provider` (no DefaultProvider). After the fix
the call is `Render(mdl, "", ...)` → identical. ⇒ **All existing decompose tests are byte-identical and
must pass unchanged.** (The bug only manifests when a manifest HAS DefaultProvider + the caller passes a
non-empty manifest-name `prov`, which only happens with a pi-shaped manifest + cfg.Provider="pi".)

## 7. -race consideration for the END-TO-END decompose test

The full `runLoop` overlaps `stager[i+1]` (Render→Execute→VerboseCommand) with `message[i]` in a
goroutine (Render→Execute→VerboseCommand). Both write to the SAME `deps.Verbose` sink
(`fmt.Fprintln(v.w, ...)` → `*bytes.Buffer`). `bytes.Buffer` is NOT concurrency-safe ⇒ a Verbose-on E2E
loop test would FAIL `go test -race`. Existing overlap tests set `Verbose: nil` (nil-safe no-op) so they
never hit this.

**Resolution for the E2E test:** either (a) pass a **thread-safe** `io.Writer` to `ui.NewVerbose` (a tiny
`lockedWriter{mu sync.Mutex; b bytes.Buffer}`) so concurrent writes are safe, OR (b) make the E2E test
assert only on the **planner**, which renders SYNCHRONOUSLY before `runLoop` starts (no concurrency for
that one write). The 4 per-role unit tests are SYNCHRONOUS (no goroutines) → race-free by construction
and are the PRIMARY regression guard. Recommend (a) for a faithful E2E + the 4 synchronous unit tests.

## 8. Scope boundary (per plan_status)

- S2 = 4 production Render-call fixes (prov→""; prov,mdl→_,mdl) + 4 inline comments + tests.
  Decompose role files ONLY.
- S2 must NOT edit: `internal/generate/*` (S1, done), `internal/provider/render.go`/`merge.go`
  (already correct), `internal/config/roles.go` (ResolveRoleModel correct for reg.Get),
  `internal/decompose/roles.go` (ResolveRoles — reg.Get(prov) usage correct, untouched), the CLI,
  `PRD.md`, `tasks.json`, `prd_snapshot.md`.
- The E2E CLI integration test is P1.M1.T2.S1 (separate) — S2 adds LIBRARY-level (decompose package)
  tests only, not a CLI binary test.
