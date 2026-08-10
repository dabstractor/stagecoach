# stagecoach-ai

`stagecoach-ai` is a thin npm wrapper that downloads the **stagecoach** native binary at install time (via a `postinstall` script) and execs it on every `stagecoach` invocation. stagecoach is a snapshot-based AI commit-message generator that uses YOUR local CLI agent. If the `postinstall` download was blocked (for example by running `npm install --ignore-scripts` or a corporate npm mirror that disables scripts), the `stagecoach` command prints a message pointing at https://github.com/dabstractor/stagecoach#install — install directly or use a package manager that allows `postinstall`.

## Install

```sh
npm install -g stagecoach-ai
stagecoach
```

See the main repository for features, configuration, and the full CLI reference.

## Publishing (for maintainers)

This package is published **automatically** when a `v*` tag is pushed (the same tag that fires
[goreleaser](https://github.com/dabstractor/stagecoach/blob/main/.github/workflows/release.yml)).
goreleaser has no native npm pipe, so an `npm-publish` job (in `release.yml`) runs after the
`goreleaser` job:

1. Syncs `package.json` `version` to the tag (`v1.2.0` → `1.2.0`; the committed `version` stays
 `0.0.0`, a dev placeholder).
2. `npm install --ignore-scripts --no-package-lock` — validates the `tar` / `extract-zip` deps
 resolve WITHOUT running `postinstall` (the native binary is fetched at *user* install time, not
 publish time) and WITHOUT writing a lockfile into the tarball.
3. `npm publish` — the package is **unscoped** (`stagecoach-ai`), so it is public by default; no
 `--access` flag is needed (that flag is scoped-packages-only).

**Required secret:** `NPM_TOKEN` — an npm **automation** token (npmjs.com → Access Tokens →
"Automation"), which bypasses publish 2FA for non-interactive CI. Add it under the repo's
Settings → Secrets → Actions. (A "Publish" token will fail in CI if the account has publish 2FA on.)

The wrapper's smoke tests (the shim + the installer) run on every PR in
[`ci.yml`](https://github.com/dabstractor/stagecoach/blob/main/.github/workflows/ci.yml) (`npm-smoke`
job) to catch regressions before a release. See the main repo's
[distribution notes](https://github.com/dabstractor/stagecoach#install) for all install channels.

> Optional future enhancement: add a `files` field to `package.json` to slim the tarball (the `test/*`
> files currently ship, ~9KB extra, which is harmless) and/or use `npm publish --tag next` for
> prerelease tags so `npm install` does not resolve an RC as `latest`. Both require a
> `package.json` change and are intentionally out of scope for the initial publish wiring.