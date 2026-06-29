# Research: go-toml/v2 pointer / slice / map behavior (empirical probe)

> Verified live against `github.com/pelletier/go-toml/v2 v2.4.2` (the version already pinned in this
> repo's `go.mod`). This probe is the empirical basis for the Manifest struct's pointer-type design.

## Probe

A struct mixing plain string, `*string`, `*bool`, `[]string`, and `map[string]string` fields was
marshaled (full + partial) and unmarshaled (partial + explicit-empty). Full source in the PRP author's
scratch; results reproduced exactly below.

## Result 1 — MARSHAL (full manifest)

```
name = 'pi'
command = 'pi'
print_flag = '-p'
strip_code_fence = true
subcommand = []
bare_flags = ['--no-tools']

[env]
X = '1'
```

All fields present are emitted. (Note: go-toml/v2 emits literal single-quoted strings by default —
cosmetic, valid TOML; the hand-written `providers/*.toml` reference files in P1.M5.T2 are independent.)

## Result 2 — MARSHAL (partial: only Name set, everything else zero/nil)

```
name = 'gemini'
subcommand = []
bare_flags = []
```

**FINDING A — nil `*string`/`*bool` pointer fields are OMITTED on marshal.** `command`, `print_flag`,
`strip_code_fence` (all nil) do NOT appear. This is the §5.4 Option A property: pointer fields give us
omitempty-like behavior for free, exactly what `providers show` (P1.M4.T1.S3) wants.

**FINDING B — nil slices are marshaled as `[]` (NOT omitted).** `subcommand = []` and `bare_flags = []`
appear even though they were nil. This is acceptable: the PRD §12.1 manifests themselves show
`subcommand = []`. It does mean slices lack the "omit-when-unset" property; that is irrelevant to merge
correctness (see Result 3) and only mildly cosmetic for `providers show`.

## Result 3 — UNMARSHAL (partial TOML: name + print_flag + bare_flags present; rest absent)

```
Name="x"  Command=<nil>  Print=0x...  Strip=<nil>  Sub=[]  Bare=[a b]  Env=map[]
  Command==nil? true   Strip==nil? true   Sub==nil? true   Env==nil? true
  *Print="-p"   Bare=[a b]
```

**FINDING C — absent TOML keys decode to NIL pointers, NIL slices, NIL maps.** `Command`, `Strip`
(nil ptrs); `Sub`, `Env` (nil slice/map). This is the merge foundation: "field absent in the override"
is detectable as `nil`.

## Result 4 — UNMARSHAL (explicit empty / false values)

Input: `print_flag = ""`, `subcommand = []`, `strip_code_fence = false`.

```
Print=0x...  Sub=[] (len 0)  Strip=0x...
  *Print=""   (explicit empty string kept as NON-NIL pointer)
  *Strip=false (explicit false kept as NON-NIL pointer)
  Sub==nil? false  (explicit [] -> non-nil empty slice)
```

**FINDING D — present-but-zero values decode to NON-NIL pointers/slices.** `print_flag = ""` → a
non-nil `*string` whose value is `""`. `strip_code_fence = false` → a non-nil `*bool` whose value is
`false`. `subcommand = []` → a non-nil empty slice.

**THIS IS THE CRITICAL RESULT.** It proves pointers DISTINGUISH "user set the field to its zero value"
(non-nil) from "user did not set the field" (nil). A plain `string`/`bool` CANNOT make this
distinction (both look like the zero value). Therefore:

- A user override of `strip_code_fence = false` against a built-in `true` is correctly applied
  (non-nil `*bool=false` wins the merge) — IMPOSSIBLE with plain `bool`.
- A user override of `print_flag = ""` (disable the print flag) against a built-in `-p` is correctly
  applied (non-nil `*string=""` wins) — IMPOSSIBLE with plain `string`.

## Conclusion — design decision confirmed

**Manifest uses pointer types (`*string`, `*bool`) for the optional scalar fields.** This is mandated
by correctness for the field-by-field manifest merge (P1.M2.T1.S2 + registry P1.M2.T3), which must
distinguish "absent" (nil → inherit built-in) from "explicitly-set-to-zero" (non-nil → override).

- **Slices (`[]string`) and maps (`map[string]string`) stay plain** — they have a natural nil sentinel
  (absent → nil; present → non-nil even if empty) per FINDING C/D. No pointers needed.
- **`Name` stays plain `string`** — it is the identity/key, always populated by the registry from the
  `[provider.<name>]` table key; it is never a merge subject.
- **`Command` is `*string`** despite being "required" — a partial user OVERRIDE omits it (nil → inherit
  the built-in's command). `Validate()` (run on the FINAL merged manifest) enforces `*Command != ""`.

`Resolve()` (returns a copy with all nil optional pointers filled to defaults) gives consumers
(renderer/executor/parser) a guaranteed-non-nil manifest after the registry merges. nil slices/maps are
left nil — Go's `append(nilSlice...)` is a no-op, so the §12.2 rendering algorithm handles them fine.

This matches the frozen `internal/config/config.go` Providers comment ("re-encodes the entry to TOML
and unmarshals into a Manifest, then field-merges with the built-in manifest per PRD §16.1") — that
struct-level field-merge only works cleanly with nil-detection, i.e. pointers.
