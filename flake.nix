# flake.nix — Stagecoach Nix packaging surface (PRD §21.2; external_deps.md §5).
#
# Builds the SAME binary goreleaser ships — `cmd/stagecoach`, CGO_ENABLED=0 — via nixpkgs
# `buildGoModule`, over the four systems (x86_64-linux, aarch64-linux, x86_64-darwin,
# aarch64-darwin). goreleaser remains the canonical release path (real version strings via
# ldflags); this flake is the reproducible local/dev/Nix-OS install surface.
#
# Usage:
#   nix run github:dabstractor/stagecoach             # run without installing
#   nix profile install github:dabstractor/stagecoach # install into the user profile
#   nix develop                                       # hermetic dev shell (go + gopls)
#   nix build .#default && ./result/bin/stagecoach --help
#
# ─── vendorHash update workflow (maintainer) ────────────────────────────────────
# `buildGoModule` pins the Go dependency tree with a `vendorHash`. Every go.mod/go.sum change
# invalidates it. To obtain the real hash:
#   1. Set `vendorHash = pkgs.lib.fakeHash;` (the placeholder) if starting fresh.
#   2. Run `nix build .#default` (or `nix flake check`). It FAILS with a fixed-output mismatch:
#          specified: sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
#             got:    sha256-<REAL-SRI-HASH>
#   3. Paste the `got:` value (the whole `sha256-...=` SRI string) into `vendorHash` below.
#   4. Re-run `nix build`/`nix flake check` → green.
# CI's `nix flake-check` job keeps this current: a stale hash fails the job with the got-hash
# to paste, so a stale Nix surface cannot merge.
#
# ─── Gotchas ────────────────────────────────────────────────────────────────────
# * Binary version is "dev": the flake does NOT inject goreleaser's `-X main.version=…` ldflags,
#   and Go's VCS embedding (vcs.revision) is unavailable in the sandbox (source is copied to the
#   store WITHOUT .git) → `resolveVersion()` returns plain "dev". For a real version string, use a
#   goreleaser GitHub Release. The flake's `version` below is the DERIVATION name, not the binary's
#   self-reported version.
# * CGO_ENABLED=0 via `env` (the modern idiom): matches .goreleaser.yaml + ci.yml's static-build
#   invariant.
# * `flake.lock` is COMMITTED (stagecoach is an application; the lock pins nixpkgs for reproducible
#   CI). Do NOT gitignore it.
# * eachSystem via `nixpkgs.lib.genAttrs` — no flake-utils input (one fewer dependency).
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
          # Date-based unstable: zero-maintenance (no edge cases around dirty refs). This is the
          # DERIVATION name; the nix-built binary still reports "dev" (see header comment).
          version = "unstable-${self.sourceInfo.lastModifiedDate}";
          src = self.outPath; # the flake source root (working tree / store path)
          # REAL vendorHash (extracted via ci.yml's `nix build .#default` — `nix flake check` only
          # evaluates, so a stale hash is caught only by an actual build). To update when go.mod/
          # go.sum change: set this to lib.fakeHash, run `nix build .#default`, paste the `got:` SRI
          # hash, and commit flake.lock too if the pinned nixpkgs rev moved.
          vendorHash = "sha256-cfUrtq/ds3WjzwgayY3hI8/nGePdTlRFwkflA2fcpyY=";
          subPackages = [ "cmd/stagecoach" ];
          env.CGO_ENABLED = 0; # static build; matches .goreleaser.yaml + ci.yml
        };
      });

      devShells = eachSystem (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go gopls ];
        };
      });
    };
}