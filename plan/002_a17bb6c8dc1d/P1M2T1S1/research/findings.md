# P1.M2.T1.S1 — Research Findings (synthesis)

Full agy spec: PRD §12.5.1 / §12.5.1.1 (in `prd_snapshot.md`) + the work-item contract. agy's flag
surface is FIXED by the contract (researched from docs + issue tracker, ships `experimental=true`);
the implementer must NOT try to `--help`-verify agy's real flags — just implement the spec'd manifest.

## The agy manifest (contract-resolved; nil/empty decided for DeepEqual parity)

| field | value | Go | TOML |
|---|---|---|---|
| Name | "agy" | `Name: "agy"` | `name = "agy"` |
| Detect | "agy" | `Detect: strPtr("agy")` | `detect = "agy"` |
| Command | "agy" | `Command: strPtr("agy")` | `command = "agy"` |
| PromptDelivery | "stdin" | `strPtr("stdin")` | `prompt_delivery = "stdin"` |
| PrintFlag | "-p" | `strPtr("-p")` | `print_flag = "-p"` |
| ModelFlag | "-m" | `strPtr("-m")` | `model_flag = "-m"` |
| DefaultModel | "gemini-2.5-pro" | `strPtr("gemini-2.5-pro")` | `default_model = "gemini-2.5-pro"` |
| SystemPromptFlag | "" (NON-NIL) | `strPtr("")` | `system_prompt_flag = ""` |
| ProviderFlag | "" (NON-NIL) | `strPtr("")` | `provider_flag = ""` |
| BareFlags | ["--approval-mode","default"] | `[]string{...}` | `bare_flags = ["--approval-mode", "default"]` |
| **TooledFlags** | **nil** (cannot stager) | **NOT SET** | **KEY OMITTED** |
| **Experimental** | **true** | **`boolPtr(true)`** | **`experimental = true`** |
| Output | "raw" | `strPtr("raw")` | `output = "raw"` |
| StripCodeFence | true | `boolPtr(true)` | `strip_code_fence = true` |

OMITTED (nil, like gemini — the twin): Subcommand, PromptFlag, DefaultProvider, JsonField,
RetryInstruction, Env. Do NOT set them in Go; do NOT emit them in TOML (absent ⇒ nil ⇒ DeepEqual-clean).

Rendered (§12.5.1; validate via renderArgs): `agy -m gemini-2.5-pro --approval-mode default -p`

## ⚠️ THE #1 TRAP — `reflect.DeepEqual` parity across THREE artifacts

Two decode-parity guards + one coverage guard all use `reflect.DeepEqual`, and go-toml/v2 distinguishes
ABSENT-key (→ nil slice/ptr) from `key = []` (→ non-nil empty slice) / `key = ""` (→ non-nil *string):
1. `TestBuiltinManifests_DecodeParity` (builtin_test.go:311) — `builtinAgy()` vs the `agyTOML` test literal.
2. `TestProviderReferenceFiles_DecodeParity` (referencefiles_test.go) — `builtinAgy()` vs `providers/agy.toml`.
3. `TestProviderReferenceFiles_AllBuiltinsCovered` — every builtin must have a reference file (and vice versa).

⇒ `builtinAgy()` (Go), `agyTOML` (builtin_test.go literal), and `providers/agy.toml` (file) must decode to
the IDENTICAL Manifest. KEEP THE FIELD LINES BYTE-FOR-BYTE IDENTICAL across agyTOML and providers/agy.toml
(modulo comments). Per the contract: TooledFlags = nil ⇒ OMIT the `tooled_flags` key on both TOML sides
(do NOT write `tooled_flags = []` — that decodes to non-nil empty and breaks DeepEqual). `subcommand` is
NOT in the contract ⇒ omit it (nil), matching gemini (NOT `subcommand = []`).

## The full test-edit set (verified by grep — these are ALL the break sites)

- `builtin_test.go:162` comment + `:167-168` `len(m) != 6`/`want 6` → 7 (KeysAndCount).
- `builtin_test.go`: ADD `agyTOML` const (after cursorTOML) + ADD `{"agy", builtinAgy(), agyTOML}` to the
  DecodeParity table (:317-322). (Conventional, following the per-provider pattern: ADD
  `TestBuiltinManifests_AgyFields` + `TestBuiltinManifests_RenderedCommand_Agy`.)
- `referencefiles_test.go:14-26`: ADD `{"agy", "providers/agy.toml"}` to `providerFiles`; fix the "6
  shipped" comment. (Drives BOTH DecodeParity and AllBuiltinsCovered.)
- `registry.go:15`: ADD `"agy"` to `preferredBuiltins` (append at end — least-preferred, appropriate for
  an experimental provider). This is REQUIRED: `TestPreferredBuiltins_MatchesBuiltinKeys`
  (registry_test.go:15) enforces set-equality + count between preferredBuiltins and builtin keys. The
  precise ORDER is finalized by P1.M2.T2.S1 ("Reorder preferredBuiltins") — do NOT preempt it.
- `registry_test.go:35`: comment "exactly the 6 built-ins" → 7 (comment only; the count assertion at
  :40-41 uses `len(BuiltinManifests())` dynamically — auto-adjusts, NO code change).
- `docs/providers.md:55` "The 6 built-in providers" → 7; ADD an agy row to the table (:59-66) noting
  experimental + the non-TTY stdout drop (issue #76).

## Why no external/online research is warranted

agy's flags are fully specified by the contract (it explicitly ships `experimental=true` precisely
BECAUSE it isn't `--help`-verified). This task is "add a 7th builtin following the established
builtinGemini pattern + keep three sync guards green." Nothing to look up online.
