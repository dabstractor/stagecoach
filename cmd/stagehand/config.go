// config.go implements the `stagehand config` command tree (PRD §15.3, §16.1,
// §16.2, FR38). It is pure CLI glue over the already-built dependency
// config.GlobalConfigPath (P1.M5.T2.S1) and the inline-documented example
// template config.ExampleConfig (P1.M7.T3.S2), exposing two subcommands:
//
//   - `config path` (FR38): print the XDG-resolved global config path.
//   - `config init` (FR38): write the commented §16.2 example template to that
//     path (refusing to clobber an existing file without --force) and, when run
//     inside a git repo, offer to add ./.stagehand.toml to ./.gitignore
//     (PRD §16.1 layer-4 note).
//
// The tree self-registers onto the package-level rootCmd (defined in main.go)
// via a package-level init() below, so main.go stays untouched. That keeps
// this task (P1.M7.T3.S2) conflict-free with the sibling task P1.M7.T2.S1,
// which owns rootCmd.Run and the persistent flags. No Run/RunE is added to
// rootCmd here, and no persistent flags are registered; --force is a LOCAL
// flag on the `init` command only.
//
// This file is intentionally thin: it imports ONLY fmt/io/os/bufio/
// path/filepath/strings, cobra, and internal/config. It deliberately does NOT
// import internal/git or internal/generate — the config command needs neither,
// and keeping the leaf thin preserves the layering invariant.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dustin/stagehand/internal/config"
)

// init registers the config command tree onto rootCmd at package init time.
// Registering here (rather than by editing main.go) means this task does not
// touch main.go, avoiding a conflict with the sibling task P1.M7.T2.S1 that
// owns rootCmd.Run and the persistent flags.
func init() {
	rootCmd.AddCommand(newConfigCmd())
}

// newConfigCmd builds the `config` parent command with its `init` and `path`
// children (FR38). The parent itself has no Run, so a bare `stagehand config`
// prints the command's help (cobra default).
func newConfigCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:   "config",
		Short: "Scaffold and locate the stagehand config file",
		Long: "Scaffold and locate the stagehand configuration file (FR38).\n\n" +
			"  config init   Write a commented example config to the global path\n" +
			"                ($XDG_CONFIG_HOME/stagehand/config.toml). Refuses to\n" +
			"                overwrite without --force; offers to add\n" +
			"                ./.stagehand.toml to ./.gitignore when in a repo.\n\n" +
			"  config path   Print the resolved global config path.",
	}
	parent.AddCommand(newConfigInitCmd(), newConfigPathCmd())
	return parent
}

// newConfigPathCmd builds the `config path` subcommand (FR38). It resolves the
// global config path via config.GlobalConfigPath (XDG_CONFIG_HOME then
// $HOME/.config, always appending stagehand/config.toml) and prints it. When
// both XDG_CONFIG_HOME and HOME are unset GlobalConfigPath returns an error,
// which cobra surfaces as exit 1 via main.go.
func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved global config path (FR38)",
		Long: "Print the resolved global stagehand config path, honoring\n" +
			"XDG_CONFIG_HOME then $HOME/.config and always appending\n" +
			"stagehand/config.toml (PRD §16.1/§16.2, FR38). Errors (exit 1) when\n" +
			"neither XDG_CONFIG_HOME nor HOME is set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.GlobalConfigPath()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

// newConfigInitCmd builds the `config init` subcommand (FR38). It carries a
// LOCAL --force flag (NOT persistent — persistent flags are owned by the
// sibling task P1.M7.T2.S1). The resolved global path is the single write
// target: config init always targets GlobalConfigPath, so the future
// --config/STAGEHAND_CONFIG override discovery does not change where init
// writes. Writing the template and the optional gitignore offer are delegated
// to the hermetic helpers writeExampleConfig and offerGitignore.
func newConfigInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a commented example config to the global path (FR38)",
		Long: "Write a fully-commented example config (PRD §16.2 — every key\n" +
			"documented inline) to the resolved global config path\n" +
			"($XDG_CONFIG_HOME/stagehand/config.toml). The file is a documented\n" +
			"no-op: uncomment a line to change that setting. Refuses to overwrite an\n" +
			"existing file without --force. When run inside a git repository, offers\n" +
			"to add ./.stagehand.toml to ./.gitignore (PRD §16.1).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.GlobalConfigPath()
			if err != nil {
				return err
			}
			if err := writeExampleConfig(cmd.OutOrStdout(), p, force); err != nil {
				return err
			}
			return offerGitignore(cmd.OutOrStdout(), cmd.InOrStdin(), ".")
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite an existing config file")
	return cmd
}

// writeExampleConfig writes the commented example template (config.ExampleConfig)
// to path, creating parent directories as needed. It refuses to overwrite an
// existing file unless force is true, returning an error mentioning "already
// exists" so cobra surfaces it as exit 1 with the file left untouched. It is a
// hermetic test target: all I/O is via the explicit path argument and the out
// writer.
func writeExampleConfig(out io.Writer, path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config: %s already exists (rerun with --force to overwrite)", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(config.ExampleConfig()), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(out, "Wrote commented example config to %s\n", path)
	return nil
}

// offerGitignore optionally appends ./.stagehand.toml to cwd/.gitignore (PRD
// §16.1 layer-4 note: the per-repo file is "added to a generated .gitignore
// only on config init if the user confirms"). It is a no-op unless cwd/.git
// exists (not a repo), and defaults to No on an empty/EOF/non-yes answer so it
// is safe non-interactively. It is a hermetic test target via its (out, in,
// cwd) parameters.
func offerGitignore(out io.Writer, in io.Reader, cwd string) error {
	if _, err := os.Stat(filepath.Join(cwd, ".git")); err != nil {
		return nil // not a git repo: nothing to offer
	}
	fmt.Fprint(out, "Add ./.stagehand.toml to your local .gitignore? [y/N] ")
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		// empty/EOF → default No, no-op.
		return nil
	}
	if !isYes(sc.Text()) {
		return nil
	}
	gi := filepath.Join(cwd, ".gitignore")
	f, err := os.OpenFile(gi, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, ".stagehand.toml"); err != nil {
		return err
	}
	fmt.Fprintln(out, "Added ./.stagehand.toml to .gitignore")
	return nil
}

// isYes reports whether s is an affirmative answer to a yes/no prompt. It
// accepts "y"/"yes" case-insensitively after trimming surrounding whitespace;
// anything else (including "" and EOF, which never reaches here) is treated as
// No.
func isYes(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	return t == "y" || t == "yes"
}
