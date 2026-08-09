# stagecoach — asdf / mise plugin

An [asdf](https://asdf-vm.com/) / [mise](https://mise.jdx.dev/) plugin for
[stagecoach](https://github.com/dabstractor/stagecoach) — a snapshot-based AI
commit message generator that uses YOUR local CLI agent.

This plugin installs a SHA256-verified goreleaser release binary into
`$ASDF_INSTALL_PATH/bin/stagecoach`. **One set of POSIX-sh scripts serves BOTH
asdf and mise** — mise runs asdf plugins unchanged and sets the same `ASDF_*`
environment variables (see [mise asdf-legacy-plugins](https://mise.jdx.dev/asdf-legacy-plugins.html)).

## Prerequisites

- [asdf](https://asdf-vm.com/) **or** [mise](https://mise.jdx.dev/)
- `git`
- `curl`
- `tar`
- `awk`
- a SHA256 tool: **`sha256sum`** (Linux/coreutils) **or** `shasum -a 256` (macOS/BSD)

## Install (asdf)

```sh
asdf plugin add stagecoach https://github.com/dabstractor/asdf-stagecoach.git
asdf install stagecoach latest
```

`asdf install` only downloads the binary — you also have to **select** a version for
the `stagecoach` shim to resolve. The command depends on your asdf: **asdf 0.16**
(the Go rewrite) replaced `global`/`local` with `set`.

| Scope | asdf < 0.16 (classic) | asdf ≥ 0.16 (Go rewrite) |
| --- | --- | --- |
| Every shell (`~/.tool-versions`) | `asdf global stagecoach latest` | `asdf set --home stagecoach latest` |
| Current project (`./.tool-versions`) | `asdf local stagecoach latest` | `asdf set stagecoach latest` |

`latest` is resolved via `bin/latest-stable`, so the pinned value is concrete
(e.g. `0.1.10`), not the literal string `latest`.

> **PATH prerequisite.** For *any* asdf-managed tool to run, asdf's shims directory
> (`~/.asdf/shims`, i.e. `$ASDF_DATA_DIR/shims`) must be on `PATH` — that's part of
> asdf's own setup, not the plugin's. If `asdf install` succeeds but `stagecoach`
> reports `command not found`, add this to your shell rc:
> `export PATH="${ASDF_DATA_DIR:-$HOME/.asdf}/shims:$PATH"`.

```sh
stagecoach --version      # → stagecoach version 0.1.10
asdf current stagecoach   # confirm which version is active
```

Pin a project to an exact version with a `.tool-versions` file (read by both asdf
variants and by mise):

```
stagecoach 1.2.3
```

## Install (mise)

mise runs the SAME asdf plugin scripts, so `latest` is resolved by mise with no
extra setup:

```sh
mise plugin add stagecoach https://github.com/dabstractor/asdf-stagecoach.git
mise install stagecoach@latest
mise use -g stagecoach@latest      # global; drop -g to pin just this project
stagecoach --version
```

> **Activation prerequisite.** For mise-managed tools to be found as bare commands,
> mise must be activated in your shell — add this ONE line to your shell rc:
> `eval "$(mise activate zsh)"` (use `bash` or `fish` as appropriate). Without it
> the install still works (`mise exec stagecoach -- stagecoach …` always runs) but a
> plain `stagecoach` won't resolve.

Pin a project to a version in `mise.toml`:

```toml
[tools]
stagecoach = "1.2.3"
```

## Supported platforms

- **macOS** (darwin) — amd64, arm64
- **Linux** — amd64, arm64

Windows is **not** supported via this plugin (asdf/mise are Unix-only). Windows
users should use [Scoop](https://scoop.sh/),
[Chocolatey](https://chocolatey.org/docs/installation), or the
[PowerShell installer](https://github.com/dabstractor/stagecoach#install) —
see the [stagecoach install guide](https://github.com/dabstractor/stagecoach#install).

## How it works

- **`bin/list-all`** enumerates installable versions using
  `git ls-remote --refs --tags` (it does **not** use the GitHub Releases API,
  which is rate-limited to 60 req/h/IP for unauthenticated callers and would
  break `list-all` / tab-completion at scale). Tags are stripped of their
  leading `v` and sorted ascending via `sort -V` (newest last).
- **`bin/latest-stable`** resolves the single newest version for
  `asdf install stagecoach latest` / `asdf latest stagecoach [<prefix>]` (and
  the `mise` equivalents). asdf/mise take this callback's output at face value;
  without it, asdf would hand `bin/install` the **whole** version list as
  `ASDF_INSTALL_VERSION`, producing a multi-line download URL that `curl`
  rejects. It is simply the last (newest) line of `bin/list-all`.
- **`bin/install`** reads the standard `ASDF_INSTALL_TYPE` / `ASDF_INSTALL_VERSION`
  / `ASDF_INSTALL_PATH` environment variables (mise sets the **same** vars), maps
  the host `uname` to a goreleaser `GOOS`/`GOARCH`, downloads the matching
  `stagecoach_<v>_<os>_<arch>.tar.gz` + `stagecoach_<v>_checksums.txt` from the
  project's GitHub Releases, SHA256-verifies the archive against its checksums
  line, and extracts the single `stagecoach` binary into `$ASDF_INSTALL_PATH/bin/`.
  On any failure (checksum mismatch, missing checksum line, unsupported platform)
  it aborts **non-zero with no binary left behind** (abort-before-write).

## Repository / canonical source

The scripts under
[`github.com/dabstractor/stagecoach` → `plugins/asdf-stagecoach/`](https://github.com/dabstractor/stagecoach/tree/main/plugins/asdf-stagecoach)
are the **canonical source**. They are **mirrored** to the separate plugin repo
[`github.com/dabstractor/asdf-stagecoach`](https://github.com/dabstractor/asdf-stagecoach)
(the URL `asdf plugin add` / `mise plugin add` points at). Changes are made in
the stagecoach repo first.

## License

Same as the stagecoach project — see the [main repository](https://github.com/dabstractor/stagecoach).