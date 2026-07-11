# Research: P1.M2.T1.S1 — Blank pi models in the commented-provider block loop (Issue 2, FR-R5b/FR-B1)

Fix the commented-out pi provider block in `config init` output so it no longer ships BARE models
(`gpt-5.4*`) that are a hard error on pi under FR-R5b. When uncommented (the documented FR-B1 workflow),
the block must produce a VALID config. Mirror the existing target=="pi" active-block blanking: write
`# model = ""` for all four roles + emit a multi-backend guidance comment. Pure production-code fix in
`internal/config/bootstrap.go` + one docs line in `docs/providers.md`. The dedicated test is the separate
P1.M2.T1.S2 subtask.

All claims verified against the current tree (2026-07-10, post-P1.M1.T1 landing).

---

## 0. THE BUG (Issue 2)

`buildBootstrapConfig`'s commented-provider loop emits a `# [role.*]` group for each OTHER installed
provider. For pi it calls `DefaultModelsForProvider("pi")` and writes the bare models verbatim:

```
# === pi (installed) — uncomment a [role.*] block to route that role to pi ===
# [role.planner]
# provider = "pi"
# model = "gpt-5.4"          ← BARE: hard error on pi under FR-R5b if uncommented
...
```

These are commented so they don't error at LOAD — but FR-B1 says uncommenting should yield a WORKING
config. A user who uncomments the pi block hits `provider render "pi": model "gpt-5.4-nano" on pi must
be inference/model`. Contrast: the opencode commented block is fine (`openai/gpt-5.4` is prefixed). The
bug is pi-specific (pi's role-default models are bare strings; opencode's are already prefixed).

---

## 1. THE FIX SITE — `internal/config/bootstrap.go`, the commented-provider loop (~lines 226-240)

Current code (locate by `grep -n 'preferredBuiltins' internal/config/bootstrap.go` — line numbers drift):

```go
// other installed providers as COMMENTED [role.*] groups
for _, name := range preferredBuiltins {
    if name == target || !isInstalledName(name, installed) {
        continue
    }
    other := DefaultModelsForProvider(name)
    if other == nil {
        continue
    }
    b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
    writeCommentedRoleBlock(&b, "planner", name, other["planner"])
    writeCommentedRoleBlock(&b, "stager", name, other["stager"])
    writeCommentedRoleBlock(&b, "message", name, other["message"])
    writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])
}
```

### The fix (mirror the active-block target=="pi" blanking)

After `other := DefaultModelsForProvider(name)` and the nil check, detect pi: blank `other` in place and
emit the guidance comment AFTER the `# === pi (installed) ===` header, BEFORE the writeCommentedRoleBlock
calls. Cleanest structure:

```go
    other := DefaultModelsForProvider(name)
    if other == nil {
        continue
    }
    piCommented := name == "pi"
    if piCommented {
        // pi is a multi-backend provider (FR-R5b): a bare model (no '/') is a hard error. Blank the
        // commented models so uncommenting yields a valid (model-less) config the user fills in —
        // mirroring the target=="pi" active-block blanking above. (other is a fresh per-call copy from
        // DefaultModelsForProvider, so this mutation is isolated — see §2.)
        for role := range other {
            other[role] = ""
        }
    }
    b.WriteString("\n# === " + name + " (installed) — uncomment a [role.*] block to route that role to " + name + " ===\n")
    if piCommented {
        b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
        b.WriteString("# e.g. model = \"zai/gpt-5.4\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
    }
    writeCommentedRoleBlock(&b, "planner", name, other["planner"])
    writeCommentedRoleBlock(&b, "stager", name, other["stager"])
    writeCommentedRoleBlock(&b, "message", name, other["message"])
    writeCommentedRoleBlock(&b, "arbiter", name, other["arbiter"])
```

Result for the pi commented block:
```
# === pi (installed) — uncomment a [role.*] block to route that role to pi ===
# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,
# e.g. model = "zai/gpt-5.4". A bare model (no "/") on pi is a config error (FR-R5b).
# [role.planner]
# provider = "pi"
# model = ""
...
```

### `writeCommentedRoleBlock` (the emission seam — NO change needed)

```go
func writeCommentedRoleBlock(b *strings.Builder, role, prov, model string) {
    fmt.Fprintf(b, "# [role.%s]\n", role)
    fmt.Fprintf(b, "# provider = %q\n", prov)
    fmt.Fprintf(b, "# model = %q\n", model)   // blank model → renders `# model = ""`
}
```

`%q` on `""` renders `""` → `# model = ""`. Correct. Do NOT modify this helper.

---

## 2. COPY-SEMANTICS — mutating `other` is SAFE (verified)

`DefaultModelsForProvider` (role_defaults.go) returns a FRESH defensive copy:

```go
func DefaultModelsForProvider(name string) map[string]string {
    if col, ok := roleDefaults[name]; ok {
        out := make(map[string]string, len(col)) // fresh map ≠ roleDefaults[name]
        for role, model := range col { out[role] = model }
        return out
    }
    return nil
}
```

So `for role := range other { other[role] = "" }` mutates ONLY the per-call copy — it does NOT corrupt the
package-level `roleDefaults` table. The active-block code already relies on this exact discipline
(`models := DefaultModelsForProvider(target); for role := range models { models[role] = "" }`). The
commented-loop fix reuses the identical, already-proven pattern.

---

## 3. THE TEMPLATE TO MIRROR — the active-block target=="pi" blanking (~bootstrap.go:162-175)

The active block already solves this for the target provider. The commented-loop fix is its twin:

```go
models := DefaultModelsForProvider(target)
piBlanked := target == "pi"
if piBlanked {
    for role := range models { models[role] = "" }   // ← the exact blank to mirror
}
...
if piBlanked && !piHasOverrides {
    b.WriteString("# NOTE: pi is a multi-backend provider — prefix the model with your inference backend,\n")
    b.WriteString("# e.g. model = \"zai/glm-5.2\". A bare model (no '/') on pi is a config error (FR-R5b).\n")
    b.WriteString("# The shipped per-role models are empty so you can supply your own backend/model.\n")
}
```

(The active block uses `zai/glm-5.2`; the contract specifies `zai/gpt-5.4` for the commented block —
gpt-5.4 is the actual pi role-default model, so it's the relevant example for the pi block. Use the
contract's wording.)

---

## 4. DOCS — `docs/providers.md` (~line 125, Mode A)

Current sentence (in the "Per-role default models" section):
> The config bootstrap (`config init`) uses these defaults — EXCEPT for **pi**, whose per-role models
> are written EMPTY (pi needs an inference-provider prefix on the model, FR-R5b; its shipped per-role
> models are blank so you supply backend/model, e.g. `zai/gpt-5.4`).

Extend to clarify BOTH the active AND commented pi blocks are blanked:
> The config bootstrap (`config init`) uses these defaults — EXCEPT for **pi**, whose per-role models
> are written EMPTY in BOTH the active `[role.*]` block AND the commented-out pi block (pi needs an
> inference-provider prefix on the model, FR-R5b; its shipped per-role models are blank so you supply
> backend/model, e.g. `zai/gpt-5.4`).

(Locate by `grep -n 'written EMPTY' docs/providers.md`. The change is inserting "in BOTH the active
`[role.*]` block AND the commented-out pi block" into the existing sentence.)

---

## 5. TEST-IMPACT — no existing test breaks (verified by scout)

- **No test positively asserts a commented pi block contains `gpt-5.4*`.** The commented-block tests:
  - `TestBuildBootstrapConfig_OtherInstalledCommented` (target="pi", installed={pi,claude}) — asserts
    the CLAUDE commented block; pi is the TARGET so no pi commented block is emitted. Unaffected.
  - `TestBuildBootstrapConfig_Pi` (installed={pi}) — pi is target AND only install → commented loop
    emits nothing. Its `strings.Contains(content, "gpt-5.4")` negative guard still passes (no commented
    pi block exists). Unaffected.
- **`TestBuildBootstrapConfig_ValidTOML`** has cases that install pi as a NON-target:
  `{"claude",["claude","pi"]}` and `{"agy",["agy","pi","claude"]}`. After the fix the commented pi
  block becomes `# model = ""` + `#` guidance lines — ALL inert TOML comments → `toml.Unmarshal` still
  succeeds. **No test edit required.**
- The fix breaks ZERO existing assertions. The dedicated commented-pi-block test is P1.M2.T1.S2.

### For the S2 author (note, not this task):
The S2 test must use a NON-pi target with pi installed (e.g. `target="claude", installed={"claude","pi"}`)
so the commented pi block is actually emitted. The existing `target="pi"` tests skip it (pi is the target).

---

## 6. PARALLEL-OVERLAP & SCOPE FENCES

- **Parallel sibling P1.M1.T2.S1** adds `internal/config/bootstrap_validate_test.go` — a `ValidateModel`
  regression net over ACTIVE role models ONLY. It does NOT touch bootstrap.go production code and does
  NOT cover commented blocks (commented TOML is inert → never decoded → ValidateModel can't reach it).
  Its own doc comment (bootstrap_validate_test.go:58) says so explicitly. NO overlap, NO conflict.
- **`exampleConfigTemplate` (internal/cmd/config.go:537)** is a SEPARATE inert doc with its own content.
  It does NOT have the pi/gpt-5 bug (zero `gpt-5` references; its `[role.*]` examples use agy + blanks).
  OUT OF SCOPE — do NOT touch it (it has a byte-equality golden test at config_test.go:438).
- **Active-block paths** are already fixed (P1.M1.T1.S1 — the stager-fallback blanking). Do NOT touch them.
- This task is ONLY: the bootstrap.go commented-loop pi blanking + the docs/providers.md sentence.

---

## 7. Validation commands (Makefile)

- Build: `go build ./...`
- Vet: `go vet ./internal/config/...`
- Format: `gofmt -l internal/config/bootstrap.go` (must be empty)
- Existing tests stay green: `go test ./internal/config/ -run 'BuildBootstrapConfig|GenerateBootstrapConfig' -v`
  (especially ValidTOML with the claude/pi + agy/pi/claude cases)
- Full suite (race): `make test`
- Lint: `make lint`
- Manual proof: generate a config with a non-pi target + pi installed, grep the commented pi block:
  ```
  buildBootstrapConfig("claude", []string{"claude","pi"}, nil)  # in a test or scratch program
  # expect: the "# === pi (installed) ===" block has `# model = ""` (NOT gpt-5.4*) + the NOTE
  ```
