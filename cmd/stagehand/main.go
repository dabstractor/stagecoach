// Package main implements the stagehand CLI entrypoint.
//
// This is the root of the build DAG. It wires a cobra root command with the
// module version injected at link time via -ldflags "-X main.version=<VERSION>".
//
// The root command intentionally has NO Run/RunE: the default action
// (maybeAutoStage + CommitStaged) is wired in a later task (M7.T2). For now
// `stagehand` prints help (exit 0) and `stagehand --version` prints the
// version (exit 0) via cobra's built-in Version field.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

// version is the build version. It is overridden at link time with
// -ldflags "-X main.version=$(VERSION)". The "dev" default is only used when
// building without ldflags injection (e.g. `go run`, `go build` without flags).
var version = "dev"

// rootCmd is the stagehand root command. Setting Version (rather than
// registering a manual --version flag) lets cobra auto-add a --version flag
// that prints "stagehand version <version>" to stdout and exits 0 — even
// without a Run handler. A manual StringVar --version flag would require an
// argument and break bare `stagehand --version`, so the built-in field is used.
var rootCmd = &cobra.Command{
	Use:     "stagehand",
	Short:   "Conventional, AI-friendly Git commits staged from natural language",
	Version: version,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
