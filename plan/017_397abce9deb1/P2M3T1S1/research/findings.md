# Research — P2.M3.T1.S1: flake.nix (buildGoModule) + nix flake check CI

Captured 2025 (FR-D5: re-verify action pin + nixpkgs Go version + eachSystem behavior at impl).
This is a Nix-packaging task. The flake builds the SAME binary goreleaser ships (cmd/stagecoach,
CGO_ENABLED=0), just via nixpkgs buildGoModule. No Go changes.

## 1. Codebase shape (the build inputs the flake consumes)

- **Module path**: `github.com/dabstractor/stagecoach` (go.mod). go directive: `go 1.22`.
- **Main package**: `cmd/stagecoach` (`./cmd/stagecoach` — Makefile `MAIN_PKG`, goreleaser `main: ./cmd/stagecoach`).
  `cmd/stagecoach/main.go` has `var version = "dev"` (ldflags-injected; `resolveVersion()` enriches
  "dev" with Go's embedded VCS info when `.git` is present).
- **No vendor/ dir** → buildGoModule fetches deps itself (default go-modules FOD; do NOT set proxyVendor).
- **CGO_ENABLED=0** is the project's static-build invariant (.goreleaser.yaml `env: CGO_ENABLED=0`;
  ci.yml cross-build sets `CGO_ENABLED=0`). The flake MUST match → `env.CGO_ENABLED = 0`.
- **go.sum present** (16 lines, small) → buildGoModule computes `vendorHash` from it.
- **Only git tag**: `v0.1.0`. No released tarball → the flake builds from `src = self.outPath` (the
  working tree / the flake's source root).
- **Existing CI** (.github/workflows/ci.yml): 5 jobs (build-test, lint, vulncheck, coverage, npm-smoke).
  This task ADDS a 6th job `nix-flake-check`. Sibling P2.M2.T1.S1 (Winget) edits release.yml (no collision).

## 2. The flake.nix shape (buildGoModule — current nixpkgs API)

```nix
{
  description = "Stagecoach — AI-written commit messages via your installed CLI agent";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      eachSystem = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in {
      packages = eachSystem (pkgs: {
        default = pkgs.buildGoModule {
          pname = "stagecoach";
          version = "unstable-${self.sourceInfo.lastModifiedDate}";
          src = self.outPath;                 # the flake source root (working tree / store path)
          vendorHash = pkgs.lib.fakeHash;     # placeholder — run `nix build`, paste the got-hash (§4)
          subPackages = [ "cmd/stagecoach" ];
          env.CGO_ENABLED = 0;                 # static build; matches .goreleaser.yaml + ci.yml
        };
      });

      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go gopls ];
        };
      });
    };
}
```

### Load-bearing API facts (verified against nixpkgs build-support/go)
- **`vendorHash`** is the CURRENT argument; `vendorSha256` is the DEPRECATED alias (still works, warns).
  Use `vendorHash`. (external_deps.md §5 explicitly says "vendorHash".)
- **`lib.fakeHash`** = `"sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="` (the canonical placeholder
  for SRI sha256). It is NOT `lib.fakeSha256` (that's the base32 `0000...=` form, pre-SRI). For
  `vendorHash` use `lib.fakeHash` (SRI).
- **`env = { CGO_ENABLED = 0; }`** is the modern idiom (nixpkgs moved env to the `env` attr set).
  Bare top-level `CGO_ENABLED = 0;` also still works (stdenv passes it through), but `env.CGO_ENABLED`
  is the current convention.
- **`subPackages = [ "cmd/stagecoach" ]`** — builds only that package (avoids building test/example mains).
- **No `proxyVendor`/`modHash` needed**: the default go-modules fixed-output derivation fetches deps
  from the Go module proxy (matches our no-vendor/ repo).
- **`src = self.outPath`** (=`.`) — the flake source. (Some flakes use `src = ./.;` which also works
  inside a flake; `self.outPath` is the explicit store-path form.)

## 3. The `version` field (the contract: "from the tag or 'unstable'")

The binary's INTERNAL version (`main.version`) is "dev" regardless — the flake does not pass goreleaser's
ldflags, and Go's VCS embedding is unavailable in the nix sandbox (source is copied to the store
WITHOUT `.git`, so `vcs.revision` is absent → `resolveVersion()` returns plain "dev"). goreleaser
remains the canonical release path for real version strings; the flake's `version` is the DERIVATION
name (e.g. `stagecoach-unstable-20250101...`), not the binary's self-reported version.

Two acceptable idioms (pick one, both robust):
- **(A) date+rev unstable (recommended, zero-maintenance):**
  `version = "unstable-${self.sourceInfo.lastModifiedDate}";`
- **(B) tag-derive (resolves to "0.1.0" when checked out on the v0.1.0 tag):**
  ```nix
  version =
    let ref = self.sourceInfo.ref or null;
    in if ref != null && builtins.match "refs/tags/v.*" ref != null
       then builtins.substring 11 (builtins.stringLength ref - 11) ref
       else "0.0.0-unstable-${self.sourceInfo.lastModifiedDate}";
  ```
  (`refs/tags/v` is 11 chars; substring from 11 yields the bare version.)

The PRP recommends (A) — robust, no edge cases around dirty refs. Document the "dev" binary-version
gotcha in the flake header comment.

## 4. The vendorHash update workflow (THE key mechanism)

1. Set `vendorHash = pkgs.lib.fakeHash;` (the `sha256-AAAA...=` placeholder).
2. Run `nix build` (or `nix flake check`). It FAILS with a fixed-output hash mismatch, e.g.:
   ```
   error: hash mismatch in fixed-output derivation '/nix/store/...-stagecoach-go-modules':
         specified: sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
            got:    sha256-<REAL-SRI-HASH>
   ```
3. Copy the `got:` value (the `sha256-...` SRI string) into `vendorHash`.
4. Re-run `nix build` → succeeds; `nix flake check` → green.

This is exactly what the contract specifies ("set to lib.fakeHash initially, then run nix build to
get the real hash"). The nix error literally prints the value to paste. CI's `nix flake check` KEEPS
this current: after any go.sum change, the committed vendorHash becomes stale and `nix flake check`
fails with the got-hash to paste — so a stale hash cannot merge (the contract's "CI must keep it current").

## 5. CI job — cachix/install-nix-action + `nix flake check` (FR-D5 verified)

- **install-nix-action** (https://github.com/cachix/install-nix-action) installs Nix with the
  experimental `flakes` + `nix-command` features ENABLED BY DEFAULT (confirmed in the action README +
  marketplace listing). Current major moving tag ≈ **v31** (late 2025; verify at impl — pin the moving
  major `@v31`-style for auto minor/patch, mirroring the winget PRP's `@v2` convention).
- **`nix flake check`** evaluates + BUILDS the flake's outputs. On recent Nix (2.21+, which
  install-nix-action's default 2025 Nix is), it defaults to the CURRENT system's outputs (linux on an
  ubuntu runner) → it builds the `x86_64-linux` package → a stale `vendorHash` FAILS the check with the
  got-hash. That is the contract's CI gate. (Older Nix built ALL systems and failed on darwin/aarch64;
  modern Nix defaults to current-system, fixing that. If the impl observes darwin/aarch64 build
  attempts failing, fall back to `nix build .#default --print-build-logs`, which deterministically
  builds only the local system.)
- **DECISION (contract): NO cachix for v3.0** — skip the cachix action. Builds are small (Go static
  binary + the go-modules FOD); accept the cold-build cost now, add cachix later. The job is simple:
  checkout → install-nix-action → `nix flake check`.
- **continue-on-error: false** (the default) — let a vendorHash mismatch FAIL the job so the nix error
  (with the got-hash) surfaces clearly. This is what "keeps it current".

Minimal job:
```yaml
  # --- (6) Nix flake check (PRD §21.2; external_deps.md §5) ---------------------
  nix-flake-check:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v4
      - uses: cachix/install-nix-action@v31   # flakes + nix-command enabled by default; pin moving major
        with:
          nix_path: nixpkgs=channel:nixos-unstable
      - name: nix flake check
        # Builds the flake's outputs for the current system (x86_64-linux). A stale vendorHash fails
        # here with a clear "got: sha256-..." mismatch — copy it into flake.nix (see header comment).
        run: nix --print-build-logs flake check
```

## 6. eachSystem idiom — NO extra flake input (no flake-utils)

The contract wants a minimal in-repo flake. `nixpkgs.lib.genAttrs` gives a dependency-free `eachSystem`:
```nix
systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
eachSystem = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
```
This avoids adding `flake-utils` (numtide/flake-utils) as an input — one fewer dependency, fewer lock
entries, fewer supply-chain surfaces. (flake-utils is fine but unnecessary here.)

## 7. .gitignore entries (canonical nix artifacts)

- `result` — the symlink `nix build` creates in the repo root.
- `result-*` — `nix build .#foo` creates `result-foo`; cover the glob.
- `.direnv` — `direnv use` creates `.direnv/` (the cached flake env) when a user adds `.envrc`.
- DO NOT gitignore `flake.lock` — commit it (stagecoach is an APPLICATION, so pin nixpkgs for CI
  determinism; `nix flake check` uses the committed lock). Convention: apps commit flake.lock,
  libraries don't.

## 8. devShells.default — mkShell with go + gopls

```nix
devShells = eachSystem (pkgs: {
  default = pkgs.mkShell {
    packages = with pkgs; [ go gopls ];
  };
});
```
Canonical Go dev shell. (Optional adds: `gotools`, `delve`/`dlv`, `golangci-lint` — the contract
names only go + gopls; keep minimal.) `nix develop` enters it.

## 9. Scope & sibling coordination

| Artifact | This task (Nix) | Sibling P2.M2.T1.S1 (Winget) |
|----------|-----------------|------------------------------|
| `flake.nix` | CREATE | — |
| `.github/workflows/ci.yml` | MODIFY (add `nix-flake-check` job) | — |
| `.github/workflows/release.yml` | — | MODIFY (add `winget` job) |
| `.gitignore` | MODIFY (result, result-*, .direnv) | — |
| `docs/packaging.md` | APPEND "## Nix (flakes)" section | CREATE (WinGet section) |

**docs/packaging.md is the only shared file.** Both tasks write additive Markdown sections that
coexist (WinGet + Nix). Handle the race: if docs/packaging.md already exists (WinGet landed first),
APPEND the Nix section; if not, CREATE it with a neutral intro + the Nix section (the WinGet task's
own intro prose will merge harmlessly — its intro already says "This file covers WinGet...; npm is
documented in npm/README.md"). The Nix section is self-contained and appendable.

**Out of scope**: README install rewrite (Mode B, P3.M1), any Go file, .goreleaser.yaml, npm/*,
PRD.md/plan/**/tasks.json, flake.lock is CREATED+COMMITTED (not a forbidden file — it's auto-generated
nix metadata, like go.sum).