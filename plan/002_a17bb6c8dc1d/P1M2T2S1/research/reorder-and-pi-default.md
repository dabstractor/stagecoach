# Research — P1.M2.T2.S1: Reorder preferredBuiltins + pi default_model → empty (FR-D1/D2)

> Scope: TWO small value changes with a wide-but-mechanical test ripple.
> (1) **FR-D1**: reorder `preferredBuiltins` to `[pi, opencode, cursor, agy, gemini, codex, claude]`
> (open/self-hostable harnesses first; closed subscription CLIs last).
> (2) **FR-D2**: pi's `default_model` `glm-5-turbo` → `""` (decouple from the author's personal z.ai
> subscription; `config init` fills per-role from FR-D4). pi's `default_provider` is ALREADY `""`.
>
> **Prerequisite (assumed COMPLETE):** P1.M2.T1.S1 (agy) — appends `"agy"` to `preferredBuiltins`,
> adds `builtinAgy()` + `"agy"` to `BuiltinManifests()`, creates `providers/agy.toml`, and updates the
> agy test coverage (KeysAndCount→7, agyTOML, AgyFields, providerFiles+agy). When THIS task begins,
> `BuiltinManifests()` has 7 keys and `preferredBuiltins` is `[pi, claude, gemini, opencode, codex,
> cursor, agy]` (agy appended at the END by the agy task). This task REORDERS that 7-element slice to
> FR-D1 and does NOT touch any agy-specific edit.

---

## 1. The two changes (PRD basis)

### FR-D1 — cascading provider priority (PRD §9.16 FR-D1, h3.32)
> "The auto-default provider is the highest-priority built-in whose command is found on `$PATH`, in this
> order: **pi, opencode, cursor, agy, gemini, codex, claude.** (Rationale: open / self-hostable harnesses
> first; closed subscription CLIs last.) … Implemented as `Registry.DefaultProvider(installed)` over
> `preferredBuiltins`."

So the new `preferredBuiltins` (registry.go:15) is exactly:
```go
var preferredBuiltins = []string{"pi", "opencode", "cursor", "agy", "gemini", "codex", "claude"}
```
The comment (registry.go:11) currently says "PRD §12.3–12.7 listing order; pi first" → rewrite to cite
FR-D1 (open/self-hostable first; closed last; pi first). `DefaultProvider(installed)` walks this slice
and returns the first name the caller reports installed — its LOGIC is unchanged; only the slice order
(and its doc comment) change.

### FR-D2 — pi decoupled from the z.ai subscription (PRD §9.16 FR-D2 + §12.3 h3.45)
> "No built-in default assumes a specific account or backend — notably **pi no longer ships `glm-*` /
> `zai` as its default** … The shipped pi default routes to a generally-available model … the personal
> z.ai/GLM setup becomes a documented *override*, not the default."

PRD §12.3 (h3.45) now shows the shipped pi manifest with `default_model = ""` and `default_provider = ""`
(both empty), and frames the `zai`/`glm-5-turbo` invocation as a **"Personal-override example (NOT the
shipped default)"**. So in `builtinPi()` (builtin.go:42): `DefaultModel: strPtr("glm-5-turbo")` →
`strPtr("")`. `DefaultProvider` is ALREADY `strPtr("")` — no change there. Update the `builtinPi` doc
comment (it currently claims the shipped default "reproduces commit-pi byte-for-byte" — now false).

---

## 2. Complete break-site map (verified by grep `glm-5-turbo` + `preferredBuiltins`)

### CHANGE (required edits)

| # | File:line | Current | Required edit |
|---|-----------|---------|---------------|
| 1 | `internal/provider/registry.go:15` | `preferredBuiltins = []string{"pi","claude","gemini","opencode","codex","cursor","agy"}` | reorder to `{"pi","opencode","cursor","agy","gemini","codex","claude"}` (FR-D1) |
| 2 | `internal/provider/registry.go:11` | comment "§12.3–12.7 listing order; pi first" | rewrite to cite FR-D1 (open/self-hostable first; closed last; pi first) |
| 3 | `internal/provider/registry_test.go` `TestPreferredBuiltins_MatchesBuiltinKeys` | set-equality + pi-first only | ADD exact-order assertion `reflect.DeepEqual(preferredBuiltins, []string{"pi","opencode","cursor","agy","gemini","codex","claude"})` to pin FR-D1 |
| 4 | `internal/provider/registry_test.go` `TestDefaultProvider` | case `["claude","gemini"]→"claude"` | BREAKS — gemini(5) now precedes claude(7) → returns "gemini". Rewrite cases to assert FR-D1 cascade (see §3) |
| 5 | `internal/provider/builtin.go:42` (`builtinPi`) | `DefaultModel: strPtr("glm-5-turbo")` | → `strPtr("")` (FR-D2) |
| 6 | `internal/provider/builtin.go` `builtinPi` doc comment (~L28-35) | "Rendered with provider=zai, model=default … reproduces commit-pi byte-for-byte" | rewrite: FR-D2 decoupling — default_model AND default_provider empty; config init fills per-role; zai/glm-5-turbo is a documented PERSONAL OVERRIDE (not the default) |
| 7 | `internal/provider/builtin_test.go:22` (`piTOML`) | `default_model = "glm-5-turbo"` | → `default_model = ""` (PARITY ORACLE — must equal builtinPi; see §4) |
| 8 | `internal/provider/builtin_test.go:~202` (`PiFields`) | `assertStr(... "DefaultModel", m.DefaultModel, "glm-5-turbo")` | → `""` |
| 9 | `internal/provider/builtin_test.go:~336` (`RenderedCommand_Pi_MatchesCommitPi`) | `renderArgs(builtinPi(), "zai", "", "<sys>")` → `--model glm-5-turbo` | REFRAME into shipped-default + personal-override (see §5) |
| 10 | `internal/provider/render.go:~67` (comment only) | "lets the pi golden test pass with model="" → glm-5-turbo" | stale comment — rewrite (logic unchanged; the fallback `if modelToUse=="" { modelToUse = *r.DefaultModel }` is correct, just now yields "" for pi) |
| 11 | `internal/provider/render_test.go` `TestRender_GoldenPerProvider` pi case (~L47) | `{"pi", pi, "", "zai", …, wantArgs with "--model","glm-5-turbo"}` | → shipped default `{"pi", pi, "", "", …, wantArgs with NO --model/--provider}` (see §5) |
| 12 | `internal/provider/render_test.go` `TestRender_Pi_ByteForByteCommitPi` (~L88) | `Render("", "zai", "<sys>", "<user>")` → commit-pi via default | → explicit override `Render("glm-5-turbo", "zai", …)` → commit-pi byte-for-byte (FR-D2: this is the override path now) |
| 13 | `internal/provider/render_test.go` `TestRender_ModelDefaultFallback` (~L128) | uses pi (default was glm-5-turbo) for the fallback mechanic | pi default is now "" → use **claude** (default "sonnet") for the fallback mechanic + ADD a pi-emits-no-model case (see §5) |
| 14 | `providers/pi.toml:43` | `default_model = "glm-5-turbo"` | → `default_model = ""` (PARITY ORACLE — must equal builtinPi/builtin_test piTOML; see §4) |
| 15 | `providers/pi.toml:21` (rendered-command comment) | `# pi --provider zai --model glm-5-turbo …` | → placeholders `# pi --provider <backend> --model <m> …` + note zai/glm-5-turbo is a personal override (match PRD §12.3 h3.45) |
| 16 | `internal/generate/realagent_test.go:36` | `"pi": {"", "zai"},` comment "glm-5-turbo from manifest default" | → `"pi": {"glm-5-turbo", "zai"},` (model now explicit — manifest default is "") + comment "personal override (commit-pi); manifest default empty (FR-D2)" (opt-in `//go:build integration_real` test; correctness fix) |
| 17 | `internal/generate/realagent_test.go:44` (`providerNames`) | `[]string{"pi","claude","gemini","opencode","codex","cursor"}` comment "preferredBuiltins order" | reorder to FR-D1 minus agy `["pi","opencode","cursor","gemini","codex","claude"]` (agy excluded — experimental/non-TTY); fix comment. (Subtest display order only; agy is intentionally not real-tested.) |
| 18 | `internal/cmd/providers_test.go:208` | `"default_model = 'glm-5-turbo'"` | → `"default_model = ''"` (providers show pi now marshals empty) |

### LEAVE (independent fixtures — NOT parity oracles; editing them is unnecessary churn)

| File:line | Why it's independent |
|-----------|----------------------|
| `internal/provider/manifest_test.go:18,63` (`piManifestTOML`) | S1's STRUCTURAL decode fixture. It OMITS `default_provider` (asserts `DefaultProvider`→nil), so it is ALREADY ≠ builtinPi (which has `default_provider=""`). It tests the Manifest TYPE's decode mechanics, not the built-in value. Its `glm-5-turbo` is an arbitrary decode-test value. Leave as-is. |
| `internal/provider/merge_test.go:19` (`sampleBase`) | The merge test's helper fixture (tests `MergeManifest` field-merging). Uses `glm-5-turbo` as an arbitrary model value; is NOT builtinPi and is NOT a parity oracle. Leave as-is. |

> **Do NOT "fix" the LEAVE files.** They are not asserting the pi built-in's value. Touching them is
> out-of-scope churn that risks breaking unrelated tests. The grep hits there are coincidental (an
> arbitrary model string in an independent fixture).

---

## 3. The `TestDefaultProvider` break (registry_test.go)

`DefaultProvider` walks `preferredBuiltins` and returns the first installed name. Under FR-D1 order
`[pi, opencode, cursor, agy, gemini, codex, claude]`, the existing case
`DefaultProvider(["claude","gemini"])` now returns **"gemini"** (gemini is 5th, claude is 7th — gemini
wins), not "claude". Rewrite the test's cases to assert the FR-D1 cascade robustly:

```go
func TestDefaultProvider(t *testing.T) {
	r := NewRegistry(nil)
	cases := []struct{ installed []string; want string }{
		{[]string{"pi"}, "pi"},                                  // pi always wins (rank 1)
		{[]string{"claude", "gemini"}, "gemini"},                // gemini(5) before claude(7) under FR-D1
		{[]string{"codex", "claude"}, "codex"},                  // codex(6) before claude(7)
		{[]string{"cursor", "agy", "gemini"}, "cursor"},         // cursor(3) before agy(4)/gemini(5)
		{[]string{"opencode", "pi"}, "pi"},                      // pi still tops opencode(2)
		{[]string{"myagent"}, ""},                               // user-defined never auto-selected
		{nil, ""},                                               // nothing installed
	}
	for _, c := range cases {
		if got := r.DefaultProvider(c.installed); got != c.want {
			t.Errorf("DefaultProvider(%v) = %q, want %q", c.installed, got, c.want)
		}
	}
}
```

(The original test was a flat sequence of `if`s; a table is cleaner and the implementer may keep the
flat style — the cases + expectations are what matter.)

---

## 4. The three-way DeepEqual parity chain (the #1 correctness trap)

`default_model` participates in a three-way `reflect.DeepEqual` chain (go-toml: `""` → non-nil `*""`).
After the change, all THREE artifacts must carry `default_model = ""`:

```
builtinPi()  ─┐
              ├─ reflect.DeepEqual ─→ must ALL agree (default_model = "" → non-nil *"" in each)
piTOML (test literal) ─┤
providers/pi.toml (file) ─┘
```

- `builtinPi()`: `DefaultModel: strPtr("")` (non-nil pointer to "").
- `piTOML` (builtin_test.go): `default_model = ""` (decodes to non-nil `*""`).
- `providers/pi.toml`: `default_model = ""` (decodes to non-nil `*""`).

If any one keeps `glm-5-turbo` (or omits the key → nil), `TestBuiltinManifests_DecodeParity/pi` or
`TestProviderReferenceFiles_DecodeParity/pi` FAILS with a DeepEqual diff. The fix is mechanical: change
all three to `""`. `default_provider` is ALREADY `""` in all three (pi always had it) — DO NOT change
it (and do NOT omit it: pi writes `default_provider = ""` as a NON-NIL empty; the chain requires that).

---

## 5. The pi render reframe — shipped default vs personal override (the trickiest part)

FR-D2 splits the pi render story into TWO cases. Both `renderArgs` (builtin_test.go test scaffolding)
and `Render` (render.go) compute `modelToUse = model; if modelToUse=="" { modelToUse = *r.DefaultModel }`.
With `*r.DefaultModel == ""` now, a `model=""` call yields NO `--model` flag.

### (a) Shipped default (the NEW normal): `model="", provider=""`
- provider_flag="--provider" set BUT provider="" → `if provider_flag && provider` false → **no --provider**
- model_flag="--model" set BUT modelToUse="" (default is "") → **no --model**
- system_prompt_flag="--system-prompt", sys="<sys>" → `--system-prompt <sys>`
- bare_flags → all 6 verbatim
- print_flag="-p" → `-p` (LAST)
- argv: `["pi","--system-prompt","<sys>","--no-tools","--no-extensions","--no-skills","--no-prompt-templates","--no-context-files","--no-session","-p"]`

### (b) Personal override / commit-pi (the OLD normal, now explicit): `model="glm-5-turbo", provider="zai"`
- provider="zai" → `--provider zai`
- model="glm-5-turbo" (explicit) → `--model glm-5-turbo`
- sys="<sys>" → `--system-prompt <sys>`
- bare_flags → 6; print → `-p`
- argv: `["pi","--provider","zai","--model","glm-5-turbo","--system-prompt","<sys>","--no-tools",…,"--no-session","-p"]`
  ← byte-for-byte the commit-pi invocation (preserved as a regression, now via explicit params)

### Test mapping
- **`builtin_test.go` `RenderedCommand_Pi`**: replace the single `MatchesCommitPi` test with TWO:
  `RenderedCommand_Pi_ShippedDefault` (case a — `renderArgs(builtinPi(), "", "", "<sys>")`) and
  `RenderedCommand_Pi_PersonalOverride` (case b — `renderArgs(builtinPi(), "zai", "glm-5-turbo", "<sys>")`,
  asserting the commit-pi argv). This preserves the byte-for-byte commit-pi regression while verifying
  the new shipped default.
- **`render_test.go` `TestRender_GoldenPerProvider` pi row**: change to the shipped default
  (`model="", provider=""`, wantArgs WITHOUT `--provider`/`--model`).
- **`render_test.go` `TestRender_Pi_ByteForByteCommitPi`**: pass EXPLICIT `model="glm-5-turbo"`
  (`Render("glm-5-turbo", "zai", "<sys>", "<user>")`) so it still asserts the commit-pi argv. Add a
  comment that this is the personal-override path (FR-D2 decoupled the default).
- **`render_test.go` `TestRender_ModelDefaultFallback`**: pi's default is now "" so it can no longer
  demonstrate the fallback. Switch the fallback mechanic to **claude** (`DefaultModel="sonnet"`):
  `claude.Render("","","","")` → `--model sonnet`; `claude.Render("custom","","","")` → `--model custom`.
  ADD a pi case asserting NO `--model` token appears (`!containsToken(args,"--model")`) — pins FR-D2.

### render.go logic = UNCHANGED (comment-only edit)
The fallback `if modelToUse == "" { modelToUse = *r.DefaultModel }` is correct for ALL providers; for
pi it now yields "" (no --model), which is the intended FR-D2 behavior. Only the stale comment at
render.go:~67 ("→ glm-5-turbo") needs rewriting. Do NOT change the Render code.

---

## 6. Scope fences (what is NOT this task)

- **NOT agy (P1.M2.T1.S1).** agy appends "agy" + adds the 7th builtin + its test coverage. This task
  assumes agy is COMPLETE and only REORDERS the (now 7-element) `preferredBuiltins` and sets pi's
  default_model="". Do NOT add/remove agy or touch agy's tests.
- **NOT tooled_flags (P1.M2.T2.S2).** The sibling task adds `tooled_flags` to pi + claude (stager-
  capable). This task does NOT touch tooled_flags. (If S2 lands first, pi/claude gain tooled_flags —
  that's orthogonal to default_model + ordering; the parity chain still holds as long as both the Go
  struct and the TOML agree.)
- **NOT the FR-D4 model table / config init (P1.M3/P1.M4).** This task only EMPTIES pi's default_model;
  populating per-role models is a later task. The shipped pi manifest is intentionally model-less until
  config init fills it.
- **NOT a render/Validate/Resolve/Merge logic change.** Only data values (the slice order, one struct
  field, one TOML field) + comments + test assertions/reframes. The render fallback code is unchanged.
- **NOT the `LEAVE` fixtures** (manifest_test.go `piManifestTOML`, merge_test.go `sampleBase`). They are
  independent of the pi built-in value.
- **`registry.go:47` `make(..., len(userOverrides)+6)`**: a capacity-hint comment that's now stale (7
  built-ins). Cosmetic, harmless (capacity hint only). OPTIONAL adjacent fix (`+6`→`+7`); not required.

---

## 7. Validation commands (verified against this codebase)

```bash
# Build + vet + fmt
go build ./...
go vet ./...
gofmt -l internal/provider/ internal/generate/ internal/cmd/ providers/

# The provider-package suite (where most break sites live):
go test ./internal/provider/... -v
#   MUST pass: TestPreferredBuiltins_MatchesBuiltinKeys (new order assertion),
#              TestDefaultProvider (reframed FR-D1 cases), TestBuiltinManifests_PiFields (DefaultModel=""),
#              TestBuiltinManifests_DecodeParity/pi (piTOML default_model=""),
#              TestBuiltinManifests_RenderedCommand_Pi_* (shipped + override),
#              TestProviderReferenceFiles_DecodeParity/pi (providers/pi.toml default_model=""),
#              TestRender_GoldenPerProvider/pi, TestRender_Pi_ByteForByteCommitPi, TestRender_ModelDefaultFallback,
#              AND all agy tests (untouched) still green.

# The CLI providers test (providers_test.go:208):
go test ./internal/cmd/... -run TestProvidersShow -v

# Whole repo (no non-provider package should regress):
go test ./...

# Catch any straggler that still hardcodes the old order or glm-5-turbo as a DEFAULT assertion:
grep -rn 'glm-5-turbo' internal/ providers/   # should remain ONLY in: manifest_test.go (structural fixture),
                                              # merge_test.go (sampleBase fixture), realagent_test.go (explicit override),
                                              # render_test.go (explicit-override byte-for-byte test) — NOT in builtin.go,
                                              # builtin_test.go piTOML/PiFields, providers/pi.toml default, or providers_test.go show assertion.
grep -rn '"pi", "claude", "gemini", "opencode", "codex", "cursor"' internal/   # the OLD order should be GONE
                                                                              # (realagent providerNames becomes the FR-D1-minus-agy order).
```
