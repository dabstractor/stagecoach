# @dabstractor/stagecoach

`@dabstractor/stagecoach` is a thin npm wrapper that downloads the **stagecoach** native binary at install time (via a `postinstall` script) and execs it on every `stagecoach` invocation. stagecoach is a snapshot-based AI commit-message generator that uses YOUR local CLI agent. If the `postinstall` download was blocked (for example by running `npm install --ignore-scripts` or a corporate npm mirror that disables scripts), the `stagecoach` command prints a message pointing at https://github.com/dabstractor/stagecoach#install — install directly or use a package manager that allows `postinstall`.

## Install

```sh
npm install -g @dabstractor/stagecoach
stagecoach
```

See the main repository for features, configuration, and the full CLI reference.